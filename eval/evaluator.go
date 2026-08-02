package eval

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/antlr4-go/antlr/v4"

	"github.com/gofhir/fhirpath/parser/grammar"
	"github.com/gofhir/fhirpath/types"
)

// FuncImpl is the signature for function implementations.
type FuncImpl func(ctx *Context, input types.Collection, args []interface{}) (types.Collection, error)

// FuncDef defines a FHIRPath function.
type FuncDef struct {
	Name     string
	MinArgs  int
	MaxArgs  int
	Fn       FuncImpl
	TypeArgs []int // Indices of arguments that are type specifiers (extracted as strings, not evaluated as expressions)
}

// FuncRegistry is an interface for function lookup.
type FuncRegistry interface {
	Get(name string) (FuncDef, bool)
}

// Resolver handles FHIR reference resolution.
type Resolver interface {
	Resolve(ctx context.Context, reference string) ([]byte, error)
}

// TerminologyService handles terminology operations like ValueSet membership.
type TerminologyService interface {
	// MemberOf checks if a code/Coding/CodeableConcept is in the specified ValueSet.
	// Returns true if the code is in the ValueSet, false otherwise.
	// Returns error if the ValueSet cannot be resolved or validation fails.
	MemberOf(ctx context.Context, code interface{}, valueSetURL string) (bool, error)
}

// ProfileValidator handles profile conformance validation.
type ProfileValidator interface {
	// ConformsTo checks if a resource conforms to the specified profile.
	// Returns true if the resource conforms, false otherwise.
	ConformsTo(ctx context.Context, resource []byte, profileURL string) (bool, error)
}

// Model provides FHIR version-specific type and path metadata.
// When set on a Context, the evaluator uses it for precise polymorphic
// field resolution, type hierarchy checking, and path-based type inference.
// When nil, the evaluator falls back to built-in heuristics.
type Model interface {
	ChoiceTypes(path string) []string
	TypeOf(path string) string
	ReferenceTargets(path string) []string
	ParentType(typeName string) string
	IsSubtype(child, parent string) bool
	ResolvePath(path string) string
	IsResource(typeName string) bool
}

// Evaluator evaluates FHIRPath expressions using the visitor pattern.
type Evaluator struct {
	grammar.BasefhirpathVisitor
	ctx   *Context
	funcs FuncRegistry
}

// Context holds the evaluation state.
type Context struct {
	root               types.Collection
	this               types.Collection
	index              int
	total              types.Value
	variables          map[string]types.Collection
	outer              types.Collection            // scope a function's arguments are evaluated in
	defined            map[string]types.Collection // variables introduced by defineVariable()
	limits             map[string]int
	goCtx              context.Context
	resolver           Resolver
	terminologyService TerminologyService
	profileValidator   ProfileValidator
	model              Model  // FHIR version-specific model data (nil = use heuristics)
	path               string // current FHIR navigation path (e.g., "Patient.name")
}

// FHIR environment variables with a fixed value, defined by the FHIR
// specification rather than supplied by the caller. Invariants such as age-1,
// drt-1, cnt-3 and dis-1 compare against %ucum.
const (
	ucumSystem  = "http://unitsofmeasure.org"
	sctSystem   = "http://snomed.info/sct"
	loincSystem = "http://loinc.org"

	// Prefixes for the parameterized forms %"vs-[name]" and %"ext-[name]".
	valueSetPrefix          = "vs-"
	valueSetBaseURL         = "http://hl7.org/fhir/ValueSet/"
	structureDefPrefix      = "ext-"
	structureDefinitionBase = "http://hl7.org/fhir/StructureDefinition/"
)

// NewContext creates a new evaluation context.
// Automatically sets %resource, %rootResource, and %context to the root resource for FHIR constraint evaluation.
// Per FHIRPath spec:
//   - %resource: the root resource being evaluated
//   - %rootResource: the root resource in the evaluation context (differs from %resource for contained/Bundle resources)
//   - %context: the original node passed to the evaluation engine (same as %resource for top-level evaluation)
//
// The fixed FHIR constants %ucum, %sct and %loinc are also defined; callers can
// override any of them via SetVariable.
func NewContext(resource []byte) *Context {
	//nolint:errcheck // Empty collection is acceptable for invalid JSON in context creation
	root, _ := types.JSONToCollection(resource)

	// Initialize variables map with %resource, %rootResource, and %context pointing to root
	// %resource is required by FHIR constraints like bdl-3, bdl-4
	// %rootResource defaults to %resource; callers can override via SetVariable for nested evaluation
	// %context represents the evaluation context (same as root for top-level evaluation)
	variables := make(map[string]types.Collection)
	variables["resource"] = root
	variables["rootResource"] = root
	variables["context"] = root
	variables["ucum"] = types.Collection{types.NewString(ucumSystem)}
	variables["sct"] = types.Collection{types.NewString(sctSystem)}
	variables["loinc"] = types.Collection{types.NewString(loincSystem)}

	return &Context{
		root:      root,
		this:      root,
		variables: variables,
		limits:    make(map[string]int),
		goCtx:     context.Background(),
	}
}

// SetLimit sets a limit value (e.g., maxDepth, maxCollectionSize).
func (c *Context) SetLimit(name string, value int) {
	if c.limits == nil {
		c.limits = make(map[string]int)
	}
	c.limits[name] = value
}

// GetLimit gets a limit value.
func (c *Context) GetLimit(name string) int {
	if c.limits == nil {
		return 0
	}
	return c.limits[name]
}

// SetContext sets the Go context for cancellation.
func (c *Context) SetContext(ctx context.Context) {
	c.goCtx = ctx
}

// Context returns the Go context.
func (c *Context) Context() context.Context {
	if c.goCtx == nil {
		return context.Background()
	}
	return c.goCtx
}

// SetResolver sets the reference resolver.
func (c *Context) SetResolver(r Resolver) {
	c.resolver = r
}

// GetResolver returns the reference resolver.
func (c *Context) GetResolver() Resolver {
	return c.resolver
}

// SetTerminologyService sets the terminology service for memberOf validation.
func (c *Context) SetTerminologyService(ts TerminologyService) {
	c.terminologyService = ts
}

// GetTerminologyService returns the terminology service.
func (c *Context) GetTerminologyService() TerminologyService {
	return c.terminologyService
}

// SetProfileValidator sets the profile validator for conformsTo validation.
func (c *Context) SetProfileValidator(pv ProfileValidator) {
	c.profileValidator = pv
}

// GetProfileValidator returns the profile validator.
func (c *Context) GetProfileValidator() ProfileValidator {
	return c.profileValidator
}

// SetModel sets the FHIR model for version-specific type resolution.
func (c *Context) SetModel(m Model) {
	c.model = m
}

// GetModel returns the FHIR model, or nil if not set.
func (c *Context) GetModel() Model {
	return c.model
}

// SetPath sets the current FHIR navigation path.
func (c *Context) SetPath(path string) {
	c.path = path
}

// Path returns the current FHIR navigation path.
func (c *Context) Path() string {
	return c.path
}

// CheckCancellation checks if the context has been canceled.
func (c *Context) CheckCancellation() error {
	if c.goCtx == nil {
		return nil
	}
	select {
	case <-c.goCtx.Done():
		return c.goCtx.Err()
	default:
		return nil
	}
}

// CheckCollectionSize validates that a collection doesn't exceed the maximum size.
// Returns an error if the collection is too large.
func (c *Context) CheckCollectionSize(col types.Collection) error {
	maxSize := c.GetLimit("maxCollectionSize")
	if maxSize > 0 && len(col) > maxSize {
		return NewEvalError(ErrInvalidExpression,
			"collection size %d exceeds maximum allowed %d", len(col), maxSize)
	}
	return nil
}

// EnforceCollectionLimit truncates a collection if it exceeds the maximum size.
// Returns the (possibly truncated) collection and whether truncation occurred.
func (c *Context) EnforceCollectionLimit(col types.Collection) (types.Collection, bool) {
	maxSize := c.GetLimit("maxCollectionSize")
	if maxSize > 0 && len(col) > maxSize {
		return col[:maxSize], true
	}
	return col, false
}

// Root returns the root collection.
func (c *Context) Root() types.Collection {
	return c.root
}

// This returns the current $this value.
func (c *Context) This() types.Collection {
	return c.this
}

// WithThis returns a new context with the given $this value.
func (c *Context) WithThis(this types.Collection) *Context {
	newCtx := *c
	newCtx.this = this
	return &newCtx
}

// WithIndex returns a new context with the given $index value.
func (c *Context) WithIndex(index int) *Context {
	newCtx := *c
	newCtx.index = index
	return &newCtx
}

// SetVariable sets an external variable.
func (c *Context) SetVariable(name string, value types.Collection) {
	c.variables[name] = value
}

// GetVariable gets an external variable.
func (c *Context) GetVariable(name string) (types.Collection, bool) {
	// A variable introduced by defineVariable() takes precedence: it belongs to
	// the expression currently being evaluated, which is narrower than the
	// environment the caller set up.
	if v, ok := c.defined[name]; ok {
		return v, true
	}

	v, ok := c.variables[name]
	return v, ok
}

