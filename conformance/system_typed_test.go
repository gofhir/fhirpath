package conformance

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r4b/v2"
	"github.com/gofhir/models/r5/v2"
)

// Resource.id, Element.id and Extension.url have a System type as their type
// code and name their FHIR type in structuredefinition-fhir-type, which the
// generated models hand the engine. Each case is as the HL7 validator 6.10.2
// answers it, and where fhirpath.js agrees, as both do:
//
//   - one the definitions call a uri or an id is that FHIR type: Extension.url
//     is a uri in every version, Patient.id an id from R4B;
//   - one they call a string stays a System.String, as both engines read it:
//     R4's Patient.id, a primitive's id, a backbone element's id;
//   - a System.String is none of the FHIR types that convert to it, and a uri
//     no string, though both convert to System.String.
//
// A backbone element's id is the one its own path defines: R5 makes
// BackboneElement.id an id and Patient.contact.id a string. And a walk keeps
// typing beneath a path defined elsewhere, R4's PlanDefinition.action.action,
// which dom-3's descendants().as(canonical) relies on.
func TestSystemTypedElementsTakeTheirFHIRType(t *testing.T) {
	patient := `{"resourceType":"Patient","id":"p1",` +
		`"extension":[{"url":"http://example.org/x","valueString":"v"}],` +
		`"name":[{"id":"n1","family":"F"}],"gender":"male","_gender":{"id":"g1"},` +
		`"contact":[{"id":"c1"}]}`
	plan := `{"resourceType":"PlanDefinition","status":"draft","contained":[{"resourceType":"ActivityDefinition","id":"a","status":"draft"}],` +
		`"action":[{"action":[{"action":[{"definitionCanonical":"#a"}]}]}]}`

	tests := []struct {
		resource, expr string
		r4, later      string
	}{
		{patient, "Patient.id.is(System.String)", "[true]", "[false]"},
		{patient, "Patient.id.is(id)", "[false]", "[true]"},
		{patient, "Patient.id.is(uri)", "[false]", "[false]"},
		{patient, "Patient.id.ofType(id).exists()", "[false]", "[true]"},
		{patient, "Patient.id.type().name", "[String]", "[id]"},
		{patient, "Patient.id = 'p1'", "[true]", "[true]"},
		{patient, "Patient.extension.url.is(uri)", "[true]", "[true]"},
		{patient, "Patient.extension.url.is(string)", "[false]", "[false]"},
		{patient, "Patient.extension.url.is(System.String)", "[false]", "[false]"},
		{patient, "Patient.name.id.is(id)", "[false]", "[true]"},
		{patient, "Patient.gender.id.is(System.String)", "[true]", "[true]"},
		{patient, "Patient.gender.id.is(id)", "[false]", "[false]"},
		{patient, "Patient.contact.id.is(id)", "[false]", "[false]"},
		{patient, "Patient.contact.id.is(System.String)", "[true]", "[true]"},
		{plan, "PlanDefinition.descendants().ofType(canonical).count()", "[1]", "[1]"},
		{plan, "PlanDefinition.descendants().where($this = '#a').type().name", "[canonical]", "[canonical]"},
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
				want := tt.later
				if model.name == "r4" {
					want = tt.r4
				}
				if got := result.String(); got != want {
					t.Errorf("%s = %s, want %s", tt.expr, got, want)
				}
			})
		}
	}
}
