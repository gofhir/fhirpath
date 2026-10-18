package fhirpath_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/funcs"
	"github.com/gofhir/fhirpath/types"
)

// TestArgumentsEvaluateInTheOuterScope covers where a function's arguments are
// navigated from.
//
// A function's input is what precedes the dot, but its arguments are not
// navigated from there: in name.given.combine(name.family), family belongs to
// name, not to given. The conformance suite settles it by giving these two the
// same expected result, one written with an explicit $this and one without:
//
//	name.given.combine(name.family)
//	name.given.combine($this.name.family)
//
// Which means the scope in force before the dot is the one the argument sees.
func TestArgumentsEvaluateInTheOuterScope(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		// Five given names and two family names
		{"name.given.combine(name.family).count()", "7"},
		{"name.given.combine($this.name.family).count()", "7"},
		{"name.given.combine(name.family).exclude('Jim').count()", "6"},

		// Inside a projection the scope is the item being projected, so use and
		// given are both read from the same name
		{"name.select(use.union(given)).count()", "8"},
		{"name.select(use.union($this.given)).count()", "8"},
		{"name.first().select(use.union(given)).count()", "3"},

		// The input is still the input: it is only the arguments that are
		// navigated from outside
		{"name.given.count()", "5"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, threeNames); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestSingletonBooleanCoercion covers the FHIRPath "Singleton Evaluation of
// Collections" rule: where a Boolean is expected, a single node of any other
// type evaluates to true. FHIR relies on it — age-1 and drt-1 open with
// "(code or value.empty())", where code is a string, and used to evaluate to
// empty instead of true.
func TestSingletonBooleanCoercion(t *testing.T) {
	resource := []byte(`{"code":"a","value":30,"flag":false}`)

	t.Run("or with a string operand", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, resource, "code or false"), true)
	})

	t.Run("and with a string operand", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, resource, "code and true"), true)
		assertBooleanResult(t, evalOrFatal(t, resource, "code and false"), false)
	})

	t.Run("not on a string operand", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, resource, "code.not()"), false)
	})

	t.Run("iif with a string condition", func(t *testing.T) {
		assertStringResult(t, evalOrFatal(t, resource, "iif(code, 'yes', 'no')"), "yes")
	})

	t.Run("where with a non-boolean criteria", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, resource, "where(code).exists()"), true)
	})

	t.Run("boolean nodes keep their own value", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, resource, "flag or false"), false)
		assertBooleanResult(t, evalOrFatal(t, resource, "flag.not()"), true)
	})

	t.Run("empty still propagates", func(t *testing.T) {
		expr := "missing or missing"
		assertEmptyResult(t, evalOrFatal(t, resource, expr), expr)
	})

	t.Run("age-1 on a valid Age", func(t *testing.T) {
		age := []byte(`{"value":30,"unit":"years","system":"http://unitsofmeasure.org","code":"a"}`)
		assertBooleanResult(t, evalOrFatal(t, age, age1), true)
	})

	t.Run("age-1 rejects a non-time code", func(t *testing.T) {
		age := []byte(`{"value":30,"unit":"milligram","system":"http://unitsofmeasure.org","code":"mg"}`)
		assertBooleanResult(t, evalOrFatal(t, age, age1), false)
	})

	t.Run("cnt-3 rejects a non-unity code", func(t *testing.T) {
		count := []byte(`{"value":3,"system":"http://unitsofmeasure.org","code":"mg"}`)
		assertBooleanResult(t, evalOrFatal(t, count, cnt3), false)
	})

	t.Run("cnt-3 accepts the unity code", func(t *testing.T) {
		count := []byte(`{"value":3,"system":"http://unitsofmeasure.org","code":"1"}`)
		assertBooleanResult(t, evalOrFatal(t, count, cnt3), true)
	})
}

