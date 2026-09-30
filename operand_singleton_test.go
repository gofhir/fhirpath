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
