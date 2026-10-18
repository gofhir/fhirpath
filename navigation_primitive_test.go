package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

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

	result, err := fhirpath.MustCompile("Patient.name.given.select($this.hasValue())").Evaluate(patient)
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
		// = compares its null, which equals no value.
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active = true", "[false]"},
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active = false", "[false]"},
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active != true", "[true]"},
		// A Boolean with no value is neither true nor false.
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active.allTrue()", "[false]"},
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active.anyTrue()", "[false]"},
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active.allFalse()", "[false]"},
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active.anyFalse()", "[false]"},
		// join() leaves it out.
		{`{"resourceType":"Patient","name":[{"given":[null,"a"],"_given":[{},null]}]}`, "Patient.name.given.join(',')", "[a]"},
		// Not measured: the HL7 validator raises a NullPointerException.
		{`{"resourceType":"Patient","_birthDate":{}}`, "Patient.birthDate < @2020", "[]"},
		{`{"resourceType":"Patient","_birthDate":{}}`, "Patient.birthDate + 1 day", "[]"},
		{`{"resourceType":"Patient","name":[{"given":[null],"_given":[{}]}]}`, "Patient.name.given.length()", "[]"},
		// The HL7 validator answers nullx, Java's concatenation of a null; +
		// propagates empty, as it does for any empty operand.
		{`{"resourceType":"Patient","name":[{"given":[null],"_given":[{}]}]}`, "Patient.name.given + 'x'", "[]"},
		// Nor do the other operators and functions that want a value.
		{`{"resourceType":"Patient","_multipleBirthInteger":{}}`, "-Patient.multipleBirth", "[]"},
		{`{"resourceType":"Patient","_gender":{}}`, "Patient.gender & 'x'", "[x]"},
		{`{"resourceType":"Patient","_birthDate":{}}`, "Patient.birthDate.duration(@2020, 'years')", "[]"},
		{`{"resourceType":"Patient","_birthDate":{}}`, "Patient.birthDate.difference(@2020, 'years')", "[]"},
		{`{"resourceType":"Patient","_birthDate":{}}`, "@2020.duration(Patient.birthDate, 'years')", "[]"},
		// A resourceType written with escapes is still the resource's name.
		{`{"resourceType":"Patient","contained":[{"resource\u0054ype":"Patient"}]}`, "Patient.contained.exists()", "[false]"},
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

// FHIR writes a primitive's id and extensions in the element beside it, under
// _name. They belong to the primitive: birthDate.extension is the extensions
// of birthDate. extension(url) and descendants() reached them, while the
// extension property and children() did not, so CH Core's ch-core-hm-3, which
// compares descendants().extension with family.extension, failed on every
// example that puts the extension on family.
func TestAPrimitivesElementIsReachedByNavigation(t *testing.T) {
	patient := `{"resourceType":"Patient","birthDate":"2020",` +
		`"_birthDate":{"id":"b1","extension":[{"url":"u","valueCode":"o"}]},` +
		`"name":[{"family":"M","_family":{"extension":[{"url":"u","valueCode":"o"}]},` +
		`"given":[null,"James"],"_given":[{"extension":[{"url":"u","valueCode":"absent"}]},null]}]}`

	tests := []struct {
		expr, want string
	}{
		{"birthDate.extension('u').count()", "[1]"},
		{"birthDate.extension.count()", "[1]"},
		{"birthDate.extension.where(url = 'u').value", "[o]"},
		{"birthDate.id", "[b1]"},
		{"name.family.extension.count()", "[1]"},
		{"name.given.extension.value", "[absent]"},
		// Its id and extension. Its value is not a child: see CONFORMANCE.md.
		{"birthDate.children().count()", "[2]"},
		{"birthDate.descendants().count() > birthDate.children().count()", "[true]"},
		// A primitive and its element are one child of their parent, not two,
		// and a value with only an element is still a child; resourceType is
		// not one.
		{"children().count()", "[2]"},
		{"name.children().count()", "[3]"},
		{"name.descendants().extension.count()", "[2]"},
		// ch-core-hm-3's shape.
		{"descendants().extension.where(url = 'u').count() = (birthDate | name.family | name.given).extension.where(url = 'u').count()", "[true]"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			compiled := fhirpath.MustCompile(tt.expr)
			result, err := compiled.Evaluate([]byte(patient))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}

			doc := fhirpath.MustNewDocument([]byte(patient))
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

// Only what FHIR writes beside a primitive — an object, or an array of objects
// and nulls — is taken as its element. Any other field named with an
// underscore is a field like any other, and a repeated key keeps every
// occurrence, whichever way an object's children are read.
func TestAFieldThatIsNotAPrimitivesElementIsAChildAsItStands(t *testing.T) {
	tests := []struct {
		resource, expr, want string
	}{
		{`{"resourceType":"Patient","_tags":["a","b"],"_meta":"x"}`, "children().count()", "[3]"},
		{`{"resourceType":"Patient","_tags":["a","b"],"_meta":"x"}`, "_tags", "[a, b]"},
		{`{"resourceType":"Patient","_tags":["a","b"],"_active":{"id":"z"},"active":true}`, "children().count()", "[3]"},
		{`{"resourceType":"Patient","birthDate":"2020","birthDate":"2021"}`, "children().count()", "[2]"},
		{`{"resourceType":"Patient","birthDate":"2020","birthDate":"2021","_active":{"id":"z"}}`, "children().count()", "[3]"},
		{`{"resourceType":"Patient","active":true,"_active":{"id":"a"},"active":false,"_active":{"id":"b"}}`, "children().count()", "[3]"},
		// The id b is found once; its value, a System string equal to it, is a
		// descendant too, and hasValue() tells the element from the value.
		{`{"resourceType":"Patient","active":true,"_active":{"id":"a"},"active":false,"_active":{"id":"b"}}`, "descendants().where(hasValue() and $this = 'b').count()", "[1]"},
	}
	for _, tt := range tests {
		t.Run(tt.resource+" "+tt.expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(tt.resource), tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}
		})
	}
}

