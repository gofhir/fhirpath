package conformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// Plan 025, criterio 3: una propiedad de consistencia sobre los ejemplos
// oficiales. Para dos objetos del mismo tipo que un modelo colocó, si a ~ b
// entonces, para cada nombre lógico de un solo valor que ambos tienen,
// a.nombre ~ b.nombre. Un tipo Quantity (o un objeto con value numérico) se deja
// fuera: compara como cantidad. El oráculo es la propia navegación
// (a.nombre), que ya tipa por el modelo; no certifica un error del resolutor
// que ambas lecturas compartan (eso lo cubren los casos escritos a mano).
//
// Es acotada: semilla fija, un tope de recursos por versión y de pares por
// recurso. Usa los paquetes que corpusdiff deja en build/corpusdiff/cache y se
// salta si no están (no descarga nada).

const (
	propertyResourcesPerVersion = 300
	propertyPairsPerType        = 16
	propertyMaxResourceBytes    = 60 << 10
)

var propertyTypes = []string{"Identifier", "HumanName", "ContactPoint", "Period", "Coding"}

// mutableKeys son las claves de cadenas libres que la mutación toca.
var mutableKeys = map[string]bool{
	"value": true, "text": true, "display": true, "code": true, "family": true, "given": true,
	"line": true, "city": true, "title": true, "description": true, "name": true,
}

var propertyDateTime = regexp.MustCompile(`^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(\.\d+)?(Z|[+-]\d\d:\d\d)$`)

// stringMutations: cada modo da una variante para el lado A y otra para el B.
var stringMutations = []struct {
	name string
	a, b func(s string) string
}{
	{"identical", keep, keep},
	{"case flip", keep, flipCase},
	{"same instant, another offset", keep, shiftOffset},
	{"injected date-like text, other offsets", func(string) string { return "2020-01-01T10:00:00Z" }, func(string) string { return "2020-01-01T12:00:00+02:00" }},
	{"injected date-like text, Z against z", func(string) string { return "2020-01-01T10:00:00Z" }, func(string) string { return "2020-01-01T10:00:00z" }},
	{"injected date-like text, equal", func(string) string { return "2020-01-01T10:00:00Z" }, func(string) string { return "2020-01-01T10:00:00Z" }},
}

func keep(s string) string { return s }

func flipCase(s string) string {
	if lower := strings.ToLower(s); lower != s {
		return lower
	}
	return strings.ToUpper(s)
}

func shiftOffset(s string) string {
	if propertyDateTime.MatchString(s) && strings.HasSuffix(s, "Z") {
		return strings.TrimSuffix(s, "Z") + "+00:00"
	}
	return s
}

// mutate visita un valor JSON y reescribe las cadenas de las claves libres.
func mutate(v any, key string, f func(string) string) any {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			x[k] = mutate(child, k, f)
		}
		return x
	case []any:
		for i, child := range x {
			x[i] = mutate(child, key, f)
		}
		return x
	case string:
		if mutableKeys[key] {
			return f(x)
		}
	}
	return v
}

