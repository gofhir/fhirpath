package types

import (
	"errors"
	"testing"
	"time"
)

// The case this exists for, written the way a CQL engine meets it: a measurement
// period declared without an offset, against served FHIR data that always has
// one. Without the default the comparison has no answer, and a caller that drops
// what it cannot confirm loses the encounter.
func TestDefaultOffsetAnswersTheMeasurementPeriodCase(t *testing.T) {
	encounter := mustDateTime(t, "2020-01-01T02:00:00Z")
	periodStart := mustDateTime(t, "2020-01-01T00:00:00.0")

	if _, err := encounter.Compare(periodStart); !errors.Is(err, ErrOffsetMismatch) {
		t.Fatalf("without a default the comparison should have no answer, got %v", err)
	}

	// The caller says what its language defines: the evaluation request was at
	// UTC, so that is the period's offset.
	cmp, err := encounter.Compare(periodStart.WithDefaultOffset(0))
	if err != nil {
		t.Fatalf("with the default supplied: %v", err)
	}
	if cmp != 1 {
		t.Errorf("got %d, want 1 — the encounter starts two hours into the period", cmp)
	}
}

// The default must not become part of the value. A literal written without an
// offset still evaluates to itself, which is what a CQL corpus requires of the
// same values a default is supplied for — twenty-two of its cases read the
// value's representation.
func TestDefaultOffsetDoesNotMaterialize(t *testing.T) {
	written := mustDateTime(t, "2012-03-10T10:20:00")
	defaulted := written.WithDefaultOffset(-5 * time.Hour)

	if got := defaulted.String(); got != written.String() {
		t.Errorf("the value now prints as %q, where it was written %q", got, written.String())
	}
	if defaulted.HasTZ() {
		t.Error("HasTZ is true, so the value claims to state an offset it does not")
	}
	if got := defaulted.TZOffset(); got != 0 {
		t.Errorf("TZOffset is %d, but the value states no offset", got)
	}

	// What the default is for: the offset to place the value at.
	minutes, ok := defaulted.EffectiveOffset()
	if !ok || minutes != -300 {
		t.Errorf("EffectiveOffset() = %d, %v; want -300, true", minutes, ok)
	}
}

// Ordering and duration read the same offset, so supplying one moves both. They
// disagree without it: the comparison declines while the duration answers from
// UTC.
func TestDefaultOffsetMovesOrderingAndDurationTogether(t *testing.T) {
	encounter := mustDateTime(t, "2020-01-01T02:00:00Z")

	tests := []struct {
		name     string
		offset   time.Duration
		after    bool
		duration int64
	}{
		{"at UTC", 0, true, 2},
		{"at UTC-5", -5 * time.Hour, false, -3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			period := mustDateTime(t, "2020-01-01T00:00:00.0").WithDefaultOffset(tt.offset)

			cmp, err := encounter.Compare(period)
			if err != nil {
				t.Fatalf("comparing: %v", err)
			}
			if after := cmp > 0; after != tt.after {
				t.Errorf("encounter after the period start = %v, want %v", after, tt.after)
			}

			hours, err := TemporalDuration(period, encounter, "hour")
			if err != nil {
				t.Fatalf("measuring: %v", err)
			}
			if hours != tt.duration {
				t.Errorf("hours between = %d, want %d", hours, tt.duration)
			}
		})
	}
}

// The offset is the caller's to state, so a different one gives a different
// answer — which is the whole point of not guessing it.
func TestDefaultOffsetIsTheCallersToChoose(t *testing.T) {
	encounter := mustDateTime(t, "2020-01-01T02:00:00Z")
	periodStart := mustDateTime(t, "2020-01-01T00:00:00.0")

	// An offset east of UTC puts the same wall-clock time earlier in UTC, so the
	// period opens sooner and the encounter is further inside it. West of UTC
	// the period opens later, and 02:00Z falls before it starts.
	tests := []struct {
		name   string
		offset time.Duration
		want   int
	}{
		{"at UTC the period opens at 00:00Z and the encounter is inside", 0, 1},
		{"three hours east it opens at 21:00Z the day before, still inside", 3 * time.Hour, 1},
		{"five hours west it opens at 05:00Z, so the encounter is before it", -5 * time.Hour, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp, err := encounter.Compare(periodStart.WithDefaultOffset(tt.offset))
			if err != nil {
				t.Fatalf("%v", err)
			}
			if cmp != tt.want {
				t.Errorf("got %d, want %d", cmp, tt.want)
			}
		})
	}
}

