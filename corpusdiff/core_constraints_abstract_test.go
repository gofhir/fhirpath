package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// snapshotAbstractModel is a version's published model that also describes the
// abstract types, as a model built from the StructureDefinition snapshots does:
// the elements of Resource, DomainResource and the other abstract types, and
// which types are abstract. The published models leave both out.
type snapshotAbstractModel struct {
	fhirpath.Model
	elements map[string]string
	abstract map[string]bool
}

func (m snapshotAbstractModel) TypeOf(path string) string {
	if t := m.Model.TypeOf(path); t != "" {
		return t
	}
	return m.elements[path]
}

func (m snapshotAbstractModel) IsAbstract(typeName string) bool {
	return m.abstract[typeName]
}

// abstractDefinition is the part of a StructureDefinition the model reads.
type abstractDefinition struct {
	Kind       string `json:"kind"`
	Type       string `json:"type"`
	Abstract   bool   `json:"abstract"`
	Derivation string `json:"derivation"`
	Snapshot   struct {
		Element []struct {
			Path string `json:"path"`
			Type []struct {
				Code string `json:"code"`
			} `json:"type"`
		} `json:"element"`
	} `json:"snapshot"`
}

// loadAbstractTypes reads, from the core package's StructureDefinitions, the
// types marked `abstract: true` (structuredefinition.html: "Whether structure
// this definition describes is abstract or not - that is, whether the
// structure is not intended to be instantiated") and their own elements. A
// constraint profile of a type is left out: it is not the type's definition.
func loadAbstractTypes(t *testing.T, coreDir string) (elements map[string]string, abstract map[string]bool) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(coreDir, "StructureDefinition-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	elements = map[string]string{}
	abstract = map[string]bool{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var sd abstractDefinition
		if json.Unmarshal(data, &sd) != nil || sd.Kind == "logical" ||
			(sd.Derivation != "specialization" && sd.Derivation != "") || !sd.Abstract {
			continue
		}
		abstract[sd.Type] = true
		for _, e := range sd.Snapshot.Element {
			// The type's own elements; a choice element has no single type
			rest, ok := strings.CutPrefix(e.Path, sd.Type+".")
			if !ok || strings.Contains(rest, ".") || strings.HasSuffix(rest, "[x]") || len(e.Type) != 1 {
				continue
			}
			elements[e.Path] = e.Type[0].Code
		}
	}
	return elements, abstract
}

// TestCoreConstraintsAnalyzeThroughAbstractTypes is TestCoreConstraintsAnalyze
// with a model that describes the abstract types, from the snapshots, and says
// which are abstract: dom-2's contained.contained.empty() and the like must
// not be rejected, so only coreConstraintFaults remain. Paired with the
// misspellings under a backbone element, which the abstract BackboneElement
// must not hide, since the core constraints hold none. Same caching and skip
// behavior as that test; the CI does not run it either.
func TestCoreConstraintsAnalyzeThroughAbstractTypes(t *testing.T) {
	cache := filepath.Join("..", "build", "corpusdiff", "cache")
	names := make([]string, 0, len(versions))
	for name := range versions {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		version := versions[name]
		t.Run(name, func(t *testing.T) {
			dir, err := fetch(cache, version.core)
			coreDir := fetchedOrSkip(t, dir, err)
			elements, abstract := loadAbstractTypes(t, coreDir)
			for _, want := range []string{"Resource", "DomainResource", "BackboneElement"} {
				if !abstract[want] {
					t.Fatalf("%s not read as abstract from the package", want)
				}
			}
			model := snapshotAbstractModel{Model: version.model(), elements: elements, abstract: abstract}

			files, err := filepath.Glob(filepath.Join(coreDir, "StructureDefinition-*.json"))
			if err != nil {
				t.Fatal(err)
			}
			analyzed := 0
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
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

			// A misspelling under a backbone element is still a fault, though
			// BackboneElement is abstract
			for _, tt := range []struct{ context, expr string }{
				{"Questionnaire", "Questionnaire.item.lnkId"},
				{"Questionnaire.item", "item.item.lnkId"},
				{"Patient", "Patient.contact.nme"},
				{"Parameters", "Parameters.parameter.nme"},
			} {
				if err := fhirpath.MustCompile(tt.expr).Analyze(model, tt.context); err == nil {
					t.Errorf("%s on %s: no fault", tt.expr, tt.context)
				}
			}
		})
	}
}
