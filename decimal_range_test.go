package fhirpath

import (
	"strings"
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
	    {"url":"beyondCounted","valueDecimal":1000000000000000E985},
	    {"url":"finest","valueDecimal":1E-1000},
	    {"url":"finer","valueDecimal":1E-1001},
	    {"url":"plain","valueDecimal":2.5},
	    {"url":"zero","valueDecimal":0E2000000000},
	    {"url":"zeroWidest","valueDecimal":0E1000},
	    {"url":"zeroBeyond","valueDecimal":0E1001},
	    {"url":"large","valueQuantity":{"value":1E200,"system":"http://unitsofmeasure.org","code":"m"}},
	    {"url":"widestQ","valueQuantity":{"value":1E999,"system":"http://unitsofmeasure.org","code":"m"}}],
	  "valueQuantity":{"value":1E999999,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}`)
	ext := func(url string) string { return "Observation.extension.where(url = '" + url + "').value" }
	widestQ := ext("widestQ")

	start := time.Now()
	for expr, want := range map[string]string{
		// Beyond the bound, a value reads as nothing
		ext("huge") + ".exists()":   "false",
		ext("tiny") + ".exists()":   "false",
		ext("beyond") + ".exists()": "false",
		// 10^1000 written with a coefficient of sixteen digits, counted exactly
		ext("beyondCounted") + ".exists()": "false",
		ext("finer") + ".exists()":         "false",
		// A zero at a scale of two billion is no larger, but writing it out was
		ext("zero") + ".exists()":       "false",
		ext("zeroWidest") + ".exists()": "true",
		ext("zeroBeyond") + ".exists()": "false",
		ext("huge") + " = 9":            "EMPTY",
		ext("huge") + " ~ 9":            "false",
		// At the bound, it reads
		ext("widest") + " > 9": "true",
		ext("finest") + " < 1": "true",
		ext("finest") + " > 0": "true",
		// An operation past the bound overflows
		ext("widest") + " * " + ext("widest"): "EMPTY",
		ext("widest") + " * 10":               "EMPTY",
		ext("widest") + " + 1 > 0":            "true",
		ext("plain") + ".round(2147483647)":   "EMPTY",
		ext("plain") + ".round(2)":            "2.5",
		ext("plain") + ".round(1000)":         "2.5",
		ext("plain") + ".round(1001)":         "EMPTY",
		// A literal beyond the bound overflows as the same text does
		"1" + strings.Repeat("0", 1000) + " > 0":            "EMPTY",
		"1" + strings.Repeat("0", 999) + " > 0":             "true",
		"(1" + strings.Repeat("0", 1000) + " 'g').exists()": "false",
		// A quantity's value overflowing in arithmetic or conversion
		ext("widest") + ".toQuantity() * 10": "EMPTY",
		"1 'Ym'.toQuantity('ym') > 0 'ym'":   "true",
		// Functions computed in float64 overflow where the result is infinite
		"1000.exp()": "EMPTY",
		// Products and quotients of quantities are computed in float64, with
		// each unit's scale; a value past its range has no answer there
		"(" + ext("large") + " * " + ext("large") + ").exists()": "false",
		widestQ + " / " + widestQ:                                "EMPTY",
		"2 'Cel' * 1 'm'":                                        "275.15 'K.m'",
		// A value too small for float64 is zero there, as it always was
		"(" + ext("finest") + ".toQuantity() * 1 '1') = 0 '1'": "true",
		// A logarithm of infinity over another is no number
		ext("widest") + ".log(" + ext("widest") + ")": "EMPTY",
		// A converted value is held to the range
		widestQ + ".toQuantity('mm')": "EMPTY",
		// A zero at a scale past the bound is zero
		ext("zeroWidest") + " * " + ext("widest") + " = 0":          "true",
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
		"Observation.value * 2":                "EMPTY",
		// Two that overflow compare as nothing, not as an error
		"Observation.value > Observation.value": "EMPTY",
		"Observation.value - Observation.value": "EMPTY",
		// A sort meets an overflowing key as an empty one
		"(Observation.value | 1 'mg').sort().count()": "2",
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

// TestRoundNegativePrecision checks the rule round() states: "the number of
// digits of precision must be >= 0 or the evaluation will end and signal an
// error". Rounding to ten million places left of the point also took seconds.
func TestRoundNegativePrecision(t *testing.T) {
	for _, expr := range []string{"2.5.round(-1)", "1.5.round(-10000000)"} {
		if _, err := Evaluate([]byte(`{"resourceType":"Basic"}`), expr); err == nil {
			t.Errorf("%s: expected an error", expr)
		}
	}
}
