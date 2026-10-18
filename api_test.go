package fhirpath_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/funcs"
	"github.com/gofhir/fhirpath/types"
)

// Evaluate compiles an expression once and reuses it, as EvaluateCached does:
// the convenience function is what the README starts with, and compiling on
// every call cost several times the evaluation.
func TestEvaluateReusesTheCompiledExpression(t *testing.T) {
	expr := "Patient.name.where(use = 'official').given.first()"
	before := fhirpath.DefaultCache.Stats()
	for i := 0; i < 5; i++ {
		if _, err := fhirpath.Evaluate([]byte(`{"resourceType":"Patient"}`), expr); err != nil {
			t.Fatal(err)
		}
	}
	after := fhirpath.DefaultCache.Stats()
	if hits := after.Hits - before.Hits; hits < 4 {
		t.Errorf("five evaluations of one expression hit the cache %d times, want at least 4", hits)
	}
	if _, err := fhirpath.Evaluate([]byte(`{}`), "Patient.("); err == nil {
		t.Error("an expression that does not compile evaluated without an error")
	}
}

// Both caches are read from many goroutines at once — a validator evaluates
// constraints in parallel — and a hit is a read. Run with -race.
func TestCachesCanBeHitFromSeveralGoroutines(t *testing.T) {
	exprs := fhirpath.NewExpressionCache(4)
	regexes := funcs.NewRegexCache(4, 1000, 0)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				// Mostly hits, with misses past the limit to make it evict.
				n := i % 6
				if _, err := exprs.Get(fmt.Sprintf("Patient.id.count() = %d", n)); err != nil {
					t.Error(err)
				}
				if _, err := regexes.Compile(fmt.Sprintf("^a{%d}$", n+1)); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()

	if size := exprs.Size(); size > 4 {
		t.Errorf("expression cache holds %d entries over a limit of 4", size)
	}
	if size := regexes.Size(); size > 4 {
		t.Errorf("regex cache holds %d entries over a limit of 4", size)
	}
}

// Eviction keeps what is used: an entry hit since the hand last passed it is
// spared, and one that was not is evicted, so a hot expression survives a
// stream of one-off ones — expressions built with an id pasted in.
func TestEvictionKeepsTheExpressionsInUse(t *testing.T) {
	c := fhirpath.NewExpressionCache(8)
	hot := "Patient.name.given"
	if _, err := c.Get(hot); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ {
		if _, err := c.Get(hot); err != nil {
			t.Fatal(err)
		}
		if _, err := c.Get(fmt.Sprintf("Patient.id = 'p%d'", i)); err != nil {
			t.Fatal(err)
		}
		if size := c.Size(); size > 8 {
			t.Fatalf("cache holds %d entries over a limit of 8", size)
		}
	}
	before := c.Stats().Misses
	if _, err := c.Get(hot); err != nil {
		t.Fatal(err)
	}
	if c.Stats().Misses != before {
		t.Error("the expression hit on every round was evicted by one-off ones")
	}

	c.Clear()
	if c.Size() != 0 || c.Stats().Hits != 0 {
		t.Errorf("after Clear: size %d, hits %d", c.Size(), c.Stats().Hits)
	}
	if _, err := c.Get(hot); err != nil || c.Size() != 1 {
		t.Errorf("after Clear the cache does not take entries again: size %d, %v", c.Size(), err)
	}
}

func TestNewDocumentRejectsInvalidJSON(t *testing.T) {
	if _, err := fhirpath.NewDocument([]byte("{not json")); err == nil {
		t.Error("expected an error for invalid JSON, got none")
	}
}

