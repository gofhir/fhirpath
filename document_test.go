package fhirpath

import (
	"testing"
)

var documentPatient = []byte(`{
	"resourceType": "Patient",
	"id": "example",
	"active": true,
	"name": [
		{"use": "official", "family": "Chalmers", "given": ["Peter", "James"]},
		{"use": "usual", "given": ["Jim"]}
	],
	"birthDate": "1974-12-25",
	"_birthDate": {"extension": [{"url": "http://example.org/when", "valueString": "morning"}]}
}`)

// A document answers what an ordinary evaluation answers, and goes on doing so
// once it has read a field: the second expression down a path gets what the
// first one navigated.
func TestDocumentMatchesEvaluate(t *testing.T) {
	doc := MustNewDocument(documentPatient)

	expressions := []string{
		"Patient.name.given",
		"Patient.name.given",
		"Patient.name.where(use = 'official').family",
		"Patient.name.count()",
		"Patient.birthDate",
		"Patient.birthDate.extension.url",
		"Patient.active",
		"Patient.name.given.first()",
	}

	for _, expr := range expressions {
		t.Run(expr, func(t *testing.T) {
			want, err := Evaluate(documentPatient, expr)
			if err != nil {
				t.Fatalf("Evaluate(%q): %v", expr, err)
			}

			got, err := doc.Evaluate(expr)
			if err != nil {
				t.Fatalf("Document.Evaluate(%q): %v", expr, err)
			}

			if len(got) != len(want) {
				t.Fatalf("got %d results, want %d", len(got), len(want))
			}
			for i := range got {
				if got[i].String() != want[i].String() {
					t.Errorf("result %d: got %q, want %q", i, got[i].String(), want[i].String())
				}
			}
		})
	}
}

// What a document hands back belongs to the caller: appending to one result
// must not reach the cached collection, and so must not be visible to the next
// evaluation.
func TestDocumentResultsAreIndependent(t *testing.T) {
	doc := MustNewDocument(documentPatient)

	first, err := doc.Evaluate("Patient.name.given")
	if err != nil {
		t.Fatalf("first evaluation: %v", err)
	}
	firstLen := len(first)

	_ = append(first, first[0])

	second, err := doc.Evaluate("Patient.name.given")
	if err != nil {
		t.Fatalf("second evaluation: %v", err)
	}

	if len(second) != firstLen {
		t.Errorf("second evaluation returned %d results, want %d — the appended item leaked into the cache",
			len(second), firstLen)
	}
}

// A variable defined against one context does not carry into the next
// evaluation over the same document.
func TestDocumentContextsAreSeparate(t *testing.T) {
	doc := MustNewDocument(documentPatient)

	if _, err := doc.Evaluate("defineVariable('n', name.first()).select(%n.family)"); err != nil {
		t.Fatalf("first evaluation: %v", err)
	}

	// Defining the same name again would fail if the earlier definition had
	// outlived its evaluation.
	if _, err := doc.Evaluate("defineVariable('n', name.first()).select(%n.family)"); err != nil {
		t.Fatalf("second evaluation: %v", err)
	}
}

func TestNewDocumentRejectsInvalidJSON(t *testing.T) {
	if _, err := NewDocument([]byte("{not json")); err == nil {
		t.Error("expected an error for invalid JSON, got none")
	}
}
