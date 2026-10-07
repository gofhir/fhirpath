package fhirpath

import (
	"fmt"
	"strings"
	"testing"
)

// The operators that compare items to find one — union, distinct(), =, in —
// compare every pair, and almost every pair differs. Comparing content must
// cost those pairs nothing beyond what comparing bytes did; these hold it to
// that, on names written alike and on the same names written at another depth.

func namesPatient(n int, indented bool) []byte {
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf(`{"use":"official","family":"F%d","given":["G%d","H%d"]}`, i, i, i)
	}
	other := names[0]
	if indented {
		other = strings.ReplaceAll(strings.ReplaceAll(other, `,"`, ",\n      \""), `{"`, "{\n      \"")
	}
	return []byte(`{"resourceType":"Patient","name":[` + strings.Join(names, ",") +
		`],"contact":[{"name":` + other + `}]}`)
}

func benchmarkObjectEquality(b *testing.B, expr string, indented bool) {
	data := namesPatient(40, indented)
	compiled := MustCompile(expr)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := compiled.Evaluate(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkObjectUnion(b *testing.B) {
	benchmarkObjectEquality(b, "(Patient.name | Patient.name).count()", false)
}

func BenchmarkObjectDistinct(b *testing.B) {
	benchmarkObjectEquality(b, "Patient.name.distinct().count()", false)
}

func BenchmarkObjectEqual(b *testing.B) {
	benchmarkObjectEquality(b, "Patient.name = (Patient.name.tail() | Patient.name.first())", false)
}

func BenchmarkObjectEquivalent(b *testing.B) {
	benchmarkObjectEquality(b, "Patient.name ~ (Patient.name.tail() | Patient.name.first())", false)
}

func BenchmarkObjectInIndented(b *testing.B) {
	benchmarkObjectEquality(b, "Patient.contact.name in Patient.name", true)
}
