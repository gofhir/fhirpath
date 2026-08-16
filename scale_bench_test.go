package fhirpath

import (
	"fmt"
	"strings"
	"testing"
)

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

// The field read sits at the end of the object, which is what a scan per
// field access costs.
func BenchmarkScaleLastField(b *testing.B) {
	expr := MustCompile("Bundle.entry.resource.birthDate")
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
	expr := MustCompile("Bundle.entry.resource.name.given")
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
	expr := MustCompile("Bundle.entry.resource.where(active = true).name.family")
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

func validationExpressions() []*Expression {
	return []*Expression{
		MustCompile("Bundle.entry.resource.name.exists()"),
		MustCompile("Bundle.entry.resource.name.family.exists()"),
		MustCompile("Bundle.entry.resource.telecom.value.exists()"),
		MustCompile("Bundle.entry.resource.all(gender in ('male' | 'female' | 'other'))"),
		MustCompile("Bundle.entry.resource.birthDate.exists()"),
		MustCompile("Bundle.entry.count() > 0"),
		MustCompile("Bundle.entry.resource.active = true"),
		MustCompile("Bundle.entry.resource.id.exists()"),
	}
}
