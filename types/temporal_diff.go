package types

import (
	"errors"
	"time"
)

// ErrUnsupportedPrecision reports that the precision named in a difference() or
// duration() call is not one the input's type admits.
//
// The specification fixes the permitted set per type: a date takes year, month,
// week or day; a datetime adds hour, minute, second and millisecond; a time
// takes only the four time-of-day precisions. Asking a date for the number of
// hours crossed is not an unknown result — it is a nonsensical request, so it is
// an error rather than empty.
var ErrUnsupportedPrecision = errors.New("precision is not valid for this temporal type")

// diffPrecision is a precision that difference() and duration() accept as their
// second argument.
//
// This is a finer scale than temporalUnit, which folds seconds and milliseconds
// into one precision because that is how comparison treats them. Here they are
// distinct: a value specified only to the second cannot answer how many
// millisecond boundaries were crossed.
type diffPrecision int

const (
	precYear diffPrecision = iota
	precMonth
	precWeek
	precDay
	precHour
	precMinute
	precSecond
	precMillisecond
)

var diffPrecisionNames = map[string]diffPrecision{
	"year":        precYear,
	"month":       precMonth,
	"week":        precWeek,
	"day":         precDay,
	"hour":        precHour,
	"minute":      precMinute,
	"second":      precSecond,
	"millisecond": precMillisecond,
}

// isTimeOfDay reports whether the precision names a component of the clock
// rather than the calendar. The distinction drives two rules: only these
// precisions call for normalizing timezone offsets, and only these are valid for
// a Time.
func (p diffPrecision) isTimeOfDay() bool {
	return p >= precHour
}

// available returns the finest precision to which a value is actually specified,
// which is what the "less precision than requested" rule is measured against.
func availablePrecision(v Value) (diffPrecision, bool) {
	switch t := v.(type) {
	case Date:
		switch t.precision {
		case YearPrecision:
			return precYear, true
		case MonthPrecision:
			return precMonth, true
		default:
			return precDay, true
		}
	case DateTime:
		switch t.precision {
		case DTYearPrecision:
			return precYear, true
		case DTMonthPrecision:
			return precMonth, true
		case DTDayPrecision:
			return precDay, true
		case DTHourPrecision:
			return precHour, true
		case DTMinutePrecision:
			return precMinute, true
		case DTSecondPrecision:
			return precSecond, true
		default:
			return precMillisecond, true
		}
	case Time:
		switch t.precision {
		case HourPrecision:
			return precHour, true
		case MinutePrecision:
			return precMinute, true
		case SecondPrecision:
			return precSecond, true
		default:
			return precMillisecond, true
		}
	}
	return 0, false
}

// permits reports whether a value's type admits the requested precision.
func permitsPrecision(v Value, p diffPrecision) bool {
	switch v.(type) {
	case Date:
		// A date has no clock, so only calendar precisions apply
		return !p.isTimeOfDay()
	case Time:
		// A time has no calendar, so only clock precisions apply
		return p.isTimeOfDay()
	case DateTime:
		return true
	}
	return false
}

// TemporalDifference returns the number of boundaries of the given precision
// crossed between two temporal values, negative when the input is the later of
// the two.
//
// A boundary is a point at which the named component changes: the difference in
// weeks between two dates is the number of Sundays that fall after the first and
// on or before the second, which is why @2025-01-02.difference(@2025-01-07,
// 'week') is 1 even though only five days separate them.
//
// Reports ErrPrecisionMismatch when either value is specified less precisely
// than the request, which callers translate to empty, and
// ErrUnsupportedPrecision when the precision does not apply to the type.
func TemporalDifference(from, to Value, precision string) (int64, error) {
	left, right, prec, err := diffOperands(from, to, precision)
	if err != nil {
		return 0, err
	}

	switch prec {
	case precYear:
		return int64(right.parts[unitYear] - left.parts[unitYear]), nil

	case precMonth:
		return int64(right.monthsSinceYearZero() - left.monthsSinceYearZero()), nil

	case precWeek:
		return int64(right.weekIndex() - left.weekIndex()), nil

	case precDay:
		return int64(right.dayIndex() - left.dayIndex()), nil
	}

	// The remaining precisions count boundaries of the clock, which is the
	// distance between the two instants truncated to that precision.
	unit := prec.duration()
	return int64(right.instant().Truncate(unit).Sub(left.instant().Truncate(unit)) / unit), nil
}

// TemporalDuration returns the number of whole periods of the given precision
// between two temporal values, negative when the input is the later of the two.
//
// This is the "how long since" reading, distinct from the boundary count:
// @2025-01-01.duration(@2025-09-01, 'year') is 0 because the year has not
// elapsed, where difference() would report 0 as well but
// @2024-12-31.difference(@2025-01-01, 'year') is 1 and its duration is 0.
//
// Reports the same errors as [TemporalDifference].
func TemporalDuration(from, to Value, precision string) (int64, error) {
	left, right, prec, err := diffOperands(from, to, precision)
	if err != nil {
		return 0, err
	}

	switch prec {
	case precYear, precMonth:
		months := int64(right.monthsSinceYearZero() - left.monthsSinceYearZero())

		// A whole month has passed only once the later value reaches the same
		// point within the month, so a partial trailing month is dropped —
		// towards zero, which for a negative span means dropping the magnitude.
		if remainder := right.withinMonth().compare(left.withinMonth()); months > 0 && remainder < 0 {
			months--
		} else if months < 0 && remainder > 0 {
			months++
		}

		if prec == precYear {
			return months / 12, nil
		}
		return months, nil
	}

	// Weeks and every finer precision are fixed-length periods, so the elapsed
	// time divided by the period gives the count, truncated towards zero.
	unit := prec.duration()
	return int64(right.instant().Sub(left.instant()) / unit), nil
}

