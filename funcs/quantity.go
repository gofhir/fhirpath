// Quantity-specific functions.

package funcs

import (
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

func init() {
	Register(FuncDef{Name: "comparable", MinArgs: 1, MaxArgs: 1, Fn: fnComparable})
}

// fnComparable returns true when the input quantity and the argument quantity
// have commensurable units, that is when comparing them is meaningful:
// 1 'cm' is comparable to 1 '[in_i]' but not to 1 's'.
//
// This function is exercised by the official test suite but is not defined in
// the FHIRPath 2.0.0 specification that the suite references, so its behavior
// here follows the suite's cases.
//
// Returns empty unless both sides are single quantities.
func fnComparable(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if len(args) == 0 {
		return nil, eval.InvalidArgumentsError("comparable", 1, 0)
	}

	left, ok := singleQuantity(input)
	if !ok {
		return types.Collection{}, nil
	}

	argCol, ok := args[0].(types.Collection)
	if !ok {
		return types.Collection{}, nil
	}
	right, ok := singleQuantity(argCol)
	if !ok {
		return types.Collection{}, nil
	}

	return types.Collection{types.NewBoolean(left.Comparable(right))}, nil
}

// singleQuantity extracts a lone quantity from a collection, converting a FHIR
// Quantity object when that is what it holds.
func singleQuantity(col types.Collection) (types.Quantity, bool) {
	if len(col) != 1 {
		return types.Quantity{}, false
	}

	switch v := col[0].(type) {
	case types.Quantity:
		return v, true
	case *types.ObjectValue:
		return v.ToQuantity()
	}
	return types.Quantity{}, false
}
