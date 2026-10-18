package fhirpath_test

import (
	"fmt"
	"testing"

	"github.com/gofhir/fhirpath"
)

// The field read sits at the end of the object, which is what a scan per
// field access costs.
func BenchmarkScaleLastField(b *testing.B) {
	expr := fhirpath.MustCompile("Bundle.entry.resource.birthDate")
	for _, n := range []int{10, 100, 500} {
		data := makeBundle(n)
		b.Run(fmt.Sprintf("entries=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = expr.Evaluate(data)
			}
		})
	}
}

// Deep navigation across every entry.
func BenchmarkScaleDeepNav(b *testing.B) {
	expr := fhirpath.MustCompile("Bundle.entry.resource.name.given")
	for _, n := range []int{10, 100, 500} {
		data := makeBundle(n)
		b.Run(fmt.Sprintf("entries=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = expr.Evaluate(data)
			}
		})
	}
}

// descendants() walks every node of the bundle, reading each object's children
// once, primitives paired with the elements beside them.
func BenchmarkScaleDescendants(b *testing.B) {
	expr := fhirpath.MustCompile("Bundle.descendants().count()")
	for _, n := range []int{10, 100, 500} {
		data := makeBundle(n)
		b.Run(fmt.Sprintf("entries=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = expr.Evaluate(data)
			}
		})
	}
}

// where() over the bundle: a predicate evaluated per element.
func BenchmarkScaleWhere(b *testing.B) {
	expr := fhirpath.MustCompile("Bundle.entry.resource.where(active = true).name.family")
	for _, n := range []int{10, 100, 500} {
		data := makeBundle(n)
		b.Run(fmt.Sprintf("entries=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_, _ = expr.Evaluate(data)
			}
		})
	}
}

// The validation case: several invariants over one resource, which is what
// pays for the document being read again per expression.
func BenchmarkValidationSuite(b *testing.B) {
	data := makeBundle(50)
	exprs := validationExpressions()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, e := range exprs {
			_, _ = e.Evaluate(data)
		}
	}
}
