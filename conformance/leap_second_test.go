package conformance

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4/v2"
)

// A FHIR time written with a leap second, which FHIR allows, is read: it is
// after 23:59:59, and a shift leaves a new value, without the element the
// time was read with, whether it shifts by nothing or by a second.
func TestALeapSecondTimeFromTheResource(t *testing.T) {
	data := []byte(`{"resourceType":"Observation","status":"final","code":{"text":"x"},` +
		`"valueTime":"23:59:60","_valueTime":{"id":"e"}}`)
	for expr, want := range map[string]string{
		"Observation.value > @T23:59:59":     "[true]",
		"Observation.value.id":               "[e]",
		"Observation.value + 0 seconds":      "[23:59:60]",
		"(Observation.value + 0 seconds).id": "[]",
		"(Observation.value + 1 second).id":  "[]",
		"Observation.value + 1 second":       "[00:00:01]",
	} {
		result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(data, fhirpath.WithModel(r4.FHIRPathModel()))
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got := result.String(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
