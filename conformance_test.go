package fhirpath

// Conformance harness for the official HL7 FHIRPath test suite.
//
// The suite is the one maintained at
// https://github.com/FHIR/fhir-test-cases (r4/fhirpath/tests-fhir-r4.xml) and
// used by the other reference engines. It is vendored under
// testdata/fhirpath-suite together with its input resources; the commit it was
// taken from is recorded in testdata/fhirpath-suite/SUITE_COMMIT.
//
// The harness measures results, not just that expressions compile: each case
// carries the expected output, and 36 cases assert that evaluation must fail.
//
// Cases that do not pass yet are listed in testdata/fhirpath-suite/known-failures.txt
// so that this test guards against regressions without failing the build for
// pre-existing gaps. The test fails when a case that used to pass breaks, and
// also when a listed case starts passing — that keeps the list honest and forces
// it to shrink as gaps get closed. Regenerate it with:
//
//	go test -run TestOfficialSuite -update-known-failures

import (
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var updateKnownFailures = flag.Bool("update-known-failures", false,
	"rewrite testdata/fhirpath-suite/known-failures.txt from this run")

const (
	suiteDir             = "testdata/fhirpath-suite"
	suiteFile            = suiteDir + "/tests-fhir-r4.xml"
	suiteInputDir        = suiteDir + "/input"
	knownFailuresFile    = suiteDir + "/known-failures.txt"
	knownFailuresComment = `# Cases from the official FHIRPath suite that this engine does not pass yet.
# One "group/test" per line. Maintained by TestOfficialSuite; regenerate with
#   go test -run TestOfficialSuite -update-known-failures
# A line removed from here must stay passing; a new failure fails the build.
`
)

type suiteFileXML struct {
	XMLName xml.Name     `xml:"tests"`
	Groups  []suiteGroup `xml:"group"`
}

type suiteGroup struct {
	Name  string      `xml:"name,attr"`
	Tests []suiteCase `xml:"test"`
}

type suiteCase struct {
	Name       string          `xml:"name,attr"`
	InputFile  string          `xml:"inputfile,attr"`
	Predicate  string          `xml:"predicate,attr"`
	Mode       string          `xml:"mode,attr"`
	Expression suiteExpression `xml:"expression"`
	Outputs    []suiteOutput   `xml:"output"`
}

type suiteExpression struct {
	Invalid string `xml:"invalid,attr"`
	Text    string `xml:",chardata"`
}

type suiteOutput struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

// expectsError reports whether the case asserts that evaluation must fail.
// The suite distinguishes syntax, semantic and execution failures; this engine
// does not separate those, so any error satisfies the expectation.
func (c suiteCase) expectsError() bool {
	return c.Expression.Invalid != "" && c.Expression.Invalid != "false"
}

func TestOfficialSuite(t *testing.T) {
	data, err := os.ReadFile(suiteFile)
	if err != nil {
		t.Fatalf("read suite: %v", err)
	}

	var suite suiteFileXML
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("parse suite: %v", err)
	}

	inputs := newInputLoader(t)
	known := loadKnownFailures(t)

	var (
		failures []string
		passed   int
		executed int
	)
	skippedByInput := map[string]int{}

	for _, group := range suite.Groups {
		for _, tc := range group.Tests {
			id := group.Name + "/" + tc.Name

			resource, ok := inputs.load(tc.InputFile)
			if !ok {
				skippedByInput[tc.InputFile]++
				continue
			}

			executed++
			if err := runSuiteCase(tc, resource); err != nil {
				failures = append(failures, id)
				if !known[id] {
					t.Errorf("%s: %v\n  expression: %s", id, err, strings.TrimSpace(tc.Expression.Text))
				}
				continue
			}
			passed++
			if known[id] {
				t.Errorf("%s now passes — remove it from %s", id, knownFailuresFile)
			}
		}
	}

	sort.Strings(failures)
	if *updateKnownFailures {
		writeKnownFailures(t, failures)
	}

	// Coverage is reported, never silently reduced: a case the harness could not
	// run is as important as one that failed.
	t.Logf("official suite: %d/%d executed cases pass (%.1f%%), %d known failures",
		passed, executed, 100*float64(passed)/float64(executed), len(failures))
	for file, n := range skippedByInput {
		t.Logf("skipped %d case(s): no JSON available for input %q", n, file)
	}
}

