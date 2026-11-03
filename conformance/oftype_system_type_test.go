package conformance

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"
	"unicode"

	"github.com/gofhir/fhirpath"
)

// ofType(S), where S names a System type, keeps a FHIR primitive exactly when
// the specification's conversion table maps the primitive's FHIR type to S
// (plan 017, D1). The tables below are written out from the texts; they are
// not read from the engine, whose own tables TestConversionTable checks
// against the pinned texts.
//
// R4 and R4B fhirpath.html, 2.9.1.2: "The evaluation engine will
// automatically convert the value of FHIR types representing primitives to
// FHIRPath types when they are used in expressions according to the following
// mapping:" followed by the rows below. R5 fhirpath.html, 2.1.9.1.2, is the
// same table without sid, and with FHIR.integer64 mapped to System.Long.
// canonical, url and xhtml are named by neither, so convert to no System type.
var conversionR4 = map[string][]string{
	"Boolean":  {"boolean"},
	"String":   {"string", "uri", "code", "oid", "id", "uuid", "sid", "markdown", "base64Binary"},
	"Integer":  {"integer", "unsignedInt", "positiveInt"},
	"Long":     {},
	"Decimal":  {"decimal"},
	"Date":     {},
	"DateTime": {"date", "dateTime", "instant"},
	"Time":     {"time"},
}

var conversionR5 = map[string][]string{
	"Boolean":  {"boolean"},
	"String":   {"string", "uri", "code", "oid", "id", "uuid", "markdown", "base64Binary"},
	"Integer":  {"integer", "unsignedInt", "positiveInt"},
	"Long":     {"integer64"},
	"Decimal":  {"decimal"},
	"Date":     {},
	"DateTime": {"date", "dateTime", "instant"},
	"Time":     {"time"},
}

var systemTypeOrder = []string{"Boolean", "String", "Integer", "Long", "Decimal", "Date", "DateTime", "Time"}

// conversionFor is the table of a version, R4B's the R4 one.
func conversionFor(version string) map[string][]string {
	if version == "r5" {
		return conversionR5
	}
	return conversionR4
}

// converts reports whether the version's table maps the FHIR type to S.
func converts(version, fhirType, system string) bool {
	return slices.Contains(conversionFor(version)[system], fhirType)
}

// spellings are the ways a System type is asked for: qualified, unqualified
// as FHIRPath capitalizes its types, and qualified in lower case, as R5 writes
// the example ofType(System.string). Unqualified Long is an error with a model
// and is out of scope (plan 017, D4).
func spellings(system string) []string {
	out := []string{"System." + system, "System." + strings.ToLower(system)}
	if system != "Long" {
		out = append(out, system)
	}
	return out
}

func countOf(doc, expr string, opts ...fhirpath.EvalOption) string {
	return evaluateRendered(doc, expr+".count()", opts...)
}

func versionOptions(version string) []fhirpath.EvalOption {
	for _, v := range versionModels {
		if v.name == version {
			return []fhirpath.EvalOption{fhirpath.WithModel(v.model)}
		}
	}
	return nil
}

func isPrimitiveName(name string) bool { return unicode.IsLower(rune(name[0])) }

// choiceDocument writes one extension for every primitive the version's
// Extension.value[x] allows.
func choiceDocument(t *testing.T, types []string) string {
	t.Helper()
	extensions := make([]map[string]any, 0, len(types))
	for _, typ := range types {
		sample, ok := primitiveSamples[typ]
		if !ok {
			t.Fatalf("no sample value for the primitive %s", typ)
		}
		field := "value" + strings.ToUpper(typ[:1]) + typ[1:]
		extensions = append(extensions, map[string]any{"url": "u/" + typ, field: json.RawMessage(sample)})
	}
	doc, err := json.Marshal(map[string]any{"resourceType": "Patient", "extension": extensions})
	if err != nil {
		t.Fatal(err)
	}
	return string(doc)
}

func primitiveChoices(version string) []string {
	var out []string
	for _, v := range versionModels {
		if v.name == version {
			for _, typ := range v.model.ChoiceTypes("Extension.value") {
				if isPrimitiveName(typ) {
					out = append(out, typ)
				}
			}
		}
	}
	return out
}

