package funcs

import (
	"errors"
	"math"

	"github.com/shopspring/decimal"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

func init() {
	// Register math functions
	Register(FuncDef{
		Name:    "abs",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("abs", fnAbs),
	})

	Register(FuncDef{
		Name:    "ceiling",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("ceiling", fnCeiling),
	})

	Register(FuncDef{
		Name:    "exp",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("exp", fnExp),
	})

	Register(FuncDef{
		Name:    "floor",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("floor", fnFloor),
	})

	Register(FuncDef{
		Name:    "ln",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("ln", fnLn),
	})

	Register(FuncDef{
		Name:    "log",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      singleInput("log", fnLog),
	})

	Register(FuncDef{
		Name:    "power",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      singleInput("power", fnPower),
	})

	Register(FuncDef{
		Name:    "round",
		MinArgs: 0,
		MaxArgs: 1,
		Fn:      singleInput("round", fnRound),
	})

	Register(FuncDef{
		Name:    "sqrt",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("sqrt", fnSqrt),
	})

	Register(FuncDef{
		Name:    "truncate",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("truncate", fnTruncate),
	})

	// Aggregate functions
	Register(FuncDef{
		Name:    "sum",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnSum,
	})

	Register(FuncDef{
		Name:    "min",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnMin,
	})

	Register(FuncDef{
		Name:    "max",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnMax,
	})

	Register(FuncDef{
		Name:    "avg",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnAvg,
	})
}

// fnAbs returns the absolute value.
func fnAbs(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	switch v := input[0].(type) {
	case types.Integer:
		val := v.Value()
		if val < 0 {
			val = -val
		}
		return types.Collection{types.NewInteger(val)}, nil
	case types.Decimal:
		// Through the decimal's own absolute value rather than a float, which
		// would round: abs() must not change the number it is given
		return types.Collection{types.NewDecimalFromDecimal(v.Value().Abs())}, nil
	case types.Quantity:
		// "abs() : Integer | Decimal | Quantity" — the unit is carried through
		return types.Collection{v.Abs()}, nil
	default:
		return types.Collection{}, nil
	}
}

// fnCeiling returns the smallest integer >= input.
func fnCeiling(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	switch v := input[0].(type) {
	case types.Integer:
		return types.Collection{v}, nil
	case types.Decimal:
		ceil := math.Ceil(v.Value().InexactFloat64())
		return types.Collection{types.NewInteger(int64(ceil))}, nil
	default:
		return types.Collection{}, nil
	}
}

// fnExp returns e raised to the power of input.
func fnExp(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	var val float64
	switch v := input[0].(type) {
	case types.Integer:
		val = float64(v.Value())
	case types.Decimal:
		val = v.Value().InexactFloat64()
	default:
		return types.Collection{}, nil
	}

	return floatResult(math.Exp(val)), nil
}

// fnFloor returns the largest integer <= input.
func fnFloor(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	switch v := input[0].(type) {
	case types.Integer:
		return types.Collection{v}, nil
	case types.Decimal:
		floor := math.Floor(v.Value().InexactFloat64())
		return types.Collection{types.NewInteger(int64(floor))}, nil
	default:
		return types.Collection{}, nil
	}
}

// fnLn returns the natural logarithm.
func fnLn(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	var val float64
	switch v := input[0].(type) {
	case types.Integer:
		val = float64(v.Value())
	case types.Decimal:
		val = v.Value().InexactFloat64()
	default:
		return types.Collection{}, nil
	}

	if val <= 0 {
		return types.Collection{}, nil
	}

	return floatResult(math.Log(val)), nil
}

// fnLog returns the logarithm with the given base.
func fnLog(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() || len(args) == 0 {
		return types.Collection{}, nil
	}

	var val float64
	switch v := input[0].(type) {
	case types.Integer:
		val = float64(v.Value())
	case types.Decimal:
		val = v.Value().InexactFloat64()
	default:
		return types.Collection{}, nil
	}

	base, err := toFloat(args[0])
	if err != nil {
		return types.Collection{}, nil
	}

	if val <= 0 || base <= 0 || base == 1 {
		return types.Collection{}, nil
	}

	result := math.Log(val) / math.Log(base)
	return floatResult(result), nil
}

// fnPower returns input raised to the given power.
func fnPower(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() || len(args) == 0 {
		return types.Collection{}, nil
	}

	var base float64
	switch v := input[0].(type) {
	case types.Integer:
		base = float64(v.Value())
	case types.Decimal:
		base = v.Value().InexactFloat64()
	default:
		return types.Collection{}, nil
	}

	exp, err := toFloat(args[0])
	if err != nil {
		return types.Collection{}, nil
	}

	result := math.Pow(base, exp)

	// Check for invalid results
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return types.Collection{}, nil
	}

	return floatResult(result), nil
}

