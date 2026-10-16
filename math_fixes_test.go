package fhirpath

import (
	"strings"
	"testing"
)

// TestMathAsBothEnginesAndTheTextHaveIt covers what the math area of the spec
// cases found (#167). Up to 1.15.0 a Math function answered empty for what is
// no number and read the first item of an argument of several; the rounding
// functions refused a quantity; log() of 0 was empty; round() of an Integer
// was an Integer; a leading + took anything; + and - kept the left unit; a
// quantity over zero, a calendar duration in * and /, and a special unit in
// arithmetic ended the evaluation or were converted; and * and / gave UCUM's
// base units, 12 'cm' * 3 'cm' as 0.0036 'm2', until gofhir/ucum 4.3.0.
func TestMathAsBothEnginesAndTheTextHaveIt(t *testing.T) {
	doc := []byte(`{"resourceType":"Observation","status":"final","code":{"coding":[{"system":"http://loinc.org","code":"1"}]},` +
		`"valueQuantity":{"value":-5.5,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"},` +
		`"referenceRange":[{"low":{"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}],` +
		`"component":[{"code":{"text":"y"},"_valueInteger":{"extension":[{"url":"http://x","valueString":"z"}]}}]}`)
	for expr, want := range map[string]string{
		// the rounding functions keep a quantity's unit
		"1.1 'cm'.ceiling()":      "2 'cm'",
		"(-1.1 'cm').floor()":     "-2 'cm'",
		"(-1.56 'cm').truncate()": "-1 'cm'",
		"3.14159 'cm'.round(2)":   "3.14 'cm'",
		"Observation.value.abs()": "5.5 'mg'",
		// round() is a Decimal, and rounds left of the point as both engines do
		"1.round() is Decimal": "true",
		"15.round(-1)":         "20",
		// log() of 1's base has no answer
		"16.log(1)": "EMPTY",
		// + and - take the more granular unit, the left on a tie
		"3 'm' + 3 'cm'":  "303 'cm'",
		"1 'g' - 1 'mg'":  "999 'mg'",
		"1 'L' + 1 'dm3'": "2 'L'",
		// * and / keep the operands' units, as 2.0.0's examples give them
		"12 'cm' * 3 'cm'":  "36 'cm2'",
		"3 'cm' * 12 'cm2'": "36 'cm3'",
		"12 'cm2' / 3 'cm'": "4 'cm'",
		"2 'cm' * 2 'm'":    "400 'cm2'",
		// a number beside a quantity is one of unit '1'
		"2 + 2 '1'":  "4 '1'",
		"2 + 2 'mg'": "EMPTY",
		// empty where the units do not combine, or the divisor is zero
		"12 'cm' / 0":      "EMPTY",
		"12 'cm' / 0 'cm'": "EMPTY",
		"12 day * 45 'm'":  "EMPTY",
		"10 'Cel' + 5 'K'": "EMPTY",
		"2 'Cel' * 3 'm'":  "EMPTY",
		// a FHIR Quantity takes the unary operators
		"-Observation.value":            "5.5 'mg'",
		"(+Observation.value).exists()": "true",
		// a quantity of unit '1' scales a calendar duration, as a number does
		"2 day * 3 '1'": "6 day",
		"2 day / 2 '1'": "1 day",
		// a calendar duration with a UCUM one is in the calendar (3.0.0)
		"60 's' + 2 minutes": "180 seconds",
		"1 minute - 30 's'":  "30 seconds",
		"1 day + 1 'h'":      "25 hours",
		// the operand not converted keeps its precision
		"2.00 'mg' + 1 'g'": "1002.00 'mg'",
		// a FHIR Quantity with no value holds no amount
		"Observation.referenceRange.low.abs()":       "EMPTY",
		"Observation.referenceRange.low + 1 'mg'":    "EMPTY",
		"(+Observation.referenceRange.low).exists()": "false",
		// a primitive with no value is empty, as the operators read it
		"Observation.component.value.abs()":      "EMPTY",
		"Observation.component.value.round()":    "EMPTY",
		"2.power(Observation.component.value)":   "EMPTY",
		"2.5.round(Observation.component.value)": "EMPTY",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
	// Logarithms are read from a decimal's digits, not a float64, which holds
	// one far below or above its range as zero or infinity; a rounded quantity
	// is held to the decimals this engine holds, at its natural scale
	tiny := "0." + strings.Repeat("0", 340) + "1"
	nines := strings.Repeat("9", 1000) + ".5"
	for expr, want := range map[string]string{
		tiny + ".log(10)": "-341",
		"1" + strings.Repeat("0", 400) + ".log(10)": "400",
		"(" + nines + " 'mg').ceiling().exists()":   "false",
		"(" + nines + " 'mg').round().exists()":     "false",
		"(1.5 'mg').round(2)":                       "1.5 'mg'",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%.40s = %s, want %s", expr, got, want)
		}
	}
	for _, expr := range []string{
		"'-5'.abs()", "true.ceiling()", "1 'cm'.exp()", "16.log('2')", "2.power(2 | 3)",
		"3.14159.round(1.5)", "0.log(10)", "10.log(0)", "10.log(-2)", "+'a'",
		// a Coding, with a system and a code and no value, is no quantity
		"Observation.code.coding.abs()", "Observation.code.coding + 1", "-Observation.code.coding",
	} {
		if _, err := Evaluate(doc, expr); err == nil {
			t.Errorf("%s: no error, want one", expr)
		}
	}
}