// "FHIR.string is a different type to System.String. The FHIR.string type
// specializes FHIR.Element, and has the properties id, extension, and also the
// implicit value property that is actually of type of System.String." value
// navigated to nothing, so DEQM's ra-3, value.startsWith(...) on a valueString,
// could not hold where the HL7 validator holds it. hasValue() and getValue()
// read the same property, of "a single value which is a FHIR primitive".
//
// A FHIR primitive is told from a System value by where it came from: one read
// from the resource is a FHIR primitive, with or without a model, and even
// where a model types it a System one, as R4 types Resource.id and
// Extension.url; a literal or a function's result is a System value. Where a
// model makes a difference, both answers are given.
func TestAFHIRPrimitiveHasAValueProperty(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","id":"p1","birthDate":"2000-01-01","_birthDate":{"id":"b"},` +
		`"active":true,"name":[{"family":"Abc","given":["X",null],` +
		`"_given":[null,{"extension":[{"url":"u","valueCode":"absent"}]}]}],` +
		`"extension":[{"url":"x","valueString":"MeasureReport.group.where(id=1)"}]}`)
	model := &testModel{
		typeOf: map[string]string{
			"Patient.id":          "http://hl7.org/fhirpath/System.String",
			"Patient.birthDate":   "date",
			"Patient.active":      "boolean",
			"Patient.name":        "HumanName",
			"HumanName.family":    "string",
			"HumanName.given":     "string",
			"Patient.extension":   "Extension",
			"Extension.url":       "http://hl7.org/fhirpath/System.String",
			"Extension.valueCode": "code",
		},
		choiceTypes: map[string][]string{"Extension.value": {"string", "code"}},
	}

	tests := []struct {
		expr                  string
		withModel, withoutOne string
	}{
		// The value property, as a System value.
		{"Patient.birthDate.value", "[2000-01-01]", "[2000-01-01]"},
		{"Patient.birthDate.value is System.Date", "[true]", "[true]"},
		{"Patient.birthDate.value.type().namespace", "[System]", "[System]"},
		{"Patient.birthDate is FHIR.date", "[true]", "[true]"},
		{"Patient.active.value", "[true]", "[true]"},
		{"Patient.name.family.value.startsWith('A')", "[true]", "[true]"},
		{"Patient.name.family.value is System.String", "[true]", "[true]"},
		// ra-3's shape, on a choice element.
		{"Patient.extension.value.value.startsWith('MeasureReport.group.where(id=')", "[true]", "[true]"},
		// The value is a System value, with no id or extensions of its own.
		{"Patient.birthDate.value.id", "[]", "[]"},
		{"Patient.name.given.value.extension", "[]", "[]"},
		// A position with extensions and no value has no value.
		{"Patient.name.given.value", "[X]", "[X]"},
		{"Patient.name.given.value.count()", "[1]", "[1]"},
		// A System value has no properties.
		{"'abc'.value", "[]", "[]"},
		{"Patient.birthDate.value.value", "[]", "[]"},
		{"Patient.extension.url.value", "[x]", "[x]"},

		// hasValue() and getValue(): "a single value which is a FHIR primitive,
		// and it has a primitive value".
		{"Patient.name.family.hasValue()", "[true]", "[true]"},
		{"Patient.name.family.getValue()", "[Abc]", "[Abc]"},
		{"Patient.name.given.getValue().is(System.String)", "[]", "[]"},
		{"Patient.name.given.first().getValue().is(System.String)", "[true]", "[true]"},
		{"Patient.name.given.first().getValue().is(FHIR.string)", "[false]", "[false]"},
		{"Patient.name.given.last().hasValue()", "[false]", "[false]"},
		{"Patient.name.given.hasValue()", "[false]", "[false]"},
		// A System value of a primitive type has a value too, as fhirpath.js
		// and the HL7 validator answer; getValue() keeps to FHIR primitives
		{"'abc'.hasValue()", "[true]", "[true]"},
		{"'abc'.getValue()", "[]", "[]"},
		// An element a model types a System type is still read from the
		// resource; R4 names its FHIR type in structuredefinition-fhir-type.
		{"Patient.extension.url.hasValue()", "[true]", "[true]"},
		{"Patient.id.hasValue()", "[true]", "[true]"},
		{"Patient.id.value", "[p1]", "[p1]"},
		{"Patient.id.type().namespace", "[System]", "[System]"},
		// A function's result is a System value, even one that returns a
		// value of the type its input already had.
		{"Patient.active.toBoolean().hasValue()", "[true]", "[true]"},
		{"Patient.active.toBoolean().value", "[]", "[]"},
		{"Patient.active.toBoolean().type().namespace", "[System]", "[System]"},
		{"Patient.birthDate.toDate().hasValue()", "[true]", "[true]"},
		{"Patient.birthDate.toDateTime().hasValue()", "[true]", "[true]"},
		{"Patient.birthDate.toString().hasValue()", "[true]", "[true]"},
		{"Patient.birthDate.toString().getValue()", "[]", "[]"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			compiled := fhirpath.MustCompile(tt.expr)
			for _, run := range []struct {
				how  string
				opts []fhirpath.EvalOption
				want string
			}{
				{"with a model", []fhirpath.EvalOption{fhirpath.WithModel(model)}, tt.withModel},
				{"without one", nil, tt.withoutOne},
			} {
				result, err := compiled.EvaluateWithOptions(patient, run.opts...)
				if err != nil {
					t.Fatalf("%s: %v", run.how, err)
				}
				if got := result.String(); got != run.want {
					t.Errorf("%s = %s %s, want %s", tt.expr, got, run.how, run.want)
				}
			}
		})
	}
}

