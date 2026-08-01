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

// dom-3 as each FHIR version publishes it. One invariant, three expressions:
// R4B added an id.exists() guard and fixed a clause R4 had duplicated, then R5
// replaced as() with ofType() and reintroduced the duplicate.
const (
	dom3R4  = `contained.where((('#'+id in (%resource.descendants().reference | %resource.descendants().as(canonical) | %resource.descendants().as(uri) | %resource.descendants().as(url))) or descendants().where(reference = '#').exists() or descendants().where(as(canonical) = '#').exists() or descendants().where(as(canonical) = '#').exists()).not()).trace('unmatched', id).empty()`
	dom3R4B = `contained.where(((id.exists() and ('#'+id in (%resource.descendants().reference | %resource.descendants().as(canonical) | %resource.descendants().as(uri) | %resource.descendants().as(url)))) or descendants().where(reference = '#').exists() or descendants().where(as(canonical) = '#').exists() or descendants().where(as(uri) = '#').exists()).not()).trace('unmatched', id).empty()`
	dom3R5  = `contained.where((('#'+id in (%resource.descendants().reference | %resource.descendants().ofType(canonical) | %resource.descendants().ofType(uri) | %resource.descendants().ofType(url))) or descendants().where(reference = '#').exists() or descendants().where(ofType(canonical) = '#').exists() or descendants().where(ofType(canonical) = '#').exists()).not()).trace('unmatched', id).empty()`
)

// r4bModel declares itself as R4B, which is still pre-R5.
type r4bModel struct{ testModel }

func (r4bModel) FHIRVersion() string { return "4.3.0" }

// TestDom3AcrossVersions runs the real invariant from all three versions against
// every model configuration.
//
// This is what the version-aware rule exists for: an engine has to evaluate
// whichever wording the resource's version publishes. The one combination that
// errors is incoherent by construction — an R4 expression under R5 rules — which
// means a server whose StructureDefinitions and model disagree about the
// version. Failing there is the point: a validator that quietly degrades on a
// version mismatch is worse than one that stops.
func TestDom3AcrossVersions(t *testing.T) {
	orphan := []byte(`{"resourceType":"Observation","id":"o1","contained":[{"resourceType":"Patient","id":"p1"}]}`)
	referenced := []byte(`{"resourceType":"Observation","id":"o1","contained":[{"resourceType":"Patient","id":"p1"}],"subject":{"reference":"#p1"}}`)

	preR5 := []struct {
		name string
		opts []EvalOption
	}{
		{"no model", nil},
		{"R4 model", []EvalOption{WithModel(testModel{})}},
		{"R4B model", []EvalOption{WithModel(r4bModel{})}},
	}

	for _, version := range []struct{ name, expr string }{
		{"R4", dom3R4},
		{"R4B", dom3R4B},
		{"R5", dom3R5},
	} {
		for _, model := range preR5 {
			t.Run(version.name+" invariant, "+model.name, func(t *testing.T) {
				caught, err := MustCompile(version.expr).EvaluateWithOptions(orphan, model.opts...)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				assertBooleanResult(t, caught, false)

				passed, err := MustCompile(version.expr).EvaluateWithOptions(referenced, model.opts...)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				assertBooleanResult(t, passed, true)
			})
		}
	}

	t.Run("the R5 invariant evaluates under R5 rules", func(t *testing.T) {
		// R5 rewrote the expression with ofType(), so the stricter rule does not
		// reach it — presumably why it was rewritten
		result, err := MustCompile(dom3R5).EvaluateWithOptions(orphan, WithModel(r5Model{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, false)
	})

	t.Run("an R4 invariant under R5 rules fails loudly", func(t *testing.T) {
		if _, err := MustCompile(dom3R4).EvaluateWithOptions(referenced, WithModel(r5Model{})); err == nil {
			t.Error("expected an error when an R4 expression is evaluated under R5 rules")
		}
	})
}
