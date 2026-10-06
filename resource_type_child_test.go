package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// resourceType is how JSON says which resource an object is, not an element
// of it: no StructureDefinition lists it, and the HL7 validator skips it when
// it reads a resource's children. It was a child here, so a resource had one
// child more than it has, and children().first() on one written with
// resourceType first, as every resource is, was the resource's type name.
func TestResourceTypeIsNotAChild(t *testing.T) {
	observation := []byte(`{"resourceType":"Observation","id":"o1","status":"final",` +
		`"contained":[{"resourceType":"Patient","id":"p"}],"subject":{"reference":"#p"}}`)
	model := &testModel{typeOf: map[string]string{"Observation.subject": "Reference"}}

	for expr, want := range map[string]string{
		"Observation.children().count()":                         "[4]",
		"Observation.children().where($this = 'Observation')":    "[]",
		"Observation.children().first()":                         "[o1]",
		"Observation.descendants().where($this = 'Patient')":     "[]",
		"Observation.contained.children().count()":               "[1]",
		"Observation.descendants().ofType(Patient).id":           "[p]",
		"Observation.contained.ofType(Patient).exists()":         "[true]",
		"Observation.children().select(children()).count() >= 0": "[true]",
	} {
		compiled := fhirpath.MustCompile(expr)
		for how, opts := range map[string][]fhirpath.EvalOption{
			"without a model": nil,
			"with a model":    {fhirpath.WithModel(model)},
		} {
			result, err := compiled.EvaluateWithOptions(observation, opts...)
			if err != nil {
				t.Fatalf("%s %s: %v", expr, how, err)
			}
			if got := result.String(); got != want {
				t.Errorf("%s %s = %s, want %s", expr, how, got, want)
			}
		}
	}
}
