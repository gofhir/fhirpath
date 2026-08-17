package funcs

import (
	"fmt"
	"strings"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// The component extractors FHIRPath 3.0.0 defines: yearOf, monthOf, dayOf,
// hourOf, minuteOf, secondOf, millisecondOf, timezoneOffsetOf, dateOf and
// timeOf.
//
// The names carry the "Of" for a reason worth recording: year, month, day,
// hour, minute and second are calendar units in the grammar, so a call written
// as Patient.birthDate.month() does not parse — the identifier after the dot is
// read as the unit. This engine registered those names anyway, which left the
// functions reachable only as Patient.birthDate.`month`(), and nobody writes
// that. The 3.0.0 names are the ones that can actually be called.
//
// The older names are kept and now share these implementations, so the two
// spellings cannot drift apart.
func init() {
	Register(FuncDef{Name: "yearOf", MinArgs: 0, MaxArgs: 0, Fn: fnYearOf})
	Register(FuncDef{Name: "monthOf", MinArgs: 0, MaxArgs: 0, Fn: fnMonthOf})
	Register(FuncDef{Name: "dayOf", MinArgs: 0, MaxArgs: 0, Fn: fnDayOf})
	Register(FuncDef{Name: "hourOf", MinArgs: 0, MaxArgs: 0, Fn: fnHourOf})
	Register(FuncDef{Name: "minuteOf", MinArgs: 0, MaxArgs: 0, Fn: fnMinuteOf})
	Register(FuncDef{Name: "secondOf", MinArgs: 0, MaxArgs: 0, Fn: fnSecondOf})
	Register(FuncDef{Name: "millisecondOf", MinArgs: 0, MaxArgs: 0, Fn: fnMillisecondOf})
	Register(FuncDef{Name: "timezoneOffsetOf", MinArgs: 0, MaxArgs: 0, Fn: fnTimezoneOffsetOf})
	Register(FuncDef{Name: "dateOf", MinArgs: 0, MaxArgs: 0, Fn: fnDateOf})
	Register(FuncDef{Name: "timeOf", MinArgs: 0, MaxArgs: 0, Fn: fnTimeOf})
}

// componentOf is the shape all ten share: an empty input gives empty, more than
// one item is an error, and a value that does not carry the component asked for
// — because of its type or because its precision stops short of it — gives
// empty.
func componentOf(input types.Collection, extract func(types.Value) (types.Value, bool)) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}
	if len(input) != 1 {
		return nil, eval.SingletonError(len(input))
	}

	value, ok := extract(input[0])
	if !ok {
		return types.Collection{}, nil
	}
	return types.Collection{value}, nil
}

func integerComponent(n int) (types.Value, bool) {
	return types.NewInteger(int64(n)), true
}

func fnYearOf(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return componentOf(input, func(v types.Value) (types.Value, bool) {
		switch t := v.(type) {
		case types.Date:
			return integerComponent(t.Year())
		case types.DateTime:
			return integerComponent(t.Year())
		}
		return nil, false
	})
}

func fnMonthOf(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return componentOf(input, func(v types.Value) (types.Value, bool) {
		switch t := v.(type) {
		case types.Date:
			if t.Precision() < types.MonthPrecision {
				return nil, false
			}
			return integerComponent(t.Month())
		case types.DateTime:
			// A DateTime may carry a time without a day — @2012T12:30:00 is
			// written that way — so its precision does not say whether the
			// month is there. The value does: a month is 1 through 12, and
			// zero is how an absent one is held.
			if t.Month() == 0 {
				return nil, false
			}
			return integerComponent(t.Month())
		}
		return nil, false
	})
}

func fnDayOf(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return componentOf(input, func(v types.Value) (types.Value, bool) {
		switch t := v.(type) {
		case types.Date:
			if t.Precision() < types.DayPrecision {
				return nil, false
			}
			return integerComponent(t.Day())
		case types.DateTime:
			if t.Day() == 0 {
				return nil, false
			}
			return integerComponent(t.Day())
		}
		return nil, false
	})
}

func fnHourOf(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return componentOf(input, func(v types.Value) (types.Value, bool) {
		switch t := v.(type) {
		case types.DateTime:
			if t.Precision() < types.DTHourPrecision {
				return nil, false
			}
			return integerComponent(t.Hour())
		case types.Time:
			return integerComponent(t.Hour())
		}
		return nil, false
	})
}

