package fhirpath

import (
	"strings"
	"testing"
)

// TestMinMax covers min() and max(), which compare as the comparison
// operators do: "Comparison semantics are defined by the Comparison Operators
// for the type of value being aggregated."
func TestMinMax(t *testing.T) {
	obs := []byte(`{"resourceType":"Observation",
	  "valueQuantity":{"value":5,"comparator":"<","unit":"mg","system":"http://unitsofmeasure.org","code":"mg"},
	  "component":[
	  {"valueQuantity":{"value":3,"unit":"g","system":"http://unitsofmeasure.org","code":"g"}},
	  {"valueQuantity":{"value":500,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}},
	  {"valueQuantity":{"value":1E1000,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}]}`)
	for expr, want := range map[string]string{
		// The specification's examples
		"(2 | 4 | 8 | 6).min()":                           "2",
		"(@2012-12-31 | @2013-01-01 | @2012-01-01).min()": "2012-01-01",
		// Integer against Decimal exactly, strings, times
		"(1 | 2.5).max()":                "2.5",
		"(1 | 1.0000000000000001).max()": "1.0000000000000001",
		"('a' | 'c' | 'b').max()":        "c",
		"(@T10:00 | @T09:00:00).min()":   "09:00:00",
		// Quantities, through their units, FHIR Quantities included
		"(1 'mg' | 2 'g').max()":                          "2 'g'",
		"Observation.component.take(2).value.min().value": "500",
		// A quantity whose value overflows orders against nothing
		"Observation.component.value.min()": "EMPTY",
		// One item is its own extreme, a bound among them
		"Observation.value.max().comparator": "<",
		// Where some pair has no order, the extreme is the item that orders
		// against all and stands beyond them all, whatever the input order
		"(@2019 | @2020-01-01T10Z | @2020-01-01T10:30Z).min()": "2019",
		"(@2020-01-01T10Z | @2020-01-01T10:30Z | @2019).min()": "2019",
		"(@2019 | @2020-01-01T10Z | @2020-01-01T10:30Z).max()": "EMPTY",
		"(@2020-01-01T10:30Z | @2020-01-01T10Z).min()":         "EMPTY",
		"(@2020 | @2020-06 | @2019).max()":                     "EMPTY",
		"(@2020-06 | @2020 | @2019).max()":                     "EMPTY",
		"(1 'mg' | 1 'cm').max()":                              "EMPTY",
		// Two equal items that order differently against a third: the one that
		// orders against both others is the extreme
		"(@2020-06-15T23Z).combine(@2020-06-16T00+01:00).combine(@2020-06-15).max()": "2020-06-16T00+01:00",
		// A temporal with an offset against one without has no order
		"(@2020-01-01 | @2020-01-01T10:00Z).max()": "EMPTY",
		// Types min() does not order
		"(true | false).max()": "EMPTY",
		"{}.max()":             "EMPTY",
	} {
		if got := evaluateScalar(t, expr, obs); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}

	// "All items in the input collection SHALL be the same type, otherwise an
	// exception is thrown"
	for _, expr := range []string{"(1 | 'a').min()", "('a' | @2020).max()"} {
		if _, err := Evaluate(obs, expr); err == nil {
			t.Errorf("%s: expected an error", expr)
		}
	}
}

// TestMinMaxOrderIndependent checks that a collection with unordered pairs
// answers alike in every order.
func TestMinMaxOrderIndependent(t *testing.T) {
	items := []string{"@2019", "@2020-01-01T10Z", "@2020-01-01T10:30Z", "@2021"}
	perms := [][]int{{0, 1, 2, 3}, {3, 2, 1, 0}, {1, 0, 3, 2}, {2, 3, 0, 1}, {1, 2, 0, 3}, {3, 1, 2, 0}}
	for _, fn := range []string{"min", "max"} {
		var first string
		for i, p := range perms {
			parts := make([]string, len(p))
			for j, k := range p {
				parts[j] = items[k]
			}
			got := evaluateScalar(t, "("+strings.Join(parts, ").combine(")+")."+fn+"()", simpleJSON)
			if i == 0 {
				first = got
			} else if got != first {
				t.Errorf("%s() over %v = %s, but %s in another order", fn, parts, got, first)
			}
		}
	}
}
