package fhirpath

import "testing"

// TestUCUMUnitHandling covers unit handling delegated to the UCUM library:
// dimensional analysis, exact conversion, affine scales and unit algebra.
//
// Before this, units came from a hardcoded table of about sixty codes with no
// dimensional analysis, which answered wrongly and silently — most visibly for
// temperature, where Celsius and Fahrenheit shared a canonical unit with a
// factor of one.
func TestUCUMUnitHandling(t *testing.T) {
	t.Run("affine scales convert correctly", func(t *testing.T) {
		// 100 °F is 37.8 °C, so it is not greater than 50 °C
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "100 '[degF]' > 50 'Cel'"), false)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "212 '[degF]' = 100 'Cel'"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "32 '[degF]' = 0 'Cel'"), true)
	})

	t.Run("conversion is exact", func(t *testing.T) {
		// A float round-trip made these off by one part in 10^16
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "1 'L' = 1000 'mL'"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "1 'mol/L' = 1000 'mmol/L'"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "1 'g' = 1000 'mg'"), true)
	})

	t.Run("unit algebra", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "2.0 'cm' * 2.0 'm' = 0.040 'm2'"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "4.0 'g' / 2.0 'm' = 2 'g/m'"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "1.0 'm' / 1.0 'm' = 1 '1'"), true)
	})

	t.Run("compound units and annotations", func(t *testing.T) {
		// Neither is in any table: both are parsed from the UCUM grammar
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "1 'mg/kg/d' < 2 'mg/kg/d'"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "70 '{beats}/min' = 70 '/min'"), true)
	})

	t.Run("different dimensions cannot be compared", func(t *testing.T) {
		for _, expr := range []string{"1 'cm2' <= 1 'cm'", "1 'mg' > 1 'm'"} {
			assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
		}
	})

	t.Run("calendar keywords convert to their UCUM equivalents", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "7 days = 1 week"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "7 days = 1 'wk'"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "6 days < 1 week"), true)
	})

	t.Run("UCUM codes are case sensitive", func(t *testing.T) {
		// KG is not a unit; treating it as kg would be worse than refusing.
		// An invalid unit makes the comparison unknown rather than false:
		// "Attempting to operate on quantities with invalid units will result
		// in empty".
		expr := "1 'kg' = 1 'KG'"
		assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
	})
}

// TestPrefixOnASpecialUnit covers a prefix applied to a unit on an affine
// scale, which UCUM §22.4 says multiplies the argument of the scale's function
// rather than its result.
//
// A milli-Celsius is a thousandth of a degree Celsius, not a thousandth of the
// distance from absolute zero. Read the other way, 1 'mCel' came out as
// -272.87585 'Cel', which is not a temperature anyone wrote down. No conformance
// case covers this, so the check lives here.
func TestPrefixOnASpecialUnit(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"1 'mCel'.toQuantity('Cel')", "0.001 'Cel'"},
		{"1 'kCel'.toQuantity('Cel')", "1000 'Cel'"},

		// Crossing to a linear scale still applies the offset, once
		{"1 'cCel'.toQuantity('K')", "273.16 'K'"},
		{"0 'Cel'.toQuantity('K')", "273.15 'K'"},

		// The unprefixed conversions are unchanged
		{"0 'Cel'.toQuantity('[degF]')", "32 '[degF]'"},
		{"37 'Cel'.toQuantity('[degF]')", "98.6 '[degF]'"},
		// Parenthesised because unary minus binds looser than the invocation:
		// -40 'Cel'.toQuantity(...) negates the converted 104, and -40 °C being
		// -40 °F is the point of the case
		{"(-40 'Cel').toQuantity('[degF]')", "-40 '[degF]'"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, simpleJSON); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}
