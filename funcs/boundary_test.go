package funcs

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

func TestLowBoundary(t *testing.T) {
	ctx := eval.NewContext([]byte(`{}`))
	fn, _ := Get("lowBoundary")

	t.Run("empty input", func(t *testing.T) {
		result, err := fn.Fn(ctx, types.Collection{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Empty() {
			t.Error("expected empty collection")
		}
	})

	// Date tests
	t.Run("date year precision", func(t *testing.T) {
		d, _ := types.NewDate("2023")
		result, err := fn.Fn(ctx, types.Collection{d}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result[0].String() != "2023-01-01" {
			t.Errorf("expected 2023-01-01, got %s", result[0].String())
		}
	})

	t.Run("date month precision", func(t *testing.T) {
		d, _ := types.NewDate("1970-06")
		result, err := fn.Fn(ctx, types.Collection{d}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result[0].String() != "1970-06-01" {
			t.Errorf("expected 1970-06-01, got %s", result[0].String())
		}
	})

	t.Run("date day precision", func(t *testing.T) {
		d, _ := types.NewDate("2023-06-15")
		result, err := fn.Fn(ctx, types.Collection{d}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result[0].String() != "2023-06-15" {
			t.Errorf("expected 2023-06-15, got %s", result[0].String())
		}
	})

	// DateTime tests
	t.Run("datetime day precision", func(t *testing.T) {
		dt, _ := types.NewDateTime("2010-10-10")
		result, err := fn.Fn(ctx, types.Collection{dt}, nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := "2010-10-10T00:00:00.000+14:00"
		if result[0].String() != expected {
			t.Errorf("expected %s, got %s", expected, result[0].String())
		}
	})

	t.Run("datetime year precision", func(t *testing.T) {
		dt, _ := types.NewDateTime("2010")
		result, err := fn.Fn(ctx, types.Collection{dt}, nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := "2010-01-01T00:00:00.000+14:00"
		if result[0].String() != expected {
			t.Errorf("expected %s, got %s", expected, result[0].String())
		}
	})

	t.Run("datetime with timezone", func(t *testing.T) {
		dt, _ := types.NewDateTime("2010-10-10T10:00:00+02:00")
		result, err := fn.Fn(ctx, types.Collection{dt}, nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := "2010-10-10T10:00:00.000+02:00"
		if result[0].String() != expected {
			t.Errorf("expected %s, got %s", expected, result[0].String())
		}
	})

	// Time tests
	t.Run("time hour precision", func(t *testing.T) {
		tm, _ := types.NewTime("12")
		result, err := fn.Fn(ctx, types.Collection{tm}, nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := "12:00:00.000"
		if result[0].String() != expected {
			t.Errorf("expected %s, got %s", expected, result[0].String())
		}
	})

	t.Run("time minute precision", func(t *testing.T) {
		tm, _ := types.NewTime("12:34")
		result, err := fn.Fn(ctx, types.Collection{tm}, nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := "12:34:00.000"
		if result[0].String() != expected {
			t.Errorf("expected %s, got %s", expected, result[0].String())
		}
	})

	// Decimal tests.
	//
	// Two precisions are at play and must not be conflated: the precision of the
	// *input* fixes how wide the interval is (1.0 stands for anything within
	// 0.05), while the argument only says how many digits to present.
	lowDecimal := func(t *testing.T, value string, args []interface{}) string {
		t.Helper()
		d, err := types.NewDecimal(value)
		if err != nil {
			t.Fatalf("parse %s: %v", value, err)
		}
		result, err := fn.Fn(ctx, types.Collection{d}, args)
		if err != nil {
			t.Fatal(err)
		}
		if result.Empty() {
			return ""
		}
		return result[0].String()
	}

	precisionArg := func(p int64) []interface{} {
		return []interface{}{types.Collection{types.NewInteger(p)}}
	}

	t.Run("output precision does not change the interval", func(t *testing.T) {
		// 1.0 - 0.05 = 0.95, presented with one digit and rounded down to stay
		// a lower bound
		if got := lowDecimal(t, "1.0", precisionArg(1)); got != "0.9" {
			t.Errorf("expected 0.9, got %s", got)
		}
	})

	t.Run("default precision pads to eight digits", func(t *testing.T) {
		// Per the specification, absent an argument the greatest precision of
		// the type is used: at least 8 for Decimal
		if got := lowDecimal(t, "1.0", nil); got != "0.95000000" {
			t.Errorf("expected 0.95000000, got %s", got)
		}
	})

	t.Run("a decimal without fractional digits still has a boundary", func(t *testing.T) {
		// The suite's 1.toDecimal().lowBoundary() is 0.50000000
		if got := lowDecimal(t, "1", nil); got != "0.50000000" {
			t.Errorf("expected 0.50000000, got %s", got)
		}
	})

	t.Run("specification examples", func(t *testing.T) {
		cases := []struct {
			value    string
			args     []interface{}
			expected string
		}{
			{"1.587", nil, "1.58650000"},
			{"1.587", precisionArg(6), "1.586500"},
			{"1.587", precisionArg(2), "1.58"},
			{"1.587", precisionArg(0), "1"},
			// A precision beyond what the implementation supports yields empty
			{"1.587", precisionArg(-1), ""},
			{"1.587", precisionArg(39), ""},
		}
		for _, tc := range cases {
			if got := lowDecimal(t, tc.value, tc.args); got != tc.expected {
				t.Errorf("%s.lowBoundary(%v): expected %q, got %q",
					tc.value, tc.args, tc.expected, got)
			}
		}
	})

	// Integer tests
	t.Run("integer is bounded as a decimal", func(t *testing.T) {
		// Defined on integer for language consistency: 1.lowBoundary() is 0.5
		result, err := fn.Fn(ctx, types.Collection{types.NewInteger(1)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.Empty() || result[0].String() != "0.50000000" {
			t.Errorf("expected 0.50000000, got %v", result)
		}
	})
}

func TestHighBoundary(t *testing.T) {
	ctx := eval.NewContext([]byte(`{}`))
	fn, _ := Get("highBoundary")

	t.Run("empty input", func(t *testing.T) {
		result, err := fn.Fn(ctx, types.Collection{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Empty() {
			t.Error("expected empty collection")
		}
	})

	// Date tests
	t.Run("date year precision", func(t *testing.T) {
		d, _ := types.NewDate("2023")
		result, err := fn.Fn(ctx, types.Collection{d}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result[0].String() != "2023-12-31" {
			t.Errorf("expected 2023-12-31, got %s", result[0].String())
		}
	})

	t.Run("date month precision", func(t *testing.T) {
		d, _ := types.NewDate("2023-02")
		result, err := fn.Fn(ctx, types.Collection{d}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result[0].String() != "2023-02-28" {
			t.Errorf("expected 2023-02-28, got %s", result[0].String())
		}
	})

	t.Run("date month precision leap year", func(t *testing.T) {
		d, _ := types.NewDate("2024-02")
		result, err := fn.Fn(ctx, types.Collection{d}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result[0].String() != "2024-02-29" {
			t.Errorf("expected 2024-02-29, got %s", result[0].String())
		}
	})

	// DateTime tests
	t.Run("datetime day precision", func(t *testing.T) {
		dt, _ := types.NewDateTime("2010-10-10")
		result, err := fn.Fn(ctx, types.Collection{dt}, nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := "2010-10-10T23:59:59.999-12:00"
		if result[0].String() != expected {
			t.Errorf("expected %s, got %s", expected, result[0].String())
		}
	})

	t.Run("datetime year precision", func(t *testing.T) {
		dt, _ := types.NewDateTime("2010")
		result, err := fn.Fn(ctx, types.Collection{dt}, nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := "2010-12-31T23:59:59.999-12:00"
		if result[0].String() != expected {
			t.Errorf("expected %s, got %s", expected, result[0].String())
		}
	})

	// Time tests
	t.Run("time minute precision", func(t *testing.T) {
		tm, _ := types.NewTime("12:34")
		result, err := fn.Fn(ctx, types.Collection{tm}, nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := "12:34:59.999"
		if result[0].String() != expected {
			t.Errorf("expected %s, got %s", expected, result[0].String())
		}
	})

	t.Run("time hour precision", func(t *testing.T) {
		tm, _ := types.NewTime("12")
		result, err := fn.Fn(ctx, types.Collection{tm}, nil)
		if err != nil {
			t.Fatal(err)
		}
		expected := "12:59:59.999"
		if result[0].String() != expected {
			t.Errorf("expected %s, got %s", expected, result[0].String())
		}
	})

	// Decimal tests
	t.Run("output precision does not change the interval", func(t *testing.T) {
		// 1.0 + 0.05 = 1.05, presented with one digit and rounded up to stay an
		// upper bound
		d, _ := types.NewDecimal("1.0")
		args := []interface{}{types.Collection{types.NewInteger(1)}}
		result, err := fn.Fn(ctx, types.Collection{d}, args)
		if err != nil {
			t.Fatal(err)
		}
		if result[0].String() != "1.1" {
			t.Errorf("expected 1.1, got %s", result[0].String())
		}
	})

	// Quantity tests
	t.Run("quantity with precision", func(t *testing.T) {
		val, _ := decimal.NewFromString("1.0")
		q := types.NewQuantityFromDecimal(val, "mg")
		args := []interface{}{types.Collection{types.NewInteger(1)}}
		result, err := fn.Fn(ctx, types.Collection{q}, args)
		if err != nil {
			t.Fatal(err)
		}
		qty := result[0].(types.Quantity)
		if qty.Value().String() != "1.1" {
			t.Errorf("expected 1.1 mg, got %s %s", qty.Value().String(), qty.Unit())
		}
		if qty.Unit() != "mg" {
			t.Errorf("expected unit mg, got %s", qty.Unit())
		}
	})
}