// DefineVariable introduces a variable for the remainder of the current
// expression scope, as defineVariable() does.
//
// Redefining a name already in scope is an error the specification calls for
// explicitly: "If the name already exists in the current expression scope, the
// evaluation will end and signal an error to the calling environment." That
// covers the environment's own variables, so an expression cannot shadow
// %resource or %context either.
func (c *Context) DefineVariable(name string, value types.Collection) error {
	if _, exists := c.defined[name]; exists {
		return NewEvalError(ErrInvalidOperation, "variable %%%s is already defined in this scope", name)
	}
	if _, exists := c.variables[name]; exists {
		return NewEvalError(ErrInvalidOperation, "variable %%%s is already defined by the evaluation environment", name)
	}

	if c.defined == nil {
		c.defined = make(map[string]types.Collection, 1)
	}
	c.defined[name] = value
	return nil
}

// enterIterationScope opens the scope one element of an iteration is evaluated
// in, returning the call that closes it.
//
// Two things belong to that scope. Variables defined inside it are discarded at
// the end, so the second element of select(defineVariable('x')...) does not find
// x already defined — the specification describes exactly this: "the temporary
// variable would be popped off the stack". And the scope a function's arguments
// are navigated from becomes the element itself, so that in
// where(substring($this.length()-3) = 'ter') the argument reads the item under
// test rather than whatever preceded the iteration.
//
// Variables are copied only when there are any, so an expression that never
// calls defineVariable() pays a length check per element.
func (c *Context) enterIterationScope() func() {
	outer := c.outer
	c.outer = nil

	if len(c.defined) == 0 {
		// Nothing defined yet, so ending the scope means discarding whatever the
		// iteration introduces
		return func() {
			c.defined = nil
			c.outer = outer
		}
	}

	saved := make(map[string]types.Collection, len(c.defined))
	for name, value := range c.defined {
		saved[name] = value
	}
	return func() {
		c.defined = saved
		c.outer = outer
	}
}

// NewEvaluator creates a new evaluator with the given context and function registry.
func NewEvaluator(ctx *Context, funcs FuncRegistry) *Evaluator {
	return &Evaluator{ctx: ctx, funcs: funcs}
}

// Evaluate evaluates a parse tree and returns the result.
func (e *Evaluator) Evaluate(tree antlr.ParseTree) (types.Collection, error) {
	result := e.Visit(tree)
	if err, ok := result.(error); ok {
		return nil, err
	}
	if col, ok := result.(types.Collection); ok {
		return col, nil
	}
	return types.Collection{}, nil
}

// Visit dispatches to the appropriate visitor method.
func (e *Evaluator) Visit(tree antlr.ParseTree) interface{} {
	if tree == nil {
		return types.Collection{}
	}
	return tree.Accept(e)
}

// VisitEntireExpression visits the root expression.
func (e *Evaluator) VisitEntireExpression(ctx *grammar.EntireExpressionContext) interface{} {
	return e.Visit(ctx.Expression())
}

// VisitTermExpression visits a term expression.
func (e *Evaluator) VisitTermExpression(ctx *grammar.TermExpressionContext) interface{} {
	return e.Visit(ctx.Term())
}

// VisitInvocationTerm visits an invocation term.
func (e *Evaluator) VisitInvocationTerm(ctx *grammar.InvocationTermContext) interface{} {
	return e.Visit(ctx.Invocation())
}

// VisitLiteralTerm visits a literal term.
func (e *Evaluator) VisitLiteralTerm(ctx *grammar.LiteralTermContext) interface{} {
	return e.Visit(ctx.Literal())
}

// VisitParenthesizedTerm visits a parenthesized expression.
func (e *Evaluator) VisitParenthesizedTerm(ctx *grammar.ParenthesizedTermContext) interface{} {
	return e.Visit(ctx.Expression())
}

// VisitExternalConstantTerm visits an external constant.
func (e *Evaluator) VisitExternalConstantTerm(ctx *grammar.ExternalConstantTermContext) interface{} {
	return e.Visit(ctx.ExternalConstant())
}

// VisitExternalConstant visits an external constant (%name).
func (e *Evaluator) VisitExternalConstant(ctx *grammar.ExternalConstantContext) interface{} {
	var name string
	if ctx.Identifier() != nil {
		name = stripBackticks(ctx.Identifier().GetText())
	} else if ctx.STRING() != nil {
		name = unquoteString(ctx.STRING().GetText())
	}

	if value, ok := e.ctx.GetVariable(name); ok {
		return value
	}
	if url, ok := fhirConstantURL(name); ok {
		return types.Collection{types.NewString(url)}
	}
	return NewEvalError(ErrInvalidPath, "undefined variable: %%%s", name)
}

// fhirConstantURL resolves the parameterized FHIR environment variables
// %"vs-[name]" and %"ext-[name]" to their canonical URLs.
// Note that the FHIR specification writes them with double quotes, which the
// FHIRPath grammar does not accept; use %'vs-name' or %`vs-name` instead.
func fhirConstantURL(name string) (string, bool) {
	switch {
	case strings.HasPrefix(name, valueSetPrefix):
		return valueSetBaseURL + strings.TrimPrefix(name, valueSetPrefix), true
	case strings.HasPrefix(name, structureDefPrefix):
		return structureDefinitionBase + strings.TrimPrefix(name, structureDefPrefix), true
	}
	return "", false
}

// Literal visitors

// VisitNullLiteral visits a null literal {}.
func (e *Evaluator) VisitNullLiteral(ctx *grammar.NullLiteralContext) interface{} {
	return types.Collection{}
}

// VisitBooleanLiteral visits a boolean literal.
func (e *Evaluator) VisitBooleanLiteral(ctx *grammar.BooleanLiteralContext) interface{} {
	text := ctx.GetText()
	return types.Collection{types.NewBoolean(text == "true")}
}

// VisitStringLiteral visits a string literal.
func (e *Evaluator) VisitStringLiteral(ctx *grammar.StringLiteralContext) interface{} {
	text := unquoteString(ctx.STRING().GetText())
	return types.Collection{types.NewString(text)}
}

// VisitNumberLiteral visits a number literal.
func (e *Evaluator) VisitNumberLiteral(ctx *grammar.NumberLiteralContext) interface{} {
	text := ctx.NUMBER().GetText()

	// Check if it's an integer
	if !strings.Contains(text, ".") {
		if i, err := strconv.ParseInt(text, 10, 64); err == nil {
			return types.Collection{types.NewInteger(i)}
		}
	}

	// Parse as decimal
	d, err := types.NewDecimal(text)
	if err != nil {
		return ParseError("invalid number: " + text)
	}
	return types.Collection{d}
}

// VisitDateLiteral visits a date literal.
func (e *Evaluator) VisitDateLiteral(ctx *grammar.DateLiteralContext) interface{} {
	text := ctx.DATE().GetText()
	// Remove the @ prefix
	if text != "" && text[0] == '@' {
		text = text[1:]
	}
	d, err := types.NewDate(text)
	if err != nil {
		return ParseError("invalid date: " + text)
	}
	return types.Collection{d}
}

// VisitDateTimeLiteral visits a datetime literal.
func (e *Evaluator) VisitDateTimeLiteral(ctx *grammar.DateTimeLiteralContext) interface{} {
	text := ctx.DATETIME().GetText()
	// Remove the @ prefix
	if text != "" && text[0] == '@' {
		text = text[1:]
	}
	dt, err := types.NewDateTime(text)
	if err != nil {
		return ParseError("invalid datetime: " + text)
	}
	return types.Collection{dt}
}

// VisitTimeLiteral visits a time literal.
func (e *Evaluator) VisitTimeLiteral(ctx *grammar.TimeLiteralContext) interface{} {
	text := ctx.TIME().GetText()
	// Remove the @ prefix
	if text != "" && text[0] == '@' {
		text = text[1:]
	}
	t, err := types.NewTime(text)
	if err != nil {
		return ParseError("invalid time: " + text)
	}
	return types.Collection{t}
}

// VisitQuantityLiteral visits a quantity literal.
func (e *Evaluator) VisitQuantityLiteral(ctx *grammar.QuantityLiteralContext) interface{} {
	text := ctx.GetText()
	q, err := types.NewQuantity(text)
	if err != nil {
		return ParseError("invalid quantity: " + text)
	}
	return types.Collection{q}
}

// Invocation visitors

// VisitMemberInvocation visits a member access.
func (e *Evaluator) VisitMemberInvocation(ctx *grammar.MemberInvocationContext) interface{} {
	name := stripBackticks(ctx.Identifier().GetText())
	return e.navigateMember(e.ctx.This(), name)
}

