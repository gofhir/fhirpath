package fhirpath

import "testing"

// What preparing an evaluation costs on its own: a trivial expression on a
// Document, with options as a validator passes them.
func BenchmarkEvaluationSetup(b *testing.B) {
	doc := MustNewDocument([]byte(`{"resourceType":"Patient","id":"p","active":true}`))
	expr := MustCompile("true")
	b.Run("with-options", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = doc.EvaluateWithOptions(expr, WithTimeout(0))
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
