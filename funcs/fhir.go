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
	if resolver == nil {
		// Without a resolver, we can't resolve references
		// Return empty collection as per FHIRPath spec
		return types.Collection{}, nil
	}

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
		obj, ok := item.(*types.ObjectValue)
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

// fnConformsTo checks if a resource conforms to a specified profile.
// Usage: resource.conformsTo('http://hl7.org/fhir/StructureDefinition/Patient')
// Returns true if the resource conforms, false if not, empty if cannot be determined.
func fnConformsTo(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	// Get the profile URL from the argument
	var profileURL string
	if len(args) > 0 {
		if col, ok := args[0].(types.Collection); ok && !col.Empty() {
			if str, ok := col[0].(types.String); ok {
				profileURL = str.Value()
			}
		}
	}

	if profileURL == "" {
		return types.Collection{}, nil
	}

	// Get the profile validator
	pv := ctx.GetProfileValidator()
	if pv == nil {
		// Without a profile validator, we can't validate conformance
		// Return empty collection (unknown) as per FHIRPath spec
		return types.Collection{}, nil
	}

	// Process the input - conformsTo typically operates on a single resource
	for _, item := range input {
		obj, ok := item.(*types.ObjectValue)
		if !ok {
			continue
		}

		// Get the raw JSON data for validation
		resourceJSON := obj.Data()
		if len(resourceJSON) == 0 {
			continue
		}

		// Check conformance
		conforms, err := pv.ConformsTo(ctx.Context(), resourceJSON, profileURL)
		if err != nil {
			// On error, return empty (unknown)
			continue
		}

		return types.Collection{types.NewBoolean(conforms)}, nil
	}

	return types.Collection{}, nil
}
