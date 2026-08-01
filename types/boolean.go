package types

import "fmt"

// Boolean represents a FHIRPath boolean value.
type Boolean struct {
	value    bool
	fhirType string // FHIR type when the value was read through a model
}

// NewBoolean creates a new Boolean value.
func NewBoolean(v bool) Boolean {
	return Boolean{value: v}
}

// Bool returns the underlying boolean value.
func (b Boolean) Bool() bool {
	return b.value
}

// Type returns "Boolean".
func (b Boolean) Type() string {
	if b.fhirType != "" {
		return b.fhirType
	}
	return TypeNameBoolean
}

// Equal returns true if other is a Boolean with the same value.
func (b Boolean) Equal(other Value) bool {
	if o, ok := other.(Boolean); ok {
		return b.value == o.value
	}
	return false
}

// Equivalent is the same as Equal for booleans.
func (b Boolean) Equivalent(other Value) bool {
	return b.Equal(other)
}

// String returns "true" or "false".
func (b Boolean) String() string {
	return fmt.Sprintf("%t", b.value)
}

// IsEmpty returns false for boolean values.
func (b Boolean) IsEmpty() bool {
	return false
}

// Not returns the logical negation.
func (b Boolean) Not() Boolean {
	return NewBoolean(!b.value)
}

// WithFHIRType returns a copy that reports the FHIR type it was declared with.
// FHIR primitives are types in their own right — a FHIR.boolean is not a
// System.Boolean — so a value keeps the name the model gave it.
func (b Boolean) WithFHIRType(fhirType string) Boolean {
	b.fhirType = fhirType
	return b
}
