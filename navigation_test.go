package fhirpath_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// TestReferenceElementNavigation covers an element whose name collides with the
// name of its own inferred type. Any object carrying a "reference" field is
// inferred as a Reference, and type-name navigation used to swallow the field:
// "reference" returned the enclosing object instead of the string, so
// startsWith('#') was always false and ref-1 could never fail.
func TestReferenceElementNavigation(t *testing.T) {
	reference := []byte(`{"reference":"#nope","display":"contained patient"}`)

	t.Run("navigates to the field value, not the object", func(t *testing.T) {
		assertStringResult(t, evalOrFatal(t, reference, "reference"), "#nope")
	})

	t.Run("string functions apply to the field value", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, reference, "reference.startsWith('#')"), true)
		assertStringResult(t, evalOrFatal(t, reference, "reference.substring(1)"), "nope")
		assertIntegerResult(t, evalOrFatal(t, reference, "reference.length()"), 5)
	})

	t.Run("type-name navigation still works for uppercase names", func(t *testing.T) {
		patient := []byte(`{"resourceType":"Patient","id":"p1"}`)
		assertStringResult(t, evalOrFatal(t, patient, "Patient.id"), "p1")
	})

	t.Run("ref-1 fails for a dangling local reference", func(t *testing.T) {
		resource := []byte(`{
			"resourceType": "Observation",
			"subject": {"reference": "#nope"},
			"contained": [{"resourceType": "Patient", "id": "p1"}]
		}`)
		assertBooleanResult(t, evalOrFatal(t, resource, "subject.all("+ref1+")"), false)
	})

	t.Run("ref-1 passes for a resolvable local reference", func(t *testing.T) {
		resource := []byte(`{
			"resourceType": "Observation",
			"subject": {"reference": "#p1"},
			"contained": [{"resourceType": "Patient", "id": "p1"}]
		}`)
		assertBooleanResult(t, evalOrFatal(t, resource, "subject.all("+ref1+")"), true)
	})

	t.Run("ref-1 passes for an external reference", func(t *testing.T) {
		resource := []byte(`{"resourceType":"Observation","subject":{"reference":"Patient/123"}}`)
		assertBooleanResult(t, evalOrFatal(t, resource, "subject.all("+ref1+")"), true)
	})
}

func TestNavigation(t *testing.T) {
	t.Run("simple path", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "Patient.id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "123")
	})

	t.Run("boolean field", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "Patient.active")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("nested path", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "Patient.name.family")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should return all family names
		if result.Count() != 1 {
			t.Errorf("expected 1 family name, got %d", result.Count())
		}
		assertStringResult(t, result, "Doe")
	})

	t.Run("array navigation", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "Patient.name.given")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should return all given names from all name entries
		if result.Count() != 3 {
			t.Errorf("expected 3 given names, got %d: %v", result.Count(), result)
		}
	})

	t.Run("non-existent path", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "Patient.nonexistent")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Empty() {
			t.Errorf("expected empty collection for non-existent path, got %v", result)
		}
	})
}

func TestPolymorphicElements(t *testing.T) {
	t.Run("Observation.value resolves valueQuantity", func(t *testing.T) {
		result, err := fhirpath.Evaluate(observationWithQuantity, "Observation.value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Empty() {
			t.Fatal("expected non-empty result for Observation.value")
		}
		// Should resolve to the Quantity object
		if result[0].Type() != "Quantity" {
			t.Errorf("expected Quantity type, got %s", result[0].Type())
		}
	})

	t.Run("Observation.value resolves valueString", func(t *testing.T) {
		result, err := fhirpath.Evaluate(observationWithString, "Observation.value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Empty() {
			t.Fatal("expected non-empty result for Observation.value")
		}
		assertStringResult(t, result, "Normal findings")
	})

	t.Run("Observation.value resolves valueCodeableConcept", func(t *testing.T) {
		result, err := fhirpath.Evaluate(observationWithCodeableConcept, "Observation.value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Empty() {
			t.Fatal("expected non-empty result for Observation.value")
		}
		if result[0].Type() != "CodeableConcept" {
			t.Errorf("expected CodeableConcept type, got %s", result[0].Type())
		}
	})

	t.Run("Direct access valueQuantity still works", func(t *testing.T) {
		result, err := fhirpath.Evaluate(observationWithQuantity, "Observation.valueQuantity")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Empty() {
			t.Fatal("expected non-empty result")
		}
		if result[0].Type() != "Quantity" {
			t.Errorf("expected Quantity type, got %s", result[0].Type())
		}
	})

	t.Run("Access value.value (nested) for Quantity", func(t *testing.T) {
		result, err := fhirpath.Evaluate(observationWithQuantity, "Observation.value.value")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 72)
	})

	t.Run("Access value.unit for Quantity", func(t *testing.T) {
		result, err := fhirpath.Evaluate(observationWithQuantity, "Observation.value.unit")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "beats/min")
	})
}

