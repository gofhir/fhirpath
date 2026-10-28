package ucum

import (
	"math/big"
	"testing"

	"github.com/shopspring/decimal"
)

// fromRatReference is how fromRat read a rational before #237: written to
// exactDigits places by big.Rat, rounding half away from zero, then read
// back at its natural scale through the library under its defaults.
func fromRatReference(r *big.Rat) decimal.Decimal {
	if r.IsInt() {
		return decimal.NewFromBigInt(r.Num(), 0)
	}
	padded, err := decimal.NewFromString(r.FloatString(exactDigits))
	if err != nil {
		panic(err)
	}
	natural, err := decimal.NewFromString(padded.String())
	if err != nil {
		panic(err)
	}
	return natural
}

func sameDecimal(a, b decimal.Decimal) bool {
	return a.Coefficient().Cmp(b.Coefficient()) == 0 && a.Exponent() == b.Exponent()
}

func TestFromRatReadsAsBefore(t *testing.T) {
	for _, text := range []string{
		"0", "1", "-1", "1000", "1/3", "-1/3", "2/3", "1/8", "-1/8", "1/2", "-1/2",
		"127/50", "50/127", "1/1000000", "3/7", "22/7", "-22/7",
		"1/2000000000000000000000000000000000000",
		"-1/3000000000000000000000000000000000000",
		"5/100000000000000000000000000000000000", // a half at the last place
		"-5/100000000000000000000000000000000000",
		"123456789012345678901234567890/7",
	} {
		r, ok := new(big.Rat).SetString(text)
		if !ok {
			t.Fatalf("bad rational %s", text)
		}
		want := fromRatReference(r)
		got, err := fromRat(r)
		if err != nil || !sameDecimal(got, want) {
			t.Errorf("fromRat(%s) = %se%d, %v, want %se%d", text, got.Coefficient(), got.Exponent(), err, want.Coefficient(), want.Exponent())
		}
	}
}

// toRat reads a decimal as it read its String() before #237, under the
// library's defaults.
func TestToRatReadsAsBefore(t *testing.T) {
	for _, d := range []decimal.Decimal{decimal.New(254, -2), decimal.New(-1, -30), decimal.New(15, 3), decimal.New(-7, 40), {}} {
		want, ok := new(big.Rat).SetString(d.String())
		if !ok {
			t.Fatalf("big.Rat refuses %s", d)
		}
		if got := toRat(d); got.Cmp(want) != 0 {
			t.Errorf("toRat(%s) = %s, want %s", d, got, want)
		}
	}
}

func FuzzFromRat(f *testing.F) {
	f.Add(int64(1), int64(3))
	f.Add(int64(-5), int64(100))
	f.Add(int64(254), int64(100))
	f.Fuzz(func(t *testing.T, num, denom int64) {
		if denom == 0 {
			return
		}
		r := big.NewRat(num, denom)
		want := fromRatReference(r)
		got, err := fromRat(r)
		if err != nil || !sameDecimal(got, want) {
			t.Fatalf("fromRat(%s) = %se%d, %v, want %se%d", r, got.Coefficient(), got.Exponent(), err, want.Coefficient(), want.Exponent())
		}
	})
}
