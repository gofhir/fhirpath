package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofhir/fhirpath"
)

// orderedModel is a version's model that also gives the order its
// definitions list their children in, read from the core package's
// StructureDefinitions, as a validator building its model from them can.
// gofhir/models does not carry the order, and without it children() and
// descendants() come in the order the JSON writes them.
type orderedModel struct {
	fhirpath.Model
	children map[string][]string
}

// withDefinitionOrder wraps model with the children of every type and
// element the core package defines, in its snapshots' order.
func withDefinitionOrder(model fhirpath.Model, coreDir string) (fhirpath.Model, error) {
	files, err := filepath.Glob(filepath.Join(coreDir, "StructureDefinition-*.json"))
	if err != nil {
		return nil, err
	}
	children := map[string][]string{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var sd struct {
			Derivation string `json:"derivation"`
			Snapshot   struct {
				Element []struct {
					ID   string `json:"id"`
					Path string `json:"path"`
				} `json:"element"`
			} `json:"snapshot"`
		}
		if json.Unmarshal(data, &sd) != nil || sd.Derivation != "specialization" {
			continue
		}
		for _, e := range sd.Snapshot.Element {
			// A slice constrains an element of the same path; it is not a child.
			if strings.Contains(e.ID, ":") {
				continue
			}
			if i := strings.LastIndex(e.Path, "."); i > 0 {
				children[e.Path[:i]] = append(children[e.Path[:i]], e.Path[i+1:])
			}
		}
	}
	return &orderedModel{Model: model, children: children}, nil
}

func (m *orderedModel) ChildElements(path string) []string { return m.children[path] }

// FHIRVersion and HasType forward what the wrapped model declares, so that
// ordering children changes nothing else about the run.
func (m *orderedModel) FHIRVersion() string {
	if versioned, ok := m.Model.(fhirpath.VersionedModel); ok {
		return versioned.FHIRVersion()
	}
	return ""
}

func (m *orderedModel) HasType(typeName string) bool {
	if registry, ok := m.Model.(fhirpath.TypeRegistry); ok {
		return registry.HasType(typeName)
	}
	return true
}
