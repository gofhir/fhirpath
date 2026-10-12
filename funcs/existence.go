package funcs

import (
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// The registration blocks across this package are alike by construction: each is
// a table of one Register call per function, and dupl measures that shape rather
// than any repeated logic. Folding them into a loop over a slice would trade a
// readable table for an unreadable one.
//
//nolint:dupl // a declarative registration table, not duplicated logic
func init() {
	// Register existence functions
	Register(FuncDef{
		Name:    "empty",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnEmpty,
	})

	Register(FuncDef{
		Name:    "exists",
		MinArgs: 0,
		MaxArgs: 1,
		Fn:      fnExists,
	})

	Register(FuncDef{
		Name:    "all",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnAll,
	})

	Register(FuncDef{
		Name:    "allTrue",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnAllTrue,
	})

	Register(FuncDef{
		Name:    "anyTrue",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnAnyTrue,
	})

	Register(FuncDef{
		Name:    "allFalse",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnAllFalse,
	})

	Register(FuncDef{
		Name:    "anyFalse",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnAnyFalse,
	})

	Register(FuncDef{
		Name:    "count",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnCount,
	})

	Register(FuncDef{
		Name:    "distinct",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnDistinct,
	})

	Register(FuncDef{
		Name:    "isDistinct",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnIsDistinct,
	})

	Register(FuncDef{
		Name:    "subsetOf",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnSubsetOf,
	})

	Register(FuncDef{
		Name:    "supersetOf",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnSupersetOf,
	})
}

// fnEmpty returns true if the collection holds nothing but items that hold no
// value; see holdsAValue.
func fnEmpty(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if holdsAValue(input) {
		return types.FalseCollection, nil
	}
	return types.TrueCollection, nil
}

// fnExists returns true if the collection holds an item that holds a value;
// see holdsAValue. With criteria, the evaluation is handled by the evaluator,
// and counts every item the criteria keep.
func fnExists(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if holdsAValue(input) {
		return types.TrueCollection, nil
	}
	return types.FalseCollection, nil
}

// holdsAValue reports a collection with an item that holds a value. An
// element written as {}, which ele-1 makes invalid, is an item of its
// collection, but exists() and empty() answer as though it were not there, as
// the HL7 validator does: Bundle.entry[0].count() is 1 and
// Bundle.entry[0].exists() false on an entry written {}. FHIRPath defines
// both on the collection alone; CONFORMANCE.md records the divergence.
func holdsAValue(input types.Collection) bool {
	for _, item := range input {
		if !types.HoldsNoValue(item) {
			return true
		}
	}
	return false
}

// fnAll returns true if all elements match the criteria.
// Criteria evaluation is handled by the evaluator.
// Empty collection returns true (vacuous truth).
func fnAll(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	return types.TrueCollection, nil
}

// fnAllTrue returns true if all items are boolean true.
func fnAllTrue(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if err := requireBooleans("allTrue", input); err != nil {
		return nil, err
	}
	if input.Empty() || input.AllTrue() {
		return types.TrueCollection, nil
	}
	return types.FalseCollection, nil
}

// requireBooleans enforces the input each of these four functions is defined
// over: "Takes a collection of Boolean values".
//
// A non-Boolean item is not a false one. Reading it as false would answer
// (true | 'foo').allTrue() with a confident no, when what happened is that the
// collection was never the kind of thing the function accepts. A Boolean
// written with no value is one, neither true nor false, as the HL7 validator
// reads it: on "_active": {}, Patient.active.allTrue() and allFalse() are
// both false.
func requireBooleans(name string, input types.Collection) error {
	for _, item := range input {
		if _, ok := item.(types.Boolean); !ok && !types.IsValueless(item) {
			return eval.NewEvalError(eval.ErrType,
				"%s() takes a collection of Boolean values, and found %s", name, item.Type())
		}
	}
	return nil
}

// fnAnyTrue returns true if any item is boolean true.
func fnAnyTrue(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if err := requireBooleans("anyTrue", input); err != nil {
		return nil, err
	}
	if !input.Empty() && input.AnyTrue() {
		return types.TrueCollection, nil
	}
	return types.FalseCollection, nil
}

// fnAllFalse returns true if all items are boolean false.
func fnAllFalse(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if err := requireBooleans("allFalse", input); err != nil {
		return nil, err
	}
	if input.Empty() || input.AllFalse() {
		return types.TrueCollection, nil
	}
	return types.FalseCollection, nil
}

// fnAnyFalse returns true if any item is boolean false.
func fnAnyFalse(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if err := requireBooleans("anyFalse", input); err != nil {
		return nil, err
	}
	if !input.Empty() && input.AnyFalse() {
		return types.TrueCollection, nil
	}
	return types.FalseCollection, nil
}

// fnCount returns the number of items in the collection.
func fnCount(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return types.Collection{types.GetInteger(int64(input.Count()))}, nil
}

// fnDistinct returns a collection with duplicates removed.
func fnDistinct(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	return input.Distinct(), nil
}

// fnIsDistinct returns true if all items are distinct.
func fnIsDistinct(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.IsDistinct() {
		return types.TrueCollection, nil
	}
	return types.FalseCollection, nil
}

// fnSubsetOf returns true if all items in input are in other.
func fnSubsetOf(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if len(args) == 0 {
		return nil, eval.InvalidArgumentsError("subsetOf", 1, 0)
	}

	other, ok := args[0].(types.Collection)
	if !ok {
		return nil, eval.TypeError("Collection", "unknown", "subsetOf")
	}

	// All items in input must be in other
	for _, item := range input {
		if !other.Contains(item) {
			return types.FalseCollection, nil
		}
	}

	return types.TrueCollection, nil
}

// fnSupersetOf returns true if all items in other are in input.
func fnSupersetOf(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if len(args) == 0 {
		return nil, eval.InvalidArgumentsError("supersetOf", 1, 0)
	}

	other, ok := args[0].(types.Collection)
	if !ok {
		return nil, eval.TypeError("Collection", "unknown", "supersetOf")
	}

	// All items in other must be in input
	for _, item := range other {
		if !input.Contains(item) {
			return types.FalseCollection, nil
		}
	}

	return types.TrueCollection, nil
}
