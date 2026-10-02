package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// and, or and implies do not evaluate their right operand when the left one
// decides the result, as the HL7 validator does (FHIRPathEngine.preOperate).
// FHIR's published invariants are written for it: tim-9 in R4 evaluates
// when in (...) on a timing with two when values, and eld-11 in R5
// type.code.contains(':') on an element of two types, each behind a left
// operand that is already true. An error from the right operand used to end
// the whole expression.
func TestBooleanOperatorsDoNotEvaluateARightOperandTheLeftDecides(t *testing.T) {
	element := `{"type":[{"code":"boolean"},{"code":"dateTime"}]}`
	timing := `{"when":["MORN","EVE"]}`

	decided := []struct {
		resource, expr, want string
	}{
		{element, "true or type.code.contains(':')", "[true]"},
		{element, "false and type.code.contains(':')", "[false]"},
		{element, "false implies type.code.contains(':')", "[true]"},
		{element, "binding.empty() or type.code.contains(':')", "[true]"},
		// tim-9 as R4 and R4B publish it.
		{timing, "offset.empty() or (when.exists() and ((when in ('C' | 'CM' | 'CD' | 'CV')).not()))", "[true]"},
		// The results of three-valued logic are unchanged where nothing is skipped.
		{element, "{} or true", "[true]"},
		{element, "{} and false", "[false]"},
		{element, "true and {}", "[]"},
		{element, "false or {}", "[]"},
		{element, "true implies {}", "[]"},
	}
	for _, tt := range decided {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(tt.resource), tt.expr)
			if err != nil {
				t.Fatalf("%s: %v", tt.expr, err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}
		})
	}

	// The right operand is evaluated, and its error is the answer, where the
	// left one does not decide: empty, the other boolean, or not a boolean —
	// as the HL7 validator does — and always for xor.
	evaluated := []string{
		"{} or type.code.contains(':')",
		"false or type.code.contains(':')",
		"true and type.code.contains(':')",
		"true implies type.code.contains(':')",
		"'a' or type.code.contains(':')",
		"true xor type.code.contains(':')",
	}
	for _, expr := range evaluated {
		t.Run(expr, func(t *testing.T) {
			if result, err := fhirpath.Evaluate([]byte(element), expr); err == nil {
				t.Errorf("%s = %s, want the right operand's error", expr, result)
			}
		})
	}
}