func TestCompile(t *testing.T) {
	t.Run("valid expression", func(t *testing.T) {
		expr, err := fhirpath.Compile("Patient.name.given")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if expr.String() != "Patient.name.given" {
			t.Errorf("expected 'Patient.name.given', got '%s'", expr.String())
		}
	})

	t.Run("empty expression", func(t *testing.T) {
		_, err := fhirpath.Compile("")
		if err == nil {
			t.Error("expected error for empty expression")
		}
	})

	t.Run("invalid syntax", func(t *testing.T) {
		_, err := fhirpath.Compile("Patient.name..")
		if err == nil {
			t.Error("expected error for invalid syntax")
		}
	})
}

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
	expr := fhirpath.MustCompile("Patient.managingOrganization.resolve()")
	before := time.Now()
	_, err := expr.EvaluateWithOptions([]byte(`{"resourceType":"Patient","managingOrganization":{"reference":"Organization/1"}}`),
		fhirpath.WithResolver(resolver), fhirpath.WithTimeout(2*time.Second))
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

	_, err := fhirpath.MustCompile("extension.select(%resource.extension.count()).count()").
		EvaluateWithOptions([]byte(b.String()), fhirpath.WithTimeout(time.Millisecond))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

// DefaultOptions hands a caller options it can add variables to directly, as
// it always did.
func TestDefaultOptionsHaveAVariablesMap(t *testing.T) {
	options := fhirpath.DefaultOptions()
	options.Variables["x"] = nil
	if _, ok := options.Variables["x"]; !ok {
		t.Error("DefaultOptions().Variables did not keep a variable")
	}
}

// An option a caller writes itself finds a map for variables, as it always
// did, rather than having to make one as WithVariable does.
func TestACallersOwnOptionCanSetAVariableDirectly(t *testing.T) {
	setX := func(o *fhirpath.EvalOptions) {
		o.Variables["x"] = fhirpath.Collection{}
	}
	if _, err := fhirpath.MustCompile("%x.exists()").EvaluateWithOptions([]byte(`{"resourceType":"Basic"}`), setX); err != nil {
		t.Fatal(err)
	}
}

