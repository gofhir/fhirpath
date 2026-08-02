package fhirpath

import "testing"

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
			if _, err := MustCompile(expr).Evaluate(patient); err == nil {
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
