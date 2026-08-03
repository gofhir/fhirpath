package fhirpath

import (
	"fmt"
	"strings"

	"github.com/gofhir/fhirpath/parser/grammar"
	"github.com/gofhir/fhirpath/types"
)

// Static analysis of an expression against a model, which answers a different
// question from evaluation.
//
// Evaluating Patient.name.given1 against a document yields empty, and that is
// the right answer to the question asked: the document has no such value. But
// given1 is not an element of HumanName in any document, so the expression is
// wrong however it is run — a typo in an invariant that will never fail, and
// never say why. The conformance suite calls this an invalid="semantic" case and
// distinguishes it from invalid="execution" for exactly this reason.
//
// The analysis is deliberately conservative. It reports an error only where the
// model is certain, and stops following a branch as soon as the type becomes
// unknown, because a false positive rejects an expression that works. Silence
// means "nothing provably wrong", not "correct".

// unknownType marks a value whose type the analysis could not determine, which
// is where checking stops rather than guesses.
const unknownType = ""

// AnalysisError reports an expression that cannot be valid against the model.
type AnalysisError struct {
	Expression string
	Reason     string
}

func (e *AnalysisError) Error() string {
	return fmt.Sprintf("%s: %s", e.Expression, e.Reason)
}

// Analyze checks the expression against a model, in the context of a resource
// or element type, and reports the first fault it can prove.
//
// contextType names the type the expression is evaluated against — "Patient"
// for a constraint on Patient. An empty contextType leaves the starting type
// unknown, which limits the analysis to the checks that do not depend on it.
//
// Returns nil when nothing is provably wrong. That is not a guarantee of
// correctness: a model cannot describe every function's result, and the
// analysis stops where it stops seeing.
func (e *Expression) Analyze(model Model, contextType string) error {
	if model == nil {
		return nil
	}

	a := &analyzer{model: model, source: e.source, path: contextType}
	_, err := a.expression(e.tree.Expression(), contextType)
	return err
}

type analyzer struct {
	model  Model
	source string

	// path is where navigation has reached, which is how a model indexes the
	// backbone elements: ValueSet.expansion.contains is an element of that
	// path, while BackboneElement — the type it nominally has — has no
	// elements of its own at all.
	path string
}

func (a *analyzer) fault(reason string, args ...interface{}) error {
	return &AnalysisError{Expression: a.source, Reason: fmt.Sprintf(reason, args...)}
}

// expression walks one node, returning the type its result carries.
func (a *analyzer) expression(node grammar.IExpressionContext, contextType string) (string, error) {
	switch expr := node.(type) {
	case *grammar.TermExpressionContext:
		return a.term(expr.Term(), contextType)

	case *grammar.InvocationExpressionContext:
		return a.invocationExpression(expr, contextType)

	case *grammar.IndexerExpressionContext:
		return a.indexerExpression(expr, contextType)

	case *grammar.TypeExpressionContext:
		// is and as: the operand is walked, and the result takes the named type
		if _, err := a.expression(expr.Expression(), contextType); err != nil {
			return unknownType, err
		}
		operator := expr.GetChild(1).(interface{ GetText() string }).GetText()
		if operator == "is" {
			return types.TypeNameBoolean, nil
		}

		// as narrows to the type it names, which is what the rest of the
		// expression then navigates from
		if specifier := expr.TypeSpecifier(); specifier != nil {
			return strings.Trim(strings.TrimPrefix(specifier.GetText(), "FHIR."), "`"), nil
		}
		return unknownType, nil
	}

	// Every other operator: walk the operands, and do not claim a result type
	for i := 0; ; i++ {
		child := childExpression(node, i)
		if child == nil {
			break
		}
		if _, err := a.expression(child, contextType); err != nil {
			return unknownType, err
		}
	}
	return unknownType, nil
}

// invocationExpression walks `expr.invocation`, where the left side supplies
// the type the invocation navigates from.
func (a *analyzer) invocationExpression(expr *grammar.InvocationExpressionContext, contextType string) (string, error) {
	base, err := a.expression(expr.Expression(), contextType)
	if err != nil {
		return unknownType, err
	}

	// A positional read of what precedes it, when that has no defined order
	if call, ok := expr.Invocation().(*grammar.FunctionInvocationContext); ok {
		reader := strings.Trim(call.Function().Identifier().GetText(), "`")
		if orderDependentFunctions[reader] {
			if err := a.checkOrderDependence(expr.Expression(), reader+"()"); err != nil {
				return unknownType, err
			}
		}
	}

	return a.invocation(expr.Invocation(), base, contextType)
}

