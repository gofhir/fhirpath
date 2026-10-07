package fhirpath

import (
	"strings"
	"testing"
)

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
			_, err := MustCompile("@2014-01-01 + Observation.value").Evaluate(observationWith(tc.quantity))
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

	if _, err := MustCompile("@2014-01-01 + Observation.value").Evaluate(observationWith(quantity)); err == nil {
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
		{"Claim.total.comparable(10 'mg')", "EMPTY"},
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

	if _, err := MustCompile("Claim.total + 1 'USD'").Evaluate(money); err == nil {
		t.Error("Claim.total + 1 'USD': no error, want a Money refused as a quantity")
	}

	// Two Money values are equal by amount and currency, however each is
	// written, as they were while read as quantities
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

// TestFHIRQuantityComparatorIsRefused covers a quantity whose comparator makes
// it a bound: < 5 'mg' is below 5 mg, not 5 mg. Reading it as 5 mg answered
// for a value nobody measured. Where it would be read as a quantity, against
// another quantity or in any order, the evaluation ends with an error, as it
// does in fhirpath.js; elsewhere it is any object.
func TestFHIRQuantityComparatorIsRefused(t *testing.T) {
	bound := observationWith(`{"value":5,"comparator":"<","system":"http://unitsofmeasure.org","code":"mg"}`)

	refused := func(t *testing.T, data []byte, expr string) {
		t.Helper()
		_, err := MustCompile(expr).Evaluate(data)
		if err == nil || !strings.Contains(err.Error(), "is a bound, not a value") || !strings.Contains(err.Error(), "'<'") {
			t.Errorf("%s: error %v, want the comparator refused", expr, err)
		}
	}

	for _, expr := range []string{
		"Observation.value = 5 'mg'",
		"Observation.value != 5 'mg'",
		"Observation.value ~ 5 'mg'",
		"Observation.value < 6 'mg'",
		"6 'mg' > Observation.value",
		"Observation.value < 'x'",
		"Observation.value + 1 'mg'",
		"Observation.value * 2",
		"Observation.value / 2",
		"@2014-01-01 + Observation.value",
		"Observation.value.comparable(1 'mg')",
		"(1 'mg').comparable(Observation.value)",
		"Observation.value.lowBoundary()",
		"Observation.value.highBoundary()",
		"(Observation.value | 1 'mg' | 10 'mg').sort()",
		"('x' | Observation.value).sort()",
	} {
		t.Run(expr, func(t *testing.T) { refused(t, bound, expr) })
	}

	// Whatever its unit, in an order
	for _, quantity := range []string{
		`{"value":5,"comparator":"<","unit":"tablet"}`,
		`{"value":5,"comparator":"<"}`,
	} {
		refused(t, observationWith(quantity), "Observation.value < 6 'mg'")
	}

	// Elsewhere it is any object: empty still propagates, it is no match for a
	// string, two bounds are equal as objects, and it converts to nothing
	for _, tc := range []struct{ expr, want string }{
		{"Observation.value.toQuantity()", "EMPTY"},
		{"Observation.value.convertsToQuantity()", "false"},
		{"Observation.value.value < 6", "true"},
		{"Observation.value.comparator", "<"},
		{"{} = Observation.value", "EMPTY"},
		{"Observation.value + {}", "EMPTY"},
		{"Observation.value = 'x'", "false"},
		{"'x' in Observation.descendants()", "false"},
		{"Observation.value.comparable(1 'mg' | 2 'mg')", "EMPTY"},
		{"Observation.value = Observation.value", "true"},
		{"(5 'mg' | 'x') ~ (Observation.value | 5 'mg')", "false"},
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

	// Two bounds in arithmetic name the comparator
	pair := []byte(`{"resourceType":"Observation","component":[{"valueQuantity":` + `{"value":4,"comparator":"<","system":"http://unitsofmeasure.org","code":"mg"}` +
		`},{"valueQuantity":{"value":4,"comparator":"<","system":"http://unitsofmeasure.org","code":"mg"}}]}`)
	refused(t, pair, "Observation.component[0].value + Observation.component[1].value")
}

// TestFHIRQuantityBoundSortKey checks that a sort refuses a bound where it
// orders by it, the first item of a key, even beside an empty key that no
// comparison reaches, and not where it is further along a key.
func TestFHIRQuantityBoundSortKey(t *testing.T) {
	withEmpty := []byte(`{"resourceType":"Observation","component":[
		{"valueQuantity":{"value":5,"comparator":"<","system":"http://unitsofmeasure.org","code":"mg"}},
		{"code":{"text":"no value"}}]}`)
	if _, err := MustCompile("Observation.component.sort(value)").Evaluate(withEmpty); err == nil ||
		!strings.Contains(err.Error(), "is a bound, not a value") {
		t.Errorf("beside an empty key: error %v, want the comparator refused", err)
	}

	further := []byte(`{"resourceType":"Observation","component":[
		{"referenceRange":[{"low":{"value":5,"system":"http://unitsofmeasure.org","code":"mg"}},
			{"low":{"value":1,"comparator":"<","system":"http://unitsofmeasure.org","code":"mg"}}]},
		{"referenceRange":[{"low":{"value":3,"system":"http://unitsofmeasure.org","code":"mg"}}]}]}`)
	if got := evaluateScalar(t, "Observation.component.sort(referenceRange.low).first().referenceRange.count()", further); got != "1" {
		t.Errorf("sorted first a component with %s ranges, want the one whose first low is 3 mg", got)
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
