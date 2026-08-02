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

// DefaultQuantityUnit is the unit a quantity carries when its source gave none.
//
// The specification calls it "the UCUM default unit" and writes it as '1':
// 42.toQuantity() is 42 '1'. It matters that the unit is present rather than
// blank, because '1' is dimensionless and so not convertible to a meter — which
// is why 45.toQuantity('m') is empty rather than 45 'm'.
const DefaultQuantityUnit = "1"

// quantityStringPattern is the format toQuantity() accepts on a String,
// transcribed from the specification:
//
//	(?'value'(\+|-)?\d+(\.\d+)?)\s*('(?'unit'[^']+)'|(?'time'[a-zA-Z]+))?
//
// Anchored, because a partial match is not a conversion: '1.a' matches the
// pattern's value group alone and the remainder is not a unit, so the string
// does not convert at all.
var quantityStringPattern = regexp.MustCompile(`^([+-]?\d+(?:\.\d+)?)\s*(?:'([^']+)'|([a-zA-Z]+))?$`)

// ParseQuantityString converts a string to a Quantity under the rule
// toQuantity() states, and reports whether the string is convertible at all.
//
// The distinction the pattern draws is between a quoted unit and a bare word.
// A quoted unit is a UCUM code and is taken as written. A bare word is a
// calendar duration keyword, so it has to be one — '1 wk' does not convert,
// because wk is a UCUM code that was written without its quotes, while
// '4 days' does.
func ParseQuantityString(s string) (Quantity, bool) {
	matches := quantityStringPattern.FindStringSubmatch(strings.TrimSpace(s))
	if matches == nil {
		return Quantity{}, false
	}

	value, err := decimal.NewFromString(matches[1])
	if err != nil {
		return Quantity{}, false
	}

	switch {
	case matches[2] != "":
		return Quantity{value: value, unit: matches[2]}, true

	case matches[3] != "":
		if !calendarDurationUnits[matches[3]] {
			return Quantity{}, false
		}
		return Quantity{value: value, unit: matches[3]}, true
	}

	return Quantity{value: value, unit: DefaultQuantityUnit}, true
}

// ConvertTo restates a quantity in another unit, reporting false when the units
// are not commensurable — which the specification treats as an empty result
// rather than an error: "24 'm'.toQuantity('kg') // empty".
func (q Quantity) ConvertTo(unit string) (Quantity, bool) {
	if q.unit == unit {
		return q, true
	}

	// Both sides on the calendar
	if converted, ok := convertCalendar(q.value, q.unit, unit); ok {
		return Quantity{value: dropTrailingZeros(converted), unit: unit}, true
	}

	// "When explicitly converting between UCUM definite durations and calendar
	// units of differing magnitudes, perform the conversion within the unit
	// system of the source, then change the unit to the corresponding target
	// unit." So 182.5 days is half a calendar year before it becomes 0.5 'a'.
	if isCalendarUnit(q.unit) && !isCalendarUnit(unit) {
		if keyword, ok := ucumCodeToCalendar[unit]; ok {
			if converted, ok := convertCalendar(q.value, q.unit, keyword); ok {
				return Quantity{value: dropTrailingZeros(converted), unit: unit}, true
			}
		}
	}

	// An explicit conversion may cross from a calendar keyword to its UCUM
	// counterpart, which comparison is not allowed to do on its own
	source := convertibleUnit(q.unit)
	if code, ok := explicitCalendarToUCUM[q.unit]; ok {
		source = code
	}
	target := convertibleUnit(unit)
	if code, ok := explicitCalendarToUCUM[unit]; ok {
		target = code
	}

	converted, err := ucum.Convert(q.value, source, target)
	if err != nil {
		return Quantity{}, false
	}
	return Quantity{value: dropTrailingZeros(converted), unit: unit}, true
}

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
	return TypeNameQuantity
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
var calendarDurationUnits = func() map[string]bool {
	units := make(map[string]bool)
	for _, name := range withPlurals(calendarUnitNames) {
		units[name] = true
	}
	return units
}()

// Negate returns the quantity with its value's sign flipped, keeping the unit:
// a negative mass is still a mass.
func (q Quantity) Negate() Quantity {
	return Quantity{value: q.value.Neg(), unit: q.unit}
}

