package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// An element written as an empty object has neither a value nor children, and
// ele-1, which every element inherits, says "All FHIR elements must have a
// @value or children": it is not there. Read as an item, it made pat-1's
// name.exists() true on a contact whose name is {}, where the HL7 validator
// fails pat-1. One that holds only an id has content, and is there.
func TestAnElementWrittenAsAnEmptyObjectIsNotThere(t *testing.T) {
	tests := []struct {
		resource, expr, want string
	}{
		{`{"resourceType":"Patient","contact":[{"name":{}}]}`, "contact.name.exists()", "[false]"},
		{`{"resourceType":"Patient","contact":[{"name":{ }}]}`, "contact.name.count()", "[0]"},
		{`{"resourceType":"Patient","contact":[{"name":{}}]}`, "contact.children().count()", "[0]"},
		{`{"resourceType":"Patient","contact":[{"name":{}}]}`, "contact.descendants().count()", "[0]"},
		{`{"resourceType":"Patient","name":[{},{"family":"M"},{}]}`, "name.count()", "[1]"},
		{`{"resourceType":"Patient","name":[{},{"family":"M"},{}]}`, "children().count()", "[1]"},
		{`{"resourceType":"Patient","contact":[{"name":{"id":"x"}}]}`, "contact.name.exists()", "[true]"},
		{`{"resourceType":"Patient","contact":[{"name":{"id":"x"}}]}`, "contact.name.children().count()", "[1]"},
		// ext-1, on an extension whose value is {}.
		{`{"resourceType":"Patient","extension":[{"url":"u","valueAddress":{}}]}`, "extension.value.exists()", "[false]"},
		{`{"resourceType":"Patient","extension":[{"url":"u","valueAddress":{}}]}`, "extension.all(extension.exists() != value.exists())", "[false]"},
		// The element beside a primitive is no different: one written as {}
		// adds nothing to the value, and stands for nothing without one.
		{`{"resourceType":"Patient","birthDate":"2020","_birthDate":{}}`, "birthDate.children().count()", "[1]"},
		{`{"resourceType":"Patient","name":[{"given":[null,"J"],"_given":[{},null]}]}`, "name.given.count()", "[1]"},
		{`{"resourceType":"Patient","_active":{}}`, "active.exists()", "[false]"},
		// A field read directly, as getExtensionValue() reads value[x], is
		// not there either.
		{`{"resourceType":"Patient","extension":[{"url":"u","valueCoding":{}}]}`, "getExtensionValue('u').count()", "[0]"},
		{`{"resourceType":"Patient","extension":[{"url":"u","valueCoding":null}]}`, "getExtensionValue('u').count()", "[0]"},
		// Nor is a field the type is told by without a model.
		{`{"resourceType":"Basic","c":{"system":"s","code":"k","value":{}}}`, "c.type().name", "[Coding]"},
		{`{"resourceType":"Basic","c":{"system":"s","code":"k","value":null}}`, "c.type().name", "[Coding]"},
		// ~ compares what navigation reads: an empty object is no child there
		// either. = compares content as written, and is left as it is.
		{`{"resourceType":"Patient","contact":[{"gender":"male","name":{}},{"gender":"male"}]}`, "contact[0] ~ contact[1]", "[true]"},
		{`{"resourceType":"Patient","contact":[{"gender":"male","name":[{},{"family":"M"}]},{"gender":"male","name":[{"family":"M"}]}]}`, "contact[0] ~ contact[1]", "[true]"},
		{`{"resourceType":"Patient","contact":[{"gender":"male","name":[{}]},{"gender":"male"}]}`, "contact[0] ~ contact[1]", "[true]"},
		{`{"resourceType":"Patient","contact":[{"gender":"male","name":{"id":"x"}},{"gender":"male"}]}`, "contact[0] ~ contact[1]", "[false]"},
		{`{"resourceType":"Patient","contact":[{"gender":"male","name":[{},null]},{"gender":"male"}]}`, "contact[0] ~ contact[1]", "[true]"},
		{`{"resourceType":"Patient","name":[{"family":"x","given":["J"],"_given":[{},null]},{"family":"x","given":["J"]}]}`, "name[0] ~ name[1]", "[true]"},
		{`{"resourceType":"Patient","name":[{"given":[null,"A"]},{"given":["A"]}]}`, "name[0] ~ name[1]", "[true]"},
		{`{"resourceType":"Patient","name":[{"given":[null,"A"],"_given":[{"id":"x"},null]},{"given":["A"]}]}`, "name[0] ~ name[1]", "[false]"},
	}

	for _, tt := range tests {
		t.Run(tt.resource+" "+tt.expr, func(t *testing.T) {
			compiled := fhirpath.MustCompile(tt.expr)
			result, err := compiled.Evaluate([]byte(tt.resource))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}

			doc := fhirpath.MustNewDocument([]byte(tt.resource))
			for i := 0; i < 2; i++ {
				result, err = doc.EvaluateCompiled(compiled)
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s on a Document (pass %d) = %s, want %s", tt.expr, i+1, got, tt.want)
				}
			}
		})
	}
}
