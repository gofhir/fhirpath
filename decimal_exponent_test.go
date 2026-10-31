package fhirpath_test

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// writtenDecimal is a decimal as FHIR's JSON writes it, with the parts its
// precision is read from: the mantissa's digits after the point and the
// exponent, counted apart from the engine.
type writtenDecimal struct {
	text     string
	places   int64 // digits after the mantissa's point
	exponent int64
}

// lastPlace is the place of the last written digit: 2 for 1.50, -1 for
// 1.5e2, 17 for 1E-17.
func (w writtenDecimal) lastPlace() int64 { return w.places - w.exponent }

// randomWrittenDecimal writes a decimal in FHIR's form,
// -?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?, with an exponent in most
// draws, trailing zeros, either sign and either case.
func randomWrittenDecimal(r *rand.Rand) writtenDecimal {
	var b strings.Builder
	if r.IntN(2) == 0 {
		b.WriteByte('-')
	}
	if r.IntN(4) == 0 {
		b.WriteByte('0')
	} else {
		b.WriteByte(byte('1' + r.IntN(9)))
		for range r.IntN(6) {
			b.WriteByte(byte('0' + r.IntN(10)))
		}
	}
	places := r.IntN(7)
	if places > 0 {
		b.WriteByte('.')
		for range places {
			if r.IntN(3) == 0 {
				b.WriteByte('0')
			} else {
				b.WriteByte(byte('0' + r.IntN(10)))
			}
		}
	}
	exponent := 0
	if r.IntN(5) > 0 {
		exponent = r.IntN(51) - 25
		b.WriteString([]string{"e", "E"}[r.IntN(2)])
		if exponent >= 0 && r.IntN(3) == 0 {
			b.WriteByte('+')
		}
		if exponent < 0 {
			b.WriteByte('-')
		}
		// FHIR's [0-9]+ and JSON take leading zeros: e-08, E+002
		b.WriteString(strings.Repeat("0", r.IntN(3)))
		b.WriteString(strconv.Itoa(abs(exponent)))
	}
	return writtenDecimal{b.String(), int64(places), int64(exponent)}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// exactBoundaryAt is lowBoundary or highBoundary of value written to its
// lastPlace-th place, at q places: the half unit of that place, 5 at the place
// after it, taken away or added, then rounded outward. A negative lastPlace
// is a digit left of the point, so 1.5e2 stands for [145, 155], as FHIR
// search's rule has it: "The number of significant digits of the implicit
// range is the number of digits specified" (R5, 1e2: "[50 ... 150)").
func exactBoundaryAt(value *big.Rat, lastPlace int64, q int, low bool) string {
	ten := big.NewInt(10)
	half := new(big.Rat)
	if e := lastPlace + 1; e >= 0 {
		half.SetFrac(big.NewInt(5), new(big.Int).Exp(ten, big.NewInt(e), nil))
	} else {
		half.SetInt(new(big.Int).Mul(big.NewInt(5), new(big.Int).Exp(ten, big.NewInt(-e), nil)))
	}
	scale := new(big.Rat).SetInt(new(big.Int).Exp(ten, big.NewInt(int64(q)), nil))
	bound := new(big.Rat).Set(value)
	if low {
		bound.Sub(bound, half)
	} else {
		bound.Add(bound, half)
	}
	bound.Mul(bound, scale)
	var n *big.Int
	if low {
		n, _ = new(big.Int).DivMod(bound.Num(), bound.Denom(), new(big.Int))
	} else {
		n, _ = new(big.Int).DivMod(new(big.Int).Neg(bound.Num()), bound.Denom(), new(big.Int))
		n.Neg(n)
	}
	return fixedPlaces(n, q)
}

// quantityObservation holds value as a resource writes it, a Quantity's value.
func quantityObservation(value string) []byte {
	return []byte(`{"resourceType":"Observation","id":"o","status":"final","code":{"text":"x"},` +
		`"valueQuantity":{"value":` + value + `,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}`)
}

// A decimal's precision is the places after the point of the value written,
// its exponent applied, never below 0 (#246): FHIRPath 3.0.0, precision(),
// "the number of digits of precision after the decimal place in the input
// value". Its boundaries are those of its last written digit, through the
// Decimal and through the Quantity that holds it.
func TestDecimalWithAnExponentIsBoundedByItsLastDigit(t *testing.T) {
	r := rand.New(rand.NewPCG(246, 1))
	for range 1500 {
		w := randomWrittenDecimal(r)
		resource := quantityObservation(w.text)
		value, ok := new(big.Rat).SetString(w.text)
		if !ok {
			t.Fatalf("%s: not a number", w.text)
		}

		wantPrecision := fmt.Sprint(max(0, w.lastPlace()))
		result, err := fhirpath.Evaluate(resource, "Observation.value.value.precision()")
		if err != nil || len(result) != 1 || result[0].String() != wantPrecision {
			t.Errorf("%s: precision() = %v, %v, want %s", w.text, result, err, wantPrecision)
		}

		q, arg := randomPrecision(r)
		for _, low := range []bool{true, false} {
			name := "highBoundary"
			if low {
				name = "lowBoundary"
			}
			want := exactBoundaryAt(value, w.lastPlace(), q, low)
			expr := fmt.Sprintf("Observation.value.value.%s(%s)", name, arg)
			result, err := fhirpath.Evaluate(resource, expr)
			if err != nil || len(result) != 1 || result[0].String() != want {
				t.Errorf("%s: %s = %v, %v, want %s", w.text, expr, result, err, want)
			}
			expr = fmt.Sprintf("Observation.value.%s(%s)", name, arg)
			result, err = fhirpath.Evaluate(resource, expr)
			if err != nil || len(result) != 1 || result[0].String() != want+" 'mg'" {
				t.Errorf("%s: %s = %v, %v, want %s 'mg'", w.text, expr, result, err, want)
			}
		}
	}
}

// Without an exponent, the precision is the characters after the point, as
// before #246.
func TestDecimalWithoutAnExponentKeepsItsPrecision(t *testing.T) {
	r := rand.New(rand.NewPCG(246, 2))
	for range 500 {
		written := randomDecimal(r)
		_, fraction, _ := strings.Cut(written, ".")
		want := fmt.Sprint(len(fraction))
		result, err := fhirpath.Evaluate(quantityObservation(written), "Observation.value.value.precision()")
		if err != nil || len(result) != 1 || result[0].String() != want {
			t.Errorf("%s: precision() = %v, %v, want %s", written, result, err, want)
		}
	}
}

// The rows, each from its source: FHIRPath 3.0.0's examples, FHIR search's
// ranges, fhirpath.js 5.2.0 where it keeps the text's places (battery
// decimal-exponent), and the exact value elsewhere. A value past the bound
// answers precision() from its text and bounds nothing, and a zero written
// past it computes as 0 does (CONFORMANCE.md, "A decimal a resource holds
// beyond the bound is read").
func TestDecimalExponentPrecisionRows(t *testing.T) {
	ones := "1." + strings.Repeat("0", 1001)
	for _, tc := range []struct{ value, expr, want, source string }{
		{"1.58700", "precision()", "5", "3.0.0 example"},
		{"100", "precision()", "0", "3.0.0 example"},
		{"100", "lowBoundary()", "99.50000000", "FHIR search, 100: [99.5 ... 100.5)"},
		{"1.00e2", "lowBoundary()", "99.50000000", "FHIR search: 100 and 1.00e2 alike"},
		{"1.00e2", "highBoundary()", "100.50000000", "FHIR search: 100 and 1.00e2 alike"},
		{"1e2", "lowBoundary()", "50.00000000", "FHIR search R5, 1e2: [50 ... 150)"},
		{"1e2", "highBoundary()", "150.00000000", "FHIR search R5, 1e2: [50 ... 150)"},
		{"1e2", "precision()", "0", "3.0.0: digits after the decimal place"},
		{"1.5e2", "lowBoundary()", "145.00000000", "exact, two significant digits"},
		{"1.5e2", "precision()", "0", "3.0.0: digits after the decimal place"},
		{"1.234e1", "lowBoundary()", "12.33500000", "fhirpath.js"},
		{"15e-1", "lowBoundary()", "1.45000000", "fhirpath.js"},
		{"1.5e-2", "lowBoundary()", "0.01450000", "fhirpath.js"},
		{"1E-17", "precision()", "17", "exact"},
		{"1E-17", "lowBoundary()", "0.00000000", "fhirpath.js"},
		{"1E-17", "highBoundary()", "0.00000001", "fhirpath.js (1e-8)"},
		{"1.0e0", "lowBoundary()", "0.95000000", "exact; Java 0.950000000"},
		{"1.50e-1", "precision()", "3", "exact"},
		{"0e-3", "lowBoundary()", "-0.00050000", "exact"},
		{"0e-2000", "precision()", "2000", "exact"},
		{"0e-2000", "lowBoundary()", "-0.00000001", "exact"},
		{"0e-2147483648", "highBoundary()", "0.00000001", "exact"},
		{"1e-08", "precision()", "8", "exact"},
		{"1e-08", "highBoundary()", "0.00000002", "fhirpath.js (2e-8)"},
		{"2.50E-007", "precision()", "9", "exact"},
		{"2.50E-007", "lowBoundary()", "0.00000024", "fhirpath.js (2.4e-7)"},
		{"1.5e+02", "lowBoundary()", "145.00000000", "exact, as 1.5e2"},
		{"0e2000", "precision()", "0", "its text"},
		{"0e2000", "lowBoundary()", "-0.50000000", "computes as 0 does (CONFORMANCE.md)"},
		{"1e-1001", "precision()", "1001", "its text"},
		{"1e-1001", "lowBoundary()", "", "past the bound"},
		{"1e-2147483648", "precision()", "2147483648", "its text"},
		{ones, "precision()", "1001", "its text"},
	} {
		expr := "Observation.value.value." + tc.expr
		result, err := fhirpath.Evaluate(quantityObservation(tc.value), expr)
		got := ""
		if len(result) == 1 {
			got = result[0].String()
		}
		if err != nil || len(result) > 1 || got != tc.want {
			t.Errorf("%.20s: %s = %v, %v, want %q (%s)", tc.value, tc.expr, result, err, tc.want, tc.source)
		}
	}
}

// A value whose text is built through the Go API is read as one from a
// resource.
func TestImplicitPrecisionAppliesTheExponent(t *testing.T) {
	for text, want := range map[string]int{"1.50": 2, "1.5e2": 0, "1E-17": 17, "15e-1": 1, "1.0e0": 1, "0e0": 0} {
		d, err := types.NewDecimal(text)
		if err != nil || d.ImplicitPrecision() != want {
			t.Errorf("NewDecimal(%q).ImplicitPrecision() = %d, %v, want %d", text, d.ImplicitPrecision(), err, want)
		}
	}
}

// A computed value's precision is the places its result carries, as the HL7
// validator answers (battery decimal-exponent's probe rows).
func TestComputedDecimalPrecision(t *testing.T) {
	for expr, want := range map[string]string{
		"(1.587 + 0).precision()":  "3",
		"(1.50 * 2).precision()":   "2",
		"(-1.587).precision()":     "3",
		"(1.5 + 0.25).precision()": "2",
	} {
		result, err := fhirpath.Evaluate([]byte(`{}`), expr)
		if err != nil || len(result) != 1 || result[0].String() != want {
			t.Errorf("%s = %v, %v, want %s (Java)", expr, result, err, want)
		}
	}
}

// A computed value is bounded at the units however its result is held: a
// positive exponent is the computation's, (100.0).power(2) held as 1e4, not
// a digit written left of the point. fhirpath.js 5.2.0 answers each Decimal
// row (battery decimal-exponent); it bounds no Quantity, and a Quantity row
// is bounded as its value is.
func TestComputedDecimalBoundariesStayAtTheUnits(t *testing.T) {
	resource := quantityObservation("1.5e2")
	for expr, want := range map[string]string{
		"(100.0).power(2).lowBoundary()":                                        "9999.50000000",
		"(10.0).power(2).lowBoundary()":                                         "99.50000000",
		"(22500.0).sqrt().lowBoundary()":                                        "149.50000000",
		"(15).round(-1).lowBoundary()":                                          "19.50000000",
		"(Observation.value.value * 2).lowBoundary()":                           "299.50000000",
		"((100.0).power(2) * 1 'mg').lowBoundary()":                             "9999.50000000 'mg'",
		"(100 'm' * 100 'm').lowBoundary()":                                     "9999.50000000 'm2'",
		"(1000000 'm' / 0.01 's').lowBoundary()":                                "99999999.50000000 'm.s-1'",
		"(10 'B').toQuantity('dB').lowBoundary()":                               "99.50000000 'dB'",
		"(Observation.value * 1 'm').lowBoundary()":                             "149.50000000 'mg.m'",
		"(Observation.value / 1 'm').lowBoundary()":                             "149.50000000 'mg.m-1'",
		"(2 'cm' * 50 'm').lowBoundary()":                                       "9999.50000000 'cm2'",
		"(Observation.value * 2 'mg').lowBoundary()":                            "299.50000000 'mg2'",
		"((10.0).power(2) * 1 'mg').lowBoundary()":                              "99.50000000 'mg'",
		"(Observation.value * 2).lowBoundary()":                                 "299.50000000 'mg'",
		"(Observation.value.value + Observation.value.value).lowBoundary()":     "299.50000000",
		"(Observation.value.value * 3 - Observation.value.value).lowBoundary()": "299.50000000",
		"(Observation.value + Observation.value).lowBoundary()":                 "299.50000000 'mg'",
		"(Observation.value * 3 - Observation.value).lowBoundary()":             "299.50000000 'mg'",
		"(Observation.value.value div 1).lowBoundary()":                         "149.50000000",
		"(Observation.value.value - -Observation.value.value).lowBoundary()":    "299.50000000",
		"(Observation.value - -Observation.value).lowBoundary()":                "299.50000000 'mg'",
		"(Observation.value.value mod Observation.value.value).lowBoundary()":   "-0.50000000",
		"(Observation.value.value mod 7).lowBoundary()":                         "2.50000000",
		"(Observation.value.value * 2).precision()":                             "0",
	} {
		result, err := fhirpath.Evaluate(resource, expr)
		if err != nil || len(result) != 1 || result[0].String() != want {
			t.Errorf("%s = %v, %v, want %s (fhirpath.js)", expr, result, err, want)
		}
	}
}

// An operation that keeps the value keeps where its last digit is, so a
// resource's 1.5e2 stands for [145, 155] (FHIR R5 search, "1.5e2 ... [145 ...
// 155)") negated, made absolute or converted, as a Decimal and as a Quantity.
func TestWrittenExponentSurvivesValuePreservingOperations(t *testing.T) {
	resource := quantityObservation("1.5e2")
	for expr, want := range map[string]string{
		"Observation.value.value.lowBoundary()":              "145.00000000",
		"Observation.value.value.highBoundary()":             "155.00000000",
		"(-Observation.value.value).lowBoundary()":           "-155.00000000",
		"(-Observation.value.value).highBoundary()":          "-145.00000000",
		"Observation.value.value.abs().lowBoundary()":        "145.00000000",
		"(-Observation.value.value).abs().lowBoundary()":     "145.00000000",
		"Observation.value.lowBoundary()":                    "145.00000000 'mg'",
		"(-Observation.value).lowBoundary()":                 "-155.00000000 'mg'",
		"Observation.value.abs().lowBoundary()":              "145.00000000 'mg'",
		"Observation.value.toQuantity().lowBoundary()":       "145.00000000 'mg'",
		"Observation.value.value.toQuantity().lowBoundary()": "145.00000000 '1'",
		"Observation.value.value.precision()":                "0",
		"(-Observation.value.value).precision()":             "0",
	} {
		result, err := fhirpath.Evaluate(resource, expr)
		if err != nil || len(result) != 1 || result[0].String() != want {
			t.Errorf("%s = %v, %v, want %s", expr, result, err, want)
		}
	}
}
