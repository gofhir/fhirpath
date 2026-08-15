package conformance

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4"
)

// These benchmarks live here because they need a FHIR model, and the model
// packages are what this module exists to keep out of the engine's go.mod.
//
// A model is not a detail of configuration: with one, navigation resolves each
// element's type through it and reads fields under a type hint, which is a
// different path through the engine than the one the benchmarks in the root
// module measure. It is also the path a validator takes.

// makeBundle builds a Bundle with n Patient entries.
func makeBundle(n int) []byte {
	var b strings.Builder
	b.WriteString(`{"resourceType":"Bundle","type":"searchset","entry":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"fullUrl":"urn:uuid:%d","resource":{"resourceType":"Patient","id":"p%d","active":true,`+
			`"name":[{"use":"official","family":"Fam%d","given":["Given%d","Middle%d"]}],`+
			`"telecom":[{"system":"phone","value":"555-%04d"}],"gender":"male","birthDate":"1974-12-25"}}`,
			i, i, i, i, i, i)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

// invariants stands for what a validator runs over one resource.
func invariants() []*fhirpath.Expression {
	return []*fhirpath.Expression{
		fhirpath.MustCompile("Bundle.entry.resource.name.exists()"),
		fhirpath.MustCompile("Bundle.entry.resource.name.family.exists()"),
		fhirpath.MustCompile("Bundle.entry.resource.telecom.value.exists()"),
		fhirpath.MustCompile("Bundle.entry.resource.all(gender in ('male' | 'female' | 'other'))"),
		fhirpath.MustCompile("Bundle.entry.resource.birthDate.exists()"),
		fhirpath.MustCompile("Bundle.entry.count() > 0"),
		fhirpath.MustCompile("Bundle.entry.resource.active = true"),
		fhirpath.MustCompile("Bundle.entry.resource.id.exists()"),
	}
}

// BenchmarkModelValidation evaluates the invariants the way a validator does
// today: each against the resource, which reads it again each time.
func BenchmarkModelValidation(b *testing.B) {
	data := makeBundle(50)
	exprs := invariants()
	model := fhirpath.WithModel(r4.FHIRPathModel())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, expr := range exprs {
			if _, err := expr.EvaluateWithOptions(data, model); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkModelValidationDocument evaluates the same invariants against one
// reading of the resource.
func BenchmarkModelValidationDocument(b *testing.B) {
	data := makeBundle(50)
	exprs := invariants()
	model := fhirpath.WithModel(r4.FHIRPathModel())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		doc, err := fhirpath.NewDocument(data)
		if err != nil {
			b.Fatal(err)
		}
		for _, expr := range exprs {
			if _, err := doc.EvaluateWithOptions(expr, model); err != nil {
				b.Fatal(err)
			}
		}
	}
}
