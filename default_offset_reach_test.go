package fhirpath

import (
	"testing"
	"time"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

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

	result, err := MustCompile(expr).EvaluateWithContext(ctx)
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