// A choice carries its FHIR type from its key, so every primitive the version
// allows in value[x] is answered by the table, in the three spellings. With
// no model the R4 table applies (plan 017, D2).
func TestOfTypeSystemTypeAChoiceIsAnsweredByTheTable(t *testing.T) {
	for _, version := range []string{"r4", "r4b", "r5", "no model"} {
		choices := primitiveChoices(version)
		tableVersion := version
		if version == "no model" {
			choices = primitiveChoices("r4")
			tableVersion = "r4"
		}
		if len(choices) < 15 {
			t.Fatalf("%s: %d primitives in Extension.value", version, len(choices))
		}
		doc := choiceDocument(t, choices)
		for _, typ := range choices {
			for _, system := range systemTypeOrder {
				want := "[Integer 1]"
				if !converts(tableVersion, typ, system) {
					want = "[Integer 0]"
				}
				for _, spelling := range spellings(system) {
					expr := fmt.Sprintf("Patient.extension.where(url='u/%s').value.ofType(%s)", typ, spelling)
					if got := countOf(doc, expr, versionOptions(version)...); got != want {
						t.Errorf("%s: %s = %s, want %s (%s to %s)", version, expr, got, want, typ, system)
					}
				}
			}
		}
	}
}

// plainFixtures are elements the models type with a FHIR primitive, one per
// type the Go-side models give a plain element, with the resource that holds
// it. oid, uuid, integer64 and sid have no plain element in the resources, and
// are answered as choices above.
var plainFixtures = []struct{ fhirType, doc, expr string }{
	{"boolean", `{"resourceType":"Patient","active":true}`, "Patient.active"},
	{"string", `{"resourceType":"Patient","name":[{"family":"F"}]}`, "Patient.name.family"},
	{"uri", `{"resourceType":"Patient","implicitRules":"urn:r"}`, "Patient.implicitRules"},
	{"code", `{"resourceType":"Patient","gender":"male"}`, "Patient.gender"},
	{"canonical", `{"resourceType":"Patient","meta":{"profile":["http://example.org/p"]}}`, "Patient.meta.profile"},
	{"positiveInt", `{"resourceType":"Patient","telecom":[{"rank":2}]}`, "Patient.telecom.rank"},
	{"unsignedInt", `{"resourceType":"Bundle","type":"collection","total":3}`, "Bundle.total"},
	{"integer", `{"resourceType":"Questionnaire","status":"draft","item":[{"linkId":"a","type":"string","maxLength":9}]}`, "Questionnaire.item.maxLength"},
	{"instant", `{"resourceType":"Patient","meta":{"lastUpdated":"2020-02-03T04:05:06.007Z"}}`, "Patient.meta.lastUpdated"},
	{"date", `{"resourceType":"Patient","birthDate":"1974-12-25"}`, "Patient.birthDate"},
	{"dateTime", `{"resourceType":"Patient","contact":[{"period":{"start":"2020-02-03T04:05:06Z"}}]}`, "Patient.contact.period.start"},
	{"decimal", `{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":{"value":1.5}}`, "Observation.value.ofType(Quantity).value"},
	{"markdown", `{"resourceType":"Questionnaire","status":"draft","description":"**m**"}`, "Questionnaire.description"},
	{"base64Binary", `{"resourceType":"Patient","photo":[{"data":"aGVsbG8="}]}`, "Patient.photo.data"},
	{"time", `{"resourceType":"Patient","extension":[{"url":"u","valueTiming":{"repeat":{"timeOfDay":["04:05:06"]}}}]}`, "Patient.extension.value.ofType(Timing).repeat.timeOfDay"},
	{"url", `{"resourceType":"CapabilityStatement","status":"draft","date":"2020-01-01","kind":"instance","fhirVersion":"4.0.1","format":["json"],"implementation":{"description":"d","url":"http://example.org/u"}}`, "CapabilityStatement.implementation.url"},
	{"xhtml", `{"resourceType":"Patient","text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"}}`, "Patient.text.`div`"},
}

