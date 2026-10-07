package fhirpath

import (
	"fmt"
	"strings"
	"testing"
)

// The operators that compare items to find one — union, distinct(), =, in —
// compare every pair, and almost every pair differs, so a differing pair must
// stay about as cheap as comparing bytes made it. These measure that, on names
// written alike, on the same names written at another depth, and on items
// whose first keys differ. types/equality_bench_test.go measures large objects.

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

// questionnaireItems writes n items as a serializer does, half of them with an
// extension, which comes first: pairs differ from their first key on, and
// every comparison over the collection meets them.
func questionnaireItems(n int) []byte {
	items := make([]string, n)
	for i := range items {
		item := fmt.Sprintf(`"linkId":"%d","text":"Question %d","type":"string"`, i, i)
		if i%2 == 0 {
			item = `"extension":[{"url":"http://example.org/hidden","valueBoolean":true}],` + item
		}
		items[i] = "{" + item + "}"
	}
	return []byte(`{"resourceType":"Questionnaire","status":"active","item":[` + strings.Join(items, ",") + `]}`)
}

func benchmarkQuestionnaire(b *testing.B, expr string) {
	data := questionnaireItems(300)
	compiled := MustCompile(expr)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := compiled.Evaluate(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkObjectRepeatMixedKeys(b *testing.B) {
	benchmarkQuestionnaire(b, "Questionnaire.repeat(item).count()")
}

func BenchmarkObjectIntersectMixedKeys(b *testing.B) {
	benchmarkQuestionnaire(b, "Questionnaire.item.intersect(Questionnaire.item.tail()).count()")
}
