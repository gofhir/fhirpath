package fhirpath_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

func benchmarkResolveChain(b *testing.B, restful bool) {
	data := chainBundle(500, restful)
	expr := fhirpath.MustCompile("Bundle.entry.resource.hasMember.resolve().count()")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := expr.Evaluate(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResolveBundleRESTful(b *testing.B) { benchmarkResolveChain(b, true) }

func BenchmarkResolveBundleURN(b *testing.B) { benchmarkResolveChain(b, false) }

func BenchmarkResolveBundleDocument(b *testing.B) {
	doc := fhirpath.MustNewDocument(chainBundle(500, true))
	expr := fhirpath.MustCompile("Bundle.entry.resource.hasMember.resolve().count()")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := doc.EvaluateCompiled(expr); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkResolveContained resolves references to contained resources, each
// from a resource that holds many.
func BenchmarkResolveContained(b *testing.B) {
	contained := make([]string, 50)
	for i := range contained {
		contained[i] = fmt.Sprintf(`{"resourceType":"Practitioner","id":"pr%d","name":[{"family":"F%d"}]}`, i, i)
	}
	refs := make([]string, 50)
	for i := range refs {
		refs[i] = fmt.Sprintf(`{"individual":{"reference":"#pr%d"}}`, i)
	}
	data := []byte(`{"resourceType":"Encounter","id":"e","status":"finished","contained":[` +
		strings.Join(contained, ",") + `],"participant":[` + strings.Join(refs, ",") + `]}`)
	expr := fhirpath.MustCompile("Encounter.participant.individual.resolve().name.family")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := expr.Evaluate(data); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkResolveContainedDeep resolves fragments written deep in a resource,
// below elements that hold no resource, as a Questionnaire's answer options do.
func BenchmarkResolveContainedDeep(b *testing.B) {
	items := make([]string, 200)
	for i := range items {
		items[i] = fmt.Sprintf(`{"linkId":"%d","text":"Question %d with some text","type":"group","item":[{"linkId":"%d.1","type":"group","item":[{"linkId":"%d.1.1","type":"choice","answerOption":[{"valueReference":{"reference":"#vs"}}]}]}]}`, i, i, i, i)
	}
	data := []byte(`{"resourceType":"Questionnaire","id":"q","status":"active","contained":[{"resourceType":"ValueSet","id":"vs","status":"active"}],"item":[` +
		strings.Join(items, ",") + `]}`)
	expr := fhirpath.MustCompile("Questionnaire.repeat(item).answerOption.value.resolve().id")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := expr.Evaluate(data); err != nil {
			b.Fatal(err)
		}
	}
}
