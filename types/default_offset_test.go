package types

import (
	"errors"
	"testing"
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

// The offset is the caller's to state, so a different one gives a different
// answer — which is the whole point of not guessing it.
func TestDefaultOffsetIsTheCallersToChoose(t *testing.T) {
	encounter := mustDateTime(t, "2020-01-01T02:00:00Z")
	periodStart := mustDateTime(t, "2020-01-01T00:00:00.0")

	// An offset east of UTC puts the same wall-clock time earlier in UTC, so the
	// period opens sooner and the encounter is further inside it. West of UTC
	// the period opens later, and 02:00Z falls before it starts.
	tests := []struct {
		name    string
		minutes int
		want    int
	}{
		{"at UTC the period opens at 00:00Z and the encounter is inside", 0, 1},
		{"three hours east it opens at 21:00Z the day before, still inside", 180, 1},
		{"five hours west it opens at 05:00Z, so the encounter is before it", -300, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp, err := encounter.Compare(periodStart.WithDefaultOffset(tt.minutes))
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
	if written.String() != withDefault.String() {
		t.Errorf("value changed from %q to %q", written.String(), withDefault.String())
	}
}

// Extracting the offset yields it, which is what the language defining the
// default requires: "the result of extracting the timezone offset component
// will be the timezone offset of the evaluation request, not null".
func TestDefaultOffsetIsVisibleToExtraction(t *testing.T) {
	bare := mustDateTime(t, "2020-01-01T00:00:00.0")
	if bare.HasTZ() {
		t.Fatal("the value states an offset before any default is applied")
	}

	defaulted := bare.WithDefaultOffset(-300)
	if !defaulted.HasTZ() {
		t.Error("HasTZ is false, so timezoneOffsetOf() would answer empty")
	}
	if got := defaulted.TZOffset(); got != -300 {
		t.Errorf("got %d minutes, want -300", got)
	}
}

// A value whose precision stops above the time of day has no instant to place,
// so there is nothing for an offset to mean and it is left alone. @2020 at
// UTC-5 would otherwise become comparable to things it is not.
func TestDefaultOffsetNeedsATimeOfDay(t *testing.T) {
	for _, written := range []string{"2020", "2020-01", "2020-01-01"} {
		t.Run(written, func(t *testing.T) {
			value := mustDateTime(t, written)
			if got := value.WithDefaultOffset(0); got.HasTZ() {
				t.Errorf("%q was given an offset, which it has no time of day to carry", written)
			}
		})
	}
}

// An offset no timezone can have is not applied, so the comparison declines
// rather than answering from an invented instant. The unit is minutes, and a
// caller reading timezoneOffsetOf() — which reports hours — would otherwise
// turn UTC-5 into UTC-00:05 without being told.
func TestDefaultOffsetRejectsWhatIsNotAnOffset(t *testing.T) {
	bare := mustDateTime(t, "2020-01-01T00:00:00.0")

	for _, minutes := range []int{5000, -5000, 841, -841} {
		if got := bare.WithDefaultOffset(minutes); got.HasTZ() {
			t.Errorf("WithDefaultOffset(%d) produced %s, which is not an offset a timezone can have",
				minutes, got.String())
		}
	}

	// The bounds themselves are offsets that exist.
	for _, minutes := range []int{840, -840, 0} {
		if got := bare.WithDefaultOffset(minutes); !got.HasTZ() {
			t.Errorf("WithDefaultOffset(%d) was rejected, but that offset is a real one", minutes)
		}
	}

	// Minutes, not hours: -5 is five minutes west, not five hours, and is
	// applied as such rather than guessed at.
	if got := bare.WithDefaultOffset(-5); got.TZOffset() != -5 {
		t.Errorf("got %d minutes, want -5", got.TZOffset())
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
