// Command difftest reports where this engine and fhirpath.js answer the
// official suite's expressions differently.
//
// The suite says what each expression should answer, and the conformance
// harness already measures that. This asks a different question: where two
// independent engines disagree, and where they agree with each other but not
// with the suite. The second kind is what settled three decisions recorded in
// CONFORMANCE.md, each of which was run through fhirpath.js by hand.
//
// Usage:
//
//	go run . [-corpus r4|r5|both] [-model] [-v]
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
	"github.com/gofhir/models/r4"
	"github.com/gofhir/models/r5"
)

func main() {
	corpusName := flag.String("corpus", "both", "which suite to run: r4, r5 or both")
	withModel := flag.Bool("model", true, "supply each engine its FHIR model")
	verbose := flag.Bool("v", false, "list every divergence rather than a summary")
	flag.Parse()

	if err := run(*corpusName, *withModel, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "difftest:", err)
		os.Exit(1)
	}
}

func run(corpusName string, withModel, verbose bool) error {
	for _, c := range corpora() {
		if corpusName != "both" && corpusName != c.name {
			continue
		}

		cases, err := c.load(withModel)
		if err != nil {
			return err
		}

		theirs, err := evaluateWithFHIRPathJS(cases)
		if err != nil {
			return err
		}

		report(c.name, cases, theirs, verbose)
	}
	return nil
}

// A testCase is one expression, the resource it runs against, and what the
// suite says it should answer.
type testCase struct {
	ID         string          `json:"id"`
	Expression string          `json:"expression"`
	Resource   json.RawMessage `json:"resource"`
	Model      string          `json:"model,omitempty"`

	// What the suite says the case should answer: an error, or these values —
	// of which none is a real answer, meaning empty.
	outputs     []string
	outputTypes []string
	expectErr   bool
	predicate   bool
	skip        string
}

// An answer is what one engine said: results, or the error it raised.
type answer struct {
	ID      string   `json:"id"`
	Results []string `json:"results"`
	Error   string   `json:"error"`
}

func (a answer) failed() bool { return a.Error != "" }

func (a answer) text() string {
	if a.failed() {
		return "error: " + firstLine(a.Error)
	}
	return "[" + strings.Join(a.Results, ", ") + "]"
}

// evaluateWithFHIRPathJS hands the whole batch to node in one call. Starting a
// process per case would dominate the running time and say nothing extra.
func evaluateWithFHIRPathJS(cases []testCase) (map[string]answer, error) {
	batch, err := json.Marshal(cases)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("node", "evaluate.js")
	cmd.Stdin = bytes.NewReader(batch)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running fhirpath.js: %w (has 'npm install' been run in difftest/?)", err)
	}

	var answers []answer
	if err := json.Unmarshal(out, &answers); err != nil {
		return nil, fmt.Errorf("reading what fhirpath.js answered: %w", err)
	}

	byID := make(map[string]answer, len(answers))
	for _, a := range answers {
		byID[a.ID] = a
	}
	return byID, nil
}

// evaluateHere answers a case with this engine.
func evaluateHere(c testCase) answer {
	var (
		result types.Collection
		err    error
	)

	expr, compileErr := fhirpath.Compile(c.Expression)
	if compileErr != nil {
		return answer{ID: c.ID, Error: compileErr.Error()}
	}

	switch c.Model {
	case "r4":
		result, err = expr.EvaluateWithOptions(c.Resource, fhirpath.WithModel(r4.FHIRPathModel()))
	case "r5":
		result, err = expr.EvaluateWithOptions(c.Resource, fhirpath.WithModel(r5.FHIRPathModel()))
	default:
		result, err = expr.Evaluate(c.Resource)
	}
	if err != nil {
		return answer{ID: c.ID, Error: err.Error()}
	}

	results := make([]string, 0, len(result))
	for _, value := range result {
		results = append(results, value.String())
	}
	return answer{ID: c.ID, Results: results}
}

// report prints how the two engines line up, and against what the suite says.
//
// A divergence is only actionable once you know which side the suite is on, so
// they are grouped by that: what this engine gets wrong is the first thing to
// read, and what fhirpath.js gets wrong is worth knowing before quoting it as
// evidence.
func report(corpus string, cases []testCase, theirs map[string]answer, verbose bool) {
	var (
		agree          int
		bothFail       int
		skipped        int
		unsupported    int
		oursWrong      []string
		theirsWrong    []string
		neitherRight   []string
		noVerdict      []string
		agreedButWrong []string
	)

	for _, c := range cases {
		if c.skip != "" {
			skipped++
			continue
		}

		ours := evaluateHere(c)
		their := theirs[c.ID]

		// fhirpath.js refusing because it has no such function says nothing
		// about the expression, and crediting that refusal as satisfying an
		// "invalid" case would credit it for the wrong reason.
		if notImplemented(their) {
			unsupported++
			continue
		}

		if sameAnswer(ours, their) {
			agree++
			if ours.failed() && their.failed() {
				bothFail++
			}
			// Both engines answering alike but unlike the suite is worth
			// reading on its own: two independent readings against one
			// expected value.
			if c.hasVerdict() && !matchesExpected(c, ours) {
				agreedButWrong = append(agreedButWrong, line(c, "both", ours.text(), "suite", c.verdict()))
			}
			continue
		}

		switch {
		case !c.hasVerdict():
			noVerdict = append(noVerdict, line(c, "here", ours.text(), "fhirpath.js", their.text()))
		case matchesExpected(c, ours):
			theirsWrong = append(theirsWrong, line(c, "here", ours.text(), "fhirpath.js", their.text()))
		case matchesExpected(c, their):
			oursWrong = append(oursWrong, line(c, "here", ours.text(), "fhirpath.js", their.text()))
		default:
			neitherRight = append(neitherRight, line(c, "here", ours.text(), "fhirpath.js", their.text()))
		}
	}

	total := len(cases) - skipped - unsupported
	if total <= 0 {
		fmt.Printf("\n%s: nothing to compare\n", corpus)
		return
	}
	divergent := len(oursWrong) + len(theirsWrong) + len(neitherRight) + len(noVerdict)

	fmt.Printf("\n%s: %d cases, %d agree (%.1f%%), %d diverge",
		corpus, total, agree, 100*float64(agree)/float64(total), divergent)
	if bothFail > 0 {
		fmt.Printf(" — of those agreeing, %d are both refusing", bothFail)
	}
	if skipped > 0 {
		fmt.Printf("; %d not comparable here", skipped)
	}
	if unsupported > 0 {
		fmt.Printf("; %d fhirpath.js does not implement", unsupported)
	}
	fmt.Println()

	section(corpus, "this engine differs and the suite is against it", oursWrong, verbose)
	section(corpus, "both engines agree, and neither matches the suite", agreedButWrong, verbose)
	section(corpus, "fhirpath.js differs and the suite is against it", theirsWrong, verbose)
	section(corpus, "the two differ and the suite backs neither", neitherRight, verbose)
	section(corpus, "the two differ, and the suite states no result to judge by", noVerdict, verbose)
}

