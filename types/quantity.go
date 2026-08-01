package types

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/gofhir/fhirpath/internal/ucum"
)

// ErrIncompatibleUnits reports that two quantities are not commensurable, so no
// conversion between their units exists. Per the FHIRPath spec, "attempting to
// operate on quantities with invalid units will result in empty ({})", so
// callers translate this sentinel into an empty collection instead of failing
// the whole expression.
var ErrIncompatibleUnits = errors.New("incompatible units")

// Quantity represents a FHIRPath quantity value with a numeric value and unit.
type Quantity struct {
	value decimal.Decimal
	unit  string
}

// Quantity regex pattern: number followed by optional unit
var quantityPattern = regexp.MustCompile(`^([+-]?\d+\.?\d*)\s*(?:'([^']+)'|(\S+))?$`)

// NewQuantity creates a Quantity from a string.
func NewQuantity(s string) (Quantity, error) {
	matches := quantityPattern.FindStringSubmatch(strings.TrimSpace(s))
	if matches == nil {
		return Quantity{}, fmt.Errorf("invalid quantity format: %s", s)
	}

	val, err := decimal.NewFromString(matches[1])
	if err != nil {
		return Quantity{}, fmt.Errorf("invalid quantity value: %s", matches[1])
	}

	unit := ""
	if matches[2] != "" {
		unit = matches[2] // Quoted unit
	} else if matches[3] != "" {
		unit = matches[3] // Unquoted unit
	}

	return Quantity{value: val, unit: unit}, nil
}

// NewQuantityFromDecimal creates a Quantity from a decimal value and unit.
func NewQuantityFromDecimal(value decimal.Decimal, unit string) Quantity {
	return Quantity{value: value, unit: unit}
}

// Type returns the type name.
func (q Quantity) Type() string {
	return "Quantity"
}

// Equal checks equality with another value.
// For quantities with different units, uses UCUM normalization per FHIRPath spec.
func (q Quantity) Equal(other Value) bool {
	o, ok := other.(Quantity)
	if !ok {
		return false
	}

	// Convert through the same exact path comparison uses, so that units that
	// convert — including calendar keywords against their UCUM codes — agree.
	rhs, err := o.valueIn(q)
	if err != nil {
		return false
	}
	return q.value.Equal(rhs)
}

// Equivalent checks equivalence with another value.
// For quantities, this uses UCUM normalization to compare values with different units.
// Per FHIRPath spec: quantities are equivalent if their canonical normalized forms are equal.
func (q Quantity) Equivalent(other Value) bool {
	o, ok := other.(Quantity)
	if !ok {
		return false
	}

	rhs, err := o.valueIn(q)
	if err != nil {
		return false
	}

	// Equivalence compares "on values rounded to the precision of the least
	// precise operand", so 4 'g' ~ 4040 'mg': 4.040 g rounded to the whole
	// gram the left side was written with is 4.
	places := scaleOf(q.value)
	if other := scaleOf(rhs); other < places {
		places = other
	}
	return q.value.Round(places).Equal(rhs.Round(places))
}

// scaleOf returns how many fractional digits a value carries.
func scaleOf(d decimal.Decimal) int32 {
	if exponent := d.Exponent(); exponent < 0 {
		return -exponent
	}
	return 0
}

// String returns the string representation.
// String returns the quantity in FHIRPath literal notation. A UCUM unit is
// quoted — 1 'wk' — while a calendar duration keyword is not — 1 week — which is
// how the grammar distinguishes the two.
func (q Quantity) String() string {
	value := q.valueText()
	if q.unit == "" {
		return value
	}
	if calendarDurationUnits[q.unit] {
		return value + " " + q.unit
	}
	return fmt.Sprintf("%s '%s'", value, q.unit)
}

// valueText renders the value keeping the scale it carries, so that a quantity
// computed to a given precision presents it: 1.58650000 'cm', not 1.5865 'cm'.
func (q Quantity) valueText() string {
	if exponent := q.value.Exponent(); exponent < 0 {
		return q.value.StringFixed(-exponent)
	}
	return q.value.String()
}

// calendarDurationUnits are the time-valued keywords the grammar accepts as an
// unquoted unit (rules dateTimePrecision and pluralDateTimePrecision). Every
// other unit is a UCUM code and is quoted.
var calendarDurationUnits = map[string]bool{
	"year": true, "month": true, "week": true, "day": true,
	"hour": true, "minute": true, "second": true, "millisecond": true,
	"years": true, "months": true, "weeks": true, "days": true,
	"hours": true, "minutes": true, "seconds": true, "milliseconds": true,
}

// IsEmpty returns false for Quantity.
func (q Quantity) IsEmpty() bool {
	return false
}

// Value returns the numeric value.
func (q Quantity) Value() decimal.Decimal {
	return q.value
}

// Unit returns the unit string.
func (q Quantity) Unit() string {
	return q.unit
}

// Compare compares two quantities.
// Returns -1, 0, or 1 if units are compatible, or error if not.
// Uses UCUM normalization to compare quantities with different but compatible units.
// Implements the Comparable interface.
func (q Quantity) Compare(other Value) (int, error) {
	otherQ, ok := other.(Quantity)
	if !ok {
		return 0, fmt.Errorf("cannot compare Quantity with %s", other.Type())
	}

	// Express the right operand in the left operand's unit, so that the
	// comparison is exact decimal arithmetic rather than a float round-trip
	// through both canonical forms.
	rhs, err := otherQ.valueIn(q)
	if err != nil {
		return 0, err
	}
	return q.value.Cmp(rhs), nil
}

