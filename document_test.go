package fhirpath

import (
	"testing"

	"github.com/gofhir/fhirpath/types"
)

var documentPatient = []byte(`{
	"resourceType": "Patient",
	"id": "example",
	"active": true,
	"name": [
		{"use": "official", "family": "Chalmers", "given": ["Peter", "James"]},
		{"use": "usual", "given": ["Jim"]}
	],
	"given": [null, "James"],
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

// What a cached field hands back belongs to the caller: appending to it must
// copy rather than write into the collection the document keeps.
//
// The test reaches the cache directly, because an expression's result is
// several steps removed from it — navigation builds a fresh collection out of
// what each object returns, and appending to that could never reach the cache
// whether or not it were capped.
func TestCachedCollectionCannotBeAppendedInto(t *testing.T) {
	doc := MustNewDocument(documentPatient)

	root, ok := doc.root[0].(*types.ObjectValue)
	if !ok {
		t.Fatalf("expected the document root to be an object, got %T", doc.root[0])
	}

	// A null with no element beside it is a position that yields nothing, so
	// the collection built for this field has room to spare — which is what
	// makes an append to it land in shared memory rather than in a copy.
	given := root.GetCollection("given")
	if len(given) != 1 {
		t.Fatalf("got %d given names, want 1", len(given))
	}

	if cap(given) != len(given) {
		t.Errorf("cached collection came back with cap %d and len %d: appending to it writes into the cache",
			cap(given), len(given))
	}

	// Two callers appending to the same result must not write over each other.
	//nolint:gocritic // appending a shared result into a slice of one's own is the case under test
	first := append(given, types.NewString("first"))
	//nolint:gocritic // as above
	second := append(given, types.NewString("second"))

	if first[len(given)].String() != "first" {
		t.Errorf("after a second caller appended, the first caller's result reads %q, want %q",
			first[len(given)].String(), "first")
	}
	if second[len(given)].String() != "second" {
		t.Errorf("the second caller's result reads %q, want %q",
			second[len(given)].String(), "second")
	}
}

// A field read under a type hint is cached too, which is the path navigation
// takes when a model is configured — and the path a validator uses.
func TestDocumentCachesTypedReads(t *testing.T) {
	doc := MustNewDocument(documentPatient)

	root, ok := doc.root[0].(*types.ObjectValue)
	if !ok {
		t.Fatalf("expected the document root to be an object, got %T", doc.root[0])
	}

	// Reading objects, so that the second read can be shown to be the first
	// one's result rather than an equal one built again.
	firstRead := root.GetCollectionWithType("name", "HumanName")
	secondRead := root.GetCollectionWithType("name", "HumanName")

	if len(firstRead) != 2 || len(secondRead) != 2 {
		t.Fatalf("got %d and %d names, want 2 each", len(firstRead), len(secondRead))
	}
	if firstRead[0] != secondRead[0] {
		t.Error("the second typed read built the field again instead of answering from the cache")
	}

	first := root.GetCollectionWithType("birthDate", "date")
	if len(first) != 1 {
		t.Fatalf("got %d results for birthDate, want 1", len(first))
	}

	// A different type hint is a different reading of the same field, and must
	// not be answered from the first one's entry.
	asString := root.GetCollectionWithType("birthDate", "string")
	if len(asString) != 1 {
		t.Fatalf("got %d results for the string reading, want 1", len(asString))
	}
	if asString[0].Type() == first[0].Type() {
		t.Errorf("reading birthDate as a string gave type %q, the same as reading it as a date",
			asString[0].Type())
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