// Abs returns the quantity with a non-negative value, keeping the unit.
func (q Quantity) Abs() Quantity {
	return Quantity{value: q.value.Abs(), unit: q.unit}
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

	// Calendar durations compare with each other on the calendar's own scale
	leftIsCalendar, rightIsCalendar := isCalendarUnit(q.unit), isCalendarUnit(other.unit)
	if leftIsCalendar && rightIsCalendar {
		return true
	}

	// One of each system meets only at a week and below, where the two agree by
	// definition. A year or a month differs between them — a calendar year is
	// 365 days, a UCUM year 365.25 — so no common unit exists and the comparison
	// has no answer: 1 'a' = 365 days is empty, as is 1 month = 30 'd'.
	if leftIsCalendar != rightIsCalendar {
		if aboveWeek(q.unit) || aboveWeek(other.unit) {
			return false
		}
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

// calendarUnitSeconds is the length the specification assigns each calendar
// duration, from its table of conversion factors:
//
//	1 year = 12 months or 365 days     1 day    = 24 hours
//	1 month = 30 days                  1 hour   = 60 minutes
//	1 week = 7 days                    1 minute = 60 seconds
//
// These are the calendar's own lengths, not UCUM's. A UCUM year is 365.25 days
// and a UCUM month 30.4375, which is why the two systems only meet at a week and
// below, where the specification says the factors "are the same as UCUM, so can
// be used interchangeably".
var calendarUnitSeconds = map[string]int64{
	unitNameYear:   365 * 24 * 60 * 60,
	unitNameMonth:  30 * 24 * 60 * 60,
	unitNameWeek:   7 * 24 * 60 * 60,
	unitNameDay:    24 * 60 * 60,
	unitNameHour:   60 * 60,
	unitNameMinute: 60,
	unitNameSecond: 1,
}

// calendarUnitMilliseconds indexes the same table in milliseconds, so that the
// millisecond keyword takes part without fractional arithmetic.
var calendarUnitMilliseconds = func() map[string]decimal.Decimal {
	lengths := map[string]decimal.Decimal{
		unitNameMillisecond: decimal.NewFromInt(1),
	}
	for unit, seconds := range calendarUnitSeconds {
		lengths[unit] = decimal.NewFromInt(seconds * 1000)
	}

	// The grammar accepts either spelling
	for unit, length := range lengths {
		lengths[unit+"s"] = length
	}
	return lengths
}()

// yearsAndMonths are the two units whose relationship to each other is fixed at
// twelve while their relationship to a day is not.
//
// This is the non-transitivity the specification warns about: "1 year ~ 12
// months and 1 month = 30 days, but 1 year != 360 days". Going through days
// would make a year 12.1666 months, so year and month convert directly.
var yearsAndMonths = map[string]int{
	unitNameYear: 12, unitNameYear + "s": 12,
	unitNameMonth: 1, unitNameMonth + "s": 1,
}

// aboveWeek reports whether a unit measures a year or a month, in either system.
// These are the durations whose length the calendar and UCUM disagree about, so
// they are the ones that cannot cross between the two.
func aboveWeek(unit string) bool {
	if _, ok := yearsAndMonths[unit]; ok {
		return true
	}
	return unit == "a" || unit == "mo"
}

// isCalendarUnit reports whether a unit is one of the calendar duration keywords.
func isCalendarUnit(unit string) bool {
	_, ok := calendarUnitMilliseconds[unit]
	return ok
}

// convertCalendar restates a calendar duration in another calendar unit,
// choosing the shortest conversion chain — which the specification requires:
// "If converting to/from years or months you shall use the shortest conversion
// chain possible".
func convertCalendar(value decimal.Decimal, from, to string) (decimal.Decimal, bool) {
	if from == to {
		return value, true
	}
	if !isCalendarUnit(from) || !isCalendarUnit(to) {
		return decimal.Decimal{}, false
	}

	// Year against month is the one pair that does not go through a duration
	if fromFactor, ok := yearsAndMonths[from]; ok {
		if toFactor, ok := yearsAndMonths[to]; ok {
			return value.Mul(decimal.NewFromInt(int64(fromFactor))).
				Div(decimal.NewFromInt(int64(toFactor))), true
		}
	}

	return value.Mul(calendarUnitMilliseconds[from]).Div(calendarUnitMilliseconds[to]), true
}

// explicitCalendarToUCUM extends calendarToUCUM with the two keywords that have
// no exact UCUM equivalent, for use by toQuantity() alone.
//
// The specification allows the crossing only when it is asked for: "Note that
// explicit conversion using toQuantity() will change code-systems to
// intentionally perform this equality", after stating that 1 year = 1 'a' is
// otherwise empty. So this table is not reachable from comparison.
var explicitCalendarToUCUM = map[string]string{
	unitNameYear: "a", unitNameYear + "s": "a",
	unitNameMonth: "mo", unitNameMonth + "s": "mo",
}

// ucumCodeToCalendar inverts the correspondence, naming the calendar keyword a
// UCUM duration code stands opposite. Used when an explicit conversion has to
// finish inside the calendar before changing systems.
var ucumCodeToCalendar = map[string]string{
	"a": unitNameYear, "mo": unitNameMonth, "wk": unitNameWeek,
	"d": unitNameDay, "h": unitNameHour, "min": unitNameMinute,
	"s": unitNameSecond, "ms": unitNameMillisecond,
}

// valueIn returns q's value expressed in target's unit.
// Returns ErrIncompatibleUnits when the two units are not commensurable.
func (q Quantity) valueIn(target Quantity) (decimal.Decimal, error) {
	// Same unit, or a unitless operand: the value carries over untouched.
	if q.unit == target.unit || q.unit == "" || target.unit == "" {
		return q.value, nil
	}

	// Two calendar durations convert within the calendar
	if converted, ok := convertCalendar(q.value, q.unit, target.unit); ok {
		return converted, nil
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
