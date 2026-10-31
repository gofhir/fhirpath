package types

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/gofhir/fhirpath/internal/decimals"
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

// DivisionPlaces is the places a quotient that does not end is rounded to.
// The engine passes it to every division itself: shopspring's Div reads a
// process-wide setting a program may change (#237).
const DivisionPlaces = 16

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
	bits := int64(coefficient.BitLen())
	if exponent+bits*30103/100000+1 <= DecimalDigits {
		return true
	}
	// Nor where it is far beyond: such a coefficient has at least
	// (b-1)·log10(2) + 1 digits, taken a little short so as never to overcount
	if exponent+(bits-1)*30102/100000+1 > DecimalDigits {
		return false
	}
	// Near the bound, the digits are counted exactly: NumDigits estimates a
	// small coefficient through a float logarithm, and counts 10^15 as 15
	digits := len(coefficient.Abs(coefficient).Text(10))
	return exponent+int64(digits) <= DecimalDigits
}

// CompareDecimals orders two decimals exactly, as decimal.Decimal.Cmp does, a
// decimal beyond DecimalDigits among them. Comparing rescales one to the
// other's exponent, which costs the digits between them: nothing for two
// decimals within the bound, millions for one beyond it read from a resource,
// 1e5000000 against 1 (#232). So where the exponents lie far apart the sign
// and the place of the leading digit decide first, and only decimals that tie
// on both, whose exponents then lie within their own digits of each other, are
// rescaled.
func CompareDecimals(a, b decimal.Decimal) int {
	if gap := int64(a.Exponent()) - int64(b.Exponent()); gap >= -2*DecimalDigits && gap <= 2*DecimalDigits {
		return a.Cmp(b)
	}
	sign := a.Sign()
	if other := b.Sign(); sign != other {
		return cmp.Compare(sign, other)
	}
	if sign == 0 {
		return 0
	}
	if ma, mb := leadingPlace(a), leadingPlace(b); ma != mb {
		return sign * cmp.Compare(ma, mb)
	}
	return a.Cmp(b)
}

// leadingPlace is the power of ten of a nonzero decimal's leading digit: 2 for
// 123, -3 for 0.00123. It counts the coefficient's digits, as written.
func leadingPlace(d decimal.Decimal) int64 {
	digits := new(big.Int).Abs(d.Coefficient()).Text(10)
	return int64(d.Exponent()) + int64(len(digits)) - 1
}

// roundTo rounds a decimal to places after the point as Round does, without
// writing out one that has no more places already: Round first rescales to
// one place past the one asked for, which for 1e5000000 is five million
// digits. A decimal smaller than a tenth of the last place rounds to zero
// without dividing by its scale.
func roundTo(d decimal.Decimal, places int32) decimal.Decimal {
	if d.Exponent() >= -places {
		return d
	}
	if d.IsZero() || leadingPlace(d) < -int64(places)-1 {
		return decimal.Decimal{}
	}
	return d.Round(places)
}

// normalizedZero returns a zero computed at a scale beyond the bound at the
// nearest scale within it: 0E1000 times 1E999 is zero, not an overflow.
func normalizedZero(d decimal.Decimal) decimal.Decimal {
	switch exponent := d.Exponent(); {
	case exponent > DecimalDigits:
		return decimal.Decimal{}
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

// decodeDecimal reads a decimal from text as shopspring/decimal v1.4.0's
// NewFromString read it: an optional sign, digits with a point that has
// digits on one side at least, and an exponent, all ASCII. The library's
// later versions refuse an exponent past MaxDecodeExponent, a process-wide
// setting, and read a Unicode minus; this engine's bound is DecimalDigits,
// checked by its callers, so the value is built from its digits and written
// exponent, and the exponent's own cost is never paid here (#235). A sign
// after the point, which v1.4.0 read (".-5" as -0.05), is refused. The
// exponent must fit in the int32 the library holds it in (FP-6).
func decodeDecimal(s string) (decimal.Decimal, error) {
	mantissa, exponentText := s, ""
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mantissa, exponentText = s[:i], s[i+1:]
		if !isExponent(exponentText) {
			return decimal.Decimal{}, fmt.Errorf("invalid decimal: %.40s", s)
		}
	}
	sign := ""
	if mantissa != "" && (mantissa[0] == '-' || mantissa[0] == '+') {
		sign, mantissa = mantissa[:1], mantissa[1:]
	}
	whole, fraction, _ := strings.Cut(mantissa, ".")
	if !isDigits(whole) || !isDigits(fraction) {
		return decimal.Decimal{}, fmt.Errorf("invalid decimal: %.40s", s)
	}
	var exponent int64
	if exponentText != "" {
		written, err := strconv.ParseInt(exponentText, 10, 32)
		if err != nil {
			return decimal.Decimal{}, fmt.Errorf("invalid decimal: exponent out of range: %.40s", s)
		}
		exponent = written
	}
	exponent -= int64(len(fraction))
	if exponent < math.MinInt32 {
		return decimal.Decimal{}, fmt.Errorf("invalid decimal: exponent out of range: %.40s", s)
	}
	d, ok := fromDigits(sign, whole, fraction, int32(exponent))
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("invalid decimal: %.40s", s)
	}
	return d, nil
}

