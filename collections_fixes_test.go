package fhirpath

import "testing"

// TestCollectionsFixes covers what the collections area of the spec cases
// found (#167): combine() takes 3.0.0's preserveOrder, single() of an empty
// input is empty, and skip() and take() refuse a num of several items, as
// fhirpath.js does and as the text says. Up to 1.14.6 combine() refused a
// second argument, single() ended with an error on an empty input, and skip()
// and take() read the first item of num.
func TestCollectionsFixes(t *testing.T) {
	doc := []byte(`{"resourceType":"Patient","active":true,"name":[{"given":["A","B"]},{"given":["C"]}],"_birthDate":{"extension":[{"url":"http://example.org","valueString":"x"}]}}`)
	for expr, want := range map[string]string{
		"1.combine(2, true).combine(1, true).count()":                            "3",
		"(1 | 2).combine(2 | 3, false).count()":                                  "4",
		"1.combine({}, {}).count()":                                              "1",
		"Patient.name.given.combine(Patient.name.given, Patient.active).count()": "6",
		"1.combine(2, Patient.birthDate).count()":                                "2",
		"{}.single().empty()":                                                    "true",
		"Patient.name[1].family.single().empty()":                                "true",
		"1.single()":                       "1",
		"(1 | 2 | 3).skip(1 | {}).count()": "2",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
	for _, expr := range []string{
		"1.combine(2, true | false)",
		"1.combine(2, 'true')",
		"1.combine(2, 1)",
		"1.combine(2, true, true)",
		"(1 | 2).single()",
		"(1 | 2 | 3).skip(1 | 2)",
		"(1 | 2 | 3).take(1 | 2)",
	} {
		if _, err := Evaluate(doc, expr); err == nil {
			t.Errorf("%s: no error, want one", expr)
		}
	}
}
