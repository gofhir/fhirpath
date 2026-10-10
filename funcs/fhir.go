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
			// The 'reference' field of a Reference object, read without writing
			// to it: the caller may share it
			reference = readString(v, "reference")
		}

		if reference == "" {
			continue
		}

		// A reference into the document being evaluated needs no resolver: the
		// target is already here. Tried first, so that a contained resource
		// resolves the same way whether or not the caller wired one up.
		holder, _ := item.(*types.ObjectValue) // the Reference, when it is one
		if local, found := resolveWithinDocument(ctx, holder, reference); found {
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

		// The resource was not read from the input, so it has no location in
		// it: it is read again as an object that is no input's root.
		for _, v := range col {
			if obj, ok := v.(*types.ObjectValue); ok {
				v = types.NewObjectValueWithType(obj.Data(), obj.Type())
			}
			result = append(result, v)
		}
	}

	return result, nil
}

// resolveWithinDocument finds the target of a reference inside the resource
// being evaluated.
//
// FHIR writes two kinds of reference that point inward. A fragment — "#obs1" —
// names a resource contained in the resource that makes the reference
// (references.html#contained); "#" alone names that resource from one of its
// contained resources. A reference into a Bundle names one of its entries
// (bundle.html#references), resolved from the entry that holds the reference.
//
// Both are resolved from where the reference is written, which the Reference
// object knows by the objects it was read out of. A reference read as a bare
// string has lost that, and is resolved from the root of the expression.
//
// The walk reads what Parent returns, fresh shared objects that keep nothing
// they read, and reaching the root of the evaluation it takes the root itself,
// which it holds, with what that has read.
//
// Neither needs a resolver, and returning empty for them would be wrong rather
// than merely limited: the data is right there, and an invariant like dom-3
// that walks contained resources would silently pass on documents it should
// reject.
func resolveWithinDocument(ctx *eval.Context, holder *types.ObjectValue, reference string) (types.Value, bool) {
	root := rootResourceOf(ctx)
	if fragment, ok := strings.CutPrefix(reference, "#"); ok {
		resource, container := containingResource(holder, root)
		if container == nil {
			container = root
		}
		if container == nil {
			return nil, false
		}
		if fragment == "" {
			// "#" names the container from one of its contained resources only
			return container, resource != nil && resource != container
		}
		return findContained(container, fragment)
	}

	rootBundle := root
	if rootBundle != nil && rootBundle.Type() != "Bundle" {
		rootBundle = nil
	}
	entry, bundle := referringEntry(holder, rootBundle)
	if bundle != nil {
		if found, ok := findBundleEntry(bundle, entry, reference); ok {
			return found, true
		}
	}
	// Not in the Bundle it is written in, or written nowhere a chain records:
	// the Bundle being evaluated, as it always was, from the same entry
	if rootBundle != nil && bundle != rootBundle {
		return findBundleEntry(rootBundle, entry, reference)
	}
	return nil, false
}

// sameObject reports two objects over the same JSON, as a parent and the fresh
// object Parent returns for it are.
func sameObject(a, b *types.ObjectValue) bool {
	if a == nil || b == nil {
		return a == b
	}
	x, y := a.Data(), b.Data()
	return len(x) == len(y) && (len(x) == 0 || &x[0] == &y[0])
}

// containingResource returns the resource a reference is written in, and the
// resource whose contained list its fragments name: the same one, or, when the
// reference is written in a contained resource, the resource that contains it.
// Both are nil when the reference does not know where it was read from.
//
// Reaching the root of the evaluation, where that is a resource, the walk stops
// there and takes the root itself as the container, as resolve() always has, without reading it again.
func containingResource(o, root *types.ObjectValue) (resource, container *types.ObjectValue) {
	for ; o != nil; o = o.Parent() {
		if readString(o, "resourceType") == "" {
			continue
		}
		if sameObject(o, root) {
			if resource == nil {
				resource = root
			}
			return resource, root
		}
		if resource == nil {
			resource = o
		}
		container = o
		if o.ParentField() != "contained" {
			break
		}
	}
	return resource, container
}

// referringEntry returns the Bundle entry that holds the resource a reference
// is written in, and that Bundle: the nearest one, so that a reference inside
// a Bundle held by another — a document or a message in a collection — is
// resolved in the Bundle it is written in. The root Bundle, whose type is
// known, is recognized without reading it.
func referringEntry(o, root *types.ObjectValue) (entry, bundle *types.ObjectValue) {
	for o != nil {
		parent := o.Parent()
		if parent == nil {
			return nil, nil
		}
		if o.ParentField() == "resource" && parent.ParentField() == "entry" {
			grand := parent.Parent()
			if sameObject(grand, root) {
				return parent, root
			}
			if grand != nil && readString(grand, "resourceType") == "Bundle" {
				return parent, grand
			}
		}
		o = parent
	}
	return nil, nil
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
		if readString(obj, "id") == id {
			return obj, true
		}
	}
	return nil, false
}

