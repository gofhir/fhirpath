package types

import (
	"errors"
	"math"
	"testing"

	"github.com/shopspring/decimal"
)

// The exact operations answer within the int64 range, its ends included, and
// report ErrIntegerRange past it, where Go's arithmetic wraps.
func TestIntegerExactOperations(t *testing.T) {
	const maxInt, minInt = math.MaxInt64, math.MinInt64
	tests := []struct {
		name string
		op   func() (Integer, error)
		want int64
		over bool
	}{
		{"max + 1", func() (Integer, error) { return NewInteger(maxInt).AddExact(NewInteger(1)) }, 0, true},
		{"max + 0", func() (Integer, error) { return NewInteger(maxInt).AddExact(NewInteger(0)) }, maxInt, false},
		{"min + -1", func() (Integer, error) { return NewInteger(minInt).AddExact(NewInteger(-1)) }, 0, true},
		{"-max - 1", func() (Integer, error) { return NewInteger(-maxInt).SubtractExact(NewInteger(1)) }, minInt, false},
		{"min - 1", func() (Integer, error) { return NewInteger(minInt).SubtractExact(NewInteger(1)) }, 0, true},
		{"max - -1", func() (Integer, error) { return NewInteger(maxInt).SubtractExact(NewInteger(-1)) }, 0, true},
		{"2^62 * 2", func() (Integer, error) { return NewInteger(1 << 62).MultiplyExact(NewInteger(2)) }, 0, true},
		{"-2^62 * 2", func() (Integer, error) { return NewInteger(-(1 << 62)).MultiplyExact(NewInteger(2)) }, minInt, false},
		{"min * -1", func() (Integer, error) { return NewInteger(minInt).MultiplyExact(NewInteger(-1)) }, 0, true},
		{"-1 * min", func() (Integer, error) { return NewInteger(-1).MultiplyExact(NewInteger(minInt)) }, 0, true},
		{"0 * min", func() (Integer, error) { return NewInteger(0).MultiplyExact(NewInteger(minInt)) }, 0, false},
		{"min div -1", func() (Integer, error) { return NewInteger(minInt).Div(NewInteger(-1)) }, 0, true},
		{"min mod -1", func() (Integer, error) { return NewInteger(minInt).Mod(NewInteger(-1)) }, 0, false},
		{"-min", func() (Integer, error) { return NewInteger(minInt).NegateExact() }, 0, true},
		{"-max", func() (Integer, error) { return NewInteger(maxInt).NegateExact() }, -maxInt, false},
		{"abs(min)", func() (Integer, error) { return NewInteger(minInt).AbsExact() }, 0, true},
		{"abs(-max)", func() (Integer, error) { return NewInteger(-maxInt).AbsExact() }, maxInt, false},
	}
	for _, tt := range tests {
		got, err := tt.op()
		if over := errors.Is(err, ErrIntegerRange); over != tt.over {
			t.Errorf("%s: error %v, want overflow %v", tt.name, err, tt.over)
			continue
		}
		if !tt.over && got.Value() != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, got.Value(), tt.want)
		}
	}
}

// A whole decimal reads as an Integer within the int64 range, and reports
// ErrIntegerRange past it; a fraction is dropped toward zero.
func TestIntegerFromDecimal(t *testing.T) {
	for text, want := range map[string]int64{
		"9223372036854775807":  math.MaxInt64,
		"-9223372036854775808": math.MinInt64,
		"9007199254740993":     9007199254740993,
		"-2.5":                 -2,
	} {
		got, err := IntegerFromDecimal(decimal.RequireFromString(text))
		if err != nil || got.Value() != want {
			t.Errorf("IntegerFromDecimal(%s) = %d, %v; want %d", text, got.Value(), err, want)
		}
	}
	for _, text := range []string{"9223372036854775808", "-9223372036854775809", "1000000000000000000000000000000"} {
		if _, err := IntegerFromDecimal(decimal.RequireFromString(text)); !errors.Is(err, ErrIntegerRange) {
			t.Errorf("IntegerFromDecimal(%s): error %v, want ErrIntegerRange", text, err)
		}
		// Nor is it an Integer for Decimal.ToInteger
		if i, ok := NewDecimalFromDecimal(decimal.RequireFromString(text)).ToInteger(); ok {
			t.Errorf("ToInteger(%s) = %d, true; want false", text, i.Value())
		}
	}
	if i, ok := NewDecimalFromDecimal(decimal.RequireFromString("9223372036854775807")).ToInteger(); !ok || i.Value() != math.MaxInt64 {
		t.Errorf("ToInteger(9223372036854775807) = %d, %v; want the int64 maximum", i.Value(), ok)
	}
}
