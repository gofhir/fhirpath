package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// extensionValueTypes reads the types a version's Extension.value[x] allows
// from its core package's own definition.
func extensionValueTypes(t *testing.T, coreDir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(coreDir, "StructureDefinition-Extension.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sd struct {
		Snapshot struct {
			Element []struct {
				ID   string `json:"id"`
				Type []struct {
					Code string `json:"code"`
				} `json:"type"`
			} `json:"element"`
		} `json:"snapshot"`
	}
	if err := json.Unmarshal(data, &sd); err != nil {
		t.Fatal(err)
	}
	for _, e := range sd.Snapshot.Element {
		if e.ID == "Extension.value[x]" {
			codes := make([]string, len(e.Type))
			for i, typ := range e.Type {
				codes[i] = typ.Code
			}
			return codes
		}
	}
	t.Fatal("no Extension.value[x] in the core package")
	return nil
}

// TestTheModelAllowsTheExtensionValuesTheDefinitionDoes checks that the
// version's model lists for Extension.value the types the core package's
// StructureDefinition gives Extension.value[x], so that a test taking them
// from the model takes the specification's.
func TestTheModelAllowsTheExtensionValuesTheDefinitionDoes(t *testing.T) {
	if testing.Short() {
		t.Skip("reads the core packages; run without -short")
	}
	cache := filepath.Join("..", "build", "corpusdiff", "cache")
	for _, name := range sortedVersions() {
		version := versions[name]
		t.Run(name, func(t *testing.T) {
			coreDir, err := fetch(cache, version.core)
			if err != nil {
				t.Skip("core package unavailable:", err)
			}
			want := extensionValueTypes(t, coreDir)
			got := version.model().ChoiceTypes("Extension.value")
			sort.Strings(want)
			sorted := slices.Clone(got)
			sort.Strings(sorted)
			if !slices.Equal(sorted, want) {
				t.Errorf("model lists %d types %v, the definition %d %v", len(got), got, len(want), want)
			}
		})
	}
}

