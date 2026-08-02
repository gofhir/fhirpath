package types

// The names of the System types, as FHIRPath spells them.
//
// These are the values Type() returns and the names is, as and ofType() match
// against, so they are shared rather than written out at each site: the engine
// compares them across three packages, and a disagreement about one of them
// would surface as a type test that silently never matches.
//
// FHIR primitives are deliberately not here. FHIR.boolean is a distinct type
// from System.Boolean and is spelled in lower camel case for exactly that
// reason; the mapping between the two lives in the evaluator, which is where the
// distinction is decided.
const (
	TypeNameBoolean  = "Boolean"
	TypeNameString   = "String"
	TypeNameInteger  = "Integer"
	TypeNameDecimal  = "Decimal"
	TypeNameDate     = "Date"
	TypeNameDateTime = "DateTime"
	TypeNameTime     = "Time"
	TypeNameQuantity = "Quantity"
)

// systemTypeNames indexes the System type names for lookup.
var systemTypeNames = map[string]bool{
	TypeNameBoolean:  true,
	TypeNameString:   true,
	TypeNameInteger:  true,
	TypeNameDecimal:  true,
	TypeNameDate:     true,
	TypeNameDateTime: true,
	TypeNameTime:     true,
	TypeNameQuantity: true,
}

// IsSystemTypeName reports whether the name is one of the System types FHIRPath
// declares in its Literals section.
//
// This is the type system of the language rather than a FHIR version-specific
// list, so it does not go stale: any other type name on a primitive value came
// from the FHIR model (code, uri, id, markdown, ...).
func IsSystemTypeName(name string) bool {
	return systemTypeNames[name]
}
