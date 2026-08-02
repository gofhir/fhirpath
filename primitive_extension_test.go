package fhirpath

import "testing"

// FHIR splits a primitive that carries extensions across two JSON fields: the
// value under its own name, and everything else about the element under the same
// name prefixed with an underscore. The two are one element, and FHIRPath
// navigates to it as one.
//
//	"birthDate": "1974-12-25",
//	"_birthDate": {"extension": [{"url": ".../patient-birthTime", ...}]}
//
// Patient.birthDate.extension(url) is the ordinary way to read a birth time, and
// the whole data-absent-reason pattern depends on reaching an extension on a
// value that is not there at all.

var patientWithPrimitiveExtensions = []byte(`{
  "resourceType":"Patient",
  "birthDate":"1974-12-25",
  "_birthDate":{
    "extension":[{
      "url":"http://hl7.org/fhir/StructureDefinition/patient-birthTime",
      "valueDateTime":"1974-12-25T14:35:45-05:00"
    }]
  },
  "name":[{
    "given":["Peter","James"],
    "_given":[null,{"extension":[{"url":"http://example.org/nickname","valueString":"Jim"}]}]
  }]
}`)

// TestExtensionOnPrimitive covers reading an extension from beside a value.
func TestExtensionOnPrimitive(t *testing.T) {
	const birthTime = "http://hl7.org/fhir/StructureDefinition/patient-birthTime"

	cases := []struct {
		expr string
		want string
	}{
		{"Patient.birthDate.extension('" + birthTime + "').exists()", "true"},
		{"Patient.birthDate.extension('" + birthTime + "').value", "1974-12-25T14:35:45-05:00"},
		{"Patient.birthDate.extension('http://example.org/absent').exists()", "false"},

		// The value is untouched by carrying its element
		{"Patient.birthDate", "1974-12-25"},
		{"Patient.birthDate = @1974-12-25", "true"},
		{"Patient.birthDate.hasValue()", "true"},

		// The underscore field is positional, so the extension belongs to the
		// second given name and not the first
		{"Patient.name.given.extension('http://example.org/nickname').valueString", "Jim"},
		{"Patient.name.given.first().extension('http://example.org/nickname').exists()", "false"},
		{"Patient.name.given.last().extension('http://example.org/nickname').exists()", "true"},
		{"Patient.name.given.count()", "2"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patientWithPrimitiveExtensions); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestExtensionWithoutValue covers a position that has extensions and no value.
//
// FHIR writes a null in the value array and puts the element in the underscore
// array at the same index. The position is still an item of the collection —
// dropping it would renumber everything after it — and hasValue() is what
// distinguishes it.
func TestExtensionWithoutValue(t *testing.T) {
	patient := []byte(`{
	  "resourceType":"Patient",
	  "name":[{
	    "use":"maiden","family":"Windsor",
	    "given":[null,"James"],
	    "_given":[{"extension":[{"url":"https://example.org/syllable-count","valueString":"five"}]}]
	  }]
	}`)

	result, err := MustCompile("Patient.name.given.select($this.hasValue())").Evaluate(patient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 || result[0].String() != "false" || result[1].String() != "true" {
		t.Errorf("hasValue() over given = %v, want [false true]", result)
	}

	cases := []struct {
		expr string
		want string
	}{
		// Both positions are present, so the one with a value is still second
		{"Patient.name.given.count()", "2"},
		{"Patient.name.given.last()", "James"},

		// The valueless position is reachable, and carries its extension
		{"Patient.name.given.first().hasValue()", "false"},
		{"Patient.name.given.first().extension('https://example.org/syllable-count').valueString", "five"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestPrimitiveWithoutElementIsUnchanged checks that the common case — a
// primitive with no underscore sibling — behaves as it always did.
func TestPrimitiveWithoutElementIsUnchanged(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","birthDate":"1974-12-25","active":true,"name":[{"given":["Jim"]}]}`)

	cases := []struct {
		expr string
		want string
	}{
		{"Patient.birthDate", "1974-12-25"},
		{"Patient.birthDate.extension('http://example.org/x').exists()", "false"},
		{"Patient.active", "true"},
		{"Patient.name.given.count()", "1"},
		{"Patient.birthDate.hasValue()", "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}
