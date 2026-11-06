package types

import (
	"fmt"
	"testing"
)

// BenchmarkIndexThreshold mide Distinct con el índice siempre y con el
// recorrido siempre, a varios tamaños, para elegir indexFrom: el tamaño desde
// el que el índice gana. Los elementos son todos distintos, que es el caso en
// que el recorrido compara más.
func BenchmarkIndexThreshold(b *testing.B) {
	shapes := map[string]func(i int) Value{
		"string":  func(i int) Value { return NewString(fmt.Sprintf("text-%d", i)) },
		"integer": func(i int) Value { return NewInteger(int64(i)) },
		"object": func(i int) Value {
			return object(fmt.Sprintf(`{"code":{"coding":[{"system":"http://loinc.org","code":"c%d"}],"text":"t%d"},"valueQuantity":{"value":%d,"unit":"mg"}}`, i, i, i))
		},
		"small-object": func(i int) Value { return object(fmt.Sprintf(`{"use":"official","family":"F%d"}`, i)) },
		"quantity":     func(i int) Value { return mustQuantity(fmt.Sprintf("%d 'mg'", i)) },
	}
	saved := indexFrom
	defer func() { indexFrom = saved }()
	for name, build := range shapes {
		for _, n := range []int{8, 16, 24, 32, 48, 64} {
			c := make(Collection, n)
			for i := range c {
				c[i] = build(i)
			}
			for _, mode := range []struct {
				name string
				from int
			}{{"scan", 1 << 30}, {"index", 1}} {
				b.Run(fmt.Sprintf("%s/n=%d/%s", name, n, mode.name), func(b *testing.B) {
					indexFrom = mode.from
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if got := c.Distinct(); len(got) != n {
							b.Fatalf("%d items", len(got))
						}
					}
				})
			}
		}
	}
}
