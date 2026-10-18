package fhirpath_test

import (
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// A patient with three names, so that a per-name projection has something to
// count and the family names are not all present on every name.
var threeNames = []byte(`{
  "resourceType":"Patient",
  "name":[
    {"use":"official","family":"Chalmers","given":["Peter","James"]},
    {"use":"usual","given":["Jim"]},
    {"use":"maiden","family":"Windsor","given":["Peter","James"]}
  ]
}`)

// The invariants below lock down behavior that FHIR invariant evaluation
// depends on. Each maps to a reported defect: element names colliding with type
// names, the fixed FHIR environment variables, quantity comparison on JSON
// objects, and singleton-to-Boolean coercion.

// FHIR R4 invariant texts, verbatim.
const (
	ref1 = `reference.startsWith('#').not() or (reference.substring(1).trace('url') in %rootResource.contained.id.trace('ids'))`
	rng2 = `low.empty() or high.empty() or (low <= high)`
	age1 = `(code or value.empty()) and (system.empty() or system = %ucum) and (code.empty() or code = 'a' or code = 'mo' or code = 'wk' or code = 'd' or code = 'h' or code = 'min' or code = 's')`
	cnt3 = `(code.exists() or value.empty()) and (system.empty() or system = %ucum) and (code.empty() or code = '1')`
)

func evalOrFatal(t *testing.T, resource []byte, expr string) types.Collection {
	t.Helper()
	result, err := fhirpath.Evaluate(resource, expr)
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", expr, err)
	}
	return result
}

func assertEmptyResult(t *testing.T, result types.Collection, expr string) {
	t.Helper()
	if !result.Empty() {
		t.Errorf("%s: expected empty collection, got %v", expr, result)
	}
}

// Test data
var patientJSON = []byte(`{
	"resourceType": "Patient",
	"id": "123",
	"active": true,
	"name": [
		{
			"use": "official",
			"family": "Doe",
			"given": ["John", "James"]
		},
		{
			"use": "nickname",
			"given": ["Johnny"]
		}
	],
	"birthDate": "1990-01-15",
	"address": [
		{
			"city": "Boston",
			"state": "MA"
		}
	]
}`)

var simpleJSON = []byte(`{
	"value": 42,
	"decimal": 3.14,
	"text": "hello",
	"active": true,
	"items": [1, 2, 3, 4, 5]
}`)

// Test data for polymorphic elements
var observationWithQuantity = []byte(`{
	"resourceType": "Observation",
	"id": "obs1",
	"status": "final",
	"code": {
		"coding": [{
			"system": "http://loinc.org",
			"code": "8867-4"
		}]
	},
	"valueQuantity": {
		"value": 72,
		"unit": "beats/min",
		"system": "http://unitsofmeasure.org",
		"code": "/min"
	}
}`)

var observationWithString = []byte(`{
	"resourceType": "Observation",
	"id": "obs2",
	"status": "final",
	"code": {
		"coding": [{
			"system": "http://loinc.org",
			"code": "8867-4"
		}]
	},
	"valueString": "Normal findings"
}`)

var observationWithCodeableConcept = []byte(`{
	"resourceType": "Observation",
	"id": "obs3",
	"status": "final",
	"code": {
		"coding": [{
			"system": "http://loinc.org",
			"code": "8867-4"
		}]
	},
	"valueCodeableConcept": {
		"coding": [{
			"system": "http://snomed.info/sct",
			"code": "260385009",
			"display": "Negative"
		}],
		"text": "Negative"
	}
}`)

// Helper functions

func assertBooleanResult(t *testing.T, result types.Collection, expected bool) {
	t.Helper()
	if result.Empty() {
		t.Fatalf("expected boolean %v, got empty collection", expected)
	}
	if len(result) != 1 {
		t.Fatalf("expected single value, got %d: %v", len(result), result)
	}
	b, ok := result[0].(types.Boolean)
	if !ok {
		t.Fatalf("expected Boolean, got %s: %v", result[0].Type(), result[0])
	}
	if b.Bool() != expected {
		t.Errorf("expected %v, got %v", expected, b.Bool())
	}
}

