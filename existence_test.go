package fhirpath_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofhir/fhirpath"
)

// TestBooleanAggregatesRequireBooleans covers the input allTrue, anyTrue,
// allFalse and anyFalse are each defined over: "Takes a collection of Boolean
// values".
//
// A non-Boolean item is not a false one. Reading it as false would answer
// (true | 'foo').allTrue() with a confident no, when the collection was never
// the kind of thing the function accepts.
func TestBooleanAggregatesRequireBooleans(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	t.Run("a non-Boolean item is an error", func(t *testing.T) {
		for _, expr := range []string{
			"(true | 'foo').allTrue()",
			"(true | 'foo').anyTrue()",
			"(false | 1).allFalse()",
			"(false | 1).anyFalse()",
		} {
			if _, err := fhirpath.MustCompile(expr).Evaluate(patient); err == nil {
				t.Errorf("%s: expected an error", expr)
			}
		}
	})

	t.Run("collections of Booleans still answer", func(t *testing.T) {
		cases := []struct{ expr, want string }{
			{"(true | true).allTrue()", "true"},
			{"(true | false).allTrue()", "false"},
			{"(true | false).anyTrue()", "true"},
			{"(false | false).allFalse()", "true"},
			{"(true | false).anyFalse()", "true"},
			{"{}.allTrue()", "true"},
			{"{}.anyTrue()", "false"},
		}
		for _, tc := range cases {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		}
	})
}

// TestExistsReadsAResourceOnlyUpToAValue checks that exists() on a resource
// costs no more than reading it up to its first value. A Bundle that writes
// its entries before its type would otherwise be read whole for each entry
// here, and 20000 entries took minutes.
func TestExistsReadsAResourceOnlyUpToAValue(t *testing.T) {
	if testing.Short() {
		t.Skip("reads a large Bundle")
	}
	var bundle strings.Builder
	bundle.WriteString(`{"resourceType":"Bundle","entry":[`)
	const entries = 20000
	for i := 0; i < entries; i++ {
		if i > 0 {
			bundle.WriteString(",")
		}
		fmt.Fprintf(&bundle, `{"resource":{"resourceType":"Patient","id":"p%d"}}`, i)
	}
	bundle.WriteString(`],"type":"collection"}`)

	start := time.Now()
	result, err := fhirpath.Evaluate([]byte(bundle.String()), "Bundle.entry.where(%resource.exists()).count()")
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != fmt.Sprintf("[%d]", entries) {
		t.Errorf("count = %s, want %d", got, entries)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("took %v: exists() reads the whole Bundle for each entry", elapsed)
	}
}