// TestDescendantsDepthLimit covers descendants() honoring MaxDepth, which
// EvalOptions documents but the previous unbounded recursion ignored.
func TestDescendantsDepthLimit(t *testing.T) {
	nested := []byte(`{"resourceType":"Observation","a":{"b":{"c":{"d":"deep"}}}}`)

	full := evalOrFatal(t, nested, "descendants().count()")
	shallow, err := fhirpath.MustCompile("descendants().count()").EvaluateWithOptions(nested, fhirpath.WithMaxDepth(1))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(full) != 1 || len(shallow) != 1 {
		t.Fatalf("expected singleton counts, got %v and %v", full, shallow)
	}
	fullCount := full[0].String()
	shallowCount := shallow[0].String()
	if fullCount == shallowCount {
		t.Errorf("MaxDepth had no effect: both returned %s", fullCount)
	}
	// depth 1 reaches only the resource's direct children: status is absent
	// here, and resourceType is not a child, so "a" alone
	assertIntegerResult(t, shallow, 1)
}

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

// orderedModel is a testModel that also knows the order definitions list
// their children in, as fhirpath.ElementOrder asks.
type orderedModel struct {
	*testModel
	children map[string][]string
}

func (m *orderedModel) ChildElements(path string) []string { return m.children[path] }

func newOrderedModel() *orderedModel {
	return &orderedModel{
		testModel: &testModel{
			typeOf: map[string]string{
				"Observation.subject":        "Reference",
				"Observation.code":           "CodeableConcept",
				"Observation.component":      "BackboneElement",
				"Observation.component.code": "CodeableConcept",
				"Observation.status":         "code",
			},
			choiceTypes: map[string][]string{
				"Observation.value":           {"Quantity", "string"},
				"Observation.component.value": {"Quantity", "string"},
			},
			resources: map[string]bool{"Observation": true, "Patient": true, "Bundle": true, "Questionnaire": true},
		},
		children: map[string][]string{
			"Observation":           {"id", "meta", "extension", "status", "code", "subject", "value[x]", "component"},
			"Reference":             {"id", "extension", "reference", "type", "identifier", "display"},
			"Observation.component": {"id", "extension", "modifierExtension", "code", "value[x]"},
			// What every backbone element has, which its own path, not its
			// type, completes.
			"BackboneElement":    {"id", "extension", "modifierExtension"},
			"Questionnaire.item": {"id", "linkId", "text", "item"},
			"CodeableConcept":    {"id", "extension", "coding", "text"},
		},
	}
}

