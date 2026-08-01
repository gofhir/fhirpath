package fhirpath

import "testing"

// TestTemporalPrecisionSemantics covers comparing and equating dates, datetimes
// and times specified to different precisions.
//
// The rule, from the FHIRPath specification: the comparison proceeds precision
// by precision from the most significant down and stops at the first difference;
// if one value has a component the other does not and everything before matched,
// the result is empty, "to indicate that the result of the comparison is
// unknown". Seconds and milliseconds count as a single precision compared as a
// decimal.
//
// The distinction matters for FHIR invariants: a comparison that cannot be
// decided must not silently answer true or false.
func TestTemporalPrecisionSemantics(t *testing.T) {
	t.Run("ordering decides at the first differing precision", func(t *testing.T) {
		cases := map[string]bool{
			"@2018-03-01 > @2018-01-01":                     true,
			"@2018-03-01T10:30:00 > @2018-03-01T10:00:00":   true,
			"@T10:30:00 > @T10:00:00":                       true,
			"@2018-01-01 > @2018-03-01":                     false,
			"@2018-03-01T10:30:00 > @2018-03-01T10:30:00.0": false,
			"@T10:30:00 > @T10:30:00.0":                     false,
		}
		for expr, expected := range cases {
			t.Run(expr, func(t *testing.T) {
				assertBooleanResult(t, evalOrFatal(t, simpleJSON, expr), expected)
			})
		}
	})

	t.Run("ordering is unknown across precisions", func(t *testing.T) {
		// Every shared component matches, so the order cannot be determined
		for _, expr := range []string{
			"@2018-03 > @2018-03-01",
			"@2018-03-01T10 > @2018-03-01T10:30",
			"@T10 > @T10:30",
			"@2018 < @2018-03",
			"@2018-03-01T10:30 >= @2018-03-01T10:30:00",
		} {
			t.Run(expr, func(t *testing.T) {
				assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
			})
		}
	})

	t.Run("equality follows the same rule", func(t *testing.T) {
		assertEmptyResult(t, evalOrFatal(t, simpleJSON, "@2012-01 = @2012"), "@2012-01 = @2012")
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "@2012-01 = @2012-02"), false)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "@2012-01 = @2012-01"), true)
		// Seconds and milliseconds are one precision
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "@T10:30:00 = @T10:30:00.0"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "@T10:30:00 = @T10:30:00.100"), false)
	})

	t.Run("a date compares against a datetime", func(t *testing.T) {
		resource := []byte(`{"resourceType":"Patient","birthDate":"1974-12-25"}`)

		// Decided at the year, long before precision runs out
		assertBooleanResult(t, evalOrFatal(t, resource, "birthDate < @2020-01-01T10:00:00"), true)

		// Same calendar day: the datetime carries a time the date does not, so
		// the order is unknown
		expr := "birthDate != @1974-12-25T12:34:00"
		assertEmptyResult(t, evalOrFatal(t, resource, expr), expr)
	})

	t.Run("offsets are respected", func(t *testing.T) {
		// The same instant written against two offsets
		assertBooleanResult(t, evalOrFatal(t, simpleJSON,
			"@2018-03-01T10:30:00Z = @2018-03-01T12:30:00+02:00"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON,
			"@2018-03-01T10:30:00Z < @2018-03-01T12:30:00+01:00"), true)
	})

	t.Run("date arithmetic accepts both unit systems", func(t *testing.T) {
		cases := map[string]string{
			// Calendar keywords
			"(@1973-12-25 + 1 day).toString()":   "1973-12-26",
			"(@1973-12-25 + 1 week).toString()":  "1974-01-01",
			"(@1973-12-25 + 1 month).toString()": "1974-01-25",
			"(@1973-12-25 + 1 year).toString()":  "1974-12-25",
			// UCUM definite durations up to a week convert exactly
			"(@1973-12-25 + 1 'd').toString()":                    "1973-12-26",
			"(@1973-12-25 + 1 'wk').toString()":                   "1974-01-01",
			"(@1973-12-25T00:00:00.000+10:00 + 1 's').toString()": "1973-12-25T00:00:01.000+10:00",
			// Subtraction
			"(@1974-01-01 - 1 week).toString()": "1973-12-25",
		}
		for expr, expected := range cases {
			t.Run(expr, func(t *testing.T) {
				assertStringResult(t, evalOrFatal(t, simpleJSON, expr), expected)
			})
		}
	})

	t.Run("UCUM years and months need explicit conversion", func(t *testing.T) {
		// A UCUM year is a fixed 365.25 days and a UCUM month 30.44, neither of
		// which is what adding a calendar year or month means. The spec keeps
		// the systems apart rather than silently choosing one.
		for _, expr := range []string{
			"@1973-12-25 + 1 'a'",
			"@1975-12-25 + 1 'a'",
			"@1973-12-25 + 1 'mo'",
		} {
			t.Run(expr, func(t *testing.T) {
				if _, err := Evaluate(simpleJSON, expr); err == nil {
					t.Error("expected an error rather than a silently chosen meaning")
				}
			})
		}
	})

	t.Run("an unrecognized unit is an error, not a no-op", func(t *testing.T) {
		// Previously any unknown unit returned the date unchanged
		if _, err := Evaluate(simpleJSON, "@1973-12-25 + 1 'furlong'"); err == nil {
			t.Error("expected an error for a unit that cannot shift a date")
		}
	})

	t.Run("partial datetime literals are DateTime, not Date", func(t *testing.T) {
		// The grammar allows the T marker with no time after it
		for _, expr := range []string{
			"@2015T.is(DateTime)",
			"@2015-02T.is(DateTime)",
			"@2015-02-04T.is(DateTime)",
		} {
			t.Run(expr, func(t *testing.T) {
				assertBooleanResult(t, evalOrFatal(t, simpleJSON, expr), true)
			})
		}
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "@2015.is(Date)"), true)
	})
}
