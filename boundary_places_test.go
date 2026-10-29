package fhirpath_test

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// boundaryMaxPrecision is this engine's maximum for a decimal's boundary,
// which FHIRPath 3.0.0 leaves to the implementation: "If the precision is
// greater than the maximum possible precision of the implementation, the
// result is empty" (CONFORMANCE.md).
const boundaryMaxPrecision = 18

// exactBoundary is what FHIRPath 3.0.0 asks of lowBoundary and highBoundary,
// computed apart from the engine: "The least possible value of the input to
// the specified precision", and the greatest. A decimal written to p places
// stands for anything within half a unit of its p-th place; the boundary
// rounds that outward to q places, as the text's examples do
// (1.587.lowBoundary(2) is 1.58, 1.587.highBoundary(2) is 1.59). The places
// are counted from the text as written, not by the engine.
func exactBoundary(written string, q int, low bool) string {
	unsigned := strings.TrimPrefix(written, "-")
	_, fraction, _ := strings.Cut(unsigned, ".")
	value, ok := new(big.Rat).SetString(written)
	if !ok {
		panic(written)
	}
	half := new(big.Rat).SetFrac(big.NewInt(5), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(len(fraction)+1)), nil))
	scale := new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(q)), nil))
	var bound *big.Rat
	if low {
		bound = new(big.Rat).Sub(value, half)
	} else {
		bound = new(big.Rat).Add(value, half)
	}
	bound.Mul(bound, scale)
	// big.Int.DivMod is Euclidean, so with a positive denominator it floors,
	// negatives included; a ceiling is the negated floor of the negation
	var n *big.Int
	if low {
		n, _ = new(big.Int).DivMod(bound.Num(), bound.Denom(), new(big.Int))
	} else {
		neg := new(big.Int).Neg(bound.Num())
		n, _ = new(big.Int).DivMod(neg, bound.Denom(), new(big.Int))
		n.Neg(n)
	}
	return fixedPlaces(n, q)
}

// fixedPlaces writes n·10^-q with exactly q places.
func fixedPlaces(n *big.Int, q int) string {
	sign := ""
	if n.Sign() < 0 {
		sign = "-"
	}
	digits := new(big.Int).Abs(n).String()
	if q == 0 {
		return sign + digits
	}
	if len(digits) <= q {
		digits = strings.Repeat("0", q-len(digits)+1) + digits
	}
	return sign + digits[:len(digits)-q] + "." + digits[len(digits)-q:]
}

// randomDecimal writes a decimal as a FHIRPath literal can: up to 30 places,
// either sign, an integer part of up to 18 digits, zero and trailing zeros
// among them.
func randomDecimal(r *rand.Rand) string {
	whole := "0"
	if r.IntN(4) > 0 {
		whole = fmt.Sprint(r.Int64N(pow10(1 + r.IntN(18))))
	}
	places := r.IntN(31)
	var fraction strings.Builder
	for range places {
		switch r.IntN(5) {
		case 0:
			fraction.WriteByte('0')
		default:
			fraction.WriteByte(byte('0' + r.IntN(10)))
		}
	}
	text := whole
	if places > 0 {
		text += "." + fraction.String()
	}
	if r.IntN(2) == 0 {
		text = "-" + text
	}
	return text
}

// pow10 is 10^n, for n up to 18.
func pow10(n int) int64 {
	p := int64(1)
	for range n {
		p *= 10
	}
	return p
}

// randomPrecision draws a precision up to the maximum, and its argument; or no
// argument, and the default of 8 it stands for:"If no precision is specified, the greatest precision
// of the type of the input value is used (i.e. at least 8 for Decimal ...)".
func randomPrecision(r *rand.Rand) (q int, arg string) {
	if r.IntN(5) == 0 {
		return 8, ""
	}
	q = r.IntN(boundaryMaxPrecision + 1)
	return q, fmt.Sprint(q)
}

