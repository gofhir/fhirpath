package types

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// DateTime represents a FHIRPath datetime value.
type DateTime struct {
	year      int
	month     int
	day       int
	hour      int
	minute    int
	second    int
	millis    int
	tzOffset  int  // timezone offset in minutes
	hasTZ     bool // whether timezone is specified
	precision DateTimePrecision
	fhirType  string // FHIR type when the value was read through a model

	// The FHIR element this value was read with, when it carried one
	primitiveElement
}

// DateTimePrecision indicates the precision of a datetime.
type DateTimePrecision int

const (
	DTYearPrecision DateTimePrecision = iota
	DTMonthPrecision
	DTDayPrecision
	DTHourPrecision
	DTMinutePrecision
	DTSecondPrecision
	DTMillisPrecision
)

// DateTime regex pattern
// The grammar's DATETIME is a date followed by 'T' and an optional time:
//
//	DATETIME : '@' DATEFORMAT 'T' (TIMEFORMAT TIMEZONEOFFSETFORMAT?)?
//
// so the marker may trail a date of any precision with no time after it —
// @2015T, @2015-02T and @2015-02-04T are all DateTime values, distinct from the
// Date literals @2015, @2015-02 and @2015-02-04.
var dateTimePattern = regexp.MustCompile(
	`^(\d{4})(?:-(\d{2})(?:-(\d{2}))?)?(?:T(?:(\d{2})(?::(\d{2})(?::(\d{2})(?:\.(\d+))?)?)?)?)?(Z|[+-]\d{2}:\d{2})?$`,
)

// NewDateTime creates a DateTime from a string.
func NewDateTime(s string) (DateTime, error) {
	matches := dateTimePattern.FindStringSubmatch(s)
	if matches == nil {
		return DateTime{}, fmt.Errorf("invalid datetime format: %s", s)
	}

	dt := DateTime{}
	precision := DTYearPrecision

	// Year (required)
	year, err := strconv.Atoi(matches[1])
	if err != nil {
		return DateTime{}, fmt.Errorf("invalid year in datetime: %s", s)
	}
	dt.year = year

	// Month
	if matches[2] != "" {
		month, err := strconv.Atoi(matches[2])
		if err != nil {
			return DateTime{}, fmt.Errorf("invalid month in datetime: %s", s)
		}
		dt.month = month
		precision = DTMonthPrecision
	}

	// Day
	if matches[3] != "" {
		day, err := strconv.Atoi(matches[3])
		if err != nil {
			return DateTime{}, fmt.Errorf("invalid day in datetime: %s", s)
		}
		dt.day = day
		precision = DTDayPrecision
	}

	// Hour
	if matches[4] != "" {
		hour, err := strconv.Atoi(matches[4])
		if err != nil {
			return DateTime{}, fmt.Errorf("invalid hour in datetime: %s", s)
		}
		dt.hour = hour
		precision = DTHourPrecision
	}

	// Minute
	if matches[5] != "" {
		minute, err := strconv.Atoi(matches[5])
		if err != nil {
			return DateTime{}, fmt.Errorf("invalid minute in datetime: %s", s)
		}
		dt.minute = minute
		precision = DTMinutePrecision
	}

	// Second
	if matches[6] != "" {
		second, err := strconv.Atoi(matches[6])
		if err != nil {
			return DateTime{}, fmt.Errorf("invalid second in datetime: %s", s)
		}
		dt.second = second
		precision = DTSecondPrecision
	}

	// Milliseconds
	if matches[7] != "" {
		// Pad or truncate to 3 digits
		ms := matches[7]
		for len(ms) < 3 {
			ms += "0"
		}
		if len(ms) > 3 {
			ms = ms[:3]
		}
		millis, err := strconv.Atoi(ms)
		if err != nil {
			return DateTime{}, fmt.Errorf("invalid milliseconds in datetime: %s", s)
		}
		dt.millis = millis
		precision = DTMillisPrecision
	}

	// Timezone
	if matches[8] != "" {
		dt.hasTZ = true
		if matches[8] == "Z" {
			dt.tzOffset = 0
		} else {
			// Parse timezone offset
			sign := 1
			if matches[8][0] == '-' {
				sign = -1
			}
			hours, err := strconv.Atoi(matches[8][1:3])
			if err != nil {
				return DateTime{}, fmt.Errorf("invalid timezone hours in datetime: %s", s)
			}
			mins, err := strconv.Atoi(matches[8][4:6])
			if err != nil {
				return DateTime{}, fmt.Errorf("invalid timezone minutes in datetime: %s", s)
			}
			dt.tzOffset = sign * (hours*60 + mins)
		}
	}

	dt.precision = precision
	return dt, nil
}

