package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// A FHIR decimal is a JSON number whose representation is kept, so read as
// decimal any number is a Decimal as it is written. One without a fraction
// was read as an Integer by its shape, so -0 lost its sign: an extension's
// contextInvariant value.toString() = '-0', which the HL7 validator accepts
// on an Observation with valueQuantity.value -0, was rejected.
func TestANumberReadAsDecimalKeepsItsRepresentation(t *testing.T) {
	for _, number := range []string{"-0", "0", "100", "-0.0", "1.50", "1e2"} {
		t.Run(number, func(t *testing.T) {
			col, err := types.JSONToCollectionWithType([]byte(number), "decimal")
			if err != nil {
				t.Fatal(err)
			}
			d, ok := col[0].(types.Decimal)
			if !ok {
				t.Fatalf("%s read as decimal is a %T, want types.Decimal", number, col[0])
			}
			if d.String() != number || d.Type() != "decimal" {
				t.Errorf("%s read as decimal is %s of type %s", number, d.String(), d.Type())
			}

			col, err = types.ReadRootWithType([]byte(number), "decimal")
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := col[0].(types.Decimal); !ok || col[0].String() != number {
				t.Errorf("%s read as a checked root is %T %s", number, col[0], col[0])
			}
		})
	}

	model := &testModel{typeOf: map[string]string{
		"Observation.valueQuantity": "Quantity",
		"Quantity.value":            "decimal",
	}}
	tests := []struct {
		value, expr, want string
	}{
		{"-0", "valueQuantity.value.toString()", "[-0]"},
		{"-0", "valueQuantity.value.toString() = '-0'", "[true]"},
		{"-0", "valueQuantity.value = 0", "[true]"},
		{"100", "valueQuantity.value.toString()", "[100]"},
		{"100", "valueQuantity.value is decimal", "[true]"},
		{"100", "valueQuantity.value.precision()", "[0]"},
		{"100", "valueQuantity.value = 100.0", "[true]"},
		{"100.0", "valueQuantity.value.toString()", "[100.0]"},
	}
	for _, tt := range tests {
		t.Run(tt.value+" "+tt.expr, func(t *testing.T) {
			obs := []byte(`{"resourceType":"Observation","valueQuantity":{"value":` + tt.value + `,"unit":"mg"}}`)
			compiled := fhirpath.MustCompile(tt.expr)

			result, err := compiled.EvaluateWithOptions(obs, fhirpath.WithModel(model))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}

			doc := fhirpath.MustNewDocument(obs)
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