// A primitive read from the resource is a FHIR primitive whether or not a model
// knows its path, so ele-1, hasValue() or (children().count() > id.count()),
// holds on it: a root read without a path, which nothing types, or an element
// the model has no entry for, was taken for a System value and failed it.
func TestAPrimitiveReadWithoutATypeIsStillAFHIRPrimitive(t *testing.T) {
	model := &testModel{typeOf: map[string]string{"Patient.name": "HumanName"}}
	ele1 := fhirpath.MustCompile("hasValue() or (children().count() > id.count())")

	for _, root := range []string{`"2000-01-01"`, `true`, `5`, `1.50`} {
		result, err := ele1.EvaluateWithOptions([]byte(root), fhirpath.WithModel(model))
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != "[true]" {
			t.Errorf("ele-1 on the root %s with a model = %s, want [true]", root, got)
		}
	}

	patient := []byte(`{"resourceType":"Patient","name":[{"given":["X"]}]}`)
	for expr, want := range map[string]string{
		"Patient.name.given.hasValue()": "[true]",
		"Patient.name.given.value":      "[X]",
		"Patient.name.given.select(" + "hasValue() or (children().count() > id.count()))": "[true]",
	} {
		result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(patient, fhirpath.WithModel(model))
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != want {
			t.Errorf("%s with a model that does not know HumanName.given = %s, want %s", expr, got, want)
		}
	}
}

