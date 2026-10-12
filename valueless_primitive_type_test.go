package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// A choice element written with no value is of the type its name tells, with
// or without a model, as in the HL7 validator 6.10.2: "_valueString" is a
// string. Without a model a field whose name tells no type, "_active", has
// none to give, and stays an Object.
func TestAValuelessChoiceIsOfTheTypeItsNameTells(t *testing.T) {
	const dar = `{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"unknown"}]}`
	tests := []struct{ resource, expr, want string }{
		{`{"resourceType":"Observation","_valueString":` + dar + `}`, "Observation.value is string", "[true]"},
		{`{"resourceType":"Observation","_valueString":` + dar + `}`, "Observation.value.ofType(string).count()", "[1]"},
		{`{"resourceType":"Observation","_valueString":` + dar + `}`, "Observation.value.ofType(Quantity).count()", "[0]"},
		{`{"resourceType":"Observation","_valueBoolean":` + dar + `}`, "Observation.value.type().name", "[boolean]"},
		{`{"resourceType":"Patient","extension":[{"url":"u","_valueDate":` + dar + `}]}`, "Patient.extension.value is date", "[true]"},
		{`{"resourceType":"Patient","_active":` + dar + `}`, "Patient.active.type().name", "[Object]"},
		// A complex type names no primitive: the element of an invalid
		// "_valueQuantity" is no "quantity".
		{`{"resourceType":"Observation","_valueQuantity":` + dar + `}`, "Observation.value.type().name", "[Object]"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(tt.resource), tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}
		})
	}
}
