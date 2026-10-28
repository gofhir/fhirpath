// Package ucum adapts UCUM unit handling to the exact decimal arithmetic this
// engine uses.
//
// It holds no unit table of its own. Every question about a unit — is it valid,
// is it commensurable with another, what converts one into the other, what unit
// results from multiplying two — is answered by github.com/gofhir/ucum, which
// parses the full UCUM grammar from the official definitions. That covers
// prefixes, compound expressions such as mg/kg/d, annotations, and the special
// scales where conversion is affine rather than a factor.
//
// Conversions run through the library's exact rational API so that results are
// free of float64 rounding: 1 'L' is exactly 1000 'mL', which decides whether an
// equality holds.
package ucum

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/shopspring/decimal"

	ucumlib "github.com/gofhir/ucum/v4"

	"github.com/gofhir/fhirpath/internal/decimals"
)

// exactDigits bounds the decimal expansion of a conversion whose exact result is
// a repeating fraction — 100 '[degF]' in 'Cel' is 340/9. FHIR caps decimals at
// 18 digits, so this leaves ample headroom before any value the engine can hold.
const exactDigits = 34

var (
	serviceOnce sync.Once
	service     ucumlib.ExactService
	serviceErr  error
)

// exact returns the shared UCUM service, loading the definitions once. The
// service is immutable after construction and safe for concurrent use.
func exact() (ucumlib.ExactService, error) {
	serviceOnce.Do(func() {
		service, serviceErr = ucumlib.NewExact()
	})
	return service, serviceErr
}

// Comparable reports whether two unit codes measure the same dimension, which is
// what makes comparing or adding their quantities meaningful. Unknown or
// malformed codes are not comparable to anything.
func Comparable(from, to string) bool {
	svc, err := exact()
	if err != nil {
		return false
	}
	ok, err := svc.IsComparable(from, to)
	return err == nil && ok
}

// Convert expresses a value given in one unit in terms of another, exactly.
//
// Returns an error when the units are not commensurable or either is unknown.
// Affine scales such as Cel and [degF] are handled: converting between them is
// not a multiplication, and the library applies the offset.
func Convert(value decimal.Decimal, from, to string) (decimal.Decimal, error) {
	if from == to {
		return value, nil
	}

	svc, err := exact()
	if err != nil {
		return decimal.Decimal{}, err
	}

	converted, err := svc.ConvertRat(toRat(value), from, to)
	if err != nil {
		// Logarithmic and trigonometric scales have no exact rational form; the
		// library's float path is the defined route for those.
		if errors.Is(err, ucumlib.ErrNotRational) {
			approximate, floatErr := svc.Convert(value.InexactFloat64(), from, to)
			if floatErr != nil {
				return decimal.Decimal{}, floatErr
			}
			// Zero on a logarithmic scale is minus infinity, which no decimal holds
			if math.IsInf(approximate, 0) || math.IsNaN(approximate) {
				return decimal.Decimal{}, fmt.Errorf("%w: %s in %s", ErrNotFinite, from, to)
			}
			return decimal.NewFromFloat(approximate), nil
		}
		return decimal.Decimal{}, err
	}

	return fromRat(converted)
}

// factors holds what Factor has found, by unit pair: a factor, or that the
// pair has none. factorCount counts the pairs, so that units read from hostile
// input cannot grow it without bound.
var (
	factors     sync.Map
	factorCount atomic.Int64
)

// maxFactors bounds how many unit pairs Factor remembers.
const maxFactors = 4096

// foundFactor is what Factor found for a pair of units, and for a pair with no
// factor, the step Step measured.
type foundFactor struct {
	factor *big.Rat
	err    error
	step   decimal.Decimal
}

// Factor returns how much of one unit one of another is, exactly, when both
// stand on ratio scales: 1000 for 'L' to 'mL'. A unit on a scale with an
// offset, or worse, has no factor: Factor returns an error NotLinear reports,
// and the caller converts points instead. Either answer is found once per
// pair of units. The factor returned is shared and must not be modified.
func Factor(from, to string) (*big.Rat, error) {
	key := from + "\x00" + to
	if found, ok := factors.Load(key); ok {
		f := found.(foundFactor)
		return f.factor, f.err
	}
	svc, err := exact()
	if err != nil {
		return nil, err
	}
	factor, err := svc.ConversionFactor(from, to)
	if err != nil && !NotLinear(err) {
		return nil, err
	}
	// A place is taken before the pair is stored, so that callers at once
	// cannot overshoot the bound
	if factorCount.Add(1) <= maxFactors {
		if _, loaded := factors.LoadOrStore(key, foundFactor{factor: factor, err: err}); loaded {
			factorCount.Add(-1)
		}
	} else {
		factorCount.Add(-1)
	}
	return factor, err
}

// errNotAffine reports a scale on which a step has no constant size, a
// logarithm or an angle, where no unit is more granular than another.
var errNotAffine = errors.New("ucum: the scale is not affine: a step has no size")

// Step returns how much of one unit a step of another is, for units with no
// factor, on a scale with an offset: the difference two points convert to,
// since a point is not a step — 1 '[degF]' is 255.9 'K', a degree of it 5/9
// of a kelvin. A scale on which steps differ, a logarithm or an angle, has no
// step, and Step returns an error. Either answer is found once per pair.
func Step(from, to string) (decimal.Decimal, error) {
	key := "step\x00" + from + "\x00" + to
	if found, ok := factors.Load(key); ok {
		f := found.(foundFactor)
		return f.step, f.err
	}
	step, err := measureStep(from, to)
	if factorCount.Add(1) <= maxFactors {
		if _, loaded := factors.LoadOrStore(key, foundFactor{err: err, step: step}); loaded {
			factorCount.Add(-1)
		}
	} else {
		factorCount.Add(-1)
	}
	return step, err
}

