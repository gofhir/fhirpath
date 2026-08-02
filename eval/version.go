package eval

import (
	"strconv"
	"strings"
)

// typeRegistry is implemented by a model that can say whether a name resolves to
// one of its types.
//
// The second result separates "this model has no such type" from "this model
// cannot answer", which a single bool would conflate — and conflating them would
// make every type specifier an error against a model that does not carry the
// list. The public side of this is fhirpath.TypeRegistry, where the interface is
// documented; the adapter in the parent package translates between the two.
type typeRegistry interface {
	LookupType(typeName string) (known, supported bool)
}

// versionedModel is implemented by a model that declares which FHIR version it
// describes. It mirrors fhirpath.VersionedModel across the package boundary.
type versionedModel interface {
	FHIRVersion() string
}

// enforcesR5Rules reports whether the rules that changed in R5 apply.
//
// The reference validator settles these by version rather than choosing one
// reading, because FHIR's own R4 invariants depend on the earlier behavior: it
// disables the rule for anything below R5. A context with no model, or a model
// that does not declare its version, is treated as pre-R5 — the behavior every
// existing caller already has.
func (c *Context) enforcesR5Rules() bool {
	if c == nil || c.model == nil {
		return false
	}
	versioned, ok := c.model.(versionedModel)
	if !ok {
		return false
	}
	return isR5Plus(versioned.FHIRVersion())
}

// isR5Plus reports whether a FHIR version string names R5 or later, accepting
// the forms models use: "5.0.0", "R5", "4.0.1".
func isR5Plus(version string) bool {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(strings.TrimPrefix(version, "R"), "r")
	if version == "" {
		return false
	}

	major := version
	if index := strings.IndexByte(major, '.'); index >= 0 {
		major = major[:index]
	}

	number, err := strconv.Atoi(major)
	if err != nil {
		return false
	}
	return number >= 5
}
