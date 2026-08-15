package eval

import (
	"github.com/antlr4-go/antlr/v4"

	"github.com/gofhir/fhirpath/parser/grammar"
	"github.com/gofhir/fhirpath/types"
)

// A Node is an expression that has been compiled: the work that depends only on
// the expression — reading identifiers out of the parse tree, parsing literals,
// deciding which operator a token stands for — is done once, when the Node is
// built, and what remains at evaluation time is the work that depends on the
// resource.
//
// A Node returns what a visitor method returns: a types.Collection, or an error.
//
// Compilation is partial by design. Anything Compile does not have a case for
// keeps its visitor, reached through a Node that calls it, so a construct is
// either compiled or behaves exactly as it did before — never something in
// between. Migrating one more construct is then a local change with the
// conformance suite as its check.
type Node func(e *Evaluator) interface{}

// Compile turns a parse tree into a Node.
func Compile(tree antlr.ParseTree) Node {
	if tree == nil {
		return func(*Evaluator) interface{} { return types.Collection{} }
	}

	if node, ok := compileWrapper(tree); ok {
		return node
	}
	if node, ok := compileLiteral(tree); ok {
		return node
	}
	if node, ok := compileNavigation(tree); ok {
		return node
	}
	if node, ok := compileOperator(tree); ok {
		return node
	}

	// Everything else keeps its visitor.
	return func(e *Evaluator) interface{} { return tree.Accept(e) }
}

// compileWrapper compiles the rules the grammar needs and evaluation does not:
// each one stands for its only child, and compiling through them removes a call
// per level.
func compileWrapper(tree antlr.ParseTree) (Node, bool) {
	switch ctx := tree.(type) {
	case *grammar.EntireExpressionContext:
		return Compile(ctx.Expression()), true
	case *grammar.TermExpressionContext:
		return Compile(ctx.Term()), true
	case *grammar.InvocationTermContext:
		return Compile(ctx.Invocation()), true
	case *grammar.LiteralTermContext:
		return Compile(ctx.Literal()), true
	case *grammar.ParenthesizedTermContext:
		return Compile(ctx.Expression()), true
	case *grammar.ExternalConstantTermContext:
		return Compile(ctx.ExternalConstant()), true
	}
	return nil, false
}

// compileLiteral compiles a literal, which denotes the same value on every
// evaluation and is therefore built once. The visitor is what builds it, which
// keeps one reading of the literal syntax rather than two.
func compileLiteral(tree antlr.ParseTree) (Node, bool) {
	switch tree.(type) {
	case *grammar.NullLiteralContext,
		*grammar.BooleanLiteralContext,
		*grammar.StringLiteralContext,
		*grammar.NumberLiteralContext,
		*grammar.DateLiteralContext,
		*grammar.DateTimeLiteralContext,
		*grammar.TimeLiteralContext,
		*grammar.QuantityLiteralContext:
		return constant(literalValue(tree)), true
	}
	return nil, false
}

// compileNavigation compiles the ways an expression reaches a value: a member
// of what precedes it, a variable, an element by position, or a function call.
func compileNavigation(tree antlr.ParseTree) (Node, bool) {
	switch ctx := tree.(type) {
	case *grammar.ExternalConstantContext:
		return compileExternalConstant(ctx), true

	case *grammar.MemberInvocationContext:
		name := stripBackticks(ctx.Identifier().GetText())
		return func(e *Evaluator) interface{} {
			return e.navigateMember(e.ctx.This(), name)
		}, true

	case *grammar.ThisInvocationContext:
		return func(e *Evaluator) interface{} { return e.ctx.This() }, true
	case *grammar.IndexInvocationContext:
		return func(e *Evaluator) interface{} {
			return types.Collection{types.NewInteger(int64(e.ctx.index))}
		}, true
	case *grammar.TotalInvocationContext:
		return func(e *Evaluator) interface{} {
			if e.ctx.total != nil {
				return types.Collection{e.ctx.total}
			}
			return types.Collection{}
		}, true

	case *grammar.FunctionInvocationContext:
		return compileFunction(ctx), true
	case *grammar.InvocationExpressionContext:
		return compileInvocation(ctx), true
	case *grammar.IndexerExpressionContext:
		return compileIndexer(ctx), true
	}
	return nil, false
}