// NewContext checks the document it is given before reading it, and answers a
// malformed one empty: a document cut off, or one followed by more, is not read
// as though it were whole.
func TestNewContextAnswersAMalformedDocumentEmpty(t *testing.T) {
	expr := fhirpath.MustCompile("id")
	for _, tt := range []struct {
		input, want string
	}{
		{`{"resourceType":"Patient","id":"x"}`, "[x]"},
		{"  \n\t{\"resourceType\":\"Patient\",\"id\":\"x\"}  \n", "[x]"},
		{`{"resourceType":"Patient","id":"x"}  trailing`, "[x]"},
		{`{"resourceType":"Patient","id":"x"`, "[]"},
		{`{"resourceType":"Patient","id":"x",`, "[]"},
		{`{"resourceType":"Patient","id":"x","meta":{"versionId":"1"}`, "[]"},
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

// A document followed by more is read as far as its own value ends, so $this
// is the object alone.
func TestNewContextReadsADocumentAsFarAsItsValue(t *testing.T) {
	result, err := fhirpath.MustCompile("$this").Evaluate([]byte(`{"id":"x"}{"id":"y"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != `[{"id":"x"}]` {
		t.Errorf(`$this = %s, want [{"id":"x"}]`, got)
	}
}

// NewContextForValidJSON reads a document already known to be well formed —
// one its caller has decoded or checked — without scanning it first, and
// answers for it exactly as NewContext does.
func TestNewContextForValidJSONAnswersAsNewContext(t *testing.T) {
	resource := []byte(` {"resourceType":"Patient","id":"x","active":true,` +
		`"name":[{"family":"F","given":["A","B"]}],"_birthDate":{"id":"b"},"birthDate":"2020"} `)
	for _, text := range []string{"id", "active", "name.given", "birthDate.id", "birthDate", "$this.id", "%resource.name.family"} {
		expr := fhirpath.MustCompile(text)
		want, err := expr.EvaluateWithContext(eval.NewContext(resource))
		if err != nil {
			t.Fatal(err)
		}
		got, err := expr.EvaluateWithContext(eval.NewContextForValidJSON(resource))
		if err != nil {
			t.Fatal(err)
		}
		if got.String() != want.String() {
			t.Errorf("%s = %s, want %s as NewContext answers", text, got, want)
		}
	}
}

// testPatient is a minimal Patient struct for testing the Resource interface.
type testPatient struct {
	ResourceType string          `json:"resourceType"`
	ID           string          `json:"id,omitempty"`
	Active       bool            `json:"active,omitempty"`
	Gender       string          `json:"gender,omitempty"`
	BirthDate    string          `json:"birthDate,omitempty"`
	Name         []testHumanName `json:"name,omitempty"`
}

type testHumanName struct {
	Use    string   `json:"use,omitempty"`
	Family string   `json:"family,omitempty"`
	Given  []string `json:"given,omitempty"`
}

func (p testPatient) GetResourceType() string { return "Patient" }

// Test evaluating FHIRPath against JSON bytes
func TestEvaluateJSON(t *testing.T) {
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "example",
		"active": true,
		"name": [
			{
				"use": "official",
				"family": "Smith",
				"given": ["John", "James"]
			}
		],
		"birthDate": "1990-01-15"
	}`)

	tests := []struct {
		name      string
		expr      string
		wantCount int
		wantFirst string
		wantBool  *bool
	}{
		{
			name:      "simple path",
			expr:      "Patient.id",
			wantCount: 1,
			wantFirst: "example",
		},
		{
			name:      "nested path",
			expr:      "Patient.name.family",
			wantCount: 1,
			wantFirst: "Smith",
		},
		{
			name:      "array access",
			expr:      "Patient.name.given",
			wantCount: 2,
			wantFirst: "John",
		},
		{
			name:      "first function",
			expr:      "Patient.name.given.first()",
			wantCount: 1,
			wantFirst: "John",
		},
		{
			name:      "count function",
			expr:      "Patient.name.given.count()",
			wantCount: 1,
			wantFirst: "2",
		},
		{
			name:      "exists function",
			expr:      "Patient.name.exists()",
			wantCount: 1,
			wantBool:  boolPtr(true),
		},
		{
			name:      "empty check",
			expr:      "Patient.telecom.empty()",
			wantCount: 1,
			wantBool:  boolPtr(true),
		},
		{
			name:      "where filter",
			expr:      "Patient.name.where(use = 'official').family",
			wantCount: 1,
			wantFirst: "Smith",
		},
		{
			name:      "boolean field",
			expr:      "Patient.active",
			wantCount: 1,
			wantBool:  boolPtr(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fhirpath.Evaluate(patient, tt.expr)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}

			if len(result) != tt.wantCount {
				t.Errorf("got %d results, want %d", len(result), tt.wantCount)
			}

			if tt.wantFirst != "" && len(result) > 0 {
				if got := result[0].String(); got != tt.wantFirst {
					t.Errorf("first result = %q, want %q", got, tt.wantFirst)
				}
			}

			if tt.wantBool != nil && len(result) > 0 {
				got, err := result.ToBoolean()
				if err != nil {
					t.Errorf("ToBoolean() error = %v", err)
				} else if got != *tt.wantBool {
					t.Errorf("boolean result = %v, want %v", got, *tt.wantBool)
				}
			}
		})
	}
}

// Test evaluating against Go structs using EvaluateResource
func TestEvaluateResource(t *testing.T) {
	patient := testPatient{
		ResourceType: "Patient",
		ID:           "test-patient",
		Active:       true,
		Gender:       "female",
		BirthDate:    "1985-06-20",
		Name: []testHumanName{
			{Use: "official", Family: "Doe", Given: []string{"Jane", "Marie"}},
		},
	}

	// Test with EvaluateResource - should work now that MarshalJSON includes resourceType
	result, err := fhirpath.EvaluateResource(patient, "Patient.name.given.first()")
	if err != nil {
		t.Fatalf("EvaluateResource() error = %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("got %d results, want 1", len(result))
	}

	if got := result[0].String(); got != "Jane" {
		t.Errorf("got %q, want %q", got, "Jane")
	}

	// Verify JSON serialization includes resourceType
	jsonBytes, err := json.Marshal(patient)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &data); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if data["resourceType"] != "Patient" {
		t.Errorf("resourceType = %v, want Patient", data["resourceType"])
	}
}

// Test ResourceJSON wrapper for efficient repeated evaluation
func TestResourceJSON(t *testing.T) {
	patient := testPatient{
		ResourceType: "Patient",
		ID:           "cached-patient",
		Name:         []testHumanName{{Family: "Cached"}},
	}

	rj, err := fhirpath.NewResourceJSON(patient)
	if err != nil {
		t.Fatalf("NewResourceJSON() error = %v", err)
	}

	// Evaluate multiple expressions efficiently
	expressions := []string{
		"Patient.id",
		"Patient.name.family",
		"Patient.name.exists()",
	}

	for _, expr := range expressions {
		result, err := rj.EvaluateCached(expr)
		if err != nil {
			t.Errorf("EvaluateCached(%q) error = %v", expr, err)
		}
		if result.Empty() {
			t.Errorf("EvaluateCached(%q) returned empty", expr)
		}
	}
}

// Test expression caching
func TestExpressionCache(t *testing.T) {
	cache := fhirpath.NewExpressionCache(100)

	patient := []byte(`{"resourceType": "Patient", "id": "test"}`)

	// First call compiles and caches
	expr1, err := cache.Get("Patient.id")
	if err != nil {
		t.Fatalf("cache.Get() error = %v", err)
	}

	// Second call should return cached
	expr2, err := cache.Get("Patient.id")
	if err != nil {
		t.Fatalf("cache.Get() second call error = %v", err)
	}

	// Should be the same pointer
	if expr1 != expr2 {
		t.Error("cache should return same expression instance")
	}

	// Verify it works
	result, err := expr1.Evaluate(patient)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}

	if result[0].String() != "test" {
		t.Errorf("got %q, want %q", result[0].String(), "test")
	}

	// Check cache size
	if cache.Size() != 1 {
		t.Errorf("cache size = %d, want 1", cache.Size())
	}
}

// Test evaluation with options
func TestEvaluateWithOptions(t *testing.T) {
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "options-test",
		"name": [{"family": "Test"}]
	}`)

	expr := fhirpath.MustCompile("Patient.id")

	// Test with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result, err := expr.EvaluateWithOptions(patient,
		fhirpath.WithContext(ctx),
		fhirpath.WithTimeout(1*time.Second),
		fhirpath.WithMaxDepth(50),
	)

	if err != nil {
		t.Fatalf("EvaluateWithOptions() error = %v", err)
	}

	if result[0].String() != "options-test" {
		t.Errorf("got %q, want %q", result[0].String(), "options-test")
	}
}

// Test helper functions
func TestHelperFunctions(t *testing.T) {
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "helper-test",
		"active": true,
		"name": [{"family": "Helper"}, {"family": "Test"}]
	}`)

	t.Run("EvaluateToBoolean", func(t *testing.T) {
		result, err := fhirpath.EvaluateToBoolean(patient, "Patient.active")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !result {
			t.Error("expected true")
		}
	})

	t.Run("EvaluateToString", func(t *testing.T) {
		result, err := fhirpath.EvaluateToString(patient, "Patient.id")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if result != "helper-test" {
			t.Errorf("got %q, want %q", result, "helper-test")
		}
	})

	t.Run("EvaluateToStrings", func(t *testing.T) {
		result, err := fhirpath.EvaluateToStrings(patient, "Patient.name.family")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if len(result) != 2 {
			t.Errorf("got %d results, want 2", len(result))
		}
	})

	t.Run("Exists", func(t *testing.T) {
		result, err := fhirpath.Exists(patient, "Patient.name")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !result {
			t.Error("expected true")
		}
	})

	t.Run("Count", func(t *testing.T) {
		result, err := fhirpath.Count(patient, "Patient.name")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if result != 2 {
			t.Errorf("got %d, want 2", result)
		}
	})
}