// fromDigits is the decimal of a sign, ASCII digits before and after the
// point, and an exponent; false where there are no digits.
func fromDigits(sign, whole, fraction string, exponent int32) (decimal.Decimal, bool) {
	digits := len(whole) + len(fraction)
	if digits == 0 {
		return decimal.Decimal{}, false
	}
	// Eighteen digits fit an int64, which costs no big.Int
	if digits <= 18 {
		var n int64
		for _, part := range [2]string{whole, fraction} {
			for i := range len(part) {
				n = n*10 + int64(part[i]-'0')
			}
		}
		if sign == "-" {
			n = -n
		}
		return decimal.New(n, exponent), true
	}
	coefficient, ok := new(big.Int).SetString(sign+whole+fraction, 10)
	if !ok {
		return decimal.Decimal{}, false
	}
	return decimal.NewFromBigInt(coefficient, exponent), true
}

// isDigits reports text of ASCII digits alone, or none.
func isDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isExponent reports an exponent's text: an optional sign and ASCII digits.
func isExponent(s string) bool {
	if s != "" && (s[0] == '-' || s[0] == '+') {
		s = s[1:]
	}
	return s != "" && isDigits(s)
}

// ParseDecimal reads a decimal, refusing one beyond DecimalDigits.
func ParseDecimal(s string) (decimal.Decimal, error) {
	d, err := decodeDecimal(s)
	if err != nil {
		return decimal.Decimal{}, err
	}
	if !InDecimalRange(d) {
		return decimal.Decimal{}, fmt.Errorf("%w: %.40s", ErrDecimalRange, s)
	}
	return d, nil
}

// ReadDecimal reads a decimal as a resource or a string writes it, one beyond
// DecimalDigits too: FHIR's decimal has no such bound, and a value read is no
// operation that overflows (#232). Such a value is held as written and
// compared exactly, but nothing computes with it: see InRange. A zero at a
// scale beyond the bound is taken to the nearest scale within it, keeping its
// text, so 0e2000 + 1 is 1, and so is a value whose places past the bound are
// all zeros: 1 written with 1001 zeros after the point is 1.
func ReadDecimal(s string) (Decimal, error) {
	d, err := decodeDecimal(s)
	if err != nil {
		return Decimal{}, fmt.Errorf("invalid decimal: %s", s)
	}
	if d.IsZero() {
		d = normalizedZero(d)
	} else {
		d = withinScale(d, s)
	}
	return Decimal{value: d, original: s}, nil
}

// withinScale takes a decimal written with more places after the point than
// DecimalDigits, those past the bound all zeros, to the scale of the bound.
// The zeros are counted on the written digits, which are the coefficient's,
// so that nothing is divided to find them, and the value is read again from
// those digits without them.
func withinScale(d decimal.Decimal, s string) decimal.Decimal {
	exponent := int64(d.Exponent())
	if exponent >= -DecimalDigits {
		return d
	}
	digits := s
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		digits = s[:i]
	}
	if !strings.HasSuffix(strings.TrimSuffix(digits, "."), "0") {
		return d
	}
	digits = strings.Replace(digits, ".", "", 1)
	excess := -DecimalDigits - exponent
	if int64(len(digits)-len(strings.TrimRight(digits, "0"))) < excess {
		return d
	}
	within, err := decodeDecimal(digits[:int64(len(digits))-excess] + "e" + strconv.FormatInt(exponent+excess, 10))
	if err != nil {
		return d
	}
	return within
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
	return Decimal{value: decimals.Computed(decimal.NewFromFloat(v))}
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
//
// A decimal read from a resource may lie beyond DecimalDigits, 1e5000000 held
// as a coefficient and an exponent; computing with it, or writing it out with
// String, writes out every digit. Check InDecimalRange before computing.
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
		return CompareDecimals(d.value, o.value) == 0
	case Integer:
		return CompareDecimals(d.value, decimal.NewFromInt(o.value)) == 0
	case Quantity:
		// As a quantity of unit '1': see UnitOne
		return o.Equal(d)
	case *ObjectValue:
		// A FHIR Quantity: see ObjectValue.Equal
		return o.Equal(d)
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
	case Quantity:
		return v.Equivalent(d)
	default:
		return false
	}

	places := d.significantPlaces()
	if theirs := o.significantPlaces(); theirs < places {
		places = theirs
	}

	return CompareDecimals(roundTo(d.value, places), roundTo(o.value, places)) == 0
}

