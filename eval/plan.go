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

	switch ctx := tree.(type) {
	// Wrappers the grammar needs and evaluation does not: each one stands for
	// its only child, and compiling through them removes a call per level.
	case *grammar.EntireExpressionContext:
		return Compile(ctx.Expression())
	case *grammar.TermExpressionContext:
		return Compile(ctx.Term())
	case *grammar.InvocationTermContext:
		return Compile(ctx.Invocation())
	case *grammar.LiteralTermContext:
		return Compile(ctx.Literal())
	case *grammar.ParenthesizedTermContext:
		return Compile(ctx.Expression())
	case *grammar.ExternalConstantTermContext:
		return Compile(ctx.ExternalConstant())

	// Literals denote the same value on every evaluation, so they are built
	// once. The visitor is what builds them, which keeps one reading of the
	// literal syntax rather than two.
	case *grammar.NullLiteralContext:
		return constant(literalValue(ctx))
	case *grammar.BooleanLiteralContext:
		return constant(literalValue(ctx))
	case *grammar.StringLiteralContext:
		return constant(literalValue(ctx))
	case *grammar.NumberLiteralContext:
		return constant(literalValue(ctx))
	case *grammar.DateLiteralContext:
		return constant(literalValue(ctx))
	case *grammar.DateTimeLiteralContext:
		return constant(literalValue(ctx))
	case *grammar.TimeLiteralContext:
		return constant(literalValue(ctx))
	case *grammar.QuantityLiteralContext:
		return constant(literalValue(ctx))

	case *grammar.ExternalConstantContext:
		return compileExternalConstant(ctx)

	case *grammar.MemberInvocationContext:
		name := stripBackticks(ctx.Identifier().GetText())
		return func(e *Evaluator) interface{} {
			return e.navigateMember(e.ctx.This(), name)
		}

	case *grammar.ThisInvocationContext:
		return func(e *Evaluator) interface{} { return e.ctx.This() }
	case *grammar.IndexInvocationContext:
		return func(e *Evaluator) interface{} {
			return types.Collection{types.NewInteger(int64(e.ctx.index))}
		}
	case *grammar.TotalInvocationContext:
		return func(e *Evaluator) interface{} {
			if e.ctx.total != nil {
				return types.Collection{e.ctx.total}
			}
			return types.Collection{}
		}

	case *grammar.InvocationExpressionContext:
		return compileInvocation(ctx)
	case *grammar.IndexerExpressionContext:
		return compileIndexer(ctx)

	case *grammar.MultiplicativeExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), operatorOf(ctx), applyMultiplicative)
	case *grammar.AdditiveExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), operatorOf(ctx), applyAdditive)
	case *grammar.InequalityExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), operatorOf(ctx), applyInequality)
	case *grammar.EqualityExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), operatorOf(ctx), applyEquality)
	case *grammar.MembershipExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), operatorOf(ctx), applyMembership)
	case *grammar.AndExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), "and", applyAnd)
	case *grammar.OrExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), operatorOf(ctx), applyOr)
	case *grammar.ImpliesExpressionContext:
		return compileBinary(ctx.Expression(0), ctx.Expression(1), "implies", applyImplies)

	case *grammar.UnionExpressionContext:
		return compileUnion(ctx)
	}

	// Everything else — function invocations above all, whose arguments are
	// evaluated in scopes the visitor manages — keeps the visitor.
	return func(e *Evaluator) interface{} { return tree.Accept(e) }
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
	if node, ok := ctx.GetChild(1).(antlr.TerminalNode); ok {
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
