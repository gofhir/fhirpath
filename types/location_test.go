package types

import (
	"testing"
	"unsafe"
)

// Navigation creates an ObjectValue for every object it passes, so the struct
// is allocated more than anything else the engine makes. It fits the 128-byte
// size class, and where an object sits costs no more than the room that left;
// a field that takes it past 128 costs every object 16 bytes more.
func TestObjectValueFitsItsSizeClass(t *testing.T) {
	if size := unsafe.Sizeof(ObjectValue{}); size > 128 {
		t.Errorf("ObjectValue is %d bytes, past the 128-byte size class", size)
	}
}

// An object is found in its parent by where its JSON starts, not by what it
// holds, so equal siblings are told apart, and a primitive's element is named
// as the primitive is.
func TestLocateFindsTheFieldByPosition(t *testing.T) {
	col, err := JSONToCollection([]byte(`{"resourceType":"Patient","a":{"x":1},"b":[{"x":1},null,{"x":1}],"_c":[null,{"x":1}]}`))
	if err != nil {
		t.Fatal(err)
	}
	root := col[0].(*ObjectValue)

	tests := []struct {
		field string
		want  []string
	}{
		{"a", []string{"Patient.a"}},
		{"b", []string{"Patient.b[0]", "Patient.b[2]"}},
		{"c", []string{"Patient.c[1]"}},
	}
	for _, tt := range tests {
		got := root.GetCollection(tt.field)
		if len(got) != len(tt.want) {
			t.Fatalf("%s: got %d values, want %d", tt.field, len(got), len(tt.want))
		}
		for i, v := range got {
			element, _ := ElementOf(v)
			if loc := element.Location(); loc != tt.want[i] {
				t.Errorf("%s[%d] is at %q, want %q", tt.field, i, loc, tt.want[i])
			}
		}
	}

	// An object no read created is no input's root.
	if loc := NewObjectValue([]byte(`{"resourceType":"Patient"}`)).Location(); loc != "" {
		t.Errorf("a constructed object is at %q, want \"\"", loc)
	}
}
