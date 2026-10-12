package fhirpath

import (
	"testing"
	"time"
)

// TestDecimalRange covers the bound this engine sets on decimals. FHIRPath
// asks for at least (-10^28+1)/10^8 to (10^28-1)/10^8 and lets an engine go
// further; this one holds up to 1000 digits before the point and none further
// than 1000 places after it. A value beyond that is an overflow, "Operations
// that cause arithmetic overflow or underflow will result in empty", where it
// used to be held exactly: a number written 1E8227822 had eight million
// digits, and each comparison wrote them out.
func TestDecimalRange(t *testing.T) {
	doc := []byte(`{"resourceType":"Observation",
	  "extension":[
	    {"url":"huge","valueDecimal":1E8227822},
	    {"url":"tiny","valueDecimal":1E-8227822},
	    {"url":"widest","valueDecimal":1E999},
	    {"url":"beyond","valueDecimal":1E1000},
	    {"url":"finest","valueDecimal":1E-1000},
	    {"url":"finer","valueDecimal":1E-1001},
	    {"url":"plain","valueDecimal":2.5},
	    {"url":"zero","valueDecimal":0E2000000000}],
	  "valueQuantity":{"value":1E999999,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}`)
	ext := func(url string) string { return "Observation.extension.where(url = '" + url + "').value" }

	start := time.Now()
	for expr, want := range map[string]string{
		// Beyond the bound, a value reads as nothing
		ext("huge") + ".exists()":   "false",
		ext("tiny") + ".exists()":   "false",
		ext("beyond") + ".exists()": "false",
		ext("finer") + ".exists()":  "false",
		// A zero at a scale of two billion is no larger, but writing it out was
		ext("zero") + ".exists()": "false",
		ext("huge") + " = 9":      "EMPTY",
		ext("huge") + " ~ 9":      "false",
		// At the bound, it reads
		ext("widest") + " > 9": "true",
		ext("finest") + " < 1": "true",
		ext("finest") + " > 0": "true",
		// An operation past the bound overflows
		ext("widest") + " * " + ext("widest"):                       "EMPTY",
		ext("widest") + " * 10":                                     "EMPTY",
		ext("widest") + " + 1 > 0":                                  "true",
		ext("plain") + ".round(2147483647)":                         "EMPTY",
		ext("plain") + ".round(2)":                                  "2.5",
		"'1E8227822'.toDecimal()":                                   "EMPTY",
		"'1E8227822'.convertsToDecimal()":                           "false",
		"(" + ext("widest") + " | " + ext("widest") + ").sum() > 0": "true",
		// A FHIR Quantity whose value overflows is a quantity, and nothing to
		// compare
		"Observation.value > 1 'mg'":           "EMPTY",
		"Observation.value = 1 'mg'":           "EMPTY",
		"Observation.value + 1 'mg'":           "EMPTY",
		"Observation.value ~ 1 'mg'":           "false",
		"Observation.value.comparable(1 'mg')": "EMPTY",
		"Observation.value.lowBoundary()":      "EMPTY",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
	// Each of these wrote out millions of digits; together they took minutes
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("took %s", elapsed)
	}
}