// A plain element read with a model has the FHIR type its definition gives it,
// and the table answers it. The fixture's type is checked first, by type(),
// which does not go through ofType.
func TestOfTypeSystemTypePlainElementsAreAnsweredByTheTable(t *testing.T) {
	for _, version := range []string{"r4", "r4b", "r5"} {
		opts := versionOptions(version)
		for _, fx := range plainFixtures {
			if got := evaluateRendered(fx.doc, fx.expr+".type().name", opts...); got != "[String "+fx.fhirType+"]" {
				t.Errorf("%s: %s.type().name = %s, want the fixture's type %s", version, fx.expr, got, fx.fhirType)
				continue
			}
			for _, system := range systemTypeOrder {
				want := "[Integer 1]"
				if !converts(version, fx.fhirType, system) {
					want = "[Integer 0]"
				}
				for _, spelling := range spellings(system) {
					expr := fx.expr + ".ofType(" + spelling + ")"
					if got := countOf(fx.doc, expr, opts...); got != want {
						t.Errorf("%s: %s = %s, want %s (%s to %s)", version, expr, got, want, fx.fhirType, system)
					}
				}
			}
		}
	}
}

// The examples of the text, verbatim. R5 fhirpath.html 2.1.9.1.2: "Note that
// ofType() does not have such restrictions - both of the following are valid:
// Patient.name.given.ofType(FHIR.string); Patient.name.given.ofType(System.string);".
// R4 and R4B say it of as(), so ofType follows the same rule here (plan 017).
func TestOfTypeSystemTypeTheSpecificationExamples(t *testing.T) {
	const doc = `{"resourceType":"Patient","name":[{"given":["Ewout","E"]}]}`
	for _, version := range []string{"r4", "r4b", "r5", "no model"} {
		opts := versionOptions(version)
		for _, expr := range []string{
			"Patient.name.given.ofType(FHIR.string)",
			"Patient.name.given.ofType(System.string)",
		} {
			if got := countOf(doc, expr, opts...); got != "[Integer 2]" {
				t.Errorf("%s: %s = %s, want both given names", version, expr, got)
			}
		}
	}
}

// ofType over a collection that mixes types keeps the items whose FHIR type
// maps to S, whatever the collection holds beside them.
func TestOfTypeSystemTypeAMixedCollection(t *testing.T) {
	const doc = `{"resourceType":"Patient","active":true,"gender":"male","birthDate":"1970-01-01",` +
		`"name":[{"family":"F","given":["G"]}],"telecom":[{"rank":2}],` +
		`"meta":{"lastUpdated":"2020-01-01T00:00:00Z","profile":["http://example.org/p"]},` +
		`"implicitRules":"urn:r","multipleBirthInteger":3}`
	want := map[string][]string{
		"Boolean":  {"boolean"},
		"String":   {"code", "string", "string", "uri"},
		"Integer":  {"integer", "positiveInt"},
		"Long":     {},
		"Decimal":  {},
		"Date":     {},
		"DateTime": {"date", "instant"},
		"Time":     {},
	}
	for _, version := range []string{"r4", "r4b", "r5"} {
		for _, system := range systemTypeOrder {
			result, err := fhirpath.MustCompile("Patient.descendants().ofType(System."+system+")").
				EvaluateWithOptions([]byte(doc), versionOptions(version)...)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, 0, len(result))
			for _, item := range result {
				got = append(got, item.Type())
			}
			sort.Strings(got)
			if !slices.Equal(got, want[system]) {
				t.Errorf("%s: descendants().ofType(System.%s) holds %v, want %v", version, system, got, want[system])
			}
		}
	}
}

// The item kept is the FHIR primitive itself: it keeps its FHIR type and its
// extensions, and is not the System value it converts to.
func TestOfTypeSystemTypeTheItemKeptKeepsItsTypeAndExtensions(t *testing.T) {
	const doc = `{"resourceType":"Patient","birthDate":"1970-01-01","gender":"male",` +
		`"_birthDate":{"extension":[{"url":"http://example.org/e","valueString":"x"}]},` +
		`"_gender":{"extension":[{"url":"http://example.org/g","valueString":"y"}]}}`
	for _, version := range []string{"r4", "r4b", "r5"} {
		opts := versionOptions(version)
		for _, tt := range []struct{ expr, want string }{
			{"Patient.birthDate.ofType(System.DateTime).type().name", "[String date]"},
			{"Patient.birthDate.ofType(System.DateTime).extension.value", "[string x]"},
			{"Patient.gender.ofType(System.String).type().name", "[String code]"},
			{"Patient.gender.ofType(System.string).extension.value", "[string y]"},
			{"Patient.gender.ofType(String).extension.count()", "[Integer 1]"},
		} {
			if got := evaluateRendered(doc, tt.expr, opts...); got != tt.want {
				t.Errorf("%s: %s = %s, want %s", version, tt.expr, got, tt.want)
			}
		}
	}
}

