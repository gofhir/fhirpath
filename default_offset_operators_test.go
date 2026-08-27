package fhirpath

import (
	"testing"
	"time"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// Every operator that needs an instant reads the same offset, so a default
// supplied for a value moves all of them together. They would otherwise say
// different things about one value: that it is later than another, and that it
// is two hours earlier.
func TestOperatorsAgreeOnTheDefaultOffset(t *testing.T) {
	tests := []struct {
		name       string
		offset     time.Duration
		afterBare  bool
		hoursApart string
	}{
		{"at UTC", 0, true, "2"},
		{"at UTC-5", -5 * time.Hour, false, "-3"},
		{"at UTC+9", 9 * time.Hour, true, "11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bare, err := types.NewDateTime("2020-01-01T00:00:00.0")
			if err != nil {
				t.Fatalf("NewDateTime: %v", err)
			}
			instant, err := types.NewDateTime("2020-01-01T02:00:00Z")
			if err != nil {
				t.Fatalf("NewDateTime: %v", err)
			}

			ctx := eval.NewContext([]byte(`{}`))
			ctx.SetVariable("bare", types.Collection{bare.WithDefaultOffset(tt.offset)})
			ctx.SetVariable("instant", types.Collection{instant})

			answer := func(expr string) string {
				t.Helper()

				result, err := MustCompile(expr).EvaluateWithContext(ctx)
				if err != nil {
					t.Fatalf("%s: %v", expr, err)
				}
				if len(result) != 1 {
					t.Fatalf("%s: got %d results, want 1", expr, len(result))
				}
				return result[0].String()
			}

			if got := answer("%instant > %bare"); got != boolText(tt.afterBare) {
				t.Errorf("%%instant > %%bare = %s, want %s", got, boolText(tt.afterBare))
			}
			if got := answer("%bare.difference(%instant, 'hour')"); got != tt.hoursApart {
				t.Errorf("difference = %s, want %s", got, tt.hoursApart)
			}
			if got := answer("%bare.duration(%instant, 'hour')"); got != tt.hoursApart {
				t.Errorf("duration = %s, want %s", got, tt.hoursApart)
			}
		})
	}
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
