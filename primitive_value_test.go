package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// "FHIR.string is a different type to System.String. The FHIR.string type
// specializes FHIR.Element, and has the properties id, extension, and also the
// implicit value property that is actually of type of System.String." value
// navigated to nothing, so DEQM's ra-3, value.startsWith(...) on a valueString,
// could not hold where the HL7 validator holds it. hasValue() and getValue()
// read the same property, of "a single value which is a FHIR primitive".
//
// With a model a FHIR primitive is told from a System value by its type.
// Without one a primitive read from the resource cannot be told from a
// literal, and both are taken for FHIR primitives, as is() takes them; where
// the two answers differ, both are given.
func TestAFHIRPrimitiveHasAValueProperty(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","birthDate":"2000-01-01","_birthDate":{"id":"b"},` +
		`"active":true,"name":[{"family":"Abc","given":["X",null],` +
		`"_given":[null,{"extension":[{"url":"u","valueCode":"absent"}]}]}],` +
		`"extension":[{"url":"x","valueString":"MeasureReport.group.where(id=1)"}]}`)
	model := &testModel{
		typeOf: map[string]string{
			"Patient.birthDate":   "date",
			"Patient.active":      "boolean",
			"Patient.name":        "HumanName",
			"HumanName.family":    "string",
			"HumanName.given":     "string",
			"Patient.extension":   "Extension",
			"Extension.url":       "http://hl7.org/fhirpath/System.String",
			"Extension.valueCode": "code",
		},
		choiceTypes: map[string][]string{"Extension.value": {"string", "code"}},
	}

	tests := []struct {
		expr                  string
		withModel, withoutOne string
	}{
		// The value property, as a System value.
		{"Patient.birthDate.value", "[2000-01-01]", "[2000-01-01]"},
		{"Patient.birthDate.value is System.Date", "[true]", "[true]"},
		{"Patient.birthDate.value.type().namespace", "[System]", "[System]"},
		{"Patient.birthDate is FHIR.date", "[true]", "[true]"},
		{"Patient.active.value", "[true]", "[true]"},
		{"Patient.name.family.value.startsWith('A')", "[true]", "[true]"},
		{"Patient.name.family.value is System.String", "[true]", "[true]"},
		// ra-3's shape, on a choice element.
		{"Patient.extension.value.value.startsWith('MeasureReport.group.where(id=')", "[true]", "[true]"},
		// The value is a System value, with no id or extensions of its own.
		{"Patient.birthDate.value.id", "[]", "[]"},
		{"Patient.name.given.value.extension", "[]", "[]"},
		// A position with extensions and no value has no value.
		{"Patient.name.given.value", "[X]", "[X]"},
		{"Patient.name.given.value.count()", "[1]", "[1]"},
		// A System value has no properties.
		{"'abc'.value", "[]", "[abc]"},
		{"Patient.birthDate.value.value", "[]", "[2000-01-01]"},
		{"Patient.extension.url.value", "[]", "[x]"},

		// hasValue() and getValue(): "a single value which is a FHIR primitive,
		// and it has a primitive value".
		{"Patient.name.family.hasValue()", "[true]", "[true]"},
		{"Patient.name.family.getValue()", "[Abc]", "[Abc]"},
		{"Patient.name.given.getValue().is(System.String)", "[]", "[]"},
		{"Patient.name.given.first().getValue().is(System.String)", "[true]", "[true]"},
		{"Patient.name.given.first().getValue().is(FHIR.string)", "[false]", "[true]"},
		{"Patient.name.given.last().hasValue()", "[false]", "[false]"},
		{"Patient.name.given.hasValue()", "[false]", "[false]"},
		{"'abc'.hasValue()", "[false]", "[true]"},
		{"'abc'.getValue()", "[]", "[abc]"},
		{"Patient.extension.url.hasValue()", "[false]", "[true]"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			compiled := fhirpath.MustCompile(tt.expr)
			for _, run := range []struct {
				how  string
				opts []fhirpath.EvalOption
				want string
			}{
				{"with a model", []fhirpath.EvalOption{fhirpath.WithModel(model)}, tt.withModel},
				{"without one", nil, tt.withoutOne},
			} {
				result, err := compiled.EvaluateWithOptions(patient, run.opts...)
				if err != nil {
					t.Fatalf("%s: %v", run.how, err)
				}
				if got := result.String(); got != run.want {
					t.Errorf("%s = %s %s, want %s", tt.expr, got, run.how, run.want)
				}
			}
		})
	}
}