// The specification leaves the order of children() and descendants()
// undefined and allows "the logical order implied by the object model", the
// order the HL7 validator returns them in. Without a model, or with one that
// does not know the order, they come as the JSON writes them, so an instance
// written with its keys in another order gave another answer: a profile's
// children().first().toString() = 'Patient/1' on Observation.subject held in
// HL7 and failed here when type came before reference.
func TestChildrenComeInTheDefinitionsOrder(t *testing.T) {
	observation := []byte(`{"resourceType":"Observation",` +
		`"subject":{"display":"Peter","type":"Patient","reference":"Patient/1"},` +
		`"valueString":"v","status":"final","unknown":"u",` +
		`"component":[{"valueString":"c1","code":{"text":"t1","coding":[{"code":"a"}]}},{"code":{"text":"t2"}}],` +
		`"id":"o1","code":{"text":"x"}}`)

	tests := []struct {
		expr, ordered, asWritten string
	}{
		{"Observation.subject.children()", "[Patient/1, Patient, Peter]", "[Peter, Patient, Patient/1]"},
		{"Observation.subject.children().first().toString() = 'Patient/1'", "[true]", "[false]"},
		// A choice element is listed by its name with [x]; a field the
		// definition does not list comes last, as written.
		{"Observation.children().first()", "[o1]", `[{"display":"Peter","type":"Patient","reference":"Patient/1"}]`},
		{"Observation.children().last()", "[u]", `[{"text":"x"}]`},
		{"Observation.children().skip(1).first()", "[final]", "[v]"},
		// A backbone element is ordered by the path it was reached by, and
		// what it holds by its own type; an array keeps its own order.
		{"Observation.component.first().children().first()", `[{"text":"t1","coding":[{"code":"a"}]}]`, "[c1]"},
		{"Observation.component.first().code.children().first()", "[{\"code\":\"a\"}]", "[t1]"},
		{"Observation.component.children().count()", "[3]", "[3]"},
		{"Observation.component.code.text", "[t1, t2]", "[t1, t2]"},
		// descendants() is children() repeated, in the same order.
		{"Observation.subject.descendants().first()", "[Patient/1]", "[Peter]"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			compiled := fhirpath.MustCompile(tt.expr)

			// Through WithModel, which wraps the model in an adapter.
			result, err := compiled.EvaluateWithOptions(observation, fhirpath.WithModel(newOrderedModel()))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.ordered {
				t.Errorf("with an ordering model = %s, want %s", got, tt.ordered)
			}

			// Set on a context directly, as gofhir/validator sets its own.
			col, err := types.JSONToCollection(observation)
			if err != nil {
				t.Fatal(err)
			}
			ctx := eval.NewContextForRoot(col)
			ctx.SetModel(newOrderedModel())
			result, err = compiled.EvaluateWithContext(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.ordered {
				t.Errorf("with an ordering model on a context = %s, want %s", got, tt.ordered)
			}

			// A model that does not know the order leaves them as written.
			result, err = compiled.EvaluateWithOptions(observation, fhirpath.WithModel(newOrderedModel().testModel))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.asWritten {
				t.Errorf("with a model that does not order = %s, want %s", got, tt.asWritten)
			}
		})
	}
}

// The element FHIR writes beside a primitive under _name has no type of its
// own; it is ordered, and its children typed, by the type the primitive's path
// declares: _birthDate as date, id, then extension; its value is no child. An
// extension in it is typed as an Extension, as one on the resource is. An object the
// model knows nothing of stays as the JSON writes it.
func TestAPrimitivesElementIsInItsTypesOrder(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient",` +
		`"birthDate":"2000","_birthDate":{"extension":[{"valueCode":"c","url":"u"}],"id":"b1"},` +
		`"extension":[{"valueCode":"c","url":"u"}]}`)
	model := &orderedModel{
		testModel: &testModel{resources: map[string]bool{"Patient": true}, typeOf: map[string]string{
			"Patient.birthDate":   "date",
			"Patient.extension":   "Extension",
			"date.extension":      "Extension",
			"Extension.valueCode": "code",
		}},
		children: map[string][]string{
			"Patient":   {"id", "extension", "birthDate"},
			"date":      {"id", "extension", "value"},
			"Extension": {"id", "extension", "url", "value[x]"},
		},
	}

	for expr, want := range map[string]string{
		"Patient.birthDate.children()":                    `[b1, {"valueCode":"c","url":"u"}]`,
		"Patient.birthDate.extension.children().first()":  "[u]",
		"Patient.extension.children().first()":            "[u]",
		"Patient.birthDate.descendants().first()":         "[b1]",
		"Patient.birthDate.extension.value.is(FHIR.code)": "[true]",
	} {
		result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(patient, fhirpath.WithModel(model))
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}

	// A resource the model does not know: as written.
	unknown := []byte(`{"resourceType":"ActorDefinition","meta":{"versionId":"1"},"text":{"status":"generated"},` +
		`"extension":[{"url":"u","valueCode":"c"}]}`)
	result, err := fhirpath.MustCompile("children().first()").EvaluateWithOptions(unknown, fhirpath.WithModel(model))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != `[{"versionId":"1"}]` {
		t.Errorf("children().first() of a resource the model does not know = %s, want its meta, as written", got)
	}
}