// A primitive's children are its id and its extensions; its value is not one,
// as in the HL7 validator, in which the children() and descendants() counts
// below were measured (6.10.2), though FHIR says "the primitive value will be
// included in the set returned by children() or descendants()": see
// CONFORMANCE.md. A System value, a literal or a primitive's value, is only
// ever a System type.
func TestAPrimitivesValueIsNotAChild(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","birthDate":"2000-01-01",` +
		`"_birthDate":{"id":"b","extension":[{"url":"u","valueCode":"x"}]},` +
		`"name":[{"family":"F","given":["A","B"]}]}`)
	model := &testModel{typeOf: map[string]string{
		"Patient.birthDate": "date",
		"Patient.name":      "HumanName",
		"HumanName.family":  "string",
		"HumanName.given":   "string",
	}}

	tests := []struct {
		expr                  string
		withModel, withoutOne string
	}{
		{"Patient.birthDate.children().count()", "[2]", "[2]"},
		{"Patient.birthDate.children().last()", `[{"url":"u","valueCode":"x"}]`, `[{"url":"u","valueCode":"x"}]`},
		{"Patient.birthDate.descendants().count()", "[4]", "[4]"},
		{"Patient.name.family.children()", "[]", "[]"},
		{"Patient.name.descendants().count()", "[3]", "[3]"},
		{"Patient.name.descendants().ofType(string).count()", "[3]", "[3]"},
		{"Patient.name.descendants().where(hasValue()).count()", "[3]", "[3]"},
		{"Patient.birthDate.descendants().where($this is System.Date).count()", "[0]", "[0]"},
		// A filter by value finds the primitive alone.
		{"Patient.descendants().where($this = 'A').count()", "[1]", "[1]"},
		// "ofType() does not have such restrictions - both of the following
		// are valid: Patient.name.given.ofType(FHIR.string);
		// Patient.name.given.ofType(System.string)": the primitives. The HL7
		// validator answers 0, which #153 is to settle.
		{"Patient.name.descendants().ofType(System.String).count()", "[3]", "[3]"},
		// The value is still read by name.
		{"Patient.birthDate.value", "[2000-01-01]", "[2000-01-01]"},
		// A System value is only ever a System type.
		{"'abc'.is(string)", "[false]", "[false]"},
		{"'abc'.is(FHIR.string)", "[false]", "[false]"},
		{"'abc'.is(String)", "[true]", "[true]"},
		{"'abc'.is(System.string)", "[true]", "[true]"},
		{"'abc'.as(string)", "[]", "[]"},
		{"Patient.name.family.is(string)", "[true]", "[true]"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			compiled := fhirpath.MustCompile(tt.expr)
			for _, run := range []struct {
				how  string
				opts []fhirpath.EvalOption
				want string
			}{
				{"with a model", []fhirpath.EvalOption{fhirpath.WithModel(model)}, tt.withModel},
				{"without one", nil, tt.withoutOne},
			} {
				result, err := compiled.EvaluateWithOptions(patient, run.opts...)
				if err != nil {
					t.Fatalf("%s: %v", run.how, err)
				}
				if got := result.String(); got != run.want {
					t.Errorf("%s = %s %s, want %s", tt.expr, got, run.how, run.want)
				}
			}
		})
	}
}

// TestDescendantsCountEachPrimitiveOnce pins the counts CONFORMANCE.md
// records beyond the patient above, each measured in the HL7 validator
// (6.10.2).
func TestDescendantsCountEachPrimitiveOnce(t *testing.T) {
	tests := []struct {
		resource, expr, want string
	}{
		// The issue's Bundle: the entry written {} and the response's status.
		{`{"resourceType":"Bundle","type":"transaction-response","entry":[{},{"response":{"status":"200"}}]}`, "Bundle.descendants().count()", "[5]"},
		// A primitive written with only an extension: the extension, its url
		// and its value.
		{`{"resourceType":"Patient","_active":{"extension":[{"url":"u","valueBoolean":false}]}}`, "Patient.active.descendants().count()", "[3]"},
		{`{"resourceType":"Patient","_active":{}}`, "Patient.active.children().count()", "[0]"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(tt.resource), tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}
		})
	}
}

// A choice element written with no value is of the type its name tells, with
// or without a model, as in the HL7 validator 6.10.2: "_valueString" is a
// string. Without a model a field whose name tells no type, "_active", has
// none to give, and stays an Object.
func TestAValuelessChoiceIsOfTheTypeItsNameTells(t *testing.T) {
	const dar = `{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"unknown"}]}`
	tests := []struct{ resource, expr, want string }{
		{`{"resourceType":"Observation","_valueString":` + dar + `}`, "Observation.value is string", "[true]"},
		{`{"resourceType":"Observation","_valueString":` + dar + `}`, "Observation.value.ofType(string).count()", "[1]"},
		{`{"resourceType":"Observation","_valueString":` + dar + `}`, "Observation.value.ofType(Quantity).count()", "[0]"},
		{`{"resourceType":"Observation","_valueBoolean":` + dar + `}`, "Observation.value.type().name", "[boolean]"},
		{`{"resourceType":"Patient","extension":[{"url":"u","_valueDate":` + dar + `}]}`, "Patient.extension.value is date", "[true]"},
		{`{"resourceType":"Patient","_active":` + dar + `}`, "Patient.active.type().name", "[Object]"},
		// Read by its own name, with no model to say it is a choice, it is a
		// field like any other.
		{`{"resourceType":"Observation","_valueDateTime":` + dar + `}`, "Observation.value is dateTime", "[true]"},
		{`{"resourceType":"Observation","_valueDateTime":` + dar + `}`, "Observation.valueDateTime.type().name", "[Object]"},
		// A complex type names no primitive: the element of an invalid
		// "_valueQuantity" is no "quantity".
		{`{"resourceType":"Observation","_valueQuantity":` + dar + `}`, "Observation.value.type().name", "[Object]"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(tt.resource), tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}
		})
	}
}
