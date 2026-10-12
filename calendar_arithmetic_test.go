package fhirpath

import "testing"

// TestCalendarArithmeticClampsToMonthEnd covers the rule that separates calendar
// arithmetic from adding a fixed number of days.
//
// The specification states it in both the year and the month rows of its
// arithmetic table: "If the month and day of the date or time value is not a
// valid date in the resulting year, the last day of the calendar month is used."
//
// The distinction is easy to lose, because Go's own AddDate normalizes instead —
// it rolls 2017-02-29 forward into March. A year after the 29th of February is
// the 28th, not the 1st of March.
func TestCalendarArithmeticClampsToMonthEnd(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// A leap day, plus a year that has none
		{"@2016-02-29 + 1 year", "2017-02-28"},
		{"@2016-02-29 - 1 year", "2015-02-28"},
		// ...and one that does, four years on
		{"@2016-02-29 + 4 years", "2020-02-29"},

		// The end of a long month, into a shorter one
		{"@2014-01-31 + 1 month", "2014-02-28"},
		{"@2014-03-31 - 1 month", "2014-02-28"},
		{"@2014-01-31 + 3 months", "2014-04-30"},
		{"@2016-01-31 + 1 month", "2016-02-29"},

		// Days are not clamped: they overflow the month, which the same table
		// calls for
		{"@1973-12-25 + 7 days", "1974-01-01"},
		{"@1973-12-25 + 1 week", "1974-01-01"},
		{"@2014-01-31 + 1 day", "2014-02-01"},

		// The specification's own worked example
		{"@2019-03-01 + 24 months", "2021-03-01"},

		// A datetime clamps the same way, keeping its time of day
		{"@2016-02-29T10:30:00 + 1 year", "2017-02-28T10:30:00"},
		{"@2014-01-31T23:59:00 + 1 month", "2014-02-28T23:59:00"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestCalendarArithmeticKeepsPrecision checks that a shift never makes a partial
// date more precise than it was.
func TestCalendarArithmeticKeepsPrecision(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		{"@2014 + 1 year", "2015"},
		{"@2014-01 + 1 month", "2014-02"},
		{"@2014-12 + 1 month", "2015-01"},
		// The specification's example of a quantity finer than the value it
		// shifts: a year-precision date plus two years' worth of months
		{"@2014 + 24 months", "2016"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestCalendarArithmeticTruncatesAFinerQuantity covers a quantity finer than
// the value it shifts:
//
// "For partial date/time values where the time-valued quantity is more precise
// than the partial date/time, the operation is performed by converting the
// time-valued quantity to the highest precision in the partial (truncating any
// decimal fraction) and then adding"
//
// The quantity is truncated before the shift, so a subtraction of less than a
// whole unit of the value leaves it where it was rather than a unit back.
func TestCalendarArithmeticTruncatesAFinerQuantity(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// The specification's examples
		{"@2014 + 24 months", "2016"},
		{"@2014 + 23 months", "2015"},
		{"@2016 + 365 days", "2017"},
		{"@2014 + 11 months", "2014"},
		{"@2026-02 + 5 weeks", "2026-03"},
		{"@2026-02 + 4 weeks", "2026-02"},

		// A year is 365 days and a month 30, whatever the calendar holds
		{"@2016 + 364 days", "2016"},
		{"@2016-01 + 59 days", "2016-02"},
		{"@2016-01 + 60 days", "2016-03"},

		// Subtraction truncates toward zero
		{"@2000 - 1 day", "2000"},
		{"@2014 - 23 months", "2013"},
		{"@2016-03 - 31 days", "2016-02"},
		{"@2000-01-01 - 1 hour", "2000-01-01"},
		{"@2000-01-01 - 25 hours", "1999-12-31"},
		{"@2000-01-01T10 - 30 minutes", "2000-01-01T10"},
		{"@2000-01-01T10 - 90 minutes", "2000-01-01T09"},
		// So it stays within the years a date may name
		{"@0001 - 1 day", "0001"},
		{"@9999 + 364 days", "9999"},

		// Seconds and milliseconds are one precision
		{"@2000-01-01T00:00:00 - 1 'ms'", "1999-12-31T23:59:59"},

		// A time of day truncates as a date does
		{"@T10 - 30 minutes", "10"},
		{"@T10:00 + 90 seconds", "10:01"},
		{"@T00:00 - 1 second", "00:00"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}
