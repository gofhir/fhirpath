package fhirpath_test

import (
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// An operator given more than one item on a side says which side, and how many
// each had. Adding the two counts up reported (1 | 2) + 1 as "got 3 elements",
// which names neither the operand at fault nor what it held.
func TestOperandSingletonErrorNamesTheSide(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{"(1 | 2) + 1", "+ expects a single item on each side, got 2 on the left and 1 on the right"},
		{"1 - (1 | 2 | 3)", "- expects a single item on each side, got 1 on the left and 3 on the right"},
		{"(1 | 2) * 3", "* expects a single item on each side, got 2 on the left and 1 on the right"},
		{"(1 | 2) < 3", "< expects a single item on each side, got 2 on the left and 1 on the right"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			_, err := fhirpath.Evaluate([]byte(`{"resourceType":"Basic"}`), tt.expr)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("%s: error %v, want it to say %q", tt.expr, err, tt.want)
			}
		})
	}
}

// A boolean operator is held to the same rule. Its three-valued logic answers
// an empty operand, but more than one item is an error like any other: US Core's
// pd-1, telecom or endpoint, was empty on a PractitionerRole with two telecoms,
// which a validator reads as the invariant failing rather than as an error.
func TestBooleanOperandSingleton(t *testing.T) {
	role := []byte(`{"resourceType":"PractitionerRole","telecom":[{"value":"1"},{"value":"2"}]}`)

	for _, tt := range []struct{ expr, want string }{
		{"telecom or endpoint", "or expects a single item on each side, got 2 on the left and 0 on the right"},
		{"telecom and true", "and expects a single item on each side, got 2 on the left and 1 on the right"},
		{"false xor telecom", "xor expects a single item on each side, got 1 on the left and 2 on the right"},
		{"telecom implies true", "implies expects a single item on each side, got 2 on the left and 1 on the right"},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			_, err := fhirpath.Evaluate(role, tt.expr)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("%s: error %v, want it to say %q", tt.expr, err, tt.want)
			}
		})
	}

	// A left operand that decides the result still leaves the right one
	// unevaluated, so its count is never seen.
	for _, tt := range []struct{ expr, want string }{
		{"true or telecom", "true"},
		{"false and telecom", "false"},
		{"false implies telecom", "true"},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := fhirpath.Evaluate(role, tt.expr)
			if err != nil || len(got) != 1 || got[0].String() != tt.want {
				t.Errorf("%s = %v, %v; want %s", tt.expr, got, err, tt.want)
			}
		})
	}
}
