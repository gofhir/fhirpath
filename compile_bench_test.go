package fhirpath

import (
	"fmt"
	"html"
	"os"
	"testing"
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
				_, _ = Compile(expr)
			}
		})
	}
}

// BenchmarkCompileSuite compiles every expression of the official R4 suite,
// with no expression cache, as a caller does that evaluates each only once.
func BenchmarkCompileSuite(b *testing.B) {
	data, err := os.ReadFile("conformance/testdata/fhirpath-suite/tests-fhir-r4.xml")
	if err != nil {
		b.Skip(err)
	}
	matches := expressionElement.FindAllSubmatch(data, -1)
	exprs := make([]string, 0, len(matches))
	for _, m := range matches {
		exprs = append(exprs, html.UnescapeString(string(m[1])))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expr := range exprs {
			_, _ = Compile(expr)
		}
	}
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(exprs)), "ns/expr")
}
