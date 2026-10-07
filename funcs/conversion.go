package funcs

import (
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

func init() {
	// Register conversion functions
	Register(FuncDef{
		Name:    "iif",
		MinArgs: 2,
		MaxArgs: 3,
		Fn:      fnIif,
	})

	Register(FuncDef{
		Name:    "coalesce",
		MinArgs: 1,
		MaxArgs: -1,
		Fn:      fnCoalesce,
	})

	Register(FuncDef{
		Name:    "toBoolean",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("toBoolean", fnToBoolean),
	})

	Register(FuncDef{
		Name:    "convertsToBoolean",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("convertsToBoolean", fnConvertsToBoolean),
	})

	Register(FuncDef{
		Name:    "toInteger",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("toInteger", fnToInteger),
	})

	Register(FuncDef{
		Name:    "convertsToInteger",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("convertsToInteger", fnConvertsToInteger),
	})

	Register(FuncDef{
		Name:    "toDecimal",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("toDecimal", fnToDecimal),
	})

	Register(FuncDef{
		Name:    "convertsToDecimal",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("convertsToDecimal", fnConvertsToDecimal),
	})

	Register(FuncDef{
		Name:    "toString",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("toString", fnToString),
	})

	Register(FuncDef{
		Name:    "convertsToString",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("convertsToString", fnConvertsToString),
	})

	Register(FuncDef{
		Name:    "toDate",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("toDate", fnToDate),
	})

	Register(FuncDef{
		Name:    "convertsToDate",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("convertsToDate", fnConvertsToDate),
	})

	Register(FuncDef{
		Name:    "toDateTime",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("toDateTime", fnToDateTime),
	})

	Register(FuncDef{
		Name:    "convertsToDateTime",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("convertsToDateTime", fnConvertsToDateTime),
	})

	Register(FuncDef{
		Name:    "toTime",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("toTime", fnToTime),
	})

	Register(FuncDef{
		Name:    "convertsToTime",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      singleInput("convertsToTime", fnConvertsToTime),
	})

	Register(FuncDef{
		Name:    "toQuantity",
		MinArgs: 0,
		MaxArgs: 1,
		Fn:      fnToQuantity,
	})

	Register(FuncDef{
		Name:    "convertsToQuantity",
		MinArgs: 0,
		MaxArgs: 1,
		Fn:      fnConvertsToQuantity,
	})
}

// fnCoalesce returns the first argument that is not an empty collection.
//
// The evaluator intercepts coalesce() to evaluate the arguments lazily, as the
// specification requires; this implementation is the fallback for callers that
// arrive with the arguments already evaluated, and agrees with it on the result.
func fnCoalesce(_ *eval.Context, _ types.Collection, args []interface{}) (types.Collection, error) {
	for _, arg := range args {
		if coll, ok := arg.(types.Collection); ok && !coll.Empty() {
			return coll, nil
		}
	}
	return types.Collection{}, nil
}

// fnIif returns the second argument if the first is true, otherwise the third.
func fnIif(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if len(args) < 2 {
		return nil, eval.InvalidArgumentsError("iif", 2, len(args))
	}
	// As evaluateIif: no context, or a single item
	if len(input) > 1 {
		return nil, eval.NewEvalError(eval.ErrSingletonExpected,
			"iif() takes a single item as its context, got %d", len(input))
	}

	condition := false
	if cond, ok := args[0].(types.Collection); ok {
		var err error
		if condition, err = eval.CriterionBoolean("iif()", cond); err != nil {
			return nil, err
		}
	}

	if condition {
		// Return the true branch
		if result, ok := args[1].(types.Collection); ok {
			return result, nil
		}
		return types.Collection{}, nil
	}

	// Return the false branch (if provided)
	if len(args) > 2 {
		if result, ok := args[2].(types.Collection); ok {
			return result, nil
		}
	}

	return types.Collection{}, nil
}

// fnToBoolean converts the input to a boolean.
func fnToBoolean(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	item := input[0]

	switch v := item.(type) {
	case types.Boolean:
		return systemResult(v), nil
	case types.String:
		str := strings.ToLower(v.Value())
		switch str {
		case "true", "t", "yes", "y", "1", "1.0":
			return types.Collection{types.NewBoolean(true)}, nil
		case "false", "f", "no", "n", "0", "0.0":
			return types.Collection{types.NewBoolean(false)}, nil
		default:
			return types.Collection{}, nil
		}
	case types.Integer:
		if v.Value() == 1 {
			return types.Collection{types.NewBoolean(true)}, nil
		} else if v.Value() == 0 {
			return types.Collection{types.NewBoolean(false)}, nil
		}
		return types.Collection{}, nil
	case types.Decimal:
		if v.Value().Equal(decimal.NewFromInt(1)) {
			return types.Collection{types.NewBoolean(true)}, nil
		} else if v.Value().Equal(decimal.NewFromInt(0)) {
			return types.Collection{types.NewBoolean(false)}, nil
		}
		return types.Collection{}, nil
	default:
		return types.Collection{}, nil
	}
}

// fnConvertsToBoolean returns true if the input can be converted to boolean.
func fnConvertsToBoolean(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{types.NewBoolean(false)}, nil
	}

	item := input[0]

	switch v := item.(type) {
	case types.Boolean:
		return types.Collection{types.NewBoolean(true)}, nil
	case types.String:
		str := strings.ToLower(v.Value())
		switch str {
		case "true", "t", "yes", "y", "1", "1.0", "false", "f", "no", "n", "0", "0.0":
			return types.Collection{types.NewBoolean(true)}, nil
		default:
			return types.Collection{types.NewBoolean(false)}, nil
		}
	case types.Integer:
		if v.Value() == 0 || v.Value() == 1 {
			return types.Collection{types.NewBoolean(true)}, nil
		}
		return types.Collection{types.NewBoolean(false)}, nil
	case types.Decimal:
		if v.Value().Equal(decimal.NewFromInt(0)) || v.Value().Equal(decimal.NewFromInt(1)) {
			return types.Collection{types.NewBoolean(true)}, nil
		}
		return types.Collection{types.NewBoolean(false)}, nil
	default:
		return types.Collection{types.NewBoolean(false)}, nil
	}
}

