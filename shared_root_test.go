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

// What an evaluation returns is handed to the caller, who may share it across
// goroutines as a root of its own: Bundle.entry.resource read once, each
// resource then validated in parallel. Objects private while the evaluation
// read them are not private once returned. Run with -race.
func TestAReturnedObjectCanBeSharedAcrossGoroutines(t *testing.T) {
	bundle := []byte(`{"resourceType":"Bundle","type":"collection","entry":[` +
		`{"resource":{"resourceType":"Patient","id":"p","gender":"female","name":[{"family":"F"}],` +
		`"extension":[{"url":"http://example.org/e","valueCode":"x"}]}}]}`)
	resources, err := fhirpath.MustCompile("entry.resource").Evaluate(bundle)
	if err != nil || len(resources) != 1 {
		t.Fatalf("entry.resource = %v, %v", resources, err)
	}

	exprs := []*fhirpath.Expression{
		fhirpath.MustCompile("id | gender | name.family"),
		fhirpath.MustCompile("$this is Patient"),
		fhirpath.MustCompile("extension('http://example.org/e').value"),
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				for _, expr := range exprs {
					if _, err := expr.EvaluateWithContext(eval.NewContextForRoot(resources)); err != nil {
						t.Error(err)
					}
				}
			}
		}()
	}
	wg.Wait()
}

// An expression can return the root it was given — $this, a where() keeping
// it, %resource — and a shared root comes back shared without being written
// to. Run with -race.
func TestReturningASharedRootDoesNotWriteToIt(t *testing.T) {
	shared, err := types.JSONToCollection([]byte(`{"resourceType":"Patient","id":"p","active":true}`))
	if err != nil {
		t.Fatal(err)
	}
	exprs := []*fhirpath.Expression{
		fhirpath.MustCompile("$this"),
		fhirpath.MustCompile("%resource"),
		fhirpath.MustCompile("where(active)"),
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
