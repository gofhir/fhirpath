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
	"math/big"
	"sync"

	"github.com/shopspring/decimal"

	ucumlib "github.com/gofhir/ucum/v4"
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
			return decimal.NewFromFloat(approximate), nil
		}
		return decimal.Decimal{}, err
	}

	return fromRat(converted)
}

// Multiply combines two quantities, returning the value and the unit of the
// product — 2 'cm' by 2 'm' is 0.04 'm2'.
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

	left := ucumlib.Pair{Value: leftValue.InexactFloat64(), Code: leftUnit}
	right := ucumlib.Pair{Value: rightValue.InexactFloat64(), Code: rightUnit}

	var result ucumlib.Pair
	if divide {
		result, err = svc.Divide(left, right)
	} else {
		result, err = svc.Multiply(left, right)
	}
	if err != nil {
		return decimal.Decimal{}, "", err
	}

	return decimal.NewFromFloat(result.Value), result.Code, nil
}

// Validate reports whether a code is a well-formed UCUM unit.
func Validate(code string) error {
	svc, err := exact()
	if err != nil {
		return err
	}
	return svc.Validate(code)
}

// toRat converts a decimal to an exact rational. A decimal's own text is an
// exact base-ten representation, so no precision is lost.
func toRat(d decimal.Decimal) *big.Rat {
	rat, ok := new(big.Rat).SetString(d.String())
	if !ok {
		return new(big.Rat)
	}
	return rat
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

	value, err := decimal.NewFromString(r.FloatString(exactDigits))
	if err != nil {
		return decimal.Decimal{}, err
	}

	// FloatString pads to the requested places; drop the padding so the value
	// presents the precision it actually carries.
	normalized, err := decimal.NewFromString(value.String())
	if err != nil {
		return value, nil
	}
	return normalized, nil
}