// NewDateTimeFromTime creates a DateTime from time.Time.
func NewDateTimeFromTime(t time.Time) DateTime {
	_, offset := t.Zone()
	return DateTime{
		year:      t.Year(),
		month:     int(t.Month()),
		day:       t.Day(),
		hour:      t.Hour(),
		minute:    t.Minute(),
		second:    t.Second(),
		millis:    t.Nanosecond() / 1000000,
		tzOffset:  offset / 60,
		hasTZ:     true,
		precision: DTMillisPrecision,
	}
}

// Type returns the type name.
func (dt DateTime) Type() string {
	if dt.fhirType != "" {
		return dt.fhirType
	}
	return TypeNameDateTime
}

// Equal checks equality with another value.
func (dt DateTime) Equal(other Value) bool {
	if o, ok := other.(DateTime); ok {
		return dt.ToTime().Equal(o.ToTime())
	}
	return false
}

// Equivalent checks equivalence with another value.
func (dt DateTime) Equivalent(other Value) bool {
	return dt.Equal(other)
}

// String returns the string representation.
func (dt DateTime) String() string {
	result := fmt.Sprintf("%04d", dt.year)

	if dt.precision >= DTMonthPrecision {
		result += fmt.Sprintf("-%02d", dt.month)
	}
	if dt.precision >= DTDayPrecision {
		result += fmt.Sprintf("-%02d", dt.day)
	}
	if dt.precision >= DTHourPrecision {
		result += fmt.Sprintf("T%02d", dt.hour)
	}
	if dt.precision >= DTMinutePrecision {
		result += fmt.Sprintf(":%02d", dt.minute)
	}
	if dt.precision >= DTSecondPrecision {
		result += fmt.Sprintf(":%02d", dt.second)
	}
	if dt.precision >= DTMillisPrecision {
		result += fmt.Sprintf(".%03d", dt.millis)
	}

	if dt.hasTZ {
		if dt.tzOffset == 0 {
			result += "Z"
		} else {
			sign := "+"
			offset := dt.tzOffset
			if offset < 0 {
				sign = "-"
				offset = -offset
			}
			result += fmt.Sprintf("%s%02d:%02d", sign, offset/60, offset%60)
		}
	}

	return result
}

// IsEmpty returns false for DateTime.
func (dt DateTime) IsEmpty() bool {
	return false
}

// ToTime converts to time.Time.
func (dt DateTime) ToTime() time.Time {
	month := dt.month
	if month == 0 {
		month = 1
	}
	day := dt.day
	if day == 0 {
		day = 1
	}

	var loc *time.Location
	if dt.hasTZ {
		loc = time.FixedZone("", dt.tzOffset*60)
	} else {
		loc = time.UTC
	}

	return time.Date(dt.year, time.Month(month), day, dt.hour, dt.minute, dt.second, dt.millis*1000000, loc)
}

// Precision returns the datetime precision.
func (dt DateTime) Precision() DateTimePrecision { return dt.precision }

// HasTZ returns whether the datetime has an explicit timezone.
func (dt DateTime) HasTZ() bool { return dt.hasTZ }

// TZOffset returns the timezone offset in minutes.
func (dt DateTime) TZOffset() int { return dt.tzOffset }

// Accessors
func (dt DateTime) Year() int        { return dt.year }
func (dt DateTime) Month() int       { return dt.month }
func (dt DateTime) Day() int         { return dt.day }
func (dt DateTime) Hour() int        { return dt.hour }
func (dt DateTime) Minute() int      { return dt.minute }
func (dt DateTime) Second() int      { return dt.second }
func (dt DateTime) Millisecond() int { return dt.millis }

// AddDuration adds a duration (as Quantity with temporal unit) to the datetime.
// Supported units: year(s), month(s), week(s), day(s), hour(s), minute(s), second(s), millisecond(s)
func (dt DateTime) AddDuration(value int, unit string) (DateTime, error) {
	years, months, days, millis, err := durationParts(value, unit)
	if err != nil {
		return DateTime{}, err
	}

	// Years and months move the calendar components and clamp the day, which
	// AddDate would instead roll into the next month
	baseYear, baseMonth, baseDay := shiftCalendarMonths(dt.year, dt.monthOrFirst(), dt.dayOrFirst(), years, months)

	base := dt.ToTime()
	shifted := time.Date(baseYear, time.Month(baseMonth), baseDay,
		base.Hour(), base.Minute(), base.Second(), base.Nanosecond(), base.Location()).
		AddDate(0, 0, days)
	if millis != 0 {
		shifted = shifted.Add(time.Duration(millis) * time.Millisecond)
	}

	result := DateTime{
		year:      shifted.Year(),
		month:     int(shifted.Month()),
		day:       shifted.Day(),
		hour:      shifted.Hour(),
		minute:    shifted.Minute(),
		second:    shifted.Second(),
		millis:    shifted.Nanosecond() / 1000000,
		tzOffset:  dt.tzOffset,
		hasTZ:     dt.hasTZ,
		precision: dt.precision,
	}

	// A shift never makes the value more precise than it was
	if dt.precision < DTMonthPrecision {
		result.month = 0
	}
	if dt.precision < DTDayPrecision {
		result.day = 0
	}
	if dt.precision < DTHourPrecision {
		result.hour = 0
	}
	if dt.precision < DTMinutePrecision {
		result.minute = 0
	}
	if dt.precision < DTSecondPrecision {
		result.second = 0
	}
	if dt.precision < DTMillisPrecision {
		result.millis = 0
	}

	return result, nil
}

