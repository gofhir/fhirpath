package eval_test

import (
	"sync"
	"testing"
	"time"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/funcs"
	"github.com/gofhir/fhirpath/types"
)

// FHIRPath 2.0.0 and 3.0.0, "Current date and time functions": "these
// operators should return the same value regardless of how many times they
// are evaluated within any given expression"; 3.0.0's examples: timeOfDay()
// is "Equivalent to now().timeOf()", today() "equivalent to now().toDate()"
// (#225).

// movingClock is a time source that moves a millisecond every time it is
// read, so an evaluation that reads the time twice sees two instants, every
// time, whatever the real clock's resolution.
func movingClock(t *testing.T) {
	t.Helper()
	var mu sync.Mutex
	at := time.Date(2026, 3, 4, 23, 59, 59, 990_000_000, time.FixedZone("", -3*3600))
	t.Cleanup(eval.SetNowSource(func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		at = at.Add(time.Millisecond)
		return at
	}))
}

var patient = []byte(`{"resourceType":"Patient","name":[{"given":["a","b","c"]}]}`)

func evaluate(t *testing.T, ctx *eval.Context, expr string) string {
	t.Helper()
	result, err := fhirpath.MustCompile(expr).EvaluateWithContext(ctx)
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	return result.String()
}

func TestOneEvaluationSeesOneInstant(t *testing.T) {
	movingClock(t)
	for _, expr := range []string{
		"now() = now()",
		"timeOfDay() = timeOfDay()",
		"today() = today()",
		"timeOfDay() = now().timeOf()",
		"today() = now().toDate()",
		// in a scope of its own: where(), select(), repeat(), aggregate(),
		// a function's argument
		"(1 | 2 | 3).select(now()).distinct().count() = 1",
		"Patient.name.given.where(now() = now()).count() = 3",
		"Patient.name.given.select(timeOfDay()).distinct().count() = 1",
		"(1 | 2 | 3).aggregate($total.combine(now()), {}).distinct().count() = 1",
		// a projection that stops after one level, defect or not
		"(1 | 2).repeat(iif($this is Integer, now(), {})).distinct().count() = 1",
		"now().toString().length() = now().toString().length() and iif(now() = now(), true, false)",
	} {
		if got := evaluate(t, eval.NewContext(patient), expr); got != "[true]" {
			t.Errorf("%s = %s", expr, got)
		}
	}
}

func TestEachEvaluationReadsTheTimeAgain(t *testing.T) {
	movingClock(t)
	for name, ctx := range map[string]*eval.Context{
		"one context, reused": eval.NewContext(patient),
		"a zero context":      {},
	} {
		first := evaluate(t, ctx, "now()")
		second := evaluate(t, ctx, "now()")
		if first == second {
			t.Errorf("%s: two evaluations saw one instant, %s", name, first)
		}
	}
}

func TestAFixedInstantIsAnsweredInItsOwnZone(t *testing.T) {
	movingClock(t)
	at := time.Date(2026, 3, 4, 23, 59, 58, 123_000_000, time.FixedZone("", 9*3600+30*60))
	ctx := eval.NewContext(patient)
	ctx.SetNow(at)
	for expr, want := range map[string]string{
		"now()":                        "[2026-03-04T23:59:58.123+09:30]",
		"today()":                      "[2026-03-04]",
		"timeOfDay()":                  "[23:59:58.123]",
		"timeOfDay() = now().timeOf()": "[true]",
		"today() = now().toDate()":     "[true]",
	} {
		if got := evaluate(t, ctx, expr); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
	// it is the same instant in every evaluation on that context
	if a, b := evaluate(t, ctx, "now()"), evaluate(t, ctx, "now()"); a != b {
		t.Errorf("a fixed instant moved: %s, %s", a, b)
	}

	// a copy fixed to another instant leaves the context it was copied from
	copied := ctx.WithThis(nil)
	copied.SetNow(at.Add(time.Hour))
	if got := evaluate(t, ctx, "now()"); got != "[2026-03-04T23:59:58.123+09:30]" {
		t.Errorf("fixing a copy changed the original: now() = %s", got)
	}

	// WithNow does the same for EvaluateWithOptions
	result, err := fhirpath.MustCompile("now()").EvaluateWithOptions(patient, fhirpath.WithNow(at))
	if err != nil || result.String() != "[2026-03-04T23:59:58.123+09:30]" {
		t.Errorf("WithNow: now() = %v %v", result, err)
	}
}

func TestADocumentSharedAcrossGoroutinesGivesEachEvaluationOneInstant(t *testing.T) {
	movingClock(t)
	doc, err := fhirpath.NewDocument(patient)
	if err != nil {
		t.Fatal(err)
	}
	expr := fhirpath.MustCompile("(1 | 2 | 3).select(now()).distinct().count() = 1 and timeOfDay() = now().timeOf()")
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 200 {
				result, err := doc.EvaluateWithOptions(expr)
				if err != nil || result.String() != "[true]" {
					t.Errorf("%v %v", result, err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// A clock fixed during an evaluation, by a function that calls SetNow, is not
// undone when the evaluation ends.
func TestAnInstantFixedDuringAnEvaluationStays(t *testing.T) {
	movingClock(t)
	at := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
	funcs.Register(eval.FuncDef{
		Name: "fixNowForClockTest",
		Fn: func(ctx *eval.Context, _ types.Collection, _ []interface{}) (types.Collection, error) {
			ctx.SetNow(at)
			return types.Collection{types.NewBoolean(true)}, nil
		},
	})
	ctx := eval.NewContext(patient)
	if got := evaluate(t, ctx, "fixNowForClockTest() and now() = now()"); got != "[true]" {
		t.Fatalf("= %s", got)
	}
	if got := ctx.Now(); !got.Equal(at) {
		t.Errorf("after the evaluation Now() = %v, want the instant fixed during it, %v", got, at)
	}
	if got := evaluate(t, ctx, "now()"); got != "[2000-01-02T03:04:05.000Z]" {
		t.Errorf("the next evaluation: now() = %s", got)
	}
}
