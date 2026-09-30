package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

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
