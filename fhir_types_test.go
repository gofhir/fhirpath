package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// fhirTypesModel is a model that also names the FHIR type of its System-typed
// elements, as the generated R5 model does.
type fhirTypesModel struct {
	*testModel
	fhirTypes map[string]string
}

func (m fhirTypesModel) FHIRTypeOf(path string) string { return m.fhirTypes[path] }

func systemTypedModel() *testModel {
	const system = "http://hl7.org/fhirpath/System.String"
	return &testModel{
		typeOf: map[string]string{
			"Patient.id":        system,
			"Patient.name":      "HumanName",
			"Patient.extension": "Extension",
			"HumanName.id":      system,
			"Extension.url":     system,
		},
		parentType: map[string]string{
			"id":     "string",
			"string": "PrimitiveType",
			"uri":    "PrimitiveType",
		},
	}
}

// A model that implements FHIRTypes has an element whose type code is a
// System type read as the FHIR type it names, unless that is string, which
// stays the System type, as the HL7 validator and fhirpath.js read it. A
// model that does not has such an element read as the System type, which is
// none of the FHIR types that convert to it.
func TestFHIRTypesTypeSystemTypedElements(t *testing.T) {
	resource := []byte(`{"resourceType":"Patient","id":"p1","name":[{"id":"n1","family":"F"}],` +
		`"extension":[{"url":"http://example.org/x","valueString":"v"}]}`)
	named := fhirTypesModel{systemTypedModel(), map[string]string{
		"Patient.id":    "id",
		"HumanName.id":  "string",
		"Extension.url": "uri",
	}}

	for _, tt := range []struct {
		expr          string
		named, system string
	}{
		{"Patient.id.is(id)", "[true]", "[false]"},
		{"Patient.id.is(string)", "[true]", "[true]"},
		{"Patient.id.is(System.String)", "[false]", "[true]"},
		{"Patient.id.type().name", "[id]", "[String]"},
		{"Patient.id = 'p1'", "[true]", "[true]"},
		{"Patient.name.id.is(System.String)", "[true]", "[true]"},
		{"Patient.name.id.is(id)", "[false]", "[false]"},
		{"Patient.extension.url.is(uri)", "[true]", "[false]"},
		{"Patient.extension.url.is(string)", "[false]", "[true]"},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			for _, m := range []struct {
				name  string
				model fhirpath.Model
				want  string
			}{{"named", named, tt.named}, {"system", systemTypedModel(), tt.system}} {
				result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions(resource, fhirpath.WithModel(m.model))
				if err != nil {
					t.Fatalf("%s: %v", m.name, err)
				}
				if got := result.String(); got != m.want {
					t.Errorf("%s: %s = %s, want %s", m.name, tt.expr, got, m.want)
				}
			}
		})
	}
}
