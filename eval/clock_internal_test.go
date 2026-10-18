package eval

import (
	"testing"
	"time"

	"github.com/antlr4-go/antlr/v4"

	"github.com/gofhir/fhirpath/parser/grammar"
	"github.com/gofhir/fhirpath/types"
)

// nowOnly is a registry with now() alone, reading the context's instant, so
// that the evaluator's own entry points can be tested without the funcs
// package, which imports this one.
type nowOnly struct{}

func (nowOnly) Get(name string) (FuncDef, bool) {
	if name != "now" {
		return FuncDef{}, false
	}
	return FuncDef{Name: "now", Fn: func(ctx *Context, _ types.Collection, _ []interface{}) (types.Collection, error) {
		return types.Collection{types.NewDateTimeFromTime(ctx.Now())}, nil
	}}, true
}

// Evaluate, which takes a parse tree, gives its evaluation one instant as
// EvaluateNode does.
func TestEvaluatingAParseTreeSeesOneInstant(t *testing.T) {
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	restore := SetNowSource(func() time.Time {
		at = at.Add(time.Millisecond)
		return at
	})
	defer restore()

	lexer := grammar.NewfhirpathLexer(antlr.NewInputStream("now() = now()"))
	tree := grammar.NewfhirpathParser(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)).EntireExpression()
	result, err := NewEvaluator(NewContext([]byte(`{}`)), nowOnly{}).Evaluate(tree)
	if err != nil || result.String() != "[true]" {
		t.Errorf("now() = now() = %v %v", result, err)
	}
}
