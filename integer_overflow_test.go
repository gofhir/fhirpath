package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// "Operations that cause arithmetic overflow or underflow will result in empty
// ({ })", FHIRPath 2.0.0 and 3.0.0. An Integer is held in 64 bits here, and
// arithmetic that crosses either end of that range answers empty, where it
// wrapped: 9223372036854775807 + 1 was -9223372036854775808. The ends
// themselves are values. ceiling(), floor() and truncate() read the decimal
// itself, where a float held 9007199254740993 as 9007199254740994.
func TestIntegerArithmeticPastTheRangeIsEmpty(t *testing.T) {
	for expr, want := range map[string]string{
		"9223372036854775807 + 1":                                "[]",
		"-9223372036854775807 - 2":                               "[]",
		"9223372036854775807 * 2":                                "[]",
		"4611686018427387904 * 2":                                "[]",
		"(-9223372036854775807 - 1) div -1":                      "[]",
		"-(-9223372036854775807 - 1)":                            "[]",
		"(-9223372036854775807 - 1).abs()":                       "[]",
		"(9223372036854775807 | 1).sum()":                        "[]",
		"(9223372036854775807 | 1).aggregate($this + $total, 0)": "[]",
		"(9223372036854775807 + 1).exists()":                     "[false]",
		"1000000000000000000000000000000.5.floor()":              "[]",
		"(-1000000000000000000000000000000.5).ceiling()":         "[]",
		// The ends of the range are values
		"9223372036854775806 + 1":           "[9223372036854775807]",
		"-9223372036854775807 - 1":          "[-9223372036854775808]",
		"-4611686018427387904 * 2":          "[-9223372036854775808]",
		"(-9223372036854775807 - 1) mod -1": "[0]",
		"(9223372036854775806 | 1).sum()":   "[9223372036854775807]",
		// Whole numbers read from the decimal, not through a float
		"9007199254740993.5.floor()":    "[9007199254740993]",
		"9007199254740993.5.truncate()": "[9007199254740993]",
		"9007199254740993.5.ceiling()":  "[9007199254740994]",
		"(-2.5).floor()":                "[-3]",
		"(-2.5).ceiling()":              "[-2]",
		"(-2.5).truncate()":             "[-2]",
		// div and mod over decimals truncate the decimal itself
		"100000000000000000000.0 div 1.0": "[100000000000000000000]",
		"100000000000000000000.5 mod 1.0": "[0.5]",
		"7.5 mod 2":                       "[1.5]",
		// A Decimal does not convert to an Integer at all: "the item is an
		// Integer ... a String ... a Boolean"
		"100000000000000000000.0.toInteger()":         "[]",
		"100000000000000000000.0.convertsToInteger()": "[false]",
		"9223372036854775807.0.convertsToInteger()":   "[false]",
		// The quotient is exact, not a division rounded first
		"0.99999999999999999 div 1.0":       "[0]",
		"0.99999999999999999 mod 1.0":       "[0.99999999999999999]",
		"-7.5 div 2":                        "[-3]",
		"-7.5 mod 2":                        "[-1.5]",
		"7.5 mod -2":                        "[1.5]",
		"9223372036854775807.0.toInteger()": "[]",
	} {
		result, err := fhirpath.Evaluate([]byte(`{"resourceType":"Patient"}`), expr)
		if err != nil {
			t.Errorf("%s: %v", expr, err)
			continue
		}
		if got := result.String(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
