package types

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// CompareDecimals orders two decimals exactly without rescaling one to the
// other's exponent where their magnitudes already decide (#232): 1e8227822
// against 1 rescaled is eight million digits.
func TestCompareDecimals(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want int
	}{
		{"1e1000", "1", 1},
		{"1", "1e1000", -1},
		{"-1e1000", "1", -1},
		{"-1e1000", "-1", -1},
		{"-1", "-1e1000", 1},
		{"1e-1001", "0", 1},
		{"-1e-1001", "0", -1},
		{"0", "0e2000", 0},
		{"0e-2000", "0e2000", 0},
		{"1e1000", "1e1000", 0},
		{"1e1000", "10e999", 0},
		{"1e1000", "0.1e1001", 0},
		{"1e1000", "1.0e1000", 0},
		{"1e1000", "2e1000", -1},
		{"9.99e999", "1e1000", -1},
		{"1e1001", "1e1000", 1},
		{"123456789012345678901234567890e990", "123456789012345678901234567889e990", 1},
		{"123456789012345678901234567890e990", "12345678901234567890123456789e991", 0},
		// the same number of digits before the point, decided by the digits
		{"99e998", "1e1000", -1},
		{"999e997", "1000e997", -1},
		{"-99e998", "-1e1000", 1},
		{"1.5", "1.50", 0},
		{"1.5", "1.4999999", 1},
		{"12", "12.0000001", -1},
		{"1e8227822", "1", 1},
		{"1e8227822", "1e8227822", 0},
		{"1e8227822", "2e8227822", -1},
		{"-1e8227822", "1e-8227822", -1},
		{"1e-8227822", "2e-8227822", -1},
		// negatives far apart: the larger magnitude is the smaller number
		{"-1e5000", "-1", -1},
		{"-1", "-1e5000", 1},
		{"-1e-3000", "-1", 1},
		{"-2e2500", "-1", -1},
		// the leading place counts the coefficient's digits: 10^3000 written
		// out against 2e2500
		{"1" + strings.Repeat("0", 3000), "2e2500", 1},
		{"-1" + strings.Repeat("0", 3000), "-2e2500", -1},
	} {
		a, b := decimal.RequireFromString(tt.a), decimal.RequireFromString(tt.b)
		start := time.Now()
		got := CompareDecimals(a, b)
		if took := time.Since(start); took > 50*time.Millisecond {
			t.Errorf("CompareDecimals(%s, %s) took %v: it rescaled", tt.a, tt.b, took)
		}
		if got != tt.want {
			t.Errorf("CompareDecimals(%s, %s) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

// FuzzCompareDecimals checks CompareDecimals against an exact rational
// comparison, on exponents small enough to expand, within the bound and past
// it on either side.
func FuzzCompareDecimals(f *testing.F) {
	for _, seed := range []struct {
		a    int64
		aExp int32
		b    int64
		bExp int32
	}{
		{1, 1000, 1, 0}, {1, 1001, 10, 1000}, {-1, 1000, 1, 0}, {0, 2000, 0, 0},
		{99, 998, 1, 1000}, {1, -1001, 0, 0}, {-25, -1002, -3, -1001},
		{123456789, 990, 12345678, 991}, {5, 0, 50, -1}, {7, -3, 7, 2},
		{-1, 2500, -1, 0}, {-3, -2500, -7, 100}, {-1, 2999, 5, -2999},
	} {
		f.Add(seed.a, seed.aExp, seed.b, seed.bExp)
	}
	f.Fuzz(func(t *testing.T, a int64, aExp int32, b int64, bExp int32) {
		aExp, bExp = aExp%3000, bExp%3000
		x, y := decimal.New(a, aExp), decimal.New(b, bExp)
		want := rationalOf(x).Cmp(rationalOf(y))
		if got := CompareDecimals(x, y); got != want {
			t.Errorf("CompareDecimals(%s, %s) = %d, want %d", x, y, got, want)
		}
		if got := CompareDecimals(y, x); got != -want {
			t.Errorf("CompareDecimals(%s, %s) = %d, want %d", y, x, got, -want)
		}
	})
}

// rationalOf is a decimal as an exact rational, by its coefficient and
// exponent, not through the decimal package's own Rat.
func rationalOf(d decimal.Decimal) *big.Rat {
	r := new(big.Rat).SetInt(d.Coefficient())
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs32(d.Exponent()))), nil)
	if d.Exponent() >= 0 {
		return r.Mul(r, new(big.Rat).SetInt(scale))
	}
	return r.Quo(r, new(big.Rat).SetInt(scale))
}

