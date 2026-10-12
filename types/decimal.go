package types

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/shopspring/decimal"
)

// Decimal represents a FHIRPath decimal value with arbitrary precision.
type Decimal struct {
	value    decimal.Decimal
	original string // original string representation for precision preservation (empty for computed values)
	fhirType string // FHIR type when the value was read through a model
	read     bool   // read from the input, so a FHIR primitive. See IsFHIRPrimitive.

	// The FHIR element this value was read with, when it carried one
	primitiveElement
}

// DecimalDigits bounds this engine's decimals: at most this many digits before
// the decimal point, and none further than this many places after it.
//
// FHIRPath sets a range every implementation must reach, (-10^28+1)/10^8 to
// (10^28-1)/10^8 with a step of 10^-8, and lets one go further; this engine
// goes far further, but not without end. A decimal is held exactly, so one
// written 1E8227822 is a number of eight million digits, and comparing or
// rounding it wrote them all out: half a second for each =, and more than
// five for round(). FHIR's own decimal allows an exponent of nine digits, so
// the bound is the engine's, and a value beyond it is an overflow: "Operations
// that cause arithmetic overflow or underflow will result in empty".
const DecimalDigits = 1000

// InDecimalRange reports a decimal within DecimalDigits on either side of the
// point.
//
// A zero is bounded too: 0E2000000000 is no larger than 0, but writing it out
// at its scale writes two billion zeroes.
func InDecimalRange(d decimal.Decimal) bool {
	exponent := int64(d.Exponent())
	if exponent < -DecimalDigits || exponent > DecimalDigits {
		return false
	}
	if d.IsZero() {
		return true
	}
	// A coefficient of b bits has at most b·log10(2) + 1 digits; well within the
	// bound, the count is not needed, and counting a large coefficient's
	// digits costs a power of ten
	coefficient := d.Coefficient()
	if bits := int64(coefficient.BitLen()); exponent+bits*30103/100000+1 <= DecimalDigits {
		return true
	}
	// Near the bound, the digits are counted exactly: NumDigits estimates a
	// small coefficient through a float logarithm, and counts 10^15 as 15
	digits := len(coefficient.Abs(coefficient).Text(10))
	return exponent+int64(digits) <= DecimalDigits
}

// normalizedZero returns a zero computed at a scale beyond the bound at the
// nearest scale within it: 0E1000 times 1E999 is zero, not an overflow.
func normalizedZero(d decimal.Decimal) decimal.Decimal {
	switch exponent := d.Exponent(); {
	case exponent > DecimalDigits:
		return decimal.Zero
	case exponent < -DecimalDigits:
		return decimal.New(0, -DecimalDigits)
	}
	return d
}

// InRange reports a value whose decimal, its own or a quantity's, is within
// DecimalDigits; any other value is in range.
func InRange(v Value) bool {
	switch value := v.(type) {
	case Decimal:
		return InDecimalRange(value.value)
	case Quantity:
		return InDecimalRange(value.value)
	}
	return true
}

// WithinRange returns a computed value as this engine holds it, and false
// where it overflows. A zero at a scale beyond the bound is no overflow: it is
// taken to the nearest scale within it.
func WithinRange(v Value) (Value, bool) {
	switch value := v.(type) {
	case Decimal:
		if value.value.IsZero() {
			value = Decimal{value: normalizedZero(value.value)}
		}
		return value, InDecimalRange(value.value)
	case Quantity:
		if value.value.IsZero() {
			value.value = normalizedZero(value.value)
		}
		return value, InDecimalRange(value.value)
	}
	return v, true
}

// ErrDecimalRange reports a decimal beyond DecimalDigits.
var ErrDecimalRange = errors.New("decimal beyond the range this engine holds")

// ParseDecimal reads a decimal, refusing one beyond DecimalDigits.
func ParseDecimal(s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Decimal{}, err
	}
	if !InDecimalRange(d) {
		return decimal.Decimal{}, fmt.Errorf("%w: %.40s", ErrDecimalRange, s)
	}
	return d, nil
}

// NewDecimal creates a new Decimal from a string, refusing one beyond
// DecimalDigits.
func NewDecimal(s string) (Decimal, error) {
	d, err := ParseDecimal(s)
	if err != nil {
		if errors.Is(err, ErrDecimalRange) {
			return Decimal{}, err
		}
		return Decimal{}, fmt.Errorf("invalid decimal: %s", s)
	}
	return Decimal{value: d, original: s}, nil
}

