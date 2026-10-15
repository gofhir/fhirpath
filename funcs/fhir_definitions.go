package funcs

import (
	"strings"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// FHIR's FHIRPath page defines these on top of the language. Each answers what
// its text settles without anything outside the engine, and asks the caller
// for the rest: a terminology service for subsumption, a checker for xhtml.
// What needs structure definitions, a slice or an element definition, ends
// the evaluation with an error rather than an answer the engine cannot know.
func init() {
	Register(FuncDef{Name: "subsumes", MinArgs: 1, MaxArgs: 1, Fn: fnSubsumes})
	Register(FuncDef{Name: "subsumedBy", MinArgs: 1, MaxArgs: 1, Fn: fnSubsumedBy})
	Register(FuncDef{Name: "htmlChecks", MinArgs: 0, MaxArgs: 0, Fn: fnHTMLChecks})
	Register(FuncDef{Name: "checkModifiers", MinArgs: 1, MaxArgs: 1, Fn: fnCheckModifiers})
	Register(FuncDef{Name: "elementDefinition", MinArgs: 0, MaxArgs: 0, Fn: fnElementDefinition})
	Register(FuncDef{Name: "slice", MinArgs: 2, MaxArgs: 2, Fn: fnSlice})
}

// fnSubsumes: "returns true if the source code is equivalent to the given
// code, or if the source code subsumes the given code".
func fnSubsumes(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	return subsumption(ctx, "subsumes", input, args, false)
}

// fnSubsumedBy: "returns true if the source code is equivalent to the given
// code, or if the source code is subsumed by the given code".
func fnSubsumedBy(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	return subsumption(ctx, "subsumedBy", input, args, true)
}

// subsumption answers subsumes() and subsumedBy(), the latter with the two
// sides swapped. Equivalence needs no terminology: "for Coding values,
// equivalence is defined based on the code and system elements only", and for
// a CodeableConcept "any Coding in the source or given elements" counts. A
// hierarchy does, which a TerminologyService that is also an eval.Subsumer
// supplies; without one the answer is unknown, empty.
func subsumption(ctx *eval.Context, name string, input types.Collection, args []interface{}, swapped bool) (types.Collection, error) {
	given, _ := args[0].(types.Collection)
	if input.Empty() || given.Empty() {
		return types.Collection{}, nil
	}
	// R5: "If either the input or the code parameter are not single value
	// collections, the return value is empty"; before R5, the singleton rule
	if len(input) > 1 || len(given) > 1 {
		if ctx.EnforcesR5Rules() {
			return types.Collection{}, nil
		}
		return nil, eval.NewEvalError(eval.ErrSingletonExpected,
			"%s() takes a single code on each side, got %d and %d", name, len(input), len(given))
	}

	sourceCodings, givenCodings := codings(input[0]), codings(given[0])
	if len(sourceCodings) == 0 || len(givenCodings) == 0 {
		return types.Collection{}, nil
	}
	for _, s := range sourceCodings {
		for _, g := range givenCodings {
			if equivalentCodings(s, g) {
				return types.Collection{types.NewBoolean(true)}, nil
			}
		}
	}

	subsumer, ok := ctx.GetTerminologyService().(eval.Subsumer)
	if !ok {
		return types.Collection{}, nil
	}
	source, target := extractCodeValue(input[0]), extractCodeValue(given[0])
	if swapped {
		source, target = target, source
	}
	subsumes, err := subsumer.Subsumes(ctx.Context(), source, target)
	if err != nil {
		return types.Collection{}, nil
	}
	return types.Collection{types.NewBoolean(subsumes)}, nil
}

// codings are the Codings a value stands for: itself, a Coding, or those of a
// CodeableConcept.
func codings(v types.Value) []*types.ObjectValue {
	obj, ok := v.(*types.ObjectValue)
	if !ok {
		return nil
	}
	if coding := obj.GetCollection("coding"); len(coding) > 0 {
		result := make([]*types.ObjectValue, 0, len(coding))
		for _, c := range coding {
			if o, ok := c.(*types.ObjectValue); ok {
				result = append(result, o)
			}
		}
		return result
	}
	if _, ok := obj.Get("code"); ok {
		return []*types.ObjectValue{obj}
	}
	return nil
}

// equivalentCodings compares two Codings by system and code alone.
func equivalentCodings(a, b *types.ObjectValue) bool {
	code, ok := stringField(a, "code")
	if !ok {
		return false
	}
	other, ok := stringField(b, "code")
	if !ok || code != other {
		return false
	}
	system, _ := stringField(a, "system")
	otherSystem, _ := stringField(b, "system")
	return system == otherSystem
}

// stringField reads a primitive child of an object as a string.
func stringField(obj *types.ObjectValue, field string) (string, bool) {
	v, ok := obj.Get(field)
	if !ok {
		return "", false
	}
	s, ok := v.(types.String)
	if !ok {
		return "", false
	}
	return s.Value(), true
}

// fnHTMLChecks: R5 "When invoked on a single xhtml element returns true if the
// rules around HTML usage are met, and false if they are not. The return value
// is empty on any other kind of element, or a collection of xhtml elements";
// R4 makes it "undefined ( null )", the empty collection. An element is xhtml
// by the type a model gives it. The rules are FHIR's, so the caller's
// eval.HTMLChecker applies them; without one the element has no answer, and
// the evaluation ends rather than pass a narrative it never read.
func fnHTMLChecks(ctx *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if len(input) != 1 {
		return types.Collection{}, nil
	}
	xhtml, ok := input[0].(types.String)
	if !ok || xhtml.Type() != "xhtml" {
		return types.Collection{}, nil
	}
	checker := ctx.GetHTMLChecker()
	if checker == nil {
		return nil, eval.NewEvalError(eval.ErrInvalidOperation,
			"htmlChecks() needs an HTML checker to apply FHIR's rules for xhtml; none was given")
	}
	passes, err := checker.HTMLChecks(ctx.Context(), xhtml.Value())
	if err != nil {
		return nil, eval.NewEvalError(eval.ErrInvalidOperation, "htmlChecks(): %v", err)
	}
	return types.Collection{types.NewBoolean(passes)}, nil
}

// fnCheckModifiers: "For each element in the input collection, verifies that
// there are no modifying extensions defined other than the ones given by the
// modifier argument. If the check passes, the input collection is returned.
// Otherwise, an error is thrown". R5 makes the argument a "comma-separated
// string" and adds "including if modifier is empty"; R4's is a collection of
// strings, each read the same way.
func fnCheckModifiers(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	allowed := map[string]bool{}
	if modifiers, ok := args[0].(types.Collection); ok {
		for _, m := range modifiers {
			s, ok := m.(types.String)
			if !ok {
				return nil, eval.TypeError("String", m.Type(), "checkModifiers")
			}
			for _, url := range strings.Split(s.Value(), ",") {
				if url = strings.TrimSpace(url); url != "" {
					allowed[url] = true
				}
			}
		}
	}
	if len(allowed) == 0 && ctx.EnforcesR5Rules() {
		return nil, eval.NewEvalError(eval.ErrInvalidArguments, "checkModifiers() needs a modifier")
	}

	for _, item := range input {
		obj, ok := item.(*types.ObjectValue)
		if !ok {
			continue
		}
		for _, ext := range obj.GetCollection("modifierExtension") {
			extension, ok := ext.(*types.ObjectValue)
			if !ok {
				continue
			}
			url, _ := stringField(extension, "url")
			if !allowed[url] {
				return nil, eval.NewEvalError(eval.ErrInvalidOperation,
					"checkModifiers(): modifier extension %q is not among those allowed", url)
			}
		}
	}
	return input, nil
}

// fnElementDefinition: "Returns the FHIR element definition information for
// each element in the input collection. If the input collection is empty, the
// return value will be empty" (R5). The definitions are not the engine's, so
// an element ends the evaluation.
func fnElementDefinition(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}
	return nil, eval.NewEvalError(eval.ErrInvalidOperation,
		"elementDefinition() needs element definitions, which this engine does not hold")
}

// fnSlice: "Returns the given slice as defined in the given structure
// definition". The text settles an empty input, "the result is an empty
// collection", and a structure that cannot be resolved: R4 "an error is
// thrown", R5 "and empty value is returned", as are empty parameters. A string
// that is no URI resolves to nothing. Resolving one that is needs
// structure definitions, which are not the engine's.
func fnSlice(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}
	structure, hasStructure := toStringArg(args[0])
	name, hasName := toStringArg(args[1])
	if !hasStructure || structure == "" || !hasName || name == "" {
		if ctx.EnforcesR5Rules() {
			return types.Collection{}, nil
		}
		return nil, eval.NewEvalError(eval.ErrInvalidArguments, "slice() needs a structure and a slice name")
	}
	if !isURI(structure) {
		if ctx.EnforcesR5Rules() {
			return types.Collection{}, nil
		}
		return nil, eval.NewEvalError(eval.ErrInvalidArguments,
			"slice(): %q cannot be resolved to a structure definition", structure)
	}
	return nil, eval.NewEvalError(eval.ErrInvalidOperation,
		"slice() needs structure definitions, which this engine does not hold")
}
