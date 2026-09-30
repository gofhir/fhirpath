package funcs

import (
	"errors"
	"testing"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// Every to- and convertsTo- function says of its input: "If the input
// collection contains multiple items, the evaluation of the expression will end
// and signal an error to the calling environment." Reading the first item
// instead answers for a value the expression never singled out.
func TestConversionsRejectMultipleItems(t *testing.T) {
	ctx := eval.NewContext([]byte(`{}`))
	input := types.Collection{types.NewInteger(1), types.NewInteger(2)}

	for _, name := range []string{
		"toBoolean", "convertsToBoolean",
		"toInteger", "convertsToInteger",
		"toDecimal", "convertsToDecimal",
		"toString", "convertsToString",
		"toDate", "convertsToDate",
		"toDateTime", "convertsToDateTime",
		"toTime", "convertsToTime",
		"toQuantity", "convertsToQuantity",
	} {
		t.Run(name, func(t *testing.T) {
			fn, ok := Get(name)
			if !ok {
				t.Fatalf("%s is not registered", name)
			}

			result, err := fn.Fn(ctx, input, nil)

			var evalErr *eval.EvalError
			if !errors.As(err, &evalErr) || evalErr.Type != eval.ErrSingletonExpected {
				t.Fatalf("%s((1 | 2)) = %v, %v; want a SingletonExpectedError", name, result, err)
			}
		})
	}
}