// NewDecimalFromInt creates a new Decimal from an int64.
func NewDecimalFromInt(v int64) Decimal {
	return Decimal{value: decimal.NewFromInt(v)}
}

// NewDecimalFromFloat creates a new Decimal from a float64.
func NewDecimalFromFloat(v float64) Decimal {
	return Decimal{value: decimal.NewFromFloat(v)}
}

// NewDecimalFromDecimal wraps an already-exact decimal value.
func NewDecimalFromDecimal(v decimal.Decimal) Decimal {
	return Decimal{value: v}
}

// MustDecimal creates a new Decimal, panicking on error.
func MustDecimal(s string) Decimal {
	d, err := NewDecimal(s)
	if err != nil {
		panic(err)
	}
	return d
}

// Value returns the underlying decimal.Decimal value.
func (d Decimal) Value() decimal.Decimal {
	return d.value
}

// Type returns "Decimal".
func (d Decimal) Type() string {
	if d.fhirType != "" {
		return d.fhirType
	}
	return TypeNameDecimal
}

// Equal returns true if other is numerically equal.
func (d Decimal) Equal(other Value) bool {
	switch o := other.(type) {
	case Decimal:
		return d.value.Equal(o.value)
	case Integer:
		return d.value.Equal(decimal.NewFromInt(o.value))
	}
	return false
}

// Equivalent compares two decimals at the precision of the less precise one.
//
// "Decimal: values must be equal, comparison is done on values rounded to the
// precision of the least precise operand. Trailing zeroes after the decimal are
// ignored in determining precision."
//
// This is what separates ~ from =: 1.2 / 1.8 is 0.666..., which equals nothing,
// but is equivalent to 0.67 because 0.67 is given to two places and the quotient
// rounded to two places is 0.67.
func (d Decimal) Equivalent(other Value) bool {
	var o Decimal
	switch v := other.(type) {
	case Decimal:
		o = v
	case Integer:
		o = v.ToDecimal()
	default:
		return false
	}

	places := d.significantPlaces()
	if theirs := o.significantPlaces(); theirs < places {
		places = theirs
	}

	return d.value.Round(places).Equal(o.value.Round(places))
}

// significantPlaces counts the digits after the decimal point that carry
// precision, which excludes trailing zeroes: 1.10 is given to one place, not
// two.
func (d Decimal) significantPlaces() int32 {
	return significantPlaces(d.value)
}

// String returns the decimal string representation. For values created from
// string parsing, preserves the original representation (e.g., "1.0" stays "1.0").
// For computed values, uses the default shopspring representation.
func (d Decimal) String() string {
	if d.original != "" {
		return d.original
	}
	return d.value.String()
}

// ImplicitPrecision returns the number of decimal places in this value,
// inferred from the original string representation. For example, "1.0" has
// precision 1, "1.00" has precision 2, and "42" has precision 0.
// Returns 0 for computed values with no original string.
func (d Decimal) ImplicitPrecision() int {
	if d.original != "" {
		if idx := strings.Index(d.original, "."); idx >= 0 {
			return len(d.original) - idx - 1
		}
		return 0
	}

	// A computed value has no original text, but its scale still records how
	// many fractional digits it carries. Without this, negating a literal —
	// which produces a computed value — would report a precision of zero, and
	// (-1.587).lowBoundary() would bound it by 0.5 instead of 0.0005.
	if exponent := d.value.Exponent(); exponent < 0 {
		return int(-exponent)
	}
	return 0
}

// IsEmpty returns false for decimal values.
func (d Decimal) IsEmpty() bool {
	return false
}

// ToDecimal returns itself (implements Numeric interface).
func (d Decimal) ToDecimal() Decimal {
	return d
}

// Compare compares two numeric values.
func (d Decimal) Compare(other Value) (int, error) {
	switch o := other.(type) {
	case Decimal:
		return d.value.Cmp(o.value), nil
	case Integer:
		return d.value.Cmp(decimal.NewFromInt(o.value)), nil
	}
	return 0, NewTypeError(TypeNameDecimal, other.Type(), "comparison")
}

// Add returns the sum of two decimals.
func (d Decimal) Add(other Decimal) Decimal {
	return Decimal{value: d.value.Add(other.value)}
}

// Subtract returns the difference of two decimals.
func (d Decimal) Subtract(other Decimal) Decimal {
	return Decimal{value: d.value.Sub(other.value)}
}

// Multiply returns the product of two decimals.
func (d Decimal) Multiply(other Decimal) Decimal {
	return Decimal{value: d.value.Mul(other.value)}
}

