package eval

import (
	"errors"

	"github.com/shopspring/decimal"

	"github.com/gofhir/fhirpath/types"
)

// asQuantity converts a value to a Quantity when possible.
// FHIR carries quantities as JSON objects (Quantity, SimpleQuantity, Age,
// Duration, Count, Distance, ...), so they must be converted before any
// quantity operation — Range.low and Range.high are both objects, never
// [types.Quantity] values.
func asQuantity(v types.Value) (types.Quantity, bool) {
	switch q := v.(type) {
	case types.Quantity:
		return q, true
	case *types.ObjectValue:
		return q.ToQuantity()
	}
	return types.Quantity{}, false
}

// quantityOperands converts both operands to quantities when at least one of
// them is a FHIR quantity object. Returns false when either side does not
// convert, so that plain object operands keep their existing behavior.
func quantityOperands(left, right types.Value) (lq, rq types.Quantity, ok bool) {
	_, leftIsObject := left.(*types.ObjectValue)
	_, rightIsObject := right.(*types.ObjectValue)
	if !leftIsObject && !rightIsObject {
		return types.Quantity{}, types.Quantity{}, false
	}

	lq, lok := asQuantity(left)
	rq, rok := asQuantity(right)
	if !lok || !rok {
		return types.Quantity{}, types.Quantity{}, false
	}
	return lq, rq, true
}

// bothQuantities matches two quantities, whose units combine rather than having
// to agree.
func bothQuantities(left, right types.Value) (lq, rq types.Quantity, ok bool) {
	lq, lok := asQuantity(left)
	rq, rok := asQuantity(right)
	if !lok || !rok || lq.Unit() == "" || rq.Unit() == "" {
		return types.Quantity{}, types.Quantity{}, false
	}
	return lq, rq, true
}

// quantityAndNumber matches a quantity scaled by a plain number, in either
// order for multiplication. Division only ever has the quantity on the left,
// which the caller enforces.
func quantityAndNumber(left, right types.Value) (q types.Quantity, factor decimal.Decimal, ok bool) {
	if lq, converted := asQuantity(left); converted {
		if n, isNumeric := right.(types.Numeric); isNumeric {
			return lq, n.ToDecimal().Value(), true
		}
	}
	if rq, converted := asQuantity(right); converted {
		if n, isNumeric := left.(types.Numeric); isNumeric {
			return rq, n.ToDecimal().Value(), true
		}
	}
	return types.Quantity{}, decimal.Decimal{}, false
}

// Arithmetic operators

// shiftTemporal applies a duration to a Date or DateTime, which is the one
// arithmetic both + and - share verbatim: the same operand pairing, the same
// unit handling, differing only in direction.
//
// Reports false when the operands are not a temporal and a duration, leaving
// the caller to carry on with its own dispatch.
func shiftTemporal(left, right types.Value, subtract bool) (types.Value, bool, error) {
	// asQuantity rather than a type assertion, so that a duration read from FHIR
	// data works as well as a literal: Patient.birthDate + Observation.value is
	// the ordinary way to write this, and the value arrives as a FHIR Quantity
	// object rather than a System.Quantity.
	quantity, ok := asQuantity(right)
	if !ok {
		return nil, false, nil
	}

	amount := int(quantity.Value().IntPart())
	unit := quantity.Unit()

	switch temporal := left.(type) {
	case types.Date:
		if subtract {
			result, err := temporal.SubtractDuration(amount, unit)
			return result, true, err
		}
		result, err := temporal.AddDuration(amount, unit)
		return result, true, err

	case types.DateTime:
		if subtract {
			result, err := temporal.SubtractDuration(amount, unit)
			return result, true, err
		}
		result, err := temporal.AddDuration(amount, unit)
		return result, true, err
	}

	return nil, false, nil
}

// Add performs addition on two values.
func Add(left, right types.Value) (types.Value, error) {
	// FHIR quantity objects: route through Quantity arithmetic.
	if lq, rq, ok := quantityOperands(left, right); ok {
		return lq.Add(rq)
	}

	// Date and DateTime take a duration on the right
	if result, ok, err := shiftTemporal(left, right, false); ok {
		return result, err
	}

	switch l := left.(type) {
	case types.Integer:
		switch r := right.(type) {
		case types.Integer:
			return l.Add(r), nil
		case types.Decimal:
			return l.ToDecimal().Add(r), nil
		}
	case types.Decimal:
		switch r := right.(type) {
		case types.Integer:
			return l.Add(r.ToDecimal()), nil
		case types.Decimal:
			return l.Add(r), nil
		}
	case types.String:
		if r, ok := right.(types.String); ok {
			return types.NewString(l.Value() + r.Value()), nil
		}
	case types.Quantity:
		if r, ok := right.(types.Quantity); ok {
			// Quantity + Quantity
			return l.Add(r)
		}
	}
	return nil, InvalidOperationError("+", left.Type(), right.Type())
}

