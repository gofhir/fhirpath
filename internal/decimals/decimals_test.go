package decimals

import (
	"math/big"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// The reference for both functions is shopspring/decimal itself under its
// default settings: String() and a re-read of it, which is what the engine
// did before #237, and what v1.4.0, with no settings, always did.

func reference(d decimal.Decimal) (string, decimal.Decimal) {
	text := d.String()
	natural, err := decimal.NewFromString(text)
	if err != nil {
		panic(err)
	}
	return text, natural
}

var formatCases = []decimal.Decimal{
	{},
	decimal.New(0, -3),
	decimal.New(0, 3),
	decimal.New(1, 0),
	decimal.New(-1, 0),
	decimal.New(1500, -1),
	decimal.New(1500, 0),
	decimal.New(15, 2),
	decimal.New(-15, 2),
	decimal.New(25, -1),
	decimal.New(250000, -5),
	decimal.New(-250000, -5),
	decimal.New(1, -20),
	decimal.New(-100, -20),
	decimal.New(10, -1),
	decimal.New(1000, -3),
	decimal.New(5, -1),
	decimal.New(-5, -1),
	decimal.New(123456789, -4),
	decimal.New(1, 30),
	decimal.NewFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(40), nil), -45),
	decimal.New(7, -1001),
	decimal.New(7, 1001),
}

func TestFormatWritesAsStringDoesUnderTheDefaults(t *testing.T) {
	for _, d := range formatCases {
		want, _ := reference(d)
		if got := Format(d); got != want {
			t.Errorf("Format(%se%d) = %s, want %s", d.Coefficient(), d.Exponent(), got, want)
		}
	}
}

func TestNaturalReadsAsAReadOfStringDoes(t *testing.T) {
	for _, d := range formatCases {
		_, want := reference(d)
		got := Natural(d)
		if got.Coefficient().Cmp(want.Coefficient()) != 0 || got.Exponent() != want.Exponent() {
			t.Errorf("Natural(%se%d) = %se%d, want %se%d", d.Coefficient(), d.Exponent(), got.Coefficient(), got.Exponent(), want.Coefficient(), want.Exponent())
		}
	}
}

// Neither reads a setting of the library: with every one changed, both give
// what they give under the defaults. The settings are restored before the
// test returns; no test of this package runs in parallel.
func TestFormatAndNaturalReadNoSetting(t *testing.T) {
	trim, scientific := decimal.TrimTrailingZeros, decimal.UseScientificNotation
	decimal.TrimTrailingZeros, decimal.UseScientificNotation = false, true
	defer func() { decimal.TrimTrailingZeros, decimal.UseScientificNotation = trim, scientific }()
	for _, d := range formatCases {
		decimal.TrimTrailingZeros, decimal.UseScientificNotation = trim, scientific
		wantText, wantNatural := reference(d)
		decimal.TrimTrailingZeros, decimal.UseScientificNotation = false, true
		if got := Format(d); got != wantText {
			t.Errorf("Format(%se%d) = %s with the settings changed, want %s", d.Coefficient(), d.Exponent(), got, wantText)
		}
		if got := Natural(d); !got.Equal(wantNatural) || got.Exponent() != wantNatural.Exponent() {
			t.Errorf("Natural(%se%d) = %se%d with the settings changed", d.Coefficient(), d.Exponent(), got.Coefficient(), got.Exponent())
		}
	}
}

func FuzzFormatAndNatural(f *testing.F) {
	f.Add("0", int32(0))
	f.Add("1500", int32(-1))
	f.Add("-25", int32(3))
	f.Add("100000000000000000000", int32(-25))
	f.Fuzz(func(t *testing.T, digits string, exponent int32) {
		coefficient, ok := new(big.Int).SetString(digits, 10)
		if !ok || len(digits) > 200 || exponent > 200 || exponent < -200 {
			return
		}
		d := decimal.NewFromBigInt(coefficient, exponent)
		wantText, wantNatural := reference(d)
		if got := Format(d); got != wantText {
			t.Fatalf("Format(%se%d) = %s, want %s", coefficient, exponent, got, wantText)
		}
		got := Natural(d)
		if got.Coefficient().Cmp(wantNatural.Coefficient()) != 0 || got.Exponent() != wantNatural.Exponent() {
			t.Fatalf("Natural(%se%d) = %se%d, want %se%d", coefficient, exponent, got.Coefficient(), got.Exponent(), wantNatural.Coefficient(), wantNatural.Exponent())
		}
		if strings.ContainsAny(Format(d), "eE") {
			t.Fatalf("Format(%se%d) wrote an exponent", coefficient, exponent)
		}
	})
}
