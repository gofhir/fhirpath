package fhirpath_test

import (
	"sync"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// A root a caller reads once and evaluates against from several goroutines is
// only read, never written: it does not cache, and only an object that caches
// is documented as single-goroutine. Working out its type and reading a field
// through Get used to write to it. Run with -race.
func TestASharedRootCanBeReadFromSeveralGoroutines(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","id":"p","gender":"female",` +
		`"extension":[{"url":"http://example.org/e","valueCode":"x"}],` +
		`"name":[{"family":"F","given":["A","B"]}],"managingOrganization":{"reference":"#o"}}`)
	shared, err := types.JSONToCollection(patient)
	if err != nil {
		t.Fatal(err)
	}

	exprs := []*fhirpath.Expression{
		fhirpath.MustCompile("id | gender | name.family | name.given"),
		fhirpath.MustCompile("Patient.name.given.count()"),
		fhirpath.MustCompile("extension('http://example.org/e').value"),
		fhirpath.MustCompile("$this is Patient"),
	}

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				for _, expr := range exprs {
					if _, err := expr.EvaluateWithContext(eval.NewContextForRoot(shared)); err != nil {
						t.Error(err)
					}
				}
			}
		}()
	}
	wg.Wait()
}