// fnToInteger converts the input to an integer.
func fnToInteger(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	item := input[0]

	switch v := item.(type) {
	case types.Integer:
		return systemResult(v), nil
	case types.Boolean:
		if v.Bool() {
			return types.Collection{types.NewInteger(1)}, nil
		}
		return types.Collection{types.NewInteger(0)}, nil
	case types.String:
		i, err := strconv.ParseInt(v.Value(), 10, 64)
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{types.NewInteger(i)}, nil
	case types.Decimal:
		return types.Collection{types.NewInteger(v.Value().IntPart())}, nil
	default:
		return types.Collection{}, nil
	}
}

// fnConvertsToInteger returns true if the input can be converted to integer.
func fnConvertsToInteger(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{types.NewBoolean(false)}, nil
	}

	item := input[0]

	switch v := item.(type) {
	case types.Integer:
		return types.Collection{types.NewBoolean(true)}, nil
	case types.Boolean:
		return types.Collection{types.NewBoolean(true)}, nil
	case types.String:
		_, err := strconv.ParseInt(v.Value(), 10, 64)
		return types.Collection{types.NewBoolean(err == nil)}, nil
	case types.Decimal:
		return types.Collection{types.NewBoolean(true)}, nil
	default:
		return types.Collection{types.NewBoolean(false)}, nil
	}
}

// fnToDecimal converts the input to a decimal.
func fnToDecimal(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	item := input[0]

	switch v := item.(type) {
	case types.Decimal:
		return systemResult(v), nil
	case types.Integer:
		return types.Collection{types.NewDecimalFromInt(v.Value())}, nil
	case types.Boolean:
		if v.Bool() {
			return types.Collection{types.NewDecimalFromInt(1)}, nil
		}
		return types.Collection{types.NewDecimalFromInt(0)}, nil
	case types.String:
		d, err := types.NewDecimal(v.Value())
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{d}, nil
	default:
		return types.Collection{}, nil
	}
}

