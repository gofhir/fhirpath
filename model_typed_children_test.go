package fhirpath

import (
	"testing"

	"github.com/gofhir/fhirpath/types"
)

// testModel is a minimal Model covering the paths these tests navigate. It keeps
// the engine's own test suite free of a dependency on gofhir/models/rX, while
// mirroring how that model indexes elements: complex types index their own
// elements ("Identifier.system"), backbone elements live under their parent path
// ("Observation.component.code"), and polymorphic elements are indexed under
// their concrete field name ("Observation.valueQuantity").
type testModel struct{}

var testTypeOf = map[string]string{
	"Observation.identifier":         "Identifier",
	"Observation.subject":            "Reference",
	"Observation.code":               "CodeableConcept",
	"Observation.valueQuantity":      "Quantity",
	"Observation.component":          "BackboneElement",
	"Observation.component.code":     "CodeableConcept",
	"Observation.component.valueAge": "Age",
	"Observation.extension":          "Extension",
	"Identifier.system":              "uri",
	"Identifier.value":               "string",
	"Reference.reference":            "string",
	"CodeableConcept.coding":         "Coding",
	"Coding.system":                  "uri",
	"Coding.code":                    "code",
	"Quantity.value":                 "decimal",
	"Quantity.system":                "uri",
	"Quantity.code":                  "code",
	"Extension.url":                  "uri",
	"Extension.valueAge":             "Age",
	"Age.value":                      "decimal",
	"Age.code":                       "code",
	"Patient.contained":              "Resource",
	"Patient.birthDate":              "date",
	"Patient.active":                 "boolean",
	"Patient.gender":                 "code",
	"Patient.name":                   "HumanName",
}

var testParent = map[string]string{
	"Age":             "Quantity",
	"Observation":     "DomainResource",
	"Patient":         "DomainResource",
	"Quantity":        "Element",
	"Identifier":      "Element",
	"CodeableConcept": "Element",
	"Coding":          "Element",
	"Reference":       "Element",
	"Extension":       "Element",
	"BackboneElement": "Element",
	"code":            "string",
	"string":          "Element",
	"uri":             "Element",
	"DomainResource":  "Resource",
}

func (testModel) TypeOf(path string) string { return testTypeOf[path] }

func (testModel) ChoiceTypes(path string) []string {
	if path == "Observation.value" || path == "Observation.component.value" {
		return []string{"Quantity", "CodeableConcept", "string", "Range"}
	}
	return nil
}

func (testModel) ReferenceTargets(string) []string { return nil }
func (testModel) ParentType(t string) string       { return testParent[t] }

func (m testModel) IsSubtype(child, parent string) bool {
	for t := child; t != ""; t = testParent[t] {
		if t == parent {
			return true
		}
	}
	return false
}

func (testModel) ResolvePath(path string) string { return path }

func (testModel) IsResource(typeName string) bool {
	return typeName == "Observation" || typeName == "Patient"
}

var typedResource = []byte(`{
	"resourceType": "Observation",
	"status": "final",
	"subject": {"reference": "Patient/1"},
	"valueQuantity": {"value": 10, "unit": "mg", "system": "http://unitsofmeasure.org", "code": "mg"},
	"identifier": [{"system": "http://h.org", "value": "abc"}],
	"code": {"coding": [{"system": "http://loinc.org", "code": "1234-5"}]},
	"component": [{
		"code": {"coding": [{"system": "http://loinc.org", "code": "8480-6"}]},
		"valueAge": {"value": 30, "unit": "a", "code": "a", "system": "http://unitsofmeasure.org"}
	}],
	"extension": [{"url": "http://x.org/ext", "valueAge": {"value": 40, "code": "a"}}]
}`)

func evalWithModel(t *testing.T, resource []byte, expr string) types.Collection {
	t.Helper()
	result, err := MustCompile(expr).EvaluateWithOptions(resource, WithModel(testModel{}))
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", expr, err)
	}
	return result
}