// significantPlaces counts the digits after the decimal point that carry
// precision, which excludes trailing zeroes: 1.10 is given to one place, not
// two.
func (d Decimal) significantPlaces() int32 {
	return significantPlaces(d.value)
}

// String returns the decimal string representation. For values created from
// string parsing, preserves the original representation (e.g., "1.0" stays "1.0").
// For computed values, writes no exponent and drops the fraction's trailing
// zeros, whatever a program sets in the decimal library (#237).
func (d Decimal) String() string {
	if d.original != "" {
		return d.original
	}
	return decimals.Format(d.value)
}

// ImplicitPrecision returns the number of decimal places in this value, its
// exponent applied and never below 0: "1.0" has precision 1, "1.00" 2, "42"
// 0, "1.5e2" 0 and "1E-17" 17. It is LastPlace, clamped.
func (d Decimal) ImplicitPrecision() int {
	places := d.LastPlace()
	if places <= 0 {
		return 0
	}
	return int(min(places, math.MaxInt))
}

// LastPlace returns the place of the last digit written, counted after the
// point: 2 for 1.50, 0 for 150, -1 for 1.5e2, 17 for 1E-17 (#246). A value
// read from text answers from that text, the mantissa's places less the
// exponent, even past the bound, where the decoded scale is capped; the
// decoder refuses an exponent past int32, so this cannot overflow.
//
// A computed value has no text, but its scale still records how many
// fractional digits it carries: (1.50 * 2).precision() is 2, as the HL7
// validator answers.
func (d Decimal) LastPlace() int64 {
	if d.original == "" {
		return -int64(d.value.Exponent())
	}
	mantissa, exponent := d.original, int64(0)
	if i := strings.IndexAny(d.original, "eE"); i >= 0 {
		mantissa = d.original[:i]
		// The decoder has read it already, within int32
		if e, err := strconv.ParseInt(d.original[i+1:], 10, 64); err == nil {
			exponent = e
		}
	}
	_, fraction, _ := strings.Cut(mantissa, ".")
	return int64(len(fraction)) - exponent
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
		return CompareDecimals(d.value, o.value), nil
	case Integer:
		return CompareDecimals(d.value, decimal.NewFromInt(o.value)), nil
	}
	return 0, NewTypeError(TypeNameDecimal, other.Type(), "comparison")
}

// Add returns the sum of two decimals.
func (d Decimal) Add(other Decimal) Decimal {
	return Decimal{value: decimals.Computed(d.value.Add(other.value))}
}

// Subtract returns the difference of two decimals.
func (d Decimal) Subtract(other Decimal) Decimal {
	return Decimal{value: decimals.Computed(d.value.Sub(other.value))}
}

// Multiply returns the product of two decimals.
func (d Decimal) Multiply(other Decimal) Decimal {
	return Decimal{value: decimals.Computed(d.value.Mul(other.value))}
}

// Divide returns the result of division.
func (d Decimal) Divide(other Decimal) (Decimal, error) {
	if other.value.IsZero() {
		return Decimal{}, fmt.Errorf("division by zero")
	}
	return Decimal{value: d.value.DivRound(other.value, DivisionPlaces)}, nil
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
// It wraps past the int64 range.
//
// Deprecated: use IntegerFromDecimal, which reports a value past the range
// as ErrIntegerRange.
func (d Decimal) Ceiling() Integer {
	return NewInteger(d.value.Ceil().IntPart())
}

// Floor returns the largest integer <= d.
//
// It wraps past the int64 range.
//
// Deprecated: use IntegerFromDecimal, which reports a value past the range
// as ErrIntegerRange.
func (d Decimal) Floor() Integer {
	return NewInteger(d.value.Floor().IntPart())
}

// Truncate returns the integer part.
//
// It wraps past the int64 range.
//
// Deprecated: use IntegerFromDecimal, which reports a value past the range
// as ErrIntegerRange.
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
