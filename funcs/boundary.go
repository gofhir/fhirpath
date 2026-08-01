package funcs

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

func init() {
	Register(FuncDef{
		Name:    "lowBoundary",
		MinArgs: 0,
		MaxArgs: 1,
		Fn:      fnLowBoundary,
	})

	Register(FuncDef{
		Name:    "highBoundary",
		MinArgs: 0,
		MaxArgs: 1,
		Fn:      fnHighBoundary,
	})

	Register(FuncDef{
		Name:    "precision",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnPrecision,
	})
}

// fnLowBoundary returns the lowest possible value for the input based on its precision.
//
//nolint:dupl // mirrors fnHighBoundary intentionally; extracting shared logic would hurt readability
func fnLowBoundary(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	precision, provided := extractPrecisionArg(args)

	return boundaryOf(input[0], precision, provided, true)
}

// fnHighBoundary returns the highest possible value for the input based on its precision.
//
//nolint:dupl // mirrors fnLowBoundary intentionally; extracting shared logic would hurt readability
func fnHighBoundary(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	precision, provided := extractPrecisionArg(args)

	return boundaryOf(input[0], precision, provided, false)
}

// boundaryOf dispatches a boundary computation by input type. low selects which
// end of the interval to return.
//
// Integer is included for language consistency, as the specification notes,
// even though a discrete domain makes the result trivial: it is treated as a
// decimal with no fractional digits, so 1.lowBoundary() is 0.5.
func boundaryOf(value types.Value, precision int, provided, low bool) (types.Collection, error) {
	switch v := value.(type) {
	case types.Decimal:
		return decimalBoundary(v.Value(), v.ImplicitPrecision(), decimalPrecisionOr(precision, provided), low)
	case types.Integer:
		return decimalBoundary(decimal.NewFromInt(v.Value()), 0, decimalPrecisionOr(precision, provided), low)
	case types.Quantity:
		return quantityBoundary(v, precision, provided, low)
	case types.Date:
		return dateBoundary(v, precision, provided, low)
	case types.DateTime:
		return dateTimeBoundary(v, precision, provided, low)
	case types.Time:
		return timeBoundary(v, precision, provided, low)
	default:
		return types.Collection{}, nil
	}
}

// decimalPrecisionOr resolves the output precision for decimal boundaries,
// applying the type's default when none was requested.
func decimalPrecisionOr(precision int, provided bool) int {
	if !provided {
		return defaultDecimalBoundaryPrecision
	}
	return precision
}

// extractPrecisionArg extracts the optional integer precision argument.
// provided is false when the call had no argument, which is distinct from an
// argument of -1: the first means "use the type's default precision", the second
// is an invalid precision and yields empty.
func extractPrecisionArg(args []interface{}) (precision int, provided bool) {
	if len(args) == 0 {
		return 0, false
	}
	if col, ok := args[0].(types.Collection); ok && !col.Empty() {
		if intVal, ok := col[0].(types.Integer); ok {
			return int(intVal.Value()), true
		}
	}
	return 0, false
}

// --- Decimal boundaries ---

// Boundary precision, per the FHIRPath 3.0.0 specification:
//
//	"If no precision is specified, the greatest precision of the type of the
//	 input value is used (i.e. at least 8 for Decimal, 4 for Date, at least 17
//	 for DateTime, and at least 9 for Time)."
//
//	"If the precision is greater than the maximum possible precision of the
//	 implementation, the result is empty."
//
// The maximum is therefore ours to set. FHIR bounds the decimal type — "decimals
// in FHIR cannot have more than 18 digits and a decimal point" — so that is the
// limit applied here.
const (
	defaultDecimalBoundaryPrecision = 8
	maxDecimalBoundaryPrecision     = 18
)

// boundaryInterval returns half of the last significant unit of value, which is
// how far the true value may lie from what was written: a value given to three
// decimal places, 1.587, stands for anything within 0.0005 of it.
//
// Note this depends on the precision of the *input*, independent of the
// precision requested for the output.
func boundaryInterval(inputPrecision int) decimal.Decimal {
	unit := decimal.New(1, int32(-inputPrecision))
	return unit.Div(decimal.NewFromInt(2))
}

// decimalBoundary computes one boundary of a decimal value and renders it at the
// requested precision. low selects the lower boundary, rounding away from the
// value in that direction so that the result still bounds it.
func decimalBoundary(value decimal.Decimal, inputPrecision, outputPrecision int, low bool) (types.Collection, error) {
	if outputPrecision < 0 || outputPrecision > maxDecimalBoundaryPrecision {
		return types.Collection{}, nil
	}

	interval := boundaryInterval(inputPrecision)
	places := int32(outputPrecision)

	var bounded decimal.Decimal
	if low {
		bounded = value.Sub(interval).RoundFloor(places)
	} else {
		bounded = value.Add(interval).RoundCeil(places)
	}

	// StringFixed pads to exactly the requested precision, which is significant:
	// 1.587.lowBoundary() is 1.58650000, not 1.5865.
	result, err := types.NewDecimal(bounded.StringFixed(places))
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{result}, nil
}

