package fhirpath_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// TestArithmeticEdges covers division by a zero divisor, which is not an error,
// and the types div and mod accept.
func TestArithmeticEdges(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// "12 / 0 // empty ({ })", and the same for div and mod
		{"1 / 0", "EMPTY"},
		{"12 / 0", "EMPTY"},
		{"5 div 0", "EMPTY"},
		{"5 mod 0", "EMPTY"},
		{"1.5 / 0", "EMPTY"},

		// "supported for Integer, Long and Decimal", with the result taking the
		// input's type
		{"5 div 2", "2"},
		{"5.5 div 0.7", "7"},
		{"2.2 div 1.8", "1"},
		{"5 mod 2", "1"},
		{"2.2 mod 1.8", "0.4"},

		// abs() is defined over Quantity too, and keeps the unit
		{"(-5.5 'mg').abs()", "5.5 'mg'"},
		{"(-5).abs()", "5"},
		{"(-5.5).abs()", "5.5"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

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
		// A special unit does not combine: 3.0.0 "Attempting to operate on
		// quantities with invalid or "special" units will result in empty"
		"2 'Cel' * 1 'm'": "EMPTY",
		// A quantity of unit '1' scales exactly, as a number does, where float64
		// held a value this small as zero
		"(" + ext("finest") + ".toQuantity() * 1 '1') = " + ext("finest") + ".toQuantity()": "true",
		// A logarithm is read from a decimal's digits and exponent, not through
		// a float64 that holds the widest as infinity
		ext("widest") + ".log(" + ext("widest") + ")": "1",
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

// TestRoundNegativePrecision checks round() left of the point, as fhirpath.js
// and the HL7 validator both round, though the text says "the number of
// digits of precision must be >= 0 or the evaluation will end and signal an
// error". Rounding to ten million places left of the point took seconds; it
// is zero, found without rounding there.
func TestRoundNegativePrecision(t *testing.T) {
	start := time.Now()
	for expr, want := range map[string]string{
		"15.round(-1)":              "20",
		"125.5.round(-2)":           "100",
		"2.5.round(-1)":             "0",
		"1.5.round(-10000000)":      "0",
		"15.round(-1) is Decimal":   "true",
		"1.5 'cm'.round(-10000000)": "0 'cm'",
	} {
		if got := evaluateScalar(t, expr, simpleJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %s", elapsed)
	}
}

func TestArithmeticOperators(t *testing.T) {
	t.Run("addition integers", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "2 + 3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 5)
	})

	t.Run("subtraction", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "10 - 3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 7)
	})

	t.Run("multiplication", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "4 * 5")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 20)
	})

	t.Run("division", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "10 / 4")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Division returns Decimal
		if result.Empty() || result[0].Type() != "Decimal" {
			t.Errorf("expected Decimal, got %v", result)
		}
	})

	t.Run("integer division", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "10 div 3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 3)
	})

	t.Run("modulo", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "10 mod 3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 1)
	})

	t.Run("negation", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "-5")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, -5)
	})

	t.Run("string concatenation with +", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "'hello' + ' world'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "hello world")
	})

	t.Run("string concatenation with &", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "'hello' & ' world'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "hello world")
	})
}

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
		if _, err := fhirpath.Evaluate(doc, expr); err == nil {
			t.Errorf("%s: no error, want one", expr)
		}
	}
}

