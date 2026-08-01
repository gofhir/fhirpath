package types

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// Date represents a FHIRPath date value.
// Supports partial dates: year, year-month, year-month-day.
type Date struct {
	year      int
	month     int // 0 if not specified
	day       int // 0 if not specified
	precision DatePrecision
	fhirType  string // FHIR type when the value was read through a model
}

// DatePrecision indicates the precision of a date.
type DatePrecision int

const (
	YearPrecision DatePrecision = iota
	MonthPrecision
	DayPrecision
)

// Date regex patterns
var (
	dateYearPattern  = regexp.MustCompile(`^(\d{4})$`)
	dateMonthPattern = regexp.MustCompile(`^(\d{4})-(\d{2})$`)
	dateDayPattern   = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
)

// NewDate creates a Date from a string.
func NewDate(s string) (Date, error) {
	// Try full date first
	if matches := dateDayPattern.FindStringSubmatch(s); matches != nil {
		year, err := strconv.Atoi(matches[1])
		if err != nil {
			return Date{}, fmt.Errorf("invalid year in date: %s", s)
		}
		month, err := strconv.Atoi(matches[2])
		if err != nil {
			return Date{}, fmt.Errorf("invalid month in date: %s", s)
		}
		day, err := strconv.Atoi(matches[3])
		if err != nil {
			return Date{}, fmt.Errorf("invalid day in date: %s", s)
		}
		return Date{year: year, month: month, day: day, precision: DayPrecision}, nil
	}

	// Try year-month
	if matches := dateMonthPattern.FindStringSubmatch(s); matches != nil {
		year, err := strconv.Atoi(matches[1])
		if err != nil {
			return Date{}, fmt.Errorf("invalid year in date: %s", s)
		}
		month, err := strconv.Atoi(matches[2])
		if err != nil {
			return Date{}, fmt.Errorf("invalid month in date: %s", s)
		}
		return Date{year: year, month: month, precision: MonthPrecision}, nil
	}

	// Try year only
	if matches := dateYearPattern.FindStringSubmatch(s); matches != nil {
		year, err := strconv.Atoi(matches[1])
		if err != nil {
			return Date{}, fmt.Errorf("invalid year in date: %s", s)
		}
		return Date{year: year, precision: YearPrecision}, nil
	}

	return Date{}, fmt.Errorf("invalid date format: %s", s)
}

// NewDateFromTime creates a Date from a time.Time.
func NewDateFromTime(t time.Time) Date {
	return Date{
		year:      t.Year(),
		month:     int(t.Month()),
		day:       t.Day(),
		precision: DayPrecision,
	}
}

// Type returns the type name.
func (d Date) Type() string {
	if d.fhirType != "" {
		return d.fhirType
	}
	return "Date"
}

// Equal checks equality with another value.
func (d Date) Equal(other Value) bool {
	if o, ok := other.(Date); ok {
		if d.precision != o.precision {
			return false
		}
		if d.year != o.year {
			return false
		}
		if d.precision >= MonthPrecision && d.month != o.month {
			return false
		}
		if d.precision >= DayPrecision && d.day != o.day {
			return false
		}
		return true
	}
	return false
}

// Equivalent checks equivalence with another value.
func (d Date) Equivalent(other Value) bool {
	return d.Equal(other)
}

// String returns the string representation.
func (d Date) String() string {
	switch d.precision {
	case YearPrecision:
		return fmt.Sprintf("%04d", d.year)
	case MonthPrecision:
		return fmt.Sprintf("%04d-%02d", d.year, d.month)
	default:
		return fmt.Sprintf("%04d-%02d-%02d", d.year, d.month, d.day)
	}
}

// IsEmpty returns false for Date.
func (d Date) IsEmpty() bool {
	return false
}

// Year returns the year component.
func (d Date) Year() int {
	return d.year
}

// Month returns the month component (0 if not specified).
func (d Date) Month() int {
	return d.month
}

// Day returns the day component (0 if not specified).
func (d Date) Day() int {
	return d.day
}

// Precision returns the date precision.
func (d Date) Precision() DatePrecision {
	return d.precision
}

// ToTime converts to time.Time (uses defaults for missing components).
func (d Date) ToTime() time.Time {
	month := d.month
	if month == 0 {
		month = 1
	}
	day := d.day
	if day == 0 {
		day = 1
	}
	return time.Date(d.year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// Compare compares two dates. Returns -1, 0, or 1.
// Implements the Comparable interface.
// Returns empty (error) if precisions differ and comparison is ambiguous.
func (d Date) Compare(other Value) (int, error) {
	if _, ok := other.(Date); !ok {
		if _, isDateTime := other.(DateTime); !isDateTime {
			return 0, fmt.Errorf("cannot compare Date with %s", other.Type())
		}
	}
	return compareTemporalValues(d, other)
}

// AddDuration adds a duration (as Quantity with temporal unit) to the date.
// Supported units: year(s), month(s), week(s), day(s)
func (d Date) AddDuration(value int, unit string) (Date, error) {
	years, months, days, millis, err := durationParts(value, unit)
	if err != nil {
		return Date{}, err
	}

	// A date has no time of day, so a sub-day duration cannot move it. The
	// specification keeps the value unchanged rather than rounding.
	shifted := d.ToTime().AddDate(years, months, days)
	if millis != 0 {
		shifted = shifted.Add(time.Duration(millis) * time.Millisecond)
	}

	result := Date{
		year:      shifted.Year(),
		month:     int(shifted.Month()),
		day:       shifted.Day(),
		precision: d.precision,
	}

	// A shift never makes the value more precise than it was
	if d.precision < MonthPrecision {
		result.month = 0
	}
	if d.precision < DayPrecision {
		result.day = 0
	}

	return result, nil
}

// SubtractDuration subtracts a duration from the date.
func (d Date) SubtractDuration(value int, unit string) (Date, error) {
	return d.AddDuration(-value, unit)
}

// WithFHIRType returns a copy that reports the FHIR type it was declared with.
// FHIR primitives are types in their own right — a FHIR.boolean is not a
// System.Boolean — so a value keeps the name the model gave it.
func (d Date) WithFHIRType(fhirType string) Date {
	d.fhirType = fhirType
	return d
}
