package fhirpath

import (
	"html"
	"os"
	"path/filepath"
	"testing"

	"github.com/antlr4-go/antlr/v4"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/funcs"
	"github.com/gofhir/fhirpath/parser/grammar"
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

// Compiling parses with SLL prediction first and falls back to full LL
// prediction on a syntax error. That gives an expression the tree LL gives
// it only if SLL never parses an expression to another tree, which an
// ambiguity in the grammar could do. These hold SLL to LL over every
// expression of the official suites and the spec cases, and over what the
// fuzzer makes of them.

var parserRuleNames = grammar.NewfhirpathParser(nil).GetRuleNames()

// treeIn parses an expression with a prediction mode alone, writing its tree
// as text, or reporting that it does not parse.
func treeIn(expr string, mode int) (string, bool) {
	tree, err := parse(expr, mode)
	if err != nil {
		return "", false
	}
	return antlr.TreesStringTree(tree, parserRuleNames, nil), true
}

func checkSLLGivesLLTree(t *testing.T, expr string) {
	t.Helper()
	// SLL first, as compiling parses, over what the DFA holds so far
	sll, sllOK := treeIn(expr, antlr.PredictionModeSLL)
	ll, llOK := treeIn(expr, antlr.PredictionModeLL)
	switch {
	case sllOK && !llOK:
		t.Errorf("%q parses with SLL prediction alone", expr)
	case sllOK && sll != ll:
		t.Errorf("%q parses to another tree with SLL prediction:\nLL:  %s\nSLL: %s", expr, ll, sll)
	}
}

func TestSLLPredictionGivesLLTree(t *testing.T) {
	files, err := filepath.Glob("conformance/testdata/*/*.xml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no test files: %v", err)
	}
	count := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range expressionElement.FindAllSubmatch(data, -1) {
			checkSLLGivesLLTree(t, html.UnescapeString(string(m[1])))
			count++
		}
	}
	if count < 3000 {
		t.Errorf("read %d expressions, want the suites' and spec cases' several thousand", count)
	}
}

func FuzzSLLPredictionGivesLLTree(f *testing.F) {
	for _, seed := range []string{
		"Patient.name.where(use = 'official').given.first()",
		"(1 | 2 | 3).aggregate($this + $total, 0)",
		"a.b is FHIR.string and c as Quantity > 5 'mg'",
		"%`vs-x`.contains(@2020-01-01T10:00:00.000+02:00) implies -1.5 div 2 mod 3",
		"iif(x.exists(), {}, `div`.as(System.Integer)) xor not()",
		"Observation.value.ofType(Quantity).where(unit ~ 'mg') | Observation.component[0]",
		"a is B.c.d[0] | x.where(y as FHIR.string.exists())",
		"a /*c*/ . b // x",
		"-5 days - 1 day | 5 'mg'.value | in in in",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, expr string) {
		checkSLLGivesLLTree(t, expr)
	})
}

// The parse tree is still walked where a node does not compile, so the
// interpreted and, or and implies decide from the left operand as the compiled
// ones do.
func TestInterpretedBooleanOperatorsDoNotEvaluateARightOperandTheLeftDecides(t *testing.T) {
	element := []byte(`{"type":[{"code":"boolean"},{"code":"dateTime"}]}`)

	for _, tt := range []struct {
		expr    string
		want    string
		wantErr bool
	}{
		{"true or type.code.contains(':')", "[true]", false},
		{"false and type.code.contains(':')", "[false]", false},
		{"false implies type.code.contains(':')", "[true]", false},
		{"{} or true", "[true]", false},
		{"{} or type.code.contains(':')", "", true},
		{"true and type.code.contains(':')", "", true},
		{"true xor type.code.contains(':')", "", true},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			expr := MustCompile(tt.expr)
			result, err := eval.NewEvaluator(eval.NewContext(element), funcs.GetRegistry()).Evaluate(expr.tree)
			if tt.wantErr {
				if err == nil {
					t.Errorf("%s = %s, want the right operand's error", tt.expr, result)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: %v", tt.expr, err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}
		})
	}
}
