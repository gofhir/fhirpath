package funcs

import (
	"errors"
	"math"
	"strconv"
	"strings"

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
		Fn:      singleInput("log", singleArgs("log", fnLog)),
	})

	Register(FuncDef{
		Name:    "power",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      singleInput("power", singleArgs("power", fnPower)),
	})

	Register(FuncDef{
		Name:    "round",
		MinArgs: 0,
		MaxArgs: 1,
		Fn:      singleInput("round", singleArgs("round", fnRound)),
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

// The Math functions take numbers, and the rounding ones and abs() a
// Quantity too, whose unit they keep; any other input, a String, a Boolean or
// a date, is "not compatible with the expected type" and ends the evaluation,
// as fhirpath.js and the HL7 validator both refuse it. A number argument is
// read the same way: an Integer or a Decimal, not a String to convert.

// fnAbs returns the absolute value.
func fnAbs(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if noNumber(input) {
		return types.Collection{}, nil
	}

	switch v := input[0].(type) {
	case types.Integer:
		// The least int64 has no absolute value within the range: an
		// overflow, which is empty
		abs, err := v.AbsExact()
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{abs}, nil
	case types.Decimal:
		// Through the decimal's own absolute value rather than a float, which
		// would round: abs() must not change the number it is given
		return types.Collection{types.NewDecimalFromDecimal(v.Value().Abs())}, nil
	}
	// "abs() : Integer | Decimal | Quantity" — the unit is carried through,
	// a FHIR Quantity's too
	q, err := quantityInput("abs", input[0])
	if err != nil {
		return nil, err
	}
	return types.Collection{q.Abs()}, nil
}

// noNumber reports an input or argument with nothing to compute: empty, a
// primitive written with no value, or a FHIR Quantity with a unit and no
// value, which the operators read as empty too.
func noNumber(c types.Collection) bool {
	return c.Empty() || types.IsValueless(c[0]) || types.IsValuelessQuantity(c[0])
}

// quantityInput reads the input of a function that takes a Quantity besides
// numbers: a quantity, or a FHIR Quantity; anything else is the wrong type.
func quantityInput(name string, v types.Value) (types.Quantity, error) {
	q, ok, err := eval.AsQuantity(v)
	if err != nil {
		return types.Quantity{}, err
	}
	if !ok {
		return types.Quantity{}, eval.TypeError("Integer, Decimal or Quantity", v.Type(), name)
	}
	return q, nil
}

// numberInput reads the input of a function on numbers alone.
func numberInput(name string, v types.Value) (float64, error) {
	switch n := v.(type) {
	case types.Integer:
		return float64(n.Value()), nil
	case types.Decimal:
		return n.Value().InexactFloat64(), nil
	}
	return 0, eval.TypeError("Integer or Decimal", v.Type(), name)
}

// numberSign returns the sign of an Integer or a Decimal, read from the
// number itself.
func numberSign(v types.Value) int {
	switch n := v.(type) {
	case types.Integer:
		return cmpInt64(n.Value(), 0)
	case types.Decimal:
		return n.Value().Sign()
	}
	return 0
}

// numberArgument reads a function's number argument. empty is true where it
// evaluates to nothing, which makes the result empty.
func numberArgument(name string, arg interface{}) (value float64, empty bool, err error) {
	c, ok := arg.(types.Collection)
	if !ok || noNumber(c) {
		return 0, true, nil
	}
	value, err = numberInput(name, c[0])
	return value, false, err
}

// wholeOf applies a rounding to a whole number, ceiling(), floor() or
// truncate(): an Integer is its own, a Decimal gives an Integer, and a
// Quantity a Quantity in its unit, as 3.0.0 has it and fhirpath.js answers.
func wholeOf(name string, input types.Collection, whole func(decimal.Decimal) decimal.Decimal) (types.Collection, error) {
	if noNumber(input) {
		return types.Collection{}, nil
	}
	switch v := input[0].(type) {
	case types.Integer:
		return types.Collection{v}, nil
	case types.Decimal:
		return wholeInteger(whole(v.Value()))
	}
	q, err := quantityInput(name, input[0])
	if err != nil {
		return nil, err
	}
	return quantityResult(whole(q.Value()), q.Unit()), nil
}

// quantityResult is a rounded quantity, at its natural scale as a rounded
// Decimal is, 1.5 'mg'.round(2) being 1.5 'mg', and empty past the decimals
// this engine holds, as arithmetic overflows.
func quantityResult(value decimal.Decimal, unit string) types.Collection {
	natural, err := decimal.NewFromString(value.String())
	if err != nil {
		return types.Collection{}
	}
	q, ok := types.WithinRange(types.NewQuantityFromDecimal(natural, unit))
	if !ok {
		return types.Collection{}
	}
	return types.Collection{q}
}

// fnCeiling returns the smallest integer >= input.
func fnCeiling(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return wholeOf("ceiling", input, decimal.Decimal.Ceil)
}

// fnFloor returns the largest integer <= input.
func fnFloor(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return wholeOf("floor", input, decimal.Decimal.Floor)
}

// fnTruncate returns the integer part (truncates toward zero).
func fnTruncate(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return wholeOf("truncate", input, func(d decimal.Decimal) decimal.Decimal { return d.Truncate(0) })
}

// fnExp returns e raised to the power of input.
func fnExp(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if noNumber(input) {
		return types.Collection{}, nil
	}
	val, err := numberInput("exp", input[0])
	if err != nil {
		return nil, err
	}
	return floatResult(math.Exp(val)), nil
}

// fnLn returns the natural logarithm.
func fnLn(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if noNumber(input) {
		return types.Collection{}, nil
	}
	if _, err := numberInput("ln", input[0]); err != nil {
		return nil, err
	}
	if numberSign(input[0]) <= 0 {
		return types.Collection{}, nil
	}
	return floatResult(naturalLog(input[0])), nil
}

// smallestNormal is the smallest positive normal float64, 2^-1022.
const smallestNormal = 0x1p-1022

// naturalLog returns the natural logarithm of a positive Integer or Decimal,
// read from its digits and exponent rather than through a float64, which
// holds a decimal far below its range as zero: ln of 0.(340 zeros)1 is about
// -785.
func naturalLog(v types.Value) float64 {
	var d decimal.Decimal
	switch n := v.(type) {
	case types.Integer:
		d = decimal.NewFromInt(n.Value())
	case types.Decimal:
		d = n.Value()
	}
	// Where a float64 holds it, its own logarithm, which is exact where it can
	// be: 16.log(2) is 4
	if f := d.InexactFloat64(); f >= smallestNormal && !math.IsInf(f, 0) {
		return math.Log(f)
	}
	// Otherwise d is m × 10^e with m in [1, 10)
	e := int64(d.Exponent()) + int64(d.NumDigits()) - 1
	m := d.Shift(int32(-e)).InexactFloat64() //nolint:gosec // e is bounded by the decimals this engine holds
	return math.Log(m) + float64(e)*math.Ln10
}

// fnLog returns the logarithm with the given base. 3.0.0: "If the input is 0
// or negative, the evaluation will end and signal an error to the calling
// environment", and the same of the base argument.
func fnLog(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if noNumber(input) || len(args) == 0 {
		return types.Collection{}, nil
	}
	if _, err := numberInput("log", input[0]); err != nil {
		return nil, err
	}
	_, empty, err := numberArgument("log", args[0])
	if err != nil || empty {
		return types.Collection{}, err
	}
	// Positive as written, not as a float holds it: a decimal far below
	// float64's range is still above zero
	baseValue := args[0].(types.Collection)[0]
	if numberSign(input[0]) <= 0 || numberSign(baseValue) <= 0 {
		return nil, eval.NewEvalError(eval.ErrInvalidArguments,
			"log() takes a positive input and base, got %s and %s", input[0], baseValue)
	}
	// A base of 1 divides by ln 1, which is zero
	lnBase := naturalLog(baseValue)
	if lnBase == 0 {
		return types.Collection{}, nil
	}
	return floatResult(naturalLog(input[0]) / lnBase), nil
}

// fnPower returns input raised to the given power.
func fnPower(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if noNumber(input) || len(args) == 0 {
		return types.Collection{}, nil
	}
	base, err := numberInput("power", input[0])
	if err != nil {
		return nil, err
	}
	exp, empty, err := numberArgument("power", args[0])
	if err != nil || empty {
		return types.Collection{}, err
	}

	result := math.Pow(base, exp)

	// A result with no number, (-1).power(0.5), is empty
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return types.Collection{}, nil
	}

	return floatResult(result), nil
}

// fnRound rounds to the specified number of decimal places, an Integer as a
// Decimal: "round([precision : Integer]) : Decimal". A negative precision
// rounds left of the point, 15.round(-1) is 20, as fhirpath.js and the HL7
// validator both answer, though the text makes it an error.
func fnRound(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if noNumber(input) {
		return types.Collection{}, nil
	}

	precision := int32(0)
	if len(args) > 0 {
		c, _ := args[0].(types.Collection)
		if noNumber(c) {
			return types.Collection{}, nil
		}
		p, ok := c[0].(types.Integer)
		if !ok {
			return nil, eval.TypeError("Integer", c[0].Type(), "round")
		}
		// More places than this engine's decimals hold, either way: past them
		// to the right is an overflow; to the left every value rounds to zero,
		// found without rounding 1.5 to ten million places, which took seconds
		switch {
		case p.Value() > int64(types.DecimalDigits):
			return types.Collection{}, nil
		case p.Value() < -int64(types.DecimalDigits):
			precision = -types.DecimalDigits - 1
		default:
			precision = int32(p.Value()) //nolint:gosec // bounded just above
		}
	}

	var value decimal.Decimal
	switch v := input[0].(type) {
	case types.Integer:
		value = decimal.NewFromInt(v.Value())
	case types.Decimal:
		value = v.Value()
	default:
		q, err := quantityInput("round", input[0])
		if err != nil {
			return nil, err
		}
		return quantityResult(q.Value().Round(precision), q.Unit()), nil
	}
	d, err := types.NewDecimal(value.Round(precision).String())
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{d}, nil
}

// fnSqrt returns the square root.
func fnSqrt(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if noNumber(input) {
		return types.Collection{}, nil
	}
	val, err := numberInput("sqrt", input[0])
	if err != nil {
		return nil, err
	}
	if val < 0 {
		return types.Collection{}, nil
	}
	return floatResult(math.Sqrt(val)), nil
}

// wholeInteger answers a whole decimal as the Integer ceiling(), floor() and
// truncate() give, read from the decimal itself rather than through a float,
// which held 9007199254740993 as 9007199254740994. One past the int64 range
// is an overflow, and empty.
func wholeInteger(whole decimal.Decimal) (types.Collection, error) {
	i, err := types.IntegerFromDecimal(whole)
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{i}, nil
}

// fnSum returns the sum of the input's items, numbers or quantities: "If the
// input collection is empty ({ }), the result is empty.", and "All items in the
// input collection SHALL be the same type, otherwise an exception is thrown"
// (3.0.0). Integers and Decimals together add as Decimals, the implicit
// conversion the text leaves open there. Quantities add as + adds them, and
// ones whose units do not combine give empty.
func fnSum(ctx *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() || holdsNoAmount(input) {
		return types.Collection{}, nil
	}
	if err := ctx.CheckCancellation(); err != nil {
		return nil, err
	}
	if quantities, ok, err := allQuantities("sum", input); ok || err != nil {
		if err != nil {
			return nil, err
		}
		total, ok := sumQuantities(quantities)
		if !ok {
			return types.Collection{}, nil
		}
		// A sum past the decimals this engine holds is an overflow, as for +
		if bounded, inRange := types.WithinRange(total); inRange {
			return types.Collection{bounded}, nil
		}
		return types.Collection{}, nil
	}

	sum, hasDecimal, err := sumNumbers("sum", input)
	if err != nil {
		return nil, err
	}
	// Return Integer if all inputs were Integer, otherwise Decimal
	if hasDecimal {
		d, err := types.NewDecimal(sum.String())
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{d}, nil
	}
	// A sum past the int64 range is an overflow, and empty
	return wholeInteger(sum)
}

// holdsNoAmount reports an input with an item that has nothing to add: a
// primitive with no value, a FHIR Quantity with a unit and no value, or one
// whose value overflows. The sum is unknown, empty, as + answers for such an
// operand, and such an item is no item of another type.
func holdsNoAmount(input types.Collection) bool {
	for _, item := range input {
		if types.IsValueless(item) || types.IsValuelessQuantity(item) {
			return true
		}
		if _, _, err := eval.AsQuantity(item); err != nil {
			return true
		}
	}
	return false
}

// sumNumbers adds Integers and Decimals, reporting whether any was a Decimal;
// any other item is of another type, and ends the evaluation.
func sumNumbers(name string, input types.Collection) (decimal.Decimal, bool, error) {
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
			return decimal.Decimal{}, false, eval.NewEvalError(eval.ErrType,
				"%s() takes Integer, Decimal or Quantity items of one type, and found %s", name, item.Type())
		}
	}
	return sum, hasDecimal, nil
}

