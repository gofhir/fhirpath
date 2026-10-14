// Quantity-specific functions.

package funcs

import (
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

func init() {
	Register(FuncDef{Name: "comparable", MinArgs: 1, MaxArgs: 1, Fn: singleInput("comparable", fnComparable)})
}

// fnComparable returns true when the input quantity and the argument quantity
// have commensurable units, that is when comparing them is meaningful:
// 1 'cm' is comparable to 1 '[in_i]' but not to 1 's'.
//
// Defined in the FHIRPath 3.0.0 specification, section comparable(), as
// Standard for Trial Use: comparable means both have values and the units are
// the same irrespective of system, or both carry code and system and the codes
// are commensurable within it — 'd' and 'h', or '[in_i]' and 'cm'.
//
// Returns empty unless both sides are single quantities, as the spec requires.
func fnComparable(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	if len(args) == 0 {
		return nil, eval.InvalidArgumentsError("comparable", 1, 0)
	}

	argCol, ok := args[0].(types.Collection)
	if !ok || len(input) != 1 || len(argCol) != 1 {
		return types.Collection{}, nil
	}

	// A FHIR Quantity object converts as the comparison operators convert it,
	// its comparator not read; one whose value overflows is comparable to
	// nothing. A value that is no quantity, a Money among them, cannot be
	// compared with one: false, as fhirpath.js and the HL7 validator answer.
	left, right, ok, err := eval.QuantityPair(input[0], argCol[0])
	if err != nil {
		return types.Collection{}, nil
	}
	if !ok {
		return types.Collection{types.NewBoolean(false)}, nil
	}

	return types.Collection{types.NewBoolean(left.Comparable(right))}, nil
}