// findBundleEntry finds the entry of a Bundle a reference names, following
// bundle.html#references, from the entry that holds the reference when it is
// known:
//
//   - An entry whose fullUrl is the reference as written matches, as it always
//     has: a relative fullUrl, a versioned one.
//   - A relative reference, Type/id, is made absolute with the base of the
//     referring entry's fullUrl when that is RESTful, and matched on fullUrl:
//     Observation/x from http://b.org/fhir/ names http://b.org/fhir/Observation/x
//     and no other server's. Failing that, it is matched on the type and id
//     of the resources of entries whose fullUrl names no server — a urn:uuid,
//     or none — as a transaction writes the resources it creates. Where the
//     referring fullUrl is not RESTful, or the entry is not known, it is
//     matched on the type and id of any entry's resource.
//   - Any other reference is matched on fullUrl.
//   - A versioned reference, .../_history/v, also matches with its version
//     taken off, on meta.versionId.
//
// A fullUrl matches with or without a trailing slash. When several entries
// match — which FHIR calls ambiguous, leaving what to do to the application —
// the first is taken, as it always was.
func findBundleEntry(bundle, from *types.ObjectValue, reference string) (types.Value, bool) {
	target, version, versioned := strings.Cut(reference, "/_history/")
	relative := isRelativeReference(target)
	refType, refID, _ := strings.Cut(target, "/")
	matchesID := func(resource *types.ObjectValue) bool {
		return relative && readString(resource, "id") == refID && readString(resource, "resourceType") == refType
	}

	byFullURL := !relative
	if relative && from != nil {
		if base, ok := restfulBase(readString(from, "fullUrl")); ok {
			target, byFullURL = base+target, true
		}
	}

	asWritten := strings.TrimSuffix(reference, "/")
	target = strings.TrimSuffix(target, "/")

	var local types.Value // the first by type and id among entries that name no server
	for _, entry := range bundle.GetCollection("entry") {
		entryObj, isObject := entry.(*types.ObjectValue)
		if !isObject {
			continue
		}
		resource := readObject(entryObj, "resource")
		if resource == nil {
			continue
		}
		fullURL := strings.TrimSuffix(readString(entryObj, "fullUrl"), "/")
		if fullURL != "" && fullURL == asWritten {
			return resource, true
		}
		if versioned && readString(readObject(resource, "meta"), "versionId") != version {
			continue
		}
		switch {
		case !byFullURL:
			if matchesID(resource) {
				return resource, true
			}
		case fullURL == target:
			return resource, true
		case local == nil && relative && !namesServer(fullURL) && matchesID(resource):
			local = resource
		}
	}
	return local, local != nil
}

// isRelativeReference reports a reference of the form Type/id.
func isRelativeReference(reference string) bool {
	resourceType, id, ok := strings.Cut(reference, "/")
	return ok && resourceType != "" && id != "" && !strings.Contains(id, "/") &&
		!strings.Contains(resourceType, ":") && resourceType[0] >= 'A' && resourceType[0] <= 'Z'
}

// namesServer reports a fullUrl that names a server, http or https, as a
// urn:uuid or an absent one does not.
func namesServer(fullURL string) bool {
	return strings.HasPrefix(fullURL, "http://") || strings.HasPrefix(fullURL, "https://")
}

// restfulBase returns the base of a RESTful fullUrl — http://example.org/fhir/
// for http://example.org/fhir/Observation/x — and false for any other, such as
// a urn:uuid.
func restfulBase(fullURL string) (string, bool) {
	if !namesServer(fullURL) {
		return "", false
	}
	fullURL, _, _ = strings.Cut(fullURL, "/_history/")
	fullURL = strings.TrimSuffix(fullURL, "/")
	idAt := strings.LastIndexByte(fullURL, '/')
	if idAt < 0 {
		return "", false
	}
	typeAt := strings.LastIndexByte(fullURL[:idAt], '/')
	if typeAt < 0 || !isRelativeReference(fullURL[typeAt+1:]) {
		return "", false
	}
	return fullURL[:typeAt+1], true
}

// readString reads a field that holds a string, as the JSON writes it, with
// ReadString, and "" when it is absent, holds something else, or the object is
// nil.
func readString(obj *types.ObjectValue, name string) string {
	if obj == nil {
		return ""
	}
	text, _ := obj.ReadString(name)
	return text
}

// readObject reads a field that holds an object, or nil.
func readObject(obj *types.ObjectValue, name string) *types.ObjectValue {
	if obj == nil {
		return nil
	}
	if value, ok := obj.Get(name); ok {
		if object, isObject := value.(*types.ObjectValue); isObject {
			return object
		}
	}
	return nil
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
