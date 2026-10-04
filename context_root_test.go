package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
)

// NewContext checks the document it is given before reading it, and answers a
// malformed one empty: a document cut off, or one followed by more, is not read
// as though it were whole.
func TestNewContextAnswersAMalformedDocumentEmpty(t *testing.T) {
	expr := fhirpath.MustCompile("id")
	for _, tt := range []struct {
		input, want string
	}{
		{`{"resourceType":"Patient","id":"x"}`, "[x]"},
		{"  \n\t{\"resourceType\":\"Patient\",\"id\":\"x\"}  \n", "[x]"},
		{`{"resourceType":"Patient","id":"x"}  trailing`, "[x]"},
		{`{"resourceType":"Patient","id":"x"`, "[]"},
		{`{"resourceType":"Patient","id":"x",`, "[]"},
		{`{"resourceType":"Patient","id":"x","meta":{"versionId":"1"}`, "[]"},
		{``, "[]"},
		{`   `, "[]"},
		{`not json`, "[]"},
		{`null`, "[]"},
		{`"x"`, "[]"},
		{`[{"id":"a"},{"id":"b"}]`, "[a, b]"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			result, err := expr.Evaluate([]byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("id on %q = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}

// A document followed by more is read as far as its own value ends, so $this
// is the object alone.
func TestNewContextReadsADocumentAsFarAsItsValue(t *testing.T) {
	result, err := fhirpath.MustCompile("$this").Evaluate([]byte(`{"id":"x"}{"id":"y"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != `[{"id":"x"}]` {
		t.Errorf(`$this = %s, want [{"id":"x"}]`, got)
	}
}

// NewContextForValidJSON reads a document already known to be well formed —
// one its caller has decoded or checked — without scanning it first, and
// answers for it exactly as NewContext does.
func TestNewContextForValidJSONAnswersAsNewContext(t *testing.T) {
	resource := []byte(` {"resourceType":"Patient","id":"x","active":true,` +
		`"name":[{"family":"F","given":["A","B"]}],"_birthDate":{"id":"b"},"birthDate":"2020"} `)
	for _, text := range []string{"id", "active", "name.given", "birthDate.id", "birthDate", "$this.id", "%resource.name.family"} {
		expr := fhirpath.MustCompile(text)
		want, err := expr.EvaluateWithContext(eval.NewContext(resource))
		if err != nil {
			t.Fatal(err)
		}
		got, err := expr.EvaluateWithContext(eval.NewContextForValidJSON(resource))
		if err != nil {
			t.Fatal(err)
		}
		if got.String() != want.String() {
			t.Errorf("%s = %s, want %s as NewContext answers", text, got, want)
		}
	}
}