// fitsTheBoundary checks a result apart from exactBoundary's formula: written
// at q places exactly, a low boundary L has L <= v - h < L + 10^-q, and a high
// one H has H - 10^-q < v + h <= H.
func fitsTheBoundary(t *testing.T, expr, written, got string, q int, low bool) {
	t.Helper()
	_, gotFraction, _ := strings.Cut(strings.TrimPrefix(got, "-"), ".")
	if len(gotFraction) != q {
		t.Errorf("%s = %s, not written at %d places", expr, got, q)
		return
	}
	_, fraction, _ := strings.Cut(strings.TrimPrefix(written, "-"), ".")
	v, _ := new(big.Rat).SetString(written)
	h := new(big.Rat).SetFrac(big.NewInt(5), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(len(fraction)+1)), nil))
	b, ok := new(big.Rat).SetString(got)
	if !ok {
		t.Errorf("%s = %s, no number", expr, got)
		return
	}
	step := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(q)), nil))
	if low {
		edge := new(big.Rat).Sub(v, h)
		if b.Cmp(edge) > 0 || new(big.Rat).Add(b, step).Cmp(edge) <= 0 {
			t.Errorf("%s = %s, not the least value at %d places", expr, got, q)
		}
		return
	}
	edge := new(big.Rat).Add(v, h)
	if b.Cmp(edge) < 0 || new(big.Rat).Sub(b, step).Cmp(edge) >= 0 {
		t.Errorf("%s = %s, not the greatest value at %d places", expr, got, q)
	}
}

// Every decimal's boundaries are exact at every precision up to the
// maximum, however many places it is written to (#244): through a Decimal,
// an Integer and a Quantity.
func TestDecimalBoundariesAreExact(t *testing.T) {
	r := rand.New(rand.NewPCG(244, 13))
	for range 3000 {
		written := randomDecimal(r)
		q, arg := randomPrecision(r)
		for _, low := range []bool{true, false} {
			name := "highBoundary"
			if low {
				name = "lowBoundary"
			}
			want := exactBoundary(written, q, low)
			expr := fmt.Sprintf("(%s).%s(%s)", written, name, arg)
			result, err := fhirpath.Evaluate([]byte(`{}`), expr)
			if err != nil || len(result) != 1 {
				t.Errorf("%s = %v, %v, want %s", expr, result, err, want)
				continue
			}
			got := result[0].String()
			if got != want {
				t.Errorf("%s = %s, want %s", expr, got, want)
			}
			fitsTheBoundary(t, expr, written, got, q, low)

			quantity := fmt.Sprintf("(%s 'mg').%s(%s)", written, name, arg)
			result, err = fhirpath.Evaluate([]byte(`{}`), quantity)
			if err != nil || len(result) != 1 || result[0].String() != want+" 'mg'" {
				t.Errorf("%s = %v, %v, want %s 'mg'", quantity, result, err, want)
			}
		}
	}
}

// An Integer stands for anything within half a unit of it, across the 64
// bits an Integer holds here.
func TestIntegerBoundariesAreExact(t *testing.T) {
	r := rand.New(rand.NewPCG(244, 14))
	for range 500 {
		written := fmt.Sprint(r.Int64N(pow10(1 + r.IntN(18))))
		if r.IntN(2) == 0 {
			written = "-" + written
		}
		if r.IntN(10) == 0 {
			written = "9223372036854775807"
		}
		q, arg := randomPrecision(r)
		for _, low := range []bool{true, false} {
			name := "highBoundary"
			if low {
				name = "lowBoundary"
			}
			want := exactBoundary(written, q, low)
			expr := fmt.Sprintf("(%s).%s(%s)", written, name, arg)
			result, err := fhirpath.Evaluate([]byte(`{}`), expr)
			if err != nil || len(result) != 1 || result[0].String() != want {
				t.Errorf("%s = %v, %v, want %s", expr, result, err, want)
			}
		}
	}
}

