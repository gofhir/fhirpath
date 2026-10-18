package fhirpath

import (
	"fmt"
	"strings"
	"testing"
)

// Complex values compare by their content, apart from layout: "For complex
// types, equality requires all child properties to be equal, recursively."
// Two identical Codings at different depths of an indented document were
// unequal, since the comparison read their bytes, indentation included.
//
// Under = a child is found by its key, wherever each object writes it, and a
// number or string written differently compares as what it reads (1 and 1.0,
// an escaped character and the character itself), as fhirpath.js and HL7's
// Java engine have it; a null or an empty array is no child, as navigation
// reads it, as the Java engine has it. What stays apart: arrays in another
// order, a child only one side has, and a string in another case. Under ~ each
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
		{"key order", `{"system":"http://example.org","code":"a"}`, `{"code":"a","system":"http://example.org"}`, "true", "true"},
		{"array order", `{"code":"a","extension":[{"url":"u1"},{"url":"u2"}]}`, `{"code":"a","extension":[{"url":"u2"},{"url":"u1"}]}`, "false", "true"},
		{"a child on one side", `{"code":"a"}`, `{"code":"a","display":"A"}`, "false", "false"},
		{"an element on one side", `{"code":"a"}`, `{"code":"a","_code":{"extension":[{"url":"u"}]}}`, "false", "false"},
		{"a different value", `{"code":"a"}`, `{"code":"b"}`, "false", "false"},
		// ~ sets case aside and takes any whitespace character for another,
		// collapsing no run of them, as FHIRPath 3.0.0 and fhirpath.js have it
		{"whitespace inside a string", `{"display":"a b"}`, `{"display":"a  b"}`, "false", "false"},
		{"a number written another way", `{"code":"a","version":1}`, `{"code":"a","version":1.0}`, "true", "true"},
		{"an escaped character and the character", `{"code":"\u00e9"}`, `{"code":"é"}`, "true", "true"},
		{"nested keys in another order", `{"code":"a","extension":[{"url":"u","valueString":"x"}]}`, `{"extension":[{"valueString":"x","url":"u"}],"code":"a"}`, "true", "true"},
		{"a string in another case", `{"code":"ABC"}`, `{"code":"abc"}`, "false", "true"},
		{"a nested string in another case", `{"code":"a","extension":[{"url":"u","valueString":"X"}]}`, `{"code":"a","extension":[{"url":"u","valueString":"x"}]}`, "false", "true"},
		{"a Boolean", `{"code":"a","userSelected":true}`, `{"code":"a","userSelected":false}`, "false", "false"},
		{"a repeat on one side", `{"code":"a","extension":[{"url":"u"},{"url":"u"}]}`, `{"code":"a","extension":[{"url":"u"}]}`, "false", "false"},
		{"a repeat paired twice", `{"code":"a","extension":[{"url":"u"},{"url":"u"}]}`, `{"code":"a","extension":[{"url":"u"},{"url":"v"}]}`, "false", "false"},
		{"spaces inside a string are content", `{"code":"a , b"}`, `{"code":"a,b"}`, "false", "false"},
		{"punctuation inside a string", `{"code":"[ 1 ]"}`, `{"code":"[1]"}`, "false", "false"},
		{"a string and its prefix", `{"code":"ab"}`, `{"code":"a"}`, "false", "false"},
		{"another key", `{"code":"a"}`, `{"display":"a"}`, "false", "false"},
		{"a null child is absent", `{"code":"a","display":null}`, `{"code":"a"}`, "true", "true"},
		{"an empty array is absent", `{"code":"a","extension":[]}`, `{"code":"a"}`, "true", "true"},
		{"an escape against a space", `{"display":"a\nb"}`, `{"display":"A B"}`, "false", "true"},
		{"an escape against its character", `{"display":"a\u0020b"}`, `{"display":"A B"}`, "false", "true"},
		{"a string against a number", `{"code":"a","version":"1"}`, `{"code":"a","version":1}`, "false", "false"},
		{"repeats alike and in another case", `{"code":"a","extension":[{"url":"a"},{"url":"A"}]}`, `{"code":"a","extension":[{"url":"a"},{"url":"a"}]}`, "false", "true"},
		// a key written twice, which no valid resource has, is read where it
		// first stands with a value
		{"a key written twice", `{"code":"a","code":"b"}`, `{"code":"a"}`, "true", "true"},
		{"a key written twice after a null", `{"code":null,"code":"b"}`, `{"code":"b"}`, "true", "true"},
		// a primitive's value and the element beside it pair by position under
		// =, and under ~ as one item, the items in any order (#175)
		{"an element beside the same value", `{"given":["x","y"],"_given":[null,{"id":"e"}]}`, `{"_given":[null,{"id":"e"}],"given":["x","y"]}`, "true", "true"},
		// the Java engine compares a primitive by its value alone and answers
		// true here and for an element against none; see Measured, and not
		// followed in CONFORMANCE.md
		{"an element beside another value", `{"given":["x","y"],"_given":[null,{"id":"e"}]}`, `{"given":["x","y"],"_given":[{"id":"e"},null]}`, "false", "false"},
		{"an element against none", `{"given":["x"],"_given":[{"id":"e"}]}`, `{"given":["x"]}`, "false", "false"},
		{"an element that holds no value", `{"given":["x"],"_given":[{}]}`, `{"given":["x"]}`, "true", "true"},
		{"an item in another case", `{"given":["X","y"],"_given":[{"id":"E"},null]}`, `{"given":["x","y"],"_given":[{"id":"e"},null]}`, "false", "true"},
		// in any order, as this engine compares repeats under ~; fhirpath.js
		// and the Java engine compare them in order and answer false
		{"items in another order", `{"given":["x","y"],"_given":[null,{"id":"e"}]}`, `{"given":["y","x"],"_given":[{"id":"e"},null]}`, "false", "true"},
		{"an element alone against a value", `{"given":[null],"_given":[{"id":"e"}]}`, `{"given":["x"]}`, "false", "false"},
		{"an element on a single value", `{"family":"F","_family":{"id":"e"}}`, `{"_family":{"id":"E"},"family":"f"}`, "false", "true"},
		{"a quantity in an element in another unit", `{"given":["x"],"_given":[{"extension":[{"url":"u","valueQuantity":{"value":1,"unit":"g","system":"http://unitsofmeasure.org","code":"g"}}]}]}`, `{"given":["x"],"_given":[{"extension":[{"url":"u","valueQuantity":{"value":1000,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}]}]}`, "false", "true"},
		// = reads no item there, a question of its own; left unasserted
		{"an empty element beside a null is an item", `{"given":[null,"y"],"_given":[{},null]}`, `{"given":["y"]}`, "", "false"},
		// in any order and in another case; fhirpath.js and the Java engine
		// compare in order and answer false
		{"empty elements beside nulls pair", `{"given":[null,"y"],"_given":[{},null]}`, `{"given":["Y",null],"_given":[null,{}]}`, "false", "true"},
		{"another child beside an element", `{"given":["x"],"_given":[{"id":"e"}],"text":"a"}`, `{"given":["x"],"_given":[{"id":"e"}],"text":"A"}`, "false", "true"},
		{"a child on one side beside an element", `{"given":["x"],"_given":[{"id":"e"}]}`, `{"given":["x"],"_given":[{"id":"e"}],"suffix":["a"]}`, "false", "false"},
		// items written alike pair first, as values alone do: 1 ~ 0.6 holds,
		// rounded to the less precise, and would leave 0.6 without a pair
		{"decimals with elements alike", `{"x":[1,0.6],"_x":[{"id":"e"},{"id":"e"}]}`, `{"x":[0.6,1.4],"_x":[{"id":"e"},{"id":"e"}]}`, "false", "true"},
		{"an element where the value is null", `{"given":[null,"y"],"_given":[{"id":"e"},null]}`, `{"given":["y"],"_given":[{"id":"e"}]}`, "false", "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := codingsAt(tc.first, tc.second)
			for expr, want := range map[string]string{
				"Basic.code.coding = Basic.extension.value": tc.want,
				"Basic.extension.value = Basic.code.coding": tc.want,
				"Basic.code.coding ~ Basic.extension.value": tc.equivalent,
				"Basic.extension.value ~ Basic.code.coding": tc.equivalent,
			} {
				if want == "" {
					continue
				}
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
	    {"valueQuantity":{"value":1.04,"unit":"mg"}},
	    {"valueQuantity":{"value":2,"unit":"mg"}},
	    {"valuePeriod":{"start":"2020-01-01T10:00:00Z"}},
	    {"valuePeriod":{"start":"2020-01-01T10:00:00+00:00"}},
	    {"valuePeriod":{"start":"2020-01-01"}},
	    {"valueQuantity":{"value":1,"system":"http://unitsofmeasure.org","code":"g"}},
	    {"valueQuantity":{"value":1000,"system":"http://unitsofmeasure.org","code":"mg"}},
	    {"valueQuantity":{"value":1,"system":"http://unitsofmeasure.org","code":"ms"}},
	    {"valueQuantity":{"value":1,"system":"http://unitsofmeasure.org","code":"mS"}}]}`)
	for expr, want := range map[string]string{
		// A decimal as decimals are equivalent: at the precision of the less
		// precise side, trailing zeroes carrying none
		"Observation.value ~ Observation.component[0].value": "true",
		"Observation.value ~ Observation.component[1].value": "false",
		// Two FHIR Quantities compare child by child too, as complex values: a
		// value in another unit is not equivalent, and a code is a string, its
		// case set aside even where UCUM tells 'ms' from 'mS'
		// Two FHIR Quantities compare as quantities, as in the HL7 validator
		// 6.10.2: 1 g ~ 1000 mg, and ms (millisecond) is not mS (millisiemens).
		"Observation.component[5].value ~ Observation.component[6].value": "true",
		"Observation.component[7].value ~ Observation.component[8].value": "false",
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
	  {"given":["Ann","Bea"]},
	  {"given":["Ann","Bea"],"_given":[null,{"extension":[{"url":"u","valueString":"x"}]}]}]}`)
	for expr, want := range map[string]string{
		"Patient.name[0] ~ Patient.name[1]":  "true",
		"Patient.name[0] ~ Patient.name[3]":  "false",
		"Patient.name[0] ~ Patient.name[2]":  "false",
		"Patient.name[0] !~ Patient.name[2]": "true",
	} {
		if got := evaluateScalar(t, expr, names); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// TestObjectEquivalenceRepeats covers repeats of objects in another order, each
// read once, and objects of many keys, found by key in another order.
func TestObjectEquivalenceRepeats(t *testing.T) {
	forward, backward := make([]string, 0, 20), make([]string, 0, 20)
	for i := range 20 {
		forward = append(forward, fmt.Sprintf(`"k%d":"Value %d"`, i, i))
		backward = append([]string{fmt.Sprintf(`"k%d":"VALUE %d"`, i, i)}, backward...)
	}
	data := []byte(`{"resourceType":"Basic",
	  "a":{` + strings.Join(forward, ",") + `},
	  "b":{` + strings.Join(backward, ",") + `},
	  "c":{"x":[{"c":"x"},{"c":"y"}]},
	  "d":{"x":[{"c":"Y"},{"c":"X"}]},
	  "e":{"x":[{"c":"X"},{"c":"X"}]}}`)
	for expr, want := range map[string]string{
		// Twenty keys, past the index, in reversed order and another case
		"Basic.a ~ Basic.b": "true",
		// Objects in another order and case pair each with its own
		"Basic.c ~ Basic.d": "true",
		"Basic.c ~ Basic.e": "false",
	} {
		if got := evaluateScalar(t, expr, data); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
