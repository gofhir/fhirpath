package fhirpath

import (
	"testing"

	"github.com/gofhir/fhirpath/types"
)

// This file locks down the behavior that FHIR invariant evaluation depends on.
// Every case here maps to a reported defect: element names colliding with type
// names, the fixed FHIR environment variables, quantity comparison on JSON
// objects, and singleton-to-Boolean coercion.

// FHIR R4 invariant texts, verbatim.
const (
	ref1 = `reference.startsWith('#').not() or (reference.substring(1).trace('url') in %rootResource.contained.id.trace('ids'))`
	rng2 = `low.empty() or high.empty() or (low <= high)`
	age1 = `(code or value.empty()) and (system.empty() or system = %ucum) and (code.empty() or code = 'a' or code = 'mo' or code = 'wk' or code = 'd' or code = 'h' or code = 'min' or code = 's')`
	cnt3 = `(code.exists() or value.empty()) and (system.empty() or system = %ucum) and (code.empty() or code = '1')`
)

func evalOrFatal(t *testing.T, resource []byte, expr string) types.Collection {
	t.Helper()
	result, err := Evaluate(resource, expr)
	if err != nil {
		t.Fatalf("%s: unexpected error: %v", expr, err)
	}
	return result
}

func assertEmptyResult(t *testing.T, result types.Collection, expr string) {
	t.Helper()
	if !result.Empty() {
		t.Errorf("%s: expected empty collection, got %v", expr, result)
	}
}

// TestReferenceElementNavigation covers an element whose name collides with the
// name of its own inferred type. Any object carrying a "reference" field is
// inferred as a Reference, and type-name navigation used to swallow the field:
// "reference" returned the enclosing object instead of the string, so
// startsWith('#') was always false and ref-1 could never fail.
func TestReferenceElementNavigation(t *testing.T) {
	reference := []byte(`{"reference":"#nope","display":"contained patient"}`)

	t.Run("navigates to the field value, not the object", func(t *testing.T) {
		assertStringResult(t, evalOrFatal(t, reference, "reference"), "#nope")
	})

	t.Run("string functions apply to the field value", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, reference, "reference.startsWith('#')"), true)
		assertStringResult(t, evalOrFatal(t, reference, "reference.substring(1)"), "nope")
		assertIntegerResult(t, evalOrFatal(t, reference, "reference.length()"), 5)
	})

	t.Run("type-name navigation still works for uppercase names", func(t *testing.T) {
		patient := []byte(`{"resourceType":"Patient","id":"p1"}`)
		assertStringResult(t, evalOrFatal(t, patient, "Patient.id"), "p1")
	})

	t.Run("ref-1 fails for a dangling local reference", func(t *testing.T) {
		resource := []byte(`{
			"resourceType": "Observation",
			"subject": {"reference": "#nope"},
			"contained": [{"resourceType": "Patient", "id": "p1"}]
		}`)
		assertBooleanResult(t, evalOrFatal(t, resource, "subject.all("+ref1+")"), false)
	})

	t.Run("ref-1 passes for a resolvable local reference", func(t *testing.T) {
		resource := []byte(`{
			"resourceType": "Observation",
			"subject": {"reference": "#p1"},
			"contained": [{"resourceType": "Patient", "id": "p1"}]
		}`)
		assertBooleanResult(t, evalOrFatal(t, resource, "subject.all("+ref1+")"), true)
	})

	t.Run("ref-1 passes for an external reference", func(t *testing.T) {
		resource := []byte(`{"resourceType":"Observation","subject":{"reference":"Patient/123"}}`)
		assertBooleanResult(t, evalOrFatal(t, resource, "subject.all("+ref1+")"), true)
	})
}

