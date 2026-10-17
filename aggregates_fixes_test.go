package fhirpath

import "testing"

// TestAggregatesAsTheTextHasThem covers what the aggregates area of the spec
// cases found (#167). Up to 1.16.0 $total kept only the first item of the
// aggregator's result; init was evaluated on aggregate()'s input rather than
// the outer context; sum() of an empty input was 0; sum() and avg() refused
// quantities; and items of different types gave empty rather than an error.
func TestAggregatesAsTheTextHasThem(t *testing.T) {
	doc := []byte(`{"resourceType":"Observation","status":"final","code":{"text":"x"},` +
		`"component":[{"code":{"text":"a"},"valueQuantity":{"value":2,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}},` +
		`{"code":{"text":"b"},"valueQuantity":{"value":3,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}]}`)
	for expr, want := range map[string]string{
		// $total is the aggregator's whole result
		"(1 | 2 | 3).aggregate($total | $this, {}).count()": "3",
		"(1 | 2).aggregate($this | $total, 10).count()":     "3",
		// init is evaluated on the outer context: $this there is select()'s item
		"(1 | 2).select((10 | 20).aggregate($total + $this, $this)).sum()": "63",
		// sum() of nothing is nothing
		"{}.sum().empty()": "true",
		"{}.avg().empty()": "true",
		// quantities sum and average, a FHIR Quantity among them
		"(1 'mg' | 2 'mg').sum()":           "3 'mg'",
		"(1 'cm' | 2 'cm').avg()":           "1.5 'cm'",
		"Observation.component.value.sum()": "5 'mg'",
		"(1 'g' | 500 'mg').sum()":          "1500 'mg'",
		"(1 'mg' | 2 'm').sum().empty()":    "true",
		"(1 | 2 | 3).sum()":                 "6",
		"(1 | 2).avg()":                     "1.5",
		"Observation.component.value.combine(Observation.extension.value).sum()": "5 'mg'",
		"(1 | 2 | 3).aggregate($total + $this, $this.count())":                   "7",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
	valueless := []byte(`{"resourceType":"Observation","status":"final","code":{"text":"x"},` +
		`"component":[{"code":{"text":"a"},"valueInteger":7},{"code":{"text":"b"},"_valueInteger":{"extension":[{"url":"http://x","valueString":"y"}]}}]}`)
	if got := evaluateScalar(t, "Observation.component.value.sum().empty()", valueless); got != "true" {
		t.Errorf("sum() with a primitive with no value = %s, want empty", got)
	}
	// $total is held to the maximum collection size
	if _, err := MustCompile("(1|2|3|4|5|6|7|8|9|10|11|12|13|14|15).aggregate($total.combine($total), 1)").
		EvaluateWithOptions(doc, WithMaxCollectionSize(1000)); err == nil {
		t.Error("an aggregate past the maximum collection size: no error, want one")
	}
	for _, expr := range []string{
		"(1 | 'a').sum()", "(1.0 | 'a').avg()", "(1 'mg' | 2).sum()",
		// a type the functions do not take, though all the items are of it
		"('a' | 'b').sum()", "(true | false).avg()",
	} {
		if _, err := Evaluate(doc, expr); err == nil {
			t.Errorf("%s: no error, want one", expr)
		}
	}
}
