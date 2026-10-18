package fhirpath_test

import (
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// The examples the specification gives for each extractor, run as written.
func TestComponentExtractorsFromTheSpecification(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{"@2014-01-05T10:30:00.000.yearOf()", "2014"},
		{"@2014-01-05T10:30:00.000.monthOf()", "1"},
		{"@2014-01-05T10:30:00.000.dayOf()", "5"},
		{"@2012-01-01T03:30:40.002-07:00.hourOf()", "3"},
		{"@2012-01-01T16:30:40.002-07:00.hourOf()", "16"},
		{"@2012-01-01T12:30:40.002-07:00.minuteOf()", "30"},
		{"@2012-01-01T12:30:40.002-07:00.secondOf()", "40"},
		{"@2012-01-01T12:30:00.002-07:00.millisecondOf()", "2"},
		{"@2012-01-01T12:30:00.000-07:00.timezoneOffsetOf()", "-7.0"},
		{"@2012-01-01T12:30:00.000+08:45.timezoneOffsetOf()", "8.75"},
		{"@2012-01-01T12:30:00.000-07:00.dateOf()", "2012-01-01"},
		{"@2012-01-01T12:30:00.000-07:00.timeOf()", "12:30:00.000"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(`{"resourceType":"Patient"}`), tt.expr)
			if err != nil {
				t.Fatalf("%s: %v", tt.expr, err)
			}
			if len(result) != 1 {
				t.Fatalf("got %d results, want 1", len(result))
			}
			if got := result[0].String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// A component the value does not carry is empty, not zero. The specification
// gives one of these — @2012.monthOf() — and the same rule decides the rest.
func TestComponentAbsentIsEmpty(t *testing.T) {
	for _, expr := range []string{
		"@2012.monthOf()",
		"@2012.dayOf()",
		"@2012-01.dayOf()",
		"@2012-01-01.hourOf()",
		"@2012-01-01T12.minuteOf()",
		"@2012-01-01T12:30.secondOf()",
		"@2012-01-01T12:30:40.millisecondOf()",
		"@2012-01-01T12:30:40.timezoneOffsetOf()",
		"@2012-01-01.timeOf()",
		// A time without a day: the month and day are absent even though the
		// value carries a time.
		"@2012T12:30:00.monthOf()",
		"@2012T12:30:00.dayOf()",
		"@2012-05T12:30:00.dayOf()",
		"@T12:30.secondOf()",
		"@T12:30:40.millisecondOf()",
	} {
		t.Run(expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(`{"resourceType":"Patient"}`), expr)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if !result.Empty() {
				t.Errorf("got %v, want an empty collection", result)
			}
		})
	}
}

// An offset in whole quarters of an hour divides exactly; twenty minutes is a
// third of an hour and does not, and the result says so rather than stopping at
// a few places and claiming to be exact.
func TestTimezoneOffsetDivision(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"@2012-01-01T12:30:00.000Z.timezoneOffsetOf()", "0.0"},
		{"@2012-01-01T12:30:00.000+05:45.timezoneOffsetOf()", "5.75"},
		{"@2012-01-01T12:30:00.000-03:30.timezoneOffsetOf()", "-3.5"},
		{"@2012-01-01T12:30:00.000-07:00.timezoneOffsetOf()", "-7.0"},
		{"@2012-01-01T12:30:00.000+00:20.timezoneOffsetOf()", "0.3333333333333333"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(`{"resourceType":"Patient"}`), tt.expr)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if len(result) != 1 {
				t.Fatalf("got %d results, want 1", len(result))
			}
			if got := result[0].String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// dateOf keeps the precision the value was written with, rather than filling in
// what it does not say.
func TestDateOfKeepsPrecision(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"@2012.dateOf()", "2012"},
		{"@2012-05.dateOf()", "2012-05"},
		{"@2012-05-07.dateOf()", "2012-05-07"},
		{"@2012T12:30:00.dateOf()", "2012"},
		{"@2012-05T12:30:00.dateOf()", "2012-05"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(`{"resourceType":"Patient"}`), tt.expr)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if len(result) != 1 {
				t.Fatalf("got %d results, want 1", len(result))
			}
			if got := result[0].String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// A value that is not temporal at all gives empty, and the wrong kind of
