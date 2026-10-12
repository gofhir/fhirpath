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
		// An instant at a span's start is within it, either way round
		"@2020-01-01T04:30:00Z < @2020-01-01T10+05:30": "EMPTY",
		"@2020-01-01T10+05:30 > @2020-01-01T04:30:00Z": "EMPTY",
		// A minute at an hour's start is not the hour
		"@2020-01-01T10+05:30 = @2020-01-01T04:30Z": "EMPTY",
		// An hour runs to its last minute, and a minute to its last second
		"@2020-01-01T10+05:30 < @2020-01-01T05:29Z":       "EMPTY",
		"@2020-01-01T10:30+05:30 = @2020-01-01T05:00:30Z": "EMPTY",
		// Seconds and milliseconds name instants
		"@2020-01-01T10:00:00+05:30 < @2020-01-01T04:30:00.500Z": "true",
		"@2020-01-01T10:00:00.500+05:30 > @2020-01-01T04:30:00Z": "true",
		// A year, a month and a day run to their calendar ends
		"'2020+05:00'.toDateTime() < '2020-12-31T18:00:00Z'.toDateTime()":       "EMPTY",
		"'2020-01+05:00'.toDateTime() < '2020-01-30T20:00:00Z'.toDateTime()":    "EMPTY",
		"'2020-02+05:00'.toDateTime() < '2020-02-29T19:00:00Z'.toDateTime()":    "true",
		"'2020-01-01+05:00'.toDateTime() < '2020-01-01T00:00:00Z'.toDateTime()": "EMPTY",
	} {
		if got := evaluateScalar(t, expr, simpleJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
