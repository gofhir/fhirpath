package conformance

// Spec cases are this project's own additions to the official suite, written
// from the specification's text rather than from either engine. They live in
// testdata/spec-cases/<area>.xml, in the suite's own format so that the same
// runner measures them and difftest can put them to fhirpath.js unchanged, with
// two additions the official suite does not carry:
//
//   - each group is one requirement, named REQ-<AREA>-NNN, and
//   - each requirement cites the sentence it comes from: a <source> naming one
//     of the texts pinned in SPEC_SOURCES, the section, how strongly the text
//     states it, and the sentence verbatim.
//
// TestSpecCases runs them with the R4 and the R5 model, each against its own
// baseline, exactly as TestOfficialSuite runs the suite. TestSpecCaseCatalog
// checks the shape of every file and that every quote is in the text it cites.
//
// The texts are not vendored. Without them the quote check is skipped, unless
// SPEC_SOURCES_REQUIRED is set, as CI does: `make spec-sources` fetches them.

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
	"github.com/gofhir/models/r4"
	"github.com/gofhir/models/r5"
)

const (
	specCasesDir    = "testdata/spec-cases"
	specSourcesFile = specCasesDir + "/SPEC_SOURCES"
	specTextsDir    = specCasesDir + "/.spec"

	specKnownFailuresComment = `# Spec cases this engine does not pass yet; see testdata/spec-cases/README.md.
# One "group/test" per line. Maintained by TestSpecCases; regenerate with
#   go test -run TestSpecCases -update-known-failures
# Every line needs a reason in the area's known-failure-reasons.txt.
`
)

// How strongly the cited text states a requirement. EXAMPLE marks a case
// taken from one of the specification's own worked examples.
var specStrengths = map[string]bool{
	"SHALL": true, "SHALL NOT": true, "SHOULD": true, "SHOULD NOT": true,
	"MAY": true, "DEFINITION": true, "EXAMPLE": true,
}

type specFileXML struct {
	XMLName     xml.Name       `xml:"tests"`
	Name        string         `xml:"name,attr"`
	Description string         `xml:"description,attr"`
	Groups      []specGroupXML `xml:"group"`
}

type specGroupXML struct {
	Name        string          `xml:"name,attr"`
	Description string          `xml:"description,attr"`
	FHIR        string          `xml:"fhir,attr"`
	Sources     []specSourceXML `xml:"source"`
	Tests       []suiteCase     `xml:"test"`
}

type specSourceXML struct {
	Spec     string `xml:"spec,attr"`
	Section  string `xml:"section,attr"`
	Strength string `xml:"strength,attr"`
	Quote    string `xml:",chardata"`
}

// specAreas lists the areas that have spec cases, by file name.
func specAreas(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(specCasesDir, "*.xml"))
	if err != nil {
		t.Fatalf("list spec cases: %v", err)
	}
	areas := make([]string, 0, len(files))
	for _, f := range files {
		areas = append(areas, strings.TrimSuffix(filepath.Base(f), ".xml"))
	}
	return areas
}

func specBaselineFile(area, fhir string) string {
	return filepath.Join(specCasesDir, area+".known-failures-"+fhir+".txt")
}

func specReasonsFile(area string) string {
	return filepath.Join(specCasesDir, area+".known-failure-reasons.txt")
}

