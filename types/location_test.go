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

// A read records the field and index an object sits at, and Location spells
// the path from them. They must say what finding the object in its parent's
// JSON says, for every way an object is read: a field, a field under a type,
// a choice variant, Get, and children read paired or unpaired, nulls and
// elements beside primitives included.
func TestARecordedPlaceIsWhereTheObjectSits(t *testing.T) {
	col, err := JSONToCollection([]byte(`{"resourceType":"Observation",` +
		`"code":{"coding":[{"system":"s","code":"a"},null,{"system":"s","code":"a"}]},` +
		`"valueQuantity":{"value":1,"unit":"mg"},` +
		`"component":[{"code":{"text":"x"},"valueString":"v","_valueString":{"id":"e"}},{"code":{"text":"x"}}],` +
		`"note":[{"text":"n","_text":{"extension":[{"url":"u","valueCode":"c"}]}}],` +
		`"_status":{"id":"s1"}}`))
	if err != nil {
		t.Fatal(err)
	}
	root := col[0].(*ObjectValue)

	checked := 0
	check := func(how string, v Value) {
		obj, ok := ElementOf(v)
		if !ok {
			return
		}
		recorded := obj.Location()
		field, position := obj.field, obj.position
		obj.field, obj.position = "", 0
		found := obj.Location()
		obj.field, obj.position = field, position
		if recorded != found || recorded == "" {
			t.Errorf("%s: recorded at %q, found at %q", how, recorded, found)
		}
		checked++
	}

	var walk func(obj *ObjectValue)
	walk = func(obj *ObjectValue) {
		for _, child := range obj.TypedChildren("", nil) {
			check("child of "+obj.Location(), child.Value)
			if next, ok := ElementOf(child.Value); ok {
				walk(next)
			}
		}
	}
	walk(root)

	for _, v := range root.GetCollection("code") {
		for _, coding := range v.(*ObjectValue).GetCollectionWithType("coding", "Coding") {
			check("coding", coding)
		}
	}
	for _, v := range root.GetChoiceCollection("value", []string{"String", "Quantity"}) {
		check("value", v)
	}
	for _, v := range root.GetCollection("status") {
		check("status", v)
	}
	if v, ok := root.Get("valueQuantity"); ok {
		check("Get", v)
	}
	for _, v := range root.Children() {
		check("Children", v)
	}

	if checked < 15 {
		t.Errorf("checked %d objects, expected the walk to reach more", checked)
	}
}