// --- Temporal boundaries ---
//
// Temporal precision is expressed in digits, counting the significant digits of
// the value: 4 is a year, 6 year and month, 8 a full date, then 10, 12 and 14
// add hours, minutes and seconds, and 17 adds milliseconds. Time counts from the
// hour instead: 2, 4, 6 and 9.
//
// A boundary fills the components the value does not carry with their lowest or
// highest possible value, then presents the result at the requested precision.

// Digit counts for each temporal precision.
const (
	digitsYear        = 4
	digitsMonth       = 6
	digitsDay         = 8
	digitsHour        = 10
	digitsMinute      = 12
	digitsSecond      = 14
	digitsMillisecond = 17

	digitsTimeHour        = 2
	digitsTimeMinute      = 4
	digitsTimeSecond      = 6
	digitsTimeMillisecond = 9

	// Timezones that bound an instant whose offset is unknown: the earliest
	// place on earth reaches a wall-clock time first, the latest one last.
	earliestTimezone = "+14:00"
	latestTimezone   = "-12:00"
)

// dateBoundary returns one boundary of a Date at the requested digit precision.
func dateBoundary(d types.Date, precision int, provided, low bool) (types.Collection, error) {
	if !provided {
		precision = digitsDay
	}

	year, month, day := d.Year(), d.Month(), d.Day()

	if d.Precision() < types.MonthPrecision {
		month = boundaryMonth(low)
	}
	if d.Precision() < types.DayPrecision {
		day = boundaryDay(year, month, low)
	}

	var formatted string
	switch precision {
	case digitsYear:
		formatted = fmt.Sprintf("%04d", year)
	case digitsMonth:
		formatted = fmt.Sprintf("%04d-%02d", year, month)
	case digitsDay:
		formatted = fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	default:
		return types.Collection{}, nil
	}

	result, err := types.NewDate(formatted)
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{result}, nil
}

// dateTimeBoundary returns one boundary of a DateTime at the requested digit
// precision. Below a full date the result carries no time or timezone, so it is
// rendered as a date.
func dateTimeBoundary(dt types.DateTime, precision int, provided, low bool) (types.Collection, error) {
	if !provided {
		precision = digitsMillisecond
	}

	year, month, day := dt.Year(), dt.Month(), dt.Day()
	hour, minute := dt.Hour(), dt.Minute()
	second, millis := dt.Second(), dt.Millisecond()

	if dt.Precision() < types.DTMonthPrecision {
		month = boundaryMonth(low)
	}
	if dt.Precision() < types.DTDayPrecision {
		day = boundaryDay(year, month, low)
	}
	if dt.Precision() < types.DTHourPrecision {
		hour = boundaryComponent(low, 23)
	}
	if dt.Precision() < types.DTMinutePrecision {
		minute = boundaryComponent(low, 59)
	}
	if dt.Precision() < types.DTSecondPrecision {
		second = boundaryComponent(low, 59)
	}
	if dt.Precision() < types.DTMillisPrecision {
		millis = boundaryComponent(low, 999)
	}

	var formatted string
	switch precision {
	case digitsYear:
		formatted = fmt.Sprintf("%04d", year)
	case digitsMonth:
		formatted = fmt.Sprintf("%04d-%02d", year, month)
	case digitsDay:
		formatted = fmt.Sprintf("%04d-%02d-%02d", year, month, day)
	case digitsHour:
		formatted = fmt.Sprintf("%04d-%02d-%02dT%02d", year, month, day, hour)
	case digitsMinute:
		formatted = fmt.Sprintf("%04d-%02d-%02dT%02d:%02d", year, month, day, hour, minute)
	case digitsSecond:
		formatted = fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d", year, month, day, hour, minute, second)
	case digitsMillisecond:
		formatted = fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d.%03d",
			year, month, day, hour, minute, second, millis)
	default:
		return types.Collection{}, nil
	}

	// A timezone only applies once the result carries a time of day. When the
	// value has none, the boundary is the widest possible instant.
	if precision >= digitsHour {
		formatted += boundaryTimezone(dt, low)
	}

	if precision <= digitsDay {
		result, err := types.NewDate(formatted)
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{result}, nil
	}

	result, err := types.NewDateTime(formatted)
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{result}, nil
}

