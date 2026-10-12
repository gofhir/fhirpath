package fhirpath_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofhir/fhirpath"
)

// An element written as an empty object, or as one under which nothing holds a
// value, has neither a value nor children, and ele-1, which every element
// inherits, says "All FHIR elements must have a @value or children". The HL7
// validator reads it as an item of its collection, so count(), indexing and
// all() see it, and answers exists() and empty() as though it were not there:
// pat-1 fails on a contact whose name is {}, and so does bdl-4 on an entry
// written {}. Each answer below is what the HL7 validator 6.10.2 gives.
func TestAnElementThatHoldsNoValueIsAnItemThatDoesNotExist(t *testing.T) {
	const bundle = `{"resourceType":"Bundle","type":"transaction-response","entry":[{},{"response":{"status":"200"}}]}`
	const contact = `{"resourceType":"Patient","contact":[{"name":{}}]}`
	const names = `{"resourceType":"Patient","name":[{},{"family":"M"},{}]}`
	const extension = `{"resourceType":"Patient","extension":[{"url":"u","valueAddress":{}}]}`
	tests := []struct {
		resource, expr, want string
	}{
		// bdl-4, on an entry written {}.
		{bundle, "Bundle.entry.count()", "[2]"},
		{bundle, "Bundle.entry.all(response.exists())", "[false]"},
		{bundle, "Bundle.entry[0].count()", "[1]"},
		{bundle, "Bundle.entry[0].exists()", "[false]"},
		{bundle, "Bundle.entry[0].empty()", "[true]"},
		{bundle, "Bundle.entry.exists()", "[true]"},
		{bundle, "Bundle.entry.empty()", "[false]"},
		{bundle, "Bundle.entry.first().exists()", "[false]"},
		{bundle, "Bundle.entry.where(response.empty()).count()", "[1]"},
		// exists() with criteria counts every item the criteria keep.
		{bundle, "Bundle.entry.exists(response.empty())", "[true]"},
		// pat-1, on a contact whose name is {}.
		{contact, "Patient.contact.all(name.exists() or telecom.exists() or address.exists() or organization.exists())", "[false]"},
		{contact, "Patient.contact.name.exists()", "[false]"},
		{contact, "Patient.contact.name.empty()", "[true]"},
		{contact, "Patient.contact.name.count()", "[1]"},
		{contact, "Patient.contact.name.exists(true)", "[true]"},
		{contact, "Patient.contact.name.where(true).exists()", "[false]"},
		{contact, "Patient.contact.children().count()", "[1]"},
		{contact, "Patient.contact.descendants().count()", "[1]"},
		{contact, "Patient.contact.name.descendants().count()", "[0]"},
		// Nothing under the contact, nor under the resource, holds a value.
		{contact, "Patient.contact.exists()", "[false]"},
		{contact, "Patient.contact.count()", "[1]"},
		{contact, "Patient.exists()", "[false]"},
		{`{"resourceType":"Patient"}`, "Patient.exists()", "[false]"},
		{`{"resourceType":"Patient"}`, "%resource.exists()", "[false]"},
		{`{"resourceType":"Bundle","type":"collection","entry":[{"resource":{"resourceType":"Patient"}}]}`, "Bundle.entry.resource.count()", "[1]"},
		{`{"resourceType":"Bundle","type":"collection","entry":[{"resource":{"resourceType":"Patient"}}]}`, "Bundle.entry.resource.exists()", "[false]"},
		// Items are where the JSON writes them.
		{names, "Patient.name.count()", "[3]"},
		{names, "Patient.name[1].family", "[M]"},
		{names, "Patient.name.exists()", "[true]"},
		{names, "Patient.name.empty()", "[false]"},
		{names, "Patient.children().count()", "[3]"},
		{names, "Patient.name.where(family.empty()).count()", "[2]"},
		{names, "Patient.name.where(family.empty()).exists()", "[false]"},
		// Holding no value goes all the way down; an id, or a string even
		// when empty, is a value.
		{`{"resourceType":"Patient","contact":[{"name":{"given":[]}}]}`, "Patient.contact.name.exists()", "[false]"},
		{`{"resourceType":"Patient","contact":[{"name":{"extension":[{}]}}]}`, "Patient.contact.name.exists()", "[false]"},
		{`{"resourceType":"Patient","contact":[{"name":{"id":"x"}}]}`, "Patient.contact.name.exists()", "[true]"},
		{`{"resourceType":"Patient","contact":[{"name":{"family":""}}]}`, "Patient.contact.name.exists()", "[true]"},
		{`{"resourceType":"Patient","contact":[{"name":{"given":[{}, [], null]}}]}`, "Patient.contact.name.exists()", "[false]"},
		{`{"resourceType":"Patient","contact":[{"name":{"given":[{}, "J", null]}}]}`, "Patient.contact.name.exists()", "[true]"},
		{`{"resourceType":"Patient","contact":[{"name":{"given":[ ]}}]}`, "Patient.contact.name.exists()", "[false]"},
		// ext-1, on an extension whose value is {}.
		{extension, "Patient.extension.value.exists()", "[false]"},
		{extension, "Patient.extension.value.count()", "[1]"},
		{extension, "Patient.extension.all(extension.exists() != value.exists())", "[false]"},
		{`{"resourceType":"Patient","extension":[{"url":"u","valueCoding":{}}]}`, "getExtensionValue('u').count()", "[1]"},
		{`{"resourceType":"Patient","extension":[{"url":"u","valueCoding":{}}]}`, "getExtensionValue('u').exists()", "[false]"},
		{`{"resourceType":"Patient","extension":[{"url":"u","valueCoding":null}]}`, "getExtensionValue('u').count()", "[0]"},
		// The element beside a primitive: {} beside a null is the item.
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active.count()", "[1]"},
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active.exists()", "[false]"},
		{`{"resourceType":"Patient","name":[{"given":[null,"J"],"_given":[{},null]}]}`, "Patient.name.given.count()", "[2]"},
		{`{"resourceType":"Patient","name":[{"given":[null,"J"],"_given":[{},null]}]}`, "Patient.name.given.exists()", "[true]"},
		{`{"resourceType":"Patient","name":[{"given":[null,"J"],"_given":[{},null]}]}`, "Patient.name.given.first().exists()", "[false]"},
		// Its value is null, so where a value is wanted it is empty, not an
		// object that a Boolean reading takes for true.
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active or false", "[]"},
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active.not()", "[]"},
		{`{"resourceType":"Patient","_active":{}}`, "iif(Patient.active, 'a', 'b')", "[b]"},
		{`{"resourceType":"Patient","_active":{"extension":[{"url":"u","valueString":"x"}]}}`, "Patient.active or false", "[]"},
		{`{"resourceType":"Patient","_active":{"extension":[{"url":"u","valueString":"x"}]}}`, "Patient.active.not()", "[]"},
		{`{"resourceType":"Patient","_active":{"extension":[{"url":"u","valueString":"x"}]}}`, "Patient.active.exists()", "[true]"},
		{`{"resourceType":"Patient","active":true,"_active":{}}`, "Patient.active or false", "[true]"},
		// Not measured: the HL7 validator raises a NullPointerException.
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active = true", "[]"},
		{`{"resourceType":"Patient","_birthDate":{}}`, "Patient.birthDate < @2020", "[]"},
		{`{"resourceType":"Patient","_birthDate":{}}`, "Patient.birthDate + 1 day", "[]"},
		{`{"resourceType":"Patient","name":[{"given":[null],"_given":[{}]}]}`, "Patient.name.given.length()", "[]"},
		{`{"resourceType":"Patient","name":[{"given":[null],"_given":[{}]}]}`, "Patient.name.given + 'x'", "[]"},
		// ~ compares children as navigation reads them: a member written {}
		// is a child, and an element beside a value that holds none is not.
		{`{"resourceType":"Patient","contact":[{"gender":"male","name":{}},{"gender":"male"}]}`, "Patient.contact[0] ~ Patient.contact[1]", "[false]"},
		{`{"resourceType":"Patient","name":[{"family":"x","given":["J"],"_given":[{}]},{"family":"x","given":["J"]}]}`, "Patient.name[0] ~ Patient.name[1]", "[true]"},
		{`{"resourceType":"Patient","name":[{"given":[null,"A"],"_given":[{},null]},{"given":["A"]}]}`, "Patient.name[0] ~ Patient.name[1]", "[false]"},
		{`{"resourceType":"Patient","name":[{"given":[null,"A"],"_given":[{"id":"x"},null]},{"given":["A"]}]}`, "Patient.name[0] ~ Patient.name[1]", "[false]"},
		// Not measured in the HL7 validator, which reads no such JSON: a null
		// with no element beside it is no item, there or for ~.
		{`{"resourceType":"Patient","name":[{"given":[null,"A"]},{"given":["A"]}]}`, "Patient.name[0] ~ Patient.name[1]", "[true]"},
		{`{"resourceType":"Patient","contact":[{"name":[null]},{}]}`, "Patient.contact[0] ~ Patient.contact[1]", "[true]"},
		// Nor is a field the type is told by without a model.
		{`{"resourceType":"Basic","c":{"system":"s","code":"k","value":{}}}`, "c.type().name", "[Coding]"},
		{`{"resourceType":"Basic","c":{"system":"s","code":"k","value":null}}`, "c.type().name", "[Coding]"},
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

// TestExistsReadsAResourceOnlyUpToAValue checks that exists() on a resource
// costs no more than reading it up to its first value. A Bundle that writes
// its entries before its type would otherwise be read whole for each entry
// here, and 20000 entries took minutes.
func TestExistsReadsAResourceOnlyUpToAValue(t *testing.T) {
	if testing.Short() {
		t.Skip("reads a large Bundle")
	}
	var bundle strings.Builder
	bundle.WriteString(`{"resourceType":"Bundle","entry":[`)
	const entries = 20000
	for i := 0; i < entries; i++ {
		if i > 0 {
			bundle.WriteString(",")
		}
		fmt.Fprintf(&bundle, `{"resource":{"resourceType":"Patient","id":"p%d"}}`, i)
	}
	bundle.WriteString(`],"type":"collection"}`)

	start := time.Now()
	result, err := fhirpath.Evaluate([]byte(bundle.String()), "Bundle.entry.where(%resource.exists()).count()")
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != fmt.Sprintf("[%d]", entries) {
		t.Errorf("count = %s, want %d", got, entries)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %v: exists() reads the whole Bundle for each entry", elapsed)
	}
}