// runSuiteCase evaluates one case and reports why it did not conform, or nil.
func runSuiteCase(tc suiteCase, resource []byte) error {
	expr, compileErr := Compile(strings.TrimSpace(tc.Expression.Text))

	var (
		result  Collection
		evalErr error
	)
	if compileErr == nil {
		result, evalErr = expr.Evaluate(resource)
	}

	if tc.expectsError() {
		if compileErr == nil && evalErr == nil {
			return fmt.Errorf("expected an error (invalid=%q), got %v", tc.Expression.Invalid, result)
		}
		return nil
	}

	if compileErr != nil {
		return fmt.Errorf("compile: %w", compileErr)
	}
	if evalErr != nil {
		return fmt.Errorf("evaluate: %w", evalErr)
	}

	return compareOutputs(tc, result)
}

// compareOutputs checks the result against the expected outputs. Values are
// compared by their string form, which is what the suite records.
func compareOutputs(tc suiteCase, result Collection) error {
	// predicate="true" means the expression is evaluated for its truth value
	if tc.Predicate == "true" && len(tc.Outputs) == 1 && tc.Outputs[0].Type == "boolean" {
		got := "false"
		if val, ok := result.SingletonBoolean(); ok && val {
			got = "true"
		}
		if got != strings.TrimSpace(tc.Outputs[0].Value) {
			return fmt.Errorf("predicate: expected %s, got %s", tc.Outputs[0].Value, got)
		}
		return nil
	}

	if len(result) != len(tc.Outputs) {
		return fmt.Errorf("expected %d value(s) %v, got %d %v",
			len(tc.Outputs), outputValues(tc.Outputs), len(result), result)
	}

	for i, want := range tc.Outputs {
		got := result[i].String()
		if got != expectedValue(want) {
			return fmt.Errorf("value %d: expected %q (%s), got %q (%s)",
				i, expectedValue(want), want.Type, got, result[i].Type())
		}
	}
	return nil
}

// expectedValue normalizes a recorded output for comparison. The suite writes
// temporal values in FHIRPath literal notation ("@1974-12-25"), while a value's
// string form is the plain lexical value — the leading @ is notation, not data.
func expectedValue(want suiteOutput) string {
	value := strings.TrimSpace(want.Value)
	switch want.Type {
	case "date", "dateTime", "time":
		return strings.TrimPrefix(value, "@")
	}
	return value
}

func outputValues(outputs []suiteOutput) []string {
	values := make([]string, len(outputs))
	for i, o := range outputs {
		values[i] = strings.TrimSpace(o.Value)
	}
	return values
}

// inputLoader resolves and caches the suite's input resources. The suite names
// several inputs as .xml; this engine consumes JSON, so the vendored directory
// holds the published JSON equivalent under the same base name.
type inputLoader struct {
	t     *testing.T
	cache map[string][]byte
}

func newInputLoader(t *testing.T) *inputLoader {
	return &inputLoader{t: t, cache: map[string][]byte{}}
}

func (l *inputLoader) load(name string) ([]byte, bool) {
	// Cases without an input file evaluate expressions that need no resource
	if name == "" {
		return []byte(`{}`), true
	}

	if cached, ok := l.cache[name]; ok {
		return cached, cached != nil
	}

	base := strings.TrimSuffix(strings.TrimSuffix(name, ".xml"), ".json")
	data, err := os.ReadFile(filepath.Join(suiteInputDir, base+".json"))
	if err != nil {
		l.cache[name] = nil
		return nil, false
	}
	l.cache[name] = data
	return data, true
}

func loadKnownFailures(t *testing.T) map[string]bool {
	known := map[string]bool{}
	data, err := os.ReadFile(knownFailuresFile)
	if err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("read known failures: %v", err)
		}
		return known
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		known[line] = true
	}
	return known
}

func writeKnownFailures(t *testing.T, failures []string) {
	content := knownFailuresComment
	if len(failures) > 0 {
		content += strings.Join(failures, "\n") + "\n"
	}
	if err := os.WriteFile(knownFailuresFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write known failures: %v", err)
	}
	t.Logf("wrote %d known failure(s) to %s", len(failures), knownFailuresFile)
}
