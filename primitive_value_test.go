package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

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
		{"'abc'.hasValue()", "[false]", "[false]"},
		{"'abc'.getValue()", "[]", "[]"},
		// An element a model types a System type is still read from the
		// resource; R4 names its FHIR type in structuredefinition-fhir-type.
		{"Patient.extension.url.hasValue()", "[true]", "[true]"},
		{"Patient.id.hasValue()", "[true]", "[true]"},
		{"Patient.id.value", "[p1]", "[p1]"},
		{"Patient.id.type().namespace", "[System]", "[System]"},
		// A function's result is a System value, even one that returns a
		// value of the type its input already had.
		{"Patient.active.toBoolean().hasValue()", "[false]", "[false]"},
		{"Patient.active.toBoolean().value", "[]", "[]"},
		{"Patient.active.toBoolean().type().namespace", "[System]", "[System]"},
		{"Patient.birthDate.toDate().hasValue()", "[false]", "[false]"},
		{"Patient.birthDate.toDateTime().hasValue()", "[false]", "[false]"},
		{"Patient.birthDate.toString().hasValue()", "[false]", "[false]"},
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
