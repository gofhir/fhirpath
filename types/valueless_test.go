package types

import "testing"

// An object a caller types as a FHIR primitive is the element of one written
// with no value, and keeps that as the root; the element beside a primitive
// that has a value is no item, and is not valueless.
func TestValuelessObjects(t *testing.T) {
	for _, typeName := range []string{"string", "date", "FHIR.string", "String", "http://hl7.org/fhirpath/System.String"} {
		o := NewObjectValueWithType([]byte(`{"id":"x"}`), typeName)
		if !o.Valueless() {
			t.Errorf("NewObjectValueWithType(%q).Valueless() = false, want true", typeName)
		}
		root := asRoot(Collection{o})[0].(*ObjectValue)
		if !root.Valueless() || root.role&roleRoot == 0 {
			t.Errorf("%q as the root: valueless %v, root %v; want both", typeName, root.Valueless(), root.role&roleRoot != 0)
		}
	}
	for _, typeName := range []string{"CodeableConcept", "Quantity", "BackboneElement"} {
		if NewObjectValueWithType([]byte(`{"id":"x"}`), typeName).Valueless() {
			t.Errorf("NewObjectValueWithType(%q).Valueless() = true, want false", typeName)
		}
	}

	name := NewObjectValue([]byte(`{"family":"Smith","_family":{"id":"x"}}`))
	family := name.GetCollection("family")
	if len(family) != 1 {
		t.Fatalf("family = %v, want one item", family)
	}
	if element, ok := ElementOf(family[0]); !ok || element.Valueless() || IsValueless(element) {
		t.Errorf("the element beside \"Smith\" reads as valueless")
	}

	valueless := NewObjectValue([]byte(`{"_family":{"id":"x"}}`)).GetCollection("family")
	if len(valueless) != 1 || !IsValueless(valueless[0]) {
		t.Errorf("\"_family\" with no value = %v, want one valueless item", valueless)
	}
}
