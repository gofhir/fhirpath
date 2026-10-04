package fhirpath

import (
	"fmt"
	"sync"
	"testing"

	"github.com/gofhir/fhirpath/funcs"
)

// Evaluate compiles an expression once and reuses it, as EvaluateCached does:
// the convenience function is what the README starts with, and compiling on
// every call cost several times the evaluation.
func TestEvaluateReusesTheCompiledExpression(t *testing.T) {
	expr := "Patient.name.where(use = 'official').given.first()"
	before := DefaultCache.Stats()
	for i := 0; i < 5; i++ {
		if _, err := Evaluate([]byte(`{"resourceType":"Patient"}`), expr); err != nil {
			t.Fatal(err)
		}
	}
	after := DefaultCache.Stats()
	if hits := after.Hits - before.Hits; hits < 4 {
		t.Errorf("five evaluations of one expression hit the cache %d times, want at least 4", hits)
	}
	if _, err := Evaluate([]byte(`{}`), "Patient.("); err == nil {
		t.Error("an expression that does not compile evaluated without an error")
	}
}

// Both caches are read from many goroutines at once — a validator evaluates
// constraints in parallel — and a hit is a read. Run with -race.
func TestCachesCanBeHitFromSeveralGoroutines(t *testing.T) {
	exprs := NewExpressionCache(4)
	regexes := funcs.NewRegexCache(4, 1000, 0)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				// Mostly hits, with misses past the limit to make it evict.
				n := i % 6
				if _, err := exprs.Get(fmt.Sprintf("Patient.id.count() = %d", n)); err != nil {
					t.Error(err)
				}
				if _, err := regexes.Compile(fmt.Sprintf("^a{%d}$", n+1)); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()

	if size := exprs.Size(); size > 4 {
		t.Errorf("expression cache holds %d entries over a limit of 4", size)
	}
	if size := regexes.Size(); size > 4 {
		t.Errorf("regex cache holds %d entries over a limit of 4", size)
	}
}
