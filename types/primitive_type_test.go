package types

import "testing"

// primitiveTypeOf names the FHIR primitive a declared type or a choice
// element's suffix stands for, and none for a complex type.
func TestPrimitiveTypeOf(t *testing.T) {
	for declared, want := range map[string]string{
		"boolean":                               "boolean",
		"dateTime":                              "dateTime",
		"Boolean":                               "boolean",
		"DateTime":                              "dateTime",
		"Base64Binary":                          "base64Binary",
		"http://hl7.org/fhirpath/System.String": "string",
		"http://hl7.org/fhirpath/System.Quantity": "",
		"http://hl7.org/fhirpath/System.Long":     "",
		"Quantity":                                "",
		"CodeableConcept":                         "",
		"":                                        "",
	} {
		if got := primitiveTypeOf(declared); got != want {
			t.Errorf("primitiveTypeOf(%q) = %q, want %q", declared, got, want)
		}
	}
}