// diffOperands validates a difference()/duration() request and returns both
// operands ready to measure.
func diffOperands(from, to Value, precision string) (left, right temporalValue, prec diffPrecision, err error) {
	prec, ok := diffPrecisionNames[precision]
	if !ok {
		return left, right, prec, ErrUnsupportedPrecision
	}

	// A time cannot be measured against a date: one has no calendar and the
	// other no clock, so no precision is shared between them
	if isTimeValue(from) != isTimeValue(to) {
		return left, right, prec, errNotTemporal
	}

	for _, operand := range []Value{from, to} {
		if !permitsPrecision(operand, prec) {
			return left, right, prec, ErrUnsupportedPrecision
		}

		available, ok := availablePrecision(operand)
		if !ok {
			return left, right, prec, errNotTemporal
		}

		// Measuring in weeks means knowing which day of the week each value
		// falls on, so it needs a day just as a day-precision request does
		needed := prec
		if needed == precWeek {
			needed = precDay
		}
		if available < needed {
			return left, right, prec, ErrPrecisionMismatch
		}
	}

	left, _ = asTemporalValue(from)
	right, _ = asTemporalValue(to)

	// The specification asks for the timezone to be normalized when the request
	// is for a precision of hour or finer; calendar precisions read the
	// components as written.
	if prec.isTimeOfDay() && left.hasOffset && right.hasOffset && left.offset != right.offset {
		left = left.inUTC()
		right = right.inUTC()
	}

	return left, right, prec, nil
}

// duration returns the length of one period at this precision. Only defined for
// week and the clock precisions, the ones that have a fixed length; years and
// months do not, which is why they are counted rather than divided.
func (p diffPrecision) duration() time.Duration {
	switch p {
	case precWeek:
		return 7 * 24 * time.Hour
	case precDay:
		return 24 * time.Hour
	case precHour:
		return time.Hour
	case precMinute:
		return time.Minute
	case precSecond:
		return time.Second
	default:
		return time.Millisecond
	}
}

// instant renders the value as a point in time, at its own offset — UTC when it
// carries none, which is consistent because two values are only measured
// against each other after any offsets have been normalized.
func (v temporalValue) instant() time.Time {
	return time.Date(
		v.parts[unitYear], time.Month(v.parts[unitMonth]), v.parts[unitDay],
		v.parts[unitHour], v.parts[unitMinute], v.parts[unitSecond]/1000,
		(v.parts[unitSecond]%1000)*1e6,
		time.FixedZone("", v.offset*60),
	)
}

// monthsSinceYearZero numbers the months consecutively, so that subtracting two
// of them counts the month boundaries between them across any year gap.
func (v temporalValue) monthsSinceYearZero() int {
	return v.parts[unitYear]*12 + v.parts[unitMonth] - 1
}

// dayIndex numbers the days consecutively from the epoch, counting midnight
// boundaries. Taken from the calendar components rather than the instant so that
// the time of day plays no part.
func (v temporalValue) dayIndex() int {
	midnight := time.Date(
		v.parts[unitYear], time.Month(v.parts[unitMonth]), v.parts[unitDay],
		0, 0, 0, 0, time.UTC,
	)
	return int(midnight.Unix() / (24 * 60 * 60))
}

// weekIndex numbers the weeks consecutively, counting the boundary at the start
// of each one.
//
// The specification fixes Sunday as the first day of the week for this purpose.
// The epoch, 1970-01-01, was a Thursday, so the offset of 4 aligns the division
// to land its boundary on Sunday rather than on the epoch's own weekday.
func (v temporalValue) weekIndex() int {
	const thursdayEpochToSunday = 4

	days := v.dayIndex() + thursdayEpochToSunday
	if days < 0 {
		// Integer division truncates towards zero, which would fold the week
		// either side of the epoch into one; floor the negative case instead.
		return -((-days + 6) / 7)
	}
	return days / 7
}

// withinMonth is the position of a value inside its month — the day and the time
// of day — which is what decides whether a whole month has elapsed.
type monthPosition struct {
	day, hour, minute, millis int
}

func (v temporalValue) withinMonth() monthPosition {
	return monthPosition{
		day:    v.parts[unitDay],
		hour:   v.parts[unitHour],
		minute: v.parts[unitMinute],
		millis: v.parts[unitSecond],
	}
}

func (p monthPosition) compare(other monthPosition) int {
	for _, pair := range [][2]int{
		{p.day, other.day},
		{p.hour, other.hour},
		{p.minute, other.minute},
		{p.millis, other.millis},
	} {
		switch {
		case pair[0] < pair[1]:
			return -1
		case pair[0] > pair[1]:
			return 1
		}
	}
	return 0
}
