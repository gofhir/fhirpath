package fhirpath

import "testing"

// A patient with three names, so that a per-name projection has something to
// count and the family names are not all present on every name.
var threeNames = []byte(`{
  "resourceType":"Patient",
  "name":[
    {"use":"official","family":"Chalmers","given":["Peter","James"]},
    {"use":"usual","given":["Jim"]},
    {"use":"maiden","family":"Windsor","given":["Peter","James"]}
  ]
}`)

// TestArgumentsEvaluateInTheOuterScope covers where a function's arguments are
// navigated from.
//
// A function's input is what precedes the dot, but its arguments are not
// navigated from there: in name.given.combine(name.family), family belongs to
// name, not to given. The conformance suite settles it by giving these two the
// same expected result, one written with an explicit $this and one without:
//
//	name.given.combine(name.family)
//	name.given.combine($this.name.family)
//
// Which means the scope in force before the dot is the one the argument sees.
func TestArgumentsEvaluateInTheOuterScope(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		// Five given names and two family names
		{"name.given.combine(name.family).count()", "7"},
		{"name.given.combine($this.name.family).count()", "7"},
		{"name.given.combine(name.family).exclude('Jim').count()", "6"},

		// Inside a projection the scope is the item being projected, so use and
		// given are both read from the same name
		{"name.select(use.union(given)).count()", "8"},
		{"name.select(use.union($this.given)).count()", "8"},
		{"name.first().select(use.union(given)).count()", "3"},

		// The input is still the input: it is only the arguments that are
		// navigated from outside
		{"name.given.count()", "5"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, threeNames); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

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

// TestArithmeticEdges covers division by a zero divisor, which is not an error,
// and the types div and mod accept.
func TestArithmeticEdges(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// "12 / 0 // empty ({ })", and the same for div and mod
		{"1 / 0", "EMPTY"},
		{"12 / 0", "EMPTY"},
		{"5 div 0", "EMPTY"},
		{"5 mod 0", "EMPTY"},
		{"1.5 / 0", "EMPTY"},

		// "supported for Integer, Long and Decimal", with the result taking the
		// input's type
		{"5 div 2", "2"},
		{"5.5 div 0.7", "7"},
		{"2.2 div 1.8", "1"},
		{"5 mod 2", "1"},
		{"2.2 mod 1.8", "0.4"},

		// abs() is defined over Quantity too, and keeps the unit
		{"(-5.5 'mg').abs()", "5.5 'mg'"},
		{"(-5).abs()", "5"},
		{"(-5.5).abs()", "5.5"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestDecimalEquivalence covers the rounding that separates ~ from =.
//
// "Decimal: values must be equal, comparison is done on values rounded to the
// precision of the least precise operand. Trailing zeroes after the decimal are
// ignored in determining precision."
func TestDecimalEquivalence(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		// The quotient is 0.666..., which equals nothing, but rounded to the two
		// places 0.67 carries it is 0.67
		{"1.2 / 1.8 ~ 0.67", "true"},
		{"0.67 ~ 1.2 / 1.8", "true"},
		{"1.2 / 1.8 = 0.67", "false"},

		{"1.234 ~ 1.2", "true"},
		{"1.2 ~ 1.234", "true"},
		{"1.2 ~ 1.3", "false"},

		// Trailing zeroes do not add precision
		{"1.10 ~ 1.1", "true"},
		{"1.0 ~ 1", "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}