// TestModelDrivenChildTypes covers children()/descendants() resolving types
// through the Model instead of guessing from the JSON shape. Before this,
// Children() rebuilt every child untyped, so the Model was silently discarded
// one level down: descendants().ofType(Identifier) found nothing, and
// ofType(Age) matched every Quantity-shaped object.
func TestModelDrivenChildTypes(t *testing.T) {
	cases := []struct {
		expr     string
		expected int64
	}{
		{"children().ofType(Identifier).count()", 1},
		{"descendants().ofType(Identifier).count()", 1},
		{"descendants().ofType(Reference).count()", 1},
		{"descendants().ofType(CodeableConcept).count()", 2},
		{"descendants().ofType(Coding).count()", 2},
		// Age is a Quantity subtype: valueQuantity + two valueAge
		{"descendants().ofType(Quantity).count()", 3},
		// ...but only the two Age elements are Age
		{"descendants().ofType(Age).count()", 2},
		{"descendants().ofType(Extension).count()", 1},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			assertIntegerResult(t, evalWithModel(t, typedResource, tc.expr), tc.expected)
		})
	}

	t.Run("backbone elements resolve under their parent path", func(t *testing.T) {
		// Observation.component is a BackboneElement, so its children only exist
		// as "Observation.component.code", not "BackboneElement.code"
		assertIntegerResult(t, evalWithModel(t, typedResource,
			"component.children().ofType(CodeableConcept).count()"), 1)
	})

	t.Run("contained resources restart the path at their resourceType", func(t *testing.T) {
		patient := []byte(`{
			"resourceType": "Patient",
			"birthDate": "1980-02-29",
			"contained": [{"resourceType": "Observation", "identifier": [{"system": "http://h.org", "value": "x"}]}]
		}`)
		assertIntegerResult(t, evalWithModel(t, patient,
			"descendants().ofType(Identifier).count()"), 1)
	})

	t.Run("without a model children still work via inference", func(t *testing.T) {
		result, err := Evaluate(typedResource, "descendants().ofType(Coding).count()")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 2)
	})
}

// TestIdentifierIsNotAQuantity covers the structural inference fix: an
// Identifier also carries "value" and "system", but its value is a string.
// Reading it as a Quantity made identifier.ofType(Identifier) return nothing.
func TestIdentifierIsNotAQuantity(t *testing.T) {
	identifier := []byte(`{"system":"http://h.org","value":"abc"}`)

	t.Run("inferred as Identifier, not Quantity", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, identifier, "$this is Identifier"), true)
		assertBooleanResult(t, evalOrFatal(t, identifier, "$this is Quantity"), false)
	})

	t.Run("a real quantity is still a Quantity", func(t *testing.T) {
		quantity := []byte(`{"value":10,"system":"http://unitsofmeasure.org","code":"mg"}`)
		assertBooleanResult(t, evalOrFatal(t, quantity, "$this is Quantity"), true)
	})

	t.Run("ofType finds the Identifier without a model", func(t *testing.T) {
		resource := []byte(`{"resourceType":"Observation","identifier":[{"system":"http://h.org","value":"abc"}]}`)
		assertIntegerResult(t, evalOrFatal(t, resource, "identifier.ofType(Identifier).count()"), 1)
	})
}

// TestDescendantsDepthLimit covers descendants() honoring MaxDepth, which
// EvalOptions documents but the previous unbounded recursion ignored.
func TestDescendantsDepthLimit(t *testing.T) {
	nested := []byte(`{"resourceType":"Observation","a":{"b":{"c":{"d":"deep"}}}}`)

	full := evalOrFatal(t, nested, "descendants().count()")
	shallow, err := MustCompile("descendants().count()").EvaluateWithOptions(nested, WithMaxDepth(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(full) != 1 || len(shallow) != 1 {
		t.Fatalf("expected singleton counts, got %v and %v", full, shallow)
	}
	fullCount := full[0].String()
	shallowCount := shallow[0].String()
	if fullCount == shallowCount {
		t.Errorf("MaxDepth had no effect: both returned %s", fullCount)
	}
	// depth 1 reaches only the resource's direct children: status is absent here,
	// so "a" plus nothing deeper
	assertIntegerResult(t, shallow, 2)
}
