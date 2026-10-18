package fhirpath_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
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
		"true.max()":           "EMPTY",
		"{}.max()":             "EMPTY",
	} {
		if got := evaluateScalar(t, expr, obs); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}

	// "All items in the input collection SHALL be the same type, otherwise an
	// exception is thrown"
	for _, expr := range []string{"(1 | 'a').min()", "('a' | @2020).max()"} {
		if _, err := fhirpath.Evaluate(obs, expr); err == nil {
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

// TestMinMaxUnorderedIsLinear checks that items alike are ruled out alike:
// many equal items and one they have no order against, or items in groups
// that rule one another out in turn, are checked in a pass each rather than
// one pass per item.
func TestMinMaxUnorderedIsLinear(t *testing.T) {
	values := make([]string, 0, 20001)
	for range 20000 {
		values = append(values, `{"valueDateTime":"2020-06-15"}`)
	}
	values = append(values, `{"valueDateTime":"2020"}`)
	many := []byte(`{"resourceType":"Observation","component":[` + strings.Join(values, ",") + `]}`)

	alternating := make([]string, 0, 5000)
	for i := range 3750 {
		v := "2020-06-15T23:00:00Z"
		if i%2 == 1 {
			v = "2020-06-16"
		}
		alternating = append(alternating, `{"valueDateTime":"`+v+`"}`)
	}
	for range 1250 {
		alternating = append(alternating, `{"valueDateTime":"2020-06-16T00:00:00+01:00"}`)
	}
	groups := []byte(`{"resourceType":"Observation","component":[` + strings.Join(alternating, ",") + `]}`)

	start := time.Now()
	for _, doc := range [][]byte{many, groups} {
		if got := evaluateScalar(t, "Observation.component.value.max()", doc); got != "EMPTY" {
			t.Errorf("max() = %s, want EMPTY", got)
		}
	}
	// Checking each item against every other took tens of seconds here
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %s", elapsed)
	}
}

// TestMinMaxDefaultOffsets checks that two DateTimes written alike but placed
// at different default offsets are not taken for one: they name different
// instants, and the extreme is found whatever the input order.
func TestMinMaxDefaultOffsets(t *testing.T) {
	parse := func(s string) types.DateTime {
		dt, err := types.NewDateTime(s)
		if err != nil {
			t.Fatal(err)
		}
		return dt
	}
	y := parse("2020-06-15T15-09:00")
	early := parse("2020-06-16T00").WithDefaultOffset(time.Hour)
	late := parse("2020-06-16T00").WithDefaultOffset(0)
	z, err := types.NewDate("2020-06-15")
	if err != nil {
		t.Fatal(err)
	}
	expr := fhirpath.MustCompile("(%a.combine(%b).combine(%c).combine(%d)).max()")
	for _, order := range [][4]types.Value{{y, early, late, z}, {late, y, early, z}, {z, early, y, late}} {
		ctx := eval.NewContextForRoot(types.Collection{})
		for i, name := range []string{"a", "b", "c", "d"} {
			ctx.SetVariable(name, types.Collection{order[i]})
		}
		got, err := expr.EvaluateWithContext(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Errorf("max() over %v = %v, want one item", order, got)
		}
	}
}

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
	if _, err := fhirpath.MustCompile("(1|2|3|4|5|6|7|8|9|10|11|12|13|14|15).aggregate($total.combine($total), 1)").
		EvaluateWithOptions(doc, fhirpath.WithMaxCollectionSize(1000)); err == nil {
		t.Error("an aggregate past the maximum collection size: no error, want one")
	}
	for _, expr := range []string{
		"(1 | 'a').sum()", "(1.0 | 'a').avg()", "(1 'mg' | 2).sum()",
		// a type the functions do not take, though all the items are of it
		"('a' | 'b').sum()", "(true | false).avg()",
	} {
		if _, err := fhirpath.Evaluate(doc, expr); err == nil {
			t.Errorf("%s: no error, want one", expr)
		}
	}
}

// Test aggregate() function with $total and $index — GitHub issue #8
func TestAggregateIntegration(t *testing.T) {
	t.Run("sum with aggregate", func(t *testing.T) {
		resource := []byte(`{
			"resourceType": "Observation",
			"component": [
				{"valueInteger": 10},
				{"valueInteger": 20},
				{"valueInteger": 30}
			]
		}`)

		expr := fhirpath.MustCompile("component.valueInteger.aggregate($this + $total, 0)")
		result, err := expr.Evaluate(resource)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Empty() {
			t.Fatal("expected non-empty result")
		}
		if result[0].String() != "60" {
			t.Errorf("expected 60, got %s", result[0].String())
		}
	})

	t.Run("aggregate without init returns empty for empty input", func(t *testing.T) {
		resource := []byte(`{"resourceType": "Patient"}`)
		expr := fhirpath.MustCompile("name.aggregate($this + $total)")
		result, err := expr.Evaluate(resource)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Empty() {
			t.Errorf("expected empty result, got %d elements", len(result))
		}
	})

	t.Run("aggregate with $index", func(t *testing.T) {
		resource := []byte(`{
			"resourceType": "Patient",
			"name": [
				{"family": "A"},
				{"family": "B"},
				{"family": "C"}
			]
		}`)

		// Use $index to count elements: aggregate($total + 1, 0)
		expr := fhirpath.MustCompile("name.aggregate($total + 1, 0)")
		result, err := expr.Evaluate(resource)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Empty() {
			t.Fatal("expected non-empty result")
		}
		if result[0].String() != "3" {
			t.Errorf("expected 3, got %s", result[0].String())
		}
	})
}
