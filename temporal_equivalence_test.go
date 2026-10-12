package fhirpath

import "testing"

// TestTemporalEquivalence runs the examples FHIRPath 3.0.0 works through for
// Date, DateTime and Time equivalence: "the comparison is the same as for
// equality, with the exception that if the input values have different levels
// of precision, the result is false, rather than empty". ~ compared instants
// and set precision and offset aside, so @2020T ~ @2020-01-01T00:00:00Z was
// true.
func TestTemporalEquivalence(t *testing.T) {
	for expr, want := range map[string]string{
		// The specification's examples
		"@2012 ~ @2012":                                 "true",
		"@2012 ~ @2013":                                 "false",
		"@2012-01 ~ @2012":                              "false",
		"@2012-01-01T10:30 ~ @2012-01-01T10:30":         "true",
		"@2012-01-01T10:30 ~ @2012-01-01T10:31":         "false",
		"@2012-01-01T10:30:31 ~ @2012-01-01T10:30":      "false",
		"@2012-01-01T10:30:31.0 ~ @2012-01-01T10:30:31": "true",
		"@2012-01-01T10:30:31.1 ~ @2012-01-01T10:30:31": "false",
		// Precision differs: false, where = is empty
		"@2020T ~ @2020-01-01T00:00:00Z":             "false",
		"@2020-01-01T10:00Z ~ @2020-01-01T10:00:00Z": "false",
		"@2020-01-01 ~ @2020-01-01T00:00":            "false",
		"@T10:30 ~ @T10:30:00":                       "false",
		// An offset against none: = has no answer, ~ is false
		"@2020-01-01T10:00:00Z ~ @2020-01-01T10:00:00": "false",
		// The same instant at two offsets, and seconds against milliseconds
		"@2020-01-01T10:00:00Z ~ @2020-01-01T05:00:00-05:00": "true",
		"@T10:30:00 ~ @T10:30:00.000":                        "true",
		"@2020-01-01T10:00:00Z !~ @2020-01-01T10:00:00":      "true",
	} {
		if got := evaluateScalar(t, expr, simpleJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}

	// Inside complex values, a date child compares the same way
	periods := []byte(`{"resourceType":"Basic","extension":[
	  {"url":"a","valuePeriod":{"start":"2020-01-01T10:00:00Z"}},
	  {"url":"a","valuePeriod":{"start":"2020-01-01T10:00Z"}},
	  {"url":"a","valuePeriod":{"start":"2020-01-01T05:00:00-05:00"}}]}`)
	for expr, want := range map[string]string{
		"Basic.extension[0].value ~ Basic.extension[1].value": "false",
		"Basic.extension[0].value ~ Basic.extension[2].value": "true",
	} {
		if got := evaluateScalar(t, expr, periods); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