func assertIntegerResult(t *testing.T, result types.Collection, expected int64) {
	t.Helper()
	if result.Empty() {
		t.Fatalf("expected integer %d, got empty collection", expected)
	}
	if len(result) != 1 {
		t.Fatalf("expected single value, got %d: %v", len(result), result)
	}
	i, ok := result[0].(types.Integer)
	if !ok {
		t.Fatalf("expected Integer, got %s: %v", result[0].Type(), result[0])
	}
	if i.Value() != expected {
		t.Errorf("expected %d, got %d", expected, i.Value())
	}
}

func assertStringResult(t *testing.T, result types.Collection, expected string) {
	t.Helper()
	if result.Empty() {
		t.Fatalf("expected string '%s', got empty collection", expected)
	}
	if len(result) != 1 {
		t.Fatalf("expected single value, got %d: %v", len(result), result)
	}
	s, ok := result[0].(types.String)
	if !ok {
		t.Fatalf("expected String, got %s: %v", result[0].Type(), result[0])
	}
	if s.Value() != expected {
		t.Errorf("expected '%s', got '%s'", expected, s.Value())
	}
}

// testModel is a minimal Model covering the paths these tests navigate. It keeps
// the engine's own test suite free of a dependency on gofhir/models/rX, while
// mirroring how that model indexes elements: complex types index their own
// elements ("Identifier.system"), backbone elements live under their parent path
// ("Observation.component.code"), and polymorphic elements are indexed under
// their concrete field name ("Observation.valueQuantity").
type typedChildrenModel struct{}

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

func (typedChildrenModel) TypeOf(path string) string { return testTypeOf[path] }

func (typedChildrenModel) ChoiceTypes(path string) []string {
	if path == "Observation.value" || path == "Observation.component.value" {
		return []string{"Quantity", "CodeableConcept", "string", "Range"}
	}
	return nil
}

func (typedChildrenModel) ReferenceTargets(string) []string { return nil }

func (typedChildrenModel) ParentType(t string) string { return testParent[t] }

func (m typedChildrenModel) IsSubtype(child, parent string) bool {
	for t := child; t != ""; t = testParent[t] {
		if t == parent {
			return true
		}
	}
	return false
}

func (typedChildrenModel) ResolvePath(path string) string { return path }

func (typedChildrenModel) IsResource(typeName string) bool {
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
	result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(resource, fhirpath.WithModel(typedChildrenModel{}))
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", expr, err)
	}
	return result
}

// A questionnaire nested three levels deep, which is the shape repeat() exists
// for: the items form a tree of unbounded depth that no fixed path can walk.
var nestedQuestionnaire = []byte(`{
  "resourceType":"Questionnaire",
  "item":[
    {"linkId":"1","item":[
      {"linkId":"1.1","item":[{"linkId":"1.1.1"}]},
      {"linkId":"1.2"}
    ]},
    {"linkId":"2"}
  ]
}`)

func evaluateScalar(t *testing.T, expr string, resource []byte) string {
	t.Helper()

	result, err := fhirpath.MustCompile(expr).Evaluate(resource)
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	if len(result) == 0 {
		return "EMPTY"
	}
	return result[0].String()
}

// typeRegistryModel is a model that can enumerate its types, which the Model
// contract does not require. It stands in for a published model until one
// exposes HasType.
type typeRegistryModel struct{ typedChildrenModel }

func (typeRegistryModel) HasType(name string) bool {
	switch name {
	case "Patient", "Observation", "Person", "HumanName", "Quantity", "Reference",
		"string", "code", "uri", "boolean", "dateTime",
		"Element", "DomainResource", "Resource":
		return true
	}
	return false
}

// r5Model is a testModel that declares itself as R5, which turns on the rules
// that changed in that version.
type r5Model struct{ typedChildrenModel }

func (r5Model) FHIRVersion() string { return "5.0.0" }

// escaped writes a JSON document with a backslash wherever it holds BS, so
// that the escapes under test are in the document and not decoded on the way
// into the source.
func escaped(json string) []byte {
	return []byte(strings.ReplaceAll(json, "BS", string(rune(92))))
}
