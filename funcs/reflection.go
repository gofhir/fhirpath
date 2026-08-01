// FHIRPath reflection: the type() function.
//
// Per the FHIRPath specification, section "Types and Reflection > Reflection",
// type() returns type information for each element of the input collection as
// one of the concrete subtypes of TypeInfo:
//
//	SimpleTypeInfo { namespace: string, name: string, baseType: TypeSpecifier }
//	ClassInfo      { namespace: string, name: string, baseType: TypeSpecifier,
//	                 element: List<ClassInfoElement> }
//	ListTypeInfo   { elementType: TypeSpecifier }
//	TupleTypeInfo  { element: List<TupleTypeInfoElement> }
//
// SimpleTypeInfo and ClassInfo are produced here. The remaining information is
// not reported because it cannot be obtained without inventing it:
//
//   - ClassInfo.element and TupleTypeInfo require enumerating the elements of a
//     type, which the Model interface does not expose.
//   - ListTypeInfo requires the declared cardinality of an element, which the
//     Model interface does not expose either.
//
// Reading those from the instance JSON would describe the value at hand rather
// than its type, so they are left out until the model can answer for them.
//
// Note that this section of the specification is Standard for Trial Use, and it
// is internally inconsistent: it states that type() yields one result per input
// element, yet its own ListTypeInfo example collapses a multi-item collection
// into a single result.

package funcs

import (
	"encoding/json"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

func init() {
	Register(FuncDef{
		Name:    "type",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnType,
	})
}

// Type namespaces defined by the specification: types declared by FHIRPath
// itself live in System, and the model's types live in that model's namespace,
// which for this engine is always FHIR.
const (
	namespaceSystem = "System"
	namespaceFHIR   = "FHIR"

	// systemAny is the base type of every System type.
	systemAny = "System.Any"

	// TypeInfo subtypes, used as the FHIR type of the returned objects so that
	// they answer is()/ofType() as well as member access.
	kindSimpleTypeInfo = "SimpleTypeInfo"
	kindClassInfo      = "ClassInfo"
)

// typeInfo is the serialized form of SimpleTypeInfo and ClassInfo. Empty fields
// are omitted so that navigating them yields empty rather than a made-up value.
type typeInfo struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	BaseType  string `json:"baseType,omitempty"`
}

// fnType returns the type information of each element of the input.
func fnType(ctx *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	var model eval.Model
	if ctx != nil {
		model = ctx.GetModel()
	}

	result := make(types.Collection, 0, len(input))
	for _, item := range input {
		info, err := typeInfoOf(item, model)
		if err != nil {
			return nil, err
		}
		result = append(result, info)
	}
	return result, nil
}

// typeInfoOf builds the TypeInfo describing a single value.
func typeInfoOf(item types.Value, model eval.Model) (types.Value, error) {
	name := item.Type()

	kind := kindSimpleTypeInfo
	namespace := namespaceSystem

	switch {
	case isObjectValue(item):
		// Structured values are the model's types.
		kind = kindClassInfo
		namespace = namespaceFHIR
		if name == "Object" {
			// The type could not be determined: report the name without
			// claiming it belongs to the FHIR namespace.
			namespace = ""
		}
	case !types.IsSystemTypeName(name):
		// A primitive carrying a FHIR type, e.g. code or uri. Per the FHIR
		// specification these are distinct from their System counterparts.
		namespace = namespaceFHIR
	}

	data, err := json.Marshal(typeInfo{
		Namespace: namespace,
		Name:      name,
		BaseType:  baseTypeOf(namespace, name, model),
	})
	if err != nil {
		return nil, err
	}

	return types.NewObjectValueWithType(data, kind), nil
}

// baseTypeOf returns the qualified base type of a type, or "" when it cannot be
// determined. Without a model there is no FHIR type hierarchy to report, so the
// field is omitted rather than guessed.
func baseTypeOf(namespace, name string, model eval.Model) string {
	if namespace == namespaceSystem {
		return systemAny
	}
	if model == nil {
		return ""
	}
	if parent := model.ParentType(name); parent != "" {
		return namespaceFHIR + "." + parent
	}
	return ""
}

// isObjectValue reports whether the value is a structured object.
func isObjectValue(v types.Value) bool {
	_, ok := v.(*types.ObjectValue)
	return ok
}
