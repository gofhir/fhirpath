package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

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
	withModel := []fhirpath.EvalOption{fhirpath.WithModel(typeRegistryModel{})}

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
			result, err := fhirpath.MustCompile(tc.expr).EvaluateWithOptions(patient, withModel...)
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
		if _, err := fhirpath.MustCompile(expr).EvaluateWithOptions(patient, fhirpath.WithModel(typeRegistryModel{})); err == nil {
			t.Error("expected an error for a profile that cannot be resolved")
		}
	})

	t.Run("from R5 it is empty", func(t *testing.T) {
		result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(patient, fhirpath.WithModel(r5RegistryModel{}))
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

	_, err := fhirpath.MustCompile("name.conformsTo('http://hl7.org/fhir/StructureDefinition/HumanName')").
		EvaluateWithOptions(patient, fhirpath.WithModel(typeRegistryModel{}))
	if err == nil {
		t.Error("expected an error for a multi-element input")
	}
}

// TestFHIREnvironmentVariables covers the environment variables the FHIR spec
// fixes rather than the caller supplying. Five SHALL invariants (age-1, drt-1,
// cnt-3, dis-1, ras-1) compare against %ucum.
func TestFHIREnvironmentVariables(t *testing.T) {
	cases := []struct {
		expr     string
		expected string
	}{
		{"%ucum", "http://unitsofmeasure.org"},
		{"%sct", "http://snomed.info/sct"},
		{"%loinc", "http://loinc.org"},
		{"%'vs-administrative-gender'", "http://hl7.org/fhir/ValueSet/administrative-gender"},
		{"%`ext-patient-birthTime`", "http://hl7.org/fhir/StructureDefinition/patient-birthTime"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			assertStringResult(t, evalOrFatal(t, simpleJSON, tc.expr), tc.expected)
		})
	}

	t.Run("unknown variable still errors", func(t *testing.T) {
		if _, err := fhirpath.Evaluate(simpleJSON, "%nosuchthing"); err == nil {
			t.Error("expected an error for an undefined variable")
		}
	})

	t.Run("caller can override a fixed constant", func(t *testing.T) {
		expr := fhirpath.MustCompile("%ucum")
		result, err := expr.EvaluateWithOptions(simpleJSON,
			fhirpath.WithVariable("ucum", types.Collection{types.NewString("urn:other")}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "urn:other")
	})

	t.Run("ucum comparison decides the invariant", func(t *testing.T) {
		expr := "system.empty() or system = %ucum"
		ucum := []byte(`{"value":30,"system":"http://unitsofmeasure.org","code":"a"}`)
		other := []byte(`{"value":30,"system":"http://example.org","code":"a"}`)
		none := []byte(`{"value":30,"code":"a"}`)

		assertBooleanResult(t, evalOrFatal(t, ucum, expr), true)
		assertBooleanResult(t, evalOrFatal(t, other, expr), false)
		assertBooleanResult(t, evalOrFatal(t, none, expr), true)
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
type r4bModel struct{ typedChildrenModel }

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
		opts []fhirpath.EvalOption
	}{
		{"no model", nil},
		{"R4 model", []fhirpath.EvalOption{fhirpath.WithModel(typedChildrenModel{})}},
		{"R4B model", []fhirpath.EvalOption{fhirpath.WithModel(r4bModel{})}},
	}

	for _, version := range []struct{ name, expr string }{
		{"R4", dom3R4},
		{"R4B", dom3R4B},
		{"R5", dom3R5},
	} {
		for _, model := range preR5 {
			t.Run(version.name+" invariant, "+model.name, func(t *testing.T) {
				caught, err := fhirpath.MustCompile(version.expr).EvaluateWithOptions(orphan, model.opts...)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				assertBooleanResult(t, caught, false)

				passed, err := fhirpath.MustCompile(version.expr).EvaluateWithOptions(referenced, model.opts...)
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
		result, err := fhirpath.MustCompile(dom3R5).EvaluateWithOptions(orphan, fhirpath.WithModel(r5Model{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, false)
	})

	t.Run("an R4 invariant under R5 rules fails loudly", func(t *testing.T) {
		if _, err := fhirpath.MustCompile(dom3R4).EvaluateWithOptions(referenced, fhirpath.WithModel(r5Model{})); err == nil {
			t.Error("expected an error when an R4 expression is evaluated under R5 rules")
		}
	})
}

// Test FHIR-specific functions
func TestFHIRFunctions(t *testing.T) {
	t.Run("extension", func(t *testing.T) {
		patient := []byte(`{
			"resourceType": "Patient",
			"id": "ext-test",
			"extension": [
				{
					"url": "http://example.org/birthPlace",
					"valueString": "Boston"
				},
				{
					"url": "http://example.org/race",
					"valueCode": "white"
				}
			]
		}`)

		result, err := fhirpath.Evaluate(patient, "Patient.extension('http://example.org/birthPlace')")
		if err != nil {
			t.Fatalf("error = %v", err)
		}

		if result.Empty() {
			t.Error("expected extension to be found")
		}
	})

	t.Run("hasExtension", func(t *testing.T) {
		patient := []byte(`{
			"resourceType": "Patient",
			"extension": [{"url": "http://example.org/test", "valueBoolean": true}]
		}`)

		result, err := fhirpath.EvaluateToBoolean(patient, "Patient.hasExtension('http://example.org/test')")
		if err != nil {
			t.Fatalf("error = %v", err)
		}

		if !result {
			t.Error("expected hasExtension to return true")
		}
	})
}
