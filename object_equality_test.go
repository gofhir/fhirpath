package fhirpath

import (
	"fmt"
	"testing"
)

// Complex values compare by their content, apart from layout: "For complex
// types, equality requires all child properties to be equal, recursively."
// Two identical Codings at different depths of an indented document were
// unequal, since the comparison read their bytes, indentation included.
//
// What stays apart under =: arrays in another order, a child only one side
// has, and, by choice, keys in another order and the same number or string
// written differently (1 and 1.0, an escaped character and the character
// itself), which one serializer writes alike within a document. Under ~ each
// child compares by its own equivalence, so keys and repeats in another order,
// a number at another precision and a string in another case or spacing are
// equivalent; a child only one side has, or a different value, is not.

// codingsAt places two Codings at different depths, each written as given.
func codingsAt(first, second string) []byte {
	return []byte(`{
  "resourceType": "Basic",
  "code": {
    "coding": [
      ` + first + `
    ]
  },
  "extension": [{"url": "http://example.org", "valueCoding": ` + second + `}]
}`)
}

func TestObjectEqualityByContent(t *testing.T) {
	indented := `{
        "system": "http://example.org",
        "code": "a"
      }`

	for _, tc := range []struct {
		name, first, second string
		want, equivalent    string
	}{
		{"indentation", indented, `{"system":"http://example.org","code":"a"}`, "true", "true"},
		{"layout around every token", `{ "system" : "http://example.org" , "code" : "a" }`, `{"system":"http://example.org","code":"a"}`, "true", "true"},
		{"nested layout", `{"code":"a","extension":[ {"url":"u", "valueString":"x"} ]}`, `{"code":"a","extension":[{"url":"u","valueString":"x"}]}`, "true", "true"},
		{"identical", `{"code":"a"}`, `{"code":"a"}`, "true", "true"},
		{"key order", `{"system":"http://example.org","code":"a"}`, `{"code":"a","system":"http://example.org"}`, "false", "true"},
		{"array order", `{"code":"a","extension":[{"url":"u1"},{"url":"u2"}]}`, `{"code":"a","extension":[{"url":"u2"},{"url":"u1"}]}`, "false", "true"},
		{"a child on one side", `{"code":"a"}`, `{"code":"a","display":"A"}`, "false", "false"},
		{"an element on one side", `{"code":"a"}`, `{"code":"a","_code":{"extension":[{"url":"u"}]}}`, "false", "false"},
		{"a different value", `{"code":"a"}`, `{"code":"b"}`, "false", "false"},
		{"whitespace inside a string", `{"display":"a b"}`, `{"display":"a  b"}`, "false", "true"},
		{"a number written another way", `{"code":"a","version":1}`, `{"code":"a","version":1.0}`, "false", "true"},
		{"a string in another case", `{"code":"ABC"}`, `{"code":"abc"}`, "false", "true"},
		{"a nested string in another case", `{"code":"a","extension":[{"url":"u","valueString":"X"}]}`, `{"code":"a","extension":[{"url":"u","valueString":"x"}]}`, "false", "true"},
		{"a Boolean", `{"code":"a","userSelected":true}`, `{"code":"a","userSelected":false}`, "false", "false"},
		{"a repeat on one side", `{"code":"a","extension":[{"url":"u"},{"url":"u"}]}`, `{"code":"a","extension":[{"url":"u"}]}`, "false", "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := codingsAt(tc.first, tc.second)
			for expr, want := range map[string]string{
				"Basic.code.coding = Basic.extension.value": tc.want,
				"Basic.extension.value = Basic.code.coding": tc.want,
				"Basic.code.coding ~ Basic.extension.value": tc.equivalent,
				"Basic.extension.value ~ Basic.code.coding": tc.equivalent,
			} {
				if got := evaluateScalar(t, expr, data); got != want {
					t.Errorf("%s = %s, want %s", expr, got, want)
				}
			}
		})
	}
}