// R5 fhirpath.html 2.1.9.1.2 maps FHIR.integer64 to System.Long (plan 017, D4):
// the item kept is the FHIR primitive and needs no Long value, so
// ofType(System.Long) keeps it from R5 on, and ofType(System.Integer) drops it.
// R4 and R4B name no integer64.
func TestOfTypeSystemTypeInteger64(t *testing.T) {
	const doc = `{"resourceType":"Patient","extension":[{"url":"u","valueInteger64":"9007199254740993"}]}`
	for _, tt := range []struct{ expr, want string }{
		{"Patient.extension.value.ofType(System.Long).count()", "[Integer 1]"},
		{"Patient.extension.value.ofType(System.long).count()", "[Integer 1]"},
		{"Patient.extension.value.ofType(System.Integer).count()", "[Integer 0]"},
		{"Patient.extension.value.ofType(integer64).count()", "[Integer 1]"},
	} {
		if got := evaluateRendered(doc, tt.expr, versionOptions("r5")...); got != tt.want {
			t.Errorf("r5: %s = %s, want %s", tt.expr, got, tt.want)
		}
	}
	// Before R5 and with no model, the R4 table has no integer64 row. Under an
	// R4 or R4B model valueInteger64 is no choice of Extension.value[x] at all,
	// so the key is not read as a value and nothing is kept either.
	for _, version := range []string{"r4", "r4b", "no model"} {
		for _, expr := range []string{
			"Patient.extension.value.ofType(System.Long).count()",
			"Patient.extension.value.ofType(System.Integer).count()",
		} {
			if got := evaluateRendered(doc, expr, versionOptions(version)...); got != "[Integer 0]" {
				t.Errorf("%s: %s = %s, want 0: the R4 table has no integer64 row", version, expr, got)
			}
		}
	}
}

// Plan 017, D4b: an element the model types as a System type (Element.id; R4's
// Resource.id) has no FHIR type to look up, and keeps the System-name match it
// had, so name.id.ofType(System.String) keeps the value. R4B and R5 type
// Resource.id as id, and the table maps id to System.String: the same answer.
func TestOfTypeSystemTypeItemsTypedByASystemName(t *testing.T) {
	const doc = `{"resourceType":"Patient","id":"p1","name":[{"id":"n1","family":"F"}]}`
	for _, version := range []string{"r4", "r4b", "r5"} {
		opts := versionOptions(version)
		for _, tt := range []struct{ expr, want string }{
			{"Patient.name.id.ofType(System.String).toString()", "[String n1]"},
			{"Patient.name.id.ofType(System.string).toString()", "[String n1]"},
			{"Patient.name.id.ofType(String).toString()", "[String n1]"},
			{"Patient.name.id.ofType(System.Integer)", "[]"},
			{"Patient.name.id.ofType(System.DateTime)", "[]"},
			{"Patient.id.ofType(System.String).toString()", "[String p1]"},
			{"Patient.id.ofType(System.Integer)", "[]"},
		} {
			if got := evaluateRendered(doc, tt.expr, opts...); got != tt.want {
				t.Errorf("%s: %s = %s, want %s", version, tt.expr, got, tt.want)
			}
		}
	}
}

// Plan 017, D4c: a primitive written with only its element has no value to
// convert, so ofType(System.X) drops it; ofType(date) and ofType(System.Any)
// keep it, as before.
func TestOfTypeSystemTypeAPrimitiveWithNoValue(t *testing.T) {
	const doc = `{"resourceType":"Patient","_birthDate":{"extension":[{"url":"http://hl7.org/fhir/StructureDefinition/data-absent-reason","valueCode":"unknown"}]}}`
	for _, version := range []string{"r4", "r4b", "r5"} {
		opts := versionOptions(version)
		for _, tt := range []struct{ expr, want string }{
			{"Patient.birthDate.ofType(System.DateTime).count()", "[Integer 0]"},
			{"Patient.birthDate.ofType(System.datetime).count()", "[Integer 0]"},
			{"Patient.birthDate.ofType(DateTime).count()", "[Integer 0]"},
			{"Patient.birthDate.ofType(System.Date).count()", "[Integer 0]"},
			{"Patient.birthDate.ofType(date).count()", "[Integer 1]"},
			{"Patient.birthDate.ofType(System.Any).count()", "[Integer 1]"},
		} {
			if got := evaluateRendered(doc, tt.expr, opts...); got != tt.want {
				t.Errorf("%s: %s = %s, want %s", version, tt.expr, got, tt.want)
			}
		}
	}
	for _, expr := range []string{
		"Patient.birthDate.ofType(System.DateTime).count()",
		"Patient.birthDate.ofType(System.Date).count()",
	} {
		if got := evaluateRendered(doc, expr); got != "[Integer 0]" {
			t.Errorf("no model: %s = %s, want 0", expr, got)
		}
	}
}