func fnMinuteOf(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return componentOf(input, func(v types.Value) (types.Value, bool) {
		switch t := v.(type) {
		case types.DateTime:
			if t.Precision() < types.DTMinutePrecision {
				return nil, false
			}
			return integerComponent(t.Minute())
		case types.Time:
			if t.Precision() < types.MinutePrecision {
				return nil, false
			}
			return integerComponent(t.Minute())
		}
		return nil, false
	})
}

func fnSecondOf(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return componentOf(input, func(v types.Value) (types.Value, bool) {
		switch t := v.(type) {
		case types.DateTime:
			if t.Precision() < types.DTSecondPrecision {
				return nil, false
			}
			return integerComponent(t.Second())
		case types.Time:
			if t.Precision() < types.SecondPrecision {
				return nil, false
			}
			return integerComponent(t.Second())
		}
		return nil, false
	})
}

func fnMillisecondOf(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return componentOf(input, func(v types.Value) (types.Value, bool) {
		switch t := v.(type) {
		case types.DateTime:
			if t.Precision() < types.DTMillisPrecision {
				return nil, false
			}
			return integerComponent(t.Millisecond())
		case types.Time:
			if t.Precision() < types.MillisPrecision {
				return nil, false
			}
			return integerComponent(t.Millisecond())
		}
		return nil, false
	})
}

// fnTimezoneOffsetOf returns the offset as hours from UTC, a quarter of an hour
// reading 0.25: "-7.5 for UTC-7:30".
func fnTimezoneOffsetOf(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return componentOf(input, func(v types.Value) (types.Value, bool) {
		dt, ok := v.(types.DateTime)
		if !ok || !dt.HasTZ() {
			return nil, false
		}

		offset, err := types.NewDecimal(offsetInHours(dt.TZOffset()))
		if err != nil {
			return nil, false
		}
		return offset, true
	})
}

// offsetInHours renders an offset given in minutes as hours. A whole number of
// hours keeps a decimal place, as the specification's own -7.0 does.
func offsetInHours(minutes int) string {
	sign := ""
	if minutes < 0 {
		sign = "-"
		minutes = -minutes
	}

	hours := minutes / 60
	remainder := minutes % 60
	if remainder == 0 {
		return fmt.Sprintf("%s%d.0", sign, hours)
	}

	// A minute is a sixtieth of an hour, so the fraction is exact to at most
	// four decimal places once trailing zeros are trimmed.
	fraction := strings.TrimRight(fmt.Sprintf("%04d", remainder*10000/60), "0")
	return fmt.Sprintf("%s%d.%s", sign, hours, fraction)
}

// fnDateOf returns the date a value carries, at the precision it carries it.
func fnDateOf(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return componentOf(input, func(v types.Value) (types.Value, bool) {
		switch t := v.(type) {
		case types.Date:
			return t, true
		case types.DateTime:
			date, err := types.NewDate(datePartOf(t))
			if err != nil {
				return nil, false
			}
			return date, true
		}
		return nil, false
	})
}

// datePartOf writes the date a DateTime carries, stopping where its precision
// does: @2012 gives 2012, not 2012-01-01.
func datePartOf(dt types.DateTime) string {
	switch {
	case dt.Month() == 0:
		return fmt.Sprintf("%04d", dt.Year())
	case dt.Day() == 0:
		return fmt.Sprintf("%04d-%02d", dt.Year(), dt.Month())
	default:
		return fmt.Sprintf("%04d-%02d-%02d", dt.Year(), dt.Month(), dt.Day())
	}
}

// fnTimeOf returns the time a DateTime carries, without its offset: the
// specification's example gives @T12:30:00.000 for a value written -07:00.
func fnTimeOf(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return componentOf(input, func(v types.Value) (types.Value, bool) {
		dt, ok := v.(types.DateTime)
		if !ok || dt.Precision() < types.DTHourPrecision {
			return nil, false
		}

		clock, err := types.NewTime(timePartOf(dt))
		if err != nil {
			return nil, false
		}
		return clock, true
	})
}

// timePartOf writes the time a DateTime carries, stopping where its precision
// does.
func timePartOf(dt types.DateTime) string {
	switch dt.Precision() {
	case types.DTHourPrecision:
		return fmt.Sprintf("%02d", dt.Hour())
	case types.DTMinutePrecision:
		return fmt.Sprintf("%02d:%02d", dt.Hour(), dt.Minute())
	case types.DTSecondPrecision:
		return fmt.Sprintf("%02d:%02d:%02d", dt.Hour(), dt.Minute(), dt.Second())
	default:
		return fmt.Sprintf("%02d:%02d:%02d.%03d", dt.Hour(), dt.Minute(), dt.Second(), dt.Millisecond())
	}
}
