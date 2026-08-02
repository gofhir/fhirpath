package types

import (
	"fmt"
	"regexp"
	"strconv"
	gotime "time"
)

// Time represents a FHIRPath time value.
type Time struct {
	hour      int
	minute    int
	second    int
	millis    int
	precision TimePrecision
	fhirType  string // FHIR type when the value was read through a model

	// The FHIR element this value was read with, when it carried one
	primitiveElement
}

// TimePrecision indicates the precision of a time.
type TimePrecision int

const (
	HourPrecision TimePrecision = iota
	MinutePrecision
	SecondPrecision
	MillisPrecision
)

// Time regex pattern
var timePattern = regexp.MustCompile(
	`^T?(\d{2})(?::(\d{2})(?::(\d{2})(?:\.(\d+))?)?)?$`,
)

// NewTime creates a Time from a string.
func NewTime(s string) (Time, error) {
	matches := timePattern.FindStringSubmatch(s)
	if matches == nil {
		return Time{}, fmt.Errorf("invalid time format: %s", s)
	}

	t := Time{}
	precision := HourPrecision

	// Hour (required)
	hour, err := strconv.Atoi(matches[1])
	if err != nil {
		return Time{}, fmt.Errorf("invalid hour in time: %s", s)
	}
	t.hour = hour

	// Minute
	if matches[2] != "" {
		minute, err := strconv.Atoi(matches[2])
		if err != nil {
			return Time{}, fmt.Errorf("invalid minute in time: %s", s)
		}
		t.minute = minute
		precision = MinutePrecision
	}

	// Second
	if matches[3] != "" {
		second, err := strconv.Atoi(matches[3])
		if err != nil {
			return Time{}, fmt.Errorf("invalid second in time: %s", s)
		}
		t.second = second
		precision = SecondPrecision
	}

	// Milliseconds
	if matches[4] != "" {
		ms := matches[4]
		for len(ms) < 3 {
			ms += "0"
		}
		if len(ms) > 3 {
			ms = ms[:3]
		}
		millis, err := strconv.Atoi(ms)
		if err != nil {
			return Time{}, fmt.Errorf("invalid milliseconds in time: %s", s)
		}
		t.millis = millis
		precision = MillisPrecision
	}

	t.precision = precision
	return t, nil
}

// NewTimeFromGoTime creates a Time from time.Time.
func NewTimeFromGoTime(t gotime.Time) Time {
	return Time{
		hour:      t.Hour(),
		minute:    t.Minute(),
		second:    t.Second(),
		millis:    t.Nanosecond() / 1000000,
		precision: MillisPrecision,
	}
}

// Type returns the type name.
func (t Time) Type() string {
	if t.fhirType != "" {
		return t.fhirType
	}
	return TypeNameTime
}

// Equal checks equality with another value.
func (t Time) Equal(other Value) bool {
	if o, ok := other.(Time); ok {
		if t.precision != o.precision {
			return false
		}
		if t.hour != o.hour {
			return false
		}
		if t.precision >= MinutePrecision && t.minute != o.minute {
			return false
		}
		if t.precision >= SecondPrecision && t.second != o.second {
			return false
		}
		if t.precision >= MillisPrecision && t.millis != o.millis {
			return false
		}
		return true
	}
	return false
}

// Equivalent checks equivalence with another value.
func (t Time) Equivalent(other Value) bool {
	return t.Equal(other)
}

// String returns the string representation.
func (t Time) String() string {
	result := fmt.Sprintf("%02d", t.hour)

	if t.precision >= MinutePrecision {
		result += fmt.Sprintf(":%02d", t.minute)
	}
	if t.precision >= SecondPrecision {
		result += fmt.Sprintf(":%02d", t.second)
	}
	if t.precision >= MillisPrecision {
		result += fmt.Sprintf(".%03d", t.millis)
	}

	return result
}

// IsEmpty returns false for Time.
func (t Time) IsEmpty() bool {
	return false
}

// Precision returns the time precision.
func (t Time) Precision() TimePrecision { return t.precision }

// Accessors
func (t Time) Hour() int        { return t.hour }
func (t Time) Minute() int      { return t.minute }
func (t Time) Second() int      { return t.second }
func (t Time) Millisecond() int { return t.millis }

// Compare compares two times. Returns -1, 0, or 1.
// Implements the Comparable interface.
// Returns error if precisions differ and comparison is ambiguous.
func (t Time) Compare(other Value) (int, error) {
	if _, ok := other.(Time); !ok {
		return 0, fmt.Errorf("cannot compare Time with %s", other.Type())
	}
	return compareTemporalValues(t, other)
}

// WithFHIRType returns a copy that reports the FHIR type it was declared with.
// FHIR primitives are types in their own right — a FHIR.boolean is not a
// System.Boolean — so a value keeps the name the model gave it.
func (t Time) WithFHIRType(fhirType string) Time {
	t.fhirType = fhirType
	return t
}

// WithElement returns a copy carrying the FHIR element that accompanied the
// value in the JSON, which is where its extensions and id live.
func (t Time) WithElement(element *ObjectValue) Time {
	t.element = element
	return t
}

// millisecondsPerDay is the length of the cycle a Time value wraps around.
const millisecondsPerDay = 24 * 60 * 60 * 1000

// AddDuration shifts a time of day, wrapping around the day.
//
// "As Time is cyclic, using arithmetic operations + or - on Time types can
// result in overflowing the time value, which will wrap around the beginning of
// the day. So adding 1 hour to @T23:30:00 will wrap around to @T00:30:00, which
// is consistent with the behavior of DateTime values."
//
// A time carries no date, so only the clock units apply. Adding a day to a time
// of day names no value, and the specification says so: "This includes
// attempting to add date components to a Time."
func (t Time) AddDuration(value int, unit string) (Time, error) {
	years, months, days, millis, err := durationParts(value, unit)
	if err != nil {
		return Time{}, err
	}
	if years != 0 || months != 0 || days != 0 {
		return Time{}, fmt.Errorf("%w: %q shifts a date, and a time has none",
			ErrDateComponentOnTime, unit)
	}

	shifted := t.millisecondsOfDay() + millis

	// Go's remainder keeps the sign of the dividend, so a subtraction that runs
	// past midnight would land on a negative clock
	shifted %= millisecondsPerDay
	if shifted < 0 {
		shifted += millisecondsPerDay
	}

	return Time{
		hour:      shifted / (60 * 60 * 1000),
		minute:    shifted / (60 * 1000) % 60,
		second:    shifted / 1000 % 60,
		millis:    shifted % 1000,
		precision: t.precision,
		fhirType:  t.fhirType,
	}, nil
}

// SubtractDuration shifts a time of day backwards, wrapping around the day.
func (t Time) SubtractDuration(value int, unit string) (Time, error) {
	return t.AddDuration(-value, unit)
}

// millisecondsOfDay is the time of day as a single number, which is what makes
// the wrap a remainder rather than a carry through four components.
func (t Time) millisecondsOfDay() int {
	return ((t.hour*60+t.minute)*60+t.second)*1000 + t.millis
}