// Plan 017, D4d: the namespace is System exactly, and the name ignores case.
// system.String and SYSTEM.String name no System type: with a model they are
// not a type of it either, so asking is an error, as for any unknown namespace
// and name; with none, nothing is of them.
func TestOfTypeSystemTypeSpelling(t *testing.T) {
	const doc = `{"resourceType":"Patient","gender":"male"}`
	const spellingDoc = `{"resourceType":"Patient","id":"p1","gender":"male"}`
	for _, version := range []string{"r4", "r4b", "r5", "no model"} {
		opts := versionOptions(version)
		for _, name := range []string{"System.String", "System.string", "System.STRING", "System.sTrInG"} {
			if got := countOf(doc, "Patient.gender.ofType("+name+")", opts...); got != "[Integer 1]" {
				t.Errorf("%s: ofType(%s) = %s, want 1: the name ignores case", version, name, got)
			}
		}
		for _, name := range []string{"system.String", "SYSTEM.String"} {
			got := countOf(doc, "Patient.gender.ofType("+name+")", opts...)
			if version == "no model" {
				if got != "[Integer 0]" {
					t.Errorf("%s: ofType(%s) = %s, want 0: it names no System type", version, name, got)
				}
			} else if !strings.HasPrefix(got, "error:") {
				t.Errorf("%s: ofType(%s) = %s, want an error: it names no type of the model", version, name, got)
			}
		}
		// The misspelled namespace names no System type in is(), as(), the
		// `as` operator and ofType() alike: an error with a model, and with
		// none a primitive read from the resource is not matched (on main it
		// was: is() true, as() and ofType() the value).
		for _, name := range []string{"system.String", "SYSTEM.String"} {
			for _, element := range []string{"gender", "id"} {
				for _, form := range []struct{ expr, noModel string }{
					{"Patient." + element + ".is(" + name + ")", "[Boolean false]"},
					{"Patient." + element + ".as(" + name + ")", "[]"},
					{"Patient." + element + " as " + name, "[]"},
					{"Patient." + element + ".ofType(" + name + ")", "[]"},
				} {
					got := evaluateRendered(spellingDoc, form.expr, opts...)
					if version == "no model" {
						if got != form.noModel {
							t.Errorf("%s: %s = %s, want %s: it names no System type", version, form.expr, got, form.noModel)
						}
					} else if !strings.HasPrefix(got, "error:") {
						t.Errorf("%s: %s = %s, want an error: it names no type of the model", version, form.expr, got)
					}
				}
			}
		}
		// Any misspelled namespace names no System type, and the answer does
		// not depend on the value: a Quantity, a choice and a date are all
		// unmatched (on main (1 'mg').is(SYSTEM.Quantity) and the choice's
		// were true, and the no model birthDate.is(SYSTEM.Date) too).
		const obsDoc = `{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":{"value":1.5,"unit":"mg"}}`
		const birthDoc = `{"resourceType":"Patient","birthDate":"1974-12-25"}`
		for _, tt := range []struct{ doc, expr, want string }{
			{spellingDoc, "(1 'mg').is(SYSTEM.Quantity)", "[Boolean false]"},
			{obsDoc, "Observation.value.is(SYSTEM.Quantity)", "[Boolean false]"},
			{obsDoc, "Observation.value.ofType(SYSTEM.Quantity)", "[]"},
			{birthDoc, "Patient.birthDate.is(SYSTEM.Date)", "[Boolean false]"},
		} {
			got := evaluateRendered(tt.doc, tt.expr, opts...)
			if strings.Contains(tt.expr, "SYSTEM.Date") && version != "no model" {
				if !strings.HasPrefix(got, "error:") {
					t.Errorf("%s: %s = %s, want an error: the model has no Date type", version, tt.expr, got)
				}
			} else if got != tt.want {
				t.Errorf("%s: %s = %s, want %s: it names no System type", version, tt.expr, got, tt.want)
			}
		}
	}
}

