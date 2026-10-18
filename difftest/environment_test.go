package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// The harness gives fhirpath.js the environment FHIR fixes at a resource's
// root, which fhirpath.js leaves to its caller, so that a comparison of an
// expression that reads it compares the engines and not the harness.
func TestFhirpathJSIsGivenTheEnvironmentFHIRFixes(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("no node")
	}
	if _, err := os.Stat("node_modules/fhirpath"); err != nil {
		t.Skip("no fhirpath.js installed (make difftest installs it)")
	}
	resource := []byte(`{"resourceType":"Patient","id":"p"}`)
	want := map[string]string{
		"%resource.id":         "p",
		"%rootResource.id":     "p",
		"%resource is Patient": "true",
		"%sct":                 "http://snomed.info/sct",
		"%loinc":               "http://loinc.org",
	}
	var cases []testCase
	for expr := range want {
		cases = append(cases, testCase{ID: expr, Expression: expr, Resource: resource, Model: "r4"})
	}
	answers, err := evaluateWithFHIRPathJS(cases)
	if err != nil {
		t.Fatal(err)
	}
	for expr, w := range want {
		a := answers[expr]
		if a.Error != "" || strings.Join(a.Results, ",") != w {
			t.Errorf("%s = %v %q, want %s", expr, a.Results, a.Error, w)
		}
	}
}
