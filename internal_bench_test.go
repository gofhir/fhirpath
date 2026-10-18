package fhirpath

import (
	"html"
	"os"
	"regexp"
	"testing"
)

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

var expressionElement = regexp.MustCompile(`(?s)<expression[^>]*>(.*?)</expression>`)
