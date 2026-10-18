package fhirpath_test

import (
	"testing"
	"time"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// Every operator that needs an instant reads the same offset, so a default
// supplied for a value moves all of them together. They would otherwise say
// different things about one value: that it is later than another, and that it
// is two hours earlier.
func TestOperatorsAgreeOnTheDefaultOffset(t *testing.T) {
	tests := []struct {
		name       string
		offset     time.Duration
		afterBare  bool
		hoursApart string
	}{
		{"at UTC", 0, true, "2"},
		{"at UTC-5", -5 * time.Hour, false, "-3"},
		{"at UTC+9", 9 * time.Hour, true, "11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bare, err := types.NewDateTime("2020-01-01T00:00:00.0")
			if err != nil {
				t.Fatalf("NewDateTime: %v", err)
			}
			instant, err := types.NewDateTime("2020-01-01T02:00:00Z")
			if err != nil {
				t.Fatalf("NewDateTime: %v", err)
			}

			ctx := eval.NewContext([]byte(`{}`))
			ctx.SetVariable("bare", types.Collection{bare.WithDefaultOffset(tt.offset)})
			ctx.SetVariable("instant", types.Collection{instant})

			answer := func(expr string) string {
				t.Helper()

				result, err := fhirpath.MustCompile(expr).EvaluateWithContext(ctx)
				if err != nil {
					t.Fatalf("%s: %v", expr, err)
				}
				if len(result) != 1 {
					t.Fatalf("%s: got %d results, want 1", expr, len(result))
				}
				return result[0].String()
			}

			if got := answer("%instant > %bare"); got != boolText(tt.afterBare) {
				t.Errorf("%%instant > %%bare = %s, want %s", got, boolText(tt.afterBare))
			}
			if got := answer("%bare.difference(%instant, 'hour')"); got != tt.hoursApart {
				t.Errorf("difference = %s, want %s", got, tt.hoursApart)
			}
			if got := answer("%bare.duration(%instant, 'hour')"); got != tt.hoursApart {
				t.Errorf("duration = %s, want %s", got, tt.hoursApart)
			}
		})
	}
}

