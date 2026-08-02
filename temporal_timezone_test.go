package fhirpath

import "testing"

// TestDateTimeComparisonAcrossTimezones covers the condition the specification
// places on comparing datetimes:
//
//	To support comparison of DateTime values, either both values have no
//	timezone offset specified, or both values are converted to a common
//	timezone offset.
//
// Neither holds when one side carries an offset and the other does not, and
// there is nothing to convert the bare value into. The answer is unknown rather
// than false: @2012-04-15T15:00:00Z and @2012-04-15T10:00:00 are the same
// instant at UTC-5 and different ones anywhere else, and the value does not say
// which it meant.
func TestDateTimeComparisonAcrossTimezones(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// One offset, one none
		{"@2012-04-15T15:00:00Z = @2012-04-15T10:00:00", "EMPTY"},
		{"@2012-04-15T15:00:00Z != @2012-04-15T10:00:00", "EMPTY"},
		{"@2012-04-15T15:00:00Z > @2012-04-15T10:00:00", "EMPTY"},
		// Even when the components would agree
		{"@2012-04-15T10:00:00Z = @2012-04-15T10:00:00", "EMPTY"},

		// Both carry one: the specification's own worked examples
		{"@2017-11-05T01:30:00.0-04:00 > @2017-11-05T01:15:00.0-05:00", "false"},
		{"@2017-11-05T01:30:00.0-04:00 < @2017-11-05T01:15:00.0-05:00", "true"},
		{"@2017-11-05T01:30:00.0-04:00 = @2017-11-05T01:15:00.0-05:00", "false"},
		{"@2017-11-05T01:30:00.0-04:00 = @2017-11-05T00:30:00.0-05:00", "true"},

		// Neither carries one
		{"@2012-01-01T10:30 = @2012-01-01T10:30", "true"},
		{"@2012-01-01T10:30 = @2012-01-01T10:31", "false"},
		{"@2012-01-01T10:30:31.0 = @2012-01-01T10:30:31", "true"},

		// A date has no clock, so the rule does not reach it
		{"@2012 = @2012", "true"},
		{"@2012 = @2013", "false"},
		{"@2012-01 = @2012", "EMPTY"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestRegexSingleLineMode covers the mode FHIRPath fixes for regular
// expressions: "should be case-sensitive, use 'single line' mode and allow
// Unicode characters".
//
// Single line mode is what makes . match a newline, which Go leaves off by
// default. Without it, a value that spans lines — an address, a narrative —
// cannot be matched across them.
func TestRegexSingleLineMode(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		{"'A\nB'.matches('A.*B')", "true"},
		{"'A B'.matches('A.*B')", "true"},
		{"'a\nb'.replaceMatches('a.b', 'x')", "x"},

		// The markers still address the whole string, not each line: multi-line
		// is a flag, and single line mode is not it
		{"'abc'.matches('^abc$')", "true"},
		{"'abc\ndef'.matches('^abc$')", "false"},

		// The specification's worked examples
		{"'N8000123123'.matches('^N[0-9]{8}$')", "false"},
		{"'http://fhir.org/guides/cqf/common/Library/FHIR-ModelInfo|4.0.1'.matches('Library')", "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestBooleanAggregatesRequireBooleans covers the input allTrue, anyTrue,
// allFalse and anyFalse are each defined over: "Takes a collection of Boolean
// values".
//
// A non-Boolean item is not a false one. Reading it as false would answer
// (true | 'foo').allTrue() with a confident no, when the collection was never
// the kind of thing the function accepts.
func TestBooleanAggregatesRequireBooleans(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	t.Run("a non-Boolean item is an error", func(t *testing.T) {
		for _, expr := range []string{
			"(true | 'foo').allTrue()",
			"(true | 'foo').anyTrue()",
			"(false | 1).allFalse()",
			"(false | 1).anyFalse()",
		} {
			if _, err := MustCompile(expr).Evaluate(patient); err == nil {
				t.Errorf("%s: expected an error", expr)
			}
		}
	})

	t.Run("collections of Booleans still answer", func(t *testing.T) {
		cases := []struct{ expr, want string }{
			{"(true | true).allTrue()", "true"},
			{"(true | false).allTrue()", "false"},
			{"(true | false).anyTrue()", "true"},
			{"(false | false).allFalse()", "true"},
			{"(true | false).anyFalse()", "true"},
			{"{}.allTrue()", "true"},
			{"{}.anyTrue()", "false"},
		}
		for _, tc := range cases {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		}
	})
}

// TestIifTakesASingleItemContext covers the arity rule iif states for itself:
// "Unlike most other functions it can be called with no context ... or with a
// single item context. If the input collection contains multiple items, the
// evaluation of the expression will end and signal an error."
func TestIifTakesASingleItemContext(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	if _, err := MustCompile("('item1' | 'item2').iif(true, 'yes', 'no')").Evaluate(patient); err == nil {
		t.Error("expected an error for a multi-item context")
	}

	for _, tc := range []struct{ expr, want string }{
		{"iif(true, 'yes', 'no')", "yes"},     // no context
		{"'x'.iif(true, 'yes', 'no')", "yes"}, // a single item
		{"{}.iif(true, 'yes', 'no')", "yes"},  // empty: the criterion still decides
		{"iif(false, 'yes', 'no')", "no"},
	} {
		if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
			t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
		}
	}
}
