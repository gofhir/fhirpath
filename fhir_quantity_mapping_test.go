package fhirpath

import "testing"

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
