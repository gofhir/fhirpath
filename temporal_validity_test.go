package fhirpath

import (
	"strings"
	"testing"
)

// TestTemporalValidity checks that only days on the calendar and times on the
// clock are dates and times. FHIRPath's Date runs from @0001-01-01 to
// @9999-12-31. Accepted as written and normalized by time.Date when compared
// at an offset, February 30 made equality intransitive. A leap second, which
// FHIR's dateTime, instant and time allow, is read, as fhirpath.js and the HL7
// validator read it.
func TestTemporalValidity(t *testing.T) {
	// A literal off the calendar or the clock is a parse error, as in
	// fhirpath.js
	for _, expr := range []string{
		"@2021-02-30", "@2021-13-01", "@2021-00-10", "@2100-02-29", "@0000-01-01",
		"@2021-04-31", "@2021-11-31", "@2021-13", "@0000", "@2021-01-00", "@2021-01-00T10:00",
		"@2021-01-01T25:00", "@2021-01-01T10:60", "@2021-01-01T10:00:61",
		// A leap second on no day, so on no particular second, or past the
		// last second of 9999
		"@2012T12:30:60", "@9999-12-31T23:59:60Z",
		"@T24:00", "@T10:60",
		"@2021-01-01T10:00+15:00", "@2021-01-01T10:00+05:75", "@2021-01-01T10:00+05:60",
		"@2021-01-01T10:00+14:01", "@2021-01-01T10:00-14:01",
	} {
		if _, err := Evaluate(simpleJSON, expr); err == nil || !strings.Contains(err.Error(), "invalid") {
			t.Errorf("%s: error %v, want a parse error", expr, err)
		}
	}

	doc := []byte(`{"resourceType":"Patient","birthDate":"2021-02-30",
	  "extension":[{"url":"x","valueDateTime":"2021-02-29T10:00:00Z"}]}`)
	for expr, want := range map[string]string{
		// Days that are on the calendar
		"@2020-02-29.exists()":    "true",
		"@2000-02-29.exists()":    "true",
		"@9999-12-31.exists()":    "true",
		"@T23:59:59.999.exists()": "true",
		// A leap second is the second after it, written as it was
		"@2016-12-31T23:59:60Z = @2017-01-01T00:00:00Z":           "true",
		"@2016-12-31T23:59:60Z > @2016-12-31T23:59:59Z":           "true",
		"@2016-12-31T23:59:60Z + 1 second":                        "2017-01-01T00:00:01Z",
		"'2016-12-31T23:59:60Z'.toDateTime().toString()":          "2016-12-31T23:59:60Z",
		"'2016-12-31T23:59:60Z'.convertsToDateTime()":             "true",
		"(@2016-12-31T23:59:60Z | @2017-01-01T00:00:00Z).count()": "1",
		// whose components are as written, on the day it falls on
		"@2016-12-31T23:59:60Z.toDate()":   "2016-12-31",
		"@2016-12-31T23:59:60Z.secondOf()": "60",
		"@2016-12-31T23:59:60Z.timeOf()":   "23:59:60",
		// and within its day and its minute, as toDate() has it
		"@2016-12-31T23:59:60Z > @2016-12-31":          "EMPTY",
		"@2016-12-31T23:59:60Z > @2016-12-31T23:59Z":   "EMPTY",
		"@2016-12-31T23:59:60Z < @2017-01-01":          "true",
		"@2016-12-31T23:59:60Z.toDate() = @2016-12-31": "true",
		// and a time's is after 23:59:59, before nothing that day
		"@T23:59:60 > @T23:59:59":          "true",
		"@T23:59:60 = @T00:00:00":          "false",
		"@T23:59:60 + 1 second":            "00:00:01",
		"@T23:59:60 + 0 seconds":           "23:59:60",
		"@T23:59:60 - 1 second":            "23:59:59",
		"@T10:30:60 + 0 seconds":           "10:30:60",
		"'23:59:60'.convertsToTime()":      "true",
		"@2012T12:30:00.exists()":          "true",
		"@2021-01-01T10:00+14:00.exists()": "true",
		"@2021-01-01T10:00-14:00.exists()": "true",
		// toDate() of a DateTime keeps its own precision, to the day at most
		"@2012T.toDate()":            "2012",
		"@2012-05T.convertsToDate()": "true",
		// A DateTime that writes a time without a day reads back as written
		"@2012T12:30:00.toString()":                               "2012T12:30:00",
		"@2012T12:30:00.toString().toDateTime() = @2012T12:30:00": "true",
		// An invalid date in the input is read as the string it is
		"Patient.birthDate is String":     "true",
		"Patient.birthDate = @2021-03-02": "false",
		// A choice variant keeps the type its key names, and is no DateTime
		"Patient.extension.value is DateTime":             "false",
		"Patient.extension.value is FHIR.dateTime":        "true",
		"Patient.extension.value = @2021-03-01T10:00:00Z": "false",
		// Conversions from a string answer empty, and convertsTo* false
		"'2021-02-30'.toDate()":               "EMPTY",
		"'2021-02-30'.convertsToDate()":       "false",
		"'2021-02-29T10:00:00Z'.toDateTime()": "EMPTY",
		"'25:00'.toTime()":                    "EMPTY",
		"'2020-02-29'.convertsToDate()":       "true",
		// convertsToDate() answers as toDate() does
		"'abc'.convertsToDate()":             "false",
		"@2020.convertsToDate()":             "true",
		"@2020-01-01T10:00.convertsToDate()": "true",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// TestTemporalArithmeticRange checks the range of a date reached by
// arithmetic: "If the resulting year is out of range, an error is thrown."
func TestTemporalArithmeticRange(t *testing.T) {
	for _, expr := range []string{"@9999-12-31 + 1 day", "@0001-01-01 - 1 day", "@9999 + 1 year", "@9999-12-31T23:59:59Z + 1 second", "@0001-01-01T00:00:00Z - 1 second", "@9999 + 365 days", "@0001-01 - 30 days"} {
		if _, err := Evaluate(simpleJSON, expr); err == nil {
			t.Errorf("%s: expected an error", expr)
		}
	}
	if got := evaluateScalar(t, "@9999-12-30 + 1 day", simpleJSON); got != "9999-12-31" {
		t.Errorf("@9999-12-30 + 1 day = %s, want 9999-12-31", got)
	}
}
