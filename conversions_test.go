package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// TestConversionsAsTheTextHasThem covers what the conversions area of the
// spec cases found (#167). Up to 1.15.1 convertsToX() of an empty input was
// false; toInteger() truncated a Decimal; toDecimal() read '1.', '.5' and
// '1e5'; toTime() read 'T14:34:28'; toQuantity({}) ignored the empty unit;
// toString() of a FHIR Quantity was empty; toDate() and toDateTime() took no
// format; and toLong() did not exist.
func TestConversionsAsTheTextHasThem(t *testing.T) {
	doc := []byte(`{"resourceType":"Observation","status":"final","code":{"text":"x"},` +
		`"valueQuantity":{"value":185,"unit":"lbs","system":"http://unitsofmeasure.org","code":"[lb_av]"}}`)
	for expr, want := range map[string]string{
		// convertsToX() of an empty input is empty
		"{}.convertsToBoolean().empty()":  "true",
		"{}.convertsToInteger().empty()":  "true",
		"{}.convertsToDate().empty()":     "true",
		"{}.convertsToQuantity().empty()": "true",
		// a Decimal does not convert to an Integer
		"1.5.toInteger().empty()":   "true",
		"185.0.convertsToInteger()": "false",
		"'42'.toInteger()":          "42",
		// a decimal string takes the form the regex gives
		"'1.'.toDecimal().empty()":  "true",
		"'.5'.convertsToDecimal()":  "false",
		"'1e5'.toDecimal().empty()": "true",
		"'-1.5'.toDecimal()":        "-1.5",
		// a time string has no T
		"'T14:34:28'.toTime().empty()": "true",
		"'14:34:28'.toTime()":          "14:34:28",
		// an empty unit is no unit to convert to
		"5 'mg'.toQuantity({}).empty()": "true",
		// a FHIR Quantity is written as the quantity it maps to
		"Observation.value.toString()":         "185 '[lb_av]'",
		"Observation.value.convertsToString()": "true",
		// toLong() reads what toInteger() reads
		"'5000000000'.toLong()": "5000000000",
		"'5'.convertsToLong()":  "true",
		// toDate() and toDateTime() by a template
		"'15-01-2024'.toDate('dd-MM-yyyy')":                                     "2024-01-15",
		"'2024/01/15'.toDate('yyyy/MM/dd')":                                     "2024-01-15",
		"'15-01-2024'.toDate('yyyy-MM-dd').empty()":                             "true",
		"'15/01/2024 10:30'.toDateTime('dd/MM/yyyy HH:mm')":                     "2024-01-15T10:30",
		"'01/15/24 02:05 PM'.toDateTime('MM/dd/yy hh:mm a')":                    "2024-01-15T14:05",
		"'2024-01-15 10:30:00.123+0200'.toDateTime('yyyy-MM-dd HH:mm:ss.SSSZ')": "2024-01-15T10:30:00.123+02:00",
		"'15-01-2024'.convertsToDate('dd-MM-yyyy')":                             "true",
		"@2024-01-15.toDate('dd-MM-yyyy')":                                      "2024-01-15",
		"'2024-01-15'.toDate({}).empty()":                                       "true",
		"'Jan 2024'.toDate('MMM yyyy').empty()":                                 "true",
		"'2024-01-15 30'.toDateTime('yyyy-MM-dd mm').empty()":                   "true",
		"'2024 15'.toDate('yyyy dd').empty()":                                   "true",
		"'2024-01-15 12:00'.toDateTime('yyyy-MM-dd hh:mm').empty()":             "true",
		"'2024-01-15 01:00 PM'.toDateTime('yyyy-MM-dd HH:mm a').empty()":        "true",
		"'2024-01-15 22 10'.toDateTime('yyyy-MM-dd HH hh').empty()":             "true",
		"'2024-01-15 12:00 AM'.toDateTime('yyyy-MM-dd hh:mm a')":                "2024-01-15T00:00",
		"'150124'.toDate('ddMMyy')":                                             "2024-01-15",
		"'2024-01-15'.convertsToDate({}).empty()":                               "true",
		"5 'mg'.convertsToQuantity({}).empty()":                                 "true",
		"'+1.5'.toDecimal().toString()":                                         "1.5",
		"'00001.5'.toDecimal().toString()":                                      "1.5",
		"'-0'.toDecimal().toString()":                                           "0",
		"(10 / 2).toInteger().empty()":                                          "true",
		"'5000000000'.toLong() is Integer":                                      "true",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
	for _, expr := range []string{
		"5 'mg'.toQuantity('mg' | 'g')",
		"'2024-01-15'.toDate('yyyy-MM-dd' | 'dd-MM-yyyy')",
		"'2024-01-15'.toDate(5)",
		"5 'mg'.toQuantity(5)",
	} {
		if _, err := fhirpath.Evaluate(doc, expr); err == nil {
			t.Errorf("%s: no error, want one", expr)
		}
	}
}

// TestConvertsToQuantityWithUnit tests convertsToQuantity with optional unit parameter.
func TestConvertsToQuantityWithUnit(t *testing.T) {
	t.Run("quantity without unit arg", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "(10 'kg').convertsToQuantity()")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("quantity with compatible unit", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "(10 'kg').convertsToQuantity('g')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("quantity with incompatible unit", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "(10 'kg').convertsToQuantity('m')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, false)
	})

	t.Run("quantity mg to kg compatible", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "(500 'mg').convertsToQuantity('kg')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("quantity mm to cm compatible", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "(100 'mm').convertsToQuantity('cm')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	// A bare word in a quantity string is a calendar duration keyword, not a UCUM
	// code — the pattern toQuantity() publishes names that group "time". So a
	// UCUM code written without its quotes does not convert at all, which the
	// conformance suite states directly: '1 wk'.convertsToQuantity().not() is
	// true. What holds for wk holds for kg.
	t.Run("a UCUM code written without quotes does not convert", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "'5 kg'.convertsToQuantity('g')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, false)
	})

	t.Run("the same code in quotes converts", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, `'5 \'kg\''.convertsToQuantity('g')`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("string quantity with incompatible unit", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, `'5 \'kg\''.convertsToQuantity('L')`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, false)
	})

	// A number converts to a quantity in the UCUM default unit, '1', which is
	// dimensionless and so convertible to nothing else. The specification works
	// this through: "45.toQuantity('m') // { } empty ; There is no conversion
	// from the UCUM default unit to meters."
	t.Run("a number does not convert into a dimensioned unit", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "(42).convertsToQuantity('kg')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, false)
	})

	t.Run("a number converts to a quantity when no unit is asked for", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "(3.14).convertsToQuantity()")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("time units compatible", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "(60 's').convertsToQuantity('min')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("volume units compatible", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "(1000 'mL').convertsToQuantity('L')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})
}

