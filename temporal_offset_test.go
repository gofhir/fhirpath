package fhirpath

import "testing"

// TestTemporalInstantsAcrossOffsets covers values written at different offsets,
// which compare as the instants they start at, as fhirpath.js and the HL7
// validator compare them: @2020-01-01T10+05:30 is 04:30 at UTC. Up to 1.14.1
// they compared as the spans of time they name, the hour from 04:30 to 05:30,
// and two that overlapped had no order, which neither engine answers.
func TestTemporalInstantsAcrossOffsets(t *testing.T) {
	for expr, want := range map[string]string{
		// The same instant at the same precision is equal
		"@2020-01-01T10+05:00 = @2020-01-01T05Z":       "true",
		"@2020-01-01T10:30+05:30 = @2020-01-01T05:00Z": "true",
		"@2020-01-01T10+05:30 = @2020-01-01T11+06:30":  "true",
		// Instants apart are ordered, whatever the precisions
		"@2020-01-01T10+05:30 > @2020-01-01T04Z":            "true",
		"@2020-01-01T10+05:30 >= @2020-01-01T04Z":           "true",
		"@2020-01-01T10+05:30 < @2020-01-01T05Z":            "true",
		"@2020-01-01T10+05:30 <= @2020-01-01T05Z":           "true",
		"@2020-01-01T10+05:30 = @2020-01-01T04Z":            "false",
		"@2020-01-01T10+05:30 ~ @2020-01-01T04Z":            "false",
		"@2020-01-01T10+05:30 = @2020-01-01T11:30+06:30":    "false",
		"@2020-01-01T10+05:30 > @2020-01-01T04:29Z":         "true",
		"@2020-01-01T10+05:30 < @2020-01-01T05:29Z":         "true",
		"@2020-01-01T10+05:30 < @2020-01-01T05:30Z":         "true",
		"@2020-01-01T10:15+05:30 < @2020-01-01T05Z":         "true",
		"@2019-01-01T10+05:30 < @2020-01-01T04Z":            "true",
		"@2020-01-01T10+05:30 = @2020-01-01T10:00:00Z":      "false",
		"@2020-01-01T10:30+05:30 = @2020-01-01T05:00:30Z":   "false",
		"@2020-01-01T00+05:00 < @2019-12-31T20Z":            "true",
		"@2020-01-01T10:00+05:30 = @2020-01-01T04:30+00:00": "true",
		// The same instant at different precisions is unknown, as at one offset
		"@2020-01-01T10+05:30 = @2020-01-01T04:30Z":    "EMPTY",
		"@2020-01-01T04:30:00Z < @2020-01-01T10+05:30": "EMPTY",
		"@2020-01-01T10+05:30 > @2020-01-01T04:30:00Z": "EMPTY",
		// Seconds and milliseconds name instants
		"@2020-01-01T10:00:00+05:30 < @2020-01-01T04:30:00.500Z": "true",
		"@2020-01-01T10:00:00.500+05:30 > @2020-01-01T04:30:00Z": "true",
		// An offset follows a time and nothing else, so a date with one is no
		// DateTime, as fhirpath.js and the HL7 validator read it
		"'2021+05:00'.toDateTime().exists()":                                    "false",
		"'2020-01-01+05:00'.toDateTime().exists()":                              "false",
		"'2020-01-01+05:00'.convertsToDateTime()":                               "false",
		"'2020-01-01T10+05:00'.toDateTime().exists()":                           "true",
		"'2020-01-01+05:00'.toDateTime() < '2020-01-01T00:00:00Z'.toDateTime()": "EMPTY",
	} {
		if got := evaluateScalar(t, expr, simpleJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
