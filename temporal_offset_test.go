package fhirpath

import "testing"

// TestTemporalSpansAcrossOffsets covers values written at different offsets,
// which compare as the spans of time they name. @2020-01-01T10+05:30 is the
// hour from 04:30 to 05:30 at UTC; re-expressing it at UTC, as before, put it
// on a minute it never stated and read it as 04Z.
func TestTemporalSpansAcrossOffsets(t *testing.T) {
	for expr, want := range map[string]string{
		// The same span is equal
		"@2020-01-01T10+05:00 = @2020-01-01T05Z":       "true",
		"@2020-01-01T10:30+05:30 = @2020-01-01T05:00Z": "true",
		"@2020-01-01T10+05:30 = @2020-01-01T11+06:30":  "true",
		// Spans that overlap without being the same have no order
		"@2020-01-01T10+05:30 = @2020-01-01T04Z":         "EMPTY",
		"@2020-01-01T10+05:30 = @2020-01-01T11:30+06:30": "EMPTY",
		"@2020-01-01T10+05:30 < @2020-01-01T05Z":         "EMPTY",
		"@2020-01-01T10+05:30 ~ @2020-01-01T04Z":         "false",
		// Spans apart are ordered, the end of a span not within it
		"@2020-01-01T10+05:30 > @2020-01-01T04:29Z":    "true",
		"@2020-01-01T10+05:30 < @2020-01-01T05:30Z":    "true",
		"@2019-01-01T10+05:30 < @2020-01-01T04Z":       "true",
		"@2020-01-01T10+05:30 = @2020-01-01T10:00:00Z": "false",
		// A year at one offset is not the same year at another
		"'2021+05:00'.toDateTime() = '2021Z'.toDateTime()": "EMPTY",
		"'2021+05:00'.toDateTime() < '2023Z'.toDateTime()": "true",
	} {
		if got := evaluateScalar(t, expr, simpleJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
