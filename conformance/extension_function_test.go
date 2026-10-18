package conformance

import (
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r4b/v2"
	"github.com/gofhir/models/r5/v2"
)

// FHIR's FHIRPath page, R4 §2.9.1.5 and R5 §2.1.9.1.5, defines extension(url)
// by another expression: "Will filter the input collection for items named
// "extension" with the given url. This is a syntactical shortcut for
// .extension.where(url = string), but is simpler to write. Will return an
// empty collection if the input collection is empty or the url is empty." (#214)
var extensionPatient = []byte(`{"resourceType":"Patient","id":"p",
	"extension":[
		{"url":"http://e/s","valueString":"Lorena"},
		{"url":"http://e/cc","valueCodeableConcept":{"text":"t"}},
		{"url":"http://e/q","valueQuantity":{"value":3,"unit":"mg"}},
		{"url":"http://e/cr","valueCodeableReference":{"concept":{"text":"x"}}},
		{"url":"http://e/i64","valueInteger64":"5"},
		{"url":"http://e/pi","valuePositiveInt":7},
		{"url":"http://e/n","extension":[{"url":"http://e/n/a","valueCode":"a"},{"url":"http://e/n/b","valueBoolean":true}]},
		{"url":"http://e/nn","valueString":"a url the one before is a prefix of"},
		{"url":"","valueString":"no url"}
	],
	"name":[{"family":"F","extension":[{"url":"http://e/h","valueString":"h"}]}],
	"birthDate":"1970-01-01",
	"_birthDate":{"extension":[{"url":"http://e/b","valueDateTime":"1970-01-01T10:00:00Z"}]},
	"contact":[{"extension":[{"url":"http://e/c","valueReference":{"reference":"Patient/q"}}],"gender":"female"}]}`)

var extensionModels = []struct {
	name  string
	model fhirpath.Model
}{{"r4", r4.FHIRPathModel()}, {"r4b", r4b.FHIRPathModel()}, {"r5", r5.FHIRPathModel()}, {"none", nil}}

// typed renders a collection with each item's type, which is what the two
// forms must agree on as well as the values.
func typed(t *testing.T, model fhirpath.Model, expr string) string {
	t.Helper()
	var opts []fhirpath.EvalOption
	if model != nil {
		opts = append(opts, fhirpath.WithModel(model))
	}
	result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(extensionPatient, opts...)
	if err != nil {
		return "error: " + err.Error()
	}
	items := make([]string, len(result))
	for i, v := range result {
		items[i] = v.Type() + " " + v.String()
	}
	return "[" + strings.Join(items, ", ") + "]"
}

func TestExtensionAnswersAsTheWhereItIsAShortcutFor(t *testing.T) {
	inputs := []struct{ path, url string }{
		{"Patient", "http://e/s"},
		{"Patient", "http://e/cc"},
		{"Patient", "http://e/q"},
		{"Patient", "http://e/cr"},
		{"Patient", "http://e/i64"},
		{"Patient", "http://e/pi"},
		{"Patient", "http://e/n"},
		{"Patient.name", "http://e/h"},                      // a complex element
		{"Patient.birthDate", "http://e/b"},                 // a primitive's element
		{"Patient.contact", "http://e/c"},                   // a backbone element
		{"Patient.extension('http://e/n')", "http://e/n/b"}, // nested
		{"Patient", "http://e/none"},
	}
	projections := []string{
		"", ".url", ".value", ".children()", ".descendants()",
		".ofType(Extension)", ".all($this is Extension)",
	}
	for _, m := range extensionModels {
		for _, in := range inputs {
			for _, proj := range projections {
				shortcut := in.path + ".extension('" + in.url + "')" + proj
				spelled := in.path + ".extension.where(url = '" + in.url + "')" + proj
				got, want := typed(t, m.model, shortcut), typed(t, m.model, spelled)
				if got != want {
					t.Errorf("%s: %s = %s, want %s as %s answers", m.name, shortcut, got, want, spelled)
				}
			}
		}
	}
}

func TestExtensionItemsAreTypedByTheModel(t *testing.T) {
	for _, tt := range []struct {
		versions []string
		expr     string
		want     string
	}{
		// Every item named extension is an Extension, and its url a uri
		{[]string{"r4", "r4b", "r5"}, "Patient.extension('http://e/s').all($this is Extension)", "[Boolean true]"},
		{[]string{"r4", "r4b", "r5"}, "Patient.extension('http://e/s').url", "[uri http://e/s]"},
		{[]string{"r4", "r4b", "r5"}, "Patient.birthDate.extension('http://e/b').ofType(Extension).url", "[uri http://e/b]"},
		{[]string{"r4", "r4b", "r5"}, "Patient.contact.extension('http://e/c').value.reference", "[string Patient/q]"},
		// value[x] takes the types the version's Extension allows: 50 in R4,
		// 51 in R4B (CodeableReference added), 54 in R5 (integer64 among them)
		{[]string{"r4b", "r5"}, "Patient.extension('http://e/cr').value.concept.text", "[string x]"},
		{[]string{"r4"}, "Patient.extension('http://e/cr').value", "[]"},
		{[]string{"r5"}, "Patient.extension('http://e/i64').value", "[integer64 5]"},
		{[]string{"r4", "r4b"}, "Patient.extension('http://e/i64').value", "[]"},
		{[]string{"r4", "r4b", "r5"}, "Patient.extension('http://e/pi').value", "[positiveInt 7]"},
		// the url is matched whole, not as a prefix
		{[]string{"r4", "r4b", "r5"}, "Patient.extension('http://e/n').url", "[uri http://e/n]"},
		{[]string{"r4", "r4b", "r5"}, "Patient.extension('http://e/').count()", "[Integer 0]"},
	} {
		for _, m := range extensionModels {
			if !slices.Contains(tt.versions, m.name) {
				continue
			}
			if got := typed(t, m.model, tt.expr); got != tt.want {
				t.Errorf("%s: %s = %s, want %s", m.name, tt.expr, got, tt.want)
			}
		}
	}
}

// An argument of several items, or one that is no String, ends the
// evaluation, whatever the input: both pages declare extension(url : string),
// and FHIRPath's rule for a single-valued parameter makes it an error.
func TestExtensionOfAURLThatIsNoSingleStringIsAnError(t *testing.T) {
	for _, expr := range []string{
		"Patient.extension('http://e/s' | 'http://e/cc')",
		"Patient.extension(1)",
		"Patient.extension(true)",
		"{}.extension('http://e/s' | 'http://e/cc')",
		"{}.extension(1)",
		"{}.hasExtension(1)",
	} {
		for _, m := range extensionModels {
			if got := typed(t, m.model, expr); !strings.HasPrefix(got, "error: ") {
				t.Errorf("%s: %s = %s, want an error", m.name, expr, got)
			}
		}
	}
}

func TestExtensionOfNoInputOrNoURLIsEmpty(t *testing.T) {
	for _, expr := range []string{
		"{}.extension('http://e/s')",
		"Patient.extension('')",
		"Patient.extension({})",
		"Patient.extension('http://e/none')",
		"Patient.gender.extension('http://e/s')",
	} {
		for _, m := range extensionModels {
			if got := typed(t, m.model, expr); got != "[]" {
				t.Errorf("%s: %s = %s, want []", m.name, expr, got)
			}
		}
	}
}
