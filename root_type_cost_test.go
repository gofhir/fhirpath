package fhirpath_test

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// A root read once and shared between evaluations does not write what it
// works out, so it worked its type out each time it was asked: for a Bundle
// whose resourceType comes after its entries, as Go's encoding/json writes a
// map, by reading past every entry. %resource is Bundle, asked once per entry,
// then cost the size of the Bundle per entry. A root read without a declared
// type has its type worked out when it is read, so per entry a Bundle of 4000
// costs about what one of 500 does, wherever its resourceType is. The ratio is
// of the best of several passes, so it holds on a slow or busy machine, and a
// margin of three keeps noise from failing it while catching a cost that
// grows with the Bundle.
func TestASharedRootIsTypedOnceWhereverItsResourceTypeIs(t *testing.T) {
	compiled := fhirpath.MustCompile("Bundle.entry.where(%resource is Bundle).count()")

	perEntry := func(n int, last bool) time.Duration {
		var bundle strings.Builder
		bundle.WriteString("{")
		if !last {
			bundle.WriteString(`"resourceType":"Bundle",`)
		}
		bundle.WriteString(`"entry":[`)
		for i := 0; i < n; i++ {
			if i > 0 {
				bundle.WriteByte(',')
			}
			fmt.Fprintf(&bundle, `{"fullUrl":"urn:uuid:%d","resource":{"resourceType":"Patient","id":"p%d"}}`, i, i)
		}
		bundle.WriteString("]")
		if last {
			bundle.WriteString(`,"resourceType":"Bundle"`)
		}
		bundle.WriteString("}")

		root, err := types.JSONToCollection([]byte(bundle.String()))
		if err != nil {
			t.Fatal(err)
		}

		best := time.Duration(math.MaxInt64)
		for range 10 {
			start := time.Now()
			result, err := compiled.EvaluateWithContext(eval.NewContextForRoot(root))
			elapsed := time.Since(start)
			if err != nil || result.String() != fmt.Sprintf("[%d]", n) {
				t.Fatal(err, result)
			}
			best = min(best, elapsed)
			if elapsed > 2*time.Second {
				break // already far past linear; no need to wait out the rest
			}
		}
		return best / time.Duration(n)
	}

	for _, last := range []bool{false, true} {
		small, large := perEntry(500, last), perEntry(4000, last)
		ratio := float64(large) / float64(max(small, 1))
		t.Logf("resourceType last=%v: %v per entry at 500, %v at 4000 (%.1fx)", last, small, large, ratio)
		if ratio > 3 {
			t.Errorf("resourceType last=%v: an entry of a Bundle of 4000 costs %.1fx what one of 500 does, want about the same", last, ratio)
		}
	}
}

// A root read without a declared type is typed when it is read, as Type()
// would have typed it, and one read under a declared type keeps that type.
func TestARootReadWithoutATypeIsTypedAsItWouldHaveBeen(t *testing.T) {
	for _, tt := range []struct {
		json, fhirType, want string
	}{
		{`{"resourceType":"Patient","id":"a"}`, "", "Patient"},
		{`{"id":"a","resourceType":"Patient"}`, "", "Patient"},
		{`{"value":5,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}`, "", "Quantity"},
		{`{"value":5,"unit":"mg"}`, "SimpleQuantity", "SimpleQuantity"},
		{`{"resourceType":"Patient"}`, "DomainResource", "Patient"},
	} {
		readers := map[string]func([]byte) (types.Collection, error){
			"JSONToCollection": func(data []byte) (types.Collection, error) {
				return types.JSONToCollectionWithType(data, tt.fhirType)
			},
			"ReadRoot": func(data []byte) (types.Collection, error) {
				return types.ReadRootWithType(data, tt.fhirType)
			},
		}
		for how, read := range readers {
			col, err := read([]byte(tt.json))
			if err != nil {
				t.Fatal(err)
			}
			if got := col[0].Type(); got != tt.want {
				t.Errorf("%s(%s, %q) is a %s, want %s", how, tt.json, tt.fhirType, got, tt.want)
			}
		}
	}
}
