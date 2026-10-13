package types

import (
	"errors"
	"math"
	"testing"
)

// The shift API refuses an amount past the calendar, or one whose product
// past the int range would wrap, rather than wrapping it.
func TestAShiftByAnAmountPastTheCalendarIsRefused(t *testing.T) {
	date := Date{year: 2020, month: 1, day: 1, precision: DayPrecision}
	for _, tc := range []struct {
		value int
		unit  string
	}{
		{math.MaxInt / 2, "weeks"},
		{math.MaxInt, "days"},
		{10001, "years"},
	} {
		if _, err := date.AddDuration(tc.value, tc.unit); !errors.Is(err, ErrInvalidTemporal) {
			t.Errorf("Date.AddDuration(%d, %s): error %v, want ErrInvalidTemporal", tc.value, tc.unit, err)
		}
	}

	dateTime := DateTime{year: 2020, month: 1, day: 1, hour: 10, precision: DTSecondPrecision}
	if _, err := dateTime.AddDuration(math.MaxInt/1000, "seconds"); !errors.Is(err, ErrInvalidTemporal) {
		t.Errorf("DateTime.AddDuration past the calendar: error %v, want ErrInvalidTemporal", err)
	}
	shifted, err := dateTime.AddDuration(3000000, "hours")
	if err != nil || shifted.year != 2362 {
		t.Errorf("DateTime.AddDuration(3000000, hours) = %v, %v; want the year 2362", shifted, err)
	}

	clock := Time{hour: 10, precision: MinutePrecision}
	if _, err := clock.AddDuration(0, "days"); !errors.Is(err, ErrDateComponentOnTime) {
		t.Errorf("Time.AddDuration(0, days): error %v, want ErrDateComponentOnTime", err)
	}
}
