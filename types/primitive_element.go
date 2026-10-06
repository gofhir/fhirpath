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

// IsFHIRPrimitive reports whether a value is a primitive FHIR declares rather
// than a System value: "FHIR.string is a different type to System.String". A
// FHIR primitive has the value property, and hasValue() and getValue() read
// it; a System value has no properties.
//
// What tells them apart is where the value came from. One read from the
// resource is a FHIR primitive, whether or not a model gave it a type, and
// even where the model types it a System one, as R4 types Resource.id and
// Extension.url, whose FHIR type structuredefinition-fhir-type names; one an
// expression wrote, a literal, or a function computed is a System value. A
// value built with the constructors of this package is a System value, NewString
// and the rest, unless it is built with a FHIR type or an element.
func IsFHIRPrimitive(value Value) bool {
	var read bool
	switch v := value.(type) {
	case String:
		read = v.read
	case Boolean:
		read = v.read
	case Integer:
		read = v.read
	case Decimal:
		read = v.read
	case Date:
		read = v.read
	case DateTime:
		read = v.read
	case Time:
		read = v.read
	default:
		return false
	}
	if read {
		return true
	}
	// One built with a FHIR type, NewStringWithFHIRType("x", "code"), or with
	// the element beside it, says it is FHIR's.
	if carrier, ok := value.(ElementCarrier); ok && carrier.HasElement() {
		return true
	}
	return !IsSystemTypeName(value.Type())
}

// SystemValue returns the System value a primitive holds: the same value,
// without the FHIR type it was read as, without the element beside it, and no
// longer a FHIR primitive. FHIR declares it as the primitive's value property,
// "the implicit value property that is actually of type System.String" for a
// string, so it has no id or extensions of its own. It reports false for
// anything else.
func SystemValue(value Value) (Value, bool) {
	switch v := value.(type) {
	case String:
		v = v.WithFHIRType("").WithElement(nil)
		v.read = false
		return v, true
	case Boolean:
		v = v.WithFHIRType("").WithElement(nil)
		v.read = false
		return v, true
	case Integer:
		v = v.WithFHIRType("").WithElement(nil)
		v.read = false
		return v, true
	case Decimal:
		v = v.WithFHIRType("").WithElement(nil)
		v.read = false
		return v, true
	case Date:
		v = v.WithFHIRType("").WithElement(nil)
		v.read = false
		return v, true
	case DateTime:
		v = v.WithFHIRType("").WithElement(nil)
		v.read = false
		return v, true
	case Time:
		v = v.WithFHIRType("").WithElement(nil)
		v.read = false
		return v, true
	}
	return nil, false
}
