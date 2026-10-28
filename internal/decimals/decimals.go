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

// Natural is d at its natural scale, as reading back its String() gives it:
// the trailing zeros of the coefficient dropped, down to the units, and a
// positive exponent written out, 3000 rather than 3E3.
func Natural(d decimal.Decimal) decimal.Decimal {
	if d.Exponent() < 0 {
		d = d.Normalize()
	}
	if d.Exponent() > 0 {
		return d.Round(0)
	}
	return d
}
