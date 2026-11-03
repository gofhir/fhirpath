package conformance

import (
	"os"
	"testing"
)

// Without a model a value read from a resource carries no FHIR type, only the
// System type of its JSON: a code is a String and a positiveInt an Integer,
// as fhirpath.js 5.2.0 answers with no model too
// (difftest/batteries/no-model/measure.js). A question about a FHIR type is
// then a guess, made by the System type the FHIR type converts to, so any
// String is a code and any Integer an integer64. fhirpath.js ends such a
// question with an error. With a model both engines answer from the type the
// model declares. See CONFORMANCE.md, "Without a model, a FHIR type is a
// guess".
func TestWithoutAModelAFHIRTypeIsAGuess(t *testing.T) {
	data, err := os.ReadFile("../difftest/batteries/no-model/patient.json")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	for _, tc := range []struct{ expr, want string }{
		// Answered alike by fhirpath.js with no model
		{"Patient.telecom.rank is Integer", "[Boolean true]"},
		{"Patient.gender is String", "[Boolean true]"},
		{"Patient.gender.type().name", "[String String]"},
		{"Patient.telecom.rank.type().name", "[String Integer]"},
		// Guessed: right here, as the model answers
		{"Patient.gender is code", "[Boolean true]"},
		{"Patient.telecom.rank is positiveInt", "[Boolean true]"},
		// Guessed: wrong here, where the model answers false and [9]
		{"Patient.telecom.value is code", "[Boolean true]"},
		{"Patient.descendants().ofType(integer64)", "[Integer 2; Integer 5]"},
	} {
		if got := evaluateRendered(doc, tc.expr); got != tc.want {
			t.Errorf("%s with no model = %s, want %s", tc.expr, got, tc.want)
		}
	}
}
