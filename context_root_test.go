package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// NewContext reads the root it is given as an object without first scanning the
// whole document for its end; what it answers for well-formed, empty, truncated
// or otherwise malformed input is what it answered before.
func TestTheRootIsReadWithoutScanningItFirst(t *testing.T) {
	expr := fhirpath.MustCompile("id")
	for _, tt := range []struct {
		input, want string
	}{
		{`{"resourceType":"Patient","id":"x"}`, "[x]"},
		{"  \n\t{\"resourceType\":\"Patient\",\"id\":\"x\"}  \n", "[x]"},
		{`{"resourceType":"Patient","id":"x"}  trailing`, "[x]"},
		{`{"resourceType":"Patient","id":"x"`, "[]"},
		{`{"resourceType":"Patient","id":"x",`, "[]"},
		{``, "[]"},
		{`   `, "[]"},
		{`not json`, "[]"},
		{`null`, "[]"},
		{`"x"`, "[]"},
		{`[{"id":"a"},{"id":"b"}]`, "[a, b]"},
	} {
		t.Run(tt.input, func(t *testing.T) {
			result, err := expr.Evaluate([]byte(tt.input))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("id on %q = %s, want %s", tt.input, got, tt.want)
			}
		})
	}
}
