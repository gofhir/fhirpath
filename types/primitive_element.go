package types

// FHIR splits a primitive that carries extensions across two JSON fields: the
// value under its own name, and everything else about the element under the
// same name prefixed with an underscore.
//
//	"birthDate": "1974-12-25",
//	"_birthDate": {
//	  "extension": [{"url": ".../patient-birthTime", "valueDateTime": "..."}]
//	}
//
// The two are one element. FHIRPath navigates to birthDate and gets the date,
// but the extensions have to come with it — Patient.birthDate.extension(url) is
// the ordinary way to read a birth time, and the whole data-absent-reason
// pattern depends on reaching an extension on a value that is not there.
//
// So a primitive carries its element alongside its value. The value is what it
// compares and renders as; the element is what extension() and id read.
type primitiveElement struct {
	element *ObjectValue
}

// Element returns the FHIR element a primitive was read with, or nil when the
// primitive stood alone in the JSON.
func (p primitiveElement) Element() *ObjectValue {
	return p.element
}

// HasElement reports whether any element accompanied the value.
func (p primitiveElement) HasElement() bool {
	return p.element != nil
}

// ElementCarrier is implemented by the primitive types, which may carry the
// FHIR element their JSON representation keeps beside the value.
//
// Declared as an interface so that extension() and friends can ask any value
// for its element without knowing which primitive it is.
type ElementCarrier interface {
	Element() *ObjectValue
	HasElement() bool
}

// ElementOf returns the FHIR element accompanying a value, and whether there is
// one. An ObjectValue is its own element: a complex type keeps its extensions
// in the same object as the rest of its fields.
func ElementOf(value Value) (*ObjectValue, bool) {
	switch v := value.(type) {
	case *ObjectValue:
		return v, true
	case ElementCarrier:
		if v.HasElement() {
			return v.Element(), true
		}
	}
	return nil, false
}
