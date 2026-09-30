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