// VisitFunctionInvocation visits a function call.
func (e *Evaluator) VisitFunctionInvocation(ctx *grammar.FunctionInvocationContext) interface{} {
	funcCtx := ctx.Function()
	name := stripBackticks(funcCtx.Identifier().GetText())

	// Get function from registry
	fn, ok := e.funcs.Get(name)
	if !ok {
		return FunctionNotFoundError(name)
	}

	// Validate argument count
	paramList := funcCtx.ParamList()
	argCount := 0
	var argExprs []grammar.IExpressionContext
	if paramList != nil {
		argExprs = paramList.AllExpression()
		argCount = len(argExprs)
	}

	if argCount < fn.MinArgs {
		return InvalidArgumentsError(name, fn.MinArgs, argCount)
	}
	if fn.MaxArgs >= 0 && argCount > fn.MaxArgs {
		return InvalidArgumentsError(name, fn.MaxArgs, argCount)
	}

	// Handle special functions that need per-element evaluation
	input := e.ctx.This()
	switch name {
	case "where":
		if argCount > 0 {
			return e.evaluateWhere(input, argExprs[0])
		}
	case "exists":
		if argCount > 0 {
			return e.evaluateExists(input, argExprs[0])
		}
	case "all":
		if argCount > 0 {
			return e.evaluateAll(input, argExprs[0])
		}
	case "select":
		if argCount > 0 {
			return e.evaluateSelect(input, argExprs[0])
		}
	case "is":
		if argCount > 0 {
			return e.evaluateIsFunction(input, argExprs[0])
		}
	case "as":
		if argCount > 0 {
			return e.evaluateAsFunction(input, argExprs[0])
		}
	case "ofType":
		if argCount > 0 {
			return e.evaluateOfType(input, argExprs[0])
		}
	case "sort":
		// sort() needs its criteria evaluated per element, with $this bound
		return e.evaluateSort(input, argExprs)
	case "aggregate":
		if argCount >= 1 {
			return e.evaluateAggregate(input, argExprs)
		}
	case "iif":
		// iif requires lazy evaluation - only evaluate the branch that matches
		if argCount >= 2 {
			return e.evaluateIif(input, argExprs)
		}
	case "repeat":
		// repeat() re-applies its projection to whatever the last round
		// produced, so the expression has to be evaluated per element per round
		if argCount > 0 {
			return e.evaluateRepeat(input, argExprs[0], true)
		}
	case "repeatAll":
		if argCount > 0 {
			return e.evaluateRepeat(input, argExprs[0], false)
		}
	case "defineVariable":
		// defineVariable() alters the scope the rest of the expression is
		// evaluated in, which a function returning a collection cannot do
		if argCount >= 1 {
			return e.evaluateDefineVariable(input, argExprs)
		}
	case "coalesce":
		// coalesce short-circuits: arguments after the first non-empty one are
		// never evaluated
		if argCount >= 1 {
			return e.evaluateCoalesce(argExprs)
		}
	}

	// Evaluate arguments in the scope the invocation sits in, not in its input
	args := make([]interface{}, argCount)
	if argCount > 0 {
		argThis := e.ctx.this
		if e.ctx.outer != nil {
			argThis = e.ctx.outer
		}

		restore := e.ctx.this
		e.ctx.this = argThis
		for i, argExpr := range argExprs {
			if isTypeArg(fn.TypeArgs, i) {
				// Extract type name from AST instead of evaluating as expression
				typeName := e.extractTypeNameFromExpr(argExpr)
				args[i] = types.Collection{types.NewString(typeName)}
				continue
			}

			result := e.Visit(argExpr)
			if err, ok := result.(error); ok {
				e.ctx.this = restore
				return err
			}
			args[i] = result
		}
		e.ctx.this = restore
	}

	// Call the function
	result, err := fn.Fn(e.ctx, e.ctx.This(), args)
	if err != nil {
		return err
	}
	return result
}

// evaluateWhere evaluates the where() function with per-element criteria.
func (e *Evaluator) evaluateWhere(input types.Collection, criteria grammar.IExpressionContext) interface{} {
	result := types.Collection{}

	// Check collection size limit
	if err := e.ctx.CheckCollectionSize(input); err != nil {
		return err
	}

	for i, item := range input {
		// Check for cancellation periodically (every 100 iterations)
		if i%100 == 0 {
			if err := e.ctx.CheckCancellation(); err != nil {
				return err
			}
		}

		// Set $this to current item and $index
		oldThis := e.ctx.this
		oldIndex := e.ctx.index
		oldPath := e.ctx.path
		endScope := e.ctx.enterIterationScope()
		e.ctx.this = types.Collection{item}
		e.ctx.index = i

		// Evaluate the criteria
		criteriaResult := e.Visit(criteria)

		// Restore context
		e.ctx.this = oldThis
		e.ctx.index = oldIndex
		e.ctx.path = oldPath
		endScope()

		if err, ok := criteriaResult.(error); ok {
			return err
		}

		// Check if criteria is true
		if col, ok := criteriaResult.(types.Collection); ok {
			if val, isBool := col.SingletonBoolean(); isBool && val {
				result = append(result, item)
			}
		}
	}

	return result
}

// evaluateExists evaluates exists() with optional criteria.
func (e *Evaluator) evaluateExists(input types.Collection, criteria grammar.IExpressionContext) interface{} {
	for i, item := range input {
		// Check for cancellation periodically
		if i%100 == 0 {
			if err := e.ctx.CheckCancellation(); err != nil {
				return err
			}
		}

		// Set $this to current item
		oldThis := e.ctx.this
		oldIndex := e.ctx.index
		oldPath := e.ctx.path
		endScope := e.ctx.enterIterationScope()
		e.ctx.this = types.Collection{item}
		e.ctx.index = i

		// Evaluate the criteria
		criteriaResult := e.Visit(criteria)

		// Restore context
		e.ctx.this = oldThis
		e.ctx.index = oldIndex
		e.ctx.path = oldPath
		endScope()

		if err, ok := criteriaResult.(error); ok {
			return err
		}

		// Check if criteria is true
		if col, ok := criteriaResult.(types.Collection); ok {
			if val, isBool := col.SingletonBoolean(); isBool && val {
				return types.Collection{types.NewBoolean(true)}
			}
		}
	}

	return types.Collection{types.NewBoolean(false)}
}

// evaluateAll evaluates all() - returns true if all elements match criteria.
func (e *Evaluator) evaluateAll(input types.Collection, criteria grammar.IExpressionContext) interface{} {
	if input.Empty() {
		return types.Collection{types.NewBoolean(true)}
	}

	for i, item := range input {
		// Check for cancellation periodically
		if i%100 == 0 {
			if err := e.ctx.CheckCancellation(); err != nil {
				return err
			}
		}

		// Set $this to current item
		oldThis := e.ctx.this
		oldIndex := e.ctx.index
		oldPath := e.ctx.path
		endScope := e.ctx.enterIterationScope()
		e.ctx.this = types.Collection{item}
		e.ctx.index = i

		// Evaluate the criteria
		criteriaResult := e.Visit(criteria)

		// Restore context
		e.ctx.this = oldThis
		e.ctx.index = oldIndex
		e.ctx.path = oldPath
		endScope()

		if err, ok := criteriaResult.(error); ok {
			return err
		}

		// Check if criteria is true
		if col, ok := criteriaResult.(types.Collection); ok {
			if val, isBool := col.SingletonBoolean(); !isBool || !val {
				return types.Collection{types.NewBoolean(false)}
			}
		}
	}

	return types.Collection{types.NewBoolean(true)}
}

// sortCriterion is one ordering key of sort(): the expression to evaluate for
// each element, and the direction it imposes.
type sortCriterion struct {
	expr       grammar.IExpressionContext
	descending bool
}

// evaluateSort evaluates sort() — orders the input collection.
//
// Without arguments the elements are ordered by their own value. Each argument
// is an ordering key evaluated with $this bound to the element, and a leading
// minus reverses that key: sort(-family, given) orders by family descending,
// then by given ascending. Note the minus is a direction marker, not arithmetic
// negation, which is why it also applies to strings and dates.
//
// Ordering is stable, so elements that compare equal keep their input order.
func (e *Evaluator) evaluateSort(input types.Collection, argExprs []grammar.IExpressionContext) interface{} {
	if len(input) < 2 {
		return input
	}

	criteria := make([]sortCriterion, 0, len(argExprs))
	for _, argExpr := range argExprs {
		expr, descending := unwrapSortDirection(argExpr)
		criteria = append(criteria, sortCriterion{expr: expr, descending: descending})
	}

	// Evaluate every key once per element rather than on each comparison
	keys := make([][]types.Collection, len(input))
	for i, item := range input {
		if len(criteria) == 0 {
			// The element itself is the key
			keys[i] = []types.Collection{{item}}
			continue
		}

		itemKeys := make([]types.Collection, len(criteria))
		for j, criterion := range criteria {
			result := e.evaluateWithThis(item, i, criterion.expr)
			if err, ok := result.(error); ok {
				return err
			}
			col, _ := result.(types.Collection)
			itemKeys[j] = col
		}
		keys[i] = itemKeys
	}

	order := make([]int, len(input))
	for i := range order {
		order[i] = i
	}

	sort.SliceStable(order, func(a, b int) bool {
		return compareSortKeys(keys[order[a]], keys[order[b]], criteria) < 0
	})

	result := make(types.Collection, len(input))
	for i, idx := range order {
		result[i] = input[idx]
	}
	return result
}

// unwrapSortDirection strips a leading minus from an ordering key, reporting
// that the key sorts descending.
func unwrapSortDirection(expr grammar.IExpressionContext) (grammar.IExpressionContext, bool) {
	polarity, ok := expr.(*grammar.PolarityExpressionContext)
	if !ok {
		return expr, false
	}
	if node, ok := polarity.GetChild(0).(antlr.TerminalNode); ok && node.GetText() == "-" {
		return polarity.Expression(), true
	}
	return expr, false
}

