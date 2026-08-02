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

// fnChildren returns all direct children of the input.
func fnChildren(ctx *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	result := types.Collection{}
	res := typeResolver(ctx)
	basePath := ""
	if ctx != nil {
		basePath = ctx.Path()
	}

	for _, item := range input {
		if obj, ok := item.(*types.ObjectValue); ok {
			for _, child := range obj.TypedChildren(basePath, res) {
				result = append(result, child.Value)
			}
		}
	}

	return result, nil
}

// fnDescendants returns all descendants of the input (recursive children).
// The walk carries each node's FHIR path so that the model keeps resolving types
// all the way down; without a model it falls back to structural inference.
func fnDescendants(ctx *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	result := types.Collection{}
	res := typeResolver(ctx)

	maxDepth := defaultMaxDepth
	basePath := ""
	if ctx != nil {
		if limit := ctx.GetLimit("maxDepth"); limit > 0 {
			maxDepth = limit
		}
		basePath = ctx.Path()
	}

	type node struct {
		value types.Value
		path  string
		depth int
	}

	queue := make([]node, 0, len(input))
	for _, item := range input {
		queue = append(queue, node{value: item, path: basePath})
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		obj, ok := current.value.(*types.ObjectValue)
		if !ok || current.depth >= maxDepth {
			continue
		}

		for _, child := range obj.TypedChildren(current.path, res) {
			result = append(result, child.Value)
			queue = append(queue, node{value: child.Value, path: child.Path, depth: current.depth + 1})
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
	if input.Empty() {
		return types.Collection{types.NewBoolean(false)}, nil
	}

	// Check if any element has a primitive value
	for _, item := range input {
		switch item.(type) {
		case types.Boolean, types.String, types.Integer, types.Decimal,
			types.Date, types.DateTime, types.Time:
			return types.Collection{types.NewBoolean(true)}, nil
		}
	}

	return types.Collection{types.NewBoolean(false)}, nil
}

// fnGetValue returns the primitive value if it exists.
func fnGetValue(_ *eval.Context, input types.Collection, _ []interface{}) (types.Collection, error) {
	if input.Empty() {
		return types.Collection{}, nil
	}

	// Return primitive values
	result := types.Collection{}
	for _, item := range input {
		switch v := item.(type) {
		case types.Boolean, types.String, types.Integer, types.Decimal,
			types.Date, types.DateTime, types.Time:
			result = append(result, v)
		}
	}

	return result, nil
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