// Plan 017, D6: with no model a plain element read from the resource is a FHIR
// primitive of the System type its JSON gives it, and is answered by that
// name; a choice carries its FHIR type from its key and is answered by the
// table. The two differ because only the choice tells its FHIR type
// (CONFORMANCE.md, "Without a model, a FHIR type is a guess").
func TestOfTypeSystemTypeWithNoModel(t *testing.T) {
	const doc = `{"resourceType":"Patient","birthDate":"1974-12-25","meta":{"profile":["http://example.org/p"]},` +
		`"gender":"male","active":true,"telecom":[{"rank":2}],` +
		`"extension":[{"url":"d","valueDate":"2020-02-03"},{"url":"w","valueUrl":"http://example.org/u"},{"url":"c","valueCode":"c"}]}`
	for _, tt := range []struct{ expr, want string }{
		{"Patient.birthDate.ofType(System.Date).count()", "[Integer 1]"},
		{"Patient.birthDate.ofType(System.date).count()", "[Integer 1]"},
		{"Patient.birthDate.ofType(Date).count()", "[Integer 1]"},
		{"Patient.birthDate.ofType(System.DateTime).count()", "[Integer 0]"},
		{"Patient.meta.profile.ofType(System.String).count()", "[Integer 1]"},
		{"Patient.gender.ofType(System.String).count()", "[Integer 1]"},
		{"Patient.active.ofType(System.Boolean).count()", "[Integer 1]"},
		{"Patient.telecom.rank.ofType(System.Integer).count()", "[Integer 1]"},
		{"Patient.extension.where(url='d').value.ofType(System.DateTime).count()", "[Integer 1]"},
		{"Patient.extension.where(url='d').value.ofType(System.Date).count()", "[Integer 0]"},
		{"Patient.extension.where(url='w').value.ofType(System.String).count()", "[Integer 0]"},
		{"Patient.extension.where(url='c').value.ofType(System.String).count()", "[Integer 1]"},
	} {
		if got := evaluateRendered(doc, tt.expr); got != tt.want {
			t.Errorf("no model: %s = %s, want %s", tt.expr, got, tt.want)
		}
	}
}

