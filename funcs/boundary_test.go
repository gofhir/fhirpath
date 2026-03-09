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
		expected := "2010-10-10T00:00:00.000"
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
		expected := "2010-01-01T00:00:00.000"
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

	// Decimal tests
	t.Run("decimal with precision 1", func(t *testing.T) {
		// 1.0.lowBoundary(1) = 1.0 - 0.05 = 0.95
		d, _ := types.NewDecimal("1.0")
		args := []interface{}{types.Collection{types.NewInteger(1)}}
		result, err := fn.Fn(ctx, types.Collection{d}, args)
		if err != nil {
			t.Fatal(err)
		}
		if result[0].String() != "0.95" {
			t.Errorf("expected 0.95, got %s", result[0].String())
		}
	})

	t.Run("decimal no precision returns empty", func(t *testing.T) {
		d, _ := types.NewDecimal("1.0")
		result, err := fn.Fn(ctx, types.Collection{d}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Empty() {
			t.Error("expected empty for decimal without precision arg")
		}
	})

	// Integer tests
	t.Run("integer returns itself", func(t *testing.T) {
		i := types.NewInteger(42)
		result, err := fn.Fn(ctx, types.Collection{i}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result[0].(types.Integer).Value() != 42 {
			t.Errorf("expected 42, got %d", result[0].(types.Integer).Value())
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
		expected := "2010-10-10T23:59:59.999"
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
		expected := "2010-12-31T23:59:59.999"
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
	t.Run("decimal with precision 1", func(t *testing.T) {
		// 1.0.highBoundary(1) = 1.0 + 0.05 = 1.05
		d, _ := types.NewDecimal("1.0")
		args := []interface{}{types.Collection{types.NewInteger(1)}}
		result, err := fn.Fn(ctx, types.Collection{d}, args)
		if err != nil {
			t.Fatal(err)
		}
		if result[0].String() != "1.05" {
			t.Errorf("expected 1.05, got %s", result[0].String())
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
		if qty.Value().String() != "1.05" {
			t.Errorf("expected 1.05 mg, got %s %s", qty.Value().String(), qty.Unit())
		}
		if qty.Unit() != "mg" {
			t.Errorf("expected unit mg, got %s", qty.Unit())
		}
	})
}