// compileOperator compiles the binary operators, each of which resolves here to
// the function that applies it rather than being recognized again per
// evaluation.
func compileOperator(tree antlr.ParseTree) (Node, bool) {
	switch ctx := tree.(type) {
	case *grammar.MultiplicativeExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), operatorOf(ctx), applyMultiplicative), true
	case *grammar.AdditiveExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), operatorOf(ctx), applyAdditive), true
	case *grammar.InequalityExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), operatorOf(ctx), applyInequality), true
	case *grammar.EqualityExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), operatorOf(ctx), applyEquality), true
	case *grammar.MembershipExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), operatorOf(ctx), applyMembership), true
	case *grammar.AndExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), "and", applyAnd), true
	case *grammar.OrExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), operatorOf(ctx), applyOr), true
	case *grammar.ImpliesExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), "implies", applyImplies), true
	case *grammar.UnionExpressionContext:
		return compileUnion(ctx), true

	case *grammar.PolarityExpressionContext:
		operand := Compile(ctx.Expression())
		negate := operatorAt(ctx, 0) == "-"
		return func(e *Evaluator) interface{} {
			result := operand(e)
			if err, ok := result.(error); ok {
				return err
			}
			return applyPolarity(result.(types.Collection), negate)
		}, true

	case *grammar.TypeExpressionContext:
		operand := Compile(ctx.Expression())
		op := operatorOf(ctx)
		typeName := ctx.TypeSpecifier().GetText()
		return func(e *Evaluator) interface{} {
			result := operand(e)
			if err, ok := result.(error); ok {
				return err
			}
			return e.applyTypeOperator(result.(types.Collection), op, typeName)
		}, true
	}
	return nil, false
}

// literalValue evaluates a literal at compile time. A literal visitor reads
// nothing from the evaluation context, so an evaluator without one answers it.
func literalValue(tree antlr.ParseTree) interface{} {
	return tree.Accept(&Evaluator{})
}

// constant returns a Node for a value that never changes.
//
// The collection is rebuilt per evaluation rather than shared: a caller is free
// to treat what it receives as its own, and one that appends to it would
// otherwise write into every later evaluation of the same expression. The value
// inside is immutable and is shared.
func constant(result interface{}) Node {
	if err, ok := result.(error); ok {
		return func(*Evaluator) interface{} { return err }
	}

	col, ok := result.(types.Collection)
	if !ok || len(col) == 0 {
		return func(*Evaluator) interface{} { return types.Collection{} }
	}
	if len(col) == 1 {
		v := col[0]
		return func(*Evaluator) interface{} { return types.Collection{v} }
	}

	return func(*Evaluator) interface{} {
		out := make(types.Collection, len(col))
		copy(out, col)
		return out
	}
}

// operatorOf returns the operator token of a binary expression, which is always
// its second child.
func operatorOf(ctx antlr.ParseTree) string {
	return operatorAt(ctx, 1)
}

// operatorAt returns the operator token at the given position among a rule's
// children.
func operatorAt(ctx antlr.ParseTree, index int) string {
	if node, ok := ctx.GetChild(index).(antlr.TerminalNode); ok {
		return node.GetText()
	}
	return ""
}

func compileExternalConstant(ctx *grammar.ExternalConstantContext) Node {
	var name string
	if ctx.Identifier() != nil {
		name = stripBackticks(ctx.Identifier().GetText())
	} else if ctx.STRING() != nil {
		name = unquoteString(ctx.STRING().GetText())
	}

	// A parameterized FHIR constant resolves to the same URL every time.
	if url, ok := fhirConstantURL(name); ok {
		constantURL := types.NewString(url)
		return func(e *Evaluator) interface{} {
			if value, ok := e.ctx.GetVariable(name); ok {
				return value
			}
			return types.Collection{constantURL}
		}
	}

	return func(e *Evaluator) interface{} {
		if value, ok := e.ctx.GetVariable(name); ok {
			return value
		}
		return NewEvalError(ErrInvalidPath, "undefined variable: %%%s", name)
	}
}