// Divide returns the result of division.
func (d Decimal) Divide(other Decimal) (Decimal, error) {
	if other.value.IsZero() {
		return Decimal{}, fmt.Errorf("division by zero")
	}
	// Use 16 decimal places of precision
	return Decimal{value: d.value.DivRound(other.value, 16)}, nil
}

// Negate returns the negation of the decimal.
func (d Decimal) Negate() Decimal {
	negated := Decimal{value: d.value.Neg()}

	// Carry the original representation across, sign flipped, so that negation
	// does not discard precision: -(120.50) must still present as -120.50.
	if d.original != "" {
		negated.original = strings.TrimPrefix(d.original, "-")
		if !strings.HasPrefix(d.original, "-") {
			negated.original = "-" + negated.original
		}
	}
	return negated
}

// Abs returns the absolute value.
func (d Decimal) Abs() Decimal {
	return Decimal{value: d.value.Abs()}
}

// Ceiling returns the smallest integer >= d.
//
// It wraps past the int64 range; IntegerFromDecimal reports that as
// ErrIntegerRange.
func (d Decimal) Ceiling() Integer {
	return NewInteger(d.value.Ceil().IntPart())
}

// Floor returns the largest integer <= d.
//
// It wraps past the int64 range; IntegerFromDecimal reports that as
// ErrIntegerRange.
func (d Decimal) Floor() Integer {
	return NewInteger(d.value.Floor().IntPart())
}

// Truncate returns the integer part.
//
// It wraps past the int64 range; IntegerFromDecimal reports that as
// ErrIntegerRange.
func (d Decimal) Truncate() Integer {
	return NewInteger(d.value.Truncate(0).IntPart())
}

// Round rounds to the given precision.
func (d Decimal) Round(precision int32) Decimal {
	return Decimal{value: d.value.Round(precision)}
}

// Power returns d raised to the given power.
func (d Decimal) Power(exp Decimal) Decimal {
	// Convert to float64 for power operation
	base, _ := d.value.Float64()
	exponent, _ := exp.value.Float64()
	result := math.Pow(base, exponent)
	return NewDecimalFromFloat(result)
}

// Sqrt returns the square root.
func (d Decimal) Sqrt() (Decimal, error) {
	if d.value.IsNegative() {
		return Decimal{}, fmt.Errorf("cannot take square root of negative number")
	}
	f, _ := d.value.Float64()
	return NewDecimalFromFloat(math.Sqrt(f)), nil
}

// Exp returns e^d.
func (d Decimal) Exp() Decimal {
	f, _ := d.value.Float64()
	return NewDecimalFromFloat(math.Exp(f))
}

// Ln returns the natural logarithm.
func (d Decimal) Ln() (Decimal, error) {
	if !d.value.IsPositive() {
		return Decimal{}, fmt.Errorf("cannot take logarithm of non-positive number")
	}
	f, _ := d.value.Float64()
	return NewDecimalFromFloat(math.Log(f)), nil
}

// Log returns the logarithm with the given base.
func (d Decimal) Log(base Decimal) (Decimal, error) {
	if !d.value.IsPositive() {
		return Decimal{}, fmt.Errorf("cannot take logarithm of non-positive number")
	}
	if !base.value.IsPositive() || base.value.Equal(decimal.NewFromInt(1)) {
		return Decimal{}, fmt.Errorf("invalid logarithm base")
	}
	f, _ := d.value.Float64()
	b, _ := base.value.Float64()
	return NewDecimalFromFloat(math.Log(f) / math.Log(b)), nil
}

// IsInteger returns true if the decimal has no fractional part.
func (d Decimal) IsInteger() bool {
	return d.value.Equal(d.value.Truncate(0))
}

// ToInteger converts to Integer if it's a whole number.
func (d Decimal) ToInteger() (Integer, bool) {
	if !d.IsInteger() {
		return Integer{}, false
	}
	// A whole decimal past the int64 range is no Integer
	i, err := IntegerFromDecimal(d.value)
	return i, err == nil
}

// WithFHIRType returns a copy that reports the FHIR type it was declared with.
// FHIR primitives are types in their own right — a FHIR.boolean is not a
// System.Boolean — so a value keeps the name the model gave it.
func (d Decimal) WithFHIRType(fhirType string) Decimal {
	d.fhirType = fhirType
	return d
}

// WithElement returns a copy carrying the FHIR element that accompanied the
// value in the JSON, which is where its extensions and id live.
func (d Decimal) WithElement(element *ObjectValue) Decimal {
	d.element = element
	return d
}
