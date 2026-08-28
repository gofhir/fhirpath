package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofhir/models/r4"
	"github.com/gofhir/models/r5"
)

// A corpus is one of the suites, read from where the conformance harness
// already vendors it rather than from a second copy.
type corpus struct {
	name     string
	file     string
	inputDir string
	// readXML turns one of the suite's XML resources into JSON, which is what
	// both engines read. The conversion goes through the version's generated
	// model, as the conformance harness does, so both engines see the resource
	// the suite was written against.
	readXML func([]byte) ([]byte, error)
}

func corpora() []corpus {
	const suites = "../conformance/testdata"

	return []corpus{
		{
			name:     "r4",
			file:     suites + "/fhirpath-suite/tests-fhir-r4.xml",
			inputDir: suites + "/fhirpath-suite/input",
			readXML:  fhirXMLToJSON(r4.UnmarshalResourceXML),
		},
		{
			name:     "r5",
			file:     suites + "/fhirpath-suite-r5/tests-fhir-r5.xml",
			inputDir: suites + "/fhirpath-suite-r5/input",
			readXML:  fhirXMLToJSON(r5.UnmarshalResourceXML),
		},
	}
}

func fhirXMLToJSON[R any](unmarshal func([]byte) (R, error)) func([]byte) ([]byte, error) {
	return func(data []byte) ([]byte, error) {
		resource, err := unmarshal(data)
		if err != nil {
			return nil, err
		}
		return json.Marshal(resource)
	}
}

// The shape of the suite file, cut down to what a comparison needs.
type suiteFile struct {
	Groups []struct {
		Name  string `xml:"name,attr"`
		Tests []struct {
			Name       string `xml:"name,attr"`
			InputFile  string `xml:"inputfile,attr"`
			Predicate  string `xml:"predicate,attr"`
			Mode       string `xml:"mode,attr"`
			Expression struct {
				Invalid string `xml:"invalid,attr"`
				Text    string `xml:",chardata"`
			} `xml:"expression"`
			Outputs []struct {
				Type string `xml:"type,attr"`
				Text string `xml:",chardata"`
			} `xml:"output"`
		} `xml:"test"`
	} `xml:"group"`
}

// load reads the suite into cases both engines can be asked.
func (c corpus) load(withModel bool) ([]testCase, error) {
	data, err := os.ReadFile(c.file)
	if err != nil {
		return nil, err
	}

	var suite suiteFile
	if err := xml.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("reading %s: %w", c.file, err)
	}

	model := ""
	if withModel {
		model = c.name
	}

	inputs := map[string]json.RawMessage{}
	var cases []testCase

	seen := map[string]int{}

	for _, group := range suite.Groups {
		for _, test := range group.Tests {
			id := group.Name + "/" + test.Name
			// The suite repeats a name for two different expressions —
			// testEquivalent23 among them — and an id that collapses them
			// would compare one engine's answer to the other engine's other
			// case.
			if seen[id]++; seen[id] > 1 {
				id = fmt.Sprintf("%s#%d", id, seen[id])
			}

			this := testCase{
				ID:         id,
				Expression: strings.TrimSpace(test.Expression.Text),
				Model:      model,
				// A case asserting a semantic fault is one only static
				// analysis can decide, and this harness evaluates rather than
				// analyzes. Judging it by what evaluation answered would
				// report a divergence that make conformance does not see.
				expectErr: test.Expression.Invalid == "execution",
				predicate: test.Predicate == "true",
			}

			if test.Expression.Invalid == "semantic" || strings.HasPrefix(test.Mode, "lenient") {
				this.skip = "decided by static analysis, which this harness does not run"
			}

			for _, out := range test.Outputs {
				this.outputs = append(this.outputs, strings.TrimSpace(out.Text))
				this.outputTypes = append(this.outputTypes, out.Type)
			}

			resource, ok := inputs[test.InputFile]
			if !ok {
				loaded, err := c.readInput(test.InputFile)
				if err != nil {
					// An input neither engine can read is not a divergence, and
					// saying so is better than dropping it silently.
					this.skip = err.Error()
					cases = append(cases, this)
					continue
				}
				resource = loaded
				inputs[test.InputFile] = resource
			}
			this.Resource = resource

			cases = append(cases, this)
		}
	}
	return cases, nil
}

// readInput resolves an input file to JSON, preferring the suite's own copy in
// whatever format it ships.
func (c corpus) readInput(name string) (json.RawMessage, error) {
	if name == "" {
		return json.RawMessage(`{}`), nil
	}

	base := strings.TrimSuffix(strings.TrimSuffix(name, ".xml"), ".json")

	if data, err := os.ReadFile(filepath.Join(c.inputDir, base+".xml")); err == nil {
		converted, err := c.readXML(data)
		if err != nil {
			return nil, fmt.Errorf("%s is not a readable FHIR resource", base+".xml")
		}
		return converted, nil
	}

	if data, err := os.ReadFile(filepath.Join(c.inputDir, base+".json")); err == nil {
		return data, nil
	}
	return nil, fmt.Errorf("no input file %q", name)
}