// Subtract performs subtraction on two values.
func Subtract(left, right types.Value) (types.Value, error) {
	// FHIR quantity objects: route through Quantity arithmetic.
	if lq, rq, ok := quantityOperands(left, right); ok {
		return lq.Subtract(rq)
	}

	// Date and DateTime take a duration on the right
	if result, ok, err := shiftTemporal(left, right, true); ok {
		return result, err
	}

	switch l := left.(type) {
	case types.Integer:
		switch r := right.(type) {
		case types.Integer:
			return l.Subtract(r), nil
		case types.Decimal:
			return l.ToDecimal().Subtract(r), nil
		}
	case types.Decimal:
		switch r := right.(type) {
		case types.Integer:
			return l.Subtract(r.ToDecimal()), nil
		case types.Decimal:
			return l.Subtract(r), nil
		}
	case types.Quantity:
		if r, ok := right.(types.Quantity); ok {
			// Quantity - Quantity
			return l.Subtract(r)
		}
	}
	return nil, InvalidOperationError("-", left.Type(), right.Type())
}

// Multiply performs multiplication on two values.
func Multiply(left, right types.Value) (types.Value, error) {
	// Quantity * Quantity combines the units: 2 'cm' by 2 'm' is 0.04 'm2'
	if lq, rq, ok := bothQuantities(left, right); ok {
		return lq.MultiplyQuantity(rq)
	}

	// Quantity * number scales the value and keeps the unit
	if q, factor, ok := quantityAndNumber(left, right); ok {
		return q.Multiply(factor), nil
	}

	switch l := left.(type) {
	case types.Integer:
		switch r := right.(type) {
		case types.Integer:
			return l.Multiply(r), nil
		case types.Decimal:
			return l.ToDecimal().Multiply(r), nil
		}
	case types.Decimal:
		switch r := right.(type) {
		case types.Integer:
			return l.Multiply(r.ToDecimal()), nil
		case types.Decimal:
			return l.Multiply(r), nil
		}
	}
	return nil, InvalidOperationError("*", left.Type(), right.Type())
}

// Divide performs division on two values.
func Divide(left, right types.Value) (types.Value, error) {
	// Quantity / Quantity combines the units: 4 'g' by 2 'm' is 2 'g.m-1'
	if lq, rq, ok := bothQuantities(left, right); ok {
		return lq.DivideQuantity(rq)
	}

	// Quantity / number scales the value and keeps the unit
	if lq, converted := asQuantity(left); converted {
		if n, isNumeric := right.(types.Numeric); isNumeric {
			return lq.Divide(n.ToDecimal().Value())
		}
	}

	// Convert both to Decimal for division
	var lDec, rDec types.Decimal
	switch l := left.(type) {
	case types.Integer:
		lDec = l.ToDecimal()
	case types.Decimal:
		lDec = l
	default:
		return nil, InvalidOperationError("/", left.Type(), right.Type())
	}

	switch r := right.(type) {
	case types.Integer:
		rDec = r.ToDecimal()
	case types.Decimal:
		rDec = r
	default:
		return nil, InvalidOperationError("/", left.Type(), right.Type())
	}

	if rDec.Value().IsZero() {
		return nil, ErrDivideByZero
	}
	return lDec.Divide(rDec)
}

// ErrDivideByZero reports a division whose divisor is zero.
//
// The specification does not treat this as an error: "12 / 0 // empty ({ })",
// and likewise for div and mod. Callers translate this sentinel into an empty
// collection, the same way an incommensurable unit comparison is translated.
var ErrDivideByZero = errors.New("division by zero")

