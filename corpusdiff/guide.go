package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// profile is a guide's constraint on a resource or data type, with the
// elements that carry constraints, and the elements whose type it constrains
// in turn with a profile of a data type.
type profile struct {
	name, resourceType string
	checks             []elementCheck
	typeProfiles       []typeProfileUse
}

// typeProfileUse is an element a resource profile types with a data type
// profile: Patient.name with ch-core-humanname, whose constraints a validator
// evaluates on each name.
type typeProfileUse struct {
	path, profile string
}

// profileDefinition is the part of a StructureDefinition a profile is read from.
type profileDefinition struct {
	URL, Name, Kind, Type, Derivation string
	Snapshot                          struct {
		Element []struct {
			ID, Path   string
			Constraint []struct{ Key, Expression string }
			Type       []struct{ Profile []string }
		}
	}
}

// buildProfiles reads a guide's profiles of resources and of data types by
// canonical URL.
func buildProfiles(dir string) (resources, dataTypes map[string]profile, err error) {
	files, err := filepath.Glob(filepath.Join(dir, "StructureDefinition-*.json"))
	if err != nil {
		return nil, nil, err
	}

	resources, dataTypes = map[string]profile{}, map[string]profile{}
	for _, file := range files {
		data, readErr := os.ReadFile(file)
		if readErr != nil {
			return nil, nil, readErr
		}
		var sd profileDefinition
		if json.Unmarshal(data, &sd) != nil || sd.Derivation != "constraint" {
			continue
		}
		switch sd.Kind {
		case kindResource:
			resources[sd.URL] = profileOf(&sd)
		case "complex-type":
			dataTypes[sd.URL] = profileOf(&sd)
		}
	}
	return resources, dataTypes, nil
}

// profileOf reads a profile's checks and the data type profiles its elements
// name. A slice's elements are left out, as a validator evaluating the base
// element evaluates them.
func profileOf(sd *profileDefinition) profile {
	p := profile{name: sd.Name, resourceType: sd.Type}
	for _, e := range sd.Snapshot.Element {
		if strings.Contains(e.ID, ":") {
			continue
		}
		var exprs []string
		for _, c := range e.Constraint {
			if c.Expression != "" && c.Key != "ele-1" {
				exprs = append(exprs, c.Expression)
			}
		}
		if len(exprs) > 0 {
			p.checks = append(p.checks, elementCheck{path: e.Path, constraints: exprs})
		}
		if e.Path == sd.Type {
			continue
		}
		for _, t := range e.Type {
			for _, url := range t.Profile {
				p.typeProfiles = append(p.typeProfiles, typeProfileUse{path: e.Path, profile: url})
			}
		}
	}
	return p
}

// evalGuide evaluates a guide's examples against the profiles each claims in
// meta.profile, element by element as a validator does, and writes the answers
// as eval does.
func evalGuide(dir string, model fhirpath.Model, out, name string) error {
	profiles, dataTypes, err := buildProfiles(dir)
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "example", "*.json"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	f, err := os.Create(out)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)

	compiled := map[string]*fhirpath.Expression{}
	n := 0
	for _, file := range files {
		written, checkErr := checkExample(w, file, profiles, dataTypes, model, compiled)
		n += written
		if checkErr != nil {
			f.Close()
			return checkErr
		}
	}

	err = w.Flush()
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	fmt.Fprintf(os.Stderr, "%s: %d evaluations\n", name, n)
	return err
}

// checkExample evaluates one example against the profiles it claims.
func checkExample(w io.Writer, file string, profiles, dataTypes map[string]profile,
	model fhirpath.Model, compiled map[string]*fhirpath.Expression) (int, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}
	var head struct {
		ResourceType string
		Meta         struct{ Profile []string }
	}
	if json.Unmarshal(data, &head) != nil {
		return 0, nil
	}
	resource, err := types.JSONToCollection(data)
	if err != nil {
		return 0, nil
	}
	for _, value := range resource {
		if obj, ok := value.(*types.ObjectValue); ok {
			obj.EnableCaching()
		}
	}

	n := 0
	for _, claimed := range head.Meta.Profile {
		url, _, _ := strings.Cut(claimed, "|")
		p, known := profiles[url]
		if !known || p.resourceType != head.ResourceType {
			continue
		}
		label := filepath.Base(file) + "|" + p.name
		for _, check := range p.checks {
			written, checkErr := checkElement(w, label, data, head.ResourceType, check, resource, model, compiled)
			n += written
			if checkErr != nil {
				return n, checkErr
			}
		}
		written, checkErr := checkTypeProfiles(w, label, data, head.ResourceType, p, dataTypes, resource, model, compiled)
		n += written
		if checkErr != nil {
			return n, checkErr
		}
	}
	return n, nil
}

// checkTypeProfiles evaluates, for each element a resource profile types with a
// data type profile, that profile's constraints on each of the element's
// instances, the instance as the root of the data type's paths.
func checkTypeProfiles(w io.Writer, label string, data []byte, resourceType string, p profile,
	dataTypes map[string]profile, resource fhirpath.Collection, model fhirpath.Model,
	compiled map[string]*fhirpath.Expression) (int, error) {
	n := 0
	for _, use := range p.typeProfiles {
		url, _, _ := strings.Cut(use.profile, "|")
		dt, known := dataTypes[url]
		if !known {
			continue
		}
		for _, element := range instancesOf(data, resourceType, use.path) {
			for _, check := range dt.checks {
				var instances []elementInstance
				if check.path == dt.resourceType {
					// The data type's own constraints: the element is the
					// root, placed at the element it is in the resource.
					instances = []elementInstance{element}
				} else {
					for _, inner := range instancesOf(element.data, dt.resourceType, check.path) {
						inner.label = element.label + "/" + inner.label
						instances = append(instances, inner)
					}
				}
				written, err := checkInstances(w, label+"|"+dt.name, instances, check, resource, model, compiled)
				n += written
				if err != nil {
					return n, err
				}
			}
		}
	}
	return n, nil
}