// temporal value does too: timeOf asks for a DateTime, and a Date has no time
// in it.
func TestComponentOfWrongTypeIsEmpty(t *testing.T) {
	for _, expr := range []string{
		"'abc'.yearOf()",
		"5.monthOf()",
		"true.dayOf()",
		"@T12:30:00.yearOf()",
		"@2012-01-01.timezoneOffsetOf()",
		"@T12:30:00.dateOf()",
		"@T12:30:00.timeOf()",
	} {
		t.Run(expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(`{"resourceType":"Patient"}`), expr)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if !result.Empty() {
				t.Errorf("got %v, want an empty collection", result)
			}
		})
	}
}

// "If the input collection contains multiple items, the evaluation of the
// expression will end and signal an error to the calling environment."
func TestComponentOfManyItemsIsAnError(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[
		{"family":"a"},{"family":"b"}]}`)

	// Two dates reached through a collection.
	if _, err := fhirpath.Evaluate(patient, "(@2012-01-01 | @2013-01-01).yearOf()"); err == nil {
		t.Error("expected an error for a collection of two dates, got none")
	}

	if _, err := fhirpath.Evaluate(patient, "Patient.name.family.yearOf()"); err == nil {
		t.Error("expected an error for a collection of two strings, got none")
	}
}

// An empty input gives empty rather than an error, which is what makes these
// safe to chain onto a path that may find nothing.
func TestComponentOfEmptyIsEmpty(t *testing.T) {
	result, err := fhirpath.Evaluate([]byte(`{"resourceType":"Patient"}`), "Patient.birthDate.yearOf()")
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !result.Empty() {
		t.Errorf("got %v, want an empty collection", result)
	}
}

// The calendar-unit spellings answer the same thing, which is what keeps the
// two names from drifting. They need backticks: the grammar reads a bare month
// after a dot as the unit.
func TestCalendarUnitSpellingsAgree(t *testing.T) {
	pairs := [][2]string{
		{"@2014-01-05.`year`()", "@2014-01-05.yearOf()"},
		{"@2014-01-05.`month`()", "@2014-01-05.monthOf()"},
		{"@2014-01-05.`day`()", "@2014-01-05.dayOf()"},
		{"@2014-01-05T10:30:40.123.`hour`()", "@2014-01-05T10:30:40.123.hourOf()"},
		{"@2014-01-05T10:30:40.123.`minute`()", "@2014-01-05T10:30:40.123.minuteOf()"},
		{"@2014-01-05T10:30:40.123.`second`()", "@2014-01-05T10:30:40.123.secondOf()"},
		{"@2014-01-05T10:30:40.123.`millisecond`()", "@2014-01-05T10:30:40.123.millisecondOf()"},
		{"@2012-01-01.`month`()", "@2012-01-01.monthOf()"},
		{"@2012.`month`()", "@2012.monthOf()"},
	}

	for _, pair := range pairs {
		t.Run(pair[0], func(t *testing.T) {
			old, err := fhirpath.Evaluate([]byte(`{"resourceType":"Patient"}`), pair[0])
			if err != nil {
				t.Fatalf("%s: %v", pair[0], err)
			}
			current, err := fhirpath.Evaluate([]byte(`{"resourceType":"Patient"}`), pair[1])
			if err != nil {
				t.Fatalf("%s: %v", pair[1], err)
			}

			if len(old) != len(current) {
				t.Fatalf("%s gave %d results, %s gave %d", pair[0], len(old), pair[1], len(current))
			}
			for i := range old {
				if old[i].String() != current[i].String() {
					t.Errorf("%s gave %q, %s gave %q", pair[0], old[i].String(), pair[1], current[i].String())
				}
			}
		})
	}
}

// TestTemporalEquivalence runs the examples FHIRPath 3.0.0 works through for
// Date, DateTime and Time equivalence: "the comparison is the same as for
// equality, with the exception that if the input values have different levels
// of precision, the result is false, rather than empty". ~ compared instants
// and set precision and offset aside, so @2020T ~ @2020-01-01T00:00:00Z was
// true.
func TestTemporalEquivalence(t *testing.T) {
	for expr, want := range map[string]string{
		// The specification's examples
		"@2012 ~ @2012":                                 "true",
		"@2012 ~ @2013":                                 "false",
		"@2012-01 ~ @2012":                              "false",
		"@2012-01-01T10:30 ~ @2012-01-01T10:30":         "true",
		"@2012-01-01T10:30 ~ @2012-01-01T10:31":         "false",
		"@2012-01-01T10:30:31 ~ @2012-01-01T10:30":      "false",
		"@2012-01-01T10:30:31.0 ~ @2012-01-01T10:30:31": "true",
		"@2012-01-01T10:30:31.1 ~ @2012-01-01T10:30:31": "false",
		// Precision differs: false, where = is empty
		"@2020T ~ @2020-01-01T00:00:00Z":             "false",
		"@2020-01-01T10:00Z ~ @2020-01-01T10:00:00Z": "false",
		"@2020-01-01 ~ @2020-01-01T00:00":            "false",
		"@T10:30 ~ @T10:30:00":                       "false",
		// An offset against none: = has no answer, ~ is false
		"@2020-01-01T10:00:00Z ~ @2020-01-01T10:00:00": "false",
		// The same instant at two offsets, and seconds against milliseconds
		"@2020-01-01T10:00:00Z ~ @2020-01-01T05:00:00-05:00": "true",
		"@T10:30:00 ~ @T10:30:00.000":                        "true",
		"@2020-01-01T10:00:00Z !~ @2020-01-01T10:00:00":      "true",
		// A Date converts to a DateTime, at the same precision
		"@2020-01-01 ~ @2020-01-01T": "true",
		"@2020 ~ @2020T":             "true",
		// union(), distinct() and in test equality the same way: values at
		// different precisions, or an offset against none, are two values
		"(@2020T | @2020-01-01T00:00:00Z).count()":                     "2",
		"@2020T in (@2020-01-01T00:00:00Z)":                            "false",
		"(@2020-01-01T10:00:00Z | @2020-01-01T10:00:00).count()":       "2",
		"(@2020-01-01T10:00:00Z | @2020-01-01T05:00:00-05:00).count()": "1",
		"@2020-01-01 in (@2020-01-01T)":                                "true",
		"(@T10:30:00 | @T10:30:00.000).count()":                        "1",
		// Values of one precision and offset differ on any component they state
		"(@2019-01-01T10:00:00Z | @2020-01-01T10:00:00Z).count()":         "2",
		"(@2020-01-01T10:00:00Z | @2020-02-01T10:00:00Z).count()":         "2",
		"(@2020-01-01T10:00:00Z | @2020-01-02T10:00:00Z).count()":         "2",
		"(@2020-01-01T10:00:00Z | @2020-01-01T11:00:00Z).count()":         "2",
		"(@2020-01-01T10:00:00Z | @2020-01-01T10:01:00Z).count()":         "2",
		"(@2020-01-01T10:00:00Z | @2020-01-01T10:00:01Z).count()":         "2",
		"(@2020-01-01T10:00:00.001Z | @2020-01-01T10:00:00.002Z).count()": "2",
		"(@2020-01-01T10:00:00Z | @2020-01-01T10:00:00Z).count()":         "1",
	} {
		if got := evaluateScalar(t, expr, simpleJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}

	// Inside complex values, a date child compares the same way
	periods := []byte(`{"resourceType":"Basic","extension":[
	  {"url":"a","valuePeriod":{"start":"2020-01-01T10:00:00Z"}},
	  {"url":"a","valuePeriod":{"start":"2020-01-01T10:00Z"}},
	  {"url":"a","valuePeriod":{"start":"2020-01-01T05:00:00-05:00"}}]}`)
	for expr, want := range map[string]string{
		"Basic.extension[0].value ~ Basic.extension[1].value": "false",
		"Basic.extension[0].value ~ Basic.extension[2].value": "true",
	} {
		if got := evaluateScalar(t, expr, periods); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// TestTemporalPrecisionSemantics covers comparing and equating dates, datetimes
// and times specified to different precisions.
//
// The rule, from the FHIRPath specification: the comparison proceeds precision
// by precision from the most significant down and stops at the first difference;
// if one value has a component the other does not and everything before matched,
// the result is empty, "to indicate that the result of the comparison is
// unknown". Seconds and milliseconds count as a single precision compared as a
// decimal.
//
// The distinction matters for FHIR invariants: a comparison that cannot be
// decided must not silently answer true or false.
func TestTemporalPrecisionSemantics(t *testing.T) {
	t.Run("ordering decides at the first differing precision", func(t *testing.T) {
		cases := map[string]bool{
			"@2018-03-01 > @2018-01-01":                     true,
			"@2018-03-01T10:30:00 > @2018-03-01T10:00:00":   true,
			"@T10:30:00 > @T10:00:00":                       true,
			"@2018-01-01 > @2018-03-01":                     false,
			"@2018-03-01T10:30:00 > @2018-03-01T10:30:00.0": false,
			"@T10:30:00 > @T10:30:00.0":                     false,
		}
		for expr, expected := range cases {
			t.Run(expr, func(t *testing.T) {
				assertBooleanResult(t, evalOrFatal(t, simpleJSON, expr), expected)
			})
		}
	})

	t.Run("ordering is unknown across precisions", func(t *testing.T) {
		// Every shared component matches, so the order cannot be determined
		for _, expr := range []string{
			"@2018-03 > @2018-03-01",
			"@2018-03-01T10 > @2018-03-01T10:30",
			"@T10 > @T10:30",
			"@2018 < @2018-03",
			"@2018-03-01T10:30 >= @2018-03-01T10:30:00",
		} {
			t.Run(expr, func(t *testing.T) {
				assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
			})
		}
	})

	t.Run("equality follows the same rule", func(t *testing.T) {
		assertEmptyResult(t, evalOrFatal(t, simpleJSON, "@2012-01 = @2012"), "@2012-01 = @2012")
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "@2012-01 = @2012-02"), false)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "@2012-01 = @2012-01"), true)
		// Seconds and milliseconds are one precision
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "@T10:30:00 = @T10:30:00.0"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "@T10:30:00 = @T10:30:00.100"), false)
	})

	t.Run("a date compares against a datetime", func(t *testing.T) {
		resource := []byte(`{"resourceType":"Patient","birthDate":"1974-12-25"}`)

		// Decided at the year, long before precision runs out
		assertBooleanResult(t, evalOrFatal(t, resource, "birthDate < @2020-01-01T10:00:00"), true)

		// Same calendar day: the datetime carries a time the date does not, so
		// the order is unknown
		expr := "birthDate != @1974-12-25T12:34:00"
		assertEmptyResult(t, evalOrFatal(t, resource, expr), expr)
	})

	t.Run("offsets are respected", func(t *testing.T) {
		// The same instant written against two offsets
		assertBooleanResult(t, evalOrFatal(t, simpleJSON,
			"@2018-03-01T10:30:00Z = @2018-03-01T12:30:00+02:00"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON,
			"@2018-03-01T10:30:00Z < @2018-03-01T12:30:00+01:00"), true)
	})

	t.Run("date arithmetic accepts both unit systems", func(t *testing.T) {
		cases := map[string]string{
			// Calendar keywords
			"(@1973-12-25 + 1 day).toString()":   "1973-12-26",
			"(@1973-12-25 + 1 week).toString()":  "1974-01-01",
			"(@1973-12-25 + 1 month).toString()": "1974-01-25",
			"(@1973-12-25 + 1 year).toString()":  "1974-12-25",
			// UCUM definite durations up to a week convert exactly
			"(@1973-12-25 + 1 'd').toString()":                    "1973-12-26",
			"(@1973-12-25 + 1 'wk').toString()":                   "1974-01-01",
			"(@1973-12-25T00:00:00.000+10:00 + 1 's').toString()": "1973-12-25T00:00:01.000+10:00",
			// Subtraction
			"(@1974-01-01 - 1 week).toString()": "1973-12-25",
		}
		for expr, expected := range cases {
			t.Run(expr, func(t *testing.T) {
				assertStringResult(t, evalOrFatal(t, simpleJSON, expr), expected)
			})
		}
	})

	t.Run("UCUM years and months need explicit conversion", func(t *testing.T) {
		// A UCUM year is a fixed 365.25 days and a UCUM month 30.44, neither of
		// which is what adding a calendar year or month means. The spec keeps
		// the systems apart rather than silently choosing one.
		for _, expr := range []string{
			"@1973-12-25 + 1 'a'",
			"@1975-12-25 + 1 'a'",
			"@1973-12-25 + 1 'mo'",
		} {
			t.Run(expr, func(t *testing.T) {
				if _, err := fhirpath.Evaluate(simpleJSON, expr); err == nil {
					t.Error("expected an error rather than a silently chosen meaning")
				}
			})
		}
	})

	t.Run("an unrecognized unit is an error, not a no-op", func(t *testing.T) {
		// Previously any unknown unit returned the date unchanged
		if _, err := fhirpath.Evaluate(simpleJSON, "@1973-12-25 + 1 'furlong'"); err == nil {
			t.Error("expected an error for a unit that cannot shift a date")
		}
	})

	t.Run("partial datetime literals are DateTime, not Date", func(t *testing.T) {
		// The grammar allows the T marker with no time after it
		for _, expr := range []string{
			"@2015T.is(DateTime)",
			"@2015-02T.is(DateTime)",
			"@2015-02-04T.is(DateTime)",
		} {
			t.Run(expr, func(t *testing.T) {
				assertBooleanResult(t, evalOrFatal(t, simpleJSON, expr), true)
			})
		}
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "@2015.is(Date)"), true)
	})
}

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
		if _, err := fhirpath.Evaluate(simpleJSON, expr); err == nil || !strings.Contains(err.Error(), "invalid") {
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
		if _, err := fhirpath.Evaluate(simpleJSON, expr); err == nil {
			t.Errorf("%s: expected an error", expr)
		}
	}
	if got := evaluateScalar(t, "@9999-12-30 + 1 day", simpleJSON); got != "9999-12-31" {
		t.Errorf("@9999-12-30 + 1 day = %s, want 9999-12-31", got)
	}
}

// TestHeuristicDateDetection verifies that JSON string fields matching ISO 8601
// date patterns are automatically detected as Date/DateTime types without a Model.
func TestHeuristicDateDetection(t *testing.T) {
	patient := []byte(`{
		"resourceType": "Patient",
		"birthDate": "1978-03-12"
	}`)

	t.Run("birthDate is detected as Date", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patient, "Patient.birthDate")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result.Empty() {
			t.Fatal("expected non-empty result")
		}
		if result[0].Type() != "Date" {
			t.Errorf("expected type Date, got %s", result[0].Type())
		}
	})

	t.Run("birthDate equals Date literal", func(t *testing.T) {
		result, err := fhirpath.EvaluateToBoolean(patient, "Patient.birthDate = @1978-03-12")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if !result {
			t.Error("expected birthDate = @1978-03-12 to be true")
		}
	})

	t.Run("birthDate.lowBoundary works", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patient, "Patient.birthDate.lowBoundary()")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result.Empty() {
			t.Fatal("expected non-empty result from lowBoundary on auto-detected Date")
		}
		if result[0].String() != "1978-03-12" {
			t.Errorf("expected 1978-03-12, got %s", result[0].String())
		}
	})

	t.Run("plain string is not misdetected as Date", func(t *testing.T) {
		resource := []byte(`{"resourceType": "Patient", "id": "test-patient"}`)
		result, err := fhirpath.Evaluate(resource, "Patient.id")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result[0].Type() != "String" {
			t.Errorf("expected type String for id field, got %s", result[0].Type())
		}
	})

	t.Run("year-only date is detected", func(t *testing.T) {
		resource := []byte(`{"resourceType": "Patient", "birthDate": "1978"}`)
		result, err := fhirpath.Evaluate(resource, "Patient.birthDate")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result[0].Type() != "Date" {
			t.Errorf("expected type Date, got %s", result[0].Type())
		}
	})
}
