package fhirpath

import "testing"

// r5Model is a testModel that declares itself as R5, which turns on the rules
// that changed in that version.
type r5Model struct{ testModel }

func (r5Model) FHIRVersion() string { return "5.0.0" }

// TestVersionAwareCastRule covers the `as` singleton rule, which applies from R5
// on and not before.
//
// The two readings both come from HL7 and contradict each other: the language
// specification requires an error when the input holds more than one item, while
// FHIR's own dom-3 invariant is written as %resource.descendants().as(canonical),
// which depends on filtering. The reference validator resolves it by version, and
// so does this engine.
func TestVersionAwareCastRule(t *testing.T) {
	patient := []byte(`{
		"resourceType": "Patient",
		"name": [
			{"use": "official", "family": "Chalmers"},
			{"use": "usual", "family": "Windsor"}
		]
	}`)

	t.Run("pre-R5 filters", func(t *testing.T) {
		// A model that does not declare a version is treated as pre-R5
		result, err := MustCompile("name.as(HumanName).count()").
			EvaluateWithOptions(patient, WithModel(testModel{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 2)

		// And with no model at all
		assertIntegerResult(t, evalOrFatal(t, patient, "name.as(HumanName).count()"), 2)
	})

	t.Run("R5 requires a single item", func(t *testing.T) {
		for _, expr := range []string{
			"name.as(HumanName)",
			"name as HumanName",
		} {
			t.Run(expr, func(t *testing.T) {
				_, err := MustCompile(expr).EvaluateWithOptions(patient, WithModel(r5Model{}))
				if err == nil {
					t.Error("expected an error for a cast over more than one item")
				}
			})
		}
	})

	t.Run("R5 still casts a single item", func(t *testing.T) {
		result, err := MustCompile("name.first().as(HumanName).use").
			EvaluateWithOptions(patient, WithModel(r5Model{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "official")
	})

	t.Run("dom-3 as R4 publishes it keeps working", func(t *testing.T) {
		// The invariant every DomainResource is validated against
		const dom3 = "contained.where((('#'+id in (%resource.descendants().reference | " +
			"%resource.descendants().as(canonical) | %resource.descendants().as(uri) | " +
			"%resource.descendants().as(url))) or descendants().where(reference = '#').exists())" +
			".not()).empty()"

		orphan := []byte(`{"resourceType":"Observation","id":"o1",
			"contained":[{"resourceType":"Patient","id":"p1"}]}`)
		referenced := []byte(`{"resourceType":"Observation","id":"o1",
			"contained":[{"resourceType":"Patient","id":"p1"}],"subject":{"reference":"#p1"}}`)

		assertBooleanResult(t, evalOrFatal(t, orphan, dom3), false)
		assertBooleanResult(t, evalOrFatal(t, referenced, dom3), true)
	})
}
