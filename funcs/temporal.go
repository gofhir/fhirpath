package funcs

import (
	"errors"
	"time"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// The registration blocks across this package are alike by construction: each is
// a table of one Register call per function, and dupl measures that shape rather
// than any repeated logic. Folding them into a loop over a slice would trade a
// readable table for an unreadable one.
//
//nolint:dupl // a declarative registration table, not duplicated logic
func init() {
	// Register temporal component functions
	Register(FuncDef{
		Name:    "difference",
		MinArgs: 2,
		MaxArgs: 2,
		Fn:      fnDifference,
	})

	Register(FuncDef{
		Name:    "duration",
		MinArgs: 2,
		MaxArgs: 2,
		Fn:      fnDuration,
	})

	Register(FuncDef{
		Name:    "year",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnYear,
	})

	Register(FuncDef{
		Name:    "month",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnMonth,
	})

	Register(FuncDef{
		Name:    "day",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnDay,
	})

	Register(FuncDef{
		Name:    "hour",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnHour,
	})

	Register(FuncDef{
		Name:    "minute",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnMinute,
	})

	Register(FuncDef{
		Name:    "second",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnSecond,
	})

	Register(FuncDef{
		Name:    "millisecond",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnMillisecond,
	})

	// Override the placeholder functions with real implementations
	Register(FuncDef{
		Name:    "now",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnNowReal,
	})

	Register(FuncDef{
		Name:    "today",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnTodayReal,
	})

	Register(FuncDef{
		Name:    "timeOfDay",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnTimeOfDayReal,
	})
}

// The calendar-unit spellings of the component extractors. They answer exactly
// what yearOf and its siblings answer — one implementation, two names — and
// they are the ones the grammar reads as units, so calling them takes
// backticks. See funcs/temporal_components.go.
func fnYear(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	return fnYearOf(ctx, input, args)
}

func fnMonth(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	return fnMonthOf(ctx, input, args)
}

func fnDay(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	return fnDayOf(ctx, input, args)
}

func fnHour(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	return fnHourOf(ctx, input, args)
}

func fnMinute(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	return fnMinuteOf(ctx, input, args)
}

func fnSecond(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	return fnSecondOf(ctx, input, args)
}

func fnMillisecond(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	return fnMillisecondOf(ctx, input, args)
}

// fnNowReal returns the current datetime.
func fnNowReal(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return types.Collection{types.NewDateTimeFromTime(time.Now())}, nil
}

// fnTodayReal returns the current date.
func fnTodayReal(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return types.Collection{types.NewDateFromTime(time.Now())}, nil
}

// fnTimeOfDayReal returns the current time.
func fnTimeOfDayReal(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return types.Collection{types.NewTimeFromGoTime(time.Now())}, nil
}

// temporalMeasure is the shape shared by difference() and duration(): both take
// a temporal and a precision, and differ only in how they count.
type temporalMeasure func(from, to types.Value, precision string) (int64, error)

// fnDifference returns the number of boundaries of the given precision crossed
// between the input and the argument.
func fnDifference(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	return measureTemporal("difference", types.TemporalDifference, input, args)
}

// fnDuration returns the number of whole periods of the given precision between
// the input and the argument.
func fnDuration(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	return measureTemporal("duration", types.TemporalDuration, input, args)
}

// measureTemporal applies a temporal measurement to a singleton input and
// argument, mapping the outcomes the specification calls for: empty when either
// operand is empty or specified less precisely than the request, an error when
// the precision does not apply to the type at hand.
func measureTemporal(name string, measure temporalMeasure, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}
	if len(input) > 1 {
		return nil, eval.NewEvalError(eval.ErrSingletonExpected, "%s() requires a singleton input, got %d items", name, len(input))
	}

	target, ok := args[0].(types.Collection)
	if !ok || target.Empty() {
		return types.Collection{}, nil
	}
	if len(target) > 1 {
		return nil, eval.NewEvalError(eval.ErrSingletonExpected, "%s() requires a singleton value argument, got %d items", name, len(target))
	}

	precision, ok := toStringArg(args[1])
	if !ok {
		return types.Collection{}, nil
	}

	result, err := measure(input[0], target[0], precision)
	switch {
	case errors.Is(err, types.ErrPrecisionMismatch):
		// One of the operands is not specified finely enough to answer, which
		// the specification treats as an unknown result rather than an error
		return types.Collection{}, nil
	case err != nil:
		return nil, eval.NewEvalError(eval.ErrInvalidArguments, "%s(): %v", name, err)
	}

	return types.Collection{types.NewInteger(result)}, nil
}