// fnRound rounds to the specified number of decimal places.
func fnRound(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	precision := int32(0)
	if len(args) > 0 {
		p, err := toInteger(args[0])
		if err != nil {
			return types.Collection{}, nil
		}
		// "the number of digits of precision must be >= 0 or the evaluation
		// will end and signal an error"; rounding 1.5 to ten million places
		// left of the point took seconds besides
		if p < 0 {
			return nil, eval.NewEvalError(eval.ErrInvalidArguments, "round() precision must be >= 0, got %d", p)
		}
		// Limit precision to reasonable bounds to avoid overflow
		if p > math.MaxInt32 {
			p = math.MaxInt32
		} else if p < math.MinInt32 {
			p = math.MinInt32
		}
		precision = int32(p)
	}

	switch v := input[0].(type) {
	case types.Integer:
		return types.Collection{v}, nil
	case types.Decimal:
		// More places than this engine's decimals hold is an overflow, found
		// before writing them out
		if precision > types.DecimalDigits {
			return types.Collection{}, nil
		}
		rounded := v.Value().Round(precision)
		d, err := types.NewDecimal(rounded.String())
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{d}, nil
	default:
		return types.Collection{}, nil
	}
}

// fnSqrt returns the square root.
func fnSqrt(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	var val float64
	switch v := input[0].(type) {
	case types.Integer:
		val = float64(v.Value())
	case types.Decimal:
		val = v.Value().InexactFloat64()
	default:
		return types.Collection{}, nil
	}

	if val < 0 {
		return types.Collection{}, nil
	}

	return floatResult(math.Sqrt(val)), nil
}

// fnTruncate returns the integer part (truncates toward zero).
func fnTruncate(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	switch v := input[0].(type) {
	case types.Integer:
		return types.Collection{v}, nil
	case types.Decimal:
		trunc := math.Trunc(v.Value().InexactFloat64())
		return types.Collection{types.NewInteger(int64(trunc))}, nil
	default:
		return types.Collection{}, nil
	}
}

// toFloat converts an argument to float64.
func toFloat(arg interface{}) (float64, error) {
	switch v := arg.(type) {
	case types.Collection:
		if v.Empty() {
			return 0, eval.NewEvalError(eval.ErrType, "expected number, got empty collection")
		}
		return toFloat(v[0])
	case types.Integer:
		return float64(v.Value()), nil
	case types.Decimal:
		return v.Value().InexactFloat64(), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case decimal.Decimal:
		return v.InexactFloat64(), nil
	default:
		return 0, eval.NewEvalError(eval.ErrType, "expected number")
	}
}

// fnSum returns the sum of all numeric values in the collection.
// Returns empty if the collection is empty or contains non-numeric values.
func fnSum(ctx *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{types.NewInteger(0)}, nil
	}

	// Check for cancellation in large collections
	if err := ctx.CheckCancellation(); err != nil {
		return nil, err
	}

	var sum decimal.Decimal
	hasDecimal := false

	for _, item := range input {
		switch v := item.(type) {
		case types.Integer:
			sum = sum.Add(decimal.NewFromInt(v.Value()))
		case types.Decimal:
			sum = sum.Add(v.Value())
			hasDecimal = true
		default:
			// Non-numeric value - return empty per FHIRPath spec
			return types.Collection{}, nil
		}
	}

	// Return Integer if all inputs were Integer, otherwise Decimal
	if hasDecimal {
		d, err := types.NewDecimal(sum.String())
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{d}, nil
	}
	return types.Collection{types.NewInteger(sum.IntPart())}, nil
}