// compileFunction compiles a function call.
//
// The name, the arguments and — for the functions that take one — the type
// specifier are read from the tree once. What cannot be settled here is
// resolution in the registry, since a custom function may be registered after
// an expression is compiled, so the lookup stays at evaluation time.
//
// The functions that evaluate an argument per element of their input receive
// that argument compiled, which is what takes the parse tree out of the loop:
// where(use = 'official') over a hundred names used to walk the criteria a
// hundred times.
func compileFunction(ctx *grammar.FunctionInvocationContext) Node {
	funcCtx := ctx.Function()
	call := &compiledCall{name: stripBackticks(funcCtx.Identifier().GetText())}

	if paramList := funcCtx.ParamList(); paramList != nil {
		call.argExprs = paramList.AllExpression()
	}

	call.args = make([]Node, len(call.argExprs))
	for i, argExpr := range call.argExprs {
		call.args[i] = Compile(argExpr)
	}

	// A type specifier is read as written rather than evaluated, so the name is
	// taken from the tree here and that argument's Node is never used.
	if len(call.argExprs) > 0 {
		call.typeName = stripDelimiters(call.argExprs[0].GetText())
	}

	// sort() resolves the direction of each ordering key at compile time. This
	// compiles the arguments a second time, so it is done for sort() alone:
	// Compile recurses, and compiling every argument twice at every level costs
	// exponentially in the nesting depth.
	if call.name == "sort" {
		call.sortCriteria = make([]sortCriterion, 0, len(call.argExprs))
		for _, argExpr := range call.argExprs {
			expr, descending := unwrapSortDirection(argExpr)
			call.sortCriteria = append(call.sortCriteria, sortCriterion{expr: Compile(expr), descending: descending})
		}
	}

	return call.eval
}

// A compiledCall holds what a function call knows before it runs.
type compiledCall struct {
	name         string
	argExprs     []grammar.IExpressionContext
	args         []Node
	typeName     string
	sortCriteria []sortCriterion
}

func (c *compiledCall) eval(e *Evaluator) interface{} {
	// Resolution stays here rather than at compile time: a custom function may
	// be registered after an expression is compiled.
	fn, ok := e.funcs.Get(c.name)
	if !ok {
		return FunctionNotFoundError(c.name)
	}

	argCount := len(c.args)
	if argCount < fn.MinArgs {
		return InvalidArgumentsError(c.name, fn.MinArgs, argCount)
	}
	if fn.MaxArgs >= 0 && argCount > fn.MaxArgs {
		return InvalidArgumentsError(c.name, fn.MaxArgs, argCount)
	}

	if result, handled := c.evalControlling(e); handled {
		return result
	}

	return c.evalOrdinary(e, fn)
}

// evalControlling handles the functions that decide for themselves whether and
// how often their arguments are evaluated. They receive their arguments
// compiled, which is what takes the parse tree out of the loop —
// where(use = 'official') over a hundred names used to walk the criteria a
// hundred times.
func (c *compiledCall) evalControlling(e *Evaluator) (interface{}, bool) {
	if result, handled := c.evalPerElement(e); handled {
		return result, true
	}
	return c.evalLazy(e)
}

// evalPerElement handles the functions that run an argument once per element of
// their input.
func (c *compiledCall) evalPerElement(e *Evaluator) (interface{}, bool) {
	input := e.ctx.This()

	// sort() orders its input whether or not ordering keys were given.
	if c.name == "sort" {
		return e.evaluateSort(input, c.sortCriteria), true
	}

	// The rest need an argument to apply. Without one the call is an ordinary
	// one — exists() with no criteria is the plain function.
	if len(c.args) == 0 {
		return nil, false
	}

	switch c.name {
	case "where":
		return e.evaluateWhere(input, c.args[0]), true
	case "exists":
		return e.evaluateExists(input, c.args[0]), true
	case "all":
		return e.evaluateAll(input, c.args[0]), true
	case "select":
		return e.evaluateSelect(input, c.args[0]), true
	case "repeat":
		return e.evaluateRepeat(input, c.args[0], true), true
	case "repeatAll":
		return e.evaluateRepeat(input, c.args[0], false), true
	case "aggregate":
		var init Node
		if len(c.args) > 1 {
			init = c.args[1]
		}
		return e.evaluateAggregate(input, c.args[0], init), true
	}

	return nil, false
}

// evalLazy handles the functions that take a branch, bind a name, or read a
// type specifier as written rather than evaluating it.
func (c *compiledCall) evalLazy(e *Evaluator) (interface{}, bool) {
	if len(c.args) == 0 {
		return nil, false
	}

	input := e.ctx.This()

	switch c.name {
	case "is":
		return e.evaluateIsFunction(input, c.typeName), true
	case "as":
		return e.evaluateAsFunction(input, c.typeName), true
	case "ofType":
		return e.evaluateOfType(input, c.typeName), true
	case "defineVariable":
		return e.evaluateDefineVariable(input, c.args), true
	case "coalesce":
		return e.evaluateCoalesce(c.args), true
	case "iif":
		// iif(criterion, true-result [, otherwise-result]) — with fewer
		// arguments the ordinary path reports the arity
		if len(c.args) >= 2 {
			return e.evaluateIif(input, c.args), true
		}
	}

	return nil, false
}

