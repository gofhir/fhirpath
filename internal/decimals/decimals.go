// Package decimals writes and scales shopspring decimals without reading the
// library's process-wide settings, which a program embedding the engine may
// change (#237). Both give what the library gives under its defaults.
package decimals

import (
	"strings"

	"github.com/shopspring/decimal"
)

// Format writes d as String() does under the library's defaults: no
// exponent, and the trailing zeros of the fraction dropped. StringFixed
// writes every digit at the value's own places and reads no setting.
func Format(d decimal.Decimal) string {
	if d.Exponent() >= 0 {
		return d.StringFixed(0)
	}
	text := strings.TrimRight(d.StringFixed(-d.Exponent()), "0")
	return strings.TrimSuffix(text, ".")
}

// Computed is the result of a computation as the engine holds it: at the
// units or finer. A positive exponent there is the computation's, 1e4 for
// NewFromFloat(10000) and 30e1 for 15e1 × 2, not a digit written left of the
// point, which only a value read from text has, 1.5e2 (#246). The value is
// unchanged.
func Computed(d decimal.Decimal) decimal.Decimal {
	if d.Exponent() > 0 {
		return d.Round(0)
	}
	return d
}

// Natural is d at its natural scale, as reading back its String() gives it:
// the trailing zeros of the coefficient dropped, down to the units, and a
// positive exponent written out, 3000 rather than 3E3.
func Natural(d decimal.Decimal) decimal.Decimal {
	if d.Exponent() < 0 {
		d = d.Normalize()
	}
	return Computed(d)
}
