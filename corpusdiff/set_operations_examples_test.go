package main

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// Las operaciones de conjunto como estaban antes del índice (#243), copiadas
// congeladas: la referencia con la que se miden las indexadas sobre los
// ejemplos oficiales. Llevan su propio containsRef. La copia de types/ mide los
// mismos operadores sobre colecciones generadas.

func containsRef(c types.Collection, v types.Value) bool {
	for _, item := range c {
		if item.Equal(v) {
			return true
		}
	}
	return false
}

func distinctRef(c types.Collection) types.Collection {
	if len(c) <= 1 {
		return c
	}
	result := make(types.Collection, 0, len(c))
	for _, item := range c {
		if !containsRef(result, item) {
			result = append(result, item)
		}
	}
	return result
}

func unionRef(c, other types.Collection) types.Collection {
	result := make(types.Collection, 0, len(c)+len(other))
	for _, item := range append(append(types.Collection{}, c...), other...) {
		if !containsRef(result, item) {
			result = append(result, item)
		}
	}
	return result
}

func intersectRef(c, other types.Collection) types.Collection {
	result := make(types.Collection, 0)
	for _, item := range c {
		if containsRef(other, item) && !containsRef(result, item) {
			result = append(result, item)
		}
	}
	return result
}

func excludeRef(c, other types.Collection) types.Collection {
	result := make(types.Collection, 0)
	for _, item := range c {
		if !containsRef(other, item) {
			result = append(result, item)
		}
	}
	return result
}

func subsetOfRef(c, other types.Collection) bool {
	for _, item := range c {
		if !containsRef(other, item) {
			return false
		}
	}
	return true
}

func supersetOfRef(c, other types.Collection) bool {
	for _, item := range other {
		if !containsRef(c, item) {
			return false
		}
	}
	return true
}

// maxReferenceItems es el tamaño de colección desde el cual la referencia,
// que compara todos los pares, es demasiado lenta para recorrer los ejemplos.
const maxReferenceItems = 300

// sameItem dice si dos elementos son el mismo: del mismo tipo Go y, donde el
// motor devuelve un elemento que recibió, con el mismo valor, la misma metadata
// y el mismo objeto. Lo que encontró un repeat() se compara por lo que imprime,
// porque cada evaluación lee sus propios objetos.
func sameItem(got, want types.Value, byIdentity bool) bool {
	if reflect.TypeOf(got) != reflect.TypeOf(want) ||
		got.Type() != want.Type() || got.String() != want.String() ||
		types.IsFHIRPrimitive(got) != types.IsFHIRPrimitive(want) {
		return false
	}
	if !byIdentity {
		return true
	}
	if reflect.TypeOf(got).Comparable() {
		return got == want
	}
	return reflect.DeepEqual(got, want)
}

func sameCollection(got, want types.Collection, byIdentity bool) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if !sameItem(got[i], want[i], byIdentity) {
			return false
		}
	}
	return true
}

