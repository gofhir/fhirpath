package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// Without a model, a name that is not a field is tried as a choice element:
// value finds valueQuantity, valueString and the rest. These pin what that
// lookup answers, whichever way it is carried out.
func TestChoiceFieldWithoutModel(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		expr     string
		want     string
	}{
		{
			name:     "a choice element is found by its base name",
			resource: `{"resourceType":"Observation","valueQuantity":{"value":5,"unit":"mg"}}`,
			expr:     "value.value",
			want:     "[5]",
		},
		{
			name:     "an absent field is empty",
			resource: `{"resourceType":"Observation","status":"final"}`,
			expr:     "value.exists()",
			want:     "[false]",
		},
		{
			// Invalid FHIR, but the answer must not depend on key order: the
			// suffix listed first wins, as it always has.
			name:     "two variants resolve to the one listed first",
			resource: `{"resourceType":"Observation","valueString":"s","valueBoolean":true}`,
			expr:     "value",
			want:     "[true]",
		},
		{
			name:     "two variants in the other key order resolve the same",
			resource: `{"resourceType":"Observation","valueBoolean":true,"valueString":"s"}`,
			expr:     "value",
			want:     "[true]",
		},
		{
			// Some serializers write every unset variant as null. A null holds
			// nothing, so it must not hide the variant that does hold a value.
			name:     "a null variant does not hide one with a value",
			resource: `{"resourceType":"Observation","valueBoolean":null,"valueString":"s"}`,
			expr:     "value",
			want:     "[s]",
		},
		{
			name:     "a variant carried only by its element is found",
			resource: `{"resourceType":"Observation","_valueString":{"extension":[{"url":"http://example.org/u","valueCode":"x"}]}}`,
			expr:     "value.extension.url",
			want:     "[http://example.org/u]",
		},
		{
			name:     "a key that extends the name with something not a type is not a variant",
			resource: `{"resourceType":"Questionnaire","valueSet":"http://example.org/vs"}`,
			expr:     "value.exists()",
			want:     "[false]",
		},
		{
			name:     "the variant keeps the type its name gives it",
			resource: `{"resourceType":"Observation","valueCode":"x"}`,
			expr:     "value is code",
			want:     "[true]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, doc := range []bool{false, true} {
				got := evaluateString(t, tt.resource, tt.expr, doc)
				if got != tt.want {
					t.Errorf("%s (document=%v) = %s, want %s", tt.expr, doc, got, tt.want)
				}
			}
		})
	}
}

// A model that knows an element and has no choice types for it has said the
// element is not a choice, so a field spelled like a variant of it is not one.
func TestChoiceFieldWithModel(t *testing.T) {
	model := &testModel{
		typeOf:      map[string]string{"Basic.subject": "Reference", "Observation.value": "Element"},
		choiceTypes: map[string][]string{"Observation.value": {"Quantity", "string"}},
	}

	tests := []struct {
		name     string
		resource string
		expr     string
		want     string
	}{
		{
			name:     "a known element that is not a choice has no variants",
			resource: `{"resourceType":"Basic","subjectString":"x"}`,
			expr:     "subject.exists()",
			want:     "[false]",
		},
		{
			name:     "a choice element is found through its choice types",
			resource: `{"resourceType":"Observation","valueString":"s"}`,
			expr:     "value",
			want:     "[s]",
		},
		{
			name:     "an element the model does not know is still tried as a choice",
			resource: `{"resourceType":"Basic","fooString":"x"}`,
			expr:     "foo",
			want:     "[x]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(tt.resource), fhirpath.WithModel(model))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}
		})
	}
}

// A model's choice types are tried in its order, and only as keys the object
// holds, so they answer as the model's list would have one field at a time.
func TestChoiceFieldThroughModelChoiceTypes(t *testing.T) {
	model := &testModel{
		typeOf:      map[string]string{"Observation.value": ""},
		choiceTypes: map[string][]string{"Observation.value": {"Quantity", "string", "boolean"}},
	}

	tests := []struct {
		name     string
		resource string
		expr     string
		want     string
	}{
		{
			name:     "two variants resolve to the one the model lists first",
			resource: `{"resourceType":"Observation","valueBoolean":true,"valueString":"s"}`,
			expr:     "value",
			want:     "[s]",
		},
		{
			name:     "in either key order",
			resource: `{"resourceType":"Observation","valueString":"s","valueBoolean":true}`,
			expr:     "value",
			want:     "[s]",
		},
		{
			name:     "a null variant does not hide one with a value",
			resource: `{"resourceType":"Observation","valueString":null,"valueBoolean":true}`,
			expr:     "value",
			want:     "[true]",
		},
		{
			name:     "a variant carried only by its element is found",
			resource: `{"resourceType":"Observation","_valueString":{"extension":[{"url":"http://example.org/u","valueCode":"x"}]}}`,
			expr:     "value.extension.url",
			want:     "[http://example.org/u]",
		},
		{
			name:     "a variant the model does not list is not found",
			resource: `{"resourceType":"Observation","valueInteger":1}`,
			expr:     "value.exists()",
			want:     "[false]",
		},
		{
			name:     "an absent choice is empty",
			resource: `{"resourceType":"Observation","status":"final"}`,
			expr:     "value.exists()",
			want:     "[false]",
		},
		{
			name:     "the variant takes the type the model gives it",
			resource: `{"resourceType":"Observation","valueString":"s"}`,
			expr:     "value is FHIR.string",
			want:     "[true]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiled := fhirpath.MustCompile(tt.expr)

			result, err := compiled.EvaluateWithOptions([]byte(tt.resource), fhirpath.WithModel(model))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}

			// Twice on a Document, so the second answer comes from the cache.
			doc := fhirpath.MustNewDocument([]byte(tt.resource))
			for i := 0; i < 2; i++ {
				result, err = doc.EvaluateWithOptions(compiled, fhirpath.WithModel(model))
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

// A Document can be evaluated against more than one model, and each may give
// the same element different choice types. What one found must not answer for
// the other.
func TestChoiceFieldOnADocumentAcrossModels(t *testing.T) {
	narrow := &testModel{choiceTypes: map[string][]string{"Observation.value": {"Quantity"}}}
	wide := &testModel{choiceTypes: map[string][]string{"Observation.value": {"Quantity", "Attachment"}}}

	doc := fhirpath.MustNewDocument([]byte(`{"resourceType":"Observation","valueAttachment":{"url":"http://example.org/a"}}`))
	expr := fhirpath.MustCompile("value.exists()")

	for _, step := range []struct {
		model *testModel
		want  string
	}{
		{narrow, "[false]"},
		{wide, "[true]"},
		{narrow, "[false]"},
	} {
		result, err := doc.EvaluateWithOptions(expr, fhirpath.WithModel(step.model))
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != step.want {
			t.Errorf("value.exists() with choice types %v = %s, want %s",
				step.model.choiceTypes["Observation.value"], got, step.want)
		}
	}
}

func evaluateString(t *testing.T, resource, expr string, document bool) string {
	t.Helper()

	compiled := fhirpath.MustCompile(expr)
	var result fhirpath.Collection
	var err error
	if document {
		doc := fhirpath.MustNewDocument([]byte(resource))
		// Twice, so the second answer comes from what the first one kept.
		if _, err = doc.EvaluateCompiled(compiled); err == nil {
			result, err = doc.EvaluateCompiled(compiled)
		}
	} else {
		result, err = compiled.Evaluate([]byte(resource))
	}
	if err != nil {
		t.Fatal(err)
	}
	return result.String()
}
