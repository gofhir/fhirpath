package fhirpath

// Model provides FHIR version-specific type and path metadata for the
// FHIRPath engine. When a Model is supplied via [WithModel], the evaluator
// uses it for precise polymorphic field resolution, type hierarchy checking,
// and path-based type inference. When nil (the default), the engine falls
// back to its built-in heuristics.
//
// This interface is satisfied by gofhir/models/r4.FHIRPathModelData (and the
// r4b/r5 equivalents) via Go's structural typing — no import dependency is
// required between the two packages.
type Model interface {
	// ChoiceTypes returns the permitted type suffixes for a polymorphic
	// element path. For example, "Observation.value" might return
	// ["Quantity","CodeableConcept","string","boolean",...].
	// Returns nil if the path is not a choice type element.
	ChoiceTypes(path string) []string

	// TypeOf returns the FHIR type code for the given element path.
	// For example, "Patient.name" returns "HumanName".
	// Returns "" if the path is unknown.
	TypeOf(path string) string

	// ReferenceTargets returns the allowed target resource type names for
	// a Reference or canonical element path.
	// Returns nil if the path is not a reference or has no constrained targets.
	ReferenceTargets(path string) []string

	// ParentType returns the immediate parent type in the FHIR type hierarchy.
	// For example, "Patient" returns "DomainResource", "Age" returns "Quantity".
	// Returns "" if the type is unknown or has no parent.
	ParentType(typeName string) string

	// IsSubtype reports whether child is the same as, or a subtype of,
	// parent in the FHIR type hierarchy.
	IsSubtype(child, parent string) bool

	// ResolvePath resolves a path whose element definition is borrowed from
	// another location via contentReference.
	// For example, "Questionnaire.item.item" returns "Questionnaire.item".
	// Returns the original path if no redirection is needed.
	ResolvePath(path string) string

	// IsResource reports whether the given type name is a known FHIR resource type.
	IsResource(typeName string) bool
}

// WithModel sets the FHIR version-specific model for the evaluation.
// When provided, the engine uses precise choice type lists, full type hierarchy,
// and path-based type resolution instead of built-in heuristics.
func WithModel(m Model) EvalOption {
	return func(o *EvalOptions) {
		o.Model = m
	}
}
