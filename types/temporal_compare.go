package types

import (
	"errors"
	"time"
)

// ErrPrecisionMismatch reports that two temporal values agree on every precision
// they have in common, but one is specified more precisely than the other, so
// the outcome cannot be determined.
//
// The FHIRPath specification requires the operation to yield empty in this case:
// "If one value is specified to a different level of precision than the other,
// the result is empty ({ }) to indicate that the result of the comparison is
// unknown." Callers translate this sentinel into an empty collection.
//
// Order matters: the comparison walks precisions from the most significant down
// and stops at the first difference, so this is only reached when everything
// shared matches. now() > today() is empty, while now() > @1974-12-25 is true,
// decided at the year.
var ErrPrecisionMismatch = errors.New("temporal values are specified to different precisions")

// ErrOffsetMismatch reports that one of two temporal values carries a timezone
// offset and the other does not, so there is nothing to convert the bare value
// into and the outcome cannot be determined.
//
// The specification leaves this to the implementation: "For DateTime values
// that do not have a timezone offsets, whether or not to provide a default
// timezone offset is a policy decision", and "To support comparison of DateTime
// values, either both values have no timezone offset specified, or both values
// are converted to a common timezone offset." This engine provides no default,
// which is the reading the specification calls the simplest case, so the
// comparison has no answer rather than a wrong one — @2012-04-15T15:00:00Z and
// @2012-04-15T10:00:00 are the same instant at UTC-5 and different ones
// anywhere else.
//
// It is its own sentinel rather than ErrPrecisionMismatch because the two say
// different things about the values: this one is reached when the precisions
// agree exactly, and reporting it as a precision mismatch sent anyone reading
// the message looking in the wrong place.
var ErrOffsetMismatch = errors.New("one temporal value specifies a timezone offset and the other does not")

// IsUnknownTemporalComparison reports whether err means a comparison of two
// temporal values has no answer, rather than that it went wrong. The
// specification expresses both as an empty result, so callers translate them
// alike; asking through this predicate is what keeps a new one from being
// handled in one place and forgotten in another.
func IsUnknownTemporalComparison(err error) bool {
	return errors.Is(err, ErrPrecisionMismatch) || errors.Is(err, ErrOffsetMismatch)
}

var errNotTemporal = errors.New("value is not a temporal that can be compared")

// temporalUnit identifies how precisely a temporal value is specified, on a
// single scale shared by Date, DateTime and Time so the three compare against
// one another.
type temporalUnit int

const (
	unitYear temporalUnit = iota
	unitMonth
	unitDay
	unitHour
	unitMinute
	// unitSecond covers milliseconds too: the specification treats seconds and
	// milliseconds as a single precision compared as a decimal, so
	// @T10:30:00 = @T10:30:00.0 is true rather than unknown.
	unitSecond

	unitCount = int(unitSecond) + 1
)

// temporalValue is a temporal broken into its calendar components, which is how
// the specification compares them — not as instants. Comparing instants would
// answer a different question: @1974-12-25 and @1974-12-25T12:34:00-10:00 fall on
// the same calendar day, and their order is unknown, but as instants they differ.
type temporalValue struct {
	parts [unitCount]int
	unit  temporalUnit

	// Set when the value carries an explicit offset, which lets two values that
	// both have one be compared on the same footing.
	hasOffset bool
	offset    int // minutes east of UTC
}

// compareTemporal orders two temporal values component by component, from the
// most significant down, stopping at the first difference.
//
// Returns ErrPrecisionMismatch when every shared component matches but the
// values are specified to different precisions.
func compareTemporal(left, right temporalValue) (int, error) {
	// Offsets only matter once both values reach a time of day; normalizing to
	// UTC is what "respecting timezone offsets" amounts to.
	if left.hasOffset && right.hasOffset && left.offset != right.offset {
		left = left.inUTC()
		right = right.inUTC()
	}

	// "To support comparison of DateTime values, either both values have no
	// timezone offset specified, or both values are converted to a common
	// timezone offset."
	//
	// Neither holds when one side carries an offset and the other does not:
	// there is nothing to convert the bare value into. See ErrOffsetMismatch,
	// which is what this reports — the precisions may well agree, and often do.
	if left.hasOffset != right.hasOffset && left.unit >= unitHour && right.unit >= unitHour {
		return 0, ErrOffsetMismatch
	}

	shared := left.unit
	if right.unit < shared {
		shared = right.unit
	}

	for unit := unitYear; unit <= shared; unit++ {
		switch {
		case left.parts[unit] < right.parts[unit]:
			return -1, nil
		case left.parts[unit] > right.parts[unit]:
			return 1, nil
		}
	}

	// Equal as far as both are specified
	if left.unit != right.unit {
		return 0, ErrPrecisionMismatch
	}
	return 0, nil
}