func section(corpus, title string, lines []string, verbose bool) {
	if len(lines) == 0 {
		return
	}
	fmt.Printf("\n%s: %s (%d)\n", corpus, title, len(lines))
	print(lines, verbose)
}

// line renders one case for the report, cutting answers that would otherwise
// run to pages — a children() over a whole Patient among them.
func line(c testCase, leftLabel, left, rightLabel, right string) string {
	return fmt.Sprintf("  %-46s %s\n      %s: %-30s %s: %s",
		c.ID, cut(c.Expression, 90), leftLabel, cut(left, 60), rightLabel, cut(right, 60))
}

func cut(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func print(lines []string, verbose bool) {
	sort.Strings(lines)
	shown := lines
	if !verbose && len(lines) > 12 {
		shown = lines[:12]
	}
	for _, line := range shown {
		fmt.Println(line)
	}
	if len(shown) < len(lines) {
		fmt.Printf("  … %d more; pass -v for all\n", len(lines)-len(shown))
	}
}

// sameAnswer compares two engines' answers. A refusal matches a refusal
// whatever each said about it: the engines word their errors differently, and
// what is being compared is whether the expression has an answer.
func sameAnswer(ours, theirs answer) bool {
	if ours.failed() || theirs.failed() {
		return ours.failed() && theirs.failed()
	}
	if len(ours.Results) != len(theirs.Results) {
		return false
	}
	for i := range ours.Results {
		if !sameValue(ours.Results[i], theirs.Results[i]) {
			return false
		}
	}
	return true
}

// sameValue compares two rendered values, allowing for the ways two engines
// write the same thing: a temporal with or without its @, a decimal with or
// without trailing zeros.
func sameValue(ours, theirs string) bool {
	ours, theirs = normalize(ours), normalize(theirs)
	if ours == theirs {
		return true
	}
	return trimZeros(ours) == trimZeros(theirs)
}

// normalize strips the syntax a literal is written with but a value does not
// carry: the @ of a temporal, and the T of a time.
func normalize(s string) string {
	s = strings.TrimPrefix(s, "@")
	if len(s) > 1 && s[0] == 'T' && s[1] >= '0' && s[1] <= '9' {
		s = s[1:]
	}
	return s
}

func trimZeros(s string) string {
	if !strings.Contains(s, ".") || strings.ContainsAny(s, "TZ:") {
		return s
	}
	return strings.TrimRight(strings.TrimRight(s, "0"), ".")
}

// hasVerdict reports whether the suite states what the case should answer.
// Some cases only say the expression is invalid, and some state nothing.
func (c testCase) hasVerdict() bool {
	// Every case says something: an error, some values, or no values at all,
	// which is the suite's way of writing empty.
	return true
}

func (c testCase) verdict() string {
	switch {
	case c.expectErr:
		return "an error"
	case len(c.outputs) == 0:
		return "[]"
	}
	return "[" + strings.Join(c.outputs, ", ") + "]"
}

func matchesExpected(c testCase, a answer) bool {
	if c.expectErr {
		return a.failed()
	}
	if a.failed() {
		return false
	}

	if c.predicate && len(c.outputs) == 1 {
		found := len(a.Results) > 0
		return found == (c.outputs[0] == "true")
	}

	if len(a.Results) != len(c.outputs) {
		return false
	}
	for i := range a.Results {
		if !sameAs(a.Results[i], c.outputs[i], c.outputType(i)) {
			return false
		}
	}
	return true
}

func (c testCase) outputType(i int) string {
	if i < len(c.outputTypes) {
		return c.outputTypes[i]
	}
	return ""
}

// sameAs compares an answer against what the suite states, normalizing only
// where the type says it is safe to. A string result is compared as written:
// '2.10' and '2.1' are different strings, whatever they would be as numbers.
func sameAs(got, want, outputType string) bool {
	if outputType == "string" || outputType == "code" {
		return got == want
	}
	return sameValue(got, want)
}

// notImplemented reports whether fhirpath.js declined for want of the function
// rather than because of the expression.
func notImplemented(a answer) bool {
	return a.failed() && strings.Contains(a.Error, "Not implemented")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