// TestFHIREnvironmentVariables covers the environment variables the FHIR spec
// fixes rather than the caller supplying. Five SHALL invariants (age-1, drt-1,
// cnt-3, dis-1, ras-1) compare against %ucum.
func TestFHIREnvironmentVariables(t *testing.T) {
	cases := []struct {
		expr     string
		expected string
	}{
		{"%ucum", "http://unitsofmeasure.org"},
		{"%sct", "http://snomed.info/sct"},
		{"%loinc", "http://loinc.org"},
		{"%'vs-administrative-gender'", "http://hl7.org/fhir/ValueSet/administrative-gender"},
		{"%`ext-patient-birthTime`", "http://hl7.org/fhir/StructureDefinition/patient-birthTime"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			assertStringResult(t, evalOrFatal(t, simpleJSON, tc.expr), tc.expected)
		})
	}

	t.Run("unknown variable still errors", func(t *testing.T) {
		if _, err := Evaluate(simpleJSON, "%nosuchthing"); err == nil {
			t.Error("expected an error for an undefined variable")
		}
	})

	t.Run("caller can override a fixed constant", func(t *testing.T) {
		expr := MustCompile("%ucum")
		result, err := expr.EvaluateWithOptions(simpleJSON,
			WithVariable("ucum", types.Collection{types.NewString("urn:other")}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "urn:other")
	})

	t.Run("ucum comparison decides the invariant", func(t *testing.T) {
		expr := "system.empty() or system = %ucum"
		ucum := []byte(`{"value":30,"system":"http://unitsofmeasure.org","code":"a"}`)
		other := []byte(`{"value":30,"system":"http://example.org","code":"a"}`)
		none := []byte(`{"value":30,"code":"a"}`)

		assertBooleanResult(t, evalOrFatal(t, ucum, expr), true)
		assertBooleanResult(t, evalOrFatal(t, other, expr), false)
		assertBooleanResult(t, evalOrFatal(t, none, expr), true)
	})
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

// TestQuantityObjectComparison covers quantities carried as JSON objects, which
// is how every FHIR quantity arrives. Range.low and Range.high are both objects,
// so rng-2 used to fail with "cannot apply 'compare' to Quantity and Quantity".
func TestQuantityObjectComparison(t *testing.T) {
	t.Run("rng-2 on an ordered range", func(t *testing.T) {
		rng := []byte(`{
			"low": {"value": 1, "unit": "mg", "system": "http://unitsofmeasure.org", "code": "mg"},
			"high": {"value": 5, "unit": "mg", "system": "http://unitsofmeasure.org", "code": "mg"}
		}`)
		assertBooleanResult(t, evalOrFatal(t, rng, rng2), true)
		assertBooleanResult(t, evalOrFatal(t, rng, "low <= high"), true)
		assertBooleanResult(t, evalOrFatal(t, rng, "low < high"), true)
		assertBooleanResult(t, evalOrFatal(t, rng, "high > low"), true)
	})

	t.Run("rng-2 on an inverted range", func(t *testing.T) {
		rng := []byte(`{"low":{"value":5,"code":"mg"},"high":{"value":1,"code":"mg"}}`)
		assertBooleanResult(t, evalOrFatal(t, rng, rng2), false)
	})

	t.Run("rng-2 with a missing bound", func(t *testing.T) {
		rng := []byte(`{"low":{"value":5,"code":"mg"}}`)
		assertBooleanResult(t, evalOrFatal(t, rng, rng2), true)
	})

	t.Run("commensurable units are converted", func(t *testing.T) {
		rng := []byte(`{
			"low": {"value": 10, "unit": "milligram", "system": "http://unitsofmeasure.org", "code": "mg"},
			"high": {"value": 2, "unit": "gram", "system": "http://unitsofmeasure.org", "code": "g"}
		}`)
		assertBooleanResult(t, evalOrFatal(t, rng, rng2), true)
		assertBooleanResult(t, evalOrFatal(t, rng, "high <= low"), false)
	})

	t.Run("coded unit wins over the display unit", func(t *testing.T) {
		// unit is a display string ("milligram") that no conversion understands;
		// code carries the UCUM symbol
		q := []byte(`{"value":10,"unit":"milligram","system":"http://unitsofmeasure.org","code":"mg"}`)
		assertBooleanResult(t, evalOrFatal(t, q, "$this <= 2 'g'"), true)
		assertBooleanResult(t, evalOrFatal(t, q, "$this = 10 'mg'"), true)
		assertBooleanResult(t, evalOrFatal(t, q, "$this ~ 0.01 'g'"), true)
		assertStringResult(t, evalOrFatal(t, q, "$this.toQuantity().toString()"), "10 'mg'")
		assertBooleanResult(t, evalOrFatal(t, q, "$this.convertsToQuantity()"), true)
		assertBooleanResult(t, evalOrFatal(t, q, "$this.convertsToQuantity('g')"), true)
		assertBooleanResult(t, evalOrFatal(t, q, "$this.convertsToQuantity('m')"), false)
	})

	t.Run("quantity objects support arithmetic", func(t *testing.T) {
		rng := []byte(`{"low":{"value":1,"code":"mg"},"high":{"value":5,"code":"mg"}}`)
		assertStringResult(t, evalOrFatal(t, rng, "(high - low).toString()"), "4 'mg'")
		assertStringResult(t, evalOrFatal(t, rng, "(low + high).toString()"), "6 'mg'")
	})

	t.Run("incommensurable units yield empty, not an error", func(t *testing.T) {
		rng := []byte(`{"low":{"value":1,"code":"cm"},"high":{"value":1,"code":"g"}}`)
		for _, expr := range []string{"low <= high", "low > high", "high - low"} {
			assertEmptyResult(t, evalOrFatal(t, rng, expr), expr)
		}
	})

	t.Run("quantity literals scale by a number", func(t *testing.T) {
		// Documented in examples/quantities-and-units
		assertStringResult(t, evalOrFatal(t, simpleJSON, "(250 'mg' * 4).toString()"), "1000 'mg'")
		assertStringResult(t, evalOrFatal(t, simpleJSON, "(4 * 250 'mg').toString()"), "1000 'mg'")
		assertStringResult(t, evalOrFatal(t, simpleJSON, "(1 'g' / 4).toString()"), "0.25 'g'")
		assertStringResult(t, evalOrFatal(t, simpleJSON, "(1 'kg' - 200 'g').toString()"), "0.8 'kg'")
	})

	t.Run("quantity literals with incommensurable units", func(t *testing.T) {
		for _, expr := range []string{"10 'kg' < 10 'm'", "10 'kg' >= 10 'm'", "10 'kg' + 10 'm'"} {
			assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
		}
		// Equality decides rather than propagating empty
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "10 'kg' = 10 'm'"), false)
	})

	t.Run("objects that are not quantities are unaffected", func(t *testing.T) {
		// A Reference has no numeric "value": comparing it stays an error
		resource := []byte(`{"a":{"reference":"Patient/1"},"b":{"reference":"Patient/2"}}`)
		if _, err := Evaluate(resource, "a <= b"); err == nil {
			t.Error("expected an error comparing two non-quantity objects")
		}
	})
}
