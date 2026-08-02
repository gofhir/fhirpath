package fhirpath

import "testing"

func evaluateScalar(t *testing.T, expr string, resource []byte) string {
	t.Helper()

	result, err := MustCompile(expr).Evaluate(resource)
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	if len(result) == 0 {
		return "EMPTY"
	}
	return result[0].String()
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
			if _, err := MustCompile(expr).Evaluate(patient); err == nil {
				t.Errorf("%s: expected an error", expr)
			}
		})
	}
}