// Past the maximum, the result is empty, as the text has it.
func TestADecimalBoundaryPastTheMaximumIsEmpty(t *testing.T) {
	for _, expr := range []string{
		fmt.Sprintf("(1.5).lowBoundary(%d)", boundaryMaxPrecision+1),
		fmt.Sprintf("(1.12345678901234567890).highBoundary(%d)", boundaryMaxPrecision+1),
	} {
		result, err := fhirpath.Evaluate([]byte(`{}`), expr)
		if err != nil || len(result) != 0 {
			t.Errorf("%s = %v, %v, want empty", expr, result, err)
		}
	}
}

// boundaryObservation holds decimals of 17 places as a resource writes them,
// as the battery boundary-places' o.json does.
const boundaryObservation = `{"resourceType":"Observation","id":"o","status":"final","code":{"text":"x"},
"component":[{"code":{"text":"b"},"valueQuantity":{"value":-1.12345678901234567,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}]}`

// The boundaries of a decimal of many places, each row from its source
// (#244): the two references where they agree, the one the text bears out
// where they split, the exact value where both stop short. Measured with
// make three-engine, battery boundary-places: fhirpath.js 5.2.0 and the HL7
// validator 6.10.2; their defects are docs/upstream-findings.md's.
func TestDecimalBoundaryAtManyPlaces(t *testing.T) {
	for _, tc := range []struct{ expr, want, source string }{
		// FHIRPath 3.0.0's examples
		{"1.587.lowBoundary()", "1.58650000", "3.0.0 example"},
		{"1.587.highBoundary()", "1.58750000", "3.0.0 example"},
		{"1.587.lowBoundary(2)", "1.58", "the official suite, LowBoundaryDecimal2"},
		{"1.587.highBoundary(2)", "1.59", "the official suite, HighBoundaryDecimal1"},
		{"1.587.lowBoundary(0)", "1", "the official suite, LowBoundaryDecimal4"},

		// Both references
		{"(1.00000000000000000).lowBoundary()", "0.99999999", "fhirpath.js and Java"},
		{"(6.20086127000000000).lowBoundary()", "6.20086126", "fhirpath.js and Java"},

		// Java, past fhirpath.js's maximum of 8 (JS-9) or where it reads a
		// JavaScript number (JS-8) or no Quantity (JS-10)
		{"(1.00000000000000000).lowBoundary(16)", "0.9999999999999999", "Java"},
		{"(1.12345678901234567).lowBoundary(16)", "1.1234567890123456", "Java"},
		{"Observation.component[0].value.value.lowBoundary(16)", "-1.1234567890123457", "Java"},
		{"Observation.component[0].value.lowBoundary()", "-1.12345679 'mg'", "Java"},
		{"(1.5 + 0.00000000000000000).lowBoundary()", "1.49999999", "Java"},

		// The references split; the greatest or least value decides. Java
		// rounds half up (JAVA-9), errs (JAVA-10), widens a zero (JAVA-11)
		{"(1.00000000000000000).highBoundary()", "1.00000001", "fhirpath.js; exact 1.000000000000000005"},
		{"(123.4567890123456789).highBoundary()", "123.45678902", "fhirpath.js; exact 123.45678901234567895"},
		{"(1.5 + 0.00000000000000000).highBoundary()", "1.50000001", "exact 1.500000000000000005"},
		{"(0.00000000000000001).highBoundary(16)", "0.0000000000000001", "exact 0.000000000000000015"},
		{"(-0.00000000000000001).lowBoundary(16)", "-0.0000000000000001", "exact -0.000000000000000015"},
		{"(0.00).lowBoundary()", "-0.00500000", "fhirpath.js; exact -0.005"},
		{"(-0.0).lowBoundary()", "-0.05000000", "exact -0.05"},

		// Past both references' maxima, 16 and 8: the exact edges
		{"(1.1234567890123456).lowBoundary(17)", "1.12345678901234555", "exact"},
		{"(1.00000000000000000).highBoundary(18)", "1.000000000000000005", "exact"},
	} {
		result, err := fhirpath.Evaluate([]byte(boundaryObservation), tc.expr)
		if err != nil || len(result) != 1 || result[0].String() != tc.want {
			t.Errorf("%s = %v, %v, want %s (%s)", tc.expr, result, err, tc.want, tc.source)
		}
	}
}
