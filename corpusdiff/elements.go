package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// An element check is how a validator evaluates an element's invariants: each
// instance of the element is taken out of the resource and becomes the root,
// placed with SetPath at the element it is, with the resource as %resource.
// A primitive's root is then a bare JSON value — "2019-12-08" — which only the
// model and the path can type. The resource-rooted corpus never evaluates that
// way, and AU Core's au-core-obs-02 failed exactly there.
type elementCheck struct {
	path        string   // the element's path in the snapshot, Observation.effective[x]
	constraints []string // its constraints' expressions, ele-1 left out
}

// buildElementChecks lists, for each resource type, the elements below the
// root that carry constraints.
func buildElementChecks(coreDir string) (map[string][]elementCheck, error) {
	files, err := filepath.Glob(filepath.Join(coreDir, "StructureDefinition-*.json"))
	if err != nil {
		return nil, err
	}

	checks := map[string][]elementCheck{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var sd structureDefinition
		if json.Unmarshal(data, &sd) != nil || sd.Kind != kindResource || sd.Derivation != "specialization" {
			continue
		}
		for _, e := range sd.Snapshot.Element {
			if e.Path == sd.Type {
				continue
			}
			var exprs []string
			for _, c := range e.Constraint {
				if c.Expression != "" && c.Key != "ele-1" {
					exprs = append(exprs, c.Expression)
				}
			}
			if len(exprs) > 0 {
				checks[sd.Type] = append(checks[sd.Type], elementCheck{path: e.Path, constraints: exprs})
			}
		}
	}
	return checks, nil
}

// elementInstance is one occurrence of an element in a resource: its JSON as
// it stands in the document, the path a validator gives it, and a label that
// tells it from the element's other occurrences.
type elementInstance struct {
	data  []byte
	path  string // Observation.effectiveDateTime: the variant, no indices
	label string // effectiveDateTime, component[1].valueQuantity
}

// instancesOf finds every occurrence of an element in a resource, following
// arrays and choice variants, and keeps each one's bytes as written, so that a
// decimal or an escape reaches the engine untouched.
func instancesOf(resource []byte, resourceType, elementPath string) []elementInstance {
	if elementPath == resourceType {
		return []elementInstance{{data: resource, path: resourceType}}
	}
	segments := strings.Split(strings.TrimPrefix(elementPath, resourceType+"."), ".")
	current := []elementInstance{{data: resource, path: resourceType}}

	for _, segment := range segments {
		var next []elementInstance
		for _, parent := range current {
			var fields map[string]json.RawMessage
			if json.Unmarshal(parent.data, &fields) != nil {
				continue
			}

			for key, value := range fields {
				if !matchesSegment(key, segment) {
					continue
				}
				prefix := parent.label
				if prefix != "" {
					prefix += "."
				}
				path := parent.path + "." + key
				if items, isArray := arrayItems(value); isArray {
					for i, item := range items {
						if !bytes.Equal(bytes.TrimSpace(item), []byte("null")) {
							next = append(next, elementInstance{data: item, path: path, label: fmt.Sprintf("%s%s[%d]", prefix, key, i)})
						}
					}
				} else if !bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
					next = append(next, elementInstance{data: value, path: path, label: prefix + key})
				}
			}
		}
		current = next
	}

	sort.Slice(current, func(i, j int) bool { return current[i].label < current[j].label })
	return current
}

// matchesSegment reports whether a JSON key is the element a path segment
// names: the same name, or for a choice, value[x], a variant such as
// valueQuantity.
func matchesSegment(key, segment string) bool {
	base, choice := strings.CutSuffix(segment, "[x]")
	if !choice {
		return key == segment
	}
	return len(key) > len(base) && strings.HasPrefix(key, base) && key[len(base)] >= 'A' && key[len(base)] <= 'Z'
}

func arrayItems(value json.RawMessage) ([]json.RawMessage, bool) {
	trimmed := bytes.TrimSpace(value)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return nil, false
	}
	var items []json.RawMessage
	if json.Unmarshal(trimmed, &items) != nil {
		return nil, false
	}
	return items, true
}

// evaluateElements writes one line per instance and constraint: the file and
// the instance's label, the element's path and the constraint, and the answer.
// The context is made as gofhir/validator makes it, with the API every
// revision from 1.7.0 has.
func evaluateElements(checks map[string][]elementCheck, examplesDir string, model fhirpath.Model, w io.Writer) (int, error) {
	files, err := filepath.Glob(filepath.Join(examplesDir, "*.json"))
	if err != nil {
		return 0, err
	}
	sort.Strings(files)

	compiled := map[string]*fhirpath.Expression{}
	n := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return n, err
		}
		if len(data) > maxResource {
			continue
		}
		var head struct {
			ResourceType string `json:"resourceType"`
		}
		if json.Unmarshal(data, &head) != nil || len(checks[head.ResourceType]) == 0 {
			continue
		}
		resource, err := types.JSONToCollection(data)
		if err != nil {
			continue
		}
		for _, value := range resource {
			if obj, ok := value.(*types.ObjectValue); ok {
				obj.EnableCaching()
			}
		}

		for _, check := range checks[head.ResourceType] {
			written, err := checkElement(w, filepath.Base(file), data, head.ResourceType, check, resource, model, compiled)
			n += written
			if err != nil {
				return n, err
			}
		}
	}
	return n, nil
}

// checkElement evaluates an element's constraints on each of its instances in
// a resource, as a validator does, and writes a line for each.
func checkElement(w io.Writer, file string, data []byte, resourceType string, check elementCheck,
	resource fhirpath.Collection, model fhirpath.Model, compiled map[string]*fhirpath.Expression) (int, error) {
	return checkInstances(w, file, instancesOf(data, resourceType, check.path), check, resource, model, compiled)
}

// checkInstances evaluates an element's constraints on the instances given.
func checkInstances(w io.Writer, file string, instances []elementInstance, check elementCheck,
	resource fhirpath.Collection, model fhirpath.Model, compiled map[string]*fhirpath.Expression) (int, error) {
	n := 0
	for _, instance := range instances {
		for _, text := range check.constraints {
			expr, seen := compiled[text]
			if !seen {
				var err error
				if expr, err = fhirpath.Compile(text); err != nil {
					expr = nil
				}
				compiled[text] = expr
			}

			line := "compile-error"
			if expr != nil {
				ctx := eval.NewContext(instance.data)
				ctx.SetModel(model)
				ctx.SetPath(instance.path)
				ctx.SetVariable("resource", resource)
				ctx.SetVariable("rootResource", resource)
				line = answer(expr.EvaluateWithContext(ctx))
			}
			if _, err := fmt.Fprintf(w, "%s#%s\t%s :: %s\t%s\n", file, instance.label, check.path, oneLine(text), line); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

// oneLine writes an expression on one line, as the output's format needs: a
// guide's constraint can span several.
func oneLine(expr string) string {
	return strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ").Replace(expr)
}
