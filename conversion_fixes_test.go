package fhirpath

import "testing"

// TestConversionsAsTheTextHasThem covers what the conversions area of the
// spec cases found (#167). Up to 1.15.1 convertsToX() of an empty input was
// false; toInteger() truncated a Decimal; toDecimal() read '1.', '.5' and
// '1e5'; toTime() read 'T14:34:28'; toQuantity({}) ignored the empty unit;
// toString() of a FHIR Quantity was empty; toDate() and toDateTime() took no
// format; and toLong() did not exist.
func TestConversionsAsTheTextHasThem(t *testing.T) {
	doc := []byte(`{"resourceType":"Observation","status":"final","code":{"text":"x"},` +
		`"valueQuantity":{"value":185,"unit":"lbs","system":"http://unitsofmeasure.org","code":"[lb_av]"}}`)
	for expr, want := range map[string]string{
		// convertsToX() of an empty input is empty
		"{}.convertsToBoolean().empty()":  "true",
		"{}.convertsToInteger().empty()":  "true",
		"{}.convertsToDate().empty()":     "true",
		"{}.convertsToQuantity().empty()": "true",
		// a Decimal does not convert to an Integer
		"1.5.toInteger().empty()":   "true",
		"185.0.convertsToInteger()": "false",
		"'42'.toInteger()":          "42",
		// a decimal string takes the form the regex gives
		"'1.'.toDecimal().empty()":  "true",
		"'.5'.convertsToDecimal()":  "false",
		"'1e5'.toDecimal().empty()": "true",
		"'-1.5'.toDecimal()":        "-1.5",
		// a time string has no T
		"'T14:34:28'.toTime().empty()": "true",
		"'14:34:28'.toTime()":          "14:34:28",
		// an empty unit is no unit to convert to
		"5 'mg'.toQuantity({}).empty()": "true",
		// a FHIR Quantity is written as the quantity it maps to
		"Observation.value.toString()":         "185 '[lb_av]'",
		"Observation.value.convertsToString()": "true",
		// toLong() reads what toInteger() reads
		"'5000000000'.toLong()": "5000000000",
		"'5'.convertsToLong()":  "true",
		// toDate() and toDateTime() by a template
		"'15-01-2024'.toDate('dd-MM-yyyy')":                                     "2024-01-15",
		"'2024/01/15'.toDate('yyyy/MM/dd')":                                     "2024-01-15",
		"'15-01-2024'.toDate('yyyy-MM-dd').empty()":                             "true",
		"'15/01/2024 10:30'.toDateTime('dd/MM/yyyy HH:mm')":                     "2024-01-15T10:30",
		"'01/15/24 02:05 PM'.toDateTime('MM/dd/yy hh:mm a')":                    "2024-01-15T14:05",
		"'2024-01-15 10:30:00.123+0200'.toDateTime('yyyy-MM-dd HH:mm:ss.SSSZ')": "2024-01-15T10:30:00.123+02:00",
		"'15-01-2024'.convertsToDate('dd-MM-yyyy')":                             "true",
		"@2024-01-15.toDate('dd-MM-yyyy')":                                      "2024-01-15",
		"'2024-01-15'.toDate({}).empty()":                                       "true",
		"'Jan 2024'.toDate('MMM yyyy').empty()":                                 "true",
		"'2024-01-15 30'.toDateTime('yyyy-MM-dd mm').empty()":                   "true",
		"'2024 15'.toDate('yyyy dd').empty()":                                   "true",
		"'2024-01-15 12:00'.toDateTime('yyyy-MM-dd hh:mm').empty()":             "true",
		"'2024-01-15 01:00 PM'.toDateTime('yyyy-MM-dd HH:mm a').empty()":        "true",
		"'2024-01-15 22 10'.toDateTime('yyyy-MM-dd HH hh').empty()":             "true",
		"'2024-01-15 12:00 AM'.toDateTime('yyyy-MM-dd hh:mm a')":                "2024-01-15T00:00",
		"'150124'.toDate('ddMMyy')":                                             "2024-01-15",
		"'2024-01-15'.convertsToDate({}).empty()":                               "true",
		"5 'mg'.convertsToQuantity({}).empty()":                                 "true",
		"'+1.5'.toDecimal().toString()":                                         "1.5",
		"'00001.5'.toDecimal().toString()":                                      "1.5",
		"'-0'.toDecimal().toString()":                                           "0",
		"(10 / 2).toInteger().empty()":                                          "true",
		"'5000000000'.toLong() is Integer":                                      "true",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
	for _, expr := range []string{
		"5 'mg'.toQuantity('mg' | 'g')",
		"'2024-01-15'.toDate('yyyy-MM-dd' | 'dd-MM-yyyy')",
		"'2024-01-15'.toDate(5)",
		"5 'mg'.toQuantity(5)",
	} {
		if _, err := Evaluate(doc, expr); err == nil {
			t.Errorf("%s: no error, want one", expr)
		}
	}
}
