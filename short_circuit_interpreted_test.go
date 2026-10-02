package fhirpath

import (
	"testing"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/funcs"
)

// The parse tree is still walked where a node does not compile, so the
// interpreted and, or and implies decide from the left operand as the compiled
// ones do.
func TestInterpretedBooleanOperatorsDoNotEvaluateARightOperandTheLeftDecides(t *testing.T) {
	element := []byte(`{"type":[{"code":"boolean"},{"code":"dateTime"}]}`)

	for _, tt := range []struct {
		expr    string
		want    string
		wantErr bool
	}{
		{"true or type.code.contains(':')", "[true]", false},
		{"false and type.code.contains(':')", "[false]", false},
		{"false implies type.code.contains(':')", "[true]", false},
		{"{} or true", "[true]", false},
		{"{} or type.code.contains(':')", "", true},
		{"true and type.code.contains(':')", "", true},
		{"true xor type.code.contains(':')", "", true},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			expr := MustCompile(tt.expr)
			result, err := eval.NewEvaluator(eval.NewContext(element), funcs.GetRegistry()).Evaluate(expr.tree)
			if tt.wantErr {
				if err == nil {
					t.Errorf("%s = %s, want the right operand's error", tt.expr, result)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: %v", tt.expr, err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}
		})
	}
}