// indexerExpression walks `expr[index]`, which reads by position and so carries
// the same constraint a positional function does.
func (a *analyzer) indexerExpression(expr *grammar.IndexerExpressionContext, contextType string) (string, error) {
	base, err := a.expression(expr.Expression(0), contextType)
	if err != nil {
		return unknownType, err
	}
	if _, err := a.expression(expr.Expression(1), contextType); err != nil {
		return unknownType, err
	}
	if err := a.checkOrderDependence(expr.Expression(0), "an indexer"); err != nil {
		return unknownType, err
	}
	return base, nil
}

// childExpression returns the i-th expression a node holds, or nil.
func childExpression(node grammar.IExpressionContext, i int) (child grammar.IExpressionContext) {
	holder, ok := node.(interface {
		Expression(int) grammar.IExpressionContext
	})
	if !ok {
		return nil
	}
	if i >= len(node.GetChildren()) {
		return nil
	}

	// The generated accessor indexes into a filtered list, so it panics past
	// the end on some node types rather than returning nil. There is no count
	// to ask for, so the end is found by reaching it.
	defer func() {
		if recover() != nil {
			child = nil
		}
	}()
	return holder.Expression(i)
}

// term walks a leaf of the expression.
func (a *analyzer) term(node grammar.ITermContext, contextType string) (string, error) {
	switch term := node.(type) {
	case *grammar.InvocationTermContext:
		return a.invocation(term.Invocation(), contextType, contextType)

	case *grammar.ParenthesizedTermContext:
		return a.expression(term.Expression(), contextType)

	case *grammar.LiteralTermContext:
		return a.literalType(term.Literal()), nil
	}

	// External constants carry no type the model knows
	return unknownType, nil
}

func (a *analyzer) literalType(node grammar.ILiteralContext) string {
	switch node.(type) {
	case *grammar.BooleanLiteralContext:
		return types.TypeNameBoolean
	case *grammar.StringLiteralContext:
		return types.TypeNameString
	case *grammar.NumberLiteralContext:
		// Integer and Decimal share a literal form; neither is navigated into
		return unknownType
	case *grammar.DateLiteralContext:
		return types.TypeNameDate
	case *grammar.DateTimeLiteralContext:
		return types.TypeNameDateTime
	case *grammar.TimeLiteralContext:
		return types.TypeNameTime
	case *grammar.QuantityLiteralContext:
		return types.TypeNameQuantity
	}
	return unknownType
}

// invocation walks a member access or function call, given the type it is
// applied to.
func (a *analyzer) invocation(node grammar.IInvocationContext, inputType, contextType string) (string, error) {
	switch inv := node.(type) {
	case *grammar.MemberInvocationContext:
		return a.member(inv.Identifier().GetText(), inputType)

	case *grammar.FunctionInvocationContext:
		return a.function(inv.Function(), inputType, contextType)
	}

	// $this, $index and $total
	return unknownType, nil
}

// member checks one step of navigation and returns the type it lands on.
func (a *analyzer) member(name, inputType string) (string, error) {
	name = strings.Trim(name, "`")

	// Nothing to check against
	if inputType == unknownType {
		return unknownType, nil
	}

	// An expression may open with the name of the type it is evaluated against,
	// which is a restatement of the context rather than navigation
	if name == inputType {
		a.path = inputType
		return inputType, nil
	}

	// The path reaches elements the type cannot name
	if a.path != "" && a.path != inputType {
		if resolved := a.model.TypeOf(a.path + "." + name); resolved != "" {
			a.path += "." + name
			return resolved, nil
		}
		if len(a.model.ChoiceTypes(a.path+"."+name)) > 0 {
			a.path += "." + name
			return unknownType, nil
		}
	}

	// A resource type that is not the context is not reachable from it:
	// Encounter.name.given evaluated against a Patient names nothing
	if a.model.IsResource(name) {
		return unknownType, a.fault(
			"%s is not reachable from %s: an expression evaluated against one resource cannot name another",
			name, inputType)
	}

	// A choice element is navigated through ofType(), not by its expanded name:
	// Observation.valueQuantity is how the JSON spells value[x], not an element
	// of Observation. Checked before TypeOf, because a published model carries
	// the expanded names as well and would resolve one.
	if base, ok := a.choiceBase(inputType, name); ok {
		return unknownType, a.fault(
			"%s.%s names a choice element by its type suffix; write %s.ofType(...) instead",
			inputType, name, base)
	}

	if resolved := a.model.TypeOf(inputType + "." + name); resolved != "" {
		a.path = inputType + "." + name
		return resolved, nil
	}

	// A choice element has no single type, so TypeOf cannot name one. It is
	// still an element: Observation.value is how value[x] is navigated.
	if len(a.model.ChoiceTypes(inputType+"."+name)) > 0 {
		return unknownType, nil
	}

	// The model may not describe this type's elements at all, in which case
	// there is nothing to prove either way. A backbone element is the usual
	// case: it holds elements, and none of them belong to its type.
	// Either view is enough to prove the element missing: the type describes
	// its own elements, or the path describes the backbone ones. Only when
	// neither does is there nothing to say.
	if !a.modelDescribes(inputType) && !a.modelDescribes(a.path) {
		return unknownType, nil
	}

	return unknownType, a.fault("%s has no element %s", inputType, name)
}

