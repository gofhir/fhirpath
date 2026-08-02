package fhirpath

import "testing"

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

// TestCalendarDurationEquality covers the two duration systems and where they
// meet.
//
// The calendar has its own lengths — a year is 365 days, a month 30 — which are
// not UCUM's 365.25 and 30.4375. From a week down the two agree by definition
// and convert freely; at a year or a month they do not, so a comparison across
// the systems has no answer rather than a negative one.
//
// Every case below is either an example the specification works through or one
// documented in fhirpath.js, the reference implementation.
func TestCalendarDurationEquality(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// Within the calendar
		{"1 year = 12 months", "true"},
		{"1 year = 365 days", "true"},
		{"1 month = 30 days", "true"},
		{"1 week = 7 days", "true"},
		{"1 day = 24 hours", "true"},
		{"1 hour = 60 minutes", "true"},
		{"1 minute = 60 seconds", "true"},

		// The chain is not transitive: a year is 365 days directly, not twelve
		// thirty-day months
		{"1 year = 360 days", "false"},

		// Across the systems, at a week and below
		{"1 week = 1 'wk'", "true"},
		{"1 second = 1 's'", "true"},
		{"1 hour = 3600 's'", "true"},
		{"1 'h' = 3600 's'", "true"},

		// Across the systems, at a year or a month: no common unit exists
		{"1 year = 1 'a'", "EMPTY"},
		{"1 year = 12 'mo'", "EMPTY"},
		{"1 month = 30 'd'", "EMPTY"},
		{"1 'a' = 365 days", "EMPTY"},

		// "explicit conversion using toQuantity() will change code-systems to
		// intentionally perform this equality"
		{"1 year.toQuantity('a') = 1 'a'", "true"},

		// Units of different dimensions are not comparable either
		{"1 'cm' = 1 's'", "EMPTY"},
		{"1 'cm' = 10.0 'mm'", "true"},
		{"1 'cm' = 1 'm'", "false"},
		{"23 'Cel' = 73.4 '[degF]'", "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestExplicitDurationConversionUsesSourceSystem covers the rule that decides
// which set of factors an explicit conversion uses.
//
// "When explicitly converting between UCUM definite durations and calendar units
// of differing magnitudes, perform the conversion within the unit system of the
// source, then change the unit to the corresponding target unit."
//
// The two expressions below differ only in whether the source is written as a
// calendar keyword or a UCUM code, and that is what decides the answer.
func TestExplicitDurationConversionUsesSourceSystem(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	// 182.5 days → 0.5 year → 0.5 'a', because the calendar puts 365 days in a year
	if got := evaluateScalar(t, "182.5 days.toQuantity('a')", patient); got != "0.5 'a'" {
		t.Errorf("182.5 days.toQuantity('a') = %s, want 0.5 'a'", got)
	}

	// 182.5 'd' stays in UCUM, where a year is 365.25 days
	got := evaluateScalar(t, "182.5 'd'.toQuantity('a')", patient)
	if got == "0.5 'a'" {
		t.Error("182.5 'd'.toQuantity('a') should use UCUM's 365.25, not the calendar's 365")
	}

	// 7 days → 1 week → 1 'wk'
	if got := evaluateScalar(t, "7 days.toQuantity('wk')", patient); got != "1 'wk'" {
		t.Errorf("7 days.toQuantity('wk') = %s, want 1 'wk'", got)
	}
}
