package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// TestSetOperations covers the membership functions, which the argument scope
// had been making unusable: subsetOf's argument is a sibling collection, so it
// was being looked for inside the input.
func TestSetOperations(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		{"name.first().subsetOf(name)", "true"},
		{"name.subsetOf(name.first())", "false"},
		{"name.first().supersetOf(name)", "false"},
		{"name.supersetOf(name.first())", "true"},

		// "Merge the two collections into a single collection, eliminating any
		// duplicate values" — including one the input already held
		{"1.combine(1).count()", "2"},
		{"1.combine(1).union(2).count()", "2"},
		{"(1 | 1 | 2).count()", "2"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, threeNames); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestCollectionsFixes covers what the collections area of the spec cases
// found (#167): combine() takes 3.0.0's preserveOrder, single() of an empty
// input is empty, and skip() and take() refuse a num of several items, as
// fhirpath.js does and as the text says. Up to 1.14.6 combine() refused a
// second argument, single() ended with an error on an empty input, and skip()
// and take() read the first item of num.
func TestCollectionsFixes(t *testing.T) {
	doc := []byte(`{"resourceType":"Patient","active":true,"name":[{"given":["A","B"]},{"given":["C"]}],"_birthDate":{"extension":[{"url":"http://example.org","valueString":"x"}]}}`)
	for expr, want := range map[string]string{
		"1.combine(2, true).combine(1, true).count()":                            "3",
		"(1 | 2).combine(2 | 3, false).count()":                                  "4",
		"1.combine({}, {}).count()":                                              "1",
		"Patient.name.given.combine(Patient.name.given, Patient.active).count()": "6",
		"1.combine(2, Patient.birthDate).count()":                                "2",
		"{}.single().empty()":                                                    "true",
		"Patient.name[1].family.single().empty()":                                "true",
		"1.single()":                       "1",
		"(1 | 2 | 3).skip(1 | {}).count()": "2",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
	for _, expr := range []string{
		"1.combine(2, true | false)",
		"1.combine(2, 'true')",
		"1.combine(2, 1)",
		"1.combine(2, true, true)",
		"(1 | 2).single()",
		"(1 | 2 | 3).skip(1 | 2)",
		"(1 | 2 | 3).take(1 | 2)",
	} {
		if _, err := fhirpath.Evaluate(doc, expr); err == nil {
			t.Errorf("%s: no error, want one", expr)
		}
	}
}

func TestCollectionOperators(t *testing.T) {
	t.Run("union", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "(1 | 2) | (2 | 3)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Union removes duplicates
		if result.Count() != 3 {
			t.Errorf("expected 3 elements, got %d: %v", result.Count(), result)
		}
	})

	t.Run("in membership", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "2 in (1 | 2 | 3)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("contains", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "(1 | 2 | 3) contains 2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})
}

func TestIndexer(t *testing.T) {
	t.Run("array index", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "Patient.name[0].family")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "Doe")
	})

	t.Run("index out of bounds", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "Patient.name[10]")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Empty() {
			t.Errorf("expected empty for out of bounds, got %v", result)
		}
	})
}