// fnConvertsToDecimal returns true if the input can be converted to decimal.
func fnConvertsToDecimal(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{types.NewBoolean(false)}, nil
	}

	item := input[0]

	switch v := item.(type) {
	case types.Decimal, types.Integer, types.Boolean:
		return types.Collection{types.NewBoolean(true)}, nil
	case types.String:
		_, err := decimal.NewFromString(v.Value())
		return types.Collection{types.NewBoolean(err == nil)}, nil
	default:
		return types.Collection{types.NewBoolean(false)}, nil
	}
}

// fnToString converts the input to a string.
func fnToString(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	if !convertsToString(input[0]) {
		return types.Collection{}, nil
	}
	return types.Collection{types.NewString(input[0].String())}, nil
}

// convertsToString reports whether a value is one of the types toString()
// renders: "a String ... an Integer, Long, Decimal, Date, Time, DateTime, or
// Quantity ... a Boolean". Anything else is empty, and so does not convert.
//
// Shared by toString() and convertsToString() so that the answer and the
// prediction of the answer cannot come apart.
func convertsToString(value types.Value) bool {
	switch value.(type) {
	case types.String, types.Boolean, types.Integer, types.Decimal,
		types.Date, types.Time, types.DateTime, types.Quantity:
		return true
	}
	return false
}

// fnConvertsToString returns true if the input can be converted to string.
func fnConvertsToString(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{types.NewBoolean(false)}, nil
	}

	return types.Collection{types.NewBoolean(convertsToString(input[0]))}, nil
}

// fnToDate converts the input to a date.
func fnToDate(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	switch v := input[0].(type) {
	case types.Date:
		return systemResult(v), nil
	case types.DateTime:
		// Extract date portion (DateTime.String() always has at least date portion)
		d, err := types.NewDate(v.String()[:10])
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{d}, nil
	case types.String:
		d, err := types.NewDate(v.Value())
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{d}, nil
	default:
		return types.Collection{}, nil
	}
}

// fnConvertsToDate returns true if the input can be converted to date.
func fnConvertsToDate(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{types.NewBoolean(false)}, nil
	}

	// Basic check - will be enhanced with temporal types
	if _, ok := input[0].(types.String); ok {
		return types.Collection{types.NewBoolean(true)}, nil
	}

	return types.Collection{types.NewBoolean(false)}, nil
}

// fnToDateTime converts the input to a datetime.
func fnToDateTime(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	switch v := input[0].(type) {
	case types.DateTime:
		return systemResult(v), nil

	case types.Date:
		// "the item is a Date, in which case the result is a DateTime with the
		// year, month, and day of the Date, and the time components empty" —
		// which is the same text, since a date has no time components to drop
		converted, err := types.NewDateTime(v.String())
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{converted}, nil

	case types.String:
		converted, err := types.NewDateTime(v.Value())
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{converted}, nil
	}

	return types.Collection{}, nil
}

// fnConvertsToDateTime returns true if the input can be converted to datetime.
//
// Derived from toDateTime() rather than restated, so that the two cannot
// disagree about what converts.
func fnConvertsToDateTime(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	converted, err := fnToDateTime(ctx, input, args)
	if err != nil {
		return nil, err
	}
	return types.Collection{types.NewBoolean(!converted.Empty())}, nil
}

// fnToTime converts the input to a time.
func fnToTime(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	switch v := input[0].(type) {
	case types.Time:
		return systemResult(v), nil

	case types.String:
		converted, err := types.NewTime(v.Value())
		if err != nil {
			return types.Collection{}, nil
		}
		return types.Collection{converted}, nil
	}

	return types.Collection{}, nil
}

