package main

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// The model the corpus runs with, gofhir/models', orders children as R4's
// definitions do, so a run measures what a validator building its model from
// them gets: the
// Reference written type before reference still has reference first.
func TestTheCorpusModelOrdersChildrenAsTheDefinitions(t *testing.T) {
	model := versions["r4"].model()
	if _, ordered := model.(fhirpath.ElementOrder); !ordered {
		t.Fatal("the R4 model does not implement fhirpath.ElementOrder")
	}

	observation := []byte(`{"resourceType":"Observation","status":"final","code":{"text":"x"},` +
		`"subject":{"display":"Peter","type":"Patient","reference":"Patient/1"},` +
		`"component":[{"valueString":"c","code":{"text":"t"}}],` +
		`"effectiveDateTime":"2020","_effectiveDateTime":{"extension":[{"valueCode":"c","url":"u"}],"id":"e1"}}`)
	for expr, want := range map[string]string{
		"Observation.subject.children().first()": "[Patient/1]",
		"Observation.children().first()":         "[final]",
		// A primitive's element by its type, date, and an extension in it as
		// an Extension.
		"Observation.effective.children().first()":           "[e1]",
		"Observation.effective.extension.children().first()": "[u]",
		// A backbone element is ordered by its path, not by BackboneElement.
		"Observation.component.children().first()": `[{"text":"t"}]`,
	} {
		result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(observation, fhirpath.WithModel(model))
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