// TestSort covers sort(), including ordering keys and the leading minus that
// marks a key as descending.
func TestSort(t *testing.T) {
	names := []byte(`{
		"resourceType": "Patient",
		"name": [
			{"use": "official", "family": "Chalmers", "given": ["Peter", "James"]},
			{"use": "usual", "given": ["Jim"]},
			{"use": "maiden", "family": "Windsor", "given": ["Peter", "James"]}
		]
	}`)

	cases := []struct {
		expr     string
		expected bool
	}{
		{"(1 | 2 | 3).sort() = (1 | 2 | 3)", true},
		{"(3 | 2 | 1).sort() = (1 | 2 | 3)", true},
		{"(1 | 2 | 3).sort($this) = (1 | 2 | 3)", true},
		{"(3 | 2 | 1).sort($this) = (1 | 2 | 3)", true},
		{"(1 | 2 | 3).sort(-$this) = (3 | 2 | 1)", true},
		{"('a' | 'b' | 'c').sort($this) = ('a' | 'b' | 'c')", true},
		{"('c' | 'b' | 'a').sort($this) = ('a' | 'b' | 'c')", true},
		{"('a' | 'b' | 'c').sort(-$this) = ('c' | 'b' | 'a')", true},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			assertBooleanResult(t, evalOrFatal(t, simpleJSON, tc.expr), tc.expected)
		})
	}

	t.Run("sorts navigated values", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, names,
			"Patient.name[0].given.sort() = ('James' | 'Peter')"), true)
	})

	t.Run("multiple descending keys", func(t *testing.T) {
		// Descending, the name without a family, the lowest, comes last
		assertBooleanResult(t, evalOrFatal(t, names,
			"Patient.name.sort(-family, -given.first()).use = ('maiden' | 'official' | 'usual')"), true)
	})

	t.Run("an empty key is lower than any value", func(t *testing.T) {
		// The name without a family has no key for it: first ascending
		assertStringResult(t, evalOrFatal(t, names, "Patient.name.sort(family).first().use"), "usual")
		assertStringResult(t, evalOrFatal(t, names, "Patient.name.sort(-family).last().use"), "usual")
	})

	t.Run("a key of several items is an error", func(t *testing.T) {
		if _, err := fhirpath.Evaluate(simpleJSON, "(1 | 2).sort($this | 3)"); err == nil {
			t.Error("expected an error for a key of two items")
		}
	})

	t.Run("keys that do not compare are an error", func(t *testing.T) {
		for _, expr := range []string{
			"(1 | 'a').sort()",
			"('a' | 2 | 1).sort($this)",
			"(1 | 2 'mg' | 3).sort()",
			"(true | false).sort()",
			// Units that do not convert, and dates whose order is unknown: <
			// answers empty, and values that do not compare must be filtered
			"(1 'mg' | 2 'cm' | 0.5 'mg').sort()",
			"(2 'mg' | 1 'cm').sort()",
			"(@2020-01-15 | @2020-01).sort()",
		} {
			if _, err := fhirpath.Evaluate(simpleJSON, expr); err == nil {
				t.Errorf("%s: expected an error", expr)
			}
		}
	})

	t.Run("whatever order the input comes in", func(t *testing.T) {
		// @2020 orders against @2021-03 and against @2020-01-15 alone, but
		// not against @2020-01-15: every order of the three fails
		for _, expr := range []string{
			"(@2020 | @2021-03 | @2020-01-15).sort()",
			"(@2020 | @2020-01-15 | @2021-03).sort()",
			"(@2021-03 | @2020 | @2020-01-15).sort()",
			"(@2021-03 | @2020-01-15 | @2020).sort()",
			"(@2020-01-15 | @2020 | @2021-03).sort()",
			"(@2020-01-15 | @2021-03 | @2020).sort()",
		} {
			if _, err := fhirpath.Evaluate(simpleJSON, expr); err == nil {
				t.Errorf("%s: expected an error", expr)
			}
		}
	})

	t.Run("whatever order temporals and quantities come in", func(t *testing.T) {
		// Each orders its middle value against the other two, which do not
		// compare: an offset against none, a calendar year against days
		for _, trio := range [][3]string{
			{"@2020-01-01T10:00Z", "@2020-01-02", "@2020-01-03T10:00"},
			{"@2020-01-01", "@2020-01-02T01:00+05:00", "@2020-01-01T22:00-05:00"},
			{"1 year", "400 days", "500 'd'"},
			// DateTimes alone: hour against minute precision across offsets,
			// and an offset against none across precisions
			{"@2020-01-01T10+05:30", "@2020-01-01T05:00Z", "@2020-01-01T10:45+05:30"},
			{"@2020-01-01T10:00Z", "@2020-01-02T", "@2020-01-03T10:00"},
		} {
			for _, p := range [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}} {
				items := trio[p[0]] + ").combine(" + trio[p[1]] + ").combine(" + trio[p[2]]
				for _, key := range []string{"", "$this", "-$this"} {
					expr := "(" + items + ").sort(" + key + ")"
					if _, err := fhirpath.Evaluate(simpleJSON, expr); err == nil {
						t.Errorf("%s: expected an error", expr)
					}
				}
			}
		}
	})

	t.Run("FHIR Quantities sort by their values", func(t *testing.T) {
		obs := []byte(`{"resourceType":"Observation","component":[
		  {"valueQuantity":{"value":3,"unit":"g","system":"http://unitsofmeasure.org","code":"g"}},
		  {"valueQuantity":{"value":500,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}},
		  {"valueQuantity":{"value":2,"unit":"g","system":"http://unitsofmeasure.org","code":"g"}}]}`)
		assertBooleanResult(t, evalOrFatal(t, obs,
			"Observation.component.sort(value).value.value = (500 | 2 | 3)"), true)
		mixed := []byte(`{"resourceType":"Observation","component":[
		  {"valueQuantity":{"value":3,"unit":"g","system":"http://unitsofmeasure.org","code":"g"}},
		  {"valueQuantity":{"value":2,"unit":"cm","system":"http://unitsofmeasure.org","code":"cm"}}]}`)
		if _, err := fhirpath.Evaluate(mixed, "Observation.component.sort(value)"); err == nil {
			t.Error("quantities whose units do not convert: expected an error")
		}
	})

	t.Run("a later key is compared only where the earlier keys tie", func(t *testing.T) {
		// The second key is a string for 1 and an integer elsewhere, which
		// only the two 1s, a string each, are ever compared on
		assertBooleanResult(t, evalOrFatal(t, simpleJSON,
			"(1 | 2 | 3).combine(1).sort($this, iif($this = 1, 'a', 1)) = (1 | 2 | 3).combine(1).sort()"), true)
		// Where the earlier keys tie, the later key must compare: 1 and 1.0
		// tie on the first, and their second keys are 'a' and 1
		if _, err := fhirpath.Evaluate(simpleJSON, "(1).combine(1.0).combine(2).sort($this, iif($this is Integer, 'a', 1))"); err == nil {
			t.Error("tied keys that do not compare: expected an error")
		}
	})

	t.Run("collections of fewer than two elements are returned as is", func(t *testing.T) {
		assertIntegerResult(t, evalOrFatal(t, simpleJSON, "(1).sort().count()"), 1)
		assertEmptyResult(t, evalOrFatal(t, simpleJSON, "nosuchfield.sort()"), "nosuchfield.sort()")
	})

	t.Run("ordering is stable", func(t *testing.T) {
		// Both names share the same family key, so they keep their input order
		result := evalOrFatal(t, names, "Patient.name.sort(given.first()).use")
		if len(result) != 3 {
			t.Fatalf("expected 3 results, got %v", result)
		}
	})
}

