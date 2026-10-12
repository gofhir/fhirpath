package types

import "testing"

// TestHoldsNoValueReadsUpToTheFirstValue checks the read HoldsNoValue makes:
// it stops at the first value it meets, so what follows that value is never
// read and may be anything, and what holds no value it reads to the end.
func TestHoldsNoValueReadsUpToTheFirstValue(t *testing.T) {
	for _, data := range []string{
		`{"entry":[{"resource":{"resourceType":"Patient","id":"p"}}, not json`,
		`[{}, null, "x", not json`,
		`{"resourceType":"Patient","_a":{"b":[]},"c":1 not json`,
	} {
		if HoldsNoValue(NewObjectValue([]byte(data))) {
			t.Errorf("%s holds no value, want a value", data)
		}
	}
	for _, data := range []string{`{}`, `{"a":{"b":[null,{}]},"resourceType":"Patient"}`, `{ "a" : [ ] , "b" : { } }`} {
		if !HoldsNoValue(NewObjectValue([]byte(data))) {
			t.Errorf("%s holds a value, want none", data)
		}
	}
	// Nor is what does not parse known to hold nothing
	for _, data := range []string{`{"a":{}`, `{"a" {}}`, `{"a":nul}`} {
		if HoldsNoValue(NewObjectValue([]byte(data))) {
			t.Errorf("%s holds no value, want it not known to", data)
		}
	}
}