// evaluateWithThis evaluates an expression with $this bound to a single element.
func (e *Evaluator) evaluateWithThis(item types.Value, index int, expr grammar.IExpressionContext) interface{} {
	oldThis, oldIndex, oldPath := e.ctx.this, e.ctx.index, e.ctx.path
	e.ctx.this = types.Collection{item}
	e.ctx.index = index

	result := e.Visit(expr)

	e.ctx.this, e.ctx.index, e.ctx.path = oldThis, oldIndex, oldPath
	return result
}

// compareSortKeys orders two elements by their keys, applying each criterion in
// turn until one of them decides. Keys whose values cannot be compared are
// treated as equal, so an incomparable pair falls through to the next criterion
// rather than failing the whole sort.
func compareSortKeys(left, right []types.Collection, criteria []sortCriterion) int {
	for i := range left {
		cmp := compareKeyValues(left[i], right[i])
		if cmp == 0 {
			continue
		}
		if i < len(criteria) && criteria[i].descending {
			return -cmp
		}
		return cmp
	}
	return 0
}

// compareKeyValues compares two ordering keys. An empty key sorts after a
// present one, so elements missing the key end up last.
func compareKeyValues(left, right types.Collection) int {
	switch {
	case left.Empty() && right.Empty():
		return 0
	case left.Empty():
		return 1
	case right.Empty():
		return -1
	}

	cmp, err := Compare(left[0], right[0])
	if err != nil {
		return 0
	}
	return cmp
}

// evaluateSelect evaluates select() - projects each element.
func (e *Evaluator) evaluateSelect(input types.Collection, projection grammar.IExpressionContext) interface{} {
	result := types.Collection{}

	// Check collection size limit
	if err := e.ctx.CheckCollectionSize(input); err != nil {
		return err
	}

	for i, item := range input {
		// Check for cancellation periodically
		if i%100 == 0 {
			if err := e.ctx.CheckCancellation(); err != nil {
				return err
			}
		}

		// Set $this to current item
		oldThis := e.ctx.this
		oldIndex := e.ctx.index
		oldPath := e.ctx.path
		endScope := e.ctx.enterIterationScope()
		e.ctx.this = types.Collection{item}
		e.ctx.index = i

		// Evaluate the projection
		projResult := e.Visit(projection)

		// Restore context
		e.ctx.this = oldThis
		e.ctx.index = oldIndex
		e.ctx.path = oldPath
		endScope()

		if err, ok := projResult.(error); ok {
			return err
		}

		// Add projection result to output
		if col, ok := projResult.(types.Collection); ok {
			result = append(result, col...)

			// Check if result is getting too large
			if err := e.ctx.CheckCollectionSize(result); err != nil {
				return err
			}
		}
	}

	return result
}

// evaluateAggregate evaluates aggregate(aggregator [, init]).
// Iterates over the collection, maintaining $total across iterations.
// Per FHIRPath spec §5.4.1: $this is the current element, $index is the 0-based index,
// and $total accumulates the result starting from init (or empty if not provided).
func (e *Evaluator) evaluateAggregate(input types.Collection, argExprs []grammar.IExpressionContext) interface{} {
	aggregator := argExprs[0]

	// Evaluate optional init value
	var total types.Collection
	if len(argExprs) > 1 {
		initResult := e.Visit(argExprs[1])
		if err, ok := initResult.(error); ok {
			return err
		}
		if col, ok := initResult.(types.Collection); ok {
			total = col
		}
	}
	if total == nil {
		total = types.Collection{}
	}

	// Save and restore context
	oldThis := e.ctx.this
	oldIndex := e.ctx.index
	oldTotal := e.ctx.total
	oldPath := e.ctx.path
	defer func() {
		e.ctx.this = oldThis
		e.ctx.index = oldIndex
		e.ctx.total = oldTotal
		e.ctx.path = oldPath
	}()

	for i, item := range input {
		// Check for cancellation periodically
		if i%100 == 0 {
			if err := e.ctx.CheckCancellation(); err != nil {
				return err
			}
		}

		// Set $this, $index, and $total for each iteration
		endScope := e.ctx.enterIterationScope()
		e.ctx.this = types.Collection{item}
		e.ctx.index = i
		if len(total) > 0 {
			e.ctx.total = total[0]
		} else {
			e.ctx.total = nil
		}

		// Evaluate the aggregator expression
		result := e.Visit(aggregator)
		endScope()
		if err, ok := result.(error); ok {
			return err
		}

		// Update $total with the result
		if col, ok := result.(types.Collection); ok {
			total = col
		}
	}

	return total
}

// evaluateIsFunction evaluates is() function - checks if input is of specified type.
// This handles is(Type) where Type is an identifier like Composition, Patient, etc.
func (e *Evaluator) evaluateIsFunction(input types.Collection, typeExpr grammar.IExpressionContext) interface{} {
	// Empty input returns empty
	if input.Empty() {
		return types.Collection{}
	}

	// is() requires singleton input
	if len(input) != 1 {
		return SingletonError(len(input))
	}

	// Extract the type name from the expression
	typeName := e.extractTypeNameFromExpr(typeExpr)
	if typeName == "" {
		return InvalidArgumentsError("is", 1, 0)
	}

	// Get actual type - Type() already returns resourceType for ObjectValue
	actualType := input[0].Type()

	matches := TypeMatchesWithModel(actualType, typeName, e.ctx.model)
	return types.Collection{types.NewBoolean(matches)}
}

// evaluateAsFunction evaluates as() function - casts input to specified type.
// Returns elements that match the type, empty otherwise.
// Per FHIRPath spec and HL7 validator behavior, as() works on collections
// by filtering/projecting elements that match the target type.
func (e *Evaluator) evaluateAsFunction(input types.Collection, typeExpr grammar.IExpressionContext) interface{} {
	// Empty input returns empty
	if input.Empty() {
		return types.Collection{}
	}

	// Extract the type name from the expression
	typeName := e.extractTypeNameFromExpr(typeExpr)
	if typeName == "" {
		return InvalidArgumentsError("as", 1, 0)
	}

	if err := e.checkAsSingleton(input); err != nil {
		return err
	}
	return e.castCollection(input, typeName)
}

// checkAsSingleton enforces the rule that as() takes a single item, which
// applies from R5 on.
//
// Before R5 it filters instead, and that is not a lenient reading: FHIR's own
// dom-3 invariant is written as %resource.descendants().as(canonical), which
// raises an error under the rule. The reference validator resolves it the same
// way, disabling the rule below R5.
func (e *Evaluator) checkAsSingleton(input types.Collection) error {
	if len(input) > 1 && e.ctx.enforcesR5Rules() {
		return SingletonError(len(input))
	}
	return nil
}

// castCollection keeps the items of a collection that are of the given type.
func (e *Evaluator) castCollection(input types.Collection, typeName string) types.Collection {
	result := types.Collection{}
	for _, item := range input {
		actualType := item.Type()
		if obj, ok := item.(*types.ObjectValue); ok {
			actualType = obj.Type()
		}
		if castMatches(item, actualType, typeName, e.ctx.model) {
			result = append(result, item)
		}
	}
	return result
}

// isTypeArg checks if the given argument index is marked as a type specifier argument.
func isTypeArg(typeArgs []int, index int) bool {
	for _, ta := range typeArgs {
		if ta == index {
			return true
		}
	}
	return false
}

// extractTypeNameFromExpr extracts a type name from a FHIRPath expression.
// Handles identifiers like Composition, Patient, and qualified names like FHIR.Patient.
func (e *Evaluator) extractTypeNameFromExpr(expr grammar.IExpressionContext) string {
	// The text of the expression, which for a type specifier is the identifier
	// or the qualified name as written
	return stripDelimiters(expr.GetText())
}

// stripDelimiters removes the backticks around a delimited identifier, in each
// part of a qualified name.
//
// The grammar admits DELIMITEDIDENTIFIER wherever it admits IDENTIFIER, which is
// what lets a type whose name collides with a keyword be written at all. The
// backticks are how the name is escaped, not part of it: FHIR.`Patient` names
// the same type as FHIR.Patient.
func stripDelimiters(name string) string {
	if !strings.Contains(name, "`") {
		return name
	}

	parts := strings.Split(name, ".")
	for i, part := range parts {
		parts[i] = strings.Trim(part, "`")
	}
	return strings.Join(parts, ".")
}

// evaluateOfType evaluates ofType() function - filters collection by type.
// Unlike is()/as() which require singleton, ofType() works on collections.
func (e *Evaluator) evaluateOfType(input types.Collection, typeExpr grammar.IExpressionContext) interface{} {
	// Empty input returns empty
	if input.Empty() {
		return types.Collection{}
	}

	// Extract the type name from the expression
	typeName := e.extractTypeNameFromExpr(typeExpr)
	if typeName == "" {
		return InvalidArgumentsError("ofType", 1, 0)
	}

	result := types.Collection{}
	for _, item := range input {
		actualType := item.Type()

		// For ObjectValue, also check if it's a FHIR type matching the request
		if obj, ok := item.(*types.ObjectValue); ok {
			// Try to get more specific type information
			actualType = obj.Type()
		}

		if castMatches(item, actualType, typeName, e.ctx.model) {
			result = append(result, item)
		}
	}

	return result
}

