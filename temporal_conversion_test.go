package fhirpath

import "testing"

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

// TestSubstringWithEmptyStart covers an absent start position, which the
// specification answers rather than refuses: "If the input or start is empty,
// the result is empty."
func TestSubstringWithEmptyStart(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		{"'string'.substring({}).empty()", "true"},
		{"{}.substring(1).empty()", "true"},
		// A start that is present still works, including out of range
		{"'string'.substring(2)", "ring"},
		{"'string'.substring(0, 3)", "str"},
		{"'string'.substring(20).empty()", "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}
