package fhirpath

import "testing"

// The specification states the rule these tests cover as an algorithm, under
// "Singleton Evaluation of Collections":
//
//	IF the collection contains a single node AND the node's value can be
//	  implicitly converted to the expected input type THEN
//	  The collection evaluates to the value of that single node
//	ELSE IF the collection contains a single node AND the expected input type
//	  is Boolean THEN the collection evaluates to true
//	ELSE IF the collection is empty THEN an empty collection
//	ELSE The evaluation will end and signal an error
//
// Two of those branches were missing. A collection of several items was being
// answered rather than refused, and an item of the wrong type was being coerced
// — an Identifier rendered as JSON text and then asked whether it starts with
// something, which yields a confident false about a question that was never
// meaningful.

var appointment = []byte(`{
  "resourceType":"Appointment",
  "identifier":[{"system":"http://example.org/sampleappointment-identifier","value":"123"}],
  "status":"proposed"
}`)

// TestSingletonRuleRejectsMultipleItems covers the final branch: more than one
// item is an error, because which of them was meant is not something to guess.
func TestSingletonRuleRejectsMultipleItems(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	for _, expr := range []string{
		"(1 | 2).not()",
		"(1 | 2 | 3) & 'b'",
		"('a' | 'b').startsWith('a')",
		"('a' | 'b').length()",
	} {
		t.Run(expr, func(t *testing.T) {
			if _, err := MustCompile(expr).Evaluate(patient); err == nil {
				t.Errorf("%s: expected an error for a multi-item input", expr)
			}
		})
	}
}

// TestSingletonRuleRejectsWrongType covers the case of a single item that is
// not of the expected type and cannot convert to it.
//
// Nothing converts implicitly to String — the specification's implicit
// conversions are Integer to Long to Decimal to Quantity, and Date to DateTime —
// so an Identifier or an Integer is an error here, not a string to test.
func TestSingletonRuleRejectsWrongType(t *testing.T) {
	for _, expr := range []string{
		"Appointment.identifier.startsWith('rand')",
		"Appointment.identifier.endsWith('rand')",
		"Appointment.identifier.contains('rand')",
	} {
		t.Run(expr, func(t *testing.T) {
			if _, err := MustCompile(expr).Evaluate(appointment); err == nil {
				t.Errorf("%s: expected an error for a non-String input", expr)
			}
		})
	}
}

// TestSingletonRuleKeepsTheOtherBranches checks that the two branches which do
// have an answer still give it.
func TestSingletonRuleKeepsTheOtherBranches(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[{"given":["Jim"]}]}`)

	cases := []struct {
		expr string
		want string
	}{
		// A single node of the expected type
		{"'abc'.startsWith('a')", "true"},
		{"true.not()", "false"},
		{"'a' & 'b'", "ab"},
		{"name.given.startsWith('J')", "true"},

		// A single node where a Boolean is expected evaluates to true
		{"name.given.not()", "false"},

		// Empty stays empty — except for &, which the specification defines to
		// treat an empty operand as an empty string
		{"{}.startsWith('a')", "EMPTY"},
		{"{}.not()", "EMPTY"},
		{"{} & 'b'", "b"},
		{"'a' & {}", "a"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestDelimitedTypeNames checks that a type name written with backticks names
// the same type as one written plainly.
//
// The grammar admits DELIMITEDIDENTIFIER wherever it admits IDENTIFIER, which is
// what lets a type whose name collides with a keyword be written at all. The
// backticks escape the name; they are not part of it.
func TestDelimitedTypeNames(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[{"given":["Jim"]}]}`)

	cases := []struct {
		expr string
		want string
	}{
		{"Patient.is(FHIR.`Patient`)", "true"},
		{"Patient.is(FHIR.Patient)", "true"},
		{"Patient.ofType(FHIR.`Patient`).type().name", "Patient"},
		{"Patient.ofType(FHIR.Patient).type().name", "Patient"},
		{"Patient.`name`.given.first()", "Jim"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}
