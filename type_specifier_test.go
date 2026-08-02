package fhirpath

import "testing"

// typeRegistryModel is a model that can enumerate its types, which the Model
// contract does not require. It stands in for a published model until one
// exposes HasType.
type typeRegistryModel struct{ testModel }

func (typeRegistryModel) HasType(name string) bool {
	switch name {
	case "Patient", "Observation", "Person", "HumanName", "Quantity", "Reference",
		"string", "code", "uri", "boolean", "dateTime",
		"Element", "DomainResource", "Resource":
		return true
	}
	return false
}

// TestTypeSpecifierMustResolve covers the rule that a type name has to name a
// type: "A type specifier is an identifier that must resolve to the name of a
// type in a model."
//
// The distinction this draws is the point of it. A specifier that names no type
// is an error; a specifier that names a type the value does not have is simply
// empty. Both look like "no result" from the outside, and conflating them turns
// a typo into a silent empty collection — Patient.gender.as(strng) would quietly
// filter everything out rather than say the name is wrong.
func TestTypeSpecifierMustResolve(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","gender":"male"}`)
	withRegistry := []EvalOption{WithModel(typeRegistryModel{})}

	t.Run("a name that resolves to no type is an error", func(t *testing.T) {
		for _, expr := range []string{
			"Patient.gender.as(string1)",
			"Patient.gender.ofType(string1)",
			"Patient.gender.is(string1)",
			"Patient.as(Patint)",
		} {
			if _, err := MustCompile(expr).EvaluateWithOptions(patient, withRegistry...); err == nil {
				t.Errorf("%s: expected an error for a name that resolves to no type", expr)
			}
		}
	})

	t.Run("a type the value does not have is empty, not an error", func(t *testing.T) {
		// gender is a code, so it is not a uri — but uri is a type, and asking
		// is a legitimate question with a legitimate negative answer
		result, err := MustCompile("Patient.gender.as(uri)").EvaluateWithOptions(patient, withRegistry...)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty, got %v", result)
		}
	})

	t.Run("System types are the language's own, not the model's", func(t *testing.T) {
		// The model has no reason to declare String, and is not asked to
		for _, expr := range []string{
			"Patient.gender.is(System.String)",
			"'x'.is(String)",
			"1.is(Integer)",
		} {
			if _, err := MustCompile(expr).EvaluateWithOptions(patient, withRegistry...); err != nil {
				t.Errorf("%s: unexpected error: %v", expr, err)
			}
		}
	})

	t.Run("valid specifiers keep working", func(t *testing.T) {
		cases := []struct {
			expr string
			want string
		}{
			{"Patient.gender.as(code)", "male"},
			{"Patient.gender.ofType(code).exists()", "true"},
			{"Patient.is(FHIR.Patient)", "true"},
			{"Patient.is(FHIR.`Patient`)", "true"},
		}
		for _, tc := range cases {
			result, err := MustCompile(tc.expr).EvaluateWithOptions(patient, withRegistry...)
			if err != nil {
				t.Errorf("%s: unexpected error: %v", tc.expr, err)
				continue
			}
			got := "EMPTY"
			if len(result) > 0 {
				got = result[0].String()
			}
			if got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		}
	})
}

// TestTypeSpecifierWithoutARegistry checks that a model which cannot enumerate
// its types is not treated as a model in which no type exists.
//
// Conflating the two would make every type specifier an error against such a
// model, which is every model published today.
func TestTypeSpecifierWithoutARegistry(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","gender":"male"}`)

	for _, opts := range [][]EvalOption{nil, {WithModel(testModel{})}} {
		for _, expr := range []string{
			"Patient.gender.as(string1)",
			"Patient.gender.as(code)",
			"Patient.is(FHIR.Patient)",
		} {
			if _, err := MustCompile(expr).EvaluateWithOptions(patient, opts...); err != nil {
				t.Errorf("%s: unexpected error without a type registry: %v", expr, err)
			}
		}
	}
}
