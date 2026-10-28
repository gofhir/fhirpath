package types

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gofhir/fhirpath/internal/decimals"
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

// partsOfField spreads an amount of one field over the year/month/day shift
// and the sub-day shift in milliseconds that time arithmetic takes.
func partsOfField(field durationField, amount int) (years, months, days, millis int) {
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
	return years, months, days, millis
}

// durationPartsAt converts a duration into the arguments time arithmetic
// needs, a year/month/day shift and a sub-day shift in milliseconds, for a
// value of the given precision. A duration finer than the value is converted
// to its precision first: "the operation is performed by converting the
// time-valued quantity to the highest precision in the partial (truncating
// any decimal fraction) and then adding". A year is 12 months or 365 days and
// a month 30 days, as the specification's conversion table has them:
// @2016 + 365 days is @2017, @2026-02 + 4 weeks is @2026-02, and the
// truncation runs toward zero, so @2000 - 1 day is @2000.
func durationPartsAt(value int, unit string, precision durationField) (years, months, days, millis int, err error) {
	field, multiplier, err := resolveDurationUnit(unit)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	amount, ok := multiplyInt(value, multiplier)
	if !ok {
		return 0, 0, 0, 0, pastTheCalendar(value, unit)
	}
	// Seconds and milliseconds are one precision, as comparison has them: the
	// millisecond shifts and is then dropped, so @2000-01-01T00:00:00 - 1 'ms'
	// is @1999-12-31T23:59:59
	if precision == fieldSecond {
		precision = fieldMillisecond
	}
	if field > precision {
		switch {
		case precision == fieldYear && field == fieldMonth:
			amount /= 12
		// Each unit of a day or finer divides the coarser one evenly, so the
		// amount is divided by a whole ratio and never multiplied past int
		case precision == fieldYear:
			amount /= 365 * millisIn(fieldDay) / millisIn(field)
		case precision == fieldMonth:
			amount /= 30 * millisIn(fieldDay) / millisIn(field)
		default:
			amount /= millisIn(precision) / millisIn(field)
		}
		field = precision
	}
	if field > fieldDay {
		// A clock amount becomes milliseconds, which must fit an int too
		if _, ok := multiplyInt(amount, millisIn(field)); !ok {
			return 0, 0, 0, 0, pastTheCalendar(value, unit)
		}
	}
	years, months, days, millis = partsOfField(field, amount)
	return years, months, days, millis, nil
}

// multiplyInt multiplies two ints, reporting false where the product lies past
// the int range.
func multiplyInt(a, b int) (int, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	product := a * b
	if product/b != a || (b == -1 && a == math.MinInt) {
		return 0, false
	}
	return product, true
}

// pastTheCalendar is the error for a shift by an amount that reaches past the
// years 1 to 9999 from any date: "If the resulting year is out of range, an
// error is thrown".
func pastTheCalendar(value any, unit string) error {
	if d, ok := value.(decimal.Decimal); ok {
		value = decimals.Format(d)
	}
	return fmt.Errorf("%w: date arithmetic by %v %s reaches past the years 1 to 9999", ErrInvalidTemporal, value, unit)
}

// checkCalendarAmount reports an amount of a unit longer than the calendar,
// which reaches past the years 1 to 9999 from any date. A unit that is no
// duration is left for the shift to refuse.
func checkCalendarAmount(value int, unit string) error {
	field, multiplier, err := resolveDurationUnit(unit)
	if err != nil {
		return nil
	}
	amount, ok := multiplyInt(value, multiplier)
	if !ok || decimal.NewFromInt(int64(amount)).Abs().GreaterThan(calendarSpan[field]) {
		return pastTheCalendar(value, unit)
	}
	return nil
}