// measureStep converts three points and returns the step between them where
// the two steps agree.
func measureStep(from, to string) (decimal.Decimal, error) {
	var points [3]decimal.Decimal
	for i := range points {
		point, err := Convert(decimal.NewFromInt(int64(i)), from, to)
		if err != nil {
			return decimal.Decimal{}, err
		}
		points[i] = point
	}
	// Conversions to a repeating fraction stop at exactDigits, so two steps of
	// an affine scale agree to well within that, and those of a logarithm
	// differ in their first digits
	step := points[1].Sub(points[0])
	tolerance := decimal.New(1, -(exactDigits - 6)).Mul(decimal.Max(decimal.NewFromInt(1), step.Abs()))
	if points[2].Sub(points[1]).Sub(step).Abs().GreaterThan(tolerance) {
		return decimal.Decimal{}, errNotAffine
	}
	return step, nil
}

// Special reports a unit on a scale with no single factor, one with an
// offset such as Cel or [degF], or a logarithmic one such as [pH]: FHIRPath's
// "special" units, which arithmetic does not combine.
func Special(code string) bool {
	_, err := Factor(code, code)
	return NotLinear(err)
}

// NotLinear reports the error Factor returns for a unit with no factor, on a
// scale with an offset such as Cel, or a logarithmic one.
func NotLinear(err error) bool {
	return errors.Is(err, ucumlib.ErrNotLinear)
}

// Scale multiplies a value by a factor, exactly.
func Scale(value decimal.Decimal, factor *big.Rat) (decimal.Decimal, error) {
	return fromRat(new(big.Rat).Mul(toRat(value), factor))
}

// Unscale divides a value by a factor, exactly.
func Unscale(value decimal.Decimal, factor *big.Rat) (decimal.Decimal, error) {
	if factor.Sign() == 0 {
		return decimal.Decimal{}, fmt.Errorf("division by a zero factor")
	}
	return fromRat(new(big.Rat).Quo(toRat(value), factor))
}

// Multiply combines two quantities, returning the value and the unit of the
// product in the operands' own units — 12 'cm' by 3 'cm' is 36 'cm2', and
// 2 'cm' by 2 'm' is 400 'cm2'.
func Multiply(leftValue decimal.Decimal, leftUnit string, rightValue decimal.Decimal, rightUnit string) (decimal.Decimal, string, error) {
	return combine(leftValue, leftUnit, rightValue, rightUnit, false)
}

// Divide divides one quantity by another, returning the value and the unit of
// the quotient — 4 'g' by 2 'm' is 2 'g.m-1'.
func Divide(leftValue decimal.Decimal, leftUnit string, rightValue decimal.Decimal, rightUnit string) (decimal.Decimal, string, error) {
	return combine(leftValue, leftUnit, rightValue, rightUnit, true)
}

// combine performs the unit algebra for multiplication and division. The
// library computes in float64 here, which is exact for the powers of ten that
// unit prefixes contribute.
func combine(leftValue decimal.Decimal, leftUnit string, rightValue decimal.Decimal, rightUnit string, divide bool) (decimal.Decimal, string, error) {
	svc, err := exact()
	if err != nil {
		return decimal.Decimal{}, "", err
	}

	// The library combines the two in float64, applying each unit's scale, an
	// offset or a logarithm included. A value past float64's range has no
	// answer there; one too small for it is zero, as it always was
	leftFloat, rightFloat := leftValue.InexactFloat64(), rightValue.InexactFloat64()
	if !finite(leftFloat) || !finite(rightFloat) {
		return decimal.Decimal{}, "", ErrNotFinite
	}
	left := ucumlib.Pair{Value: leftFloat, Code: leftUnit}
	right := ucumlib.Pair{Value: rightFloat, Code: rightUnit}

	var result ucumlib.Pair
	if divide {
		result, err = svc.Divide(left, right)
	} else {
		result, err = svc.Multiply(left, right)
	}
	if err != nil {
		return decimal.Decimal{}, "", err
	}
	if !finite(result.Value) {
		return decimal.Decimal{}, "", ErrNotFinite
	}

	return decimal.NewFromFloat(result.Value), result.Code, nil
}

// finite reports a float64 that is a number.
func finite(f float64) bool {
	return !math.IsInf(f, 0) && !math.IsNaN(f)
}

// ErrNotFinite reports a result that has no finite value, which no decimal
// holds: 1E200 'm' by 1E200 'm' in float64, or zero on a logarithmic scale.
var ErrNotFinite = errors.New("ucum: the result has no finite value")

// Validate reports whether a code is a well-formed UCUM unit.
func Validate(code string) error {
	svc, err := exact()
	if err != nil {
		return err
	}
	return svc.Validate(code)
}

// toRat converts a decimal to an exact rational: a decimal is a coefficient
// times a power of ten, so no precision is lost.
func toRat(d decimal.Decimal) *big.Rat {
	return d.Rat()
}

// fromRat converts an exact rational back to a decimal, expanding a repeating
// fraction to exactDigits places.
func fromRat(r *big.Rat) (decimal.Decimal, error) {
	if r == nil {
		return decimal.Decimal{}, fmt.Errorf("nil conversion result")
	}

	if r.IsInt() {
		return decimal.NewFromBigInt(r.Num(), 0), nil
	}

	// Rounded half away from zero at exactDigits places, as big.Rat writes
	// it, and without the padding, so the value presents the precision it
	// actually carries
	return decimals.Natural(decimal.NewFromBigRat(r, exactDigits)), nil
}