// specCorpora is one area run as the official suites are, once per FHIR
// version, always with that version's model: a spec case is about the
// language, and what the model decides is measured by the suites already.
func specCorpora(area string) []corpus {
	file := filepath.Join(specCasesDir, area+".xml")
	return []corpus{
		{
			name:     area + "/r4",
			title:    "spec cases " + area + " r4",
			fhir:     "r4",
			file:     file,
			inputDir: suiteInputDir,
			readXML:  fhirXMLToJSON(r4.UnmarshalResourceXML),
			variants: []variant{{
				name:           "with r4 model",
				baselineFile:   specBaselineFile(area, "r4"),
				baselineHeader: specKnownFailuresComment,
				model:          r4.FHIRPathModel(),
				evaluate: func(expr *fhirpath.Expression, resource []byte) (types.Collection, error) {
					return expr.EvaluateWithOptions(resource, fhirpath.WithModel(r4.FHIRPathModel()))
				},
			}},
		},
		{
			name:     area + "/r5",
			title:    "spec cases " + area + " r5",
			fhir:     "r5",
			file:     file,
			inputDir: suiteInputDirR5,
			readXML:  fhirXMLToJSON(r5.UnmarshalResourceXML),
			variants: []variant{{
				name:           "with r5 model",
				baselineFile:   specBaselineFile(area, "r5"),
				baselineHeader: specKnownFailuresComment,
				model:          r5.FHIRPathModel(),
				evaluate: func(expr *fhirpath.Expression, resource []byte) (types.Collection, error) {
					return expr.EvaluateWithOptions(resource, fhirpath.WithModel(r5.FHIRPathModel()))
				},
			}},
		},
	}
}

func TestSpecCases(t *testing.T) {
	for _, area := range specAreas(t) {
		for _, c := range specCorpora(area) {
			for _, v := range c.variants {
				t.Run(c.name, func(t *testing.T) {
					runSuite(t, c, v)
				})
			}
		}
	}
}

func TestSpecCaseCatalog(t *testing.T) {
	sources := loadSpecSources(t)
	areas := specAreas(t)
	if len(areas) == 0 {
		return // nothing quotes the texts, so nothing needs them
	}
	texts, haveTexts := loadSpecTexts(t, sources)

	for _, area := range areas {
		t.Run(area, func(t *testing.T) {
			checkSpecCatalog(t, area, sources, texts, haveTexts)
		})
	}
}

var specAreaName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

