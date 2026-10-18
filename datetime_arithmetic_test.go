package fhirpath_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

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
		{"@2016-01 + 30 days", "2016-02"},
		{"@2016 + 31536000 seconds", "2017"},
		{"@2016T + 365 days", "2017"},

		// A whole unit of the value moves it, and a unit less does not
		{"@2000-01-01 + 24 hours", "2000-01-02"},
		{"@2000-01-01 + 23 hours", "2000-01-01"},
		{"@2000-01-01T10 + 60 minutes", "2000-01-01T11"},
		{"@2000-01-01T10 + 59 minutes", "2000-01-01T10"},
		{"@T10 + 3600 seconds", "11"},
		{"@T10:00 + 60 seconds", "10:01"},
		{"@T10:00 + 59 seconds", "10:00"},

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

// TestTemporalMeasurementSpecExamples runs the worked examples the FHIRPath
// 3.0.0 specification publishes for difference() and duration(), which are the
// only ones it gives for either function.
//
// The pair is worth reading together: both measure the same span and disagree,
// because difference() counts the boundaries crossed while duration() counts the
// periods that have elapsed.
func TestTemporalMeasurementSpecExamples(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		{"@2025-01-02.difference(@2025-01-07, 'week')", "1"}, // crossed a Sunday
		{"@2025-01-02.duration(@2025-01-07, 'week')", "0"},   // but only five days
		{"@2025-01-01.duration(@2025-09-01, 'year')", "0"},   // the baby is 9 months old
		{"@2024-12-01.duration(@2025-09-01, 'year')", "0"},   // and here 10 months
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestTemporalMeasurementRules covers the rules the specification states for
// difference() and duration() without giving an example of each.
func TestTemporalMeasurementRules(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// A single day apart, with a year boundary between them: the boundary
		// count is 1 and the elapsed year is 0
		{"@2024-12-31.difference(@2025-01-01, 'year')", "1"},
		{"@2024-12-31.duration(@2025-01-01, 'year')", "0"},
		{"@2024-12-31.difference(@2025-01-01, 'month')", "1"},
		{"@2024-12-31.difference(@2025-01-01, 'day')", "1"},

		// Negative when the input is the later of the two
		{"@2025-01-07.difference(@2025-01-02, 'week')", "-1"},
		{"@2025-01-01.duration(@2024-01-01, 'year')", "-1"},

		// Whole years elapsed, which is how an age is computed
		{"@2000-06-15.duration(@2025-06-14, 'year')", "24"},
		{"@2000-06-15.duration(@2025-06-15, 'year')", "25"},

		// "If the input value or value argument are of less precision than the
		// specified precision, the result is empty"
		{"@2025.difference(@2025-01-07, 'day')", "EMPTY"},
		{"@2025-01.duration(@2025-09, 'day')", "EMPTY"},
		// Measuring in weeks means knowing the weekday, so it needs a day too
		{"@2025-01.difference(@2025-03, 'week')", "EMPTY"},
		// A request the values can answer still answers
		{"@2025.difference(@2027, 'year')", "2"},

		// "If either the input or value argument is empty, the result is empty"
		{"{}.difference(@2025-01-07, 'day')", "EMPTY"},
		{"@2025-01-07.difference({}, 'day')", "EMPTY"},

		// Offsets are normalized at clock precision, so these are one instant
		{"@2025-01-01T12:00:00Z.difference(@2025-01-01T13:00:00+01:00, 'hour')", "0"},
		{"@2025-01-01T12:00:00Z.difference(@2025-01-01T13:00:00Z, 'hour')", "1"},

		// Times carry only a clock
		{"@T10:30:00.difference(@T11:15:00, 'minute')", "45"},
		{"@T10:30:00.difference(@T11:15:00, 'hour')", "1"},
		{"@T10:30:00.duration(@T11:15:00, 'hour')", "0"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestTemporalMeasurementInvalidPrecision checks that a precision the input's
// type cannot express is an error rather than empty.
//
// The distinction matters: empty means the answer is unknown, and asking a date
// how many hours it crossed is not an unknown answer but a malformed question.
// The specification fixes the permitted set per type.
func TestTemporalMeasurementInvalidPrecision(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	for _, expr := range []string{
		"@2025-01-02.difference(@2025-01-07, 'hour')", // a date has no clock
		"@T10:30:00.difference(@T11:15:00, 'day')",    // a time has no calendar
		"@2025-01-02.difference(@2025-01-07, 'fortnight')",
		"@2025-01-02.duration(@2025-01-07, 'hour')",
	} {
		t.Run(expr, func(t *testing.T) {
			if _, err := fhirpath.MustCompile(expr).Evaluate(patient); err == nil {
				t.Errorf("%s: expected an error", expr)
			}
		})
	}
}

// TestTimeArithmeticWrapsAroundTheDay covers what makes a time of day different
// from a date: it has no calendar to overflow into.
//
// "As Time is cyclic, using arithmetic operations + or - on Time types can
// result in overflowing the time value, which will wrap around the beginning of
// the day. So adding 1 hour to @T23:30:00 will wrap around to @T00:30:00, which
// is consistent with the behavior of DateTime values."
func TestTimeArithmeticWrapsAroundTheDay(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// Within the day
		{"@T01:00:00 + 2 hours", "03:00:00"},
		{"@T10:30:00 + 30 minutes", "11:00:00"},

		// Past midnight, forwards — the specification's own example
		{"@T23:30:00 + 1 hour", "00:30:00"},
		{"@T23:00:00 + 2 hours", "01:00:00"},
		// More than a whole day round
		{"@T23:00:00 + 50 hours", "01:00:00"},

		// Past midnight, backwards
		{"@T00:30:00 - 1 hour", "23:30:00"},
		{"@T01:00:00 - 2 hours", "23:00:00"},
		// A shift never makes a value more precise than it was, so the
		// millisecond lands in a value that carries milliseconds
		{"@T00:00:00.000 - 1 'ms'", "23:59:59.999"},
		{"@T00:00:00 - 1 'ms'", "23:59:59"},

		// The finer units carry through the coarser ones
		{"@T10:30:00.500 + 500 'ms'", "10:30:01.000"},
		{"@T10:59:59 + 1 second", "11:00:00"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestTimeRejectsDateComponents covers the units a time cannot take.
//
// "If there is more than one item, an item of an incompatible type, or an
// unsupported unit for the type, the evaluation of the expression will end and
// signal an error to the calling environment. This includes attempting to add
// date components to a Time."
func TestTimeRejectsDateComponents(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	for _, expr := range []string{
		"@T10:00:00 + 1 day",
		"@T10:00:00 + 1 week",
		"@T10:00:00 - 1 month",
		"@T10:00:00 + 1 year",
	} {
		t.Run(expr, func(t *testing.T) {
			if _, err := fhirpath.MustCompile(expr).Evaluate(patient); err == nil {
				t.Errorf("%s: expected an error, a time has no date to shift", expr)
			}
		})
	}
}

// TestFractionalDurationAppliesOnlyToSeconds covers where a duration's decimal
// part survives.
//
// "The decimal portion of the time-valued quantity is only applied for second or
// millisecond precisions; for all other precisions, the decimal portion is
// ignored, since date/time arithmetic is performed with calendar duration
// semantics." N1 states the same rule from the other side: "For precisions above
// seconds, the decimal portion of the time-valued quantity is ignored."
//
// The R4 conformance suite expects the first case below to drop the fraction,
// which contradicts both. The R5 suite was corrected to expect .100 — see
// CONFORMANCE.md.
func TestFractionalDurationAppliesOnlyToSeconds(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// A fraction of a second is milliseconds
		{"@1973-12-25T00:00:00.000+10:00 + 0.1 's'", "1973-12-25T00:00:00.100+10:00"},
		{"@T10:00:00.000 + 0.5 's'", "10:00:00.500"},
		{"@T10:00:00.000 + 1.5 seconds", "10:00:01.500"},

		// Anything coarser drops it: the specification's worked example is
		// @1973-12-25 + 7.9 days, "same as above as the decimal is truncated"
		{"@1973-12-25 + 7.9 days", "1974-01-01"},
		{"@1973-12-25 + 7 days", "1974-01-01"},
		{"@T10:00:00 + 1.9 hours", "11:00:00"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// A time-valued quantity is read as a decimal before it shifts a date or a
// time, so that no amount wraps past the int range on the way. On a date, an
// amount longer than the calendar reaches past the years 1 to 9999 from any
// start: "If the resulting year is out of range, an error is thrown". A time
// of day is cyclic, so a clock amount is taken modulo a day. They wrapped:
// @2020-01-01 + 18446744073709551617 days was @2020-01-02. (#163)
func TestATemporalShiftDoesNotWrap(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	for _, expr := range []string{
		"@2020-01-01 + 18446744073709551617 days",
		"@2020-01-01 + 9223372036854775807 days",
		"@2020-01-01 + 9223372036854775807 weeks",
		"@2020-01-01 + 9223372036854775807 years",
		"@2020-01-01 + 9223372036854775807 months",
		"@2020-01-01 + 9223372036854775807 'h'",
		"@2020-01-01T10:00:00Z + 9300000000000000 seconds",
		"@2020-01-01T10:00:00Z + 100000000000000000000 'ms'",
		"@2020-01-01T10:00:00Z - 9223372036854775807 'min'",
		"@2020-01-01 + 7980 years",
		"@2020-01-01 - 2020 years",
	} {
		_, err := fhirpath.Evaluate(patient, expr)
		if !errors.Is(err, types.ErrInvalidTemporal) || !strings.Contains(err.Error(), "1 to 9999") {
			t.Errorf("%s: error %v, want the calendar's range", expr, err)
		}
	}

	for expr, want := range map[string]string{
		// Within the calendar, as before
		"@2020-01-01 + 7979 years":      "[9999-01-01]",
		"@2020-01-01 - 2019 years":      "[0001-01-01]",
		"@2020-01-01 + 7.9 days":        "[2020-01-08]",
		"@2000-01-01T00:00:00 - 1 'ms'": "[1999-12-31T23:59:59]",
		"@T23:30:00 + 1 hour":           "[00:30:00]",
		"@T01:00:00 + 48 hours":         "[01:00:00]",
		// Past what a time.Duration holds, about 292 years, within the
		// calendar: the clock amount's whole days shift the calendar
		"@2020-01-01T10:00:00Z + 3000000 hours":       "[2362-03-29T10:00:00Z]",
		"@2020-01-01T10:00:00Z - 3000000 hours":       "[1677-10-05T10:00:00Z]",
		"@2020-01-01T10:00:00Z + 10000000000000 'ms'": "[2336-11-21T03:46:40Z]",
		"@2020-01-01T10:00:00Z + 9300000000 seconds":  "[2314-09-16T07:20:00Z]",
		"@2020-01-01 + 3000000 hours":                 "[2362-03-29]",
		// A time of day takes a clock amount modulo a day
		"@T10:00 + 9223372036854775807 hours":       "[17:00]",
		"@T10:00 - 9223372036854775807 hours":       "[03:00]",
		"@T10:00 + 100000000000000000000.5 seconds": "[19:46]",
	} {
		result, err := fhirpath.Evaluate(patient, expr)
		if err != nil {
			t.Errorf("%s: %v", expr, err)
			continue
		}
		if got := result.String(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}

	// A date component cannot shift a time, whatever its amount, none or a
	// fraction of one included: "This includes attempting to add date
	// components to a Time"
	for _, expr := range []string{"@T10:00 + 1 day", "@T10:00 - 1 week", "@T10:00 + 9223372036854775808000 days", "@T10:00 + 0 days", "@T10:00 + 0.5 days"} {
		if _, err := fhirpath.Evaluate(patient, expr); !errors.Is(err, types.ErrDateComponentOnTime) {
			t.Errorf("%s: error %v, want a date component refused on a time", expr, err)
		}
	}
}
