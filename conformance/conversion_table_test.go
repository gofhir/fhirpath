package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath/eval"
)

// The engine writes the FHIR-to-System conversion table in Go (eval's
// PrimitiveConversion), and this test holds it to the text of each version.
// It is the rule-5 exception that CONFORMANCE.md records (plan 017, D3): the
// table is data the specification publishes, not something the models carry,
// and a copy that was never compared with the text would drift unseen.
//
// The texts are not vendored (see SPEC_SOURCES). The test skips when they are
// absent, unless SPEC_SOURCES_REQUIRED is set, as CI's pinned-source step sets
// it: then a missing text, a missing anchor or a missing header fails.

const conversionAnchor = "according to the following mapping:"

// conversionHeader holds the two header lines of the table, which precede its
// first row.
var conversionHeader = []string{"FHIR primitive type", "FHIRPath type"}

// parseConversionTable reads the mapping table out of a rendered fhirpath.html:
// after the anchor, the two header lines, then rows that alternate a line of
// FHIR types ("FHIR.string, FHIR.uri, ...") with the line of the System type
// they convert to. The Quantity row, a complex type ("System.Quantity (see
// below)"), ends the table and is excluded by name. The result is keyed by
// FHIR type name, and holds the System type name without its namespace.
func parseConversionTable(text string) (map[string]string, error) {
	at := strings.Index(text, conversionAnchor)
	if at < 0 {
		return nil, fmt.Errorf("the anchor %q is missing", conversionAnchor)
	}
	lines := strings.Split(text[at+len(conversionAnchor):], "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for _, header := range conversionHeader {
		if len(lines) == 0 || strings.TrimSpace(lines[0]) != header {
			return nil, fmt.Errorf("the header line %q is missing", header)
		}
		lines = lines[1:]
	}
	table := map[string]string{}
	for len(lines) >= 2 {
		fhirTypes, system := strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1])
		lines = lines[2:]
		if strings.HasPrefix(system, "System.Quantity") {
			return table, nil
		}
		name, ok := strings.CutPrefix(system, "System.")
		if !ok || strings.ContainsAny(name, " ,") {
			return nil, fmt.Errorf("%q is not a System type line", system)
		}
		for _, fhirType := range strings.Split(fhirTypes, ",") {
			bare, ok := strings.CutPrefix(strings.TrimSpace(fhirType), "FHIR.")
			if !ok {
				return nil, fmt.Errorf("%q is not a FHIR type in the row for %s", fhirType, system)
			}
			if _, dup := table[bare]; dup {
				return nil, fmt.Errorf("FHIR.%s is mapped twice", bare)
			}
			table[bare] = name
		}
	}
	return nil, fmt.Errorf("the table has no Quantity row to end it")
}

// diffConversionTable lists every difference between the specification's
// table and the engine's.
func diffConversionTable(spec, engine map[string]string) []string {
	var diffs []string
	for fhirType, system := range spec {
		got, ok := engine[fhirType]
		switch {
		case !ok:
			diffs = append(diffs, fmt.Sprintf("%s: the text maps it to System.%s, the engine does not map it", fhirType, system))
		case got != system:
			diffs = append(diffs, fmt.Sprintf("%s: the text maps it to System.%s, the engine to System.%s", fhirType, system, got))
		}
	}
	for fhirType, system := range engine {
		if _, ok := spec[fhirType]; !ok {
			diffs = append(diffs, fmt.Sprintf("%s: the engine maps it to System.%s, the text does not", fhirType, system))
		}
	}
	slices.Sort(diffs)
	return diffs
}

func TestConversionTable(t *testing.T) {
	required := os.Getenv("SPEC_SOURCES_REQUIRED") != ""
	for _, tt := range []struct {
		source string
		r5     bool
	}{{"fhir-r4", false}, {"fhir-r4b", false}, {"fhir-r5", true}} {
		t.Run(tt.source, func(t *testing.T) {
			text, err := os.ReadFile(filepath.Join("testdata", "spec-cases", ".spec", tt.source+".txt"))
			if err != nil {
				if required {
					t.Fatalf("SPEC_SOURCES_REQUIRED is set and %v (run scripts/spec-sources.py)", err)
				}
				t.Skipf("%s is not downloaded (run scripts/spec-sources.py)", tt.source)
			}
			spec, err := parseConversionTable(string(text))
			if err != nil {
				t.Fatalf("%s: %v", tt.source, err)
			}
			if len(spec) < 14 {
				t.Fatalf("%s: the table holds only %d FHIR types: %v", tt.source, len(spec), spec)
			}
			for _, diff := range diffConversionTable(spec, eval.PrimitiveConversion(tt.r5)) {
				t.Errorf("%s: %s", tt.source, diff)
			}
		})
	}
}

// The comparison fails when a row is changed, removed or added.
func TestConversionTableComparisonSeesADifference(t *testing.T) {
	spec := map[string]string{"boolean": "Boolean", "code": "String", "date": "DateTime"}
	if diffs := diffConversionTable(spec, map[string]string{"boolean": "Boolean", "code": "String", "date": "DateTime"}); len(diffs) != 0 {
		t.Errorf("equal tables differ: %v", diffs)
	}
	for name, engine := range map[string]map[string]string{
		"changed": {"boolean": "Boolean", "code": "String", "date": "Date"},
		"removed": {"boolean": "Boolean", "code": "String"},
		"added":   {"boolean": "Boolean", "code": "String", "date": "DateTime", "sid": "String"},
	} {
		if diffs := diffConversionTable(spec, engine); len(diffs) != 1 {
			t.Errorf("a row %s: %d differences %v, want 1", name, len(diffs), diffs)
		}
	}
}

// The parser rejects a text that does not hold the table in the shape it
// reads, rather than reading none.
func TestConversionTableParserRefusesAnotherShape(t *testing.T) {
	good := conversionAnchor + "\nFHIR primitive type\nFHIRPath type\nFHIR.boolean\nSystem.Boolean\nFHIR.Quantity\nSystem.Quantity (see below)\n"
	table, err := parseConversionTable(good)
	if err != nil || len(table) != 1 || table["boolean"] != "Boolean" {
		t.Fatalf("the well-formed table reads %v, %v", table, err)
	}
	for name, text := range map[string]string{
		"no anchor":   "FHIR primitive type\nFHIRPath type\nFHIR.boolean\nSystem.Boolean\nFHIR.Quantity\nSystem.Quantity",
		"no header":   conversionAnchor + "\nFHIR.boolean\nSystem.Boolean\nFHIR.Quantity\nSystem.Quantity",
		"no Quantity": conversionAnchor + "\nFHIR primitive type\nFHIRPath type\nFHIR.boolean\nSystem.Boolean\n",
		"twice":       conversionAnchor + "\nFHIR primitive type\nFHIRPath type\nFHIR.boolean\nSystem.Boolean\nFHIR.boolean\nSystem.String\nFHIR.Quantity\nSystem.Quantity",
	} {
		if _, err := parseConversionTable(text); err == nil {
			t.Errorf("%s: read without an error", name)
		}
	}
}
