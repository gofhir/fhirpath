package fhirpath_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// Compilation descends into a function's arguments, so anything it does twice
// per call is paid once per level of nesting — squared, then cubed. This is
// where that shows: select(select(select(...))) nine deep.
func BenchmarkCompileNestedArgs(b *testing.B) {
	for _, depth := range []int{1, 5, 9} {
		arg := "given"
		for i := 0; i < depth; i++ {
			arg = "select(" + arg + ")"
		}
		expr := "Patient.name." + arg

		b.Run(fmt.Sprintf("depth=%d", depth), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = fhirpath.Compile(expr)
			}
		})
	}
}

// The validation case over a Document, which reads the bundle once and shares
// it across the invariants.
func BenchmarkValidationSuiteDocument(b *testing.B) {
	data := makeBundle(50)
	exprs := validationExpressions()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc := fhirpath.MustNewDocument(data)
		for _, e := range exprs {
			_, _ = doc.EvaluateCompiled(e)
		}
	}
}

var patient = []byte(`{
	"resourceType": "Patient",
	"id": "example",
	"active": true,
	"name": [
		{
			"use": "official",
			"family": "Chalmers",
			"given": ["Peter", "James"]
		},
		{
			"use": "usual",
			"given": ["Jim"]
		}
	],
	"telecom": [
		{
			"system": "phone",
			"value": "(03) 5555 6473"
		}
	],
	"gender": "male",
	"birthDate": "1974-12-25"
}`)

func BenchmarkCompile(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = fhirpath.Compile("Patient.name.given")
	}
}

func BenchmarkEvaluateSimple(b *testing.B) {
	expr := fhirpath.MustCompile("Patient.id")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

func BenchmarkEvaluateNested(b *testing.B) {
	expr := fhirpath.MustCompile("Patient.name.given")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

func BenchmarkEvaluateWithFunction(b *testing.B) {
	expr := fhirpath.MustCompile("Patient.name.given.count()")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

func BenchmarkEvaluateComplex(b *testing.B) {
	expr := fhirpath.MustCompile("Patient.name.first().given.join(', ')")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

func BenchmarkEvaluateArithmetic(b *testing.B) {
	expr := fhirpath.MustCompile("2 + 3 * 4 - 1")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

func BenchmarkEvaluateString(b *testing.B) {
	expr := fhirpath.MustCompile("'Hello'.lower().startsWith('hel')")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

func BenchmarkEvaluateMath(b *testing.B) {
	expr := fhirpath.MustCompile("16.sqrt().power(2)")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

func BenchmarkDirectEvaluate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = fhirpath.Evaluate(patient, "Patient.name.given")
	}
}

func BenchmarkEvaluateBoolean(b *testing.B) {
	expr := fhirpath.MustCompile("true and false or true")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

func BenchmarkEvaluateComparison(b *testing.B) {
	expr := fhirpath.MustCompile("5 < 10 and 10 > 5")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

func BenchmarkEvaluateExists(b *testing.B) {
	expr := fhirpath.MustCompile("Patient.name.exists()")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

func BenchmarkEvaluateEmpty(b *testing.B) {
	expr := fhirpath.MustCompile("Patient.name.empty()")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

// What preparing an evaluation costs on its own: a trivial expression on a
// Document, with options as a validator passes them.
func BenchmarkEvaluationSetup(b *testing.B) {
	doc := fhirpath.MustNewDocument([]byte(`{"resourceType":"Patient","id":"p","active":true}`))
	expr := fhirpath.MustCompile("true")
	b.Run("with-options", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = doc.EvaluateWithOptions(expr, fhirpath.WithTimeout(0))
		}
	})
	b.Run("default-options", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = doc.EvaluateWithOptions(expr)
		}
	})
	b.Run("compiled", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = doc.EvaluateCompiled(expr)
		}
	})
}

// Benchmark compilation
func BenchmarkCompileWithFunction(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = fhirpath.Compile("Patient.name.given.first()")
	}
}

// Benchmark cached compilation
func BenchmarkCompileCached(b *testing.B) {
	cache := fhirpath.NewExpressionCache(100)
	for i := 0; i < b.N; i++ {
		_, _ = cache.Get("Patient.name.given.first()")
	}
}

// Benchmark evaluation
func BenchmarkEvaluate(b *testing.B) {
	patient := []byte(`{
		"resourceType": "Patient",
		"name": [{"given": ["John", "James"]}]
	}`)
	expr := fhirpath.MustCompile("Patient.name.given.first()")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

// Benchmark struct evaluation
func BenchmarkEvaluateResource(b *testing.B) {
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "bench",
		"name": [{"given": ["John", "James"]}]
	}`)

	expr := fhirpath.MustCompile("Patient.name.given.first()")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = expr.Evaluate(patient)
	}
}

// Location reads each ancestor's JSON to spell the path, so it costs about
// what reading the path again does: this is a result three levels into an
// entry of a Bundle of a hundred.
func BenchmarkLocation(b *testing.B) {
	entry := `{"resource":{"resourceType":"Patient","name":[{"family":"F","given":["A","B"]}],` +
		`"contact":[{"gender":"male"},{"gender":"male","name":{"family":"F","given":["A"]}}]}}`
	bundle := `{"resourceType":"Bundle","entry":[`
	for i := 0; i < 100; i++ {
		if i > 0 {
			bundle += ","
		}
		bundle += entry
	}
	bundle += `]}`

	col, err := types.JSONToCollection([]byte(bundle))
	if err != nil {
		b.Fatal(err)
	}
	result, err := fhirpath.MustCompile("Bundle.entry[50].resource.contact[1].name").EvaluateWithContext(eval.NewContextForRoot(col))
	if err != nil || len(result) != 1 {
		b.Fatal(err, result)
	}
	obj := result[0].(*types.ObjectValue)
	if got := obj.Location(); got != "Bundle.entry[50].resource.contact[1].name" {
		b.Fatal(got)
	}

	b.ReportAllocs()
	for b.Loop() {
		_ = obj.Location()
	}
}

// Locating every item of an array costs the depth of each path, not the size
// of the array: per entry, the time is the same for a Bundle of 1000 entries
// as for one of 8000.
func BenchmarkLocationOfEveryEntry(b *testing.B) {
	for _, n := range []int{1000, 8000} {
		b.Run(fmt.Sprintf("entries=%d", n), func(b *testing.B) {
			var bundle strings.Builder
			bundle.WriteString(`{"resourceType":"Bundle","entry":[`)
			for i := 0; i < n; i++ {
				if i > 0 {
					bundle.WriteByte(',')
				}
				fmt.Fprintf(&bundle, `{"fullUrl":"urn:uuid:%d","resource":{"resourceType":"Patient","id":"p%d"}}`, i, i)
			}
			bundle.WriteString(`]}`)

			col, err := types.JSONToCollection([]byte(bundle.String()))
			if err != nil {
				b.Fatal(err)
			}
			entries, err := fhirpath.MustCompile("Bundle.entry").EvaluateWithContext(eval.NewContextForRoot(col))
			if err != nil || len(entries) != n {
				b.Fatal(err, len(entries))
			}
			if got := entries[n-1].(*types.ObjectValue).Location(); got != fmt.Sprintf("Bundle.entry[%d]", n-1) {
				b.Fatal(got)
			}

			b.ReportAllocs()
			for b.Loop() {
				for _, entry := range entries {
					_ = entry.(*types.ObjectValue).Location()
				}
			}
			b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*n), "ns/entry")
		})
	}
}