// timeBoundary returns one boundary of a Time at the requested digit precision.
func timeBoundary(t types.Time, precision int, provided, low bool) (types.Collection, error) {
	if !provided {
		precision = digitsTimeMillisecond
	}

	hour, minute := t.Hour(), t.Minute()
	second, millis := t.Second(), t.Millisecond()

	if t.Precision() < types.MinutePrecision {
		minute = boundaryComponent(low, 59)
	}
	if t.Precision() < types.SecondPrecision {
		second = boundaryComponent(low, 59)
	}
	if t.Precision() < types.MillisPrecision {
		millis = boundaryComponent(low, 999)
	}

	var formatted string
	switch precision {
	case digitsTimeHour:
		formatted = fmt.Sprintf("%02d", hour)
	case digitsTimeMinute:
		formatted = fmt.Sprintf("%02d:%02d", hour, minute)
	case digitsTimeSecond:
		formatted = fmt.Sprintf("%02d:%02d:%02d", hour, minute, second)
	case digitsTimeMillisecond:
		formatted = fmt.Sprintf("%02d:%02d:%02d.%03d", hour, minute, second, millis)
	default:
		return types.Collection{}, nil
	}

	result, err := types.NewTime(formatted)
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{result}, nil
}

// boundaryComponent returns the lowest or highest value a time component can
// take, given that the value did not specify it.
func boundaryComponent(low bool, highest int) int {
	if low {
		return 0
	}
	return highest
}

func boundaryMonth(low bool) int {
	if low {
		return 1
	}
	return 12
}

func boundaryDay(year, month int, low bool) int {
	if low {
		return 1
	}
	return daysInMonth(year, month)
}

// boundaryTimezone keeps the value's own offset when it has one; otherwise the
// boundary spans every possible offset.
func boundaryTimezone(dt types.DateTime, low bool) string {
	if dt.HasTZ() {
		return formatTZOffset(dt.TZOffset())
	}
	if low {
		return earliestTimezone
	}
	return latestTimezone
}

// --- Quantity boundaries ---

// quantityBoundary bounds the quantity's value and keeps its unit.
func quantityBoundary(q types.Quantity, precision int, provided, low bool) (types.Collection, error) {
	bounded, err := decimalBoundary(q.Value(), quantityPrecision(q), decimalPrecisionOr(precision, provided), low)
	if err != nil || bounded.Empty() {
		return types.Collection{}, err
	}

	value, ok := bounded[0].(types.Decimal)
	if !ok {
		return types.Collection{}, nil
	}
	return types.Collection{types.NewQuantityFromDecimal(value.Value(), q.Unit())}, nil
}

// quantityPrecision returns the number of fractional digits the quantity's
// value carries. It reads the scale rather than the rendered text, which
// normalizes 1.0 to 1 and would report no fractional digits at all.
func quantityPrecision(q types.Quantity) int {
	if exponent := q.Value().Exponent(); exponent < 0 {
		return int(-exponent)
	}
	return 0
}

// --- Helpers ---

func daysInMonth(year, month int) int {
	return time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func formatTZOffset(offsetMinutes int) string {
	if offsetMinutes == 0 {
		return "Z"
	}
	sign := "+"
	offset := offsetMinutes
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return fmt.Sprintf("%s%02d:%02d", sign, offset/60, offset%60)
}

// --- precision() ---

// fnPrecision returns the number of digits of precision the input carries.
//
// Defined in the FHIRPath 3.0.0 specification as Standard for Trial Use, and
// usable with Decimal, Date, DateTime and Time. For a decimal it counts the
// digits after the decimal point — 1.58700 has five, trailing zeros included,
// because they are significant. For a temporal value it counts the significant
// digits of the value itself: @2014 has four, @T10:30 has four, and
// @T10:30:00.000 has nine.
//
// Returns empty for empty input or for a type without a notion of precision.
func fnPrecision(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	var digits int
	switch v := input[0].(type) {
	case types.Decimal:
		digits = v.ImplicitPrecision()
	case types.Integer:
		// Implicitly converted to a decimal, which has no fractional digits
		digits = 0
	case types.Date:
		digits = datePrecisionDigits(v.Precision())
	case types.DateTime:
		digits = dateTimePrecisionDigits(v.Precision())
	case types.Time:
		digits = timePrecisionDigits(v.Precision())
	default:
		return types.Collection{}, nil
	}

	return types.Collection{types.NewInteger(int64(digits))}, nil
}

func datePrecisionDigits(p types.DatePrecision) int {
	switch p {
	case types.YearPrecision:
		return digitsYear
	case types.MonthPrecision:
		return digitsMonth
	default:
		return digitsDay
	}
}

func dateTimePrecisionDigits(p types.DateTimePrecision) int {
	switch p {
	case types.DTYearPrecision:
		return digitsYear
	case types.DTMonthPrecision:
		return digitsMonth
	case types.DTDayPrecision:
		return digitsDay
	case types.DTHourPrecision:
		return digitsHour
	case types.DTMinutePrecision:
		return digitsMinute
	case types.DTSecondPrecision:
		return digitsSecond
	default:
		return digitsMillisecond
	}
}

func timePrecisionDigits(p types.TimePrecision) int {
	switch p {
	case types.HourPrecision:
		return digitsTimeHour
	case types.MinutePrecision:
		return digitsTimeMinute
	case types.SecondPrecision:
		return digitsTimeSecond
	default:
		return digitsTimeMillisecond
	}
}