// allQuantities reads the input as quantities, a FHIR Quantity among them,
// when its first item is one; ok is false when it is not. An item that is no
// quantity beside one ends the evaluation.
func allQuantities(name string, input types.Collection) ([]types.Quantity, bool, error) {
	if _, ok := quantityItem(input[0]); !ok {
		return nil, false, nil
	}
	quantities := make([]types.Quantity, len(input))
	for i, item := range input {
		q, ok := quantityItem(item)
		if !ok {
			return nil, false, eval.NewEvalError(eval.ErrType,
				"%s() takes items of one type, and found %s beside a Quantity", name, item.Type())
		}
		quantities[i] = q
	}
	return quantities, true, nil
}

// quantityItem reads an item as a quantity, a literal one or a FHIR Quantity.
func quantityItem(item types.Value) (types.Quantity, bool) {
	q, ok, err := eval.AsQuantity(item)
	return q, ok && err == nil
}

// sumQuantities adds quantities as + adds them; false where two do not
// combine.
func sumQuantities(quantities []types.Quantity) (types.Quantity, bool) {
	total := quantities[0]
	for _, q := range quantities[1:] {
		next, err := total.Add(q)
		if err != nil {
			return types.Quantity{}, false
		}
		total = next
	}
	return total, true
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
	// Numbers, or strings, always order, and compare directly in one pass
	if extremeItem, ok := totalExtreme(input, findMin); ok {
		return types.Collection{extremeItem}, nil
	}
	for _, item := range input {
		if !aggregable(item) {
			return types.Collection{}, nil
		}
	}
	// One item is its own extreme, compared with nothing: an item that orders
	// against no other, a quantity in a unit no other converts to, is still
	// the only one there is
	if len(input) == 1 {
		return input, nil
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
	// beyond it. The item that ruled out the last one tried is tried first:
	// items alike are ruled out alike, so a collection of many equal items and
	// one they do not order against is checked in one pass, not one per item.
	var witness types.Value
	extreme := func(a types.Value) (bool, error) {
		if witness != nil {
			if beyond, known, err := below(witness, a); err != nil || !known || beyond {
				return false, err
			}
		}
		for _, other := range input {
			beyond, known, err := below(other, a)
			if err != nil || !known || beyond {
				witness = other
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
	// checking each. An item written as one already ruled out is ruled out
	// too, unchecked: it compares alike against every other
	tried := map[string]bool{}
	for i, item := range input {
		if i%100 == 0 {
			if err := ctx.CheckCancellation(); err != nil {
				return nil, err
			}
		}
		key := triedKey(item)
		if tried[key] {
			continue
		}
		tried[key] = true
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

// totalExtreme finds the extreme of a collection of numbers only, or strings
// only, which always order, in one pass and without eval.Compare; the first
// of equal items is kept. It reports false for any other collection.
func totalExtreme(input types.Collection, findMin bool) (types.Value, bool) {
	beyond := func(cmp int) bool {
		if findMin {
			return cmp < 0
		}
		return cmp > 0
	}
	switch first := input[0].(type) {
	case types.String:
		extremeItem := first
		for _, item := range input[1:] {
			s, ok := item.(types.String)
			if !ok {
				return nil, false
			}
			if beyond(strings.Compare(s.Value(), extremeItem.Value())) {
				extremeItem = s
			}
		}
		return extremeItem, true
	case types.Integer:
		// Integers only compare as int64s; a Decimal among them, as decimals
		if extremeItem, ok := integerExtreme(input, beyond); ok {
			return extremeItem, true
		}
		return decimalExtreme(input, beyond)
	case types.Decimal:
		return decimalExtreme(input, beyond)
	}
	return nil, false
}

// integerExtreme is totalExtreme over integers only, reporting false where any
// item is not an Integer.
func integerExtreme(input types.Collection, beyond func(int) bool) (types.Value, bool) {
	extremeItem, ok := input[0].(types.Integer)
	if !ok {
		return nil, false
	}
	for _, item := range input[1:] {
		i, isInteger := item.(types.Integer)
		if !isInteger {
			return nil, false
		}
		if beyond(cmpInt64(i.Value(), extremeItem.Value())) {
			extremeItem = i
		}
	}
	return extremeItem, true
}

// cmpInt64 orders two int64s as Cmp does.
func cmpInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// decimalExtreme is totalExtreme over Integers and Decimals, compared as
// decimals, reporting false where any item is neither.
func decimalExtreme(input types.Collection, beyond func(int) bool) (types.Value, bool) {
	extremeItem, extremeValue := input[0], numericValue(input[0])
	for _, item := range input[1:] {
		switch item.(type) {
		case types.Integer, types.Decimal:
		default:
			return nil, false
		}
		if value := numericValue(item); beyond(value.Cmp(extremeValue)) {
			extremeItem, extremeValue = item, value
		}
	}
	return extremeItem, true
}

// numericValue returns an Integer's or a Decimal's value as a decimal.
func numericValue(item types.Value) decimal.Decimal {
	if i, ok := item.(types.Integer); ok {
		return decimal.NewFromInt(i.Value())
	}
	return item.(types.Decimal).Value()
}

// triedKey names an item as written, and for a DateTime with the offset it
// is placed at, which a default offset sets without changing how it is
// written: items with one key compare alike against every other.
func triedKey(item types.Value) string {
	key := item.Type() + "\x00" + item.String()
	if dt, ok := item.(types.DateTime); ok {
		if offset, placed := dt.EffectiveOffset(); placed {
			key += "\x00" + strconv.Itoa(offset)
		}
	}
	return key
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

// fnAvg returns the average of the input's items, numbers or quantities, as
// a Decimal or a Quantity: empty for an empty input, and an error for items of
// different types, as sum() (3.0.0).
func fnAvg(ctx *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() || holdsNoAmount(input) {
		return types.Collection{}, nil
	}
	if err := ctx.CheckCancellation(); err != nil {
		return nil, err
	}
	count := decimal.NewFromInt(int64(len(input)))
	if quantities, ok, err := allQuantities("avg", input); ok || err != nil {
		if err != nil {
			return nil, err
		}
		total, ok := sumQuantities(quantities)
		if !ok {
			return types.Collection{}, nil
		}
		avg, err := total.Divide(count)
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{avg}, nil
	}

	sum, _, err := sumNumbers("avg", input)
	if err != nil {
		return nil, err
	}
	d, err := types.NewDecimal(sum.Div(count).String())
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
