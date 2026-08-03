package conformance

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
// The suite runs twice: once as a caller with no FHIR model, and once with the
// R4 model supplied. Several cases can only be decided with a model — a
// semantic error such as Encounter.name.given is not detectable from the
// document alone — so the two runs keep separate baselines and show what the
// model is worth.
//
// Cases that do not pass yet are listed in testdata/fhirpath-suite/known-failures.txt
// so that this test guards against regressions without failing the build for
// pre-existing gaps. The test fails when a case that used to pass breaks, and
// also when a listed case starts passing — that keeps the list honest and forces
// it to shrink as gaps get closed. Regenerate it with:
//
//	go test -run TestOfficialSuite -update-known-failures

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
	"github.com/gofhir/models/r4"
	"github.com/gofhir/models/r5"
)

var updateKnownFailures = flag.Bool("update-known-failures", false,
	"rewrite testdata/fhirpath-suite/known-failures.txt from this run")

const (
	suiteDir               = "testdata/fhirpath-suite"
	suiteFile              = suiteDir + "/tests-fhir-r4.xml"
	suiteInputDir          = suiteDir + "/input"
	knownFailuresFile      = suiteDir + "/known-failures.txt"
	knownFailuresModelFile = suiteDir + "/known-failures-model.txt"

	suiteDirR5               = "testdata/fhirpath-suite-r5"
	suiteFileR5              = suiteDirR5 + "/tests-fhir-r5.xml"
	suiteInputDirR5          = suiteDirR5 + "/input"
	knownFailuresFileR5      = suiteDirR5 + "/known-failures.txt"
	knownFailuresModelFileR5 = suiteDirR5 + "/known-failures-model.txt"
	knownFailuresComment     = `# Cases from the official FHIRPath suite that this engine does not pass yet.
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

// expectsError reports whether the case asserts that the expression must fail.
func (c suiteCase) expectsError() bool {
	return c.Expression.Invalid != "" && c.Expression.Invalid != "false"
}

// runsLeniently reports whether the case asks to be evaluated without the
// stricter reading.
//
// The suite separates semantic faults from execution ones, and the distinction
// is real: Patient.name.given1 evaluates to empty quite correctly against a
// document that has no such value, and is still wrong, because given1 is not an
// element of HumanName in any document. Only static analysis against a model
// can say so, which is why the analysis runs for every case except these.
//
// The suite marks these with a mode: Observation.valueQuantity.exists() appears
// twice, once as a semantic error and once under mode="lenient/polymorphics"
// expecting an answer. Same expression, two modes, two correct results — which
// is what makes the strict reading a mode this engine offers rather than the way
// it behaves by default.
func (c suiteCase) runsLeniently() bool {
	return strings.HasPrefix(c.Mode, "lenient")
}

// variant is one way of calling the engine, measured against its own baseline.
type variant struct {
	name         string
	baselineFile string
	evaluate     func(expr *fhirpath.Expression, resource []byte) (types.Collection, error)

	// model is what static analysis needs; a variant without one only evaluates
	model fhirpath.Model
}

// resourceTypeOf reads the type a resource declares, which is the context an
// expression is analyzed against.
func resourceTypeOf(resource []byte) string {
	var envelope struct {
		ResourceType string `json:"resourceType"`
	}
	if err := json.Unmarshal(resource, &envelope); err != nil {
		return ""
	}
	return envelope.ResourceType
}

// corpus is one published suite: the cases, the resources they run against, and
// a baseline per variant.
//
// There are two, and they are not the same measurement. The R4 suite is what
// the other engines report against and what most deployed data conforms to; the
// R5 one is larger, covers functions R4 never exercised — defineVariable among
// them — and evaluates under the rules that changed with R5.
type corpus struct {
	name     string
	file     string
	inputDir string
	variants []variant
}

func corpora() []corpus {
	return []corpus{
		{
			name:     "r4",
			file:     suiteFile,
			inputDir: suiteInputDir,
			variants: []variant{
				{
					name:         "without model",
					baselineFile: knownFailuresFile,
					evaluate: func(expr *fhirpath.Expression, resource []byte) (types.Collection, error) {
						return expr.Evaluate(resource)
					},
				},
				{
					name:         "with r4 model",
					baselineFile: knownFailuresModelFile,
					model:        r4.FHIRPathModel(),
					evaluate: func(expr *fhirpath.Expression, resource []byte) (types.Collection, error) {
						return expr.EvaluateWithOptions(resource, fhirpath.WithModel(r4.FHIRPathModel()))
					},
				},
			},
		},
		{
			name:     "r5",
			file:     suiteFileR5,
			inputDir: suiteInputDirR5,
			variants: []variant{
				{
					name:         "without model",
					baselineFile: knownFailuresFileR5,
					evaluate: func(expr *fhirpath.Expression, resource []byte) (types.Collection, error) {
						return expr.Evaluate(resource)
					},
				},
				{
					name:         "with r5 model",
					baselineFile: knownFailuresModelFileR5,
					model:        r5.FHIRPathModel(),
					evaluate: func(expr *fhirpath.Expression, resource []byte) (types.Collection, error) {
						return expr.EvaluateWithOptions(resource, fhirpath.WithModel(r5.FHIRPathModel()))
					},
				},
			},
		},
	}
}

func TestOfficialSuite(t *testing.T) {
	for _, c := range corpora() {
		for _, v := range c.variants {
			t.Run(c.name+"/"+v.name, func(t *testing.T) {
				runSuite(t, c, v)
			})
		}
	}
}

func runSuite(t *testing.T, c corpus, v variant) {
	data, err := os.ReadFile(c.file)
	if err != nil {
		t.Fatalf("read suite: %v", err)
	}

	var suite suiteFileXML
	if err := xml.Unmarshal(data, &suite); err != nil {
		t.Fatalf("parse suite: %v", err)
	}

	inputs := newInputLoader(t, c.inputDir)
	known := loadKnownFailures(t, v.baselineFile)

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
			if err := runSuiteCase(tc, resource, v); err != nil {
				failures = append(failures, id)
				if !known[id] {
					t.Errorf("%s: %v\n  expression: %s", id, err, strings.TrimSpace(tc.Expression.Text))
				}
				continue
			}
			passed++
			if known[id] {
				t.Errorf("%s now passes — remove it from %s", id, v.baselineFile)
			}
		}
	}

	sort.Strings(failures)
	if *updateKnownFailures {
		writeKnownFailures(t, v.baselineFile, failures)
	}

	// Coverage is reported, never silently reduced: a case the harness could not
	// run is as important as one that failed.
	t.Logf("official suite %s (%s): %d/%d executed cases pass (%.1f%%), %d known failures",
		c.name, v.name, passed, executed, 100*float64(passed)/float64(executed), len(failures))
	for file, n := range skippedByInput {
		t.Logf("skipped %d case(s): no JSON available for input %q", n, file)
	}
}

// runSuiteCase evaluates one case and reports why it did not conform, or nil.
func runSuiteCase(tc suiteCase, resource []byte, v variant) error {
	expr, compileErr := fhirpath.Compile(strings.TrimSpace(tc.Expression.Text))

	// A semantic fault is found before evaluation, and only with a model. The
	// analysis runs where the case asks for the stricter reading.
	var analysisErr error
	if compileErr == nil && v.model != nil && !tc.runsLeniently() {
		analysisErr = expr.Analyze(v.model, resourceTypeOf(resource))
	}

	var (
		result  types.Collection
		evalErr error
	)
	if compileErr == nil && analysisErr == nil {
		result, evalErr = v.evaluate(expr, resource)
	}

	if tc.expectsError() {
		if compileErr == nil && analysisErr == nil && evalErr == nil {
			return fmt.Errorf("expected an error (invalid=%q), got %v", tc.Expression.Invalid, result)
		}
		return nil
	}

	if analysisErr != nil {
		return fmt.Errorf("analyze: %w", analysisErr)
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
func compareOutputs(tc suiteCase, result types.Collection) error {
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
//
// Many cases declare no output type, so the notation is detected from the value
// itself rather than trusted from the type attribute.
func expectedValue(want suiteOutput) string {
	value := strings.TrimSpace(want.Value)
	switch want.Type {
	case "date", "dateTime", "time":
		return trimTemporalNotation(value)
	case "":
		if temporalLiteral.MatchString(value) {
			return trimTemporalNotation(value)
		}
	}
	return value
}

// trimTemporalNotation strips the literal markers from a temporal value. A time
// carries two — "@T10:30" — of which only the digits are the value.
func trimTemporalNotation(value string) string {
	return strings.TrimPrefix(strings.TrimPrefix(value, "@"), "T")
}

// temporalLiteral matches the FHIRPath literal notation for a date, dateTime or
// time, so that "@2014-01" is recognized while a string that merely starts with
// an @ is left alone.
var temporalLiteral = regexp.MustCompile(`^@(T\d{2}|\d{4})`)

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
	dir   string
	cache map[string][]byte
}

func newInputLoader(t *testing.T, dir string) *inputLoader {
	return &inputLoader{t: t, dir: dir, cache: map[string][]byte{}}
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
	data, err := os.ReadFile(filepath.Join(l.dir, base+".json"))
	if err != nil {
		l.cache[name] = nil
		return nil, false
	}
	l.cache[name] = data
	return data, true
}

func loadKnownFailures(t *testing.T, path string) map[string]bool {
	known := map[string]bool{}
	data, err := os.ReadFile(path)
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

func writeKnownFailures(t *testing.T, path string, failures []string) {
	content := knownFailuresComment
	if len(failures) > 0 {
		content += strings.Join(failures, "\n") + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write known failures: %v", err)
	}
	t.Logf("wrote %d known failure(s) to %s", len(failures), path)
}
