package funcs

import (
	"strings"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

func init() {
	// Register FHIR-specific functions
	Register(FuncDef{
		Name:    "resolve",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnResolve,
	})

	Register(FuncDef{
		Name:    "extension",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnExtension,
	})

	Register(FuncDef{
		Name:    "hasExtension",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnHasExtension,
	})

	Register(FuncDef{
		Name:    "getExtensionValue",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnGetExtensionValue,
	})

	Register(FuncDef{
		Name:    "getReferenceKey",
		MinArgs: 0,
		MaxArgs: 1,
		Fn:      fnGetReferenceKey,
	})

	Register(FuncDef{
		Name:    "memberOf",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnMemberOf,
	})

	Register(FuncDef{
		Name:    "conformsTo",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnConformsTo,
	})
}

// fnResolve resolves a FHIR reference to the referenced resource.
// This function requires a resolver to be set in the context.
func fnResolve(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	resolver := ctx.GetResolver()
	result := types.Collection{}

	for _, item := range input {
		var reference string

		switch v := item.(type) {
		case types.String:
			reference = v.Value()
		case *types.ObjectValue:
			// Try to get the 'reference' field from a Reference object
			if ref, ok := v.Get("reference"); ok {
				if refStr, ok := ref.(types.String); ok {
					reference = refStr.Value()
				}
			}
		}

		if reference == "" {
			continue
		}

		// A reference into the document being evaluated needs no resolver: the
		// target is already here. Tried first, so that a contained resource
		// resolves the same way whether or not the caller wired one up.
		if local, found := resolveWithinDocument(ctx, reference); found {
			result = append(result, local)
			continue
		}

		// Anything else is somewhere the engine cannot reach on its own
		if resolver == nil {
			continue
		}

		// Resolve the reference
		resourceJSON, err := resolver.Resolve(ctx.Context(), reference)
		if err != nil {
			// Skip references that can't be resolved
			continue
		}

		// Parse the resolved resource
		col, err := types.JSONToCollection(resourceJSON)
		if err != nil {
			continue
		}

		result = append(result, col...)
	}

	return result, nil
}

// resolveWithinDocument finds the target of a reference inside the resource
// being evaluated.
//
// FHIR writes two kinds of reference that point inward. A fragment — "#obs1" —
// names a resource in the containing resource's contained list. A relative
// reference — "Observation/obs1" — names an entry of the Bundle when the
// document is one, matched on its fullUrl or on the resource's own type and id.
//
// Neither needs a resolver, and returning empty for them would be wrong rather
// than merely limited: the data is right there, and an invariant like dom-3
// that walks contained resources would silently pass on documents it should
// reject.
func resolveWithinDocument(ctx *eval.Context, reference string) (types.Value, bool) {
	root := rootResourceOf(ctx)
	if root == nil {
		return nil, false
	}

	if strings.HasPrefix(reference, "#") {
		return findContained(root, strings.TrimPrefix(reference, "#"))
	}
	return findBundleEntry(root, reference)
}

// rootResourceOf returns the resource the expression is being evaluated
// against, which is where an inward reference is resolved from.
func rootResourceOf(ctx *eval.Context) *types.ObjectValue {
	for _, name := range []string{"rootResource", "resource"} {
		if value, ok := ctx.GetVariable(name); ok && len(value) == 1 {
			if obj, isObject := value[0].(*types.ObjectValue); isObject {
				return obj
			}
		}
	}

	if root := ctx.Root(); len(root) == 1 {
		if obj, isObject := root[0].(*types.ObjectValue); isObject {
			return obj
		}
	}
	return nil
}

// findContained looks for a contained resource by its id.
func findContained(root *types.ObjectValue, id string) (types.Value, bool) {
	if id == "" {
		return nil, false
	}

	for _, candidate := range root.GetCollection("contained") {
		obj, ok := candidate.(*types.ObjectValue)
		if !ok {
			continue
		}
		if stringField(obj, "id") == id {
			return obj, true
		}
	}
	return nil, false
}

// findBundleEntry looks for a Bundle entry by its fullUrl, or by the type and
// id its resource carries.
func findBundleEntry(root *types.ObjectValue, reference string) (types.Value, bool) {
	if root.Type() != "Bundle" {
		return nil, false
	}

	for _, entry := range root.GetCollection("entry") {
		entryObj, isObject := entry.(*types.ObjectValue)
		if !isObject {
			continue
		}

		if stringField(entryObj, "fullUrl") == reference {
			if resource, hasResource := entryObj.Get("resource"); hasResource {
				return resource, true
			}
		}

		resource, hasResource := entryObj.Get("resource")
		if !hasResource {
			continue
		}
		resourceObj, isResource := resource.(*types.ObjectValue)
		if !isResource {
			continue
		}
		if id := stringField(resourceObj, "id"); id != "" &&
			reference == resourceObj.Type()+"/"+id {
			return resourceObj, true
		}
	}

	return nil, false
}

// stringField reads a field that holds a string, or "" when it is absent or
// holds something else.
func stringField(obj *types.ObjectValue, name string) string {
	value, ok := obj.Get(name)
	if !ok {
		return ""
	}
	text, isString := value.(types.String)
	if !isString {
		return ""
	}
	return text.Value()
}

// fnExtension returns extensions matching the given URL.
func fnExtension(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() || len(args) == 0 {
		return types.Collection{}, nil
	}

	// Get the extension URL to search for
	var url string
	if col, ok := args[0].(types.Collection); ok && !col.Empty() {
		if str, ok := col[0].(types.String); ok {
			url = str.Value()
		}
	}

	if url == "" {
		return types.Collection{}, nil
	}

	result := types.Collection{}

	for _, item := range input {
		// A primitive keeps its extensions in the element FHIR serializes
		// beside it, so the element is what carries them — not the value
		obj, ok := types.ElementOf(item)
		if !ok {
			continue
		}

		// Get the extension array
		extensions := obj.GetCollection("extension")
		for _, ext := range extensions {
			extObj, ok := ext.(*types.ObjectValue)
			if !ok {
				continue
			}

			// Check if the URL matches
			if extURL, ok := extObj.Get("url"); ok {
				if urlStr, ok := extURL.(types.String); ok {
					if urlStr.Value() == url {
						result = append(result, extObj)
					}
				}
			}
		}
	}

	return result, nil
}

// fnHasExtension returns true if any input element has an extension with the given URL.
func fnHasExtension(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	extensions, err := fnExtension(ctx, input, args)
	if err != nil {
		return nil, err
	}

	return types.Collection{types.NewBoolean(!extensions.Empty())}, nil
}

// fnGetExtensionValue returns the value of extensions matching the given URL.
func fnGetExtensionValue(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	extensions, err := fnExtension(ctx, input, args)
	if err != nil {
		return nil, err
	}

	result := types.Collection{}

	for _, ext := range extensions {
		extObj, ok := ext.(*types.ObjectValue)
		if !ok {
			continue
		}

		// Look for value[x] fields
		valueFields := []string{
			"valueString", "valueBoolean", "valueInteger", "valueDecimal",
			"valueDate", "valueDateTime", "valueTime", "valueCode",
			"valueCoding", "valueCodeableConcept", "valueQuantity",
			"valueReference", "valueIdentifier", "valuePeriod",
			"valueRange", "valueRatio", "valueAttachment",
			"valueUri", "valueUrl", "valueCanonical",
		}

		for _, field := range valueFields {
			if val, ok := extObj.Get(field); ok {
				result = append(result, val)
				break
			}
		}
	}

	return result, nil
}

// fnGetReferenceKey extracts the resource type and ID from a reference.
// Returns a string in the format "ResourceType/id" or just "id" if no type prefix.
func fnGetReferenceKey(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	// Optional argument: specific part to extract ("type", "id", or default "key")
	part := "key"
	if len(args) > 0 {
		if col, ok := args[0].(types.Collection); ok && !col.Empty() {
			if str, ok := col[0].(types.String); ok {
				part = str.Value()
			}
		}
	}

	result := types.Collection{}

	for _, item := range input {
		var reference string

		switch v := item.(type) {
		case types.String:
			reference = v.Value()
		case *types.ObjectValue:
			if ref, ok := v.Get("reference"); ok {
				if refStr, ok := ref.(types.String); ok {
					reference = refStr.Value()
				}
			}
		}

		if reference == "" {
			continue
		}

		// Parse the reference
		// Remove any URL prefix (e.g., "http://example.org/fhir/Patient/123")
		if idx := strings.LastIndex(reference, "/"); idx > 0 {
			// Check if there's a resource type prefix before this
			beforeSlash := reference[:idx]
			if lastSlashBefore := strings.LastIndex(beforeSlash, "/"); lastSlashBefore >= 0 {
				reference = beforeSlash[lastSlashBefore+1:] + "/" + reference[idx+1:]
			}
		}

		switch part {
		case "type":
			if idx := strings.Index(reference, "/"); idx > 0 {
				result = append(result, types.NewString(reference[:idx]))
			}
		case "id":
			if idx := strings.LastIndex(reference, "/"); idx >= 0 {
				result = append(result, types.NewString(reference[idx+1:]))
			} else {
				result = append(result, types.NewString(reference))
			}
		default: // "key" or any other value
			result = append(result, types.NewString(reference))
		}
	}

	return result, nil
}

// fnMemberOf checks if a code, Coding, or CodeableConcept is a member of a ValueSet.
// Usage: code.memberOf('http://hl7.org/fhir/ValueSet/example')
// Returns true if the code is in the ValueSet, false if not, empty if cannot be determined.
func fnMemberOf(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	// Get the ValueSet URL from the argument
	var valueSetURL string
	if len(args) > 0 {
		if col, ok := args[0].(types.Collection); ok && !col.Empty() {
			if str, ok := col[0].(types.String); ok {
				valueSetURL = str.Value()
			}
		}
	}

	if valueSetURL == "" {
		return types.Collection{}, nil
	}

	// Get the terminology service
	ts := ctx.GetTerminologyService()
	if ts == nil {
		// Without a terminology service, we can't validate membership
		// Return empty collection (unknown) as per FHIRPath spec
		return types.Collection{}, nil
	}

	// Process each item in the input
	for _, item := range input {
		// Convert the FHIRPath value to a form the terminology service can understand
		codeValue := extractCodeValue(item)
		if codeValue == nil {
			continue
		}

		// Check membership
		isMember, err := ts.MemberOf(ctx.Context(), codeValue, valueSetURL)
		if err != nil {
			// On error, return empty (unknown)
			continue
		}

		if isMember {
			return types.Collection{types.NewBoolean(true)}, nil
		}
	}

	// If we processed at least one item and none were members, return false
	if !input.Empty() {
		return types.Collection{types.NewBoolean(false)}, nil
	}

	return types.Collection{}, nil
}

// extractCodeValue extracts a code value from a FHIRPath value for terminology validation.
// Handles string (code), Coding objects, and CodeableConcept objects.
func extractCodeValue(item types.Value) interface{} {
	switch v := item.(type) {
	case types.String:
		// Simple code string
		return map[string]interface{}{
			"code": v.Value(),
		}

	case *types.ObjectValue:
		result := make(map[string]interface{})

		// Check if it's a Coding
		if system, ok := v.Get("system"); ok {
			if sysStr, ok := system.(types.String); ok {
				result["system"] = sysStr.Value()
			}
		}
		if code, ok := v.Get("code"); ok {
			if codeStr, ok := code.(types.String); ok {
				result["code"] = codeStr.Value()
			}
		}
		if version, ok := v.Get("version"); ok {
			if verStr, ok := version.(types.String); ok {
				result["version"] = verStr.Value()
			}
		}
		if display, ok := v.Get("display"); ok {
			if dispStr, ok := display.(types.String); ok {
				result["display"] = dispStr.Value()
			}
		}

		// Check if it's a CodeableConcept (has coding array)
		if codings := v.GetCollection("coding"); len(codings) > 0 {
			var codingList []map[string]interface{}
			for _, c := range codings {
				codingObj, ok := c.(*types.ObjectValue)
				if !ok {
					continue
				}
				coding := make(map[string]interface{})
				if sys, ok := codingObj.Get("system"); ok {
					if sysStr, ok := sys.(types.String); ok {
						coding["system"] = sysStr.Value()
					}
				}
				if code, ok := codingObj.Get("code"); ok {
					if codeStr, ok := code.(types.String); ok {
						coding["code"] = codeStr.Value()
					}
				}
				if ver, ok := codingObj.Get("version"); ok {
					if verStr, ok := ver.(types.String); ok {
						coding["version"] = verStr.Value()
					}
				}
				codingList = append(codingList, coding)
			}
			result["coding"] = codingList
		}

		if text, ok := v.Get("text"); ok {
			if textStr, ok := text.(types.String); ok {
				result["text"] = textStr.Value()
			}
		}

		if len(result) > 0 {
			return result
		}
	}

	return nil
}

// fnConformsTo reports whether the input conforms to a profile.
//
// FHIR defines the function, and defines it differently across versions. R4:
// "If the structure cannot be resolved to a valid profile, an error is thrown.
// If the input contains more than one element, an error is thrown. If the input
// is empty, the result is empty." R5 softened the first of those to an empty
// result, so an unresolvable profile is an error before R5 and unknown from R5
// on — the same version split the as operator has.
//
// Conformance against a real profile needs a validator, which the caller
// supplies. Without one, the base profiles are still resolvable: the canonical
// URL of a resource type names a structure the model already knows, and
// conforming to it is being of that type.
func fnConformsTo(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}
	if len(input) > 1 {
		return nil, eval.NewEvalError(eval.ErrSingletonExpected,
			"conformsTo() takes a single element, got %d", len(input))
	}

	profileURL, ok := toStringArg(args[0])
	if !ok || profileURL == "" {
		return types.Collection{}, nil
	}

	// A validator, when the caller supplied one, decides
	if pv := ctx.GetProfileValidator(); pv != nil {
		if obj, isObject := input[0].(*types.ObjectValue); isObject {
			conforms, err := pv.ConformsTo(ctx.Context(), obj.Data(), profileURL)
			if err == nil {
				return types.Collection{types.NewBoolean(conforms)}, nil
			}
		}
	}

	// Otherwise the base profiles remain answerable
	if conforms, resolved := conformsToBaseProfile(ctx, input[0], profileURL); resolved {
		return types.Collection{types.NewBoolean(conforms)}, nil
	}

	return unresolvedProfile(ctx, profileURL)
}

