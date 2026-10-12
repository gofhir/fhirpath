package types

import (
	"errors"
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

// Integer represents a FHIRPath integer value.
type Integer struct {
	value    int64
	fhirType string // FHIR type when the value was read through a model
	read     bool   // read from the input, so a FHIR primitive. See IsFHIRPrimitive.

	// The FHIR element this value was read with, when it carried one
	primitiveElement
}

// NewInteger creates a new Integer value.
func NewInteger(v int64) Integer {
	return Integer{value: v}
}

// Value returns the underlying int64 value.
func (i Integer) Value() int64 {
	return i.value
}

// Type returns "Integer".
func (i Integer) Type() string {
	if i.fhirType != "" {
		return i.fhirType
	}
	return TypeNameInteger
}

// Equal returns true if other is an Integer with the same value,
// or a Decimal with an equivalent integer value.
func (i Integer) Equal(other Value) bool {
	switch o := other.(type) {
	case Integer:
		return i.value == o.value
	case Decimal:
		// Compare as decimals for cross-type equality
		return i.ToDecimal().Equal(o)
	}
	return false
}

// Equivalent is the same as Equal between integers.
//
// Against a decimal it is the decimal's equivalence, at the precision of the
// less precise side, so that 1 ~ 1.4 answers as 1.4 ~ 1 does.
func (i Integer) Equivalent(other Value) bool {
	if d, ok := other.(Decimal); ok {
		return d.Equivalent(i)
	}
	return i.Equal(other)
}

// String returns the decimal string representation.
func (i Integer) String() string {
	return fmt.Sprintf("%d", i.value)
}

// IsEmpty returns false for integer values.
func (i Integer) IsEmpty() bool {
	return false
}

// ToDecimal converts the integer to a Decimal.
func (i Integer) ToDecimal() Decimal {
	return Decimal{value: decimal.NewFromInt(i.value)}
}

// Compare compares two numeric values.
func (i Integer) Compare(other Value) (int, error) {
	switch o := other.(type) {
	case Integer:
		if i.value < o.value {
			return -1, nil
		}
		if i.value > o.value {
			return 1, nil
		}
		return 0, nil
	case Decimal:
		return i.ToDecimal().Compare(o)
	}
	return 0, NewTypeError("Integer", other.Type(), "comparison")
}

// ErrIntegerRange reports an Integer result beyond the 64 bits this engine
// holds an Integer in. FHIRPath: "Operations that cause arithmetic overflow or
// underflow will result in empty".
var ErrIntegerRange = errors.New("integer beyond the range this engine holds")

// Add returns the sum of two integers, wrapping past the int64 range.
//
// Deprecated: use AddExact, which reports a sum past the range.
func (i Integer) Add(other Integer) Integer {
	return NewInteger(i.value + other.value)
}

// AddExact returns the sum of two integers, or ErrIntegerRange where it lies
// past the int64 range.
func (i Integer) AddExact(other Integer) (Integer, error) {
	sum := i.value + other.value
	if (other.value > 0 && sum < i.value) || (other.value < 0 && sum > i.value) {
		return Integer{}, ErrIntegerRange
	}
	return NewInteger(sum), nil
}

// Subtract returns the difference of two integers, wrapping past the int64
// range.
//
// Deprecated: use SubtractExact, which reports a difference past the range.
func (i Integer) Subtract(other Integer) Integer {
	return NewInteger(i.value - other.value)
}

// SubtractExact returns the difference of two integers, or ErrIntegerRange
// where it lies past the int64 range.
func (i Integer) SubtractExact(other Integer) (Integer, error) {
	difference := i.value - other.value
	if (other.value > 0 && difference > i.value) || (other.value < 0 && difference < i.value) {
		return Integer{}, ErrIntegerRange
	}
	return NewInteger(difference), nil
}

// Multiply returns the product of two integers, wrapping past the int64
// range.
//
// Deprecated: use MultiplyExact, which reports a product past the range.
func (i Integer) Multiply(other Integer) Integer {
	return NewInteger(i.value * other.value)
}

// MultiplyExact returns the product of two integers, or ErrIntegerRange where
// it lies past the int64 range.
func (i Integer) MultiplyExact(other Integer) (Integer, error) {
	a, b := i.value, other.value
	if a == 0 || b == 0 {
		return NewInteger(0), nil
	}
	product := a * b
	// product/b catches every wrap but the least int64 times -1, whose
	// quotient wraps back to it
	if product/b != a || (b == -1 && a == math.MinInt64) {
		return Integer{}, ErrIntegerRange
	}
	return NewInteger(product), nil
}

// Divide returns the result of division as a Decimal.
func (i Integer) Divide(other Integer) (Decimal, error) {
	if other.value == 0 {
		return Decimal{}, fmt.Errorf("division by zero")
	}
	return i.ToDecimal().Divide(other.ToDecimal())
}

// Div returns the integer division result, or ErrIntegerRange for the one
// quotient past the int64 range, the least int64 divided by -1.
func (i Integer) Div(other Integer) (Integer, error) {
	if other.value == 0 {
		return Integer{}, fmt.Errorf("division by zero")
	}
	if i.value == math.MinInt64 && other.value == -1 {
		return Integer{}, ErrIntegerRange
	}
	return NewInteger(i.value / other.value), nil
}

// Mod returns the modulo result.
func (i Integer) Mod(other Integer) (Integer, error) {
	if other.value == 0 {
		return Integer{}, fmt.Errorf("division by zero")
	}
	return NewInteger(i.value % other.value), nil
}

// Negate returns the negation of the integer, wrapping past the int64 range.
//
// Deprecated: use NegateExact, which reports the one negation past the range.
func (i Integer) Negate() Integer {
	return NewInteger(-i.value)
}

// NegateExact returns the negation of the integer, or ErrIntegerRange for the
// least int64, whose negation lies past the range.
func (i Integer) NegateExact() (Integer, error) {
	if i.value == math.MinInt64 {
		return Integer{}, ErrIntegerRange
	}
	return NewInteger(-i.value), nil
}

// Abs returns the absolute value, wrapping past the int64 range.
//
// Deprecated: use AbsExact, which reports the one absolute value past the
// range.
func (i Integer) Abs() Integer {
	if i.value < 0 {
		return NewInteger(-i.value)
	}
	return i
}

// AbsExact returns the absolute value, or ErrIntegerRange for the least
// int64, whose absolute value lies past the range.
func (i Integer) AbsExact() (Integer, error) {
	if i.value < 0 {
		return i.NegateExact()
	}
	return i, nil
}

// The ends of the int64 range as decimals, made once.
var (
	minInt64Decimal = decimal.NewFromInt(math.MinInt64)
	maxInt64Decimal = decimal.NewFromInt(math.MaxInt64)
)

// IntegerFromDecimal reads a whole decimal as an Integer, or reports
// ErrIntegerRange where it lies past the int64 range. A fraction is dropped,
// toward zero, as decimal.IntPart drops it.
func IntegerFromDecimal(d decimal.Decimal) (Integer, error) {
	whole := d.Truncate(0)
	if whole.LessThan(minInt64Decimal) || whole.GreaterThan(maxInt64Decimal) {
		return Integer{}, ErrIntegerRange
	}
	return NewInteger(whole.IntPart()), nil
}

// Power returns the integer raised to the given power.
func (i Integer) Power(exp Integer) Decimal {
	return i.ToDecimal().Power(exp.ToDecimal())
}

// Sqrt returns the square root as a Decimal.
func (i Integer) Sqrt() (Decimal, error) {
	if i.value < 0 {
		return Decimal{}, fmt.Errorf("cannot take square root of negative number")
	}
	return NewDecimalFromFloat(math.Sqrt(float64(i.value))), nil
}

// WithFHIRType returns a copy that reports the FHIR type it was declared with.
// FHIR primitives are types in their own right — a FHIR.boolean is not a
// System.Boolean — so a value keeps the name the model gave it.
func (i Integer) WithFHIRType(fhirType string) Integer {
	i.fhirType = fhirType
	return i
}

// WithElement returns a copy carrying the FHIR element that accompanied the
// value in the JSON, which is where its extensions and id live.
func (i Integer) WithElement(element *ObjectValue) Integer {
	i.element = element
	return i
}