// "Operations that cause arithmetic overflow or underflow will result in empty
// ({ })", FHIRPath 2.0.0 and 3.0.0. An Integer is held in 64 bits here, and
// arithmetic that crosses either end of that range answers empty, where it
// wrapped: 9223372036854775807 + 1 was -9223372036854775808. The ends
// themselves are values. ceiling(), floor() and truncate() read the decimal
// itself, where a float held 9007199254740993 as 9007199254740994.
func TestIntegerArithmeticPastTheRangeIsEmpty(t *testing.T) {
	for expr, want := range map[string]string{
		"9223372036854775807 + 1":                                "[]",
		"-9223372036854775807 - 2":                               "[]",
		"9223372036854775807 * 2":                                "[]",
		"4611686018427387904 * 2":                                "[]",
		"(-9223372036854775807 - 1) div -1":                      "[]",
		"-(-9223372036854775807 - 1)":                            "[]",
		"(-9223372036854775807 - 1).abs()":                       "[]",
		"(9223372036854775807 | 1).sum()":                        "[]",
		"(9223372036854775807 | 1).aggregate($this + $total, 0)": "[]",
		"(9223372036854775807 + 1).exists()":                     "[false]",
		"1000000000000000000000000000000.5.floor()":              "[]",
		"(-1000000000000000000000000000000.5).ceiling()":         "[]",
		// The ends of the range are values
		"9223372036854775806 + 1":           "[9223372036854775807]",
		"-9223372036854775807 - 1":          "[-9223372036854775808]",
		"-4611686018427387904 * 2":          "[-9223372036854775808]",
		"(-9223372036854775807 - 1) mod -1": "[0]",
		"(9223372036854775806 | 1).sum()":   "[9223372036854775807]",
		// Whole numbers read from the decimal, not through a float
		"9007199254740993.5.floor()":    "[9007199254740993]",
		"9007199254740993.5.truncate()": "[9007199254740993]",
		"9007199254740993.5.ceiling()":  "[9007199254740994]",
		"(-2.5).floor()":                "[-3]",
		"(-2.5).ceiling()":              "[-2]",
		"(-2.5).truncate()":             "[-2]",
		// div and mod over decimals truncate the decimal itself
		"100000000000000000000.0 div 1.0": "[100000000000000000000]",
		"100000000000000000000.5 mod 1.0": "[0.5]",
		"7.5 mod 2":                       "[1.5]",
		// A Decimal does not convert to an Integer at all: "the item is an
		// Integer ... a String ... a Boolean"
		"100000000000000000000.0.toInteger()":         "[]",
		"100000000000000000000.0.convertsToInteger()": "[false]",
		"9223372036854775807.0.convertsToInteger()":   "[false]",
		// The quotient is exact, not a division rounded first
		"0.99999999999999999 div 1.0":       "[0]",
		"0.99999999999999999 mod 1.0":       "[0.99999999999999999]",
		"-7.5 div 2":                        "[-3]",
		"-7.5 mod 2":                        "[-1.5]",
		"7.5 mod -2":                        "[1.5]",
		"9223372036854775807.0.toInteger()": "[]",
	} {
		result, err := fhirpath.Evaluate([]byte(`{"resourceType":"Patient"}`), expr)
		if err != nil {
			t.Errorf("%s: %v", expr, err)
			continue
		}
		if got := result.String(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// Test arithmetic operators
func TestArithmetic(t *testing.T) {
	patient := []byte(`{"resourceType": "Patient"}`)

	tests := []struct {
		expr string
		want string
	}{
		{"2 + 3", "5"},
		{"10 - 4", "6"},
		{"3 * 4", "12"},
		{"15 / 3", "5"},
		{"17 div 5", "3"},
		{"17 mod 5", "2"},
		{"-5", "-5"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate(patient, tt.expr)
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if got := result[0].String(); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

// A FHIR decimal is a JSON number whose representation is kept, so read as
// decimal any number is a Decimal as it is written. One without a fraction
// was read as an Integer by its shape, so -0 lost its sign: an extension's
// contextInvariant value.toString() = '-0', which the HL7 validator accepts
// on an Observation with valueQuantity.value -0, was rejected.
func TestANumberReadAsDecimalKeepsItsRepresentation(t *testing.T) {
	for _, number := range []string{"-0", "0", "100", "-0.0", "1.50", "1e2"} {
		t.Run(number, func(t *testing.T) {
			col, err := types.JSONToCollectionWithType([]byte(number), "decimal")
			if err != nil {
				t.Fatal(err)
			}
			d, ok := col[0].(types.Decimal)
			if !ok {
				t.Fatalf("%s read as decimal is a %T, want types.Decimal", number, col[0])
			}
			if d.String() != number || d.Type() != "decimal" {
				t.Errorf("%s read as decimal is %s of type %s", number, d.String(), d.Type())
			}

			col, err = types.ReadRootWithType([]byte(number), "decimal")
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := col[0].(types.Decimal); !ok || col[0].String() != number {
				t.Errorf("%s read as a checked root is %T %s", number, col[0], col[0])
			}
		})
	}

	// A choice element read without a model takes its type from the variant's
	// key, which spells it Decimal; with one, the variant is read as the model
	// types it.
	extension := []byte(`{"resourceType":"Extension","url":"x","valueDecimal":-0}`)
	variant := &testModel{typeOf: map[string]string{"Extension.valueDecimal": "decimal"}}
	for _, tt := range []struct {
		expr  string
		model fhirpath.EvalOption
	}{
		{"value.toString()", nil},
		{"valueDecimal.toString()", fhirpath.WithModel(variant)},
	} {
		var opts []fhirpath.EvalOption
		if tt.model != nil {
			opts = append(opts, tt.model)
		}
		result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions(extension, opts...)
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != "[-0]" {
			t.Errorf("%s on valueDecimal -0 = %s, want [-0]", tt.expr, got)
		}
	}

	model := &testModel{typeOf: map[string]string{
		"Observation.valueQuantity": "Quantity",
		"Quantity.value":            "decimal",
	}}
	tests := []struct {
		value, expr, want string
	}{
		{"-0", "valueQuantity.value.toString()", "[-0]"},
		{"-0", "valueQuantity.value.toString() = '-0'", "[true]"},
		{"-0", "valueQuantity.value = 0", "[true]"},
		{"100", "valueQuantity.value.toString()", "[100]"},
		{"100", "valueQuantity.value is decimal", "[true]"},
		{"100", "valueQuantity.value.precision()", "[0]"},
		{"100", "valueQuantity.value = 100.0", "[true]"},
		{"100.0", "valueQuantity.value.toString()", "[100.0]"},
	}
	for _, tt := range tests {
		t.Run(tt.value+" "+tt.expr, func(t *testing.T) {
			obs := []byte(`{"resourceType":"Observation","valueQuantity":{"value":` + tt.value + `,"unit":"mg"}}`)
			compiled := fhirpath.MustCompile(tt.expr)

			result, err := compiled.EvaluateWithOptions(obs, fhirpath.WithModel(model))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}

			doc := fhirpath.MustNewDocument(obs)
			for i := 0; i < 2; i++ {
				result, err = doc.EvaluateWithOptions(compiled, fhirpath.WithModel(model))
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s on a Document (pass %d) = %s, want %s", tt.expr, i+1, got, tt.want)
				}
			}
		})
	}
}
