package conformance

// Verifies the version-aware rules against the published FHIR models, rather
// than against a stand-in. The engine reads the version through the optional
// VersionedModel interface, so this is the check that the two sides actually
// meet.

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4"
	"github.com/gofhir/models/r4b"
	"github.com/gofhir/models/r5"
)

// dom-3 as each version publishes it. One invariant, three wordings: R4B added
// an id.exists() guard and fixed a clause R4 had duplicated, then R5 replaced
// as() with ofType() and reintroduced the duplicate.
const (
	dom3R4  = `contained.where((('#'+id in (%resource.descendants().reference | %resource.descendants().as(canonical) | %resource.descendants().as(uri) | %resource.descendants().as(url))) or descendants().where(reference = '#').exists() or descendants().where(as(canonical) = '#').exists() or descendants().where(as(canonical) = '#').exists()).not()).trace('unmatched', id).empty()`
	dom3R4B = `contained.where(((id.exists() and ('#'+id in (%resource.descendants().reference | %resource.descendants().as(canonical) | %resource.descendants().as(uri) | %resource.descendants().as(url)))) or descendants().where(reference = '#').exists() or descendants().where(as(canonical) = '#').exists() or descendants().where(as(uri) = '#').exists()).not()).trace('unmatched', id).empty()`
	dom3R5  = `contained.where((('#'+id in (%resource.descendants().reference | %resource.descendants().ofType(canonical) | %resource.descendants().ofType(uri) | %resource.descendants().ofType(url))) or descendants().where(reference = '#').exists() or descendants().where(ofType(canonical) = '#').exists() or descendants().where(ofType(canonical) = '#').exists()).not()).trace('unmatched', id).empty()`
)

func TestPublishedModelsDeclareTheirVersion(t *testing.T) {
	for _, tc := range []struct {
		name     string
		model    fhirpath.Model
		expected string
	}{
		{"r4", r4.FHIRPathModel(), "4.0.1"},
		{"r4b", r4b.FHIRPathModel(), "4.3.0"},
		{"r5", r5.FHIRPathModel(), "5.0.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			versioned, ok := tc.model.(fhirpath.VersionedModel)
			if !ok {
				t.Fatalf("%s model does not declare its FHIR version", tc.name)
			}
			if got := versioned.FHIRVersion(); got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

// TestDom3WithPublishedModels evaluates the real invariant of every version
// against every published model.
//
// The combinations that error are the incoherent ones — an R4 wording under R5
// rules — which is a deployment whose StructureDefinitions and model disagree
// about the version. Surfacing that is the point: most paths exist in both
// versions, so a mismatch otherwise shows up only at the edges, well after it
// has started producing wrong verdicts.
func TestDom3WithPublishedModels(t *testing.T) {
	orphan := []byte(`{"resourceType":"Observation","id":"o1","contained":[{"resourceType":"Patient","id":"p1"}]}`)
	referenced := []byte(`{"resourceType":"Observation","id":"o1","contained":[{"resourceType":"Patient","id":"p1"}],"subject":{"reference":"#p1"}}`)

	models := []struct {
		name  string
		model fhirpath.Model
	}{
		{"r4", r4.FHIRPathModel()},
		{"r4b", r4b.FHIRPathModel()},
		{"r5", r5.FHIRPathModel()},
	}

	for _, invariant := range []struct {
		version string
		expr    string
	}{
		{"R4", dom3R4},
		{"R4B", dom3R4B},
		{"R5", dom3R5},
	} {
		for _, m := range models {
			// An as()-based wording cannot be evaluated under R5 rules
			wantError := m.name == "r5" && invariant.version != "R5"

			t.Run(invariant.version+" invariant, "+m.name+" model", func(t *testing.T) {
				expr := fhirpath.MustCompile(invariant.expr)

				caught, err := expr.EvaluateWithOptions(orphan, fhirpath.WithModel(m.model))
				if wantError {
					if err == nil {
						t.Fatal("expected a version mismatch to surface as an error")
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(caught) != 1 || caught[0].String() != "false" {
					t.Errorf("an orphaned contained resource should fail dom-3, got %v", caught)
				}

				passed, err := expr.EvaluateWithOptions(referenced, fhirpath.WithModel(m.model))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if len(passed) != 1 || passed[0].String() != "true" {
					t.Errorf("a referenced contained resource should pass dom-3, got %v", passed)
				}
			})
		}
	}
}

// TestPublishedModelsResolveTypeNames checks the type registry against the
// published models, which is what decides whether a type specifier is refused.
//
// The engine reads this through the optional TypeRegistry interface, so a model
// that does not implement it leaves specifiers taken at face value. This is the
// check that the published models do implement it, and answer as the
// specification's type system requires.
func TestPublishedModelsResolveTypeNames(t *testing.T) {
	for _, m := range []struct {
		name  string
		model fhirpath.Model
	}{
		{"r4", r4.FHIRPathModel()},
		{"r4b", r4b.FHIRPathModel()},
		{"r5", r5.FHIRPathModel()},
	} {
		t.Run(m.name, func(t *testing.T) {
			registry, ok := m.model.(fhirpath.TypeRegistry)
			if !ok {
				t.Fatalf("%s model cannot resolve type names", m.name)
			}

			for _, known := range []string{
				"Patient", "Observation", // resources
				"HumanName", "Quantity", "Reference", // data types
				"string", "code", "dateTime", "boolean", // primitives
				"Element", "Resource", "DomainResource", // the root types, which
				// have no parent and so cannot be recognized by ParentType alone
			} {
				if !registry.HasType(known) {
					t.Errorf("%q should resolve to a type", known)
				}
			}

			for _, unknown := range []string{"string1", "Patint", "Nonsense", ""} {
				if registry.HasType(unknown) {
					t.Errorf("%q should not resolve to a type", unknown)
				}
			}

			// FHIR spells resources capitalized and primitives in lower camel
			// case, and the two are distinct types, so the lookup is not
			// case-insensitive
			for _, wrongCase := range []string{"patient", "STRING", "humanname"} {
				if registry.HasType(wrongCase) {
					t.Errorf("%q should not resolve: type names are case sensitive", wrongCase)
				}
			}
		})
	}
}