// SubtractDuration subtracts a duration from the datetime.
func (dt DateTime) SubtractDuration(value int, unit string) (DateTime, error) {
	return dt.AddDuration(-value, unit)
}

// Compare compares two datetimes. Returns -1, 0, or 1.
// Implements the Comparable interface.
// Returns error if precisions differ and comparison is ambiguous.
func (dt DateTime) Compare(other Value) (int, error) {
	if _, ok := other.(DateTime); !ok {
		if _, isDate := other.(Date); !isDate {
			return 0, fmt.Errorf("cannot compare DateTime with %s", other.Type())
		}
	}
	return compareTemporalValues(dt, other)
}

// WithDefaultOffset returns a copy carrying the given offset, in minutes east of
// UTC, when the value was written without one. A value that already states an
// offset is returned unchanged.
//
// This is for callers whose language settles what an unwritten offset means.
// FHIRPath does not: it calls the default "a policy decision", so this engine
// supplies none and a comparison it cannot place has no answer. CQL does settle
// it, and settles it at construction — "If no timezone offset is supplied, the
// timezone offset of the evaluation request timestamp is assumed" — so a CQL
// value written without an offset does not have an unknown one, it has the
// request's. Applying it here is how a caller says so.
//
// It is applied to the value rather than to a comparison because that is where
// the language it comes from applies it: extracting the offset from such a
// value yields the offset, not empty, which a comparison-only variant could not
// express.
//
// The offset is not marked as defaulted. Once applied, the value is one that
// carries an offset, which is what the caller asserted.
//
// The value is returned unchanged, rather than carrying something invented,
// when there is nothing sensible to apply: it already states an offset, its
// precision stops above a time of day, or the offset is not one a timezone can
// have. Declining to answer a comparison is the safe outcome; inventing an
// instant is not.
func (dt DateTime) WithDefaultOffset(minutes int) DateTime {
	if dt.hasTZ {
		return dt
	}

	// An offset only means something once the value reaches a time of day.
	// @2020 has no instant to place, and saying it sits at UTC-5 would make it
	// comparable to things it is not.
	if dt.precision < DTHourPrecision {
		return dt
	}

	// Minutes, not hours — the unit timezoneOffsetOf() reports is hours, so a
	// caller reading that and passing -5 here means UTC-5 and would otherwise
	// get UTC-00:05 without being told. Anything beyond the range a timezone
	// can take is a mistake of the same kind.
	if minutes < minTimezoneOffset || minutes > maxTimezoneOffset {
		return dt
	}

	dt.hasTZ = true
	dt.tzOffset = minutes
	return dt
}

// The offsets a timezone can take, in minutes east of UTC. FHIR and XML Schema
// both bound them at fourteen hours.
const (
	minTimezoneOffset = -14 * 60
	maxTimezoneOffset = 14 * 60
)

// WithFHIRType returns a copy that reports the FHIR type it was declared with.
// FHIR primitives are types in their own right — a FHIR.boolean is not a
// System.Boolean — so a value keeps the name the model gave it.
func (dt DateTime) WithFHIRType(fhirType string) DateTime {
	dt.fhirType = fhirType
	return dt
}

// WithElement returns a copy carrying the FHIR element that accompanied the
// value in the JSON, which is where its extensions and id live.
func (dt DateTime) WithElement(element *ObjectValue) DateTime {
	dt.element = element
	return dt
}

// monthOrFirst returns the month, or January when the value is not specified to
// a month; the result is trimmed back to the value's own precision afterwards.
func (dt DateTime) monthOrFirst() int {
	if dt.month == 0 {
		return 1
	}
	return dt.month
}

// dayOrFirst returns the day, or the first of the month when unspecified.
func (dt DateTime) dayOrFirst() int {
	if dt.day == 0 {
		return 1
	}
	return dt.day
}
