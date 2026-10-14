package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gofhir/fhirpath"
)

// coreConstraintFaults are the core constraints static analysis rejects, and
// rightly: each names an element its resource does not have in that version,
// a defect of the specification. cid-0 asks for ChargeItemDefinition.name in
// R4 and R4B, cnl-0 for Evidence.name and EvidenceReport.name in R4B.
var coreConstraintFaults = map[string]bool{
	"r4 ChargeItemDefinition cid-0":  true,
	"r4b ChargeItemDefinition cid-0": true,
	"r4b Evidence cnl-0":             true,
	"r4b EvidenceReport cnl-0":       true,
}

// TestCoreConstraintsAnalyze analyzes every constraint of the R4, R4B and R5
// core type definitions, profiles aside, against its version's model, in the
// context of the element that carries it, and expects a fault in those above
// alone. Up to 1.14.4 it rejected 55 more: a path defined elsewhere,
// Questionnaire.item.item, was not resolved, and the path leaked from one
// operand of an operator, and one argument of a function, into the next
// (#180). Fetches the core packages on
// first use, as make corpusdiff does, and is skipped when that fails. The CI
// does not run it; conformance's TestAnalysisFollowsPathsAsEvaluationDoes
// holds the same rules there.
func TestCoreConstraintsAnalyze(t *testing.T) {
	cache := filepath.Join("..", "build", "corpusdiff", "cache")
	names := make([]string, 0, len(versions))
	for name := range versions {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		version := versions[name]
		t.Run(name, func(t *testing.T) {
			coreDir, err := fetch(cache, version.core)
			if err != nil {
				t.Skip("core package unavailable:", err)
			}
			files, err := filepath.Glob(filepath.Join(coreDir, "StructureDefinition-*.json"))
			if err != nil {
				t.Fatal(err)
			}
			model := version.model()
			analyzed := 0
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				// A type's own definition; a profile, which constrains a type,
				// can name its slices, which no model of the type holds
				var sd structureDefinition
				if json.Unmarshal(data, &sd) != nil || sd.Kind == "logical" ||
					(sd.Derivation != "specialization" && sd.Derivation != "") {
					continue
				}
				for _, element := range sd.Snapshot.Element {
					for _, c := range element.Constraint {
						if c.Expression == "" {
							continue
						}
						expr, err := fhirpath.Compile(c.Expression)
						if err != nil {
							continue
						}
						analyzed++
						err = expr.Analyze(model, element.Path)
						known := coreConstraintFaults[name+" "+element.Path+" "+c.Key]
						switch {
						case err != nil && !known:
							t.Errorf("%s %s: %v", element.Path, c.Key, err)
						case err == nil && known:
							t.Errorf("%s %s: no fault, want the one that names a missing element", element.Path, c.Key)
						}
					}
				}
			}
			if analyzed == 0 {
				t.Fatal("no constraint analyzed")
			}
		})
	}
}
