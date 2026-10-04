package fhirpath_test

import (
	"sync"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

const indexedPatient = `{"resourceType":"Patient","id":"p","active":true,"gender":"female",` +
	`"birthDate":"1974-12-25","_birthDate":{"extension":[{"url":"u","valueCode":"x"}]},` +
	`"name":[{"family":"F","given":["A","B"]}],"birthDate":"1999-01-01"}`

// An object read more than once is indexed on its second read; every read
// after it answers from the index exactly as a scan would, the element beside
// a value and the first occurrence of a repeated key included.
func TestReadingAnIndexedObjectAnswersAsScanningIt(t *testing.T) {
	expr := "id | active.toString() | gender | birthDate.toString() | birthDate.extension.url | " +
		"name.family | name.given | id | gender | deceased.exists().toString()"
	want := "[p, true, female, 1974-12-25, u, F, A, B, false]"

	result, err := fhirpath.Evaluate([]byte(indexedPatient), expr)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != want {
		t.Errorf("one-shot: %s, want %s", got, want)
	}

	doc := fhirpath.MustNewDocument([]byte(indexedPatient))
	for i := 0; i < 3; i++ {
		result, err = doc.Evaluate(expr)
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != want {
			t.Errorf("Document pass %d: %s, want %s", i+1, got, want)
		}
	}
}

// A root a caller builds is not indexed, so it can be shared across
// goroutines as before; a root NewContext reads is each evaluation's own.
// Run with -race.
func TestIndexingDoesNotWriteToASharedRoot(t *testing.T) {
	shared, err := types.JSONToCollection([]byte(indexedPatient))
	if err != nil {
		t.Fatal(err)
	}
	expr := fhirpath.MustCompile("id | gender | name.family | name.given | birthDate.extension.url")

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if _, err := expr.EvaluateWithContext(eval.NewContextForRoot(shared)); err != nil {
					t.Error(err)
				}
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				if _, err := expr.Evaluate([]byte(indexedPatient)); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}

// The index keeps where each field lies in the object's JSON; every kind of
// value is read back from it exactly, and an object whose keys are written
// with escapes, which the index cannot place, is read by scanning instead.
func TestTheIndexReadsEveryKindOfValueBack(t *testing.T) {
	for _, tt := range []struct {
		resource, expr, want string
	}{
		{`{"resourceType":"Basic","a":"x\"y\\z","b":"","c":12.50,"d":{"e":[1,2]},"f":true,"g":null,"h":"é"}`,
			"a | b.exists().toString() | c.toString() | d.e.count().toString() | f.toString() | g.exists().toString() | h | a",
			`[x"y\z, true, 12.50, 2, false, é]`},
		{`{"resourceType":"Basic","family":"F","given":"G","family":"X"}`,
			"given | family | given | family", "[G, F]"},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			for _, doc := range []bool{false, true} {
				var result fhirpath.Collection
				var err error
				if doc {
					result, err = fhirpath.MustNewDocument([]byte(tt.resource)).Evaluate(tt.expr)
				} else {
					result, err = fhirpath.Evaluate([]byte(tt.resource), tt.expr)
				}
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s (document=%v) = %s, want %s", tt.expr, doc, got, tt.want)
				}
			}
		})
	}
}
