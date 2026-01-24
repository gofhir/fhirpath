package fhirpath

import (
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/funcs"
	"github.com/gofhir/fhirpath/parser/grammar"
	"github.com/gofhir/fhirpath/types"
)

// Expression represents a compiled FHIRPath expression.
type Expression struct {
	source string
	tree   *grammar.EntireExpressionContext
}

// Evaluate executes the expression against a JSON resource.
func (e *Expression) Evaluate(resource []byte) (types.Collection, error) {
	ctx := eval.NewContext(resource)
	return e.EvaluateWithContext(ctx)
}

// EvaluateWithContext executes the expression with a custom context.
func (e *Expression) EvaluateWithContext(ctx *eval.Context) (types.Collection, error) {
	evaluator := eval.NewEvaluator(ctx, funcs.GetRegistry())
	return evaluator.Evaluate(e.tree)
}

// String returns the original expression string.
func (e *Expression) String() string {
	return e.source
}
