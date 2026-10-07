package types

import (
	"fmt"
	"testing"
)

// TestLookupFindsWhatContainsFinds checks that fingerprinting changes no
// answer: on either side of the size from which a Lookup fingerprints, it
// finds an item exactly when Collection.Contains does, objects laid out apart
// and values of other kinds included.
func TestLookupFindsWhatContainsFinds(t *testing.T) {
	for _, size := range []int{1, lookupFingerprintFrom - 1, lookupFingerprintFrom, 40} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			var items Collection
			for i := range size {
				items = append(items, NewObjectValue(fmt.Appendf(nil, `{"a":%d,"b":"x"}`, i)), NewInteger(int64(i)))
			}
			lookup := NewLookup(items)

			for _, probe := range []Value{
				NewObjectValue([]byte(`{"a":0,"b":"x"}`)),
				NewObjectValue([]byte("{\n  \"a\" : 0,\n  \"b\" : \"x\"\n}")),
				NewObjectValue(fmt.Appendf(nil, `{ "a" : %d , "b" : "x" }`, size-1)),
				NewObjectValue([]byte(`{"b":"x","a":0}`)),
				NewObjectValue([]byte(`{"a":1,"b":"y"}`)),
				NewObjectValue([]byte(`{"a":10,"b":"x"}`)),
				NewInteger(0),
				NewInteger(int64(size)),
				NewString("x"),
			} {
				if got, want := lookup.Contains(probe), items.Contains(probe); got != want {
					t.Errorf("Contains(%s) = %v, want %v", probe, got, want)
				}
			}
			if got := lookup.Items(); len(got) != len(items) {
				t.Errorf("Items() holds %d, want %d", len(got), len(items))
			}
		})
	}
}
