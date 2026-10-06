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

// An element may have a field named resourceType: R4's
// ExampleScenario.instance.resourceType is a code naming the type of the
// instance the scenario describes, and a child like any other. Only a
// resource's own resourceType is not. A model types the instance a
// BackboneElement; without one nothing tells it from a resource, and it is
// read as the Patient its resourceType names, as is() reads it.
func TestAnElementsResourceTypeFieldIsAChild(t *testing.T) {
	scenario := []byte(`{"resourceType":"ExampleScenario","status":"draft",` +
		`"instance":[{"resourceId":"a","resourceType":"Patient"}]}`)
	model := &testModel{typeOf: map[string]string{
		"ExampleScenario.instance":              "BackboneElement",
		"ExampleScenario.instance.resourceId":   "string",
		"ExampleScenario.instance.resourceType": "code",
	}}

	for expr, want := range map[string]string{
		"ExampleScenario.children().count()":                              "[2]",
		"ExampleScenario.instance.children().count()":                     "[2]",
		"ExampleScenario.descendants().where($this = 'Patient').exists()": "[true]",
		"ExampleScenario.instance.resourceType":                           "[Patient]",
	} {
		result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(scenario, fhirpath.WithModel(model))
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != want {
			t.Errorf("%s with a model = %s, want %s", expr, got, want)
		}
	}
}