// fnConvertsToTime returns true if the input can be converted to time.
func fnConvertsToTime(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	converted, err := fnToTime(ctx, input, args)
	if err != nil {
		return nil, err
	}
	return types.Collection{types.NewBoolean(!converted.Empty())}, nil
}

// fnToQuantity converts the input to a quantity.
// Accepts an optional unit argument for Integer/Decimal inputs.
func fnToQuantity(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}
	if len(input) > 1 {
		return nil, eval.NewEvalError(eval.ErrSingletonExpected,
			"toQuantity() requires a singleton input, got %d items", len(input))
	}

	quantity, ok := quantityOf(input[0])
	if !ok {
		return types.Collection{}, nil
	}

	// Without a unit argument the quantity stands as it is
	unit, provided := quantityUnitArg(args)
	if !provided {
		return types.Collection{quantity}, nil
	}

	// With one, the value is converted rather than relabelled: 52 'cm' in
	// meters is 0.52 'm', and a value that cannot be converted is empty
	converted, ok := quantity.ConvertTo(unit)
	if !ok {
		return types.Collection{}, nil
	}
	return types.Collection{converted}, nil
}

// quantityOf converts a single value to a Quantity, following the list
// toQuantity() gives: a number takes the UCUM default unit, a Boolean becomes
// one or zero of it, a string is parsed, and a quantity is already one.
func quantityOf(item types.Value) (types.Quantity, bool) {
	switch v := item.(type) {
	case types.Quantity:
		return v, true

	case types.Integer:
		return types.NewQuantityFromDecimal(decimal.NewFromInt(v.Value()), types.DefaultQuantityUnit), true

	case types.Decimal:
		return types.NewQuantityFromDecimal(v.Value(), types.DefaultQuantityUnit), true

	case types.Boolean:
		// "true results in the quantity 1.0 '1', and false results in the
		// quantity 0.0 '1'"
		magnitude := "0.0"
		if v.Bool() {
			magnitude = "1.0"
		}
		value, err := decimal.NewFromString(magnitude)
		if err != nil {
			return types.Quantity{}, false
		}
		return types.NewQuantityFromDecimal(value, types.DefaultQuantityUnit), true

	case types.String:
		return types.ParseQuantityString(v.Value())

	case *types.ObjectValue:
		// FHIR Quantity (and Age, Duration, SimpleQuantity, ...) as a JSON object
		return v.ToQuantity()
	}

	return types.Quantity{}, false
}

// quantityUnitArg reads the optional unit argument, reporting whether one was
// given at all — an absent unit and an empty one lead to different results.
func quantityUnitArg(args []interface{}) (string, bool) {
	if len(args) == 0 {
		return "", false
	}
	argCol, ok := args[0].(types.Collection)
	if !ok || argCol.Empty() {
		return "", false
	}
	unit, ok := argCol[0].(types.String)
	if !ok {
		return "", false
	}
	return unit.Value(), true
}

// fnConvertsToQuantity returns true if the input can be converted to quantity.
// If a unit argument is provided, returns true only if the quantity can be converted to that unit.
func fnConvertsToQuantity(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{types.NewBoolean(false)}, nil
	}

	// Derived from toQuantity() rather than restated, so that the two cannot
	// disagree about what converts. This function answers exactly "would
	// toQuantity() return something", which is what the specification defines it
	// to mean.
	converted, err := fnToQuantity(ctx, input, args)
	if err != nil {
		return nil, err
	}
	return types.Collection{types.NewBoolean(!converted.Empty())}, nil
}

// systemResult is a conversion's result when the input already has the type
// asked for: the same value as a System value, as a function's result is,
// without the FHIR type it was read as or the element beside it. Returned as
// it stood, active.toBoolean() was still a FHIR boolean, and hasValue() and
// value answered for it.
func systemResult(value types.Value) types.Collection {
	if system, ok := types.SystemValue(value); ok {
		return types.Collection{system}
	}
	return types.Collection{value}
}
