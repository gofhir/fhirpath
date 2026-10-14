package fhirpath

import "testing"

// where(), exists() and all() end with an error when their criteria evaluates
// to more than one item, as the HL7 validator does and as where()'s text says:
// "If the result of evaluating the condition is other than a single boolean
// value, the evaluation will end and signal an error". Empty and a single item
// keep their reading. fhirpath.js reads the first item instead.
func TestCriteriaOfSeveralItemsIsAnError(t *testing.T) {
	doc := []byte(`{"resourceType":"Patient","name":[{"family":"F","given":["A","B"]},{"family":"G"}]}`)
	for _, expr := range []string{
		"(1 | 2).exists(true | false)",
		"(1 | 2).where(true | false)",
		"(1 | 2).all(true | false)",
		"Patient.name.exists(given)",
		"Patient.name.where(given)",
	} {
		if _, err := Evaluate(doc, expr); err == nil {
			t.Errorf("%s: no error, want one for a criteria of several items", expr)
		}
	}
	for expr, want := range map[string]string{
		"(1 | 2).exists(true | true)":                "true",
		"(1 | 2).exists({})":                         "false",
		"(1 | 2).where('a').count()":                 "2",
		"(1 | 2).all({})":                            "false",
		"Patient.name.exists(family = 'F')":          "true",
		"Patient.name.where(given.exists()).count()": "1",
		"Patient.name.all(family.exists())":          "true",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
