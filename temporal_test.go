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