// IntegerDivide performs truncated division (div operator).
//
// "Performs truncated division of the left operand by the right operand
// (supported for Integer, Long and Decimal) ... The resulting datatype is the
// same as the input datatype", so 5 div 2 is an Integer and 5.5 div 0.7 a
// Decimal.
func IntegerDivide(left, right types.Value) (types.Value, error) {
	if l, ok := left.(types.Integer); ok {
		if r, ok := right.(types.Integer); ok {
			if r.Value() == 0 {
				return nil, ErrDivideByZero
			}
			return l.Div(r)
		}
	}

	l, r, ok := decimalOperands(left, right)
	if !ok {
		return nil, InvalidOperationError("div", left.Type(), right.Type())
	}
	if r.Value().IsZero() {
		return nil, ErrDivideByZero
	}

	quotient, err := l.Divide(r)
	if err != nil {
		return nil, err
	}
	return types.NewDecimalFromInt(quotient.Truncate().Value()), nil
}

// Modulo computes the remainder of the truncated division, over the same types
// div accepts.
func Modulo(left, right types.Value) (types.Value, error) {
	if l, ok := left.(types.Integer); ok {
		if r, ok := right.(types.Integer); ok {
			if r.Value() == 0 {
				return nil, ErrDivideByZero
			}
			return l.Mod(r)
		}
	}

	l, r, ok := decimalOperands(left, right)
	if !ok {
		return nil, InvalidOperationError("mod", left.Type(), right.Type())
	}
	if r.Value().IsZero() {
		return nil, ErrDivideByZero
	}

	// The remainder of the truncated division: l - trunc(l/r)*r
	quotient, err := l.Divide(r)
	if err != nil {
		return nil, err
	}
	whole := types.NewDecimalFromInt(quotient.Truncate().Value())
	return l.Subtract(whole.Multiply(r)), nil
}

// decimalOperands widens two numeric values to Decimal, which is the implicit
// conversion the arithmetic operators are defined over.
func decimalOperands(left, right types.Value) (l, r types.Decimal, ok bool) {
	toDecimal := func(v types.Value) (types.Decimal, bool) {
		switch n := v.(type) {
		case types.Decimal:
			return n, true
		case types.Integer:
			return n.ToDecimal(), true
		}
		return types.Decimal{}, false
	}

	l, lok := toDecimal(left)
	r, rok := toDecimal(right)
	return l, r, lok && rok
}

// Negate negates a numeric value.
func Negate(value types.Value) (types.Value, error) {
	switch v := value.(type) {
	case types.Integer:
		return v.Negate(), nil
	case types.Decimal:
		return v.Negate(), nil
	case types.Quantity:
		// A quantity negates by its value, keeping its unit: -5.5 'mg'
		return v.Negate(), nil
	}
	return nil, NewEvalError(ErrType, "cannot negate %s", value.Type())
}

// Comparison operators

// Compare compares two values and returns -1, 0, or 1.
func Compare(left, right types.Value) (int, error) {
	// FHIR quantity objects on either side (or both, as in Range.low <=
	// Range.high) compare as quantities.
	if lq, rq, ok := quantityOperands(left, right); ok {
		return lq.Compare(rq)
	}

	if comp, ok := left.(types.Comparable); ok {
		return comp.Compare(right)
	}
	return 0, InvalidOperationError("compare", left.Type(), right.Type())
}

// compareWith compares two values and reports whether the ordering satisfies
// accept. Quantities whose units are not commensurable yield empty, per the
// FHIRPath spec: "attempting to operate on quantities with invalid units will
// result in empty ({})".
func compareWith(left, right types.Value, accept func(cmp int) bool) (types.Collection, error) {
	cmp, err := Compare(left, right)
	if err != nil {
		// Both cases are "the answer is unknown", which the spec expresses as
		// empty: quantities whose units are not commensurable, and temporals
		// specified to different precisions.
		if errors.Is(err, types.ErrIncompatibleUnits) || errors.Is(err, types.ErrPrecisionMismatch) {
			return types.EmptyCollection, nil
		}
		return nil, err
	}
	if accept(cmp) {
		return types.TrueCollection, nil
	}
	return types.FalseCollection, nil
}

// LessThan returns true if left < right.
func LessThan(left, right types.Value) (types.Collection, error) {
	return compareWith(left, right, func(cmp int) bool { return cmp < 0 })
}

// LessOrEqual returns true if left <= right.
func LessOrEqual(left, right types.Value) (types.Collection, error) {
	return compareWith(left, right, func(cmp int) bool { return cmp <= 0 })
}

// GreaterThan returns true if left > right.
func GreaterThan(left, right types.Value) (types.Collection, error) {
	return compareWith(left, right, func(cmp int) bool { return cmp > 0 })
}

// GreaterOrEqual returns true if left >= right.
func GreaterOrEqual(left, right types.Value) (types.Collection, error) {
	return compareWith(left, right, func(cmp int) bool { return cmp >= 0 })
}

