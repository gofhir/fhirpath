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
}

// fnLowBoundary returns the lowest possible value for the input based on its precision.
func fnLowBoundary(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	precision := extractPrecisionArg(args)

	switch v := input[0].(type) {
	case types.Date:
		return lowBoundaryDate(v)
	case types.DateTime:
		return lowBoundaryDateTime(v)
	case types.Time:
		return lowBoundaryTime(v)
	case types.Decimal:
		if precision < 0 {
			return types.Collection{}, nil
		}
		return lowBoundaryDecimal(v, precision)
	case types.Integer:
		return types.Collection{v}, nil
	case types.Quantity:
		if precision < 0 {
			return types.Collection{}, nil
		}
		return lowBoundaryQuantity(v, precision)
	default:
		return types.Collection{}, nil
	}
}

// fnHighBoundary returns the highest possible value for the input based on its precision.
func fnHighBoundary(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	precision := extractPrecisionArg(args)

	switch v := input[0].(type) {
	case types.Date:
		return highBoundaryDate(v)
	case types.DateTime:
		return highBoundaryDateTime(v)
	case types.Time:
		return highBoundaryTime(v)
	case types.Decimal:
		if precision < 0 {
			return types.Collection{}, nil
		}
		return highBoundaryDecimal(v, precision)
	case types.Integer:
		return types.Collection{v}, nil
	case types.Quantity:
		if precision < 0 {
			return types.Collection{}, nil
		}
		return highBoundaryQuantity(v, precision)
	default:
		return types.Collection{}, nil
	}
}

// extractPrecisionArg extracts the optional integer precision argument.
// Returns -1 if no precision argument is provided.
func extractPrecisionArg(args []interface{}) int {
	if len(args) == 0 {
		return -1
	}
	if col, ok := args[0].(types.Collection); ok && !col.Empty() {
		if intVal, ok := col[0].(types.Integer); ok {
			return int(intVal.Value())
		}
	}
	return -1
}

// --- Date boundaries ---

func lowBoundaryDate(d types.Date) (types.Collection, error) {
	year := d.Year()
	month := d.Month()
	day := d.Day()

	switch d.Precision() {
	case types.YearPrecision:
		month = 1
		day = 1
	case types.MonthPrecision:
		day = 1
	}

	result, err := types.NewDate(fmt.Sprintf("%04d-%02d-%02d", year, month, day))
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{result}, nil
}

func highBoundaryDate(d types.Date) (types.Collection, error) {
	year := d.Year()
	month := d.Month()
	day := d.Day()

	switch d.Precision() {
	case types.YearPrecision:
		month = 12
		day = 31
	case types.MonthPrecision:
		day = daysInMonth(year, month)
	}

	result, err := types.NewDate(fmt.Sprintf("%04d-%02d-%02d", year, month, day))
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{result}, nil
}

// --- DateTime boundaries ---

func lowBoundaryDateTime(dt types.DateTime) (types.Collection, error) {
	year := dt.Year()
	month := dt.Month()
	day := dt.Day()
	hour := dt.Hour()
	minute := dt.Minute()
	second := dt.Second()
	millis := dt.Millisecond()

	p := dt.Precision()

	if p < types.DTMonthPrecision {
		month = 1
	}
	if p < types.DTDayPrecision {
		day = 1
	}
	// Hour, minute, second, millis default to 0 which is the low boundary

	if p >= types.DTMillisPrecision {
		// Already at maximum precision, return as-is
		return types.Collection{dt}, nil
	}

	// Build the lowest boundary datetime string
	s := fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d.%03d",
		year, month, day, hour, minute, second, millis)

	// Per FHIRPath spec, lowBoundary for DateTime uses +14:00 (earliest timezone)
	if dt.HasTZ() {
		s += formatTZOffset(dt.TZOffset())
	}

	result, err := types.NewDateTime(s)
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{result}, nil
}

func highBoundaryDateTime(dt types.DateTime) (types.Collection, error) {
	year := dt.Year()
	month := dt.Month()
	day := dt.Day()
	hour := dt.Hour()
	minute := dt.Minute()
	second := dt.Second()
	millis := dt.Millisecond()

	p := dt.Precision()

	if p >= types.DTMillisPrecision {
		return types.Collection{dt}, nil
	}

	if p < types.DTMonthPrecision {
		month = 12
	}
	if p < types.DTDayPrecision {
		// Use last day of the resolved month
		m := month
		if m == 0 {
			m = 12
		}
		day = daysInMonth(year, m)
	}
	if p < types.DTHourPrecision {
		hour = 23
	}
	if p < types.DTMinutePrecision {
		minute = 59
	}
	if p < types.DTSecondPrecision {
		second = 59
	}
	if p < types.DTMillisPrecision {
		millis = 999
	}

	s := fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d.%03d",
		year, month, day, hour, minute, second, millis)

	if dt.HasTZ() {
		s += formatTZOffset(dt.TZOffset())
	}

	result, err := types.NewDateTime(s)
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{result}, nil
}

// --- Time boundaries ---

func lowBoundaryTime(t types.Time) (types.Collection, error) {
	hour := t.Hour()
	minute := t.Minute()
	second := t.Second()
	millis := t.Millisecond()

	if t.Precision() >= types.MillisPrecision {
		return types.Collection{t}, nil
	}

	// minute, second, millis default to 0 which is the low boundary
	s := fmt.Sprintf("%02d:%02d:%02d.%03d", hour, minute, second, millis)
	result, err := types.NewTime(s)
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{result}, nil
}

func highBoundaryTime(t types.Time) (types.Collection, error) {
	hour := t.Hour()
	minute := t.Minute()
	second := t.Second()
	millis := t.Millisecond()

	if t.Precision() >= types.MillisPrecision {
		return types.Collection{t}, nil
	}

	if t.Precision() < types.MinutePrecision {
		minute = 59
	}
	if t.Precision() < types.SecondPrecision {
		second = 59
	}
	if t.Precision() < types.MillisPrecision {
		millis = 999
	}

	s := fmt.Sprintf("%02d:%02d:%02d.%03d", hour, minute, second, millis)
	result, err := types.NewTime(s)
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{result}, nil
}

// --- Decimal boundaries ---

func lowBoundaryDecimal(d types.Decimal, precision int) (types.Collection, error) {
	// Subtract half the precision unit from the value
	half := decimal.NewFromFloat(0.5)
	offset := half.Mul(decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(-precision))))
	result := d.Value().Sub(offset)
	dec, err := types.NewDecimal(result.String())
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{dec}, nil
}

func highBoundaryDecimal(d types.Decimal, precision int) (types.Collection, error) {
	// Add half the precision unit to the value
	half := decimal.NewFromFloat(0.5)
	offset := half.Mul(decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(-precision))))
	result := d.Value().Add(offset)
	dec, err := types.NewDecimal(result.String())
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{dec}, nil
}

// --- Quantity boundaries ---

func lowBoundaryQuantity(q types.Quantity, precision int) (types.Collection, error) {
	half := decimal.NewFromFloat(0.5)
	offset := half.Mul(decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(-precision))))
	newVal := q.Value().Sub(offset)
	return types.Collection{types.NewQuantityFromDecimal(newVal, q.Unit())}, nil
}

func highBoundaryQuantity(q types.Quantity, precision int) (types.Collection, error) {
	half := decimal.NewFromFloat(0.5)
	offset := half.Mul(decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(-precision))))
	newVal := q.Value().Add(offset)
	return types.Collection{types.NewQuantityFromDecimal(newVal, q.Unit())}, nil
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
