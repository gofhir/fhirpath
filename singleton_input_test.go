package fhirpath_test

import (
	"errors"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
)

// Functions that operate on one value end the evaluation when given more:
// "If the input collection contains multiple items, the evaluation of the
// expression will end and signal an error to the calling environment." They
// used to answer for the first item, or answer empty, for a value the
// expression never singled out. fhirpath.js raises the error in every case.
func TestSingletonFunctionsRejectMultipleItems(t *testing.T) {
	for _, expr := range []string{
		"(1 | -2).abs()",
		"(1.5 | 2.5).ceiling()",
		"(1 | 2).exp()",
		"(1.5 | 2.5).floor()",
		"(1 | 2).ln()",
		"(1 | 2).log(10)",
		"(1 | 2).power(2)",
		"(1.5 | 2.5).round()",
		"(1 | 4).sqrt()",
		"(1.5 | 2.5).truncate()",
		"(1.5 | 2.5).lowBoundary()",
		"(1.5 | 2.5).highBoundary()",
		"(@2020 | @2021).precision()",
		"(1 'mg' | 2 'mg').comparable(1 'g')",
		"('ab' | 'cd').encode('base64')",
		"('YWI=' | 'Y2Q=').decode('base64')",
		"('<a>' | '<b>').escape('html')",
		"('&lt;a&gt;' | '&lt;b&gt;').unescape('html')",
	} {
		t.Run(expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(`{"resourceType":"Basic"}`), expr)

			var evalErr *eval.EvalError
			if !errors.As(err, &evalErr) || evalErr.Type != eval.ErrSingletonExpected {
				t.Fatalf("%s = %v, %v; want a SingletonExpectedError", expr, result, err)
			}
		})
	}
}