// A path defined elsewhere is ordered as the definition it borrows: R4's
// Questionnaire.item.item is Questionnaire.item, by contentReference.
func TestAPathDefinedElsewhereIsInItsDefinitionsOrder(t *testing.T) {
	questionnaire := []byte(`{"resourceType":"Questionnaire",` +
		`"item":[{"item":[{"text":"inner","linkId":"2"}],"text":"outer","linkId":"1"}]}`)
	model := newOrderedModel()
	model.typeOf["Questionnaire.item"] = "BackboneElement"
	model.typeOf["Questionnaire.item.item"] = "BackboneElement"
	model.resolvePath = map[string]string{"Questionnaire.item.item": "Questionnaire.item"}

	for expr, want := range map[string]string{
		"Questionnaire.item.children().first()":      "[1]",
		"Questionnaire.item.item.children().first()": "[2]",
	} {
		result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(questionnaire, fhirpath.WithModel(model))
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// A resource held by an element typed Resource, an entry's or a contained
// one, is ordered by the resource it is, not by Resource, whose definition
// lists only what every resource has; descendants() reads it so as well as
// children().
func TestAResourceInAResourceElementIsInItsOwnOrder(t *testing.T) {
	bundle := []byte(`{"resourceType":"Bundle","entry":[{"resource":` +
		`{"resourceType":"Patient","birthDate":"2000-01-01","gender":"male","id":"p"}}]}`)
	model := &orderedModel{
		testModel: &testModel{resources: map[string]bool{"Bundle": true, "Patient": true}, typeOf: map[string]string{
			"Bundle.entry":          "BackboneElement",
			"Bundle.entry.resource": "Resource",
			"Patient.gender":        "code",
			"Patient.birthDate":     "date",
		}},
		children: map[string][]string{
			"Bundle":       {"id", "entry"},
			"Bundle.entry": {"id", "extension", "modifierExtension", "resource"},
			"Resource":     {"id", "meta", "implicitRules", "language"},
			"Patient":      {"id", "meta", "gender", "birthDate"},
		},
	}

	for expr, want := range map[string]string{
		"Bundle.entry.resource.children()":           "[p, male, 2000-01-01]",
		"Bundle.entry.descendants().skip(1).take(3)": "[p, male, 2000-01-01]",
	} {
		result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(bundle, fhirpath.WithModel(model))
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// An object whose type is only guessed from its fields is not ordered by the
// guess: a Quantity's shape at a path the model does not know may be anything,
// and an Identifier's guess may be a ContactPoint. It stays as written.
func TestAnObjectOfAGuessedTypeStaysAsWritten(t *testing.T) {
	observation := []byte(`{"resourceType":"Observation","foo":{"unit":"mg","value":1}}`)
	model := newOrderedModel()
	model.children["Quantity"] = []string{"id", "extension", "value", "comparator", "unit", "system", "code"}

	result, err := fhirpath.MustCompile("Observation.foo.children()").EvaluateWithOptions(observation, fhirpath.WithModel(model))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != "[mg, 1]" {
		t.Errorf("Observation.foo.children() = %s, want [mg, 1], as written", got)
	}
}

// An element a model types a System type, as R4 types Resource.id
// System.String, is still a primitive FHIR writes with an element beside it,
// _id, which is ordered as FHIR's primitive of that kind: id, then extension.
func TestTheElementOfASystemTypedPrimitiveIsInOrder(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","_id":{"extension":[{"url":"u","valueCode":"c"}],"id":"i1"},"id":"p1"}`)
	model := &orderedModel{
		testModel: &testModel{
			resources: map[string]bool{"Patient": true},
			typeOf:    map[string]string{"Patient.id": "http://hl7.org/fhirpath/System.String"},
		},
		children: map[string][]string{
			"Patient": {"id", "meta"},
			"string":  {"id", "extension", "value"},
		},
	}

	result, err := fhirpath.MustCompile("Patient.id.children()").EvaluateWithOptions(patient, fhirpath.WithModel(model))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != `[i1, {"url":"u","valueCode":"c"}]` {
		t.Errorf("Patient.id.children() = %s, want its id, its extension", got)
	}
}

// Each step of a path is resolved against the element the step before it
// reached. Navigation used to lose that element between steps, so a third step
// was resolved against the first: Claim.diagnosis.diagnosis looked up
// Claim.diagnosis — the backbone, not the choice beneath it — and found no
// variant, and Basic.a.when took the type of Basic.when.
func TestEachStepResolvesAgainstTheElementBeforeIt(t *testing.T) {
	model := &testModel{
		typeOf: map[string]string{
			"Claim.diagnosis":                     "BackboneElement",
			"Claim.diagnosis.sequence":            "positiveInt",
			"ImplementationGuide.name":            "string",
			"ImplementationGuide.definition":      "BackboneElement",
			"ImplementationGuide.definition.page": "BackboneElement",
			"Basic.a":                             "BackboneElement",
			"Basic.a.when":                        "date",
			"Basic.when":                          "string",
		},
		choiceTypes: map[string][]string{
			"Claim.diagnosis.diagnosis":                {"CodeableConcept", "Reference"},
			"ImplementationGuide.definition.page.name": {"url", "Reference"},
		},
		resolvePath: map[string]string{
			"ImplementationGuide.definition.page.page": "ImplementationGuide.definition.page",
		},
	}

	claim := `{"resourceType":"Claim","diagnosis":[{"sequence":1,"diagnosisCodeableConcept":{"text":"flu"}}]}`
	guide := `{"resourceType":"ImplementationGuide","name":"guide","definition":{"page":{"nameUrl":"index.html",` +
		`"page":[{"nameUrl":"child.html"}]}}}`
	basic := `{"resourceType":"Basic","when":"text","a":{"when":"2020-01-01"}}`

	tests := []struct {
		name     string
		resource string
		expr     string
		want     string
	}{
		{"a choice named like its parent", claim, "Claim.diagnosis.diagnosis.text", "[flu]"},
		{"a choice below two backbones", guide, "ImplementationGuide.definition.page.name", "[index.html]"},
		{"a choice through a content reference", guide, "ImplementationGuide.definition.page.page.name", "[child.html]"},
		{"a primitive takes its own element's type", basic, "Basic.a.when is FHIR.date", "[true]"},
		{"after where()", basic, "Basic.a.where(when.exists()).when is FHIR.date", "[true]"},
		{"after select()", basic, "Basic.select(a).when is FHIR.date", "[true]"},
		{"children() of a backbone", basic, "Basic.a.children().first() is FHIR.date", "[true]"},
		{"descendants() of a backbone", basic, "Basic.a.descendants().first() is FHIR.date", "[true]"},
		{"a sibling of the parent keeps its type", claim, "Claim.diagnosis.sequence is FHIR.positiveInt", "[true]"},
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

			// Twice on a Document: the objects it keeps must not carry a path
			// from one evaluation into the next.
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

// A JSON string may be written with escapes, and it is the same string:
// "PatiBSu0065nt" is Patient. Navigation decoded the root's resourceType but
// Location took it as it was written, so a validator comparing Location with
// its own path found nothing. And a root whose first resourceType names
// nothing is named as navigation types it, by the one that does.
func TestARootIsNamedByItsDecodedResourceType(t *testing.T) {
	for _, tt := range []struct {
		json, want string
	}{
		{`{"resourceType":"PatiBSu0065nt","name":[{"family":"A"}]}`, "Patient.name[0]"},
		{`{"resourceType":"BSu0050atient","name":[{"family":"A"}]}`, "Patient.name[0]"},
		{`{"resourceType":null,"resourceType":"Patient","name":[{"family":"A"}]}`, "Patient.name[0]"},
		{`{"resourceType":"Patient","nBSu0061me":[{"family":"A"}]}`, "Patient.name[0]"},
	} {
		data := escaped(tt.json)
		for how, read := range map[string]func([]byte) (types.Collection, error){
			"JSONToCollection": types.JSONToCollection,
			"ReadRoot":         types.ReadRoot,
			"ReadRootWithType": func(d []byte) (types.Collection, error) { return types.ReadRootWithType(d, "Patient") },
		} {
			col, err := read(data)
			if err != nil {
				t.Fatal(err)
			}
			result, err := fhirpath.MustCompile("Patient.name").EvaluateWithContext(eval.NewContextForRoot(col))
			if err != nil || len(result) != 1 {
				t.Fatalf("%s %s: %v %v", how, data, err, result)
			}
			if got := result[0].(*types.ObjectValue).Location(); got != tt.want {
				t.Errorf("%s %s: name is at %q, want %q", how, data, got, tt.want)
			}
		}
	}
}

const indexedPatient = `{"resourceType":"Patient","id":"p","active":true,"gender":"female",` +
	`"birthDate":"1974-12-25","_birthDate":{"extension":[{"url":"u","valueCode":"x"}]},` +
	`"name":[{"family":"F","given":["A","B"]}],"birthDate":"1999-01-01"}`

// An object read more than once is indexed on its second read; every read
// after it answers from the index exactly as a scan would, the element beside
// a value and the first occurrence of a repeated key included.
func TestReadingAnIndexedObjectAnswersAsScanningIt(t *testing.T) {
	expr := "id | active.toString() | gender | birthDate.toString() | birthDate.extension.url | " +
		"name.family | name.given | id | gender | deceased.exists().toString()"
	want := "[p, true, female, 1974-12-25, u, F, A, B, false]"

	result, err := fhirpath.Evaluate([]byte(indexedPatient), expr)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != want {
		t.Errorf("one-shot: %s, want %s", got, want)
	}

	doc := fhirpath.MustNewDocument([]byte(indexedPatient))
	for i := 0; i < 3; i++ {
		result, err = doc.Evaluate(expr)
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != want {
			t.Errorf("Document pass %d: %s, want %s", i+1, got, want)
		}
	}
}

// A root a caller builds is not indexed, so it can be shared across
// goroutines as before; a root NewContext reads is each evaluation's own.
// Run with -race.
func TestIndexingDoesNotWriteToASharedRoot(t *testing.T) {
	shared, err := types.JSONToCollection([]byte(indexedPatient))
	if err != nil {
		t.Fatal(err)
	}
	expr := fhirpath.MustCompile("id | gender | name.family | name.given | birthDate.extension.url")

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if _, err := expr.EvaluateWithContext(eval.NewContextForRoot(shared)); err != nil {
					t.Error(err)
				}
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if _, err := expr.Evaluate([]byte(indexedPatient)); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}

// The index keeps where each field lies in the object's JSON; every kind of
// value is read back from it exactly, and an object whose keys are written
// with escapes, which the index cannot place, is read by scanning instead.
func TestTheIndexReadsEveryKindOfValueBack(t *testing.T) {
	for _, tt := range []struct {
		resource, expr, want string
	}{
		{`{"resourceType":"Basic","a":"x\"y\\z","b":"","c":12.50,"d":{"e":[1,2]},"f":true,"g":null,"h":"é"}`,
			"a | b.exists().toString() | c.toString() | d.e.count().toString() | f.toString() | g.exists().toString() | h | a",
			`[x"y\z, true, 12.50, 2, false, é]`},
		{`{"resourceType":"Basic","family":"F","given":"G","family":"X"}`,
			"given | family | given | family", "[G, F]"},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			for _, doc := range []bool{false, true} {
				var result fhirpath.Collection
				var err error
				if doc {
					result, err = fhirpath.MustNewDocument([]byte(tt.resource)).Evaluate(tt.expr)
				} else {
					result, err = fhirpath.Evaluate([]byte(tt.resource), tt.expr)
				}
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s (document=%v) = %s, want %s", tt.expr, doc, got, tt.want)
				}
			}
		})
	}
}

// resourceType is how JSON says which resource an object is, not an element
// of it: no StructureDefinition lists it, and the HL7 validator skips it when
// it reads a resource's children. It was a child here, so a resource had one
// child more than it has, and children().first() on one written with
// resourceType first, as every resource is, was the resource's type name.
func TestResourceTypeIsNotAChild(t *testing.T) {
	observation := []byte(`{"resourceType":"Observation","id":"o1","status":"final",` +
		`"contained":[{"resourceType":"Patient","id":"p"}],"subject":{"reference":"#p"}}`)
	model := &testModel{typeOf: map[string]string{"Observation.subject": "Reference"}}

	for expr, want := range map[string]string{
		"Observation.children().count()":                         "[4]",
		"Observation.children().where($this = 'Observation')":    "[]",
		"Observation.children().first()":                         "[o1]",
		"Observation.descendants().where($this = 'Patient')":     "[]",
		"Observation.contained.children().count()":               "[1]",
		"Observation.descendants().ofType(Patient).id":           "[p]",
		"Observation.contained.ofType(Patient).exists()":         "[true]",
		"Observation.children().select(children()).count() >= 0": "[true]",
	} {
		compiled := fhirpath.MustCompile(expr)
		for how, opts := range map[string][]fhirpath.EvalOption{
			"without a model": nil,
			"with a model":    {fhirpath.WithModel(model)},
		} {
			result, err := compiled.EvaluateWithOptions(observation, opts...)
			if err != nil {
				t.Fatalf("%s %s: %v", expr, how, err)
			}
			if got := result.String(); got != want {
				t.Errorf("%s %s = %s, want %s", expr, how, got, want)
			}
		}
	}
}

// An element may have a field named resourceType: R4's
// ExampleScenario.instance.resourceType is a code naming the type of the
// instance the scenario describes, and a child like any other. Only a
// resource's own resourceType is not. A model types the instance a
// BackboneElement; without one nothing tells it from a resource, and it is
// read as the Patient its resourceType names, as is() reads it.
func TestAnElementsResourceTypeFieldIsAChild(t *testing.T) {
	scenario := []byte(`{"resourceType":"ExampleScenario","status":"draft",` +
		`"instance":[{"resourceId":"a","resourceType":"Patient"}]}`)
	model := &testModel{typeOf: map[string]string{
		"ExampleScenario.instance":              "BackboneElement",
		"ExampleScenario.instance.resourceId":   "string",
		"ExampleScenario.instance.resourceType": "code",
	}}

	for expr, want := range map[string]string{
		"ExampleScenario.children().count()":                              "[2]",
		"ExampleScenario.instance.children().count()":                     "[2]",
		"ExampleScenario.descendants().where($this = 'Patient').exists()": "[true]",
		"ExampleScenario.instance.resourceType":                           "[Patient]",
	} {
		result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(scenario, fhirpath.WithModel(model))
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != want {
			t.Errorf("%s with a model = %s, want %s", expr, got, want)
		}
	}
}

// A resourceType written with escapes is the same resource's: "Pati\u0065nt"
// is Patient, and still not a child.
func TestAnEscapedResourceTypeIsNotAChild(t *testing.T) {
	patient := []byte(strings.ReplaceAll(`{"resourceType":"PatiBSu0065nt","id":"p"}`, "BS", string(rune(92))))
	result, err := fhirpath.MustCompile("children().count()").Evaluate(patient)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != "[1]" {
		t.Errorf("children().count() = %s, want [1]", got)
	}
}