// TestSetOperationsMatchTheReferenceOverTheExamples evalúa los operadores que
// buscan elementos iguales, con las funciones de conjunto que los respaldan,
// sobre los descendants() de cada ejemplo oficial de R4, R4B y R5, y espera la
// respuesta de la referencia de arriba elemento por elemento, en orden y con los
// mismos elementos: distinct(), isDistinct(), | consigo misma y con la mitad de
// sí misma, intersect(), exclude(), subsetOf(), supersetOf() y
// repeat(children()). Una colección de más de maxReferenceItems queda fuera,
// porque la referencia compara todos los pares.
//
// Descarga los packages de ejemplos la primera vez, como make corpusdiff, y se
// salta si eso falla. Pasa por todos los ejemplos, así que -short lo salta.
func TestSetOperationsMatchTheReferenceOverTheExamples(t *testing.T) {
	if testing.Short() {
		t.Skip("walks every official example; run without -short")
	}
	cache := filepath.Join("..", "build", "corpusdiff", "cache")
	names := make([]string, 0, len(versions))
	for name := range versions {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		version := versions[name]
		t.Run(name, func(t *testing.T) {
			dir, err := fetch(cache, version.examples)
			dir = fetchedOrSkip(t, dir, err)
			files, err := filepath.Glob(filepath.Join(dir, "*.json"))
			if err != nil {
				t.Fatal(err)
			}
			sort.Strings(files)
			model := fhirpath.WithModel(version.model())
			unlimited := fhirpath.WithMaxCollectionSize(math.MaxInt32)

			descendants := fhirpath.MustCompile("descendants()")
			children := fhirpath.MustCompile("%item.children()")
			root := fhirpath.MustCompile("%resource")
			repeated := fhirpath.MustCompile("repeat(children())")

			var resources, skipped, checks, differences int
			fail := func(file, what string) {
				differences++
				if differences <= 20 {
					t.Errorf("%s: %s", filepath.Base(file), what)
				}
			}
			for _, file := range files {
				data, err := os.ReadFile(file)
				if err != nil || len(data) > maxResource {
					continue
				}
				doc, err := fhirpath.NewDocument(data)
				if err != nil {
					continue
				}
				all, err := doc.EvaluateWithOptions(descendants, model, unlimited)
				if err != nil {
					continue
				}
				resources++
				if len(all) > maxReferenceItems {
					skipped++
					continue
				}
				half := all[:len(all)/2]
				// La otra mitad, para que los operandos difieran en las dos direcciones
				rest := all[len(all)/2:]

				evaluate := func(text string, args ...types.Collection) types.Collection {
					expr := fhirpath.MustCompile(text)
					opts := make([]fhirpath.EvalOption, 0, 2+len(args))
					opts = append(opts, model, unlimited)
					for i, arg := range args {
						opts = append(opts, fhirpath.WithVariable(string(rune('a'+i)), arg))
					}
					got, evalErr := doc.EvaluateWithOptions(expr, opts...)
					if evalErr != nil {
						t.Fatalf("%s: %s: %v", filepath.Base(file), text, evalErr)
					}
					return got
				}
				boolean := func(got types.Collection) bool {
					b, ok := got.SingletonBoolean()
					if !ok {
						t.Fatalf("%s: not a boolean: %s", filepath.Base(file), got)
					}
					return b
				}
				collections := func(label, text string, want types.Collection, args ...types.Collection) {
					checks++
					if got := evaluate(text, args...); !sameCollection(got, want, true) {
						fail(file, label+": "+text+" answers other items than the reference")
					}
				}
				booleans := func(text string, want bool, args ...types.Collection) {
					checks++
					if got := boolean(evaluate(text, args...)); got != want {
						fail(file, text+" = "+boolString(got)+", the reference "+boolString(want))
					}
				}

				collections("distinct", "%a.distinct()", distinctRef(all), all)
				booleans("%a.isDistinct()", len(distinctRef(all)) == len(all), all)
				collections("union", "%a | %a", unionRef(all, all), all)
				collections("union", "%a | %b", unionRef(all, half), all, half)
				collections("union", "%b | %a", unionRef(half, all), all, half)
				collections("union", "%a | %b", unionRef(half, rest), half, rest)
				collections("union", "%a.union(%b)", unionRef(rest, half), rest, half)
				collections("intersect", "%a.intersect(%b)", intersectRef(all, half), all, half)
				collections("intersect", "%b.intersect(%a)", intersectRef(half, all), all, half)
				collections("exclude", "%a.exclude(%b)", excludeRef(all, half), all, half)
				collections("exclude", "%b.exclude(%a)", excludeRef(half, all), all, half)
				booleans("%a.subsetOf(%b)", subsetOfRef(all, half), all, half)
				booleans("%b.subsetOf(%a)", subsetOfRef(half, all), all, half)
				booleans("%a.supersetOf(%b)", supersetOfRef(all, half), all, half)
				booleans("%b.supersetOf(%a)", supersetOfRef(half, all), all, half)

				// repeat(children()): la referencia avanza igual, pidiéndole al
				// motor solo los hijos de un elemento a la vez, y quita los
				// repetidos con la referencia
				want := repeatRef(t, doc, root, children, model, unlimited)
				checks++
				got, err := doc.EvaluateWithOptions(repeated, model, unlimited)
				if err != nil {
					t.Fatalf("%s: repeat(children()): %v", filepath.Base(file), err)
				}
				if !sameCollection(got, want, false) {
					fail(file, "repeat(children()) answers other items than the reference")
				}
			}
			t.Logf("%s: %d resources, %d above %d items left out, %d comparisons, %d differences",
				name, resources, skipped, maxReferenceItems, checks, differences)
			if resources == 0 || checks == 0 {
				t.Fatal("nothing was compared")
			}
		})
	}
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// repeatRef es repeat(children()) como era: cada vuelta aplica la proyección a
// los elementos que agregó la anterior, y un elemento entra al resultado solo
// si no hay uno igual en él.
func repeatRef(t *testing.T, doc *fhirpath.Document, root, children *fhirpath.Expression, opts ...fhirpath.EvalOption) types.Collection {
	t.Helper()
	current, err := doc.EvaluateWithOptions(root, opts...)
	if err != nil {
		t.Fatal(err)
	}
	result := types.Collection{}
	for len(current) > 0 {
		next := types.Collection{}
		for _, item := range current {
			produced, err := doc.EvaluateWithOptions(children,
				append([]fhirpath.EvalOption{fhirpath.WithVariable("item", types.Collection{item})}, opts...)...)
			if err != nil {
				t.Fatal(err)
			}
			for _, candidate := range produced {
				if containsRef(result, candidate) {
					continue
				}
				result = append(result, candidate)
				next = append(next, candidate)
			}
		}
		current = next
	}
	return result
}
