package funcs

import (
	"bytes"
	"testing"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// trace() writes nothing unless a logger is configured: FHIR's dom-3 calls it
// on every DomainResource in R4, and writing a line each time filled a
// validator's logs. It returns its input either way, and writes once a logger
// is set.
func TestTraceWritesOnlyToAConfiguredLogger(t *testing.T) {
	if _, null := GetTraceLogger().(NullTraceLogger); !null {
		t.Fatalf("the default trace logger is %T, want NullTraceLogger", GetTraceLogger())
	}

	ctx := eval.NewContext([]byte(`{}`))
	input := types.Collection{types.NewString("a")}
	args := []interface{}{types.Collection{types.NewString("label")}}

	result, err := fnTrace(ctx, input, args)
	if err != nil || len(result) != 1 {
		t.Fatalf("trace with the default logger = %v, %v; want its input", result, err)
	}

	var out bytes.Buffer
	SetTraceLogger(NewDefaultTraceLogger(&out, false))
	defer SetTraceLogger(NullTraceLogger{})

	result, err = fnTrace(ctx, input, args)
	if err != nil || len(result) != 1 {
		t.Fatalf("trace with a logger = %v, %v; want its input", result, err)
	}
	if !bytes.Contains(out.Bytes(), []byte("label")) {
		t.Errorf("a configured logger received %q, want the trace's label", out.String())
	}
}
