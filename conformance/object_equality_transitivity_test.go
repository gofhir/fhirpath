package conformance

import (
	"fmt"
	"testing"
)

// Plan 024: = between objects is transitive, so each shape of the plan's §1
// answers the same whichever path decides it. The pairs are two extensions of a
// Basic; the expected values come from the plan's text (§2, exit criterion 3).
func TestObjectEqualityOfPlan024(t *testing.T) {
	tests := []struct {
		name, a, b string
		eq         bool
		equiv      string // "" where the plan states no answer for ~
	}{
		{"a nested array in another key order", `{"url":"u","a":[[1]],"b":0}`, `{"url":"u","b":0,"a":[[1.0]]}`, true, "true"},
		{"a nested array in one key order", `{"url":"u","a":[[1]],"b":0}`, `{"url":"u","a":[[1.0]],"b":0}`, true, "true"},
		{"a nested array against the item", `{"url":"u","a":[[1]]}`, `{"url":"u","a":[1]}`, false, ""},
		{"a key written twice, the second differs", `{"url":"u","a":"x","a":"y"}`, `{"url":"u","a":"x","a":"z"}`, true, ""},
		{"a key written twice after a null", `{"url":"u","a":null,"a":"x"}`, `{"url":"u","a":null,"a":"y"}`, false, ""},
		{"an element with content beside a value", `{"url":"u","a":"x","_a":{"resourceType":"X"}}`, `{"url":"u","a":"x","_a":{"resourceType":"Y"}}`, true, ""},
		{"a key escaped", `{"url":"u","\u0061":"x"}`, `{"url":"u","a":"x"}`, true, ""},
		{"a number written another way", `{"url":"u","n":1,"m":"x"}`, `{"url":"u","n":1.0,"m":"x"}`, true, ""},
		{"a nested array holding a Quantity", `{"url":"u","a":[[{"value":1,"code":"1"}]]}`, `{"url":"u","a":[[{"value":1.0,"code":"1"}]]}`, true, "true"},
	}
	for _, model := range symmetryModels {
		for _, tt := range tests {
			t.Run(model.name+"/"+tt.name, func(t *testing.T) {
				resource := `{"resourceType":"Basic","extension":[` + tt.a + `,` + tt.b + `]}`
				for _, expr := range []string{"extension[0] = extension[1]", "extension[1] = extension[0]"} {
					if got, want := evaluateWith(t, model.model, resource, expr), fmt.Sprintf("[%v]", tt.eq); got != want {
						t.Errorf("%s over %s = %s, want %s", expr, resource, got, want)
					}
				}
				if tt.equiv == "" {
					return
				}
				for _, expr := range []string{"extension[0] ~ extension[1]", "extension[1] ~ extension[0]"} {
					if got, want := evaluateWith(t, model.model, resource, expr), "["+tt.equiv+"]"; got != want {
						t.Errorf("%s over %s = %s, want %s", expr, resource, got, want)
					}
				}
			})
		}
	}
}
