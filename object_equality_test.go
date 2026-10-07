package fhirpath

import (
	"fmt"
	"testing"
)

// Complex values compare by their content, not by how they were written:
// "For complex types, equality requires all child properties to be equal,
// recursively." Two identical Codings at different depths of an indented
// document were unequal, since the comparison read their bytes, indentation
// and key order included. fhirpath.js and the HL7 validator compare content.
//
// What stays apart: arrays in another order, a child only one side has, and,
// by choice, the same number or string written differently (1 and 1.0, µ
// and µ), which one serializer writes alike within a document.

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
		want                string
	}{
		{"indentation", indented, `{"system":"http://example.org","code":"a"}`, "true"},
		{"key order", `{"system":"http://example.org","code":"a"}`, `{"code":"a","system":"http://example.org"}`, "true"},
		{"key order and indentation", indented, `{"code":"a","system":"http://example.org"}`, "true"},
		{"nested key order", `{"code":"a","extension":[{"url":"u","valueString":"x"}]}`, `{"extension":[{"valueString":"x","url":"u"}],"code":"a"}`, "true"},
		{"identical", `{"code":"a"}`, `{"code":"a"}`, "true"},
		{"array order", `{"code":"a","extension":[{"url":"u1"},{"url":"u2"}]}`, `{"code":"a","extension":[{"url":"u2"},{"url":"u1"}]}`, "false"},
		{"a child on one side", `{"code":"a"}`, `{"code":"a","display":"A"}`, "false"},
		{"an element on one side", `{"code":"a"}`, `{"code":"a","_code":{"extension":[{"url":"u"}]}}`, "false"},
		{"a different value", `{"code":"a"}`, `{"code":"b"}`, "false"},
		{"whitespace inside a string", `{"display":"a b"}`, `{"display":"a  b"}`, "false"},
		{"the same letters in other fields", `{"code":"ab","display":"c"}`, `{"code":"ac","display":"b"}`, "false"},
		{"a number written another way", `{"code":"a","version":1}`, `{"code":"a","version":1.0}`, "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := codingsAt(tc.first, tc.second)
			for _, expr := range []string{
				"Basic.code.coding = Basic.extension.value",
				"Basic.code.coding ~ Basic.extension.value",
				"Basic.extension.value = Basic.code.coding",
			} {
				if got := evaluateScalar(t, expr, data); got != tc.want {
					t.Errorf("%s = %s, want %s", expr, got, tc.want)
				}
			}
		})
	}
}

// TestObjectContentDecidesCollections covers the operators that compare items
// to find one: union and distinct() keep one of two equal Codings, and in,
// contains, intersect() and exclude() find it, however each was written.
func TestObjectContentDecidesCollections(t *testing.T) {
	data := codingsAt(`{
        "system": "http://example.org",
        "code": "a"
      }`, `{"code":"a","system":"http://example.org"}`)

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
// enough items that objects are fingerprinted before they are compared: a
// duplicate laid out apart, or with its keys in another order, is still one.
func TestObjectContentDecidesLargeCollections(t *testing.T) {
	names := ""
	for i := 0; i < 12; i++ {
		names += fmt.Sprintf(`{"family":"F%d","given":["G%d"]},`, i, i)
	}
	data := []byte(`{"resourceType":"Patient","name":[` + names + `{
      "family": "F3",
      "given": ["G3"]
    },
    {"given":["G7"],"family":"F7"}]}`)

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