func decodeJSON(t *testing.T, data []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// quantityLike deja fuera un hijo que compara como cantidad.
func quantityLike(v any) bool {
	m, ok := v.(map[string]any)
	if !ok {
		return false
	}
	_, number := m["value"].(json.Number)
	return number
}

func TestTypedComplexEquivalenceConsistencyOverTheExamples(t *testing.T) {
	cache := filepath.Join("..", "build", "corpusdiff", "cache")
	for _, version := range []struct {
		name, pattern string
	}{
		{"r4", "hl7.fhir.r4.examples#4.0.1@*"},
		{"r4b", "hl7.fhir.r4b.examples#4.3.0@*"},
		{"r5", "hl7.fhir.r5.examples#5.0.0@*"},
	} {
		dirs, _ := filepath.Glob(filepath.Join(cache, version.pattern, "package"))
		if len(dirs) == 0 {
			t.Logf("%s: no examples in %s (run `make corpusdiff` once to fetch them)", version.name, cache)
			continue
		}
		model := symmetryModelFor(version.name)
		t.Run(version.name, func(t *testing.T) {
			files, _ := filepath.Glob(filepath.Join(dirs[0], "*.json"))
			sort.Strings(files)
			rng := rand.New(rand.NewSource(25))
			rng.Shuffle(len(files), func(i, j int) { files[i], files[j] = files[j], files[i] })

			var resources, pairs, equivalent, childChecks int
			for _, file := range files {
				if resources >= propertyResourcesPerVersion {
					break
				}
				data, err := os.ReadFile(file)
				if err != nil || len(data) > propertyMaxResourceBytes || !bytes.Contains(data, []byte(`"resourceType"`)) {
					continue
				}
				mode := stringMutations[resources%len(stringMutations)]
				resources++

				a, b := decodeJSON(t, data), decodeJSON(t, data)
				a, b = mutate(a, "", mode.a), mutate(b, "", mode.b)
				ja, _ := json.Marshal(a)
				jb, _ := json.Marshal(b)
				bundle := []byte(`{"resourceType":"Bundle","type":"collection","entry":[{"resource":` + string(ja) + `},{"resource":` + string(jb) + `}]}`)

				for _, typeName := range propertyTypes {
					left, err1 := fhirpath.MustCompile("Bundle.entry[0].resource.descendants().ofType("+typeName+")").EvaluateWithOptions(bundle, fhirpath.WithModel(model))
					right, err2 := fhirpath.MustCompile("Bundle.entry[1].resource.descendants().ofType("+typeName+")").EvaluateWithOptions(bundle, fhirpath.WithModel(model))
					if err1 != nil || err2 != nil || len(left) != len(right) {
						continue
					}
					if len(left) > propertyPairsPerType {
						left, right = left[:propertyPairsPerType], right[:propertyPairsPerType]
					}
					for i := range left {
						pairs++
						oa, ok1 := left[i].(*types.ObjectValue)
						ob, ok2 := right[i].(*types.ObjectValue)
						if !ok1 || !ok2 {
							continue
						}
						if !equivalentPair(t, model, oa, ob) {
							continue
						}
						equivalent++
						childChecks += checkChildrenEquivalent(t, model, oa, ob, version.name+" "+filepath.Base(file)+" "+typeName+" ["+mode.name+"]")
					}
				}
			}
			t.Logf("%s: %d resources, %d pairs, %d equivalent, %d child checks", version.name, resources, pairs, equivalent, childChecks)
			if resources > 0 && (equivalent == 0 || childChecks == 0) {
				t.Errorf("%s: the property was never exercised (equivalent=%d, child checks=%d)", version.name, equivalent, childChecks)
			}
		})
	}
}

func equivalentPair(t *testing.T, model fhirpath.Model, a, b *types.ObjectValue) bool {
	t.Helper()
	result, err := fhirpath.MustCompile("%a ~ %b").EvaluateWithOptions(nil, fhirpath.WithModel(model),
		fhirpath.WithVariable("a", types.Collection{a}), fhirpath.WithVariable("b", types.Collection{b}))
	return err == nil && result.String() == "[true]"
}

// checkChildrenEquivalent exige a.n ~ b.n para cada nombre de un solo valor,
// lógico, que ambos objetos tienen. Devuelve cuántos nombres comprobó.
func checkChildrenEquivalent(t *testing.T, model fhirpath.Model, a, b *types.ObjectValue, where string) int {
	t.Helper()
	var ma, mb map[string]any
	da, db := json.NewDecoder(bytes.NewReader(a.Data())), json.NewDecoder(bytes.NewReader(b.Data()))
	da.UseNumber()
	db.UseNumber()
	if da.Decode(&ma) != nil || db.Decode(&mb) != nil {
		return 0
	}
	checked := 0
	names := make([]string, 0, len(ma))
	for name := range ma {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		va, vb := ma[name], mb[name]
		if vb == nil || name == "resourceType" || strings.HasPrefix(name, "_") || !propertyName.MatchString(name) {
			continue
		}
		if _, repeats := va.([]any); repeats {
			continue
		}
		if _, repeats := vb.([]any); repeats {
			continue
		}
		if quantityLike(va) || quantityLike(vb) {
			continue
		}
		if _, hasElement := ma["_"+name]; hasElement {
			continue
		}
		result, err := fhirpath.MustCompile("%a."+name+" ~ %b."+name).EvaluateWithOptions(nil, fhirpath.WithModel(model),
			fhirpath.WithVariable("a", types.Collection{a}), fhirpath.WithVariable("b", types.Collection{b}))
		if err != nil || len(result) != 1 {
			continue
		}
		checked++
		if result.String() != "[true]" {
			t.Errorf("%s: a ~ b but a.%s ~ b.%s = %s\n  a = %s\n  b = %s", where, name, name, result, a.Data(), b.Data())
		}
	}
	return checked
}

var propertyName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*$`)

func symmetryModelFor(name string) fhirpath.Model {
	for _, m := range symmetryModels {
		if m.name == name {
			return m.model
		}
	}
	panic(fmt.Sprintf("no model %s", name))
}
