package funcs

import (
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

func init() {
	// Register aggregate functions
	Register(FuncDef{
		Name:    "aggregate",
		MinArgs: 1,
		MaxArgs: 2,
		Fn:      fnAggregate,
	})

	// Register tree navigation functions
	Register(FuncDef{
		Name:    "children",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnChildren,
	})

	Register(FuncDef{
		Name:    "descendants",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnDescendants,
	})

	// Register additional boolean functions
	Register(FuncDef{
		Name:    "not",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnNot,
	})

	// Register type checking functions
	Register(FuncDef{
		Name:    "hasValue",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnHasValue,
	})

	Register(FuncDef{
		Name:    "getValue",
		MinArgs: 0,
		MaxArgs: 0,
		Fn:      fnGetValue,
	})

	// Register combine function
	Register(FuncDef{
		Name:    "combine",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnCombine,
	})

	// Register union function
	Register(FuncDef{
		Name:    "union",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnUnion,
	})

	// Register as function for type casting
	Register(FuncDef{
		Name:    "as",
		MinArgs: 1,
		MaxArgs: 1,
		Fn:      fnAs,
	})
}

// fnAggregate performs an aggregation over the collection.
// aggregate(aggregator : expression [, init : value]) : value
func fnAggregate(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if len(args) == 0 {
		return nil, eval.InvalidArgumentsError("aggregate", 1, 0)
	}

	// For now, aggregate requires special handling in the evaluator
	// This is a placeholder that will be enhanced with proper lambda support
	// The evaluator should iterate over the collection, maintaining $total

	// If we have an initial value, use it
	if len(args) > 1 {
		if init, ok := args[1].(types.Collection); ok {
			return init, nil
		}
	}

	return types.Collection{}, nil
}

// defaultMaxDepth bounds descendants() when the context sets no limit.
// It matches the default documented for EvalOptions.MaxDepth.
const defaultMaxDepth = 100

// typeResolver returns the context's FHIR model as an element type resolver, or
// nil when no model was supplied. With a resolver, children carry the type the
// model assigns them instead of one inferred from the JSON shape.
func typeResolver(ctx *eval.Context) types.ElementTypeResolver {
	if ctx == nil {
		return nil
	}
	return ctx.GetModel()
}

// pathOf returns the path a model resolves obj's children beneath, which is
// where obj was reached — each item of the input its own, since one collection
// can hold items from different places.
func pathOf(ctx *eval.Context, obj *types.ObjectValue) string {
	if ctx == nil {
		return ""
	}
	return ctx.PathOf(obj)
}

// fnChildren returns all direct children of the input.
func fnChildren(ctx *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	result := types.Collection{}
	res := typeResolver(ctx)

	for _, item := range input {
		// A primitive's children are those of its element: its id and
		// extensions.
		if obj, ok := types.ElementOf(item); ok {
			for _, child := range obj.TypedChildren(pathOf(ctx, obj), res) {
				result = append(result, child.Value)
			}
		}
		// And its value: "FHIR primitives have a value child ... and the
		// primitive value will be included in the set returned by children()
		// or descendants()", after id and extension, as a primitive type's
		// StructureDefinition lists them.
		if value, ok := primitiveChild(item); ok {
			result = append(result, value)
		}
	}

	return result, nil
}

// primitiveChild is the value child of a FHIR primitive: its System value.
func primitiveChild(item types.Value) (types.Value, bool) {
	if !types.IsFHIRPrimitive(item) {
		return nil, false
	}
	return types.SystemValue(item)
}

// fnDescendants returns all descendants of the input (recursive children).
// The walk carries each node's FHIR path so that the model keeps resolving types
// all the way down; without a model it falls back to structural inference.
func fnDescendants(ctx *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	result := types.Collection{}
	res := typeResolver(ctx)

	maxDepth := defaultMaxDepth
	if ctx != nil {
		if limit := ctx.GetLimit("maxDepth"); limit > 0 {
			maxDepth = limit
		}
	}

	type node struct {
		value types.Value
		path  string
		depth int
	}

	queue := make([]node, 0, len(input))
	for _, item := range input {
		// A primitive is descended into through its element, from the
		// primitive's path.
		path := ""
		if obj, ok := types.ElementOf(item); ok {
			path = pathOf(ctx, obj)
		}
		queue = append(queue, node{value: item, path: path})
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current.depth >= maxDepth {
			continue
		}

		// A primitive is descended into through its element, as children()
		// does, and its value is a child as well, one with none of its own.
		if obj, ok := types.ElementOf(current.value); ok {
			for _, child := range obj.TypedChildren(current.path, res) {
				result = append(result, child.Value)
				queue = append(queue, node{value: child.Value, path: child.Path, depth: current.depth + 1})
			}
		}
		if value, ok := primitiveChild(current.value); ok {
			result = append(result, value)
		}
	}

	return result, nil
}

// fnNot returns the boolean negation.
func fnNot(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	// The singleton rule ends in an error for more than one item: which of them
	// was meant to be negated is not something to guess at.
	if len(input) > 1 {
		return nil, eval.NewEvalError(eval.ErrSingletonExpected,
			"not() expects a single item, got %d", len(input))
	}

	// A single non-Boolean node counts as true, so "reference.startsWith('#').not()"
	// and "code.not()" both behave per spec.
	val, ok := input.SingletonBoolean()
	if !ok {
		return types.Collection{}, nil
	}
	return types.Collection{types.NewBoolean(!val)}, nil
}

// fnHasValue returns true if the input has a primitive value.
func fnHasValue(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	// "Returns true if the input collection contains a single value which is a
	// FHIR primitive, and it has a primitive value (e.g. as opposed to not
	// having a value and just having extensions)." A primitive with only
	// extensions is read as its element, an object, so it has none.
	_, has := primitiveValue(input)
	return types.Collection{types.NewBoolean(has)}, nil
}

// fnGetValue returns "the underlying system value for the FHIR primitive if
// the input collection contains a single value which is a FHIR primitive, and
// it has a primitive value (see discussion for hasValue()). Otherwise the
// return value is empty."
func fnGetValue(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if value, has := primitiveValue(input); has {
		return types.Collection{value}, nil
	}
	return types.Collection{}, nil
}

// primitiveValue is the System value of the input when it is a single FHIR
// primitive with a value, which is what hasValue() and getValue() ask about.
func primitiveValue(input types.Collection) (types.Value, bool) {
	if len(input) != 1 || !types.IsFHIRPrimitive(input[0]) {
		return nil, false
	}
	return types.SystemValue(input[0])
}

// fnCombine combines two collections.
func fnCombine(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if len(args) == 0 {
		return nil, eval.InvalidArgumentsError("combine", 1, 0)
	}

	result := make(types.Collection, len(input))
	copy(result, input)

	if other, ok := args[0].(types.Collection); ok {
		result = append(result, other...)
	}

	return result, nil
}

// fnUnion returns the union of two collections (removes duplicates).
func fnUnion(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if len(args) == 0 {
		return nil, eval.InvalidArgumentsError("union", 1, 0)
	}

	// Get the other collection
	var other types.Collection
	if o, ok := args[0].(types.Collection); ok {
		other = o
	} else {
		return input, nil
	}

	// Use the Collection.Union method which handles duplicates
	return input.Union(other), nil
}

// fnAs casts the input to a specific type.
func fnAs(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if len(args) == 0 {
		return nil, eval.InvalidArgumentsError("as", 1, 0)
	}

	// Get the type name
	typeName := ""
	switch v := args[0].(type) {
	case types.Collection:
		if len(v) > 0 {
			if s, ok := v[0].(types.String); ok {
				typeName = s.Value()
			}
		}
	case types.String:
		typeName = v.Value()
	case string:
		typeName = v
	}

	if typeName == "" || input.Empty() {
		return types.Collection{}, nil
	}

	// Filter elements by type
	result := types.Collection{}
	for _, item := range input {
		if item.Type() == typeName {
			result = append(result, item)
		}
	}

	return result, nil
}