func TestLiterals(t *testing.T) {
	t.Run("boolean true", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "true")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("boolean false", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "false")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, false)
	})

	t.Run("integer", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "42")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 42)
	})

	t.Run("decimal", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "3.14")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Empty() || result[0].Type() != "Decimal" {
			t.Errorf("expected Decimal, got %v", result)
		}
	})

	t.Run("string", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "'hello world'")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "hello world")
	})

	t.Run("empty collection", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "{}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Empty() {
			t.Errorf("expected empty collection, got %v", result)
		}
	})
}

func TestEmptyPropagation(t *testing.T) {
	t.Run("empty + value", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "{} + 5")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Empty() {
			t.Errorf("expected empty, got %v", result)
		}
	})

	t.Run("empty and true", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "{} and true")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// In three-valued logic, empty and true = empty
		if !result.Empty() {
			t.Errorf("expected empty, got %v", result)
		}
	})

	t.Run("empty and false", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "{} and false")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// In three-valued logic, empty and false = false
		assertBooleanResult(t, result, false)
	})
}

func TestParentheses(t *testing.T) {
	t.Run("precedence override", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "(2 + 3) * 4")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 20)
	})

	t.Run("default precedence", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "2 + 3 * 4")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 14)
	})
}

// TestIifLazyEvaluation tests that iif() only evaluates the matching branch.
func TestIifLazyEvaluation(t *testing.T) {
	t.Run("iif true branch only", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "iif(true, 'yes', 'no')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "yes")
	})

	t.Run("iif false branch only", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "iif(false, 'yes', 'no')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "no")
	})

	t.Run("iif with no else returns empty", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "iif(false, 'yes')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Empty() {
			t.Errorf("expected empty result, got %v", result)
		}
	})

	t.Run("iif with expression criterion", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "iif(1 > 0, 'positive', 'negative')")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "positive")
	})

	t.Run("iif returns numeric values", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "iif(true, 42, 0)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 42)
	})
}

// TestContextVariable tests the %context environment variable.
func TestContextVariable(t *testing.T) {
	t.Run("%context returns root resource", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "%context.resourceType")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "Patient")
	})

	t.Run("%context equals %resource for top-level evaluation", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "%context = %resource")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("%context.id returns resource id", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "%context.id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "123")
	})
}

// TestRootResourceVariable tests the %rootResource environment variable.
func TestRootResourceVariable(t *testing.T) {
	t.Run("%rootResource returns root resource", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "%rootResource.resourceType")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "Patient")
	})

	t.Run("%rootResource equals %resource for top-level evaluation", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "%rootResource = %resource")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("%rootResource can be overridden via WithVariable", func(t *testing.T) {
		bundle := []byte(`{"resourceType": "Bundle", "type": "transaction"}`)
		patient := []byte(`{"resourceType": "Patient", "id": "p1"}`)

		expr := fhirpath.MustCompile("%rootResource.resourceType")
		bundleCol, _ := types.JSONToCollection(bundle)
		result, err := expr.EvaluateWithOptions(patient, fhirpath.WithVariable("rootResource", bundleCol))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "Bundle")
	})
}

// TestDelimitedIdentifiers tests backtick-delimited identifiers for special characters.
func TestDelimitedIdentifiers(t *testing.T) {
	// JSON with hyphenated field names
	jsonWithSpecialFields := []byte(`{
		"resourceType": "Patient",
		"PID-1": "12345",
		"PID-2": "67890",
		"normal_field": "value"
	}`)

	t.Run("backtick identifier with hyphen", func(t *testing.T) {
		result, err := fhirpath.Evaluate(jsonWithSpecialFields, "Patient.`PID-1`")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "12345")
	})

	t.Run("backtick identifier second field", func(t *testing.T) {
		result, err := fhirpath.Evaluate(jsonWithSpecialFields, "Patient.`PID-2`")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "67890")
	})

	t.Run("normal identifier still works", func(t *testing.T) {
		result, err := fhirpath.Evaluate(jsonWithSpecialFields, "Patient.normal_field")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "value")
	})

	t.Run("backtick with normal field name", func(t *testing.T) {
		result, err := fhirpath.Evaluate(jsonWithSpecialFields, "Patient.`normal_field`")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "value")
	})
}

