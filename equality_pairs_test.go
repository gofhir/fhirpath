package fhirpath

import "testing"

// = over collections answers false when any pair is unequal, however many
// pairs are unknown, and empty only when none is unequal: "if the result is
// false for any pair, returns false" (3.0.0), each item "must be equal"
// (2.0.0). fhirpath.js and the HL7 validator answer so.
func TestEqualityOfCollectionsWithAnUnknownPair(t *testing.T) {
	for expr, want := range map[string]string{
		"(@2012 | @2013) = (@2012-01 | @2014)":  "false",
		"(@2012 | @2013) != (@2012-01 | @2014)": "true",
		"(@2012 | @2013) = (@2012-01 | @2013)":  "EMPTY",
		"(@2012 | @2013) = (@2012 | @2013)":     "true",
	} {
		if got := evaluateScalar(t, expr, simpleJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// An Integer or a Decimal facing a quantity converts to a Quantity of unit
// '1', as 2.0.0's implicit conversions and 3.0.0's examples have it:
// 23 = 23 '1' and 23 ~ 23 '1' are true. A number is no quantity of another
// dimension, so 2 = 2 'mg' is false, and its order against one is unknown.
// +, - and / convert too, as fhirpath.js does and 3.0.0's 2 + 2 '1' and
// 60 / 1 's' read it; up to 1.15.0 they did not.
func TestANumberComparesWithAQuantityOfUnitOne(t *testing.T) {
	for expr, want := range map[string]string{
		"23 = 23 '1'":    "true",
		"23 ~ 23 '1'":    "true",
		"23 = 24 '1'":    "false",
		"2 < 3 '1'":      "true",
		"2.5 '1' > 2":    "true",
		"2.5 = 2.5 '1'":  "true",
		"2 = 2 'mg'":     "false",
		"2 != 2 'mg'":    "true",
		"2 < 3 'mg'":     "EMPTY",
		"1 'cm' = 1 's'": "EMPTY",
		// the operators that compare items to find one agree with =
		"(23 | 23 '1').count()":                   "1",
		"23 in (23 '1')":                          "true",
		"(23).combine(23 '1').distinct().count()": "1",
		"(2 | 2 'mg').count()":                    "2",
	} {
		if got := evaluateScalar(t, expr, simpleJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
	for expr, want := range map[string]string{
		"2 + 3 '1'":            "5 '1'",
		"5 '1' - 2":            "3 '1'",
		"60 / 1 's' = 60 '/s'": "true",
		"2 + 3 'mg'":           "EMPTY",
	} {
		if got := evaluateScalar(t, expr, simpleJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// A FHIR Quantity is found by in, contains and union as = finds it: where
// Observation.value = 5 'mg' is true, so is 5 'mg' in Observation.value, as
// fhirpath.js and the HL7 validator answer.
func TestAFHIRQuantityIsFoundAsEqualFindsIt(t *testing.T) {
	doc := []byte(`{"resourceType":"Observation","status":"final","code":{"text":"x"},` +
		`"valueQuantity":{"value":5,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}`)
	for expr, want := range map[string]string{
		"Observation.value = 5 'mg'":              "true",
		"5 'mg' in Observation.value":             "true",
		"Observation.value contains 5 'mg'":       "true",
		"(Observation.value | 5 'mg').count()":    "1",
		"(Observation.value | 5000 'ug').count()": "1",
		"5 in Observation.value":                  "false",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// A number is found in a FHIR Quantity of unit '1' as = finds it, and
// equivalent to it either way round; a Money is no quantity, and an object
// whose value is a string none either.
func TestANumberIsFoundInAFHIRQuantityOfUnitOne(t *testing.T) {
	doc := []byte(`{"resourceType":"Claim","status":"active","total":{"value":5,"currency":"USD"},` +
		`"item":[{"sequence":1,"quantity":{"value":5,"system":"http://unitsofmeasure.org","code":"1"}}],` +
		`"identifier":[{"value":"5"}]}`)
	for expr, want := range map[string]string{
		"5 in Claim.item.quantity":            "true",
		"(Claim.item.quantity | 5).count()":   "1",
		"5 ~ Claim.item.quantity":             "true",
		"Claim.item.quantity ~ 5":             "true",
		"5 in Claim.total":                    "false",
		"(Claim.total | 5).count()":           "2",
		"(Claim.identifier | '5').count()":    "2",
		"(Claim.item.quantity | '5').count()": "2",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
