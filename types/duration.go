package types

import (
	"errors"
	"fmt"
	"time"
)

// ErrDateComponentOnTime reports an attempt to shift a time of day by a unit
// that measures the calendar.
//
// A time carries no date, so there is nothing for a day or a month to move.
// "If there is more than one item, an item of an incompatible type, or an
// unsupported unit for the type, the evaluation of the expression will end and
// signal an error ... This includes attempting to add date components to a Time."
var ErrDateComponentOnTime = errors.New("a date component cannot shift a time")

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
// durationShift is how far, and along which field, one unit moves a temporal.
type durationShift struct {
	field      durationField
	multiplier int
}

// calendarShifts maps each calendar keyword onto the field it moves. Weeks are
// the one keyword that is not a field of its own: a week is seven days, both to
// the grammar and to the calendar.
var calendarShifts = map[string]durationShift{
	unitNameYear:        {fieldYear, 1},
	unitNameMonth:       {fieldMonth, 1},
	unitNameWeek:        {fieldDay, 7},
	unitNameDay:         {fieldDay, 1},
	unitNameHour:        {fieldHour, 1},
	unitNameMinute:      {fieldMinute, 1},
	unitNameSecond:      {fieldSecond, 1},
	unitNameMillisecond: {fieldMillisecond, 1},
}

// ucumShifts maps the UCUM codes for definite durations of a week or less, which
// convert exactly to days or smaller and so move the same fields. The UCUM codes
// for years and months are deliberately absent: see ucumCalendarUnits.
var ucumShifts = map[string]durationShift{
	"wk":  {fieldDay, 7},
	"d":   {fieldDay, 1},
	"h":   {fieldHour, 1},
	"min": {fieldMinute, 1},
	"s":   {fieldSecond, 1},
	"ms":  {fieldMillisecond, 1},
}

// durationUnits maps every unit that can shift a temporal value onto the field
// it moves and by how much, taking the calendar keywords in both their singular
// and plural spellings alongside the UCUM codes.
var durationUnits = buildDurationUnits()

func buildDurationUnits() map[string]durationShift {
	units := make(map[string]durationShift, len(calendarShifts)*2+len(ucumShifts))
	for name, shift := range calendarShifts {
		units[name] = shift
		units[name+"s"] = shift
	}
	for code, shift := range ucumShifts {
		units[code] = shift
	}
	return units
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

// shiftCalendarMonths moves a year, month and day by whole years and months,
// clamping the day to the end of the month it lands in.
//
// The clamp is what the specification asks for, in both the year and the month
// rows of its arithmetic table: "If the month and day of the date or time value
// is not a valid date in the resulting year, the last day of the calendar month
// is used." Go's AddDate normalizes instead — it rolls 2017-02-29 forward to
// 2017-03-01 — which is a different answer, and the wrong one here. Adding a
// year to the 29th of February lands on the 28th, not in March.
//
// Days are not shifted here. They overflow month and year boundaries normally,
// which the same table calls for, so they are left to ordinary date arithmetic.
func shiftCalendarMonths(year, month, day, deltaYears, deltaMonths int) (shiftedYear, shiftedMonth, shiftedDay int) {
	total := year*12 + (month - 1) + deltaYears*12 + deltaMonths

	shiftedYear = total / 12
	shiftedMonth = total%12 + 1
	if total < 0 && total%12 != 0 {
		// Go truncates division towards zero, which for a negative total would
		// put the result a month into the following year
		shiftedYear--
		shiftedMonth += 12
	}

	shiftedDay = day
	if last := daysInMonth(shiftedYear, shiftedMonth); shiftedDay > last {
		shiftedDay = last
	}

	return shiftedYear, shiftedMonth, shiftedDay
}

// daysInMonth returns the length of a calendar month, which is where leap years
// enter: day zero of the following month is the last day of this one.
func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