// TestObjectContentDecidesCollections covers the operators that compare items
// to find one: union and distinct() keep one of two equal Codings, and in,
// contains, intersect() and exclude() find it, however each is laid out.
func TestObjectContentDecidesCollections(t *testing.T) {
	data := codingsAt(`{
        "system": "http://example.org",
        "code": "a"
      }`, `{"system":"http://example.org","code":"a"}`)

	for expr, want := range map[string]string{
		"(Basic.code.coding | Basic.extension.value).count()":                   "1",
		"(Basic.code.coding.union(Basic.extension.value)).count()":              "1",
		"(Basic.code.coding.combine(Basic.extension.value)).distinct().count()": "1",
		"(Basic.code.coding.combine(Basic.extension.value)).isDistinct()":       "false",
		"Basic.code.coding in Basic.extension.value":                            "true",
		"Basic.extension.value contains Basic.code.coding":                      "true",
		"Basic.code.coding.intersect(Basic.extension.value).count()":            "1",
		"Basic.code.coding.exclude(Basic.extension.value).count()":              "0",
	} {
		if got := evaluateScalar(t, expr, data); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// TestObjectContentDecidesLargeCollections covers union and distinct() over
// many items: a duplicate laid out apart is still one, wherever it stands.
func TestObjectContentDecidesLargeCollections(t *testing.T) {
	names := ""
	for i := range 12 {
		names += fmt.Sprintf(`{"family":"F%d","given":["G%d"]},`, i, i)
	}
	data := []byte(`{"resourceType":"Patient","name":[` + names + `{
      "family": "F3",
      "given": ["G3"]
    },
    {"family":"F7", "given": [ "G7" ]}]}`)

	for expr, want := range map[string]string{
		"Patient.name.count()":                                       "14",
		"Patient.name.distinct().count()":                            "12",
		"Patient.name.isDistinct()":                                  "false",
		"(Patient.name | Patient.name).count()":                      "12",
		"Patient.name.take(12).union(Patient.name.skip(12)).count()": "12",
	} {
		if got := evaluateScalar(t, expr, data); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// TestObjectEquivalenceByChild covers ~ between complex values: "For complex
// types, equivalence requires all child properties to be equivalent,
// recursively." Each child is read as navigation reads it and compared with
// the equivalence of its type.
func TestObjectEquivalenceByChild(t *testing.T) {
	// The case of #121
	coding := []byte(`{"resourceType":"Basic",
	  "code":{"coding":[{"system":"http://x","code":"ABC"}]},
	  "extension":[{"url":"http://e","valueCoding":{"system":"http://x","code":"abc"}}]}`)
	if got := evaluateScalar(t, "Basic.code.coding ~ Basic.extension.value", coding); got != "true" {
		t.Errorf("Codings differing in case: ~ = %s, want true", got)
	}

	obs := []byte(`{"resourceType":"Observation",
	  "valueQuantity":{"value":1.0,"unit":"mg"},
	  "component":[
	    {"valueQuantity":{"value":1.04,"unit":"MG"}},
	    {"valueQuantity":{"value":2,"unit":"mg"}},
	    {"valuePeriod":{"start":"2020-01-01T10:00:00Z"}},
	    {"valuePeriod":{"start":"2020-01-01T10:00:00+00:00"}},
	    {"valuePeriod":{"start":"2020-01-01"}}]}`)
	for expr, want := range map[string]string{
		// A decimal as decimals are equivalent: at the precision of the less
		// precise side, trailing zeroes carrying none
		"Observation.value ~ Observation.component[0].value": "true",
		"Observation.value ~ Observation.component[1].value": "false",
		// A date by its value, not its text; a date at another precision is not
		"Observation.component[2].value ~ Observation.component[3].value": "true",
		"Observation.component[2].value ~ Observation.component[4].value": "false",
	} {
		if got := evaluateScalar(t, expr, obs); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}

	// A primitive array whose extensions stand in the parallel array: the
	// places line up, a missing one matching only another
	names := []byte(`{"resourceType":"Patient","name":[
	  {"given":["Ann",null],"_given":[null,{"extension":[{"url":"u","valueString":"x"}]}]},
	  {"given":["ANN",null],"_given":[null,{"extension":[{"url":"u","valueString":"X"}]}]},
	  {"given":["Ann","Bea"]}]}`)
	for expr, want := range map[string]string{
		"Patient.name[0] ~ Patient.name[1]":  "true",
		"Patient.name[0] ~ Patient.name[2]":  "false",
		"Patient.name[0] !~ Patient.name[2]": "true",
	} {
		if got := evaluateScalar(t, expr, names); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
