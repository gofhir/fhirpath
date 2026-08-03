package fhirpath

import "testing"

// The FHIRPath string literals HL7 publishes for these invariants, verbatim.
// They are Go raw strings so that the escaping below is the specification's and
// not this file's: FHIRPath unescapes \\s to the regex \s, and \\' to an
// apostrophe.
const (
	eld19Pattern = `'[A-Za-z][A-Za-z0-9]*(\\.[a-zA-Z0-9]+(\\[x\\])?)*(\\:[^\\s\\.]+)?(\\.[a-zA-Z0-9]+(\\[x\\])?(\\:[^\\s\\.]+)?)*'`
	eld20Pattern = `'[^\\s\\.,:;\\\'"\\/|?!@#$%&*()\\[\\]{}]{1,64}'`
)

// TestPublishedInvariantsEvaluate covers eld-19 and eld-20 through the engine,
// the way a validator reaches them.
//
// Both are invariants HL7 publishes against ElementDefinition, and both were
// refused by a pattern check that ran before the regex was compiled. eld-19 is
// SHALL-level, so while it was refused ElementDefinition.path went unchecked
// and a malformed path passed. The failure was a property of the expression
// rather than of the data, so it repeated once per element: a 200-element
// snapshot raised it some four hundred times.
//
// The assertions use matchesFull, because matches() looks for the pattern
// anywhere in the value — '.leadingDot' contains 'leadingDot', which matches.
func TestPublishedInvariantsEvaluate(t *testing.T) {
	doc := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		pattern string
		value   string
		want    string
	}{
		{eld19Pattern, "Patient.name.given", "true"},
		{eld19Pattern, "Observation.value[x]", "true"},
		{eld19Pattern, "ElementDefinition.path", "true"},
		{eld19Pattern, "Patient.extension:race", "true"},
		{eld19Pattern, ".leadingDot", "false"},
		{eld19Pattern, "has space", "false"},
		{eld19Pattern, "1startsWithADigit", "false"},

		{eld20Pattern, "given", "true"},
		{eld20Pattern, "has space", "false"},
		{eld20Pattern, "has.punctuation", "false"},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			expr := "'" + tc.value + "'.matchesFull(" + tc.pattern + ")"
			if got := evaluateScalar(t, expr, doc); got != tc.want {
				t.Errorf("%s = %s, want %s", expr, got, tc.want)
			}
		})
	}
}
