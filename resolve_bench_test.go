package fhirpath

import (
	"fmt"
	"strings"
	"testing"
)

// chainBundle writes n Observations each of which names the next as a member,
// with RESTful fullUrls or urn:uuid ones, as resolve() meets them over a
// Bundle.
func chainBundle(n int, restful bool) []byte {
	entries := make([]string, n)
	for i := range entries {
		fullURL, ref := fmt.Sprintf("http://example.org/fhir/Observation/o%d", i), fmt.Sprintf("Observation/o%d", i+1)
		if !restful {
			fullURL, ref = fmt.Sprintf("urn:uuid:00000000-0000-4000-8000-%012d", i), fmt.Sprintf("urn:uuid:00000000-0000-4000-8000-%012d", i+1)
		}
		entries[i] = fmt.Sprintf(`{"fullUrl":%q,"resource":{"resourceType":"Observation","id":"o%d","status":"final","code":{"text":"c"},"hasMember":[{"reference":%q}]}}`,
			fullURL, i, ref)
	}
	return []byte(`{"resourceType":"Bundle","type":"collection","entry":[` + strings.Join(entries, ",") + `]}`)
}

func benchmarkResolveChain(b *testing.B, restful bool) {
	data := chainBundle(500, restful)
	expr := MustCompile("Bundle.entry.resource.hasMember.resolve().count()")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := expr.Evaluate(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResolveBundleRESTful(b *testing.B) { benchmarkResolveChain(b, true) }
func BenchmarkResolveBundleURN(b *testing.B)     { benchmarkResolveChain(b, false) }

func BenchmarkResolveBundleDocument(b *testing.B) {
	doc := MustNewDocument(chainBundle(500, true))
	expr := MustCompile("Bundle.entry.resource.hasMember.resolve().count()")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := doc.EvaluateCompiled(expr); err != nil {
			b.Fatal(err)
		}
	}
}