// A value that states its own offset keeps it: the default fills a gap, it does
// not overrule what was written.
func TestDefaultOffsetLeavesAWrittenOneAlone(t *testing.T) {
	written := mustDateTime(t, "2020-01-01T00:00:00.0+05:00")
	withDefault := written.WithDefaultOffset(0)

	if got := withDefault.TZOffset(); got != 300 {
		t.Errorf("offset is now %d minutes, want the 300 it was written with", got)
	}
	minutes, ok := withDefault.EffectiveOffset()
	if !ok || minutes != 300 {
		t.Errorf("EffectiveOffset() = %d, %v; want 300, true", minutes, ok)
	}
}

// The unit is in the call, so it cannot be read wrong: -5 * time.Hour is five
// hours west and -5 * time.Minute is five minutes west, and neither is a guess.
func TestDefaultOffsetTakesTheUnitFromTheCall(t *testing.T) {
	bare := mustDateTime(t, "2020-01-01T00:00:00.0")

	tests := []struct {
		offset time.Duration
		want   int
	}{
		{-5 * time.Hour, -300},
		{-5 * time.Minute, -5},
		{5*time.Hour + 45*time.Minute, 345},
	}

	for _, tt := range tests {
		t.Run(tt.offset.String(), func(t *testing.T) {
			minutes, ok := bare.WithDefaultOffset(tt.offset).EffectiveOffset()
			if !ok || minutes != tt.want {
				t.Errorf("EffectiveOffset() = %d, %v; want %d, true", minutes, ok, tt.want)
			}
		})
	}
}

// An offset no timezone can have is not applied, so the comparison declines
// rather than answering from an invented instant.
func TestDefaultOffsetRejectsWhatIsNotAnOffset(t *testing.T) {
	bare := mustDateTime(t, "2020-01-01T00:00:00.0")

	for _, offset := range []time.Duration{
		20 * time.Hour,  // beyond the fourteen hours a timezone can take
		-20 * time.Hour, //
		30 * time.Second,
		90 * time.Millisecond,
	} {
		t.Run(offset.String(), func(t *testing.T) {
			if _, ok := bare.WithDefaultOffset(offset).EffectiveOffset(); ok {
				t.Errorf("%s was accepted, and no timezone has that offset", offset)
			}
		})
	}

	// The bounds themselves are offsets that exist.
	for _, offset := range []time.Duration{14 * time.Hour, -14 * time.Hour, 0} {
		if _, ok := bare.WithDefaultOffset(offset).EffectiveOffset(); !ok {
			t.Errorf("%s was rejected, but that offset is a real one", offset)
		}
	}
}

// A value whose precision stops above the time of day has no instant to place,
// so there is nothing for an offset to mean and it is left alone.
func TestDefaultOffsetNeedsATimeOfDay(t *testing.T) {
	for _, written := range []string{"2020", "2020-01", "2020-01-01"} {
		t.Run(written, func(t *testing.T) {
			value := mustDateTime(t, written)
			if _, ok := value.WithDefaultOffset(0).EffectiveOffset(); ok {
				t.Errorf("%q was given an offset, which it has no time of day to carry", written)
			}
		})
	}
}

// Nothing changes for a caller that does not ask: FHIRPath keeps seeing empty.
func TestDefaultOffsetIsOptOnly(t *testing.T) {
	left := mustDateTime(t, "2020-03-05T10:00:00.0Z")
	right := mustDateTime(t, "2021-01-01T00:00:00.0")

	if _, err := left.Compare(right); !errors.Is(err, ErrOffsetMismatch) {
		t.Errorf("got %v, want ErrOffsetMismatch — the default must not apply on its own", err)
	}
}