// TestMembershipOperatorArity covers the arity rule in and contains state, which
// is stricter than the empty propagation around it.
//
// in: "If the left-hand side of the operator is empty, the result is empty, if
// the right-hand side is empty, the result is false. If the left operand has
// multiple items, an exception is thrown."
//
// contains is the converse, with the sides exchanged. The error matters because
// empty would read as "not found": answering that 'a' is not in a collection
// when the question was which of three values to look for is a wrong answer, not
// an unknown one.
func TestMembershipOperatorArity(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	t.Run("multiple items on the deciding side is an error", func(t *testing.T) {
		for _, expr := range []string{
			"('a' | 'c' | 'd') in 'b'",
			"('a' | 'b') in ('a' | 'b')",
			"'b' contains ('a' | 'c' | 'd')",
		} {
			if _, err := fhirpath.MustCompile(expr).Evaluate(patient); err == nil {
				t.Errorf("%s: expected an error", expr)
			}
		}
	})

	t.Run("the other cases answer", func(t *testing.T) {
		cases := []struct{ expr, want string }{
			{"'a' in ('a' | 'b')", "true"},
			{"'z' in ('a' | 'b')", "false"},
			{"('a' | 'b') contains 'a'", "true"},
			{"('a' | 'b') contains 'z'", "false"},

			// Empty on the deciding side is empty; on the other side it is false
			{"{} in ('a' | 'b')", "EMPTY"},
			{"'a' in {}", "false"},
			{"{} contains 'a'", "false"},
			{"('a' | 'b') contains {}", "EMPTY"},
		}
		for _, tc := range cases {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		}
	})
}
