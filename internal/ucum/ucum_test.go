package ucum

import (
	"math/big"
	"sync"
	"testing"

	"github.com/shopspring/decimal"
)

// forgetFactors empties what Factor and Step remember, so that a test sees
// neither what another found nor the bound it filled.
func forgetFactors(t *testing.T) {
	t.Helper()
	forget := func() {
		factors.Range(func(key, _ any) bool {
			factors.Delete(key)
			return true
		})
		factorCount.Store(0)
	}
	forget()
	t.Cleanup(forget)
}

func TestFactor(t *testing.T) {
	forgetFactors(t)
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
	forgetFactors(t)
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

func TestStep(t *testing.T) {
	forgetFactors(t)
	// A degree Celsius is a kelvin; a degree Fahrenheit 5/9 of one
	if step, err := Step("Cel", "K"); err != nil || !step.Equal(decimal.NewFromInt(1)) {
		t.Errorf("Step(Cel, K) = %v, %v; want 1", step, err)
	}
	if step, err := Step("[degF]", "K"); err != nil || step.Sub(decimal.RequireFromString("0.5555555555")).Abs().GreaterThan(decimal.RequireFromString("0.0000000001")) {
		t.Errorf("Step([degF], K) = %v, %v; want 5/9", step, err)
	}
	// Found once
	before := factorCount.Load()
	if _, err := Step("Cel", "K"); err != nil || factorCount.Load() != before {
		t.Errorf("Step(Cel, K) again: %v, %d pairs kept, want %d", err, factorCount.Load(), before)
	}
	// A logarithm has no step, either way round
	for _, pair := range [][2]string{{"B", "1"}, {"1", "B"}, {"[pH]", "mol/L"}} {
		if step, err := Step(pair[0], pair[1]); err == nil {
			t.Errorf("Step(%s, %s) = %v, want an error", pair[0], pair[1], step)
		}
	}
}
