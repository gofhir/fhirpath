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
				"Observation.subject":   "Reference",
				"Observation.code":      "CodeableConcept",
				"Observation.component": "BackboneElement",
				"Observation.status":    "code",
			},
			choiceTypes: map[string][]string{
				"Observation.value":           {"Quantity", "string"},
				"Observation.component.value": {"Quantity", "string"},
			},
		},
		children: map[string][]string{
			"Observation":           {"id", "meta", "extension", "status", "code", "subject", "value[x]", "component"},
			"Reference":             {"id", "extension", "reference", "type", "identifier", "display"},
			"Observation.component": {"id", "extension", "modifierExtension", "code", "value[x]"},
			"CodeableConcept":       {"id", "extension", "coding", "text"},
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

// The element FHIR writes beside a primitive under _name is an Element: id,
// then extension, whatever the JSON says; and the primitive's value, a child
// since 1.10.5, comes after them.
func TestAPrimitivesElementIsInElementsOrder(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient",` +
		`"birthDate":"2000","_birthDate":{"extension":[{"url":"u","valueCode":"c"}],"id":"b1"}}`)
	model := &orderedModel{
		testModel: &testModel{typeOf: map[string]string{"Patient.birthDate": "date"}},
		children:  map[string][]string{"Patient": {"id", "birthDate"}},
	}

	result, err := fhirpath.MustCompile("Patient.birthDate.children()").EvaluateWithOptions(patient, fhirpath.WithModel(model))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != `[b1, {"url":"u","valueCode":"c"}, 2000]` {
		t.Errorf("birthDate.children() = %s, want its id, its extension, its value", got)
	}
}

// descendants() over a Bundle of Observations, with a model that orders
// children: written in the definition's order, as HL7 and HAPI write, the
// order costs only checking it; written in another order, each object is
// reordered.
func BenchmarkDescendantsInDefinitionOrder(b *testing.B) {
	entry := func(inOrder bool) string {
		if inOrder {
			return `{"resourceType":"Observation","id":"o","status":"final","code":{"text":"x"},` +
				`"subject":{"reference":"Patient/1","type":"Patient","display":"P"},"valueString":"v"}`
		}
		return `{"resourceType":"Observation","valueString":"v",` +
			`"subject":{"display":"P","type":"Patient","reference":"Patient/1"},"code":{"text":"x"},"status":"final","id":"o"}`
	}
	model := newOrderedModel()
	model.children["Observation"] = []string{"id", "status", "code", "subject", "value[x]"}

	for _, inOrder := range []bool{true, false} {
		name := "written in order"
		if !inOrder {
			name = "written out of order"
		}
		b.Run(name, func(b *testing.B) {
			doc := []byte(`[` + entry(inOrder) + `]`)
			for range 99 {
				doc = append(doc[:len(doc)-1], []byte(`,`+entry(inOrder)+`]`)...)
			}
			compiled := fhirpath.MustCompile("descendants().count()")
			b.ReportAllocs()
			for b.Loop() {
				if _, err := compiled.EvaluateWithOptions(doc, fhirpath.WithModel(model)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	b.Run("without a model that orders", func(b *testing.B) {
		doc := []byte(`[` + entry(false) + `]`)
		for range 99 {
			doc = append(doc[:len(doc)-1], []byte(`,`+entry(false)+`]`)...)
		}
		compiled := fhirpath.MustCompile("descendants().count()")
		b.ReportAllocs()
		for b.Loop() {
			if _, err := compiled.EvaluateWithOptions(doc, fhirpath.WithModel(model.testModel)); err != nil {
				b.Fatal(err)
			}
		}
	})
}
