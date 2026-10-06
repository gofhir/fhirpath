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

// IsFHIRPrimitive reports whether a value is known to be a primitive FHIR
// declares rather than a System value: one read with a FHIR type — given by a
// model, or by a choice element's key, valueString's string — or with the
// element FHIR writes beside it under _name, which only FHIR data has.
//
// A primitive read with neither, which is every primitive read without a model,
// cannot be told from a System value of the same kind, a literal; whether to
// take it for a FHIR one is the caller's to decide.
func IsFHIRPrimitive(value Value) bool {
	switch value.(type) {
	case String, Boolean, Integer, Decimal, Date, DateTime, Time:
	default:
		return false
	}
	if carrier, ok := value.(ElementCarrier); ok && carrier.HasElement() {
		return true
	}
	return !IsSystemTypeName(value.Type())
}

// SystemValue returns the System value a primitive holds: the same value,
// without the FHIR type it was read as and without the element beside it.
// FHIR declares it as the primitive's value property, "the implicit value
// property that is actually of type System.String" for a string, so it has no
// id or extensions of its own. It reports false for anything else.
func SystemValue(value Value) (Value, bool) {
	switch v := value.(type) {
	case String:
		return v.WithFHIRType("").WithElement(nil), true
	case Boolean:
		return v.WithFHIRType("").WithElement(nil), true
	case Integer:
		return v.WithFHIRType("").WithElement(nil), true
	case Decimal:
		return v.WithFHIRType("").WithElement(nil), true
	case Date:
		return v.WithFHIRType("").WithElement(nil), true
	case DateTime:
		return v.WithFHIRType("").WithElement(nil), true
	case Time:
		return v.WithFHIRType("").WithElement(nil), true
	}
	return nil, false
}