// TestTypeArgsCustomFunction verifies that custom functions with TypeArgs
// receive type specifier names as strings instead of evaluating them as expressions.
func TestTypeArgsCustomFunction(t *testing.T) {
	// Register a custom function that expects a type name as first arg
	funcs.Register(eval.FuncDef{
		Name:     "testTypeArg",
		MinArgs:  1,
		MaxArgs:  1,
		TypeArgs: []int{0},
		Fn: func(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
			if col, ok := args[0].(types.Collection); ok && !col.Empty() {
				return col, nil // Return the type name string
			}
			return types.Collection{}, nil
		},
	})

	patient := []byte(`{"resourceType": "Patient", "id": "test"}`)
	result, err := fhirpath.Evaluate(patient, "Patient.testTypeArg(Observation)")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if result.Empty() {
		t.Fatal("expected non-empty result, got empty (type specifier was not passed)")
	}

	if result[0].String() != "Observation" {
		t.Errorf("expected 'Observation', got %q", result[0].String())
	}
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

// A root a caller reads once and evaluates against from several goroutines is
// only read, never written: it does not cache, and only an object that caches
// is documented as single-goroutine. Working out its type and reading a field
// through Get used to write to it. Run with -race.
func TestASharedRootCanBeReadFromSeveralGoroutines(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","id":"p","gender":"female",` +
		`"extension":[{"url":"http://example.org/e","valueCode":"x"}],` +
		`"name":[{"family":"F","given":["A","B"]}],"managingOrganization":{"reference":"#o"}}`)
	shared, err := types.JSONToCollection(patient)
	if err != nil {
		t.Fatal(err)
	}

	exprs := []*fhirpath.Expression{
		fhirpath.MustCompile("id | gender | name.family | name.given"),
		fhirpath.MustCompile("Patient.name.given.count()"),
		fhirpath.MustCompile("extension('http://example.org/e').value"),
		fhirpath.MustCompile("$this is Patient"),
	}

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				for _, expr := range exprs {
					if _, err := expr.EvaluateWithContext(eval.NewContextForRoot(shared)); err != nil {
						t.Error(err)
					}
				}
			}
		}()
	}
	wg.Wait()
}