// Equality operators

// literalQuantityOperands converts a FHIR quantity object compared against a
// Quantity literal, so that `Observation.valueQuantity = 10 'mg'` compares as
// quantities rather than object-against-primitive.
//
// Two quantity objects are deliberately not converted here: comparing complex
// types with = and ~ compares their children, which is a separate concern from
// quantity ordering.
func literalQuantityOperands(left, right types.Value) (lq, rq types.Quantity, ok bool) {
	leftLiteral, leftIsLiteral := left.(types.Quantity)
	rightLiteral, rightIsLiteral := right.(types.Quantity)

	switch {
	case leftIsLiteral && !rightIsLiteral:
		if q, converted := asQuantity(right); converted {
			return leftLiteral, q, true
		}
	case rightIsLiteral && !leftIsLiteral:
		if q, converted := asQuantity(left); converted {
			return q, rightLiteral, true
		}
	}
	return types.Quantity{}, types.Quantity{}, false
}

// Equal returns true if left = right.
//
// Per the spec, an empty operand yields empty; collections of the same length
// are compared item by item in order; and collections of different lengths are
// not equal — which is false, not empty.
func Equal(left, right types.Collection) types.Collection {
	// Empty propagation
	if left.Empty() || right.Empty() {
		return types.EmptyCollection
	}

	if len(left) != len(right) {
		return types.FalseCollection
	}

	for i := range left {
		// Temporal equality can be unknown rather than false when the two values
		// are specified to different precisions, in which case so is the result.
		if types.IsTemporal(left[i]) && types.IsTemporal(right[i]) {
			equal, err := types.EqualTemporal(left[i], right[i])
			switch {
			case errors.Is(err, types.ErrPrecisionMismatch):
				return types.EmptyCollection
			case err != nil:
				return types.FalseCollection
			case !equal:
				return types.FalseCollection
			}
			continue
		}

		// Quantities whose units do not convert cannot be compared, which the
		// specification treats as unknown rather than unequal — the same
		// distinction it draws for temporals of different precision:
		//
		//	1 'cm' = 1 's'   // empty ; different dimensions
		//	1 year = 1 'a'   // empty ; a calendar year is not a UCUM year
		//	1 week = 1 'wk'  // true  ; these two are equal by definition
		if lq, rq, ok := comparableQuantities(left[i], right[i]); ok {
			if !lq.Comparable(rq) {
				return types.EmptyCollection
			}
			if !lq.Equal(rq) {
				return types.FalseCollection
			}
			continue
		}

		if !valuesEqual(left[i], right[i]) {
			return types.FalseCollection
		}
	}
	return types.TrueCollection
}

// comparableQuantities matches two values that are both quantities, whether
// written as literals or read from FHIR data.
//
// This is wider than literalQuantityOperands, which requires exactly one side to
// be a literal: equality has to reach the case where both are, since that is
// where a calendar keyword meets a UCUM code.
func comparableQuantities(left, right types.Value) (lq, rq types.Quantity, ok bool) {
	lq, lok := asQuantity(left)
	rq, rok := asQuantity(right)
	if !lok || !rok {
		return types.Quantity{}, types.Quantity{}, false
	}
	return lq, rq, true
}

// valuesEqual compares two single values for equality.
func valuesEqual(left, right types.Value) bool {
	if lq, rq, ok := literalQuantityOperands(left, right); ok {
		return lq.Equal(rq)
	}
	return left.Equal(right)
}

// NotEqual returns true if left != right.
func NotEqual(left, right types.Collection) types.Collection {
	result := Equal(left, right)
	if result.Empty() {
		return result
	}
	if result[0].(types.Boolean).Bool() {
		return types.FalseCollection
	}
	return types.TrueCollection
}

// Equivalent returns true if left ~ right.
//
// Unlike equality, equivalence never yields empty: two empty collections are
// equivalent, and a length mismatch is false. For collections of more than one
// item the comparison is not order dependent, so each item on the left must have
// a distinct equivalent partner on the right.
func Equivalent(left, right types.Collection) types.Collection {
	// For equivalence, empty collections are equivalent to each other
	if left.Empty() && right.Empty() {
		return types.TrueCollection
	}
	if left.Empty() || right.Empty() {
		return types.FalseCollection
	}

	if len(left) != len(right) {
		return types.FalseCollection
	}

	if len(left) == 1 {
		if valuesEquivalent(left[0], right[0]) {
			return types.TrueCollection
		}
		return types.FalseCollection
	}

	// Order-independent: pair each left item with an unused equivalent right one
	used := make([]bool, len(right))
	for _, item := range left {
		matched := false
		for j, candidate := range right {
			if used[j] || !valuesEquivalent(item, candidate) {
				continue
			}
			used[j] = true
			matched = true
			break
		}
		if !matched {
			return types.FalseCollection
		}
	}
	return types.TrueCollection
}