func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// Equivalence rounds both to the less precise operand's places without
// writing either out: 1e-5000000 against 0 rounds to zero places, which
// rescaling would do by dividing by ten to the five millionth.
func TestEquivalenceOfADecimalBeyondTheBoundDoesNotRescale(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want bool
	}{
		{"1e-5000000", "0", true},
		{"-1e-5000000", "0", true},
		{"1e-5000000", "1", false},
		{"1e5000000", "1e5000000", true},
		{"1e5000000", "1", false},
		{"0.004", "0.00", true},
		{"0.005", "0.01", true},
	} {
		a, aerr := ReadDecimal(tt.a)
		b, berr := ReadDecimal(tt.b)
		if aerr != nil || berr != nil {
			t.Fatal(aerr, berr)
		}
		start := time.Now()
		got := a.Equivalent(b)
		if took := time.Since(start); took > 50*time.Millisecond {
			t.Errorf("%s ~ %s took %v: it rescaled", tt.a, tt.b, took)
		}
		if got != tt.want {
			t.Errorf("%s ~ %s = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

// Two FHIR objects' numbers are decided where they are, one beyond the bound
// too: 1e1000 and 10e999 are alike, 1e1000 and 2e1000 are not, without
// falling back to reading both objects whole.
func TestObjectNumbersBeyondTheBoundAreDecided(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		want tokenVerdict
	}{
		{"1e1000", "10e999", tokensAlike},
		{"1e1000", "2e1000", tokensUnequal},
		{"-1e-1001", "-0.1e-1000", tokensAlike},
		{"1e5000000", "1", tokensUnequal},
	} {
		if _, _, got := compareNumbers([]byte(tt.a), 0, []byte(tt.b), 0); got != tt.want {
			t.Errorf("compareNumbers(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

// A coefficient written out far beyond the bound is told beyond by its bits,
// without counting its digits, which costs a power of ten each guard: ten to
// the five millionth written out, against the bound's thousand digits.
func TestACoefficientFarBeyondTheBoundIsToldWithoutCounting(t *testing.T) {
	coefficient := new(big.Int).Exp(big.NewInt(10), big.NewInt(5000000), nil)
	for _, d := range []decimal.Decimal{
		decimal.NewFromBigInt(coefficient, 0),
		decimal.NewFromBigInt(new(big.Int).Neg(coefficient), -900),
	} {
		start := time.Now()
		in := InDecimalRange(d)
		if took := time.Since(start); took > 50*time.Millisecond {
			t.Errorf("InDecimalRange of a %d-bit coefficient took %v: it counted the digits", d.Coefficient().BitLen(), took)
		}
		if in {
			t.Errorf("InDecimalRange of a %d-bit coefficient = true, want false", d.Coefficient().BitLen())
		}
	}
	// the shortest and the longest coefficient of each length around the
	// bound, against the digits counted
	ten := big.NewInt(10)
	for n := int64(1); n <= 1100; n++ {
		smallest := new(big.Int).Exp(ten, big.NewInt(n-1), nil)
		largest := new(big.Int).Sub(new(big.Int).Mul(smallest, ten), big.NewInt(1))
		for _, c := range []*big.Int{smallest, largest} {
			for _, exp := range []int32{0, 5, -400} {
				want := int64(exp)+int64(len(c.Text(10))) <= DecimalDigits
				if got := InDecimalRange(decimal.NewFromBigInt(c, exp)); got != want {
					t.Errorf("InDecimalRange(%d digits e%d) = %v, want %v", n, exp, got, want)
				}
			}
		}
	}
	// at the bound the digits decide, counted exactly
	for _, tt := range []struct {
		digits int
		exp    int32
		want   bool
	}{
		{1000, 0, true}, {1001, 0, false}, {999, 1, true}, {1000, 1, false},
		{2000, -1000, true}, {2001, -1000, false},
	} {
		c, _ := new(big.Int).SetString(strings.Repeat("9", tt.digits), 10)
		if got := InDecimalRange(decimal.NewFromBigInt(c, tt.exp)); got != tt.want {
			t.Errorf("InDecimalRange(%d nines e%d) = %v, want %v", tt.digits, tt.exp, got, tt.want)
		}
	}
}

// A decimal written with places past the bound that are all zeros is read at
// the scale of the bound, its value unchanged; one with a digit there that is
// no zero, or beyond the bound before the point, is held as written. The
// zeros are counted on the text, so 200000 of them cost no division.
func TestADecimalPastTheBoundByZerosIsReadAtTheBound(t *testing.T) {
	zeros := strings.Repeat("0", 1001)
	for _, tt := range []struct {
		written, value string
		exponent       int32
	}{
		{"1." + zeros, "1", -DecimalDigits},
		{"-1." + zeros, "-1", -DecimalDigits},
		{"100.0e-1001", "1e-999", -DecimalDigits},
		{"1" + zeros + "e-1001", "1", -DecimalDigits},
		{"1." + strings.Repeat("0", 1000), "1", -DecimalDigits},
		{"1." + strings.Repeat("0", 1000) + "5", "1." + strings.Repeat("0", 1000) + "5", -1001},
		{"1." + strings.Repeat("0", 1000) + "50", "1." + strings.Repeat("0", 1000) + "5", -1002},
		{"10e-1001", "1e-1000", -DecimalDigits},
		{"10e-1002", "1e-1001", -1002},
		{"1e1000", "1e1000", 1000},
		{"0e-2000", "0", -DecimalDigits},
	} {
		d, err := ReadDecimal(tt.written)
		if err != nil {
			t.Fatalf("%.40s: %v", tt.written, err)
		}
		want := decimal.RequireFromString(tt.value)
		if !d.Value().Equal(want) || d.Value().Exponent() != tt.exponent {
			t.Errorf("%.40s read as %.40s at exponent %d, want %.40s at %d", tt.written, d.Value().String(), d.Value().Exponent(), tt.value, tt.exponent)
		}
		if d.String() != tt.written {
			t.Errorf("%.40s written back as %.40s", tt.written, d.String())
		}
	}

	written := "1." + strings.Repeat("0", 200000)
	coefficient, _ := new(big.Int).SetString("1"+strings.Repeat("0", 200000), 10)
	held := decimal.NewFromBigInt(coefficient, -200000)
	start := time.Now()
	within := withinScale(held, written)
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("200000 zeros past the bound took %v to read within it", elapsed)
	}
	if within.Exponent() != -DecimalDigits || !InDecimalRange(within) {
		t.Errorf("200000 zeros past the bound read at exponent %d", within.Exponent())
	}
}