// evaluateRepeat applies a projection transitively: the projection runs over the
// input, its results are collected, and the projection then runs over those
// results, until a round produces nothing to carry forward.
//
// dedupe distinguishes the two functions the specification defines over this
// machinery. repeat() adds an item only when the output does not already hold an
// equal one — "as long as the projection yields new items (as determined by the
// equals (=) operator)" — which is also what terminates it on cyclic data.
// repeatAll() keeps duplicates, so only the absence of new results stops it;
// the configured collection limit bounds the damage when the data cycles.
func (e *Evaluator) evaluateRepeat(input types.Collection, projection grammar.IExpressionContext, dedupe bool) interface{} {
	result := types.Collection{}
	current := input

	for round := 0; len(current) > 0; round++ {
		if err := e.ctx.CheckCancellation(); err != nil {
			return err
		}

		next := types.Collection{}
		for i, item := range current {
			oldThis := e.ctx.this
			oldIndex := e.ctx.index
			oldPath := e.ctx.path
			endScope := e.ctx.enterIterationScope()
			e.ctx.this = types.Collection{item}
			// $index is undefined while a function iterates over its own output,
			// so it keeps the position within the current round
			e.ctx.index = i

			projected := e.Visit(projection)

			e.ctx.this = oldThis
			e.ctx.index = oldIndex
			e.ctx.path = oldPath
			endScope()

			if err, ok := projected.(error); ok {
				return err
			}

			produced, ok := projected.(types.Collection)
			if !ok {
				continue
			}

			for _, candidate := range produced {
				if dedupe && containsEqual(result, candidate) {
					continue
				}
				result = append(result, candidate)
				next = append(next, candidate)
			}
		}

		if err := e.ctx.CheckCollectionSize(result); err != nil {
			return err
		}

		current = next
	}

	return result
}

// containsEqual reports whether the collection already holds an item equal to
// the candidate, under the equality the repeat() definition names.
func containsEqual(collection types.Collection, candidate types.Value) bool {
	for _, existing := range collection {
		if existing.Equal(candidate) {
			return true
		}
	}
	return false
}

// evaluateDefineVariable binds a name for the remainder of the expression and
// passes its input through unchanged.
//
// Signature: defineVariable(name [, projection]). The value is the projection
// when given, otherwise the input collection itself. Either way the output is
// the input, so the function is transparent to the chain it sits in — it is the
// one function that exists for its effect on the context rather than its result.
func (e *Evaluator) evaluateDefineVariable(input types.Collection, argExprs []grammar.IExpressionContext) interface{} {
	nameResult := e.Visit(argExprs[0])
	if err, ok := nameResult.(error); ok {
		return err
	}

	nameCol, ok := nameResult.(types.Collection)
	if !ok || len(nameCol) != 1 {
		return NewEvalError(ErrInvalidArguments, "defineVariable() requires a single string name")
	}
	name, ok := nameCol[0].(types.String)
	if !ok {
		return NewEvalError(ErrInvalidArguments, "defineVariable() requires a string name, got %s", nameCol[0].Type())
	}

	value := input
	if len(argExprs) > 1 {
		projection := e.Visit(argExprs[1])
		if err, ok := projection.(error); ok {
			return err
		}
		if col, ok := projection.(types.Collection); ok {
			value = col
		} else {
			value = types.Collection{}
		}
	}

	if err := e.ctx.DefineVariable(name.Value(), value); err != nil {
		return err
	}

	return input
}

// evaluateCoalesce returns the first argument that evaluates to a non-empty
// collection, leaving the remaining arguments unevaluated.
//
// FHIRPath 3.0.0 requires this short-circuit explicitly: "arguments after the
// first non-empty argument are not evaluated", on the same grounds as iif.
func (e *Evaluator) evaluateCoalesce(argExprs []grammar.IExpressionContext) interface{} {
	for _, argExpr := range argExprs {
		result := e.Visit(argExpr)
		if err, ok := result.(error); ok {
			return err
		}
		if coll, ok := result.(types.Collection); ok && !coll.Empty() {
			return coll
		}
	}

	return types.Collection{}
}

// evaluateIif evaluates the iif() function with lazy evaluation.
// Only the matching branch is evaluated, preventing errors from the other branch.
// Signature: iif(criterion, true-result [, otherwise-result])
func (e *Evaluator) evaluateIif(_ types.Collection, argExprs []grammar.IExpressionContext) interface{} {
	if len(argExprs) < 2 {
		return InvalidArgumentsError("iif", 2, len(argExprs))
	}

	// Evaluate the criterion (first argument)
	criterionResult := e.Visit(argExprs[0])
	if err, ok := criterionResult.(error); ok {
		return err
	}

	// Convert criterion to boolean
	criterion := false
	if coll, ok := criterionResult.(types.Collection); ok {
		criterion, _ = coll.SingletonBoolean()
	}

	// Lazily evaluate only the matching branch
	if criterion {
		// Evaluate and return true-result (second argument)
		result := e.Visit(argExprs[1])
		if err, ok := result.(error); ok {
			return err
		}
		if coll, ok := result.(types.Collection); ok {
			return coll
		}
		return types.Collection{}
	}

	// Evaluate and return otherwise-result (third argument) if provided
	if len(argExprs) > 2 {
		result := e.Visit(argExprs[2])
		if err, ok := result.(error); ok {
			return err
		}
		if coll, ok := result.(types.Collection); ok {
			return coll
		}
	}

	return types.Collection{}
}

// VisitThisInvocation visits $this.
func (e *Evaluator) VisitThisInvocation(ctx *grammar.ThisInvocationContext) interface{} {
	return e.ctx.This()
}

// VisitIndexInvocation visits $index.
func (e *Evaluator) VisitIndexInvocation(ctx *grammar.IndexInvocationContext) interface{} {
	return types.Collection{types.NewInteger(int64(e.ctx.index))}
}

// VisitTotalInvocation visits $total.
func (e *Evaluator) VisitTotalInvocation(ctx *grammar.TotalInvocationContext) interface{} {
	if e.ctx.total != nil {
		return types.Collection{e.ctx.total}
	}
	return types.Collection{}
}

// Expression visitors

// VisitInvocationExpression visits expr.invocation.
func (e *Evaluator) VisitInvocationExpression(ctx *grammar.InvocationExpressionContext) interface{} {
	// Evaluate the base expression
	base := e.Visit(ctx.Expression())
	if err, ok := base.(error); ok {
		return err
	}
	baseCol := base.(types.Collection)

	// Save current this and path, set new this
	oldThis := e.ctx.this
	oldPath := e.ctx.path
	oldOuter := e.ctx.outer

	// A function's input is what precedes the dot, but its arguments are not
	// navigated from there: in name.given.combine(name.family), family belongs
	// to name, not to given. The specification's own conformance suite fixes
	// this — it gives combine(name.family) and combine($this.name.family) the
	// same expected result — so the scope in force before the dot is kept for
	// the arguments to be evaluated in.
	e.ctx.outer = oldThis
	e.ctx.this = baseCol
	defer func() {
		e.ctx.this = oldThis
		e.ctx.path = oldPath
		e.ctx.outer = oldOuter
	}()

	// Evaluate the invocation
	return e.Visit(ctx.Invocation())
}

// VisitIndexerExpression visits expr[index].
func (e *Evaluator) VisitIndexerExpression(ctx *grammar.IndexerExpressionContext) interface{} {
	base := e.Visit(ctx.Expression(0))
	if err, ok := base.(error); ok {
		return err
	}
	baseCol := base.(types.Collection)

	index := e.Visit(ctx.Expression(1))
	if err, ok := index.(error); ok {
		return err
	}
	indexCol := index.(types.Collection)

	if indexCol.Empty() {
		return types.Collection{}
	}

	// Get index as integer
	idx, ok := indexCol[0].(types.Integer)
	if !ok {
		return TypeError("Integer", indexCol[0].Type(), "indexer")
	}

	i := int(idx.Value())
	if i < 0 || i >= len(baseCol) {
		return types.Collection{}
	}

	return types.Collection{baseCol[i]}
}

// VisitPolarityExpression visits +expr or -expr.
func (e *Evaluator) VisitPolarityExpression(ctx *grammar.PolarityExpressionContext) interface{} {
	result := e.Visit(ctx.Expression())
	if err, ok := result.(error); ok {
		return err
	}
	col := result.(types.Collection)

	if col.Empty() {
		return col
	}
	if len(col) != 1 {
		return SingletonError(len(col))
	}

	// Check if it's negation
	if ctx.GetChild(0).(antlr.TerminalNode).GetText() == "-" {
		negated, err := Negate(col[0])
		if err != nil {
			return err
		}
		return types.Collection{negated}
	}

	return col
}

// VisitMultiplicativeExpression visits expr * expr, expr / expr, etc.
func (e *Evaluator) VisitMultiplicativeExpression(ctx *grammar.MultiplicativeExpressionContext) interface{} {
	left := e.Visit(ctx.Expression(0))
	if err, ok := left.(error); ok {
		return err
	}
	leftCol := left.(types.Collection)

	right := e.Visit(ctx.Expression(1))
	if err, ok := right.(error); ok {
		return err
	}
	rightCol := right.(types.Collection)

	// Empty propagation
	if leftCol.Empty() || rightCol.Empty() {
		return types.Collection{}
	}

	// Singleton check
	if len(leftCol) != 1 || len(rightCol) != 1 {
		return SingletonError(len(leftCol) + len(rightCol))
	}

	op := ctx.GetChild(1).(antlr.TerminalNode).GetText()

	var result types.Value
	var err error

	switch op {
	case "*":
		result, err = Multiply(leftCol[0], rightCol[0])
	case "/":
		result, err = Divide(leftCol[0], rightCol[0])
	case "div":
		result, err = IntegerDivide(leftCol[0], rightCol[0])
	case "mod":
		result, err = Modulo(leftCol[0], rightCol[0])
	}

	if err != nil {
		// A zero divisor is not an error: "12 / 0 // empty ({ })", and the same
		// for div and mod
		if errors.Is(err, ErrDivideByZero) {
			return types.Collection{}
		}
		return err
	}
	return types.Collection{result}
}