// TestGetExtensionValueAnswersAsTheJSONHoldsIt reads, for every place an
// official R4, R4B or R5 example holds extensions, the value[x] each
// extension carries straight from the JSON, and expects getExtensionValue(url)
// at that path to answer those values, as the JSON writes them, of the types
// their names give, in order: a value[x] whose type the version's Extension does not allow is not
// one. The JSON is read without the engine's navigation, so the two cannot
// share a defect (#218).
func TestGetExtensionValueAnswersAsTheJSONHoldsIt(t *testing.T) {
	if testing.Short() {
		t.Skip("walks every official example; run without -short")
	}
	cache := filepath.Join("..", "build", "corpusdiff", "cache")
	for _, name := range sortedVersions() {
		version := versions[name]
		t.Run(name, func(t *testing.T) {
			coreDir, err := fetch(cache, version.core)
			if err != nil {
				t.Skip("core package unavailable:", err)
			}
			dir, err := fetch(cache, version.examples)
			if err != nil {
				t.Skip("examples package unavailable:", err)
			}
			allowed := map[string]bool{}
			for _, typ := range extensionValueTypes(t, coreDir) {
				allowed[typ] = true
			}
			files, err := filepath.Glob(filepath.Join(dir, "*.json"))
			if err != nil {
				t.Fatal(err)
			}
			sort.Strings(files)
			opts := []fhirpath.EvalOption{fhirpath.WithModel(version.model())}

			seenTypes := map[string]bool{}
			var pairs, values, differences int
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				// Numbers keep the text the JSON writes them in, 1.50 not 1.5,
				// as the engine keeps it.
				var resource map[string]any
				dec := json.NewDecoder(bytes.NewReader(data))
				dec.UseNumber()
				if dec.Decode(&resource) != nil {
					continue
				}
				rt, _ := resource["resourceType"].(string)
				if rt == "" {
					continue
				}
				held := map[string][]map[string]any{} // path -> extensions, in order
				extensionsAt(rt, resource, held)
				if len(held) == 0 {
					continue
				}
				doc, err := fhirpath.NewDocument(data)
				if err != nil {
					continue
				}
				for _, path := range sortedKeys(held) {
					byURL := map[string][]string{}
					var urls []string
					for _, ext := range held[path] {
						u, _ := ext["url"].(string)
						if u == "" {
							continue
						}
						if _, seen := byURL[u]; !seen {
							urls = append(urls, u)
							byURL[u] = []string{}
						}
						byURL[u] = append(byURL[u], jsonValues(ext, allowed)...)
					}
					for _, u := range urls {
						pairs++
						want := byURL[u]
						values += len(want)
						for _, item := range want {
							typ, _, _ := strings.Cut(item, " ")
							seenTypes[typ] = true
						}
						lit := "'" + strings.ReplaceAll(strings.ReplaceAll(u, `\`, `\\`), "'", `\'`) + "'"
						expr := path + ".getExtensionValue(" + lit + ")"
						got, err := valuesOf(doc, expr, opts)
						if err != nil {
							differences++
							t.Errorf("%s: %s: %v", filepath.Base(file), expr, err)
							continue
						}
						if !slices.Equal(got, want) {
							differences++
							if differences <= 20 {
								t.Errorf("%s: %s = %v, the JSON holds %v", filepath.Base(file), expr, got, want)
							}
						}
					}
				}
			}
			t.Logf("%s: %d (path, URL) pairs, %d values of %d distinct types, %d differences",
				name, pairs, values, len(seenTypes), differences)
			if values == 0 {
				t.Fatal("no extension values found in the examples")
			}
		})
	}
}

// extensionsAt records the extensions each place in a resource's JSON holds,
// under the FHIRPath path that reaches it, as extensionPlaces does.
func extensionsAt(path string, object map[string]any, held map[string][]map[string]any) {
	for _, key := range sortedKeys(object) {
		if key == "resourceType" {
			continue
		}
		value := object[key]
		if key == "extension" {
			for _, item := range asList(value) {
				if ext, ok := item.(map[string]any); ok {
					held[path] = append(held[path], ext)
				}
			}
		}
		name := strings.TrimPrefix(key, "_")
		for _, item := range asList(value) {
			if child, ok := item.(map[string]any); ok {
				extensionsAt(path+"."+escapeName(name), child, held)
			}
		}
	}
}

// jsonValues is each value an extension's JSON holds under a value[x] name,
// as "type value": the type is the name's suffix, its first letter lowered
// when that is how the version names the type (valueDateTime is a dateTime),
// and the value the JSON's own text, or for an object, and for a primitive
// with only the element beside it, the object canonically encoded. A name
// whose type the version does not allow holds no value.
func jsonValues(ext map[string]any, allowed map[string]bool) []string {
	var found []string
	seen := map[string]bool{}
	for _, key := range sortedKeys(ext) {
		name := strings.TrimPrefix(key, "_")
		suffix, ok := strings.CutPrefix(name, "value")
		if !ok || suffix == "" || suffix[0] < 'A' || suffix[0] > 'Z' || seen[suffix] {
			continue
		}
		typ := suffix
		if lower := strings.ToLower(suffix[:1]) + suffix[1:]; allowed[lower] {
			typ = lower
		}
		if !allowed[typ] {
			continue
		}
		seen[suffix] = true
		// a primitive's value and the element beside it are one item, an
		// array's entries paired by position; a position null in both is none
		values, elements := present(ext["value"+suffix]), present(ext["_value"+suffix])
		for i := range max(len(values), len(elements)) {
			var value, element any
			if i < len(values) {
				value = values[i]
			}
			if i < len(elements) {
				element = elements[i]
			}
			switch {
			case value != nil:
				found = append(found, typ+" "+jsonText(value))
			case element != nil:
				found = append(found, typ+" "+jsonText(element))
			}
		}
	}
	return found
}

// present is a JSON value as a list, a single value its only entry; nothing
// written is no entry.
func present(value any) []any {
	if value == nil {
		return nil
	}
	return asList(value)
}

// jsonText is a JSON value as the engine renders one: a string as it reads,
// a number or boolean as written, an object canonically encoded.
func jsonText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		encoded, _ := json.Marshal(v)
		return string(encoded)
	}
}

// canonical re-encodes an object's JSON with its keys sorted, so that two
// renderings of one object compare equal.
func canonical(s string) string {
	var v any
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	if dec.Decode(&v) != nil || dec.More() {
		return s
	}
	return jsonText(v)
}

func valuesOf(doc *fhirpath.Document, text string, opts []fhirpath.EvalOption) ([]string, error) {
	expr, err := fhirpath.Compile(text)
	if err != nil {
		return nil, err
	}
	result, err := doc.EvaluateWithOptions(expr, opts...)
	if err != nil {
		return nil, err
	}
	items := make([]string, len(result))
	for i, v := range result {
		text := v.String()
		// only an object renders as JSON; a string that reads like JSON, a
		// regex such as "[0]|[1-9]", is its own text
		if _, ok := v.(*types.ObjectValue); ok {
			text = canonical(text)
		}
		items[i] = v.Type() + " " + text
	}
	return items, nil
}

func sortedVersions() []string {
	names := make([]string, 0, len(versions))
	for name := range versions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