// What an evaluation returns is handed to the caller, who may share it across
// goroutines as a root of its own: Bundle.entry.resource read once, each
// resource then validated in parallel. Objects private while the evaluation
// read them are not private once returned. Run with -race.
func TestAReturnedObjectCanBeSharedAcrossGoroutines(t *testing.T) {
	bundle := []byte(`{"resourceType":"Bundle","type":"collection","entry":[` +
		`{"resource":{"resourceType":"Patient","id":"p","gender":"female","name":[{"family":"F"}],` +
		`"extension":[{"url":"http://example.org/e","valueCode":"x"}]}}]}`)
	resources, err := fhirpath.MustCompile("entry.resource").Evaluate(bundle)
	if err != nil || len(resources) != 1 {
		t.Fatalf("entry.resource = %v, %v", resources, err)
	}

	exprs := []*fhirpath.Expression{
		fhirpath.MustCompile("id | gender | name.family"),
		fhirpath.MustCompile("$this is Patient"),
		fhirpath.MustCompile("extension('http://example.org/e').value"),
	}
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				for _, expr := range exprs {
					if _, err := expr.EvaluateWithContext(eval.NewContextForRoot(resources)); err != nil {
						t.Error(err)
					}
				}
			}
		}()
	}
	wg.Wait()
}

// An expression can return the root it was given — $this, a where() keeping
// it, %resource — and a shared root comes back shared without being written
// to. Run with -race.
func TestReturningASharedRootDoesNotWriteToIt(t *testing.T) {
	shared, err := types.JSONToCollection([]byte(`{"resourceType":"Patient","id":"p","active":true}`))
	if err != nil {
		t.Fatal(err)
	}
	exprs := []*fhirpath.Expression{
		fhirpath.MustCompile("$this"),
		fhirpath.MustCompile("%resource"),
		fhirpath.MustCompile("where(active)"),
		fhirpath.MustCompile("$this is Patient"),
	}

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				for _, expr := range exprs {
					if _, err := expr.EvaluateWithContext(eval.NewContextForRoot(shared)); err != nil {
						t.Error(err)
					}
				}
			}
		}()
	}
	wg.Wait()
}
