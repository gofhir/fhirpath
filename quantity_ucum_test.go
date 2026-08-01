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
		// M is not a unit; treating it as m would be worse than refusing
		expr := "1 'kg' = 1 'KG'"
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, expr), false)
	})
}