func boolText(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// What a supplied default reaches, and what it does not.
//
// An operator that has to place a value on a clock reads the default; one that
// reads the value's own digits does not, which is the point of not writing the
// default into the value. The split was mapped by a CQL engine measuring its
// operators at three request offsets, and these are this engine's equivalents.

func withDefault(t *testing.T, written string, offset time.Duration) types.DateTime {
	t.Helper()

	value, err := types.NewDateTime(written)
	if err != nil {
		t.Fatalf("NewDateTime(%q): %v", written, err)
	}
	return value.WithDefaultOffset(offset)
}

func answer(t *testing.T, expr string, vars map[string]types.Value) string {
	t.Helper()

	ctx := eval.NewContext([]byte(`{}`))
	for name, value := range vars {
		ctx.SetVariable(name, types.Collection{value})
	}

	result, err := fhirpath.MustCompile(expr).EvaluateWithContext(ctx)
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	if result.Empty() {
		return "{}"
	}
	if len(result) != 1 {
		t.Fatalf("%s: got %d results, want 1", expr, len(result))
	}
	return result[0].String()
}

// Placing a value is what the default is for, so these move with it.
func TestDefaultOffsetReachesWhatPlacesAValue(t *testing.T) {
	instant, err := types.NewDateTime("2020-01-01T02:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		offset           time.Duration
		afterInstant     string
		equalsInstant    string
		hoursFromInstant string
		extractedOffset  string
	}{
		{0, "false", "false", "2", "0.0"},
		{-5 * time.Hour, "true", "false", "-3", "-5.0"},
		{9 * time.Hour, "false", "false", "11", "9.0"},
	}

	for _, tt := range tests {
		t.Run(tt.offset.String(), func(t *testing.T) {
			vars := map[string]types.Value{
				"bare":    withDefault(t, "2020-01-01T00:00:00.0", tt.offset),
				"instant": instant,
			}

			if got := answer(t, "%bare > %instant", vars); got != tt.afterInstant {
				t.Errorf("ordering: got %s, want %s", got, tt.afterInstant)
			}
			if got := answer(t, "%bare = %instant", vars); got != tt.equalsInstant {
				t.Errorf("equality: got %s, want %s", got, tt.equalsInstant)
			}
			if got := answer(t, "%bare.duration(%instant, 'hour')", vars); got != tt.hoursFromInstant {
				t.Errorf("duration: got %s, want %s", got, tt.hoursFromInstant)
			}
			if got := answer(t, "%bare.timezoneOffsetOf()", vars); got != tt.extractedOffset {
				t.Errorf("timezoneOffsetOf: got %s, want %s", got, tt.extractedOffset)
			}
		})
	}
}

// Reading the value's own digits does not, whatever offset was supplied: the
// default was held apart from what was written precisely so that a value still
// says what it says.
func TestDefaultOffsetDoesNotReachTheValueAsWritten(t *testing.T) {
	for _, offset := range []time.Duration{0, 14 * time.Hour, -11 * time.Hour} {
		t.Run(offset.String(), func(t *testing.T) {
			vars := map[string]types.Value{
				"v": withDefault(t, "2020-06-15T23:00:00.0", offset),
			}

			for _, tc := range []struct{ expr, want string }{
				{"%v.toString()", "2020-06-15T23:00:00.000"},
				{"%v.dateOf()", "2020-06-15"},
				{"%v.hourOf()", "23"},
				{"%v.dayOf()", "15"},
			} {
				if got := answer(t, tc.expr, vars); got != tc.want {
					t.Errorf("%s: got %s, want %s", tc.expr, got, tc.want)
				}
			}
		})
	}
}

// A placed value stays placed through arithmetic and through everything that
// compares values: a shifted value is the same value moved, and one that
// ordering can place is one that membership and the boundaries can place too.
func TestDefaultOffsetSurvivesArithmeticAndReachesEveryComparison(t *testing.T) {
	instant, err := types.NewDateTime("2020-01-01T02:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	vars := map[string]types.Value{
		"v": withDefault(t, "2020-01-01T00:00:00.0", -5*time.Hour),
		"i": instant,
	}

	// Arithmetic carries it: the result is the same value moved.
	if got := answer(t, "(%v + 1 day).timezoneOffsetOf()", vars); got != "-5.0" {
		t.Errorf("after a shift the value reports offset %s, want -5.0", got)
	}
	if got := answer(t, "(%v + 1 day).duration(%i, 'hour')", vars); got != "-27" {
		t.Errorf("duration after a shift: got %s, want -27 (the -3 it was, a day later)", got)
	}

	// The boundaries bound the instant it names, not the span of every offset
	// it might have had.
	if got := answer(t, "%v.lowBoundary(17)", vars); got != "2020-01-01T00:00:00.000-05:00" {
		t.Errorf("lowBoundary: got %s, want the value at the offset it was placed at", got)
	}

	// One instant written two ways is one value, to every operator that
	// compares.
	stated, err := types.NewDateTime("2020-01-01T05:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	pair := map[string]types.Value{
		"a": withDefault(t, "2020-01-01T00:00:00.0", -5*time.Hour),
		"b": stated,
	}

	for _, tc := range []struct{ expr, want string }{
		{"%a = %b", "true"},
		{"%a ~ %b", "true"},
		{"%a in %b", "true"},
		{"(%a | %b).count()", "1"},
	} {
		if got := answer(t, tc.expr, pair); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.expr, got, tc.want)
		}
	}
}

// Two bare values given the same default are shifted equally, so the shift
// cancels — which is what a caller applying one evaluation-request offset to
// every value produces.
func TestDefaultOffsetCancelsWhenBothSidesAreBare(t *testing.T) {
	for _, offset := range []time.Duration{0, 14 * time.Hour, -11 * time.Hour} {
		t.Run(offset.String(), func(t *testing.T) {
			vars := map[string]types.Value{
				"a": withDefault(t, "2020-06-15T23:00:00.0", offset),
				"b": withDefault(t, "2020-06-15T22:00:00.0", offset),
			}

			if got := answer(t, "%a.duration(%b, 'day')", vars); got != "0" {
				t.Errorf("days between two bare values: got %s, want 0", got)
			}
			if got := answer(t, "%a > %b", vars); got != "true" {
				t.Errorf("ordering two bare values: got %s, want true", got)
			}
		})
	}
}

// The cancellation is a property of the defaults being equal, not of the
// operators: a default is supplied per value, and two bare values carrying
// different ones do not cancel.
func TestDefaultOffsetDoesNotCancelWhenTheDefaultsDiffer(t *testing.T) {
	vars := map[string]types.Value{
		"a": withDefault(t, "2020-06-15T12:00:00.0", -5*time.Hour),
		"b": withDefault(t, "2020-06-15T12:00:00.0", 9*time.Hour),
	}

	if got := answer(t, "%a.duration(%b, 'hour')", vars); got != "-14" {
		t.Errorf("got %s, want -14 — the two were placed fourteen hours apart", got)
	}
}

