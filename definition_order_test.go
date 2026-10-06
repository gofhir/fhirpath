package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

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
		// A backbone element is ordered by the path it was reached by; an
		// array keeps its own order.
		{"Observation.component.first().children().first().children().first()", "[{\"code\":\"a\"}]", "[c1]"},
		{"Observation.component.children().count()", "[3]", "[3]"},
		{"Observation.component.code.text", "[t1, t2]", "[t1, t2]"},
		{"Observation.component.first().children().first()", `[{"text":"t1","coding":[{"code":"a"}]}]`, "[c1]"},
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
// declares: _birthDate as date, id, then extension, and its value after them;
// an extension in it as an Extension, as one on the resource is. An object the
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
		"Patient.birthDate.children()":                    `[b1, {"valueCode":"c","url":"u"}, 2000]`,
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
