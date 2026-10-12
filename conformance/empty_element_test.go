package conformance

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r4b/v2"
	"github.com/gofhir/models/r5/v2"
)

// With the generated models, an element written as an empty object is not
// there either, whether its type is read off the model or off a choice
// element's name: pat-1 fails on a contact whose name is {}, and ext-1 on an
// extension whose value is {}, as they do in the HL7 validator.
func TestAnElementWrittenAsAnEmptyObjectIsNotThereWithTheGeneratedModels(t *testing.T) {
	tests := []struct {
		resource, expr, want string
	}{
		{`{"resourceType":"Patient","contact":[{"name":{}}]}`,
			"contact.all(name.exists() or telecom.exists() or address.exists() or organization.exists())", "[false]"},
		{`{"resourceType":"Patient","contact":[{"name":{"id":"x"}}]}`, "contact.name.exists()", "[true]"},
		{`{"resourceType":"Patient","contact":[{"name":{}}]}`, "contact.descendants().count()", "[0]"},
		{`{"resourceType":"Patient","extension":[{"url":"u","valueAddress":{}}]}`,
			"extension.all(extension.exists() != value.exists())", "[false]"},
		{`{"resourceType":"Patient","extension":[{"url":"u","valueAddress":{}}]}`, "extension.value.ofType(Address).exists()", "[false]"},
		{`{"resourceType":"Bundle","entry":[{"resource":{}}]}`, "entry.resource.exists()", "[false]"},
	}

	for _, model := range []struct {
		name  string
		model fhirpath.Model
	}{{"r4", r4.FHIRPathModel()}, {"r4b", r4b.FHIRPathModel()}, {"r5", r5.FHIRPathModel()}} {
		for _, tt := range tests {
			t.Run(model.name+"/"+tt.expr, func(t *testing.T) {
				result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(tt.resource), fhirpath.WithModel(model.model))
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
				}
			})
		}
	}
}

// A root that is an element written as an empty object is not there either,
// whether the document is checked as it is read or known to be well formed.
func TestAnElementRootWrittenAsAnEmptyObjectIsNotThere(t *testing.T) {
	for _, newContext := range []struct {
		name string
		new  func([]byte) *eval.Context
	}{{"NewContext", eval.NewContext}, {"NewContextForValidJSON", eval.NewContextForValidJSON}} {
		t.Run(newContext.name, func(t *testing.T) {
			ctx := newContext.new([]byte(` {} `))
			ctx.SetModel(r4.FHIRPathModel())
			ctx.SetPath("Patient.contact.name")
			result, err := fhirpath.MustCompile("$this.exists()").EvaluateWithContext(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != "[false]" {
				t.Errorf("$this.exists() = %s, want [false]", got)
			}
		})
	}
}
