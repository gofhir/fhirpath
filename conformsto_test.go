package fhirpath

import "testing"

// r5RegistryModel declares itself as R5, where an unresolvable profile is empty
// rather than an error.
type r5RegistryModel struct{ typeRegistryModel }

func (r5RegistryModel) FHIRVersion() string { return "5.0.0" }

// TestConformsToBaseProfiles covers the profiles that can be answered without a
// validator.
//
// The canonical URL of a resource type names a structure the model already
// knows, and conforming to it is being of that type — or of one derived from it,
// since a Patient is a DomainResource.
func TestConformsToBaseProfiles(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[{"given":["Jim"]}]}`)
	withModel := []EvalOption{WithModel(typeRegistryModel{})}

	cases := []struct {
		expr string
		want string
	}{
		{"conformsTo('http://hl7.org/fhir/StructureDefinition/Patient')", "true"},
		{"conformsTo('http://hl7.org/fhir/StructureDefinition/DomainResource')", "true"},
		{"conformsTo('http://hl7.org/fhir/StructureDefinition/Person')", "false"},
		{"name.conformsTo('http://hl7.org/fhir/StructureDefinition/HumanName')", "true"},

		// "If the input is empty, the result is empty"
		{"{}.conformsTo('http://hl7.org/fhir/StructureDefinition/Patient')", "EMPTY"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			result, err := MustCompile(tc.expr).EvaluateWithOptions(patient, withModel...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := "EMPTY"
			if len(result) > 0 {
				got = result[0].String()
			}
			if got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestConformsToUnresolvableProfile covers the rule that changed between
// versions.
//
// R4: "If the structure cannot be resolved to a valid profile, an error is
// thrown." R5 softened this to an empty result. The engine follows whichever the
// model declares, the same way it does for the as operator.
func TestConformsToUnresolvableProfile(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)
	const expr = "conformsTo('http://trash')"

	t.Run("before R5 it is an error", func(t *testing.T) {
		if _, err := MustCompile(expr).EvaluateWithOptions(patient, WithModel(typeRegistryModel{})); err == nil {
			t.Error("expected an error for a profile that cannot be resolved")
		}
	})

	t.Run("from R5 it is empty", func(t *testing.T) {
		result, err := MustCompile(expr).EvaluateWithOptions(patient, WithModel(r5RegistryModel{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty, got %v", result)
		}
	})
}

// TestConformsToRequiresASingleElement covers the arity rule: "If the input
// contains more than one element, an error is thrown."
func TestConformsToRequiresASingleElement(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[{"given":["Jim"]},{"given":["James"]}]}`)

	_, err := MustCompile("name.conformsTo('http://hl7.org/fhir/StructureDefinition/HumanName')").
		EvaluateWithOptions(patient, WithModel(typeRegistryModel{}))
	if err == nil {
		t.Error("expected an error for a multi-element input")
	}
}
