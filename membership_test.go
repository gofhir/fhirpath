package fhirpath

import "testing"

// TestMembershipOperatorArity covers the arity rule in and contains state, which
// is stricter than the empty propagation around it.
//
// in: "If the left-hand side of the operator is empty, the result is empty, if
// the right-hand side is empty, the result is false. If the left operand has
// multiple items, an exception is thrown."
//
// contains is the converse, with the sides exchanged. The error matters because
// empty would read as "not found": answering that 'a' is not in a collection
// when the question was which of three values to look for is a wrong answer, not
// an unknown one.
func TestMembershipOperatorArity(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	t.Run("multiple items on the deciding side is an error", func(t *testing.T) {
		for _, expr := range []string{
			"('a' | 'c' | 'd') in 'b'",
			"('a' | 'b') in ('a' | 'b')",
			"'b' contains ('a' | 'c' | 'd')",
		} {
			if _, err := MustCompile(expr).Evaluate(patient); err == nil {
				t.Errorf("%s: expected an error", expr)
			}
		}
	})

	t.Run("the other cases answer", func(t *testing.T) {
		cases := []struct{ expr, want string }{
			{"'a' in ('a' | 'b')", "true"},
			{"'z' in ('a' | 'b')", "false"},
			{"('a' | 'b') contains 'a'", "true"},
			{"('a' | 'b') contains 'z'", "false"},

			// Empty on the deciding side is empty; on the other side it is false
			{"{} in ('a' | 'b')", "EMPTY"},
			{"'a' in {}", "false"},
			{"{} contains 'a'", "false"},
			{"('a' | 'b') contains {}", "EMPTY"},
		}
		for _, tc := range cases {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		}
	})
}