// A value with no time of day has no instant to place, so nothing reaches it
// and a comparison against one is decided by precision as it was before.
func TestDefaultOffsetDoesNotReachAFramelessValue(t *testing.T) {
	date, err := types.NewDate("2020-01-01")
	if err != nil {
		t.Fatal(err)
	}

	for _, offset := range []time.Duration{0, 14 * time.Hour, -11 * time.Hour} {
		t.Run(offset.String(), func(t *testing.T) {
			vars := map[string]types.Value{
				"date":     date,
				"datetime": withDefault(t, "2020-01-01T10:00:00", offset),
			}

			// Empty for want of a shared precision, not because of an offset.
			if got := answer(t, "%date = %datetime", vars); got != "{}" {
				t.Errorf("got %s, want {}", got)
			}
			if got := answer(t, "%date.duration(%datetime, 'day')", vars); got != "0" {
				t.Errorf("days between: got %s, want 0 — the date is not shifted into a frame it has none of", got)
			}
		})
	}
}

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

// TestDateTimeComparisonAcrossTimezones covers the condition the specification
// places on comparing datetimes:
//
//	To support comparison of DateTime values, either both values have no
//	timezone offset specified, or both values are converted to a common
//	timezone offset.
//
// Neither holds when one side carries an offset and the other does not, and
// there is nothing to convert the bare value into. The answer is unknown rather
// than false: @2012-04-15T15:00:00Z and @2012-04-15T10:00:00 are the same
// instant at UTC-5 and different ones anywhere else, and the value does not say
// which it meant.
func TestDateTimeComparisonAcrossTimezones(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// One offset, one none
		{"@2012-04-15T15:00:00Z = @2012-04-15T10:00:00", "EMPTY"},
		{"@2012-04-15T15:00:00Z != @2012-04-15T10:00:00", "EMPTY"},
		{"@2012-04-15T15:00:00Z > @2012-04-15T10:00:00", "EMPTY"},
		// Even when the components would agree
		{"@2012-04-15T10:00:00Z = @2012-04-15T10:00:00", "EMPTY"},

		// Both carry one: the specification's own worked examples
		{"@2017-11-05T01:30:00.0-04:00 > @2017-11-05T01:15:00.0-05:00", "false"},
		{"@2017-11-05T01:30:00.0-04:00 < @2017-11-05T01:15:00.0-05:00", "true"},
		{"@2017-11-05T01:30:00.0-04:00 = @2017-11-05T01:15:00.0-05:00", "false"},
		{"@2017-11-05T01:30:00.0-04:00 = @2017-11-05T00:30:00.0-05:00", "true"},

		// Neither carries one
		{"@2012-01-01T10:30 = @2012-01-01T10:30", "true"},
		{"@2012-01-01T10:30 = @2012-01-01T10:31", "false"},
		{"@2012-01-01T10:30:31.0 = @2012-01-01T10:30:31", "true"},

		// A date has no clock, so the rule does not reach it
		{"@2012 = @2012", "true"},
		{"@2012 = @2013", "false"},
		{"@2012-01 = @2012", "EMPTY"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestBoundaryTimezoneOffsets verifies that lowBoundary/highBoundary add
// spec-mandated timezone offsets for DateTime without explicit TZ.
func TestBoundaryTimezoneOffsets(t *testing.T) {
	tests := []struct {
		name      string
		resource  string
		expr      string
		wantFirst string
	}{
		{
			name:      "lowBoundary adds +14:00 for dateTime without TZ",
			resource:  `{"resourceType":"Observation","effectiveDateTime":"2010-10-10T10:00:00"}`,
			expr:      "Observation.effectiveDateTime.lowBoundary()",
			wantFirst: "2010-10-10T10:00:00.000+14:00",
		},
		{
			name:      "highBoundary adds -12:00 for dateTime without TZ",
			resource:  `{"resourceType":"Observation","effectiveDateTime":"2010-10-10T10:00:00"}`,
			expr:      "Observation.effectiveDateTime.highBoundary()",
			wantFirst: "2010-10-10T10:00:00.999-12:00",
		},
		{
			name:      "lowBoundary preserves existing TZ",
			resource:  `{"resourceType":"Observation","effectiveDateTime":"2010-10-10T10:00:00+02:00"}`,
			expr:      "Observation.effectiveDateTime.lowBoundary()",
			wantFirst: "2010-10-10T10:00:00.000+02:00",
		},
		{
			name:      "highBoundary preserves existing TZ",
			resource:  `{"resourceType":"Observation","effectiveDateTime":"2010-10-10T10:00:00+02:00"}`,
			expr:      "Observation.effectiveDateTime.highBoundary()",
			wantFirst: "2010-10-10T10:00:00.999+02:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(tt.resource), tt.expr)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if result.Empty() {
				t.Fatal("expected non-empty result")
			}
			if result[0].String() != tt.wantFirst {
				t.Errorf("expected %q, got %q", tt.wantFirst, result[0].String())
			}
		})
	}
}
