package types

import "testing"

// Pares que las rutas rápidas dejan al lector: las claves en otro orden y un
// número escrito de otra forma. El Plan 023 (#265) cambió cómo el lector empareja los
// nombres de dos objetos.
var objectEqualityPairs = []struct {
	name, a, b string
}{
	{"NoElementsEqual",
		`{"system":"http://loinc.org","code":"1234-5","display":"A coded value","version":"2.0","userSelected":true,"rank":1}`,
		`{"rank":1.0,"userSelected":true,"version":"2.0","display":"A coded value","code":"1234-5","system":"http://loinc.org"}`},
	{"NoElementsUnequal",
		`{"system":"http://loinc.org","code":"1234-5","display":"A coded value","version":"2.0","userSelected":true,"rank":1}`,
		`{"rank":2.0,"userSelected":true,"version":"2.0","display":"A coded value","code":"1234-5","system":"http://loinc.org"}`},
	{"ElementsEqual",
		`{"family":"Smith","given":["John","Q"],"_given":[{"id":"g1"},null],"use":"official","text":"John Q Smith"}`,
		`{"text":"John Q Smith","use":"official","_given":[{"id":"g1"},null],"given":["John","Q"],"family":"Smith"}`},
	{"ElementsUnequal",
		`{"family":"Smith","given":["John","Q"],"_given":[{"id":"g1"},null],"use":"official","text":"John Q Smith"}`,
		`{"text":"John Q Smith","use":"official","_given":[{"id":"g2"},null],"given":["John","Q"],"family":"Smith"}`},
}

func BenchmarkObjectEquality(b *testing.B) {
	for _, pair := range objectEqualityPairs {
		x, y := object(pair.a), object(pair.b)
		b.Run("Equal/"+pair.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = x.Equal(y)
			}
		})
		b.Run("Equivalent/"+pair.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = x.Equivalent(y)
			}
		})
	}
}
