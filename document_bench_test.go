package fhirpath

import "testing"

// Kept apart from the other scale benchmarks because it calls API that earlier
// releases do not have: bench-compare hands this tree's benchmark files to the
// revision it measures against, and a file that revision cannot build is left
// out. On its own, this one costs the comparison nothing.

// The validation case over a Document, which reads the bundle once and shares
// it across the invariants.
func BenchmarkValidationSuiteDocument(b *testing.B) {
	data := makeBundle(50)
	exprs := validationExpressions()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc := MustNewDocument(data)
		for _, e := range exprs {
			_, _ = doc.EvaluateCompiled(e)
		}
	}
}
