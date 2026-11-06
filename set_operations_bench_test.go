package fhirpath_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// Las operaciones que buscan el igual de cada elemento -- | y union(),
// distinct(), intersect(), exclude(), subsetOf() -- sobre colecciones de n
// elementos (#243). Un resultado lineal en n lo muestra el cociente entre n =
// 1000 y n = 4000; los tamaños de alrededor del umbral del índice muestran lo
// que cuesta pasarlo.

// indexThresholdSizes son el umbral del índice (types.indexFrom) menos uno, el
// umbral y el umbral más uno. Cambian con él.
var indexThresholdSizes = []int{31, 32, 33}

// setBenchData es un recurso con n elementos de una forma, y las expresiones que
// los comparan.
type setBenchData struct {
	name  string
	build func(n int) []byte
	exprs map[string]string
	sizes []int
}

// setObservation escribe un Observation con n elementos en component, cada
// uno de la forma que da item.
func setObservation(n int, item func(i int) string) []byte {
	var b strings.Builder
	b.WriteString(`{"resourceType":"Observation","id":"o","status":"final","code":{"text":"c"},"component":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(item(i))
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

func setBenchDatasets() []setBenchData {
	all := append([]int{10, 100, 1000, 4000}, indexThresholdSizes...)
	big := []int{10, 100, 1000, 4000}
	return []setBenchData{
		{
			// elementos complejos, distintos entre sí
			name: "components",
			build: func(n int) []byte {
				return setObservation(n, func(i int) string {
					return fmt.Sprintf(`{"code":{"coding":[{"system":"http://loinc.org","code":"c%d"}],"text":"t%d"},"valueQuantity":{"value":%d,"unit":"mg"}}`, i, i, i)
				})
			},
			exprs: map[string]string{
				"union":    "Observation | Observation.component",
				"distinct": "Observation.component.distinct()",
				"exclude":  "Observation.component.exclude(Observation.component.tail())",
				"subset":   "Observation.component.subsetOf(Observation.component)",
			},
			sizes: all,
		},
		{
			// cadenas
			name: "strings",
			build: func(n int) []byte {
				return setObservation(n, func(i int) string {
					return fmt.Sprintf(`{"code":{"text":"t%d"}}`, i)
				})
			},
			exprs: map[string]string{
				"union":    "Observation.component.code.text | Observation.component.code.text",
				"distinct": "Observation.component.code.text.distinct()",
				"exclude":  "Observation.component.code.text.exclude(Observation.component.code.text.tail())",
				"subset":   "Observation.component.code.text.subsetOf(Observation.component.code.text)",
			},
			sizes: all,
		},
		{
			// objetos que se distinguen solo por el elemento de un primitivo: el
			// índice los pone en una sola cubeta, y se comparan entre sí como antes
			name: "one-bucket",
			build: func(n int) []byte {
				return setObservation(n, func(i int) string {
					return fmt.Sprintf(`{"code":{"text":"same"},"_note":{"id":"e%d"}}`, i)
				})
			},
			exprs: map[string]string{
				"union":    "Observation | Observation.component",
				"distinct": "Observation.component.distinct()",
			},
			sizes: big,
		},
		{
			// cantidades, que no tienen clave
			name: "quantities",
			build: func(n int) []byte {
				return setObservation(n, func(i int) string {
					return fmt.Sprintf(`{"valueQuantity":{"value":%d,"unit":"mg"}}`, i)
				})
			},
			exprs: map[string]string{
				"union":    "Observation.component.valueQuantity | Observation.component.valueQuantity",
				"distinct": "Observation.component.valueQuantity.distinct()",
			},
			sizes: big,
		},
	}
}

func BenchmarkSetOperations(b *testing.B) {
	for _, set := range setBenchDatasets() {
		for _, op := range []string{"union", "distinct", "exclude", "subset"} {
			text, ok := set.exprs[op]
			if !ok {
				continue
			}
			compiled := fhirpath.MustCompile(text)
			for _, n := range set.sizes {
				data := set.build(n)
				b.Run(fmt.Sprintf("%s/%s/n=%d", set.name, op, n), func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if _, err := compiled.Evaluate(data); err != nil {
							b.Fatal(err)
						}
					}
				})
			}
		}
	}
}