// findExtreme finds the minimum or, when findMin is false, the maximum of a
// collection. "Comparison semantics are defined by the Comparison Operators
// for the type of value being aggregated", so items compare as < compares
// them: Integer against Decimal exactly, quantities through their units,
// temporals by their components. "All items in the input collection SHALL be
// the same type, otherwise an exception is thrown": a pair < refuses, 1 and
// 'a', ends the evaluation.
//
// A pair whose order is unknown, temporals at different precisions or
// quantities whose units do not convert, does not end it. The extreme is the
// item that orders against every other and stands below (or above) them all,
// @2019 among @2019, @2020-01-01T10Z and @2020-01-01T10:30Z for min(); where
// no item does, as for max() there, there is none and the result is empty, as
// < answers for such a pair. The answer does not depend on the order of the
// input.
func findExtreme(ctx *eval.Context, input types.Collection, findMin bool) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}
	if err := ctx.CheckCancellation(); err != nil {
		return nil, err
	}
	for _, item := range input {
		if !aggregable(item) {
			return types.Collection{}, nil
		}
	}

	// below reports whether a stands below b, for min(), or above it; known is
	// false where their order is unknown
	below := func(a, b types.Value) (stands, known bool, err error) {
		cmp, err := eval.Compare(a, b)
		if err != nil {
			if unknownOrder(err) {
				return false, false, nil
			}
			return false, false, err
		}
		if findMin {
			return cmp < 0, true, nil
		}
		return cmp > 0, true, nil
	}
	// extreme reports whether a orders against every item and none stands
	// beyond it
	extreme := func(a types.Value) (bool, error) {
		for _, other := range input {
			beyond, known, err := below(other, a)
			if err != nil || !known || beyond {
				return false, err
			}
		}
		return true, nil
	}

	// One pass finds the extreme where every pair orders; where it is checked
	// and holds, it is the answer
	candidate := input[0]
	for _, item := range input[1:] {
		beyond, known, err := below(item, candidate)
		if err != nil {
			return nil, err
		}
		if known && beyond {
			candidate = item
		}
	}
	if ok, err := extreme(candidate); err != nil || ok {
		if err != nil {
			return nil, err
		}
		return types.Collection{candidate}, nil
	}

	// Some pair has no order: the extreme, if any item is one, is found by
	// checking each
	for i, item := range input {
		if i%100 == 0 {
			if err := ctx.CheckCancellation(); err != nil {
				return nil, err
			}
		}
		ok, err := extreme(item)
		if err != nil {
			return nil, err
		}
		if ok {
			return types.Collection{item}, nil
		}
	}
	return types.Collection{}, nil
}

// aggregable reports an item min() and max() order: a number, a string, a
// temporal, or a quantity, a FHIR Quantity among them.
func aggregable(item types.Value) bool {
	switch v := item.(type) {
	case types.Integer, types.Decimal, types.String, types.Date, types.DateTime, types.Time, types.Quantity:
		return true
	case *types.ObjectValue:
		_, ok, err := eval.AsQuantity(v)
		return ok || err != nil
	}
	return false
}

// unknownOrder reports a comparison that has no answer, rather than one that
// is refused: < answers empty for it.
func unknownOrder(err error) bool {
	return types.IsUnknownTemporalComparison(err) || errors.Is(err, types.ErrIncompatibleUnits) ||
		errors.Is(err, types.ErrDecimalRange)
}

// fnMin returns the minimum value in the collection.
// Returns empty if the collection is empty.
func fnMin(ctx *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return findExtreme(ctx, input, true)
}

// fnMax returns the maximum value in the collection.
// Returns empty if the collection is empty.
func fnMax(ctx *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return findExtreme(ctx, input, false)
}

// fnAvg returns the average of all numeric values in the collection.
// Returns empty if the collection is empty or contains non-numeric values.
func fnAvg(ctx *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	// Check for cancellation
	if err := ctx.CheckCancellation(); err != nil {
		return nil, err
	}

	var sum decimal.Decimal
	count := 0

	for _, item := range input {
		switch v := item.(type) {
		case types.Integer:
			sum = sum.Add(decimal.NewFromInt(v.Value()))
			count++
		case types.Decimal:
			sum = sum.Add(v.Value())
			count++
		default:
			// Non-numeric value - return empty per FHIRPath spec
			return types.Collection{}, nil
		}
	}

	if count == 0 {
		return types.Collection{}, nil
	}

	avg := sum.Div(decimal.NewFromInt(int64(count)))
	d, err := types.NewDecimal(avg.String())
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{d}, nil
}

// floatResult wraps the result of a function computed in float64. A result
// with no finite value, which no decimal holds, overflows to empty: 1000.exp()
// is +Inf.
func floatResult(f float64) types.Collection {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return types.Collection{}
	}
	return types.Collection{types.NewDecimalFromFloat(f)}
}