// Comparable reports whether the two quantities can be compared, that is
// whether their units are commensurable. Quantities sharing a unit always are;
// otherwise both units must reduce to the same canonical unit.
func (q Quantity) Comparable(other Quantity) bool {
	if q.unit == other.unit || q.unit == "" || other.unit == "" {
		return true
	}
	return ucum.Comparable(convertibleUnit(q.unit), convertibleUnit(other.unit))
}

// calendarToUCUM maps the calendar duration keywords onto the UCUM codes they
// convert to exactly, so that 7 days and 1 'wk' compare.
//
// Years and months are deliberately absent: a calendar year lands on the same
// date a year later while a UCUM year is a fixed 365.25 days, and the
// specification requires an explicit conversion to cross between them.
var calendarToUCUM = map[string]string{
	"week": "wk", "weeks": "wk",
	"day": "d", "days": "d",
	"hour": "h", "hours": "h",
	"minute": "min", "minutes": "min",
	"second": "s", "seconds": "s",
	"millisecond": "ms", "milliseconds": "ms",
}

// convertibleUnit returns the unit to convert with, translating a calendar
// keyword to its exact UCUM equivalent where one exists.
func convertibleUnit(unit string) string {
	if code, ok := calendarToUCUM[unit]; ok {
		return code
	}
	return unit
}

// valueIn returns q's value expressed in target's unit.
// Returns ErrIncompatibleUnits when the two units are not commensurable.
func (q Quantity) valueIn(target Quantity) (decimal.Decimal, error) {
	// Same unit, or a unitless operand: the value carries over untouched.
	if q.unit == target.unit || q.unit == "" || target.unit == "" {
		return q.value, nil
	}

	converted, err := ucum.Convert(q.value, convertibleUnit(q.unit), convertibleUnit(target.unit))
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("%w: %s and %s", ErrIncompatibleUnits, target.unit, q.unit)
	}
	return converted, nil
}

// dropTrailingZeros returns the value at its natural scale. Division runs at a
// fixed scale, so 3 'g' converted to 'mg' comes back as 3000.0000000000000000;
// the zeros are an artifact of how the quotient was computed, not precision the
// value carries, and they would otherwise be presented as significant.
func dropTrailingZeros(d decimal.Decimal) decimal.Decimal {
	normalized, err := decimal.NewFromString(d.String())
	if err != nil {
		return d
	}
	return normalized
}

// Add adds two quantities.
// Commensurable units are converted into the left operand's unit, which is also
// the unit of the result: 1 'g' + 500 'mg' is 1.5 'g'.
func (q Quantity) Add(other Quantity) (Quantity, error) {
	rhs, err := other.valueIn(q)
	if err != nil {
		return Quantity{}, err
	}
	return Quantity{value: q.value.Add(rhs), unit: q.resultUnit(other)}, nil
}

// Subtract subtracts two quantities.
// Units are handled as in [Quantity.Add].
func (q Quantity) Subtract(other Quantity) (Quantity, error) {
	rhs, err := other.valueIn(q)
	if err != nil {
		return Quantity{}, err
	}
	return Quantity{value: q.value.Sub(rhs), unit: q.resultUnit(other)}, nil
}

// resultUnit returns the unit an arithmetic result carries: the left operand's,
// unless it is unitless.
func (q Quantity) resultUnit(other Quantity) string {
	if q.unit == "" {
		return other.unit
	}
	return q.unit
}

// MultiplyQuantity multiplies two quantities, combining their units:
// 2 'cm' by 2 'm' is 0.04 'm2'.
func (q Quantity) MultiplyQuantity(other Quantity) (Quantity, error) {
	value, unit, err := ucum.Multiply(q.value, q.unit, other.value, other.unit)
	if err != nil {
		return Quantity{}, fmt.Errorf("%w: %s and %s", ErrIncompatibleUnits, q.unit, other.unit)
	}
	return Quantity{value: value, unit: unit}, nil
}

// DivideQuantity divides two quantities, combining their units:
// 4 'g' by 2 'm' is 2 'g.m-1'.
func (q Quantity) DivideQuantity(other Quantity) (Quantity, error) {
	if other.value.IsZero() {
		return Quantity{}, fmt.Errorf("division by zero")
	}
	value, unit, err := ucum.Divide(q.value, q.unit, other.value, other.unit)
	if err != nil {
		return Quantity{}, fmt.Errorf("%w: %s and %s", ErrIncompatibleUnits, q.unit, other.unit)
	}
	return Quantity{value: value, unit: unit}, nil
}

// Multiply multiplies the quantity by a number.
func (q Quantity) Multiply(factor decimal.Decimal) Quantity {
	return Quantity{value: q.value.Mul(factor), unit: q.unit}
}

// Divide divides the quantity by a number.
func (q Quantity) Divide(divisor decimal.Decimal) (Quantity, error) {
	if divisor.IsZero() {
		return Quantity{}, fmt.Errorf("division by zero")
	}
	return Quantity{value: dropTrailingZeros(q.value.Div(divisor)), unit: q.unit}, nil
}