// TestToQuantityFromString covers the format toQuantity() accepts on a String,
// which the specification gives as a regex:
//
//	(?'value'(\+|-)?\d+(\.\d+)?)\s*('(?'unit'[^']+)'|(?'time'[a-zA-Z]+))?
//
// The group names carry the rule: a quoted unit is a UCUM code, while a bare
// word is a calendar duration keyword. So '4 days' converts and '1 wk' does not,
// even though wk is a perfectly good UCUM code — written without its quotes it
// is being offered as a calendar keyword, and it is not one.
func TestToQuantityFromString(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// The specification's own examples of valid quantity strings
		{"'4 days'.toQuantity()", "4 days"},
		{`'10 \'mm[Hg]\''.toQuantity()`, "10 'mm[Hg]'"},

		// A bare UCUM code is not a calendar keyword
		{"'1 wk'.convertsToQuantity()", "false"},
		{"'5 kg'.convertsToQuantity()", "false"},
		{`'5 \'kg\''.convertsToQuantity()`, "true"},

		// The pattern is anchored: a partial match is not a conversion
		{"'1.a'.convertsToQuantity()", "false"},
		{"'not a quantity'.convertsToQuantity()", "false"},

		// No unit means the UCUM default unit, not a blank one
		{"'1'.toQuantity()", "1 '1'"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestToQuantityFromOtherTypes covers the rest of the list toQuantity() gives,
// and the conversion it performs when a unit is named.
func TestToQuantityFromOtherTypes(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// "the item is a Boolean, where true results in the quantity 1.0 '1',
		// and false results in the quantity 0.0 '1'"
		{"true.toQuantity()", "1.0 '1'"},
		{"false.toQuantity()", "0.0 '1'"},
		{"true.convertsToQuantity()", "true"},

		// "the resulting quantity will have the UCUM default unit ('1')"
		{"42.toQuantity()", "42 '1'"},
		{"3.14.toQuantity()", "3.14 '1'"},

		// The specification's worked examples for the unit argument
		{"52 'cm'.toQuantity('m')", "0.52 'm'"},
		{"45.toQuantity('m')", "EMPTY"}, // no conversion from '1' to meters
		{"24 'm'.toQuantity('kg')", "EMPTY"},
		{"1 'a'.toQuantity('d')", "365.25 'd'"},
		{"1 'wk'.toQuantity('d')", "7 'd'"},

		// A quantity renders as a string, so it converts to one
		{"1 'wk'.convertsToString()", "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestStringToTemporalConversions covers toDateTime, toDate and toTime over
// strings, which is the whole point of those functions: a value read as text
// becomes one the language can compare and shift.
//
// They had been returning the string unchanged, so '2015'.toDateTime() = @2015
// was false — not because the values differ but because a String was being
// compared against a Date.
func TestStringToTemporalConversions(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// Every precision the grammar admits
		{"'2015'.toDateTime() = @2015", "true"},
		{"'2015-02'.toDateTime() = @2015-02", "true"},
		{"'2015-02-04'.toDateTime() = @2015-02-04", "true"},
		{"'2015-02-04T14:34:28'.toDateTime() = @2015-02-04T14:34:28", "true"},
		{"'2015-02-04T14:34:28.123'.toDateTime() = @2015-02-04T14:34:28.123", "true"},
		{"'2015-02-04T14:34:28Z'.toDateTime() = @2015-02-04T14:34:28Z", "true"},
		{"'2015-02-04T14:34:28+10:00'.toDateTime() = @2015-02-04T14:34:28+10:00", "true"},

		// The result is the type the function is named for
		{"'2015'.toDateTime().type().name", "DateTime"},
		{"'2015-02-04'.toDate().type().name", "Date"},
		{"'14:34:28'.toTime().type().name", "Time"},

		// A string that is not one converts to nothing
		{"'not a datetime'.toDateTime().empty()", "true"},
		{"'not a datetime'.convertsToDateTime()", "false"},
		{"'2015-02-04T14:34:28'.convertsToDateTime()", "true"},
		{"'not a time'.convertsToTime()", "false"},

		// A Date converts to a DateTime, keeping its precision
		{"@2015-02-04.toDateTime() = @2015-02-04", "true"},
		{"@2015-02-04T14:34:28.toDateTime().type().name", "DateTime"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}