// Plan 017, D5: what the table does not touch keeps its answers. is() and
// as() are not routed through it ("The primary exception is the is()
// operation, where the difference is explicit": Patient.name.given.is(System.string).not()),
// ofType by a FHIR name still requires the declared type, a System value is
// only a System type, and a Quantity is still a Quantity.
func TestOfTypeSystemTypeUnchangedAnswers(t *testing.T) {
	const doc = `{"resourceType":"Observation","status":"final","code":{"text":"c"},` +
		`"valueQuantity":{"value":1.5,"unit":"mg"},"interpretation":[{"text":"t"}],` +
		`"issued":"2020-02-03T04:05:06.007Z"}`
	const patient = `{"resourceType":"Patient","gender":"male","name":[{"given":["Ewout"]}]}`
	// A name written in the System namespace that is no System type names
	// nothing a FHIR primitive is: is() says false, and as() and ofType() are
	// empty, in every version and with no model (plan 017, D5; code review R1
	// and R2).
	const withID = `{"resourceType":"Patient","id":"p1","gender":"male"}`
	for _, version := range []string{"r4", "r4b", "r5", "no model"} {
		for _, tt := range []struct{ expr, want string }{
			{"Patient.gender.as(System.code)", "[]"},
			{"Patient.gender as System.code", "[]"},
			{"Patient.gender.is(System.code)", "[Boolean false]"},
			{"Patient.id.as(System.id)", "[]"},
			{"Patient.gender.ofType(System.code)", "[]"},
		} {
			if got := evaluateRendered(withID, tt.expr, versionOptions(version)...); got != tt.want {
				t.Errorf("%s: %s = %s, want %s", version, tt.expr, got, tt.want)
			}
		}
	}
	for _, version := range []string{"r4", "r4b", "r5"} {
		opts := versionOptions(version)
		for _, tt := range []struct{ doc, expr, want string }{
			{patient, "Patient.gender is code", "[Boolean true]"},
			{patient, "Patient.gender is string", "[Boolean true]"},
			{patient, "Patient.name.given.is(FHIR.string)", "[Boolean true]"},
			{patient, "Patient.name.given.is(System.string).not()", "[Boolean true]"},
			{patient, "Patient.name.given.getValue().is(System.string)", "[Boolean true]"},
			{patient, "Patient.gender.as(System.String).count()", "[Integer 0]"},
			{patient, "Patient.gender.as(code).count()", "[Integer 1]"},
			{patient, "Patient.gender.ofType(code).count()", "[Integer 1]"},
			{patient, "Patient.gender.ofType(FHIR.code).count()", "[Integer 1]"},
			{patient, "Patient.gender.ofType(string).count()", "[Integer 0]"},
			{patient, "Patient.gender.ofType(FHIR.string).count()", "[Integer 0]"},
			{patient, "Patient.name.given.ofType(string).count()", "[Integer 1]"},
			{patient, "Patient.gender.ofType(System.Any).count()", "[Integer 1]"},
			{patient, "'a'.ofType(System.String).count()", "[Integer 1]"},
			{patient, "'a'.ofType(String).count()", "[Integer 1]"},
			{patient, "'a'.ofType(string).count()", "[Integer 0]"},
			{patient, "1.ofType(System.Integer).count()", "[Integer 1]"},
			{patient, "1.ofType(System.Long).count()", "[Integer 0]"},
			{patient, "Patient.gender.getValue().ofType(System.String).count()", "[Integer 1]"},
			{patient, "Patient.gender.getValue().ofType(code).count()", "[Integer 0]"},
			{doc, "Observation.value.ofType(Quantity).count()", "[Integer 1]"},
			{doc, "Observation.value.ofType(System.Quantity).count()", "[Integer 1]"},
			{doc, "Observation.value.ofType(System.String).count()", "[Integer 0]"},
			{doc, "1 'mg'.ofType(System.Quantity).count()", "[Integer 1]"},
			{doc, "Observation.interpretation.ofType(CodeableConcept).count()", "[Integer 1]"},
		} {
			if got := evaluateRendered(tt.doc, tt.expr, opts...); got != tt.want {
				t.Errorf("%s: %s = %s, want %s", version, tt.expr, got, tt.want)
			}
		}
	}
}

// Metamorphic: for a singleton FHIR primitive X, X.ofType(S) is either X
// itself or empty, before R5 and from R5 on, and the order of the keys of the
// resource changes nothing.
func TestOfTypeSystemTypeKeepsTheItemOrNothing(t *testing.T) {
	for _, version := range []string{"r4", "r5"} {
		opts := versionOptions(version)
		for _, fx := range plainFixtures {
			whole := evaluateRendered(fx.doc, fx.expr, opts...)
			reordered := reverseKeys(t, fx.doc)
			for _, system := range systemTypeOrder {
				expr := fx.expr + ".ofType(System." + system + ")"
				got := evaluateRendered(fx.doc, expr, opts...)
				if got != "[]" && got != whole {
					t.Errorf("%s: %s = %s, want [] or %s", version, expr, got, whole)
				}
				if count := evaluateRendered(fx.doc, expr+".count()", opts...); (got == whole) != (count == "[Integer 1]") {
					t.Errorf("%s: %s kept %s but counts %s", version, expr, got, count)
				}
				if again := evaluateRendered(reordered, expr, opts...); again != got {
					t.Errorf("%s: %s = %s, but %s with the keys reversed", version, expr, got, again)
				}
			}
		}
	}
}

// reverseKeys rewrites a JSON document with the keys of every object in
// reverse alphabetical order.
func reverseKeys(t *testing.T, doc string) string {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(doc), &v); err != nil {
		t.Fatal(err)
	}
	var write func(v any) string
	write = func(v any) string {
		switch x := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(x))
			for k := range x {
				keys = append(keys, k)
			}
			sort.Sort(sort.Reverse(sort.StringSlice(keys)))
			parts := make([]string, len(keys))
			for i, k := range keys {
				kb, _ := json.Marshal(k)
				parts[i] = string(kb) + ":" + write(x[k])
			}
			return "{" + strings.Join(parts, ",") + "}"
		case []any:
			parts := make([]string, len(x))
			for i, e := range x {
				parts[i] = write(e)
			}
			return "[" + strings.Join(parts, ",") + "]"
		default:
			b, _ := json.Marshal(x)
			return string(b)
		}
	}
	return write(v)
}
