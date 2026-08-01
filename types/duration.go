package types

import (
	"errors"
	"fmt"
)

// ErrCalendarConversionRequired reports that a UCUM year or month was used to
// shift a date or time.
//
// A UCUM year is a fixed span of 365.25 days and a UCUM month a fixed 30.44,
// while adding a calendar year or month moves to the same date in another year
// or month. The FHIRPath specification keeps the two systems apart above
// seconds and requires an explicit conversion to cross between them, so
// @1973-12-25 + 1 'a' does not silently pick one meaning.
var ErrCalendarConversionRequired = errors.New("UCUM year and month durations require explicit conversion to calendar units")

// durationField is the calendar component a duration shifts.
type durationField int

const (
	fieldYear durationField = iota
	fieldMonth
	fieldDay
	fieldHour
	fieldMinute
	fieldSecond
	fieldMillisecond
)

// durationUnits maps every unit that can shift a temporal value onto the field
// it moves and by how much.
//
// Two systems appear here. Calendar keywords come from the grammar (year, month,
// week, ...) and mean "the same point in the next year or month". UCUM codes
// name definite durations; those up to weeks convert exactly to days or smaller,
// so they shift the same fields.
var durationUnits = map[string]struct {
	field      durationField
	multiplier int
}{
	// Calendar units, singular and plural
	"year":         {fieldYear, 1},
	"years":        {fieldYear, 1},
	"month":        {fieldMonth, 1},
	"months":       {fieldMonth, 1},
	"week":         {fieldDay, 7},
	"weeks":        {fieldDay, 7},
	"day":          {fieldDay, 1},
	"days":         {fieldDay, 1},
	"hour":         {fieldHour, 1},
	"hours":        {fieldHour, 1},
	"minute":       {fieldMinute, 1},
	"minutes":      {fieldMinute, 1},
	"second":       {fieldSecond, 1},
	"seconds":      {fieldSecond, 1},
	"millisecond":  {fieldMillisecond, 1},
	"milliseconds": {fieldMillisecond, 1},

	// UCUM definite durations of a week or less
	"wk":  {fieldDay, 7},
	"d":   {fieldDay, 1},
	"h":   {fieldHour, 1},
	"min": {fieldMinute, 1},
	"s":   {fieldSecond, 1},
	"ms":  {fieldMillisecond, 1},
}

// ucumCalendarUnits are the UCUM durations that have no exact calendar meaning.
var ucumCalendarUnits = map[string]bool{
	"a":  true, // year
	"mo": true, // month
}

// resolveDurationUnit determines which calendar field a unit shifts.
// Returns ErrCalendarConversionRequired for a UCUM year or month, and an error
// for a unit that cannot shift a temporal value at all — never a silent no-op.
func resolveDurationUnit(unit string) (durationField, int, error) {
	if ucumCalendarUnits[unit] {
		return 0, 0, fmt.Errorf("%w: %q", ErrCalendarConversionRequired, unit)
	}

	resolved, ok := durationUnits[unit]
	if !ok {
		return 0, 0, fmt.Errorf("%q is not a duration unit", unit)
	}
	return resolved.field, resolved.multiplier, nil
}

// durationParts converts a duration into the arguments time arithmetic needs:
// a year/month/day shift and a sub-day shift in milliseconds.
func durationParts(value int, unit string) (years, months, days, millis int, err error) {
	field, multiplier, err := resolveDurationUnit(unit)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	amount := value * multiplier
	switch field {
	case fieldYear:
		years = amount
	case fieldMonth:
		months = amount
	case fieldDay:
		days = amount
	case fieldHour:
		millis = amount * 3600 * 1000
	case fieldMinute:
		millis = amount * 60 * 1000
	case fieldSecond:
		millis = amount * 1000
	case fieldMillisecond:
		millis = amount
	}
	return years, months, days, millis, nil
}
