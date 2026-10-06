package main

import (
	"path/filepath"
	"testing"

	"github.com/gofhir/fhirpath"
)

// The model the corpus runs with orders children as R4's definitions do, so a
// run measures what a validator building its model from them gets: the
// Reference written type before reference still has reference first.
func TestTheCorpusModelOrdersChildrenAsTheDefinitions(t *testing.T) {
	version := versions["r4"]
	coreDir, err := fetch(filepath.Join("..", "build", "corpusdiff", "cache"), version.core)
	if err != nil {
		t.Skip("the R4 core package is not available:", err)
	}
	model, err := withDefinitionOrder(version.model(), coreDir)
	if err != nil {
		t.Fatal(err)
	}

	observation := []byte(`{"resourceType":"Observation","status":"final","code":{"text":"x"},` +
		`"subject":{"display":"Peter","type":"Patient","reference":"Patient/1"}}`)
	for expr, want := range map[string]string{
		"Observation.subject.children().first()": "[Patient/1]",
		"Observation.children().first()":         "[final]",
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