func checkSpecCatalog(t *testing.T, area string, sources map[string]bool, texts map[string]string, haveTexts bool) {
	path := filepath.Join(specCasesDir, area+".xml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var file specFileXML
	if err := xml.Unmarshal(data, &file); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	if !specAreaName.MatchString(area) {
		t.Errorf("%s: an area is named in lower case with hyphens", path)
	}
	if file.Name != area {
		t.Errorf("%s: <tests name=%q> must be the file's name, %q", path, file.Name, area)
	}
	if strings.TrimSpace(file.Description) == "" {
		t.Errorf("%s: <tests> needs a description saying what the area covers", path)
	}
	if len(file.Groups) == 0 {
		t.Errorf("%s: no requirements", path)
	}

	reqName := regexp.MustCompile(`^REQ-` + strings.ToUpper(strings.ReplaceAll(area, "-", "")) + `-\d{3}$`)
	groups := map[string]bool{}
	tests := map[string]bool{}

	for _, g := range file.Groups {
		where := path + ": " + g.Name
		if !reqName.MatchString(g.Name) {
			t.Errorf("%s: a requirement is named %s", where, reqName)
		}
		if groups[g.Name] {
			t.Errorf("%s: requirement declared twice", where)
		}
		groups[g.Name] = true
		if strings.TrimSpace(g.Description) == "" {
			t.Errorf("%s: needs a description stating the requirement", where)
		}
		if g.FHIR != "" && g.FHIR != "r4" && g.FHIR != "r5" {
			t.Errorf("%s: fhir=%q, want r4, r5 or nothing", where, g.FHIR)
		}

		if len(g.Sources) == 0 {
			t.Errorf("%s: cites nothing; every requirement needs a <source>", where)
		}
		for _, s := range g.Sources {
			switch {
			case !sources[s.Spec]:
				t.Errorf("%s: spec=%q is not one of the texts in %s", where, s.Spec, specSourcesFile)
			case strings.TrimSpace(s.Section) == "":
				t.Errorf("%s: a source names its section", where)
			case !specStrengths[s.Strength]:
				t.Errorf("%s: strength=%q, want one of SHALL, SHALL NOT, SHOULD, SHOULD NOT, MAY, DEFINITION, EXAMPLE", where, s.Strength)
			case normalizeQuote(s.Quote) == "":
				t.Errorf("%s: a source quotes the sentence it cites", where)
			case haveTexts && !strings.Contains(texts[s.Spec], normalizeQuote(s.Quote)):
				t.Errorf("%s: quote not found verbatim in %s: %q", where, s.Spec, cutQuote(normalizeQuote(s.Quote)))
			}
		}

		if len(g.Tests) == 0 {
			t.Errorf("%s: no test measures it", where)
		}
		for _, tc := range g.Tests {
			if !strings.HasPrefix(tc.Name, g.Name+"-") {
				t.Errorf("%s: test %q must be named after its requirement, %s-…", where, tc.Name, g.Name)
			}
			if tests[tc.Name] {
				t.Errorf("%s: test %q declared twice", where, tc.Name)
			}
			tests[tc.Name] = true
			if strings.TrimSpace(tc.Expression.Text) == "" {
				t.Errorf("%s/%s: no expression", g.Name, tc.Name)
			}
			for _, fhir := range []string{"r4", "r5"} {
				if g.FHIR != "" && g.FHIR != fhir {
					continue
				}
				if !suiteInputExists(fhir, tc.InputFile) {
					t.Errorf("%s/%s: inputfile %q is not one of the %s suite's inputs", g.Name, tc.Name, tc.InputFile, fhir)
				}
			}
		}
	}
}

// suiteInputExists reports whether a spec case can use an input: the official
// suite's own, in either format, or none at all.
func suiteInputExists(fhir, name string) bool {
	if name == "" {
		return true
	}
	dir := suiteInputDir
	if fhir == "r5" {
		dir = suiteInputDirR5
	}
	base := strings.TrimSuffix(strings.TrimSuffix(name, ".xml"), ".json")
	for _, ext := range []string{".xml", ".json"} {
		if _, err := os.Stat(filepath.Join(dir, base+ext)); err == nil {
			return true
		}
	}
	return false
}

// loadSpecSources reads the names of the pinned texts.
func loadSpecSources(t *testing.T) map[string]bool {
	t.Helper()
	f, err := os.Open(specSourcesFile)
	if err != nil {
		t.Fatalf("read %s: %v", specSourcesFile, err)
	}
	defer f.Close()

	names := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			t.Fatalf("%s: want \"name sha256 url\", got %q", specSourcesFile, line)
		}
		names[fields[0]] = true
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read %s: %v", specSourcesFile, err)
	}
	return names
}

// loadSpecTexts reads the downloaded texts, normalized as quotes are. It
// reports false when they have not been downloaded and the run may go without.
func loadSpecTexts(t *testing.T, sources map[string]bool) (map[string]string, bool) {
	t.Helper()
	texts := map[string]string{}
	for name := range sources {
		data, err := os.ReadFile(filepath.Join(specTextsDir, name+".txt"))
		if err != nil {
			if os.Getenv("SPEC_SOURCES_REQUIRED") != "" {
				t.Fatalf("%s.txt is missing and SPEC_SOURCES_REQUIRED is set; run make spec-sources", name)
			}
			t.Logf("quotes not checked: %s.txt has not been downloaded (make spec-sources)", name)
			return nil, false
		}
		texts[name] = normalizeQuote(string(data))
	}
	return texts, true
}

// normalizeQuote forgives what copying a sentence changes without changing its
// words: runs of whitespace, typographic quotes and dashes, a no-break space.
var quoteForms = strings.NewReplacer(
	"‘", "'", "’", "'", "“", `"`, "”", `"`,
	"–", "-", "—", "-", " ", " ",
)

func normalizeQuote(s string) string {
	return strings.Join(strings.Fields(quoteForms.Replace(s)), " ")
}

func cutQuote(s string) string {
	if len(s) <= 110 {
		return s
	}
	return fmt.Sprintf("%s…", s[:110])
}
