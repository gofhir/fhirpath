package types

import (
	"errors"
	"testing"
)

// A comparison with an offset on one side only has no answer, and says so with
// its own sentinel: the precisions are usually equal, and reporting a precision
// mismatch pointed anyone reading the message at the wrong thing.
func TestOffsetMismatchIsNotAPrecisionMismatch(t *testing.T) {
	left := mustDateTime(t, "2020-03-05T10:00:00.0Z")
	right := mustDateTime(t, "2021-01-01T00:00:00.0")

	if left.Precision() != right.Precision() {
		t.Fatalf("the two values differ in precision (%d vs %d), which is not the case under test",
			left.Precision(), right.Precision())
	}

	_, err := left.Compare(right)
	if !errors.Is(err, ErrOffsetMismatch) {
		t.Errorf("got %v, want ErrOffsetMismatch", err)
	}
	if errors.Is(err, ErrPrecisionMismatch) {
		t.Error("the error reports a precision mismatch, but both values are specified to the millisecond")
	}
	if !IsUnknownTemporalComparison(err) {
		t.Error("IsUnknownTemporalComparison said no, so a caller would raise this instead of answering empty")
	}
}

// The two sentinels do not stand in for each other: a genuine difference in
// precision still reports one.
func TestPrecisionMismatchStillReportsItself(t *testing.T) {
	left := mustDateTime(t, "2020-03-05T10:00")
	right := mustDateTime(t, "2020-03-05T10:00:30")

	_, err := left.Compare(right)
	if !errors.Is(err, ErrPrecisionMismatch) {
		t.Errorf("got %v, want ErrPrecisionMismatch", err)
	}
	if errors.Is(err, ErrOffsetMismatch) {
		t.Error("the error reports an offset mismatch, but neither value carries an offset")
	}
	if !IsUnknownTemporalComparison(err) {
		t.Error("IsUnknownTemporalComparison said no")
	}
}

// Which side carries the offset makes no difference to the answer.
func TestOffsetMismatchIsSymmetric(t *testing.T) {
	withOffset := mustDateTime(t, "2020-03-05T10:00:00.0Z")
	without := mustDateTime(t, "2021-01-01T00:00:00.0")

	if _, err := withOffset.Compare(without); !errors.Is(err, ErrOffsetMismatch) {
		t.Errorf("offset on the left: got %v, want ErrOffsetMismatch", err)
	}
	if _, err := without.Compare(withOffset); !errors.Is(err, ErrOffsetMismatch) {
		t.Errorf("offset on the right: got %v, want ErrOffsetMismatch", err)
	}
}

// The cases that do have an answer keep it. An offset on both sides, or on
// neither, compares normally — and a difference above the time of day decides
// before the offset can matter.
func TestComparisonsThatHaveAnAnswer(t *testing.T) {
	tests := []struct {
		name        string
		left, right string
		want        int
	}{
		{"both carry an offset", "2020-03-05T10:00:00.0Z", "2021-01-01T00:00:00.0Z", -1},
		{"neither carries one", "2020-03-05T10:00:00.0", "2021-01-01T00:00:00.0", -1},
		{"different offsets, same instant", "2017-11-05T01:30:00.0-04:00", "2017-11-05T00:30:00.0-05:00", 0},
		{"different offsets, ordered", "2017-11-05T01:30:00.0-04:00", "2017-11-05T01:15:00.0-05:00", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp, err := mustDateTime(t, tt.left).Compare(mustDateTime(t, tt.right))
			if err != nil {
				t.Fatalf("got error %v, want a comparison", err)
			}
			if cmp != tt.want {
				t.Errorf("got %d, want %d", cmp, tt.want)
			}
		})
	}
}

// A value whose precision stops above the time of day has no offset to be
// missing, so the offset rule does not reach it: @2020-03-05 against
// @2021-01-01T00:00:00Z is decided at the year.
func TestOffsetRuleNeedsATimeOfDayOnBothSides(t *testing.T) {
	date, err := NewDate("2020-03-05")
	if err != nil {
		t.Fatalf("NewDate: %v", err)
	}

	cmp, err := date.Compare(mustDateTime(t, "2021-01-01T00:00:00.0Z"))
	if err != nil {
		t.Fatalf("got error %v, want the year to decide", err)
	}
	if cmp != -1 {
		t.Errorf("got %d, want -1", cmp)
	}
}

func mustDateTime(t *testing.T, s string) DateTime {
	t.Helper()

	value, err := NewDateTime(s)
	if err != nil {
		t.Fatalf("NewDateTime(%q): %v", s, err)
	}
	return value
}