// evalOrdinary evaluates the arguments and calls the function with them.
func (c *compiledCall) evalOrdinary(e *Evaluator, fn FuncDef) interface{} {
	evaluated := make([]interface{}, len(c.args))

	if len(c.args) > 0 {
		// Arguments are evaluated in the scope the invocation sits in, not in
		// its input
		argThis := e.ctx.this
		if e.ctx.outer != nil {
			argThis = e.ctx.outer
		}

		restore := e.ctx.this
		e.ctx.this = argThis
		for i, arg := range c.args {
			if isTypeArg(fn.TypeArgs, i) {
				// The type name is taken as written rather than evaluated
				evaluated[i] = types.Collection{types.NewString(stripDelimiters(c.argExprs[i].GetText()))}
				continue
			}

			// Each argument is its own scope, so two of them may define the
			// same name without colliding
			result := e.inScope(arg)
			if err, ok := result.(error); ok {
				e.ctx.this = restore
				return err
			}
			evaluated[i] = result
		}
		e.ctx.this = restore
	}

	result, err := fn.Fn(e.ctx, e.ctx.This(), evaluated)
	if err != nil {
		return err
	}
	return result
}

// compileInvocation compiles expr.invocation.
func compileInvocation(ctx *grammar.InvocationExpressionContext) Node {
	base := Compile(ctx.Expression())
	invocation := Compile(ctx.Invocation())

	return func(e *Evaluator) interface{} {
		result := base(e)
		if err, ok := result.(error); ok {
			return err
		}
		baseCol := result.(types.Collection)

		// The scope handling VisitInvocationExpression describes: the input is
		// what precedes the dot, while a function's arguments are navigated
		// from the scope in force before it.
		oldThis := e.ctx.this
		oldPath := e.ctx.path
		oldOuter := e.ctx.outer

		e.ctx.outer = oldThis
		e.ctx.this = baseCol
		defer func() {
			e.ctx.this = oldThis
			e.ctx.path = oldPath
			e.ctx.outer = oldOuter
		}()

		return invocation(e)
	}
}

// compileIndexer compiles expr[index].
func compileIndexer(ctx *grammar.IndexerExpressionContext) Node {
	base := Compile(ctx.Expression(0))
	index := Compile(ctx.Expression(1))

	return func(e *Evaluator) interface{} {
		baseResult := base(e)
		if err, ok := baseResult.(error); ok {
			return err
		}
		baseCol := baseResult.(types.Collection)

		indexResult := index(e)
		if err, ok := indexResult.(error); ok {
			return err
		}
		indexCol := indexResult.(types.Collection)

		return applyIndex(baseCol, indexCol)
	}
}

// compileUnion compiles expr | expr, each branch in its own variable scope.
func compileUnion(ctx *grammar.UnionExpressionContext) Node {
	left := Compile(ctx.Expression(0))
	right := Compile(ctx.Expression(1))

	return func(e *Evaluator) interface{} {
		leftResult := e.inScope(left)
		if err, ok := leftResult.(error); ok {
			return err
		}
		rightResult := e.inScope(right)
		if err, ok := rightResult.(error); ok {
			return err
		}
		return Union(leftResult.(types.Collection), rightResult.(types.Collection))
	}
}

// inScope evaluates a Node with its own variable scope, as visitInScope does
// for a parse tree.
func (e *Evaluator) inScope(n Node) interface{} {
	endScope := e.ctx.enterIterationScope()
	defer endScope()
	return n(e)
}

// compileBinary compiles the shape every binary operator shares: evaluate both
// sides, then apply. The operator is resolved here rather than read from the
// tree on each evaluation.
func compileBinary(leftExpr, rightExpr grammar.IExpressionContext, op string,
	apply func(left, right types.Collection, op string) interface{}) Node {
	left := Compile(leftExpr)
	right := Compile(rightExpr)

	return func(e *Evaluator) interface{} {
		leftResult := left(e)
		if err, ok := leftResult.(error); ok {
			return err
		}
		rightResult := right(e)
		if err, ok := rightResult.(error); ok {
			return err
		}
		return apply(leftResult.(types.Collection), rightResult.(types.Collection), op)
	}
}

// EvaluateNode evaluates a compiled Node and returns its result.
func (e *Evaluator) EvaluateNode(n Node) (types.Collection, error) {
	result := n(e)
	if err, ok := result.(error); ok {
		return nil, err
	}
	if col, ok := result.(types.Collection); ok {
		return col, nil
	}
	return types.Collection{}, nil
}
