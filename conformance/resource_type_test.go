package conformance

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4"
	"github.com/gofhir/models/r4b"
	"github.com/gofhir/models/r5"
)

// With the generated R4, R4B and R5 models, a resource in an element declared Resource —
// Bundle.entry.resource, contained — takes the type its resourceType names, and
// an id the model declares as System.String is a String. bdl-11 depends on the
// first: type = 'document' implies entry.first().resource.is(Composition).
func TestAResourceTakesTheTypeItNamesWithTheGeneratedModels(t *testing.T) {
	document := `{"resourceType":"Bundle","type":"document","entry":[` +
		`{"resource":{"resourceType":"Composition","id":"c","status":"final"}},` +
		`{"resource":{"resourceType":"Patient","id":"2020","birthDate":"2020-01-01"}}]}`
	observation := `{"resourceType":"Observation","status":"final",` +
		`"contained":[{"resourceType":"Patient","id":"2020","birthDate":"2020-01-01"}]}`

	tests := []struct {
		resource string
		expr     string
		want     string
	}{
		{document, "type = 'document' implies entry.first().resource.is(Composition)", "[true]"},
		{document, "entry.resource.type().name", "[Composition, Patient]"},
		{document, "entry.resource.ofType(Patient).birthDate.type().name", "[date]"},
		{observation, "contained.type().name", "[Patient]"},
		{observation, "contained.ofType(Patient).count()", "[1]"},
		{observation, "contained.id.type().name", "[String]"},
		{observation, "contained.id.type().namespace", "[System]"},
	}

	for _, model := range []struct {
		name  string
		model fhirpath.Model
	}{{"r4", r4.FHIRPathModel()}, {"r4b", r4b.FHIRPathModel()}, {"r5", r5.FHIRPathModel()}} {
		for _, tt := range tests {
			t.Run(model.name+"/"+tt.expr, func(t *testing.T) {
				result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(tt.resource), fhirpath.WithModel(model.model))
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
				}
			})
		}
	}
}
