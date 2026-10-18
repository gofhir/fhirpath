package conformance

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r4b/v2"
	"github.com/gofhir/models/r5/v2"
)

// getExtensionValue(url) is this engine's own, not FHIR's: the value of the
// extensions with that url, as extension(url).value reads it (#218). Up to
// 1.16.5 it looked for 20 value[x] names read untyped, so the other 30 to 34
// types each version's Extension allows answered empty.

var versionModels = []struct {
	name  string
	model fhirpath.Model
}{{"r4", r4.FHIRPathModel()}, {"r4b", r4b.FHIRPathModel()}, {"r5", r5.FHIRPathModel()}}

// primitiveSamples holds a valid value of each FHIR primitive type, as JSON,
// for the test below; test data, not something the engine reads. A version
// that allows a primitive missing here fails the test rather than skipping it.
var primitiveSamples = map[string]string{
	"base64Binary": `"aGVsbG8="`,
	"boolean":      `true`,
	"canonical":    `"http://example.org/StructureDefinition/x"`,
	"code":         `"c"`,
	"date":         `"2020-02-03"`,
	"dateTime":     `"2020-02-03T04:05:06Z"`,
	"decimal":      `1.50`,
	"id":           `"an-id"`,
	"instant":      `"2020-02-03T04:05:06.007Z"`,
	"integer":      `-7`,
	"integer64":    `"9007199254740993"`,
	"markdown":     `"**m**"`,
	"oid":          `"urn:oid:1.2.3"`,
	"positiveInt":  `7`,
	"string":       `"s"`,
	"time":         `"04:05:06"`,
	"unsignedInt":  `0`,
	"uri":          `"urn:x"`,
	"url":          `"http://example.org/u"`,
	"uuid":         `"urn:uuid:8e7a1e4c-6f3b-4b2c-9d8e-1a2b3c4d5e6f"`,
}

// TestGetExtensionValueFindsEveryTypeTheVersionAllows writes one extension for
// each type the version's Extension.value[x] allows, as the model lists them,
// and expects each value back, of that type. Every element has an id, so a
// complex value is written {"id": ...}; a primitive, whose type FHIR names in
// lower case, with a value of its type and the element beside it,
// "_valueX": {"id": ...}, and both are checked.
func TestGetExtensionValueFindsEveryTypeTheVersionAllows(t *testing.T) {
	for _, v := range versionModels {
		choices := v.model.ChoiceTypes("Extension.value")
		if len(choices) < 50 {
			t.Fatalf("%s: Extension.value lists %d types", v.name, len(choices))
		}
		extensions := make([]map[string]any, 0, len(choices))
		for i, typ := range choices {
			field := "value" + strings.ToUpper(typ[:1]) + typ[1:]
			id := map[string]any{"id": fmt.Sprintf("v%d", i)}
			ext := map[string]any{"url": "u/" + typ}
			if unicode.IsLower(rune(typ[0])) {
				sample, ok := primitiveSamples[typ]
				if !ok {
					t.Fatalf("%s: no sample value for the primitive %s", v.name, typ)
				}
				ext[field] = json.RawMessage(sample)
				ext["_"+field] = id
			} else {
				ext[field] = id
			}
			extensions = append(extensions, ext)
		}
		resource, err := json.Marshal(map[string]any{"resourceType": "Patient", "extension": extensions})
		if err != nil {
			t.Fatal(err)
		}

		for i, typ := range choices {
			expr := "Patient.getExtensionValue('u/" + typ + "')"
			result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(resource, fhirpath.WithModel(v.model))
			if err != nil {
				t.Errorf("%s: %s: %v", v.name, expr, err)
				continue
			}
			if len(result) != 1 || result[0].Type() != typ {
				t.Errorf("%s: %s = %s, want one %s", v.name, expr, typedItems(result), typ)
				continue
			}
			id, err := fhirpath.MustCompile(expr+".id").EvaluateWithOptions(resource, fhirpath.WithModel(v.model))
			if err != nil || id.String() != fmt.Sprintf("[v%d]", i) {
				t.Errorf("%s: %s.id = %v %v, want [v%d]", v.name, expr, id, err, i)
			}
			// the value as written: a decimal keeps its text, 1.50 not 1.5
			if sample, ok := primitiveSamples[typ]; ok {
				if got := result[0].String(); got != strings.Trim(sample, `"`) {
					t.Errorf("%s: %s = %q, want the value written, %s", v.name, expr, got, sample)
				}
			}
		}
	}
}

var extensionValues = []byte(`{"resourceType":"Patient","id":"p",
	"extension":[
		{"url":"s","valueString":"a"},
		{"url":"pi","valuePositiveInt":7},
		{"url":"md","valueMarkdown":"**x**"},
		{"url":"cr","valueCodeableReference":{"concept":{"text":"x"}}},
		{"url":"i64","valueInteger64":"5"},
		{"url":"el","_valueString":{"id":"i"}},
		{"url":"arr","valueString":["a","b"]},
		{"url":"two","valueString":"a","valueBoolean":true},
		{"url":"empty","valueCoding":{}},
		{"url":"nested","extension":[{"url":"in","valueCode":"c"}]}
	],
	"modifierExtension":[{"url":"mod","valueString":"m"}],
	"birthDate":"1970-01-01",
	"_birthDate":{"extension":[{"url":"b","valueDateTime":"1970-01-01T10:00:00Z"}]}}`)

