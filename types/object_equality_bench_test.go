package types

import (
	"fmt"
	"strings"
	"testing"
)

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

// longPatient writes a Patient of many keys, with a narrative full of escaped
// quotes first, whose last identifier is the value given.
func longPatient(last string) string {
	var b strings.Builder
	b.WriteString(`{"resourceType":"Patient","id":"p1","text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">`)
	for range 40 {
		b.WriteString(`<p class=\"note\">A paragraph of <b>narrative</b> text, with \"quotes\".</p>`)
	}
	b.WriteString(`</div>"}`)
	for i := range 60 {
		fmt.Fprintf(&b, `,"field%02d":{"system":"http://example.org/%d","value":"v%d","use":"official"}`, i, i, i)
	}
	fmt.Fprintf(&b, `,"last":"%s"}`, last)
	return b.String()
}

// nestedKeys writes objects eight deep whose members stand in the order given,
// and whose deepest object holds an element _x.
func nestedKeys(reversed bool) string {
	deepest := `{"y":1,"_x":{"id":"e"},"x":"v"}`
	text := deepest
	for range 8 {
		if reversed {
			text = `{"c":3,"n":` + text + `,"b":2,"a":1}`
		} else {
			text = `{"a":1,"b":2,"n":` + text + `,"c":3}`
		}
	}
	return text
}

// Plan 024: the check of plain text reads the whole text, which the negative
// shortcut did not.
func BenchmarkObjectEqualityPlainCheck(b *testing.B) {
	for _, pair := range []struct{ name, a, b string }{
		{"LongPatientsDifferLate", longPatient("one"), longPatient("two")},
		{"LongPatientsEqual", longPatient("one"), longPatient("one")},
		{"NestedElementKeysReordered", nestedKeys(false), nestedKeys(true)},
	} {
		x, y := object(pair.a), object(pair.b)
		b.Run(pair.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = x.Equal(y)
			}
		})
	}
}