// baseProfilePrefix is where FHIR publishes the structure definition of every
// type it defines, so a URL under it names a type rather than a constraint.
const baseProfilePrefix = "http://hl7.org/fhir/StructureDefinition/"

// conformsToBaseProfile answers for a profile that is just a type, reporting
// whether it could be resolved at all.
//
// Conforming to the base profile of a type is being of that type, or of one
// derived from it — a Patient conforms to Patient and to DomainResource, and
// not to Person.
func conformsToBaseProfile(ctx *eval.Context, item types.Value, profileURL string) (conforms, resolved bool) {
	if !strings.HasPrefix(profileURL, baseProfilePrefix) {
		return false, false
	}
	typeName := strings.TrimPrefix(profileURL, baseProfilePrefix)

	model := ctx.GetModel()
	if model == nil {
		return false, false
	}
	// The model reaches functions through an adapter that reports separately
	// whether it could answer, so that a model unable to enumerate its types is
	// not read as one in which no type exists
	registry, ok := model.(interface {
		LookupType(string) (known, supported bool)
	})
	if !ok {
		return false, false
	}
	known, supported := registry.LookupType(typeName)
	if !supported || !known {
		// A model that cannot say whether the type exists cannot tell an
		// unresolvable profile from one it simply does not match
		return false, false
	}

	return model.IsSubtype(item.Type(), typeName), true
}

// unresolvedProfile reports a profile that could not be resolved, in the way the
// evaluated version calls for.
func unresolvedProfile(ctx *eval.Context, profileURL string) (types.Collection, error) {
	if ctx.EnforcesR5Rules() {
		return types.Collection{}, nil
	}
	return nil, eval.NewEvalError(eval.ErrInvalidArguments,
		"conformsTo(): %q cannot be resolved to a valid profile", profileURL)
}
