package ucum

import (
	"math/big"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
)

func TestFactor(t *testing.T) {
	factor, err := Factor("L", "mL")
	if err != nil || factor.Cmp(big.NewRat(1000, 1)) != 0 {
		t.Fatalf("Factor(L, mL) = %v, %v; want 1000", factor, err)
	}
	// Found once: the same factor comes back
	again, err := Factor("L", "mL")
	if err != nil || again != factor {
		t.Errorf("Factor(L, mL) again = %p, %v; want the factor found before, %p", again, err, factor)
	}
	// The pair is ordered
	if inverse, err := Factor("mL", "L"); err != nil || inverse.Cmp(big.NewRat(1, 1000)) != 0 {
		t.Errorf("Factor(mL, L) = %v, %v; want 1/1000", inverse, err)
	}
}

func TestFactorOfScalesWithoutOne(t *testing.T) {
	// An offset, prefixed or not, and a logarithm have no factor; NotLinear
	// says so, every time
	for _, pair := range [][2]string{{"Cel", "K"}, {"mCel", "K"}, {"[degF]", "Cel"}, {"[pH]", "mol/L"}} {
		for range 2 {
			if _, err := Factor(pair[0], pair[1]); !NotLinear(err) {
				t.Errorf("Factor(%s, %s) error = %v, want one NotLinear reports", pair[0], pair[1], err)
			}
		}
	}
	// Units that do not convert are another error
	if _, err := Factor("m", "s"); err == nil || NotLinear(err) {
		t.Errorf("Factor(m, s) error = %v, want one NotLinear does not report", err)
	}
}

func TestFactorKeepsPairsApart(t *testing.T) {
	// "a" with "b0" and "a0" with "b" would share a key without a separator
	if _, err := Factor("m", "km"); err != nil {
		t.Fatal(err)
	}
	if factor, err := Factor("mk", "m"); err == nil {
		t.Errorf("Factor(mk, m) = %v, want an error for an unknown unit", factor)
	}
}

func TestFactorBound(t *testing.T) {
	var wg sync.WaitGroup
	for i := range 64 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 100 {
				// Only the count of pairs kept matters here
				_, _ = Factor("m", decimal.NewFromInt(int64(i*100+j)).String()+".m")
			}
		}()
	}
	wg.Wait()
	if n := factorCount.Load(); n > maxFactors {
		t.Errorf("%d pairs kept, more than %d", n, maxFactors)
	}
}

func TestScaleAndUnscale(t *testing.T) {
	third := big.NewRat(1, 3)
	if got, err := Scale(decimal.NewFromInt(3), third); err != nil || !got.Equal(decimal.NewFromInt(1)) {
		t.Errorf("Scale(3, 1/3) = %v, %v; want 1", got, err)
	}
	if got, err := Unscale(decimal.NewFromInt(1), third); err != nil || !got.Equal(decimal.NewFromInt(3)) {
		t.Errorf("Unscale(1, 1/3) = %v, %v; want 3", got, err)
	}
	if _, err := Unscale(decimal.NewFromInt(1), new(big.Rat)); err == nil {
		t.Error("Unscale by zero: want an error")
	}
	// The factor given is not changed
	if third.Cmp(big.NewRat(1, 3)) != 0 {
		t.Errorf("factor changed to %v", third)
	}
}

func TestConvertWithoutFiniteValue(t *testing.T) {
	// Zero on a logarithmic scale is minus infinity
	if got, err := Convert(decimal.Zero, "1", "B"); err == nil {
		t.Errorf("Convert(0, 1, B) = %v, want an error", got)
	}
}