// VisitAdditiveExpression visits expr + expr, expr - expr, expr & expr.
func (e *Evaluator) VisitAdditiveExpression(ctx *grammar.AdditiveExpressionContext) interface{} {
	left := e.Visit(ctx.Expression(0))
	if err, ok := left.(error); ok {
		return err
	}
	leftCol := left.(types.Collection)

	right := e.Visit(ctx.Expression(1))
	if err, ok := right.(error); ok {
		return err
	}
	rightCol := right.(types.Collection)

	op := ctx.GetChild(1).(antlr.TerminalNode).GetText()

	// String concatenation with & handles empty as empty string
	if op == "&" {
		result, err := Concatenate(leftCol, rightCol)
		if err != nil {
			return err
		}
		return result
	}

	// Empty propagation for + and -
	if leftCol.Empty() || rightCol.Empty() {
		return types.Collection{}
	}

	// Singleton check
	if len(leftCol) != 1 || len(rightCol) != 1 {
		return SingletonError(len(leftCol) + len(rightCol))
	}

	var result types.Value
	var err error

	switch op {
	case "+":
		result, err = Add(leftCol[0], rightCol[0])
	case "-":
		result, err = Subtract(leftCol[0], rightCol[0])
	}

	if err != nil {
		// Quantities with incommensurable units evaluate to empty, per spec
		if errors.Is(err, types.ErrIncompatibleUnits) {
			return types.Collection{}
		}
		return err
	}
	return types.Collection{result}
}

// VisitUnionExpression visits expr | expr.
func (e *Evaluator) VisitUnionExpression(ctx *grammar.UnionExpressionContext) interface{} {
	left := e.Visit(ctx.Expression(0))
	if err, ok := left.(error); ok {
		return err
	}
	leftCol := left.(types.Collection)

	right := e.Visit(ctx.Expression(1))
	if err, ok := right.(error); ok {
		return err
	}
	rightCol := right.(types.Collection)

	return Union(leftCol, rightCol)
}

// VisitInequalityExpression visits comparison expressions.
func (e *Evaluator) VisitInequalityExpression(ctx *grammar.InequalityExpressionContext) interface{} {
	left := e.Visit(ctx.Expression(0))
	if err, ok := left.(error); ok {
		return err
	}
	leftCol := left.(types.Collection)

	right := e.Visit(ctx.Expression(1))
	if err, ok := right.(error); ok {
		return err
	}
	rightCol := right.(types.Collection)

	// Empty propagation
	if leftCol.Empty() || rightCol.Empty() {
		return types.Collection{}
	}

	// Singleton check
	if len(leftCol) != 1 || len(rightCol) != 1 {
		return SingletonError(len(leftCol) + len(rightCol))
	}

	op := ctx.GetChild(1).(antlr.TerminalNode).GetText()

	var result types.Collection
	var err error

	switch op {
	case "<":
		result, err = LessThan(leftCol[0], rightCol[0])
	case "<=":
		result, err = LessOrEqual(leftCol[0], rightCol[0])
	case ">":
		result, err = GreaterThan(leftCol[0], rightCol[0])
	case ">=":
		result, err = GreaterOrEqual(leftCol[0], rightCol[0])
	default:
		return types.Collection{}
	}

	if err != nil {
		return err
	}
	return result
}

// VisitEqualityExpression visits equality expressions.
func (e *Evaluator) VisitEqualityExpression(ctx *grammar.EqualityExpressionContext) interface{} {
	left := e.Visit(ctx.Expression(0))
	if err, ok := left.(error); ok {
		return err
	}
	leftCol := left.(types.Collection)

	right := e.Visit(ctx.Expression(1))
	if err, ok := right.(error); ok {
		return err
	}
	rightCol := right.(types.Collection)

	op := ctx.GetChild(1).(antlr.TerminalNode).GetText()

	switch op {
	case "=":
		return Equal(leftCol, rightCol)
	case "!=":
		return NotEqual(leftCol, rightCol)
	case "~":
		return Equivalent(leftCol, rightCol)
	case "!~":
		return NotEquivalent(leftCol, rightCol)
	}

	return types.Collection{}
}

// VisitMembershipExpression visits 'in' and 'contains' expressions.
func (e *Evaluator) VisitMembershipExpression(ctx *grammar.MembershipExpressionContext) interface{} {
	left := e.Visit(ctx.Expression(0))
	if err, ok := left.(error); ok {
		return err
	}
	leftCol := left.(types.Collection)

	right := e.Visit(ctx.Expression(1))
	if err, ok := right.(error); ok {
		return err
	}
	rightCol := right.(types.Collection)

	op := ctx.GetChild(1).(antlr.TerminalNode).GetText()

	switch op {
	case "in":
		return In(leftCol, rightCol)
	case "contains":
		return Contains(leftCol, rightCol)
	}

	return types.Collection{}
}

// VisitAndExpression visits expr and expr.
func (e *Evaluator) VisitAndExpression(ctx *grammar.AndExpressionContext) interface{} {
	left := e.Visit(ctx.Expression(0))
	if err, ok := left.(error); ok {
		return err
	}
	leftCol := left.(types.Collection)

	right := e.Visit(ctx.Expression(1))
	if err, ok := right.(error); ok {
		return err
	}
	rightCol := right.(types.Collection)

	return And(leftCol, rightCol)
}

// VisitOrExpression visits expr or expr, expr xor expr.
func (e *Evaluator) VisitOrExpression(ctx *grammar.OrExpressionContext) interface{} {
	left := e.Visit(ctx.Expression(0))
	if err, ok := left.(error); ok {
		return err
	}
	leftCol := left.(types.Collection)

	right := e.Visit(ctx.Expression(1))
	if err, ok := right.(error); ok {
		return err
	}
	rightCol := right.(types.Collection)

	op := ctx.GetChild(1).(antlr.TerminalNode).GetText()

	switch op {
	case "or":
		return Or(leftCol, rightCol)
	case "xor":
		return Xor(leftCol, rightCol)
	}

	return types.Collection{}
}

// VisitImpliesExpression visits expr implies expr.
func (e *Evaluator) VisitImpliesExpression(ctx *grammar.ImpliesExpressionContext) interface{} {
	left := e.Visit(ctx.Expression(0))
	if err, ok := left.(error); ok {
		return err
	}
	leftCol := left.(types.Collection)

	right := e.Visit(ctx.Expression(1))
	if err, ok := right.(error); ok {
		return err
	}
	rightCol := right.(types.Collection)

	return Implies(leftCol, rightCol)
}

// VisitTypeExpression visits 'is' and 'as' expressions.
func (e *Evaluator) VisitTypeExpression(ctx *grammar.TypeExpressionContext) interface{} {
	left := e.Visit(ctx.Expression())
	if err, ok := left.(error); ok {
		return err
	}
	leftCol := left.(types.Collection)

	typeName := ctx.TypeSpecifier().GetText()
	op := ctx.GetChild(1).(antlr.TerminalNode).GetText()

	if leftCol.Empty() {
		return types.Collection{}
	}

	switch op {
	case "is":
		// is requires singleton input — returns a boolean
		if len(leftCol) != 1 {
			return SingletonError(len(leftCol))
		}
		actualType := leftCol[0].Type()
		return types.Collection{types.NewBoolean(TypeMatchesWithModel(actualType, typeName, e.ctx.model))}
	case "as":
		if err := e.checkAsSingleton(leftCol); err != nil {
			return err
		}
		return e.castCollection(leftCol, typeName)
	}

	return types.Collection{}
}

// The FHIR types this package tests against by name, rather than through the
// model. Resource and DomainResource sit at the root of the resource hierarchy,
// and the other three are the resources that skip DomainResource.
const (
	typeResource       = "Resource"
	typeDomainResource = "DomainResource"
	typeBundle         = "Bundle"
	typeBinary         = "Binary"
	typeParameters     = "Parameters"
)

// nonDomainResources contains FHIR resources that inherit directly from Resource,
// not from DomainResource. All other resources inherit from DomainResource.
var nonDomainResources = map[string]bool{
	typeBundle:     true,
	typeBinary:     true,
	typeParameters: true,
}