// TestDefineVariable covers the one function that exists for its effect on the
// context rather than its result.
func TestDefineVariable(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		// The output is the input, so the function is transparent to its chain
		{"item.defineVariable('x').linkId.count()", "2"},

		// The name is visible further along the same expression
		{"defineVariable('root', 'Q').item.select(%root).count()", "2"},

		// The shape the specification's own example uses: a name bound at one
		// level and read from inside a nested projection
		{"item.first().defineVariable('top', linkId).item.select(%top & '/' & linkId).first()", "1/1.1"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, nestedQuestionnaire); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestDefineVariablePerIterationScope checks that each element of an iteration
// starts from the same scope.
//
// Without that, the second name would find 'n' already defined and the
// evaluation would fail — which is why the specification describes the variable
// as popped off the stack when the expression completes.
func TestDefineVariablePerIterationScope(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[{"family":"A"},{"family":"B"}]}`)

	result, err := fhirpath.MustCompile("name.select(defineVariable('n', family).select(%n))").Evaluate(patient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 || result[0].String() != "A" || result[1].String() != "B" {
		t.Errorf("got %v, want [A B]", result)
	}
}

// TestDefineVariableRedefinition checks the error the specification requires:
// "If the name already exists in the current expression scope, the evaluation
// will end and signal an error to the calling environment."
func TestDefineVariableRedefinition(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[{"family":"A"}]}`)

	for _, expr := range []string{
		"defineVariable('x').defineVariable('x').count()",
		// The environment's own variables are in scope, so they cannot be shadowed
		"defineVariable('resource').count()",
		"defineVariable('ucum').count()",
	} {
		t.Run(expr, func(t *testing.T) {
			if _, err := fhirpath.MustCompile(expr).Evaluate(patient); err == nil {
				t.Errorf("%s: expected an error", expr)
			}
		})
	}
}

// The specification states the rule these tests cover as an algorithm, under
// "Singleton Evaluation of Collections":
//
//	IF the collection contains a single node AND the node's value can be
//	  implicitly converted to the expected input type THEN
//	  The collection evaluates to the value of that single node
//	ELSE IF the collection contains a single node AND the expected input type
//	  is Boolean THEN the collection evaluates to true
//	ELSE IF the collection is empty THEN an empty collection
//	ELSE The evaluation will end and signal an error
//
// Two of those branches were missing. A collection of several items was being
// answered rather than refused, and an item of the wrong type was being coerced
// — an Identifier rendered as JSON text and then asked whether it starts with
// something, which yields a confident false about a question that was never
// meaningful.

var appointment = []byte(`{
  "resourceType":"Appointment",
  "identifier":[{"system":"http://example.org/sampleappointment-identifier","value":"123"}],
  "status":"proposed"
}`)

// TestSingletonRuleRejectsMultipleItems covers the final branch: more than one
// item is an error, because which of them was meant is not something to guess.
func TestSingletonRuleRejectsMultipleItems(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	for _, expr := range []string{
		"(1 | 2).not()",
		"(1 | 2 | 3) & 'b'",
		"('a' | 'b').startsWith('a')",
		"('a' | 'b').length()",
		"(1 | 2) or false",
		"{} or (1 | 2)",
		"true and (1 | 2)",
		"(1 | 2) and true",
		"(1 | 2) xor true",
		"true implies (1 | 2)",
		"iif((true | false), 1, 2)",
		"iif(1 | 2 | 3, true, false)",
	} {
		t.Run(expr, func(t *testing.T) {
			if _, err := fhirpath.MustCompile(expr).Evaluate(patient); err == nil {
				t.Errorf("%s: expected an error for a multi-item input", expr)
			}
		})
	}
}