// valuesEquivalent compares two single values for equivalence.
func valuesEquivalent(left, right types.Value) bool {
	if lq, rq, ok := literalQuantityOperands(left, right); ok {
		return lq.Equivalent(rq)
	}
	return left.Equivalent(right)
}

// NotEquivalent returns true if left !~ right.
func NotEquivalent(left, right types.Collection) types.Collection {
	result := Equivalent(left, right)
	if result[0].(types.Boolean).Bool() {
		return types.FalseCollection
	}
	return types.TrueCollection
}

// Boolean operators (three-valued logic)
//
// Operands go through [types.Collection.SingletonBoolean], so a single
// non-Boolean node counts as true — the rule FHIR invariants such as age-1
// ("(code or value.empty()) and ...") depend on.

// And performs logical AND with three-valued logic.
func And(left, right types.Collection) types.Collection {
	lVal, lOk := left.SingletonBoolean()
	rVal, rOk := right.SingletonBoolean()

	// false and anything is false, even when the other side is empty
	if lOk && !lVal {
		return types.FalseCollection
	}
	if rOk && !rVal {
		return types.FalseCollection
	}

	if !lOk || !rOk {
		return types.EmptyCollection
	}
	return types.TrueCollection
}

// Or performs logical OR with three-valued logic.
func Or(left, right types.Collection) types.Collection {
	lVal, lOk := left.SingletonBoolean()
	rVal, rOk := right.SingletonBoolean()

	// true or anything is true, even when the other side is empty
	if lOk && lVal {
		return types.TrueCollection
	}
	if rOk && rVal {
		return types.TrueCollection
	}

	if !lOk || !rOk {
		return types.EmptyCollection
	}
	return types.FalseCollection
}

// Xor performs logical XOR.
func Xor(left, right types.Collection) types.Collection {
	lVal, lOk := left.SingletonBoolean()
	rVal, rOk := right.SingletonBoolean()
	if !lOk || !rOk {
		return types.EmptyCollection
	}

	if lVal != rVal {
		return types.TrueCollection
	}
	return types.FalseCollection
}

// Implies performs logical implication.
func Implies(left, right types.Collection) types.Collection {
	lVal, lOk := left.SingletonBoolean()
	rVal, rOk := right.SingletonBoolean()

	// false implies anything, and anything implies true
	if lOk && !lVal {
		return types.TrueCollection
	}
	if rOk && rVal {
		return types.TrueCollection
	}

	if !lOk || !rOk {
		return types.EmptyCollection
	}

	// left is true and right is false
	return types.FalseCollection
}

// Not performs logical NOT.
func Not(value types.Collection) types.Collection {
	val, ok := value.SingletonBoolean()
	if !ok {
		return types.EmptyCollection
	}
	if val {
		return types.FalseCollection
	}
	return types.TrueCollection
}

// String operators

// Concatenate performs string concatenation (& operator).
// Unlike +, & treats empty as empty string.
func Concatenate(left, right types.Collection) types.Collection {
	var lStr, rStr string

	if !left.Empty() {
		if s, ok := left[0].(types.String); ok {
			lStr = s.Value()
		}
	}

	if !right.Empty() {
		if s, ok := right[0].(types.String); ok {
			rStr = s.Value()
		}
	}

	return types.Collection{types.NewString(lStr + rStr)}
}

// Collection operators

// Union returns the union of two collections.
func Union(left, right types.Collection) types.Collection {
	return left.Union(right)
}

// In checks if left is in right collection.
func In(left, right types.Collection) types.Collection {
	if left.Empty() {
		return types.EmptyCollection
	}
	if len(left) != 1 {
		return types.EmptyCollection
	}
	if right.Contains(left[0]) {
		return types.TrueCollection
	}
	return types.FalseCollection
}

// Contains checks if left collection contains right.
func Contains(left, right types.Collection) types.Collection {
	if right.Empty() {
		return types.EmptyCollection
	}
	if len(right) != 1 {
		return types.EmptyCollection
	}
	if left.Contains(right[0]) {
		return types.TrueCollection
	}
	return types.FalseCollection
}
