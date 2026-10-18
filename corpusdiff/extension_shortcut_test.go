package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// TestExtensionIsItsWhereOverTheExamples evaluates FHIR's extension(url) and
// the expression FHIR defines it as a shortcut for, .extension.where(url =
// url), over every official example of R4, R4B and R5, and expects the same
// items, of the same types, from both. Each extension URL a resource holds is
// asked from two kinds of input: descendants(), which reaches elements
// through their typed children, and the path to each place the JSON holds
// extensions (Patient.contact.extension(u)), which reaches them by navigation.
// The items are compared with their url, value and descendants, each with
// its type. Up to 1.16.5 extension(url) read its items untyped (#214).
//
// where() checks the size of its input and extension() does not; that limit
// is no part of what either answers, so it is lifted here. Fetches the
// example packages on first use, as make corpusdiff does, and is skipped when
// that fails. It takes minutes, so -short skips it; the CI does not run it.
// Run it with make extension-shortcut.
func TestExtensionIsItsWhereOverTheExamples(t *testing.T) {
	if testing.Short() {
		t.Skip("walks every official example; run without -short")
	}
	cache := filepath.Join("..", "build", "corpusdiff", "cache")
	names := make([]string, 0, len(versions))
	for name := range versions {
		names = append(names, name)
	}
	sort.Strings(names)

	projections := []string{"", ".url", ".value", ".descendants()"}

	for _, name := range names {
		version := versions[name]
		t.Run(name, func(t *testing.T) {
			dir, err := fetch(cache, version.examples)
			if err != nil {
				t.Skip("examples package unavailable:", err)
			}
			files, err := filepath.Glob(filepath.Join(dir, "*.json"))
			if err != nil {
				t.Fatal(err)
			}
			sort.Strings(files)
			opts := []fhirpath.EvalOption{
				fhirpath.WithModel(version.model()),
				fhirpath.WithMaxCollectionSize(math.MaxInt32),
			}

			urls := map[string]bool{}
			var pairs, comparisons, errored, differences int
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil {
					t.Fatal(err)
				}
				var resource map[string]any
				if json.Unmarshal(data, &resource) != nil {
					continue
				}
				rt, _ := resource["resourceType"].(string)
				if rt == "" {
					continue
				}
				places := map[string]map[string]bool{} // path -> urls
				extensionPlaces(rt, resource, places)
				if len(places) == 0 {
					continue
				}

				// Each form reads its own document, so that what one places on
				// the shared objects cannot answer for the other.
				shortcutDoc, err1 := fhirpath.NewDocument(data)
				spelledDoc, err2 := fhirpath.NewDocument(data)
				if err1 != nil || err2 != nil {
					continue
				}

				byURL := map[string]bool{}
				var inputs []extensionInput
				for path, us := range places {
					for u := range us {
						inputs = append(inputs, extensionInput{path, u})
						byURL[u] = true
					}
				}
				for u := range byURL {
					inputs = append(inputs, extensionInput{rt + ".descendants()", u})
					urls[u] = true
				}
				sort.Slice(inputs, func(i, j int) bool {
					if inputs[i].path != inputs[j].path {
						return inputs[i].path < inputs[j].path
					}
					return inputs[i].url < inputs[j].url
				})

				for _, in := range inputs {
					pairs++
					lit := "'" + strings.ReplaceAll(in.url, "'", `\'`) + "'"
					for _, proj := range projections {
						shortcut := in.path + ".extension(" + lit + ")" + proj
						spelled := in.path + ".extension.where(url = " + lit + ")" + proj
						got, gotErr := typedAnswer(shortcutDoc, shortcut, opts)
						want, wantErr := typedAnswer(spelledDoc, spelled, opts)
						comparisons++
						if gotErr != nil || wantErr != nil {
							errored++
							if (gotErr == nil) != (wantErr == nil) {
								differences++
								t.Errorf("%s: %s errs %v, %s errs %v", filepath.Base(file), shortcut, gotErr, spelled, wantErr)
							}
							continue
						}
						if got != want {
							differences++
							if differences <= 20 {
								t.Errorf("%s: %s = %s\n\twant %s as %s answers",
									filepath.Base(file), shortcut, clip(got), clip(want), spelled)
							}
						}
					}
				}
			}
			t.Logf("%s: %d distinct URLs, %d (input, URL) pairs, %d comparisons, %d with an error on both sides, %d differences",
				name, len(urls), pairs, comparisons, errored, differences)
			if pairs == 0 {
				t.Fatal("no extensions found in the examples")
			}
		})
	}
}

type extensionInput struct{ path, url string }

// extensionPlaces records, under the FHIRPath path that reaches it, every
// place in a resource's JSON that holds extensions with a url, the element
// FHIR writes beside a primitive (_birthDate) under the primitive's name.
// Arrays share their path, as navigation flattens them; a resource held by
// an element is reached through the element's path.
func extensionPlaces(path string, object map[string]any, places map[string]map[string]bool) {
	for key, value := range object {
		if key == "resourceType" {
			continue
		}
		name := strings.TrimPrefix(key, "_")
		if key == "extension" {
			for _, item := range asList(value) {
				if ext, ok := item.(map[string]any); ok {
					if u, ok := ext["url"].(string); ok && u != "" {
						if places[path] == nil {
							places[path] = map[string]bool{}
						}
						places[path][u] = true
					}
				}
			}
		}
		for _, item := range asList(value) {
			if child, ok := item.(map[string]any); ok {
				extensionPlaces(path+"."+escapeName(name), child, places)
			}
		}
	}
}

func asList(value any) []any {
	if list, ok := value.([]any); ok {
		return list
	}
	return []any{value}
}

// escapeName delimits a name FHIRPath reads as a keyword or a type.
func escapeName(name string) string {
	switch name {
	case "div", "as", "is", "contains", "in", "and", "or", "xor", "implies", "mod":
		return "`" + name + "`"
	}
	return name
}

func typedAnswer(doc *fhirpath.Document, text string, opts []fhirpath.EvalOption) (string, error) {
	expr, err := fhirpath.Compile(text)
	if err != nil {
		return "", err
	}
	result, err := doc.EvaluateWithOptions(expr, opts...)
	if err != nil {
		return "", err
	}
	items := make([]string, len(result))
	for i, v := range result {
		items[i] = v.Type() + " " + v.String()
	}
	return "[" + strings.Join(items, ", ") + "]", nil
}

func clip(s string) string {
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}