// fhirPathSpecMap contains FHIRPath spec-stable primitive type mappings.
// These are defined by the FHIRPath specification and are stable across FHIR versions.
// Keys are lowercase FHIR type names, values are PascalCase FHIRPath type names.
var fhirPathSpecMap = map[string]string{
	"boolean":      types.TypeNameBoolean,
	"string":       types.TypeNameString,
	"integer":      types.TypeNameInteger,
	"decimal":      types.TypeNameDecimal,
	"date":         types.TypeNameDate,
	"datetime":     types.TypeNameDateTime,
	"time":         types.TypeNameTime,
	"instant":      types.TypeNameDateTime,
	"uri":          types.TypeNameString,
	"url":          types.TypeNameString,
	"canonical":    types.TypeNameString,
	"base64binary": types.TypeNameString,
	"code":         types.TypeNameString,
	"id":           types.TypeNameString,
	"markdown":     types.TypeNameString,
	"oid":          types.TypeNameString,
	"uuid":         types.TypeNameString,
	"positiveint":  types.TypeNameInteger,
	"unsignedint":  types.TypeNameInteger,
	"integer64":    types.TypeNameInteger,
	"quantity":     types.TypeNameQuantity,
	"money":        types.TypeNameQuantity,
}

// fhirVersionSpecificMap contains FHIR version-specific profiled type mappings.
// These types are profiled Quantity subtypes that may vary across FHIR versions.
// When a Model is available, these mappings are skipped in favor of model.IsSubtype().
var fhirVersionSpecificMap = map[string]string{
	"simplequantity": types.TypeNameQuantity,
	"age":            types.TypeNameQuantity,
	"count":          types.TypeNameQuantity,
	"distance":       types.TypeNameQuantity,
	"duration":       types.TypeNameQuantity,
}

// IsDomainResource returns true if the given resource type inherits from DomainResource.
// Bundle, Binary, and Parameters inherit directly from Resource, not DomainResource.
func IsDomainResource(resourceType string) bool {
	return !nonDomainResources[resourceType]
}

// IsSubtypeOf checks if actualType is a subtype of (or equal to) baseType.
// This handles the FHIR type hierarchy:
//
//	Resource
//	  └── DomainResource
//	        ├── Patient
//	        ├── Observation
//	        └── ... (most resources)
//	  └── Bundle, Binary, Parameters (directly inherit from Resource)
func IsSubtypeOf(actualType, baseType string) bool {
	// Direct match
	if actualType == baseType {
		return true
	}

	// Case-insensitive direct match
	if strings.EqualFold(actualType, baseType) {
		return true
	}

	// Check Resource base type - all resources inherit from Resource
	if baseType == typeResource || strings.EqualFold(baseType, "resource") {
		// Any non-empty type that looks like a resource type matches Resource
		// Resource types are PascalCase and don't include primitives
		return isPossibleResourceType(actualType)
	}

	// Check DomainResource base type
	if baseType == typeDomainResource || strings.EqualFold(baseType, "domainresource") {
		// Most resources inherit from DomainResource, except Bundle, Binary, Parameters
		return isPossibleResourceType(actualType) && IsDomainResource(actualType)
	}

	return false
}

// isPossibleResourceType checks if the type looks like a FHIR resource type.
// Resource types are PascalCase and are not primitive types.
func isPossibleResourceType(typeName string) bool {
	if typeName == "" {
		return false
	}

	// A System type is never a resource
	if types.IsSystemTypeName(typeName) || typeName == "Object" {
		return false
	}

	// Resource types start with uppercase
	return typeName[0] >= 'A' && typeName[0] <= 'Z'
}

// isResourceType returns true if typeName is a known FHIR resource type.
// Uses model.IsResource() when available, otherwise falls back to the
// isPossibleResourceType heuristic.
func isResourceType(typeName string, model Model) bool {
	if model != nil {
		return model.IsResource(typeName)
	}
	return isPossibleResourceType(typeName)
}

// IsSubtypeOfWithModel checks subtype relationship using the model if available,
// falling back to the built-in heuristic when model is nil.
// When a model is present, it is authoritative — the heuristic fallback is skipped.
func IsSubtypeOfWithModel(actualType, baseType string, model Model) bool {
	if actualType == baseType || strings.EqualFold(actualType, baseType) {
		return true
	}
	if model != nil {
		return model.IsSubtype(actualType, baseType)
	}
	return IsSubtypeOf(actualType, baseType)
}

// castMatches reports whether a value is of the requested type for the purpose
// of as() and ofType(), which is stricter than is().
//
// FHIR states that "all primitives are considered to be independent types (so
// markdown is not a subclass of string)", so casting a primitive requires the
// type it was declared with: Patient.gender is a code, and gender.as(string)
// yields empty even though is(string) is true — is() walks the type hierarchy
// the StructureDefinitions describe, while a cast does not.
//
// Structured values keep hierarchy matching, so ofType(Quantity) still selects
// an Age, and ofType(HumanName) a name.
func castMatches(item types.Value, actualType, typeName string, model Model) bool {
	if _, isObject := item.(*types.ObjectValue); isObject {
		return TypeMatchesWithModel(actualType, typeName, model)
	}

	// Only a value that kept the type FHIR declared for it can be cast exactly.
	// One reported as a system type — a String that no model narrowed to code or
	// uri — carries no such claim, so it keeps the permissive match.
	if types.IsSystemTypeName(actualType) {
		return TypeMatchesWithModel(actualType, typeName, model)
	}
	return primitiveTypeMatches(actualType, typeName)
}

// primitiveTypeMatches compares a primitive's declared type against a requested
// one, ignoring case and any model namespace, and without consulting the
// hierarchy.
func primitiveTypeMatches(actualType, typeName string) bool {
	requested := typeName
	if index := strings.LastIndex(requested, "."); index >= 0 {
		// Strip a FHIR. or System. qualifier: the type still has to match
		requested = requested[index+1:]
	}
	return strings.EqualFold(actualType, requested)
}

// isFHIRPrimitiveName reports whether a type name is a FHIR primitive, which the
// specification writes in lower camel case — boolean, dateTime, code — as
// against the capitalized names of complex types and of the FHIRPath system
// types.
func isFHIRPrimitiveName(typeName string) bool {
	if typeName == "" {
		return false
	}
	first := typeName[0]
	return first >= 'a' && first <= 'z'
}

// matchesSystemType reports whether a value is the named System type.
//
// Only the types FHIRPath declares itself live in that namespace, so
// System.Patient names nothing; and a value carrying a FHIR type is not its
// system counterpart, so a FHIR.boolean is not a System.Boolean.
func matchesSystemType(actualType, systemType string) bool {
	if !types.IsSystemTypeName(systemType) || isFHIRPrimitiveName(actualType) {
		return false
	}
	return strings.EqualFold(actualType, systemType)
}

// TypeMatchesWithModel checks type matching using the model if available,
// falling back to the built-in TypeMatches when model is nil.
// When a model is present, it is authoritative for type hierarchy — only
// spec-stable FHIRPath type mappings are applied, version-specific heuristics are skipped.
func TypeMatchesWithModel(actualType, typeName string, model Model) bool {
	if actualType == typeName {
		return true
	}

	// A FHIR primitive is not the system type of the same shape: Patient.active
	// is a FHIR.boolean, so is(Boolean) is false while is(boolean) is true. The
	// names differ only in case, so this is settled before any case-insensitive
	// comparison.
	//
	// It applies to primitives alone, which FHIR names in lower camel case. A
	// complex type such as Quantity is spelled the same in both namespaces, and
	// an Age is still a Quantity.
	if types.IsSystemTypeName(typeName) && isFHIRPrimitiveName(actualType) {
		return false
	}

	actualLower := strings.ToLower(actualType)
	typeNameLower := strings.ToLower(typeName)

	if actualLower == typeNameLower {
		return true
	}

	if model != nil {
		// Model is authoritative for type hierarchy
		if model.IsSubtype(actualType, typeName) {
			return true
		}
		// Only use spec-stable mappings (skip version-specific profiled types)
		return typeMatchesSpecMaps(actualType, typeName, actualLower, typeNameLower, fhirPathSpecMap)
	}

	// No model: full heuristic (backward compatible)
	return TypeMatches(actualType, typeName)
}

// typeMatchesSpecMaps checks if actualType matches typeName using the provided
// FHIR-to-FHIRPath type mapping and namespace handling (System.*, FHIR.*).
func typeMatchesSpecMaps(actualType, typeName, actualLower, typeNameLower string, specMap map[string]string) bool {
	// A FHIR primitive converts implicitly to its system counterpart, so a
	// FHIR.code is a System.String.
	if fhirPathType, ok := specMap[actualLower]; ok {
		if fhirPathType == typeName || strings.EqualFold(fhirPathType, typeName) {
			return true
		}
	}

	// Several FHIR primitives share one system type, so a System.String might be
	// a code, a uri or an id. Answering yes is the useful guess while the engine
	// does not carry the declared type on every primitive value — it does so
	// only for the string-like ones. Until it does, ValueSet.version reports
	// is(code) as true, which the official suite marks as wrong.
	if fhirPathType, ok := specMap[typeNameLower]; ok {
		if actualType == fhirPathType {
			return true
		}
	}

	// System type namespace handling (System.Boolean, System.String, etc.)
	if strings.HasPrefix(typeNameLower, "system.") {
		return matchesSystemType(actualType, typeName[7:])
	}

	// FHIR namespace handling (FHIR.Patient, etc.)
	if strings.HasPrefix(typeNameLower, "fhir.") {
		fhirType := typeName[5:] // Remove "FHIR." prefix
		if strings.EqualFold(actualType, fhirType) {
			return true
		}
	}

	return false
}

