package fhirpath

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// deadlineResolver records the context a resolve() call is handed.
type deadlineResolver struct {
	deadline time.Time
	has      bool
}

func (r *deadlineResolver) Resolve(ctx context.Context, _ string) ([]byte, error) {
	r.deadline, r.has = ctx.Deadline()
	return nil, nil
}

// An evaluation's timeout reaches what it calls out to: resolve() hands the
// resolver a context that carries the deadline, as memberOf() and conformsTo()
// do, whether or not the deadline's context is made before it is needed.
func TestTheTimeoutReachesWhatAnEvaluationCallsOutTo(t *testing.T) {
	resolver := &deadlineResolver{}
	expr := MustCompile("Patient.managingOrganization.resolve()")
	before := time.Now()
	_, err := expr.EvaluateWithOptions([]byte(`{"resourceType":"Patient","managingOrganization":{"reference":"Organization/1"}}`),
		WithResolver(resolver), WithTimeout(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !resolver.has {
		t.Fatal("the resolver was handed a context without a deadline")
	}
	if d := resolver.deadline.Sub(before); d < time.Second || d > 3*time.Second {
		t.Errorf("the resolver's deadline is %v away, want about 2s", d)
	}
}

// An evaluation that runs past its timeout ends with the error the context
// package names, as it always did.
func TestAnEvaluationPastItsTimeoutEndsWithDeadlineExceeded(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"resourceType":"Basic","extension":[`)
	for i := 0; i < 5000; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`{"url":"u","valueInteger":1}`)
	}
	b.WriteString(`]}`)

	_, err := MustCompile("extension.select(%resource.extension.count()).count()").
		EvaluateWithOptions([]byte(b.String()), WithTimeout(time.Millisecond))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}
