package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

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
		{"birthDate.children().count()", "[2]"},
		{"birthDate.descendants().count() > birthDate.children().count()", "[true]"},
		// A primitive and its element are one child of their parent, not two,
		// and a value with only an element is still a child.
		{"children().count()", "[3]"},
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
		{`{"resourceType":"Patient","_tags":["a","b"],"_meta":"x"}`, "children().count()", "[4]"},
		{`{"resourceType":"Patient","_tags":["a","b"],"_meta":"x"}`, "_tags", "[a, b]"},
		{`{"resourceType":"Patient","_tags":["a","b"],"_active":{"id":"z"},"active":true}`, "children().count()", "[4]"},
		{`{"resourceType":"Patient","birthDate":"2020","birthDate":"2021"}`, "children().count()", "[3]"},
		{`{"resourceType":"Patient","birthDate":"2020","birthDate":"2021","_active":{"id":"z"}}`, "children().count()", "[4]"},
		{`{"resourceType":"Patient","active":true,"_active":{"id":"a"},"active":false,"_active":{"id":"b"}}`, "children().count()", "[4]"},
		{`{"resourceType":"Patient","active":true,"_active":{"id":"a"},"active":false,"_active":{"id":"b"}}`, "descendants().where($this = 'b').count()", "[1]"},
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