// ShiftAmount converts a time-valued quantity into the whole amount, and the
// unit, that AddDuration and SubtractDuration take, reading the value as a
// decimal, so that no amount wraps past the int range on the way: a value is
// whole periods of its unit, but seconds keep their fraction as milliseconds,
// and a week is seven days.
//
// On a time of day, which is cyclic, a clock unit's amount is taken modulo a
// day: @T10:00 + 9223372036854775807 hours is @T17:00. On a date or a
// date-time, an amount longer than the calendar, ten thousand years in its
// unit, reaches past the years 1 to 9999 from any start, and is reported as
// the error "If the resulting year is out of range, an error is thrown" asks
// for. A unit that is no duration is passed through, for the shift to refuse
// as it does.
func ShiftAmount(value decimal.Decimal, unit string, onClock bool) (amount int, shiftUnit string, err error) {
	field, multiplier, err := resolveDurationUnit(unit)
	if err != nil {
		return 0, "", err
	}

	var total decimal.Decimal
	if field == fieldSecond {
		// Seconds keep their fraction, as milliseconds
		total = value.Mul(decimal.NewFromInt(1000)).Truncate(0)
		field = fieldMillisecond
	} else {
		total = value.Truncate(0).Mul(decimal.NewFromInt(int64(multiplier)))
	}

	if onClock {
		if field < fieldHour {
			// A date component cannot shift a time, whatever its amount, a
			// fraction or none included
			return 0, "", dateComponentOnTime(unit)
		}
		perDay := decimal.NewFromInt(int64(millisIn(fieldDay) / millisIn(field)))
		total = total.Mod(perDay)
	} else if total.Abs().GreaterThan(calendarSpan[field]) {
		return 0, "", pastTheCalendar(value, unit)
	}
	// On a platform whose int is 32 bits, an amount within the calendar may
	// still not fit one: it is past what this engine can shift by
	if total.GreaterThan(maxIntDecimal) || total.LessThan(minIntDecimal) {
		return 0, "", pastTheCalendar(value, unit)
	}
	return int(total.IntPart()), fieldUnitNames[field], nil
}

// clampToInt reads a whole decimal as an int, held at the ends of the int
// range rather than wrapped past them.
func clampToInt(d decimal.Decimal) int {
	switch {
	case d.GreaterThan(maxIntDecimal):
		return math.MaxInt
	case d.LessThan(minIntDecimal):
		return math.MinInt
	}
	return int(d.IntPart())
}

// The ends of the int range as decimals, made once.
var (
	minIntDecimal = decimal.NewFromInt(math.MinInt)
	maxIntDecimal = decimal.NewFromInt(math.MaxInt)
)

// fieldUnitNames names the unit of each duration field, as the shift reads it.
var fieldUnitNames = map[durationField]string{
	fieldYear: unitNameYear, fieldMonth: unitNameMonth, fieldDay: unitNameDay,
	fieldHour: unitNameHour, fieldMinute: unitNameMinute, fieldSecond: unitNameSecond,
	fieldMillisecond: unitNameMillisecond,
}

// calendarSpan is ten thousand years, the whole calendar from year 1 to 9999
// and more, in each duration field: a shift longer than it lands outside the
// calendar from any date.
var calendarSpan = map[durationField]decimal.Decimal{
	fieldYear:        decimal.NewFromInt(10000),
	fieldMonth:       decimal.NewFromInt(10000 * 12),
	fieldDay:         decimal.NewFromInt(10000 * 366),
	fieldHour:        decimal.NewFromInt(10000 * 366 * 24),
	fieldMinute:      decimal.NewFromInt(10000 * 366 * 24 * 60),
	fieldSecond:      decimal.NewFromInt(10000 * 366 * 24 * 3600),
	fieldMillisecond: decimal.NewFromInt(10000 * 366 * 24 * 3600 * 1000),
}

// millisIn returns the milliseconds in one unit of a field of a day or finer.
func millisIn(field durationField) int {
	switch field {
	case fieldDay:
		return 24 * 3600 * 1000
	case fieldHour:
		return 3600 * 1000
	case fieldMinute:
		return 60 * 1000
	case fieldSecond:
		return 1000
	}
	return 1
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
