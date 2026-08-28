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

// With both operands bare the default shifts them equally and cancels, so it
// changes nothing. This is the part of the rule that has to be measured rather
// than read: it is why an operator's answer moves only when exactly one side
// lacks a stated offset.
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
