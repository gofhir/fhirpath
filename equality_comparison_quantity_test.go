package fhirpath_test

import (
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// TestQuantityObjectComparison covers quantities carried as JSON objects, which
// is how every FHIR quantity arrives. Range.low and Range.high are both objects,
// so rng-2 used to fail with "cannot apply 'compare' to Quantity and Quantity".
func TestQuantityObjectComparison(t *testing.T) {
	t.Run("rng-2 on an ordered range", func(t *testing.T) {
		rng := []byte(`{
			"low": {"value": 1, "unit": "mg", "system": "http://unitsofmeasure.org", "code": "mg"},
			"high": {"value": 5, "unit": "mg", "system": "http://unitsofmeasure.org", "code": "mg"}
		}`)
		assertBooleanResult(t, evalOrFatal(t, rng, rng2), true)
		assertBooleanResult(t, evalOrFatal(t, rng, "low <= high"), true)
		assertBooleanResult(t, evalOrFatal(t, rng, "low < high"), true)
		assertBooleanResult(t, evalOrFatal(t, rng, "high > low"), true)
	})

	t.Run("rng-2 on an inverted range", func(t *testing.T) {
		rng := []byte(`{"low":{"value":5,"code":"mg"},"high":{"value":1,"code":"mg"}}`)
		assertBooleanResult(t, evalOrFatal(t, rng, rng2), false)
	})

	t.Run("rng-2 with a missing bound", func(t *testing.T) {
		rng := []byte(`{"low":{"value":5,"code":"mg"}}`)
		assertBooleanResult(t, evalOrFatal(t, rng, rng2), true)
	})

	t.Run("commensurable units are converted", func(t *testing.T) {
		rng := []byte(`{
			"low": {"value": 10, "unit": "milligram", "system": "http://unitsofmeasure.org", "code": "mg"},
			"high": {"value": 2, "unit": "gram", "system": "http://unitsofmeasure.org", "code": "g"}
		}`)
		assertBooleanResult(t, evalOrFatal(t, rng, rng2), true)
		assertBooleanResult(t, evalOrFatal(t, rng, "high <= low"), false)
	})

	t.Run("coded unit wins over the display unit", func(t *testing.T) {
		// unit is a display string ("milligram") that no conversion understands;
		// code carries the UCUM symbol
		q := []byte(`{"value":10,"unit":"milligram","system":"http://unitsofmeasure.org","code":"mg"}`)
		assertBooleanResult(t, evalOrFatal(t, q, "$this <= 2 'g'"), true)
		assertBooleanResult(t, evalOrFatal(t, q, "$this = 10 'mg'"), true)
		assertBooleanResult(t, evalOrFatal(t, q, "$this ~ 0.01 'g'"), true)
		assertStringResult(t, evalOrFatal(t, q, "$this.toQuantity().toString()"), "10 'mg'")
		assertBooleanResult(t, evalOrFatal(t, q, "$this.convertsToQuantity()"), true)
		assertBooleanResult(t, evalOrFatal(t, q, "$this.convertsToQuantity('g')"), true)
		assertBooleanResult(t, evalOrFatal(t, q, "$this.convertsToQuantity('m')"), false)
	})

	t.Run("quantity objects support arithmetic", func(t *testing.T) {
		rng := []byte(`{"low":{"value":1,"code":"mg"},"high":{"value":5,"code":"mg"}}`)
		assertStringResult(t, evalOrFatal(t, rng, "(high - low).toString()"), "4 'mg'")
		assertStringResult(t, evalOrFatal(t, rng, "(low + high).toString()"), "6 'mg'")
	})

	t.Run("incommensurable units yield empty, not an error", func(t *testing.T) {
		rng := []byte(`{"low":{"value":1,"code":"cm"},"high":{"value":1,"code":"g"}}`)
		for _, expr := range []string{"low <= high", "low > high", "high - low"} {
			assertEmptyResult(t, evalOrFatal(t, rng, expr), expr)
		}
	})

	t.Run("quantity literals scale by a number", func(t *testing.T) {
		// Documented in examples/quantities-and-units
		assertStringResult(t, evalOrFatal(t, simpleJSON, "(250 'mg' * 4).toString()"), "1000 'mg'")
		assertStringResult(t, evalOrFatal(t, simpleJSON, "(4 * 250 'mg').toString()"), "1000 'mg'")
		assertStringResult(t, evalOrFatal(t, simpleJSON, "(1 'g' / 4).toString()"), "0.25 'g'")
		// in the more granular unit, "the most granular unit of either input"
		assertStringResult(t, evalOrFatal(t, simpleJSON, "(1 'kg' - 200 'g').toString()"), "800 'g'")
	})

	t.Run("quantity literals with incommensurable units", func(t *testing.T) {
		for _, expr := range []string{"10 'kg' < 10 'm'", "10 'kg' >= 10 'm'", "10 'kg' + 10 'm'"} {
			assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
		}
		// Equality is unknown too, not false: "If this process returns empty
		// (e.g. because the units are not valid, or not commensurable), then the
		// result of the equality comparison is empty"
		assertEmptyResult(t, evalOrFatal(t, simpleJSON, "10 'kg' = 10 'm'"), "10 'kg' = 10 'm'")
	})

	t.Run("objects that are not quantities are unaffected", func(t *testing.T) {
		// A Reference has no numeric "value": comparing it stays an error
		resource := []byte(`{"a":{"reference":"Patient/1"},"b":{"reference":"Patient/2"}}`)
		if _, err := fhirpath.Evaluate(resource, "a <= b"); err == nil {
			t.Error("expected an error comparing two non-quantity objects")
		}
	})
}

// The mapping these tests cover is FHIR's, not FHIRPath's, so no case in the
// official FHIRPath suite exercises it. FHIR R5, "Using FHIRPath with FHIR",
// Use of FHIR Quantity:
//
//	The Mapping from FHIR Quantity to FHIRPath System.Quantity can only be
//	applied if the FHIR Quantity has a UCUM code - i.e. a system of
//	http://unitsofmeasure.org, and a code is present. As part of the mapping,
//	time-valued UCUM units are mapped to the calendar duration units defined in
//	FHIRPath, according to the following map:
//	  a -> year, mo -> month, d -> day, h -> hour, min -> minute, s -> second

func observationWith(quantity string) []byte {
	return []byte(`{"resourceType":"Observation","valueQuantity":` + quantity + `}`)
}

// TestFHIRQuantityMapsToCalendarUnits checks that a duration read from FHIR data
// can take part in date arithmetic.
//
// This is the point of the mapping. A UCUM year is a definite 365.25 days and
// cannot be added to a calendar, so 1 'a' on its own is an error; the 1 year it
// maps to is precisely what the calendar can add. Without the mapping,
// Patient.birthDate + Observation.value would fail on data that FHIR considers
// well formed.
func TestFHIRQuantityMapsToCalendarUnits(t *testing.T) {
	cases := []struct {
		name     string
		quantity string
		expr     string
		want     string
	}{
		{
			"a becomes a calendar year",
			`{"value":1,"unit":"year","system":"http://unitsofmeasure.org","code":"a"}`,
			"@2014-01-01 + Observation.value", "2015-01-01",
		},
		{
			"a leap year is respected, which a definite duration would not be",
			`{"value":1,"unit":"year","system":"http://unitsofmeasure.org","code":"a"}`,
			"@2016-02-29 + Observation.value", "2017-02-28",
		},
		{
			"mo becomes a calendar month",
			`{"value":1,"unit":"month","system":"http://unitsofmeasure.org","code":"mo"}`,
			"@2014-01-31 + Observation.value", "2014-02-28",
		},
		{
			"d becomes a day",
			`{"value":7,"unit":"day","system":"http://unitsofmeasure.org","code":"d"}`,
			"@2014-01-01 + Observation.value", "2014-01-08",
		},
		{
			"the mapped unit compares as the calendar duration it now is",
			`{"value":1,"unit":"year","system":"http://unitsofmeasure.org","code":"a"}`,
			"Observation.value = 1 year", "true",
		},
		{
			"wk is absent from FHIR's map and keeps its UCUM code, which adds the same span",
			`{"value":1,"unit":"week","system":"http://unitsofmeasure.org","code":"wk"}`,
			"@2014-01-01 + Observation.value", "2014-01-08",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, observationWith(tc.quantity)); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestFHIRQuantityMappingRequiresUCUMSystem checks the condition the
// specification places on the mapping.
//
// Without a UCUM system there is no mapping, so the code stays the definite
// duration it was — and a definite duration of a year cannot be added to a
// calendar. Both branches follow from the same rule, which is why refusing here
// is not a gap but the other half of accepting above.
func TestFHIRQuantityMappingRequiresUCUMSystem(t *testing.T) {
	for _, tc := range []struct{ name, quantity string }{
		{"no system at all", `{"value":1,"unit":"year","code":"a"}`},
		{"some other code system", `{"value":1,"unit":"year","system":"http://example.org/units","code":"a"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := fhirpath.MustCompile("@2014-01-01 + Observation.value").Evaluate(observationWith(tc.quantity))
			if err == nil {
				t.Error("expected an error: without a UCUM system, 'a' stays a definite duration")
			}
		})
	}
}

// TestFHIRQuantityArithmeticRejectsNonDurations checks that the mapping did not
// widen what date arithmetic accepts.
func TestFHIRQuantityArithmeticRejectsNonDurations(t *testing.T) {
	quantity := `{"value":5,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}`

	if _, err := fhirpath.MustCompile("@2014-01-01 + Observation.value").Evaluate(observationWith(quantity)); err == nil {
		t.Error("expected an error: a mass is not a duration")
	}
}

// TestFHIRQuantityBoundaries checks that lowBoundary() and highBoundary() read a
// FHIR Quantity through the same mapping comparable() and the comparison
// operators apply. Without it they returned empty, and so did R5's rng-2 on
// every Range with both ends, which an invariant cannot be.
func TestFHIRQuantityBoundaries(t *testing.T) {
	rangeOf := []byte(`{"resourceType":"Observation","valueRange":{
		"low":{"value":1,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"},
		"high":{"value":2,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}}`)

	cases := []struct{ expr, want string }{
		{"Observation.value.low.lowBoundary()", "0.50000000 'mg'"},
		{"Observation.value.high.highBoundary()", "2.50000000 'mg'"},
		{"Observation.value.low.lowBoundary(2)", "0.50 'mg'"},
		{"Observation.value.low.value.lowBoundary()", "0.50000000"},
		{
			"Observation.value.select(low.value.empty() or high.value.empty() or " +
				"low.lowBoundary().comparable(high.highBoundary()).not() or " +
				"(low.lowBoundary() <= high.highBoundary()))",
			"true",
		},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, rangeOf); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestFHIRMoneyIsNotAQuantity checks that a Money does not convert. It has a
// value, but its currency is no unit, and converting it with none made a sum
// of money comparable to a mass and equal to a unitless quantity.
func TestFHIRMoneyIsNotAQuantity(t *testing.T) {
	money := []byte(`{"resourceType":"Claim","total":{"value":10.5,"currency":"USD"}}`)

	for _, tc := range []struct{ expr, want string }{
		// cannot be compared with one: false, as fhirpath.js and the HL7
		// validator answer
		{"Claim.total.comparable(10 'mg')", "false"},
		{"Claim.total.comparable(Claim.total)", "false"},
		{"Claim.total = 10.5 '1'", "false"},
		{"Claim.total.lowBoundary()", "EMPTY"},
		{"Claim.total.toQuantity()", "EMPTY"},
		{"Claim.total.value < 11", "true"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, money); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}

	if _, err := fhirpath.MustCompile("Claim.total + 1 'USD'").Evaluate(money); err == nil {
		t.Error("Claim.total + 1 'USD': no error, want a Money refused as a quantity")
	}

	// Two Money values are equal by amount and currency, however each is
	// written. ~ compares them as objects, as every complex type (#118)
	claim := []byte(`{
  "resourceType": "Claim",
  "total": {
    "value": 30,
    "currency": "USD"
  },
  "item": [
    {"net": {"currency": "USD", "value": 30.0}},
    {"net": {"value": 30, "currency": "EUR"}}
  ]
}`)
	for expr, want := range map[string]string{
		"Claim.total = Claim.item[0].net": "true",
		"Claim.total = Claim.item[1].net": "false",
	} {
		if got := evaluateScalar(t, expr, claim); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// TestFHIRQuantityComparatorIsNotRead covers a quantity with a comparator. The
// HL7 validator reads its value whatever bounds it, so < 5 'mg' compares, is
// equivalent and is bounded as 5 'mg'; each answer below is what the validator
// 6.10.2 gives. Refusing it ended the evaluation, and so the constraint, where
// the validator answers. CONFORMANCE.md records the choice.
func TestFHIRQuantityComparatorIsNotRead(t *testing.T) {
	bound := observationWith(`{"value":5,"comparator":"<","system":"http://unitsofmeasure.org","code":"mg"}`)

	for _, tc := range []struct{ expr, want string }{
		{"Observation.value = 5 'mg'", "true"},
		{"Observation.value != 5 'mg'", "false"},
		{"Observation.value ~ 5 'mg'", "true"},
		{"Observation.value < 6 'mg'", "true"},
		{"6 'mg' > Observation.value", "true"},
		{"Observation.value.comparable(1 'mg')", "true"},
		{"(1 'mg').comparable(Observation.value)", "true"},
		{"Observation.value.lowBoundary()", "4.50000000 'mg'"},
		{"Observation.value.highBoundary()", "5.50000000 'mg'"},
		{"Observation.value.toQuantity()", "5 'mg'"},
		{"Observation.value.convertsToQuantity()", "true"},
		{"Observation.value = 'x'", "false"},
		{"Observation.value = Observation.value", "true"},
		{"Observation.value.value < 6", "true"},
		// The comparator is still read by name.
		{"Observation.value.comparator", "<"},
		// Arithmetic reads the value too. The validator raises an exception on
		// arithmetic with any FHIR Quantity; CONFORMANCE.md records it.
		{"Observation.value + 1 'mg'", "6 'mg'"},
		// Empty still propagates, and a collection argument has no answer.
		{"{} = Observation.value", "EMPTY"},
		{"Observation.value + {}", "EMPTY"},
		{"Observation.value.comparable(1 'mg' | 2 'mg')", "EMPTY"},
		{"'x' in Observation.descendants()", "false"},
		{"(5 'mg' | Observation.value) ~ (Observation.value | 5 'mg')", "true"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, bound); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}

	// An empty comparator is no comparator
	unbounded := observationWith(`{"value":5,"comparator":"","system":"http://unitsofmeasure.org","code":"mg"}`)
	if got := evaluateScalar(t, "Observation.value = 5 'mg'", unbounded); got != "true" {
		t.Errorf("with an empty comparator: %s, want true", got)
	}
}

// TestFHIRQuantityBoundSortKey checks that a sort orders a quantity with a
// comparator by its value, even beside an empty key, and that a key of several
// items is refused as a key of several items.
func TestFHIRQuantityBoundSortKey(t *testing.T) {
	withEmpty := []byte(`{"resourceType":"Observation","component":[
		{"valueQuantity":{"value":5,"comparator":"<","system":"http://unitsofmeasure.org","code":"mg"}},
		{"valueQuantity":{"value":3,"system":"http://unitsofmeasure.org","code":"mg"}},
		{"code":{"text":"no value"}}]}`)
	result, err := fhirpath.MustCompile("Observation.component.sort(value).value.value").Evaluate(withEmpty)
	if err != nil || result.String() != "[3, 5]" {
		t.Errorf("sort(value) = %s, %v; want [3, 5]", result, err)
	}

	further := []byte(`{"resourceType":"Observation","component":[
		{"referenceRange":[{"low":{"value":5,"system":"http://unitsofmeasure.org","code":"mg"}},
			{"low":{"value":1,"comparator":"<","system":"http://unitsofmeasure.org","code":"mg"}}]},
		{"referenceRange":[{"low":{"value":3,"system":"http://unitsofmeasure.org","code":"mg"}}]}]}`)
	if _, err := fhirpath.MustCompile("Observation.component.sort(referenceRange.low)").Evaluate(further); err == nil ||
		!strings.Contains(err.Error(), "expected a single value") {
		t.Errorf("a key of two items: error %v, want a singleton error", err)
	}
}

// TestRangeInvariantsWithoutUCUMCode guards rng-2 on Ranges whose quantities
// have no UCUM code, a local unit such as tablets, which dosage data is full
// of. R5 writes it with lowBoundary() and comparable(), and an empty answer
// fails the invariant on a valid Range; the HL7 validator reports nothing
// there. Reading FHIR's condition on the mapping strictly, as UCUM only, did
// exactly that, which is why the mapping keeps its code or unit.
func TestRangeInvariantsWithoutUCUMCode(t *testing.T) {
	r5rng2 := "Observation.value.select(low.value.empty() or high.value.empty() or " +
		"low.lowBoundary().comparable(high.highBoundary()).not() or (low.lowBoundary() <= high.highBoundary()))"
	r4rng2 := "Observation.value.select(low.empty() or high.empty() or (low <= high))"

	for _, tc := range []struct{ name, low, high, want string }{
		{"tablets", `{"value":1,"unit":"tablet"}`, `{"value":5,"unit":"tablet"}`, "true"},
		{"a local code", `{"value":1,"unit":"TAB","system":"http://terminology.hl7.org/CodeSystem/v3-orderableDrugForm","code":"TAB"}`,
			`{"value":2,"unit":"TAB","system":"http://terminology.hl7.org/CodeSystem/v3-orderableDrugForm","code":"TAB"}`, "true"},
		{"an inverted range", `{"value":5,"unit":"tablet"}`, `{"value":1,"unit":"tablet"}`, "false"},
	} {
		data := []byte(`{"resourceType":"Observation","valueRange":{"low":` + tc.low + `,"high":` + tc.high + `}}`)
		for _, expr := range []string{r5rng2, r4rng2} {
			if got := evaluateScalar(t, expr, data); got != tc.want {
				t.Errorf("%s: %s = %s, want %s", tc.name, expr, got, tc.want)
			}
		}
	}

	// Equal by value, as on main: 1 tablet is 1.0 tablet
	tablets := []byte(`{"resourceType":"Observation","valueRange":{"low":{"value":1,"unit":"tablet"},"high":{"value":1.0,"unit":"tablet"}}}`)
	if got := evaluateScalar(t, "Observation.value.low = Observation.value.high", tablets); got != "true" {
		t.Errorf("1 tablet = 1.0 tablet: %s, want true", got)
	}
}

// TestOnlyAQuantityComparesAsOne checks that = and ~ read two FHIR Quantities
// as quantities, as the HL7 validator 6.10.2 does. With a model only objects
// typed Quantity, or one of its profiles, convert: conformance holds the
// adjudication that tells them apart. Without one, an object of no known
// type with a numeric value alone converts, so a Range written without units
// still orders.
func TestOnlyAQuantityComparesAsOne(t *testing.T) {
	observation := []byte(`{"resourceType":"Observation","component":[` +
		`{"valueQuantity":{"value":1,"system":"http://unitsofmeasure.org","code":"g"}},` +
		`{"valueQuantity":{"value":1000,"system":"http://unitsofmeasure.org","code":"mg"}}],` +
		`"valueRange":{"low":{"value":1},"high":{"value":5}}}`)

	for _, tc := range []struct{ expr, want string }{
		{"Observation.component[0].value = Observation.component[1].value", "true"},
		{"Observation.component[0].value ~ Observation.component[1].value", "true"},
		{"Observation.value.low < Observation.value.high", "true"},
	} {
		if got := evaluateScalar(t, tc.expr, observation); got != tc.want {
			t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
		}
	}
}

// TestNestedQuantitiesAreEquivalentAsQuantities checks that ~ compares a
// Quantity inside a complex value as a quantity too, as the HL7 validator
// 6.10.2 does, so the answer does not depend on how deep the quantity sits;
// = compares content as written.
func TestNestedQuantitiesAreEquivalentAsQuantities(t *testing.T) {
	observation := []byte(`{"resourceType":"Observation","referenceRange":[` +
		`{"low":{"value":1,"system":"http://unitsofmeasure.org","code":"g"}},` +
		`{"low":{"value":1000,"system":"http://unitsofmeasure.org","code":"mg"}},` +
		`{"low":{"value":2,"system":"http://unitsofmeasure.org","code":"g"}}]}`)

	for _, tc := range []struct{ expr, want string }{
		{"Observation.referenceRange[0].low ~ Observation.referenceRange[1].low", "true"},
		{"Observation.referenceRange[0] ~ Observation.referenceRange[1]", "true"},
		{"Observation.referenceRange[0] ~ Observation.referenceRange[2]", "false"},
		{"Observation.referenceRange[0] = Observation.referenceRange[1]", "false"},
	} {
		if got := evaluateScalar(t, tc.expr, observation); got != tc.want {
			t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
		}
	}
}

// TestUCUMQuantityComparison tests UCUM unit normalization for quantity comparisons.
func TestUCUMQuantityComparison(t *testing.T) {
	t.Run("10 mg equals 0.01 g", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "10 'mg' = 0.01 'g'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("1000 mg equals 1 g", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "1000 'mg' = 1 'g'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("100 cm equals 1 m", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "100 'cm' = 1 'm'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("1000 mL equals 1 L", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "1000 'mL' = 1 'L'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("60 min equals 1 h", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "60 'min' = 1 'h'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("500 mg less than 1 g", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "500 'mg' < 1 'g'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("incompatible units are unknown, not unequal", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "1 'kg' = 1 'm'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// "1 'cm' = 1 's' // empty ({ }) ; different dimensions are not
		// commensurable". There is no unit both values can be stated in, so the
		// comparison has no answer rather than a negative one.
		assertEmptyResult(t, result, "1 'kg' = 1 'm'")
	})
}

// TestQuantityEquivalent tests the ~ operator for quantities with UCUM normalization.
func TestQuantityEquivalent(t *testing.T) {
	t.Run("10 mg equivalent to 0.01 g", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "10 'mg' ~ 0.01 'g'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("1 kg equivalent to 1000 g", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "1 'kg' ~ 1000 'g'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("incompatible units not equivalent", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "1 'kg' ~ 1 'm'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, false)
	})
}

// TestComparableFunction covers comparable(), which reports whether two
// quantities have commensurable units.
func TestComparableFunction(t *testing.T) {
	cases := []struct {
		expr     string
		expected bool
	}{
		{"(1 'cm').comparable(1 '[in_i]')", true},
		{"(1 'cm').comparable(1 'm')", true},
		{"(1 'cm').comparable(1 's')", false},
		{"(1 'mg').comparable(1 'g')", true},
		{"(1 'mg').comparable(1 'L')", false},
		{"(1 'cm').comparable(1 '[s]')", false},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			assertBooleanResult(t, evalOrFatal(t, simpleJSON, tc.expr), tc.expected)
		})
	}

	t.Run("works on FHIR quantity objects", func(t *testing.T) {
		q := []byte(`{"value":10,"unit":"milligram","system":"http://unitsofmeasure.org","code":"mg"}`)
		assertBooleanResult(t, evalOrFatal(t, q, "$this.comparable(1 'g')"), true)
		assertBooleanResult(t, evalOrFatal(t, q, "$this.comparable(1 's')"), false)
	})
}

// TestCalendarDurationEquality covers the two duration systems and where they
// meet.
//
// The calendar has its own lengths — a year is 365 days, a month 30 — which are
// not UCUM's 365.25 and 30.4375. From a week down the two agree by definition
// and convert freely; at a year or a month they do not, so a comparison across
// the systems has no answer rather than a negative one.
//
// Every case below is either an example the specification works through or one
// documented in fhirpath.js, the reference implementation.
func TestCalendarDurationEquality(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// Within the calendar
		{"1 year = 12 months", "true"},
		{"1 year = 365 days", "true"},
		{"1 month = 30 days", "true"},
		{"1 week = 7 days", "true"},
		{"1 day = 24 hours", "true"},
		{"1 hour = 60 minutes", "true"},
		{"1 minute = 60 seconds", "true"},

		// The chain is not transitive: a year is 365 days directly, not twelve
		// thirty-day months
		{"1 year = 360 days", "false"},

		// Across the systems, at a week and below
		{"1 week = 1 'wk'", "true"},
		{"1 second = 1 's'", "true"},
		{"1 hour = 3600 's'", "true"},
		{"1 'h' = 3600 's'", "true"},

		// Across the systems, at a year or a month: no common unit exists
		{"1 year = 1 'a'", "EMPTY"},
		{"1 year = 12 'mo'", "EMPTY"},
		{"1 month = 30 'd'", "EMPTY"},
		{"1 'a' = 365 days", "EMPTY"},

		// "explicit conversion using toQuantity() will change code-systems to
		// intentionally perform this equality"
		{"1 year.toQuantity('a') = 1 'a'", "true"},

		// Units of different dimensions are not comparable either
		{"1 'cm' = 1 's'", "EMPTY"},
		{"1 'cm' = 10.0 'mm'", "true"},
		{"1 'cm' = 1 'm'", "false"},
		{"23 'Cel' = 73.4 '[degF]'", "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestCalendarDurationOrdering checks that an ordering across the two duration
// systems answers where equality does: freely at a week and below, and not at
// all at a year or a month, which have no common unit. < used to convert where
// = refused, so 400 days < 2 'a' was true while 1 'a' = 365 days was empty.
func TestCalendarDurationOrdering(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	for expr, want := range map[string]string{
		// Across the systems, at a year or a month, in either order
		"400 days < 2 'a'":   "EMPTY",
		"1 'a' < 365 days":   "EMPTY",
		"1 year < 500 'd'":   "EMPTY",
		"1 month >= 30 'd'":  "EMPTY",
		"12 'mo' <= 1 year":  "EMPTY",
		"1 'mo' < 31 days":   "EMPTY",
		"1 'mo' > 700 hours": "EMPTY",
		// Within one system, and across them at a week and below
		"1 year < 13 months": "true",
		"1 'a' < 13 'mo'":    "true",
		"1 'a' > 1 'd'":      "true",
		"2 weeks > 10 'd'":   "true",
		"1 'wk' <= 7 days":   "true",
		// Converted explicitly, a calendar year is a UCUM one
		"1 year.toQuantity('a') > 0.5 'a'": "true",
	} {
		if got := evaluateScalar(t, expr, patient); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}

	// A quantity without a unit takes the other's, a calendar year included
	unitless := []byte(`{"resourceType":"Observation","valueQuantity":{"value":2}}`)
	if got := evaluateScalar(t, "Observation.value < 1 year", unitless); got != "false" {
		t.Errorf("a unitless 2 < 1 year = %s, want false", got)
	}

	// A sort meets such a pair as one that does not compare
	if _, err := fhirpath.Evaluate(patient, "(400 days | 2 'a').sort()"); err == nil {
		t.Error("(400 days | 2 'a').sort(): expected an error")
	}
}

// TestCalendarDurationEquivalence covers ~ across the two duration systems at a
// year or a month, where = has no answer: "1 year is not equal to 1 'a' (i.e.
// results in empty), but it is equivalent to 1 'a'". The UCUM value is taken
// in the UCUM unit that matches the calendar unit, and the two compare as
// equivalence does, rounded to the less precise. fhirpath.js answers alike.
func TestCalendarDurationEquivalence(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	for expr, want := range map[string]string{
		"1 year ~ 1 'a'":      "true",
		"1 'a' ~ 1 year":      "true",
		"1 month ~ 1 'mo'":    "true",
		"1 year ~ 12 'mo'":    "true",
		"1.5 years ~ 18 'mo'": "true",
		"1 'a' ~ 365 days":    "true",
		"365 days ~ 1 'a'":    "true",
		"1 'mo' ~ 30 days":    "true",
		"1 year ~ 365 'd'":    "true",
		"2 years ~ 1 'a'":     "false",
		"1 year !~ 1 'a'":     "false",
		"1 year ~ 1 'cm'":     "false",
		// Equality still has no answer
		"1 year = 1 'a'": "EMPTY",
		// In the less granular unit, at the precision of the less precise
		"1.04 years ~ 1 'a'": "true",
		"1.6 years ~ 1 'a'":  "false",
		"1 'a' ~ 365.0 days": "true",
	} {
		if got := evaluateScalar(t, expr, patient); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// TestQuantityEquivalenceSpecExamples runs the examples FHIRPath 3.0.0 works
// through for quantity equivalence: "the quantity values must be converted to
// the least granular unit", then compared "on values rounded to the precision
// of the least precise operand", trailing zeroes carrying no precision. Each
// also holds with its sides swapped.
func TestQuantityEquivalenceSpecExamples(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	for expr, want := range map[string]string{
		"21 'mm' ~ 2 'cm'":         "true",
		"4 'g' ~ 4000 'mg'":        "true",
		"4 'g' ~ 4040 'mg'":        "true",
		"1 '[in_i]' ~ 2.5 'cm'":    "true",
		"23 'Cel' ~ 73.4 '[degF]'": "true",
		"1 year ~ 1 'a'":           "true",
		"1 year ~ 12 'mo'":         "true",
		"1 year ~ 11 months":       "true",
		"1 second ~ 1 's'":         "true",
		// Trailing zeroes carry no precision, as for decimals: 1.0 ~ 1.4
		"1.0 'g' ~ 1.4 'g'": "true",
		"1.0 'g' ~ 1.6 'g'": "false",
		"4 'g' ~ 4600 'mg'": "false",
		"1 'cm' ~ 1 's'":    "false",
		// On a scale with an offset a step, not a point, measures granularity:
		// a degree Fahrenheit is 5/9 of a kelvin, and a millidegree Celsius a
		// thousandth of one
		"1 'K' ~ -457 '[degF]'":   "true",
		"273.15 'K' ~ 1.5 'mCel'": "true",
		// A zero carries no precision, however many zeroes it is written with
		"0.000 'g' ~ 0.01 'g'": "true",
		// A converted value keeps the precision it was written with, in its own
		// unit: 200 cm is given to the centimeter, 2.00 m, not to the meter
		"2.4 'm' ~ 200 'cm'":    "false",
		"2 'm' ~ 200 'cm'":      "true",
		"1.501 'kg' ~ 1500 'g'": "false",
		"1.01 'h' ~ 60 'min'":   "false",
		"24 'h' ~ 1.4 'd'":      "false",
		// 100.0 is given to whole grams, not to hundreds
		"100.0 'g' ~ 101 'g'": "false",
		"1 'L' ~ 1.6 'dm3'":   "false",
	} {
		left, right, _ := strings.Cut(expr, " ~ ")
		for _, e := range []string{expr, right + " ~ " + left} {
			if got := evaluateScalar(t, e, patient); got != want {
				t.Errorf("%s = %s, want %s", e, got, want)
			}
		}
	}
}

// TestQuantityEquivalenceTie covers units whose conversion factor is 1: "If the
// conversion factors are 1 (the units are equal), then choose the unit of the
// operator's left argument." So which side a value stands on can decide the
// answer, as rounding then happens in the left unit.
func TestQuantityEquivalenceTie(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	for expr, want := range map[string]string{
		// 274.6 K is 1.45 Cel, compared with 1.5 at tenths: 1.5 = 1.5
		"1.5 'Cel' ~ 274.6 'K'": "true",
		// 1.5 Cel is 274.65 K, compared with 274.6 at tenths: 274.7 is not
		"274.6 'K' ~ 1.5 'Cel'": "false",
		// -272.15 Cel is given to hundredths, so 1.00 K is too: 0.5 is not 1.0
		"0.5 'K' ~ -272.15 'Cel'": "false",
		"1 'L' ~ 1.04 'dm3'":      "true",
		"1.04 'dm3' ~ 1 'L'":      "true",
		// A logarithm has no step, and the left unit is kept as on a tie:
		// 10 '1' is 1 'B'
		"1 'B' ~ 10 '1'": "true",
		"4 '1' ~ 1 'B'":  "false",
	} {
		if got := evaluateScalar(t, expr, patient); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// TestExplicitDurationConversionUsesSourceSystem covers the rule that decides
// which set of factors an explicit conversion uses.
//
// "When explicitly converting between UCUM definite durations and calendar units
// of differing magnitudes, perform the conversion within the unit system of the
// source, then change the unit to the corresponding target unit."
//
// The two expressions below differ only in whether the source is written as a
// calendar keyword or a UCUM code, and that is what decides the answer.
func TestExplicitDurationConversionUsesSourceSystem(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	// 182.5 days → 0.5 year → 0.5 'a', because the calendar puts 365 days in a year
	if got := evaluateScalar(t, "182.5 days.toQuantity('a')", patient); got != "0.5 'a'" {
		t.Errorf("182.5 days.toQuantity('a') = %s, want 0.5 'a'", got)
	}

	// 182.5 'd' stays in UCUM, where a year is 365.25 days
	got := evaluateScalar(t, "182.5 'd'.toQuantity('a')", patient)
	if got == "0.5 'a'" {
		t.Error("182.5 'd'.toQuantity('a') should use UCUM's 365.25, not the calendar's 365")
	}

	// 7 days → 1 week → 1 'wk'
	if got := evaluateScalar(t, "7 days.toQuantity('wk')", patient); got != "1 'wk'" {
		t.Errorf("7 days.toQuantity('wk') = %s, want 1 'wk'", got)
	}
}

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

// A Quantity's code, unit and system are strings like any other, and read
// decoded: "http:BS/BS/unitsofmeasure.org", as PHP writes a slash, is UCUM, so
// a code of a is a year; "mBSu0067" is mg; "BSu00b5g", as Python writes what
// is not ASCII, is µg. A code that is not a string names no unit, which is
// left to the unit.
func TestAQuantityReadsItsUnitDecoded(t *testing.T) {
	for _, tt := range []struct {
		quantity, expr, want string
	}{
		{`{"value":1,"system":"http:BS/BS/unitsofmeasure.org","code":"a"}`, "valueQuantity = 1 year", "[true]"},
		{`{"value":1,"system":"http:BS/BS/unitsofmeasure.org","code":"a"}`, "valueQuantity.toQuantity().toString()", "[1 year]"},
		{`{"value":5,"system":"http://unitsofmeasure.org","code":"mBSu0067"}`, "valueQuantity = 5 'mg'", "[true]"},
		{`{"value":5,"system":"http://unitsofmeasure.org","code":"mBSu0067"}`, "valueQuantity.toQuantity().toString()", "[5 'mg']"},
		{`{"value":5,"unit":"BSu00b5g"}`, "valueQuantity.toQuantity().toString()", "[5 'µg']"},
		{`{"value":5,"code":null,"unit":"mg"}`, "valueQuantity.toQuantity().toString()", "[5 'mg']"},
		{`{"value":5,"code":7,"unit":"mg"}`, "valueQuantity.toQuantity().toString()", "[5 'mg']"},
	} {
		data := escaped(`{"resourceType":"Observation","valueQuantity":` + tt.quantity + `}`)
		result, err := fhirpath.MustCompile(tt.expr).Evaluate(data)
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != tt.want {
			t.Errorf("%s on %s = %s, want %s", tt.expr, data, got, tt.want)
		}
	}
}