// choiceBase reports whether a name is a choice element written with its type
// suffix — valueQuantity for value[x] — and the element it belongs to.
func (a *analyzer) choiceBase(inputType, name string) (string, bool) {
	for i := 1; i < len(name); i++ {
		if name[i] < 'A' || name[i] > 'Z' {
			continue
		}

		base := name[:i]
		suffix := name[i:]
		for _, choice := range a.model.ChoiceTypes(inputType + "." + base) {
			if strings.EqualFold(choice, suffix) {
				return base, true
			}
		}
	}
	return "", false
}

// modelDescribes reports whether the model knows the type well enough for a
// missing element to mean the element does not exist.
//
// Knowing the name is not enough. A published model carries the elements of the
// concrete types and not of the abstract ones it derives them from: it answers
// for Patient.id and HumanName.id, and not for Resource.id, though every
// resource has one. Asking for id is what tells the two apart — every FHIR type
// that has elements at all has that one.
func (a *analyzer) modelDescribes(typeName string) bool {
	return a.model.TypeOf(typeName+".id") != ""
}

// unorderedFunctions produce a collection whose order the specification does
// not define.
//
// children() returns "a collection with all immediate child nodes", and
// descendants() is defined in terms of it. Neither says in what order, so an
// expression that takes the second one, or skips the first, is asking a
// question that has no answer — the suite marks Patient.children().skip(1)
// invalid for this reason.
var unorderedFunctions = map[string]bool{
	"children":    true,
	"descendants": true,
	"repeat":      true,
	"repeatAll":   true,
}

// orderDependentFunctions read a collection by position.
var orderDependentFunctions = map[string]bool{
	"skip": true, "take": true, "first": true, "last": true,
	"tail": true, "single": true,
}

// function walks a call and returns the type of its result.
func (a *analyzer) function(node grammar.IFunctionContext, inputType, contextType string) (string, error) {
	name := strings.Trim(node.Identifier().GetText(), "`")
	args := functionArguments(node)

	if err := a.checkFunctionArguments(name, args, inputType, contextType); err != nil {
		return unknownType, err
	}

	switch name {
	// The input passes through unchanged
	case "where", "first", "last", "tail", "skip", "take", "single",
		"distinct", "intersect", "exclude", "union", "combine", "trace":
		return inputType, nil

	// A type is named in the call, and the result takes it
	case "ofType", "as":
		if len(args) == 1 {
			return strings.Trim(strings.TrimPrefix(args[0].GetText(), "FHIR."), "`"), nil
		}
		return unknownType, nil

	// The result type is fixed by the function
	case "exists", "empty", "all", "allTrue", "anyTrue", "allFalse", "anyFalse",
		"is", "not", "hasValue", "isDistinct", "subsetOf", "supersetOf",
		"contains", "startsWith", "endsWith", "matches", "matchesFull",
		"convertsToBoolean", "convertsToInteger", "convertsToDecimal",
		"convertsToString", "convertsToDate", "convertsToDateTime",
		"convertsToTime", "convertsToQuantity", "conformsTo", "memberOf":
		return types.TypeNameBoolean, nil

	case "count", "length", "indexOf", "lastIndexOf", "toInteger", "precision":
		return types.TypeNameInteger, nil

	case "toString", "join", "substring", "upper", "lower", "replace",
		"replaceMatches", "trim", "encode", "decode", "escape", "unescape":
		return types.TypeNameString, nil

	case "toQuantity":
		return types.TypeNameQuantity, nil
	case "toDate":
		return types.TypeNameDate, nil
	case "toDateTime":
		return types.TypeNameDateTime, nil
	case "toTime":
		return types.TypeNameTime, nil

	// resolve() lands on whatever the reference points at
	case "resolve", "extension", "select", "repeat", "repeatAll",
		"children", "descendants", "iif", "aggregate", "defineVariable":
		return unknownType, nil
	}

	return unknownType, nil
}

// functionArguments lists the expressions passed to a call.
func functionArguments(node grammar.IFunctionContext) []grammar.IExpressionContext {
	list := node.ParamList()
	if list == nil {
		return nil
	}
	return list.AllExpression()
}

// scopedFunctions evaluate their argument once per item of their input, with
// $this set to that item. So the argument navigates from the input's type, not
// from the type the whole expression is evaluated against: in
// Patient.name.where(use = 'official'), use belongs to HumanName.
var scopedFunctions = map[string]bool{
	"where": true, "select": true, "all": true, "exists": true,
	"repeat": true, "repeatAll": true, "aggregate": true, "sort": true,
	"iif": true, "defineVariable": true, "trace": true,
}