func TestGetExtensionValueReadsTheValueAsTheModelTypesIt(t *testing.T) {
	all := []string{"r4", "r4b", "r5"}
	for _, tt := range []struct {
		versions []string
		expr     string
		want     string
	}{
		{all, "Patient.getExtensionValue('s')", "[string a]"},
		{all, "Patient.getExtensionValue('s') is string", "[Boolean true]"},
		{all, "Patient.getExtensionValue('pi')", "[positiveInt 7]"},
		{all, "Patient.getExtensionValue('md')", "[markdown **x**]"},
		// CodeableReference is a value type from R4B on, integer64 from R5
		{[]string{"r4b", "r5"}, "Patient.getExtensionValue('cr').concept.text", "[string x]"},
		{[]string{"r4"}, "Patient.getExtensionValue('cr')", "[]"},
		{[]string{"r5"}, "Patient.getExtensionValue('i64')", "[integer64 5]"},
		{[]string{"r4", "r4b"}, "Patient.getExtensionValue('i64')", "[]"},
		// a primitive with only an element is that element, which exists; its
		// id is a System.String, as the core definitions type Element.id
		{all, "Patient.getExtensionValue('el').id", "[String i]"},
		{all, "Patient.getExtensionValue('el').exists()", "[Boolean true]"},
		// what invalid FHIR holds is read as navigation reads it: both items
		// of an array, and of two value[x] fields the one the model lists first
		{all, "Patient.getExtensionValue('arr')", "[string a, string b]"},
		{all, "Patient.getExtensionValue('two')", "[boolean true]"},
		// an empty value is an item that does not exist, of its type
		{all, "Patient.getExtensionValue('empty').count()", "[Integer 1]"},
		{all, "Patient.getExtensionValue('empty').exists()", "[Boolean false]"},
		{all, "Patient.getExtensionValue('empty') is Coding", "[Boolean true]"},
		// an extension of extensions has no value of its own
		{all, "Patient.getExtensionValue('nested')", "[]"},
		{all, "Patient.extension('nested').getExtensionValue('in')", "[code c]"},
		// a primitive's extensions are those of the element beside it
		{all, "Patient.birthDate.getExtensionValue('b')", "[dateTime 1970-01-01T10:00:00Z]"},
		// only items named extension are read
		{all, "Patient.getExtensionValue('mod')", "[]"},
	} {
		for _, v := range versionModels {
			if !slices.Contains(tt.versions, v.name) {
				continue
			}
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions(extensionValues, fhirpath.WithModel(v.model))
			got := typedItems(result)
			if err != nil {
				got = "error: " + err.Error()
			}
			if got != tt.want {
				t.Errorf("%s: %s = %s, want %s", v.name, tt.expr, got, tt.want)
			}
		}
	}
}

// Without a model value[x] is found by navigation's own list of type suffixes,
// which lacks the types R4B and R5 added: R5's valueCodeableReference is not
// found. A loss of navigation (#219), not of this function; this changes when
// navigation is fixed.
func TestGetExtensionValueWithoutAModelMissesWhatNavigationMisses(t *testing.T) {
	for _, tt := range []struct{ expr, want string }{
		{"Patient.getExtensionValue('s')", "[string a]"},
		{"Patient.getExtensionValue('pi')", "[positiveInt 7]"},
		{"Patient.getExtensionValue('cr')", "[]"},
	} {
		result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions(extensionValues)
		got := typedItems(result)
		if err != nil {
			got = "error: " + err.Error()
		}
		if got != tt.want {
			t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
		}
	}
}

func TestGetExtensionValueTakesItsURLAsExtensionDoes(t *testing.T) {
	for _, tt := range []struct{ expr, want string }{
		{"Patient.getExtensionValue('')", "[]"},
		{"Patient.getExtensionValue({})", "[]"},
		{"{}.getExtensionValue('s')", "[]"},
		{"Patient.getExtensionValue('s' | 'pi')", "error"},
		{"Patient.getExtensionValue(1)", "error"},
	} {
		for _, v := range versionModels {
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions(extensionValues, fhirpath.WithModel(v.model))
			got := typedItems(result)
			if err != nil {
				got = "error"
			}
			if got != tt.want {
				t.Errorf("%s: %s = %s, want %s", v.name, tt.expr, got, tt.want)
			}
		}
	}
}

func typedItems(result fhirpath.Collection) string {
	items := make([]string, len(result))
	for i, v := range result {
		items[i] = v.Type() + " " + v.String()
	}
	return "[" + strings.Join(items, ", ") + "]"
}
