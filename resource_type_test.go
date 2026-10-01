package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// A resource names its own type. An element declared as the abstract Resource
// — Bundle.entry.resource, every contained — says only what it must derive
// from, which is why FHIR writes resourceType on every resource. With a model
// the declared type used to win: entry.resource was a Resource, is Patient was
// false, and bdl-11 failed on every document Bundle.
func TestAResourceTakesTheTypeItNames(t *testing.T) {
	model := &testModel{
		typeOf: map[string]string{
			"Bundle.type":           "code",
			"Bundle.entry":          "BackboneElement",
			"Bundle.entry.resource": "Resource",
			"Observation.contained": "Resource",
			"Observation.status":    "code",
			"Patient.id":            "http://hl7.org/fhirpath/System.String",
			"Patient.birthDate":     "date",
		},
		parentType: map[string]string{"Patient": "DomainResource", "DomainResource": "Resource"},
		resources:  map[string]bool{"Bundle": true, "Observation": true, "Patient": true, "Composition": true},
	}

	bundle := `{"resourceType":"Bundle","type":"document","entry":[{"resource":` +
		`{"resourceType":"Patient","id":"2020","birthDate":"2020-01-01"}}]}`
	observation := `{"resourceType":"Observation","status":"final",` +
		`"contained":[{"resourceType":"Patient","id":"2020","birthDate":"2020-01-01"}]}`

	tests := []struct {
		resource string
		expr     string
		want     string
	}{
		{bundle, "entry.resource.type().name", "[Patient]"},
		{bundle, "entry.resource is Patient", "[true]"},
		{bundle, "entry.resource is Resource", "[true]"},
		{bundle, "entry.resource.as(Patient).count()", "[1]"},
		{bundle, "entry.resource.ofType(Patient).count()", "[1]"},
		{bundle, "entry.resource.ofType(Patient).birthDate.type().name", "[date]"},
		{bundle, "entry.first().resource.is(Patient)", "[true]"},
		{observation, "contained.type().name", "[Patient]"},
		{observation, "contained.birthDate.type().name", "[date]"},
		{observation, "contained.descendants().where($this is FHIR.date).count()", "[1]"},
		// An id the model declares a System.String is a String, not the Date
		// "2020" would be guessed as, and its type is named as a System type.
		{observation, "contained.id.type().name", "[String]"},
		{observation, "contained.id.type().namespace", "[System]"},
		{observation, "contained.id = '2020'", "[true]"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			compiled := fhirpath.MustCompile(tt.expr)
			result, err := compiled.EvaluateWithOptions([]byte(tt.resource), fhirpath.WithModel(model))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}

			doc := fhirpath.MustNewDocument([]byte(tt.resource))
			for i := 0; i < 2; i++ {
				result, err = doc.EvaluateWithOptions(compiled, fhirpath.WithModel(model))
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s on a Document (pass %d) = %s, want %s", tt.expr, i+1, got, tt.want)
				}
			}
		})
	}
}
