package fhirpath

import (
	"fmt"
	"testing"

	"github.com/gofhir/fhirpath/funcs"
)

type traceRecorder struct{ entries []funcs.TraceEntry }

func (r *traceRecorder) Log(entry funcs.TraceEntry) { r.entries = append(r.entries, entry) }

// trace()'s projection is an expression, evaluated "with respect to each of
// the items in the input collection" (2.0.0), with $this and $index bound to
// each, as select() evaluates its own. Up to 1.14.7 it was evaluated once,
// from the scope the call sits in, so Patient.name.trace('n', given) logged
// nothing and (1 | 2).trace('x', $this * 10) ended in an error.
func TestTraceProjectsEachItem(t *testing.T) {
	recorder := &traceRecorder{}
	previous := funcs.GetTraceLogger()
	funcs.SetTraceLogger(recorder)
	defer funcs.SetTraceLogger(previous)

	doc := []byte(`{"resourceType":"Patient","name":[{"given":["A","B"]},{"given":["C"]}]}`)
	for expr, want := range map[string]string{
		"Patient.name.trace('n', given).count()":         "[A B C]",
		"Patient.name.trace('n', given.count()).count()": "[2 1]",
		"Patient.name.trace('n', $index).count()":        "[0 1]",
		"(1 | 2).trace('x', $this * 10).count()":         "[10 20]",
	} {
		recorder.entries = nil
		if _, err := Evaluate(doc, expr); err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if len(recorder.entries) != 1 {
			t.Fatalf("%s: %d entries logged, want 1", expr, len(recorder.entries))
		}
		if got := fmt.Sprint(recorder.entries[0].Projection); got != want {
			t.Errorf("%s logged %s, want %s", expr, got, want)
		}
	}
}