// checkFunctionArguments applies the rules that constrain a call beyond the
// navigation its result allows.
func (a *analyzer) checkFunctionArguments(name string, args []grammar.IExpressionContext, inputType, contextType string) error {
	// The scope an argument is navigated from
	argumentScope := contextType
	argumentPath := contextType
	if scopedFunctions[name] {
		argumentScope = inputType
		argumentPath = a.path
	}

	// An argument navigates from where the call sits, and leaves it there: the
	// path is where this analysis has reached, not where an argument wandered.
	// Without restoring it, supportingInfo.where(category.coding.code = ...)
	// would carry on from category.coding.code rather than from supportingInfo.
	callerPath := a.path
	defer func() { a.path = callerPath }()
	a.path = argumentPath

	// "The criterion expression SHALL evaluate to a Boolean, consistent with
	// singleton evaluation of collections." A criterion that is statically a
	// String or a multi-item collection is not one.
	if name == "iif" && len(args) > 0 {
		if err := a.checkBooleanCriterion(args[0], argumentScope); err != nil {
			return err
		}
	}

	// A type specifier is a type name, not an expression to navigate
	if name == "ofType" || name == "as" || name == "is" {
		return nil
	}

	for _, arg := range args {
		if _, err := a.expression(arg, argumentScope); err != nil {
			return err
		}
	}
	return nil
}

// checkBooleanCriterion rejects an iif criterion that cannot be a Boolean.
func (a *analyzer) checkBooleanCriterion(criterion grammar.IExpressionContext, contextType string) error {
	// A union of several values is not a singleton, and the singleton rule ends
	// in an error for those rather than converting them. One of {} | true is
	// empty, so that union is a singleton and a perfectly good criterion.
	if countUnionItems(criterion) > 1 {
		return a.fault("the criterion of iif() must be a Boolean, and a union of %d values is not a singleton",
			countUnionItems(criterion))
	}

	resultType, err := a.expression(criterion, contextType)
	if err != nil {
		return err
	}

	// Only a type the analysis is sure about can be ruled out. A single node of
	// another type evaluates to true under the singleton rule, but a literal
	// String was never a criterion in the first place.
	if term, ok := criterion.(*grammar.TermExpressionContext); ok {
		if literal, ok := term.Term().(*grammar.LiteralTermContext); ok {
			if _, isString := literal.Literal().(*grammar.StringLiteralContext); isString {
				return a.fault("the criterion of iif() must be a Boolean, and %q is a String",
					criterion.GetText())
			}
		}
	}
	_ = resultType

	return nil
}

// checkOrderDependence rejects reading an unordered collection by position,
// where the collection is the result of the given expression.
func (a *analyzer) checkOrderDependence(node grammar.IExpressionContext, reader string) error {
	name, ok := trailingFunctionName(node)
	if !ok || !unorderedFunctions[name] {
		return nil
	}
	return a.fault("%s() returns a collection in no defined order, so %s of it has no defined result",
		name, reader)
}

// trailingFunctionName returns the name of the function an expression ends
// with, which is what produced the collection being read.
func trailingFunctionName(node grammar.IExpressionContext) (string, bool) {
	invocation, ok := node.(*grammar.InvocationExpressionContext)
	if !ok {
		return "", false
	}
	call, ok := invocation.Invocation().(*grammar.FunctionInvocationContext)
	if !ok {
		return "", false
	}
	return strings.Trim(call.Function().Identifier().GetText(), "`"), true
}

// countUnionItems counts the operands of a union that provably yield an item.
//
// Only literals are counted: anything else may yield nothing, one thing or many,
// and the analysis does not guess. So 1 | 2 | 3 counts three and {} | true
// counts one, while name | telecom counts none and is left alone.
func countUnionItems(node grammar.IExpressionContext) int {
	union, ok := node.(*grammar.UnionExpressionContext)
	if !ok {
		if yieldsOneItem(node) {
			return 1
		}
		return 0
	}

	total := 0
	for i := 0; i < 2; i++ {
		if operand := union.Expression(i); operand != nil {
			total += countUnionItems(operand)
		}
	}
	return total
}

// yieldsOneItem reports whether an expression is a literal that stands for
// exactly one value. The empty literal {} stands for none.
func yieldsOneItem(node grammar.IExpressionContext) bool {
	term, ok := node.(*grammar.TermExpressionContext)
	if !ok {
		return false
	}
	literal, ok := term.Term().(*grammar.LiteralTermContext)
	if !ok {
		return false
	}
	_, isEmpty := literal.Literal().(*grammar.NullLiteralContext)
	return !isEmpty
}