// TestSingletonRuleRejectsWrongType covers the case of a single item that is
// not of the expected type and cannot convert to it.
//
// Nothing converts implicitly to String — the specification's implicit
// conversions are Integer to Long to Decimal to Quantity, and Date to DateTime —
// so an Identifier or an Integer is an error here, not a string to test.
func TestSingletonRuleRejectsWrongType(t *testing.T) {
	for _, expr := range []string{
		"Appointment.identifier.startsWith('rand')",
		"Appointment.identifier.endsWith('rand')",
		"Appointment.identifier.contains('rand')",
	} {
		t.Run(expr, func(t *testing.T) {
			if _, err := fhirpath.MustCompile(expr).Evaluate(appointment); err == nil {
				t.Errorf("%s: expected an error for a non-String input", expr)
			}
		})
	}
}

// TestSingletonRuleKeepsTheOtherBranches checks that the two branches which do
// have an answer still give it.
func TestSingletonRuleKeepsTheOtherBranches(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[{"given":["Jim"]}]}`)

	cases := []struct {
		expr string
		want string
	}{
		// A single node of the expected type
		{"'abc'.startsWith('a')", "true"},
		{"true.not()", "false"},
		{"'a' & 'b'", "ab"},
		{"name.given.startsWith('J')", "true"},

		// A single node where a Boolean is expected evaluates to true
		{"name.given.not()", "false"},

		// Empty stays empty — except for &, which the specification defines to
		// treat an empty operand as an empty string
		{"{}.startsWith('a')", "EMPTY"},
		{"{}.not()", "EMPTY"},
		{"{} & 'b'", "b"},
		{"'a' & {}", "a"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestIifCriterionSingleton covers iif's criterion, which FHIRPath 3.0.0 says
// "SHALL evaluate to a Boolean, consistent with singleton evaluation of
// collections". Several items were read as false and the otherwise branch
// returned, so iif((true | false), 1, 2) answered 2.
func TestIifCriterionSingleton(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	for _, tt := range []struct{ expr, want string }{
		{"iif(true, 1, 2)", "1"},
		{"iif(false, 1, 2)", "2"},
		{"iif({}, 1, 2)", "2"},
		{"iif('x', 1, 2)", "1"},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tt.expr, patient); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}
		})
	}

	_, err := fhirpath.MustCompile("iif((true | false), 1, 2)").Evaluate(patient)
	if err == nil || !strings.Contains(err.Error(), "iif() expects a single item as its criterion, got 2") {
		t.Errorf("error %v, want it to name iif's criterion", err)
	}
}

// TestIifTakesASingleItemContext covers the arity rule iif states for itself:
// "Unlike most other functions it can be called with no context ... or with a
// single item context. If the input collection contains multiple items, the
// evaluation of the expression will end and signal an error."
func TestIifTakesASingleItemContext(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	if _, err := fhirpath.MustCompile("('item1' | 'item2').iif(true, 'yes', 'no')").Evaluate(patient); err == nil {
		t.Error("expected an error for a multi-item context")
	}

	for _, tc := range []struct{ expr, want string }{
		{"iif(true, 'yes', 'no')", "yes"},     // no context
		{"'x'.iif(true, 'yes', 'no')", "yes"}, // a single item
		{"{}.iif(true, 'yes', 'no')", "yes"},  // empty: the criterion still decides
		{"iif(false, 'yes', 'no')", "no"},
	} {
		if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
			t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
		}
	}
}

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
		if _, err := fhirpath.Evaluate(doc, expr); err != nil {
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

// Test %resource environment variable
func TestResourceVariable(t *testing.T) {
	bundle := []byte(`{
		"resourceType": "Bundle",
		"type": "transaction",
		"entry": [
			{"resource": {"resourceType": "Patient", "id": "p1"}},
			{"resource": {"resourceType": "Observation", "id": "o1"}}
		]
	}`)

	tests := []struct {
		name      string
		expr      string
		wantCount int
		wantFirst string
	}{
		{
			name:      "resource variable returns root",
			expr:      "%resource.resourceType",
			wantCount: 1,
			wantFirst: "Bundle",
		},
		{
			name:      "resource type from root",
			expr:      "%resource.type",
			wantCount: 1,
			wantFirst: "transaction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := fhirpath.Compile(tt.expr)
			if err != nil {
				t.Fatalf("Compile(%q) error = %v", tt.expr, err)
			}

			result, err := expr.Evaluate(bundle)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}

			if len(result) != tt.wantCount {
				t.Errorf("got %d results, want %d", len(result), tt.wantCount)
			}

			if tt.wantFirst != "" && len(result) > 0 {
				if result[0].String() != tt.wantFirst {
					t.Errorf("got %q, want %q", result[0].String(), tt.wantFirst)
				}
			}
		})
	}
}

// An operator given more than one item on a side says which side, and how many
// each had. Adding the two counts up reported (1 | 2) + 1 as "got 3 elements",
// which names neither the operand at fault nor what it held.
func TestOperandSingletonErrorNamesTheSide(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{"(1 | 2) + 1", "+ expects a single item on each side, got 2 on the left and 1 on the right"},
		{"1 - (1 | 2 | 3)", "- expects a single item on each side, got 1 on the left and 3 on the right"},
		{"(1 | 2) * 3", "* expects a single item on each side, got 2 on the left and 1 on the right"},
		{"(1 | 2) < 3", "< expects a single item on each side, got 2 on the left and 1 on the right"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			_, err := fhirpath.Evaluate([]byte(`{"resourceType":"Basic"}`), tt.expr)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("%s: error %v, want it to say %q", tt.expr, err, tt.want)
			}
		})
	}
}

// A boolean operator is held to the same rule. Its three-valued logic answers
// an empty operand, but more than one item is an error like any other: US Core's
// pd-1, telecom or endpoint, was empty on a PractitionerRole with two telecoms,
// which a validator reads as the invariant failing rather than as an error.
func TestBooleanOperandSingleton(t *testing.T) {
	role := []byte(`{"resourceType":"PractitionerRole","telecom":[{"value":"1"},{"value":"2"}]}`)

	for _, tt := range []struct{ expr, want string }{
		{"telecom or endpoint", "or expects a single item on each side, got 2 on the left"},
		{"telecom and true", "and expects a single item on each side, got 2 on the left"},
		{"false xor telecom", "xor expects a single item on each side, got 1 on the left and 2 on the right"},
		{"telecom implies true", "implies expects a single item on each side, got 2 on the left"},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			_, err := fhirpath.Evaluate(role, tt.expr)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("%s: error %v, want it to say %q", tt.expr, err, tt.want)
			}
		})
	}

	// A left operand of several items is refused before the right one is
	// evaluated, so an error the right one would raise does not replace it.
	for _, expr := range []string{
		"telecom and ('a' | 'b').length()",
		"telecom or ('a' | 'b').length()",
		"telecom xor ('a' | 'b').length()",
		"telecom implies ('a' | 'b').length()",
	} {
		t.Run(expr, func(t *testing.T) {
			_, err := fhirpath.Evaluate(role, expr)
			if err == nil || !strings.Contains(err.Error(), "expects a single item on each side, got 2 on the left") {
				t.Errorf("%s: error %v, want the operator's own", expr, err)
			}
		})
	}

	// A left operand that decides the result still leaves the right one
	// unevaluated, so its count is never seen.
	for _, tt := range []struct{ expr, want string }{
		{"true or telecom", "true"},
		{"false and telecom", "false"},
		{"false implies telecom", "true"},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := fhirpath.Evaluate(role, tt.expr)
			if err != nil || len(got) != 1 || got[0].String() != tt.want {
				t.Errorf("%s = %v, %v; want %s", tt.expr, got, err, tt.want)
			}
		})
	}
}

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
