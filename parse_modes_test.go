package fhirpath

import (
	"html"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/antlr4-go/antlr/v4"

	"github.com/gofhir/fhirpath/parser/grammar"
)

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

var expressionElement = regexp.MustCompile(`(?s)<expression[^>]*>(.*?)</expression>`)

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
