package conformance

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r4b/v2"
	"github.com/gofhir/models/r5/v2"
)

// A primitive written with no value, only the element beside it, is of the
// primitive's type, as in the HL7 validator 6.10.2: "_active" holding only a
// data-absent-reason is a boolean, which is and ofType() find. It was an
// Object, and filters by type left it out. The namespace is FHIR's, as for a
// primitive with a value; the validator answers System there, which
// CONFORMANCE.md records as not followed.
func TestAValuelessPrimitiveIsOfItsTypeWithTheGeneratedModels(t *testing.T) {
	const dar = `{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"unknown"}]}`
	patient := `{"resourceType":"Patient","_active":` + dar + `,"_birthDate":` + dar + `,"_gender":` + dar + `,` +
		`"name":[{"given":[null,"A"],"_given":[` + dar + `,null]}]}`
	tests := []struct{ expr, want string }{
		{"Patient.active is boolean", "[true]"},
		{"Patient.active.ofType(boolean).count()", "[1]"},
		{"Patient.active.as(boolean).count()", "[1]"},
		{"Patient.active.type().name", "[boolean]"},
		{"Patient.birthDate is date", "[true]"},
		{"Patient.gender is code", "[true]"},
		{"Patient.gender is string", "[true]"},
		{"Patient.children().ofType(boolean).count()", "[1]"},
		{"Patient.descendants().ofType(date).count()", "[1]"},
		{"Patient.name.given.ofType(string).count()", "[2]"},
		{"Patient.name.given.first() is string", "[true]"},
		// It still has no value.
		{"Patient.active.hasValue()", "[false]"},
		{"Patient.active.extension.count()", "[1]"},
	}

	// Resource.id and Extension.url have System.String as their type code and
	// name their FHIR type in structuredefinition-fhir-type, which the model
	// hands the engine: Resource.id a string in R4 and an id from R4B, whose
	// definitions say so, Extension.url a uri. Written with no value, such a
	// field is of that type, as the validator has it.
	systemTyped := `{"resourceType":"Patient","_id":` + dar + `,"extension":[{"_url":` + dar + `,"valueString":"x"}]}`
	systemTests := []struct{ expr, r4, later string }{
		{"Patient.id is string", "[true]", "[true]"},
		{"Patient.id is id", "[false]", "[true]"},
		{"Patient.id is String", "[false]", "[false]"},
		{"Patient.extension.url is uri", "[true]", "[true]"},
	}

	for _, model := range []struct {
		name  string
		model fhirpath.Model
	}{{"r4", r4.FHIRPathModel()}, {"r4b", r4b.FHIRPathModel()}, {"r5", r5.FHIRPathModel()}} {
		for _, tt := range tests {
			t.Run(model.name+"/"+tt.expr, func(t *testing.T) {
				result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(patient), fhirpath.WithModel(model.model))
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
				}
			})
		}
		for _, tt := range systemTests {
			t.Run(model.name+"/"+tt.expr, func(t *testing.T) {
				result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(systemTyped), fhirpath.WithModel(model.model))
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
