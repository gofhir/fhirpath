package types

import (
	"fmt"
	"testing"
)

// TestLookupFindsWhatContainsFinds checks that measuring changes no answer:
// on either side of the size from which a Lookup measures its objects, and
// before and after it has been searched often enough to, it
// finds an item exactly when Collection.Contains does, objects laid out apart
// and values of other kinds included.
func TestLookupFindsWhatContainsFinds(t *testing.T) {
	for _, size := range []int{1, lookupMeasureFrom - 1, lookupMeasureFrom, 40} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			var items Collection
			for i := range size {
				items = append(items, NewObjectValue(fmt.Appendf(nil, `{"a":%d,"b":"x"}`, i)), NewInteger(int64(i)))
			}
			lookup := NewLookup(items)

			probes := []Value{
				NewObjectValue([]byte(`{"a":0,"b":"x"}`)),
				NewObjectValue([]byte("{\n  \"a\" : 0,\n  \"b\" : \"x\"\n}")),
				NewObjectValue(fmt.Appendf(nil, `{ "a" : %d , "b" : "x" }`, size-1)),
				NewObjectValue([]byte(`{"b":"x","a":0}`)),
				NewObjectValue([]byte(`{"a":1,"b":"y"}`)),
				NewObjectValue([]byte(`{"a":10,"b":"x"}`)),
				NewInteger(0),
				NewInteger(int64(size)),
				NewString("x"),
			}
			// Twice over, so that the second round searches a measured Lookup
			for round := range 2 {
				for _, probe := range probes {
					if got, want := lookup.Contains(probe), items.Contains(probe); got != want {
						t.Errorf("round %d: Contains(%s) = %v, want %v", round, probe, got, want)
					}
				}
			}
			if size >= lookupMeasureFrom && lookup.prints == nil {
				t.Error("searched often enough, the Lookup did not measure its objects")
			}
			if got := lookup.Items(); len(got) != len(items) {
				t.Errorf("Items() holds %d, want %d", len(got), len(items))
			}
		})
	}
}

// TestLookupAddsAfterReading checks that a Lookup made over a collection does
// not write into it when it holds more.
func TestLookupAddsAfterReading(t *testing.T) {
	backing := make(Collection, 2, 4)
	backing[0], backing[1] = NewInteger(1), NewInteger(2)
	lookup := NewLookup(backing)
	lookup.Add(NewInteger(3))
	if extended := backing[:3]; extended[2] != nil {
		t.Errorf("Add wrote into the collection the Lookup was made over: %v", extended)
	}
	if len(lookup.Items()) != 3 {
		t.Errorf("Items() holds %d, want 3", len(lookup.Items()))
	}
}