// TypeMatches checks if actualType matches the requested typeName.
// Handles case-insensitive comparison and FHIR type aliases.
// This function is exported for use by the is() function implementation.
func TypeMatches(actualType, typeName string) bool {
	// Direct match
	if actualType == typeName {
		return true
	}

	// Normalize to lowercase for comparison
	actualLower := strings.ToLower(actualType)
	typeNameLower := strings.ToLower(typeName)

	// Case-insensitive match
	if actualLower == typeNameLower {
		return true
	}

	// Check FHIR base type inheritance (Resource, DomainResource)
	if IsSubtypeOf(actualType, typeName) {
		return true
	}

	// Check spec-stable FHIR-to-FHIRPath type mappings + namespace handling
	if typeMatchesSpecMaps(actualType, typeName, actualLower, typeNameLower, fhirPathSpecMap) {
		return true
	}

	// Check FHIR version-specific type mappings (profiled Quantity subtypes)
	if typeMatchesSpecMaps(actualType, typeName, actualLower, typeNameLower, fhirVersionSpecificMap) {
		return true
	}

	return false
}

// Helper functions

// polymorphicTypeSuffixes contains all FHIR type suffixes for polymorphic elements (value[x] pattern).
// These are used to resolve element names like "value" to "valueQuantity", "valueString", etc.
var polymorphicTypeSuffixes = []string{
	// Primitive types
	"Boolean", "Integer", "Integer64", "Decimal", "String", "Code", "Id", "Uri", "Url", "Canonical",
	"Base64Binary", "Instant", "Date", "DateTime", "Time", "Oid", "Uuid", "Markdown", "PositiveInt", "UnsignedInt",
	// Complex types
	types.TypeNameQuantity, "CodeableConcept", "Coding", "Range", "Period", "Ratio", "RatioRange",
	"Identifier", "Reference", "Attachment", "HumanName", "Address", "ContactPoint",
	"Timing", "Signature", "Annotation", "SampledData", "Age", "Distance", "Duration",
	"Count", "Money", "MoneyQuantity", "SimpleQuantity",
	// Special types
	"Meta", "Dosage", "ContactDetail", "Contributor", "DataRequirement", "Expression",
	"ParameterDefinition", "RelatedArtifact", "TriggerDefinition", "UsageContext",
}

// buildElementPath constructs a FHIR element path from the object type and field name.
// For example, buildElementPath("Observation", "value") returns "Observation.value".
// If the model provides a contentReference redirect (e.g., "Questionnaire.item.item"
// → "Questionnaire.item"), the resolved path is returned instead.
func (e *Evaluator) buildElementPath(objType, name string) string {
	if objType == "" {
		return ""
	}
	path := objType + "." + name
	if m := e.ctx.model; m != nil {
		path = m.ResolvePath(path)
	}
	return path
}

// resolveElement determines the FHIR element path for a named member of obj and
// the type the model assigns it. It tries both ways a model indexes elements: a
// complex type or resource indexes its own ("Observation.subject",
// "Quantity.value"), while a backbone element only exists beneath the path it was
// reached by ("Observation.component.valueQuantity") — hence the current
// navigation path as the second candidate.
//
// fhirType is "" when there is no model or it knows neither form, in which case
// the caller falls back to untyped field access.
func (e *Evaluator) resolveElement(obj *types.ObjectValue, name string) (elementPath, fhirType string) {
	elementPath = e.buildElementPath(obj.Type(), name)

	m := e.ctx.model
	if m == nil {
		return elementPath, ""
	}

	if elementPath != "" {
		if t := m.TypeOf(elementPath); t != "" {
			return elementPath, t
		}
	}

	// Fall back to the accumulated navigation path, which is the only form that
	// resolves elements of anonymous backbone types.
	if e.ctx.path != "" && e.ctx.path != obj.Type() {
		if nested := e.buildElementPath(e.ctx.path, name); nested != "" {
			if t := m.TypeOf(nested); t != "" {
				return nested, t
			}
		}
	}

	return elementPath, ""
}

// navigateMember navigates to a member of objects in the collection.
// Supports FHIR polymorphic elements (value[x] pattern) by automatically
// resolving element names like "value" to their typed variants.
func (e *Evaluator) navigateMember(input types.Collection, name string) types.Collection {
	result := types.Collection{}

	for _, item := range input {
		obj, ok := item.(*types.ObjectValue)
		if !ok {
			continue
		}

		// Check if name matches resourceType (for FHIR resources).
		// Skip subtype check for lowercase names (field names like "name", "status")
		// since FHIR type names always start with uppercase.
		if name != "" && name[0] >= 'A' && name[0] <= 'Z' && IsSubtypeOfWithModel(obj.Type(), name, e.ctx.model) {
			// Entering a resource — set path to resource type
			e.ctx.path = obj.Type()
			result = append(result, obj)
			continue
		}

		// Build FHIR element path for model lookups
		elementPath, fhirType := e.resolveElement(obj, name)

		// Try direct field access first, using type-aware conversion when model is available
		var children types.Collection
		if fhirType != "" {
			children = obj.GetCollectionWithType(name, fhirType)
		}
		if len(children) == 0 {
			children = obj.GetCollection(name)
		}
		if len(children) > 0 {
			e.ctx.path = elementPath
			result = append(result, children...)
			continue
		}

		// If direct access failed, try polymorphic element resolution
		// This handles FHIR's value[x] pattern where "value" can resolve to
		// "valueQuantity", "valueString", "valueCodeableConcept", etc.
		polymorphicChildren := e.resolvePolymorphicField(obj, name, elementPath)
		if len(polymorphicChildren) > 0 {
			e.ctx.path = elementPath
		}
		result = append(result, polymorphicChildren...)
	}

	return result
}

// resolvePolymorphicField attempts to resolve a polymorphic FHIR element.
// For example, accessing "value" will search for "valueQuantity", "valueString", etc.
// When a Model is available and elementPath is non-empty, uses precise choice type
// suffixes from the model. Otherwise falls back to the brute-force suffix list.
func (e *Evaluator) resolvePolymorphicField(obj *types.ObjectValue, name, elementPath string) types.Collection {
	result := types.Collection{}

	// When a model is available, use precise choice type suffixes
	if m := e.ctx.model; m != nil && elementPath != "" {
		if suffixes := m.ChoiceTypes(elementPath); len(suffixes) > 0 {
			for _, suffix := range suffixes {
				fieldName := name + strings.ToUpper(suffix[:1]) + suffix[1:]
				children := obj.GetCollectionWithType(fieldName, suffix)
				if len(children) > 0 {
					result = append(result, children...)
					return result
				}
			}
			return result
		}
	}

	// Fallback: try each possible type suffix from the hardcoded list. Without a
	// model these are guesses at the field name, so they guide parsing but do
	// not become the value's declared type.
	for _, suffix := range polymorphicTypeSuffixes {
		fieldName := name + suffix
		children := obj.GetCollectionParsedAs(fieldName, suffix)
		if len(children) > 0 {
			result = append(result, children...)
			// Return on first match - polymorphic elements have only one variant
			return result
		}
	}

	return result
}

// unquoteString removes the surrounding quotes of a string literal and resolves
// its escape sequences, as defined by the FHIRPath specification's String
// section:
//
//	\'  \"  \`  \r  \n  \t  \f  \\  \uXXXX
//
// A backslash that begins anything else is dropped and the character kept
// verbatim, which the specification states explicitly: '\p' is 'p', '\3' is '3',
// and an incomplete '\u005' is 'u005'.
//
// The sequences are resolved in a single pass. Successive replacements would be
// wrong: rewriting \\ before \n turns the literal '\\n' — a backslash followed
// by the letter n — into a line feed.
func unquoteString(s string) string {
	if len(s) < 2 {
		return s
	}
	// Remove surrounding quotes
	s = s[1 : len(s)-1]

	if !strings.Contains(s, `\`) {
		return s
	}

	var b strings.Builder
	b.Grow(len(s))

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '\\' {
			b.WriteRune(runes[i])
			continue
		}
		if i+1 >= len(runes) {
			// A trailing backslash is dropped: '\' is the empty string
			break
		}

		i++
		switch runes[i] {
		case '\'', '"', '`', '\\':
			b.WriteRune(runes[i])
		case 'r':
			b.WriteRune('\r')
		case 'n':
			b.WriteRune('\n')
		case 't':
			b.WriteRune('\t')
		case 'f':
			b.WriteRune('\f')
		case 'u':
			if i+4 < len(runes) {
				// Four hex digits, so the escape denotes a 16-bit code unit;
				// parsing it at that width is what makes the rune conversion
				// exact rather than merely likely
				if code, err := strconv.ParseUint(string(runes[i+1:i+5]), 16, 16); err == nil {
					b.WriteRune(rune(code))
					i += 4
					continue
				}
			}
			// Not four hex digits: the backslash is dropped, 'u' remains
			b.WriteRune('u')
		default:
			// Not an escape sequence: drop the backslash, keep the character
			b.WriteRune(runes[i])
		}
	}

	return b.String()
}

// stripBackticks removes backtick delimiters from delimited identifiers.
// FHIRPath allows backticks for identifiers with special characters: `PID-1`
func stripBackticks(s string) string {
	if len(s) >= 2 && s[0] == '`' && s[len(s)-1] == '`' {
		return s[1 : len(s)-1]
	}
	return s
}