// inUTC re-expresses the value's components at UTC, so that two values written
// against different offsets compare on the same footing.
func (v temporalValue) inUTC() temporalValue {
	instant := time.Date(
		v.parts[unitYear], time.Month(v.parts[unitMonth]), v.parts[unitDay],
		v.parts[unitHour], v.parts[unitMinute], v.parts[unitSecond]/1000,
		(v.parts[unitSecond]%1000)*1e6,
		time.FixedZone("", v.offset*60),
	).UTC()

	shifted := v
	shifted.parts = partsOf(instant)
	shifted.offset = 0
	return shifted
}

// partsOf breaks an instant into the component layout used for comparison.
func partsOf(t time.Time) [unitCount]int {
	var parts [unitCount]int
	parts[unitYear] = t.Year()
	parts[unitMonth] = int(t.Month())
	parts[unitDay] = t.Day()
	parts[unitHour] = t.Hour()
	parts[unitMinute] = t.Minute()
	parts[unitSecond] = t.Second()*1000 + t.Nanosecond()/1e6
	return parts
}

// temporalValue converts a Date for comparison. Absent components take their
// lowest value; comparison never reads past the date's own precision.
func (d Date) temporalValue() temporalValue {
	month, day := d.month, d.day
	if month == 0 {
		month = 1
	}
	if day == 0 {
		day = 1
	}

	unit := unitDay
	switch d.precision {
	case YearPrecision:
		unit = unitYear
	case MonthPrecision:
		unit = unitMonth
	}

	v := temporalValue{unit: unit}
	v.parts[unitYear] = d.year
	v.parts[unitMonth] = month
	v.parts[unitDay] = day
	return v
}

// temporalValue converts a DateTime for comparison, keeping its offset.
func (dt DateTime) temporalValue() temporalValue {
	unit := unitSecond
	switch dt.precision {
	case DTYearPrecision:
		unit = unitYear
	case DTMonthPrecision:
		unit = unitMonth
	case DTDayPrecision:
		unit = unitDay
	case DTHourPrecision:
		unit = unitHour
	case DTMinutePrecision:
		unit = unitMinute
	}

	month, day := dt.month, dt.day
	if month == 0 {
		month = 1
	}
	if day == 0 {
		day = 1
	}

	v := temporalValue{unit: unit, hasOffset: dt.hasTZ, offset: dt.tzOffset}
	v.parts[unitYear] = dt.year
	v.parts[unitMonth] = month
	v.parts[unitDay] = day
	v.parts[unitHour] = dt.hour
	v.parts[unitMinute] = dt.minute
	// Seconds and milliseconds form one precision, held as thousandths
	v.parts[unitSecond] = dt.second*1000 + dt.millis
	return v
}

// temporalValue converts a Time for comparison. A time carries no date, so all
// of them share one and only the clock takes part.
func (t Time) temporalValue() temporalValue {
	unit := unitSecond
	switch t.precision {
	case HourPrecision:
		unit = unitHour
	case MinutePrecision:
		unit = unitMinute
	}

	v := temporalValue{unit: unit}
	v.parts[unitYear], v.parts[unitMonth], v.parts[unitDay] = 2000, 1, 1
	v.parts[unitHour] = t.hour
	v.parts[unitMinute] = t.minute
	v.parts[unitSecond] = t.second*1000 + t.millis
	return v
}

// compareTemporalValues orders any two temporal values, which also covers a Date
// against a DateTime: the specification makes Date implicitly convertible to
// DateTime, and the shared-precision rule then either decides or reports that it
// cannot.
func compareTemporalValues(left, right Value) (int, error) {
	leftValue, ok := asTemporalValue(left)
	if !ok {
		return 0, errNotTemporal
	}
	rightValue, ok := asTemporalValue(right)
	if !ok {
		return 0, errNotTemporal
	}

	// A Time has no date, so it cannot be ordered against a Date or DateTime
	if isTimeValue(left) != isTimeValue(right) {
		return 0, errNotTemporal
	}

	return compareTemporal(leftValue, rightValue)
}

func asTemporalValue(v Value) (temporalValue, bool) {
	switch t := v.(type) {
	case Date:
		return t.temporalValue(), true
	case DateTime:
		return t.temporalValue(), true
	case Time:
		return t.temporalValue(), true
	}
	return temporalValue{}, false
}

func isTimeValue(v Value) bool {
	_, ok := v.(Time)
	return ok
}

// EqualTemporal reports whether two temporal values are equal, under the same
// precision rule as ordering: the comparison proceeds precision by precision and
// stops at the first difference, so equality is unknown — and the operator
// yields empty — only when the values match on everything they share while one
// is specified more precisely.
//
//	@2012-01 = @2012           // empty, different precision
//	@2012-01 = @2012-02        // false, decided at the month
//	@T10:30:00 = @T10:30:00.0  // true, seconds and milliseconds are one precision
//
// Returns ErrPrecisionMismatch for the unknown case, and an error when the
// values are not comparable temporals at all.
func EqualTemporal(left, right Value) (bool, error) {
	cmp, err := compareTemporalValues(left, right)
	if err != nil {
		return false, err
	}
	return cmp == 0, nil
}

// IsTemporal reports whether a value is a Date, DateTime or Time.
func IsTemporal(v Value) bool {
	switch v.(type) {
	case Date, DateTime, Time:
		return true
	}
	return false
}
