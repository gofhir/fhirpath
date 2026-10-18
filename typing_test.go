package fhirpath_test

import (
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

func TestTypeOperators(t *testing.T) {
	t.Run("is type check", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "Patient.active is Boolean")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("as type cast", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patientJSON, "Patient.active as boolean")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	// as() of a FHIR primitive to a System type answers as is() does; with
	// the generated models both say no: see conformance/system_typed_test.go
	t.Run("as a System type on a FHIR primitive", func(t *testing.T) {
		for _, expr := range []string{"Patient.active as Boolean", "Patient.active.as(System.Boolean)"} {
			is := strings.Replace(strings.Replace(expr, " as ", " is ", 1), ".as(", ".is(", 1)
			isResult, err := fhirpath.Evaluate(patientJSON, is)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", is, err)
			}
			asResult, err := fhirpath.Evaluate(patientJSON, expr)
			if err != nil {
				t.Fatalf("%s: unexpected error: %v", expr, err)
			}
			if got, want := !asResult.Empty(), isResult.String() == "[true]"; got != want {
				t.Errorf("%s found %v, where %s = %v", expr, asResult, is, isResult)
			}
		}
	})
}

var bundleWithMixedResources = []byte(`{
	"resourceType": "Bundle",
	"type": "collection",
	"entry": [
		{
			"resource": {
				"resourceType": "Patient",
				"id": "p1",
				"name": [{"family": "Doe"}]
			}
		},
		{
			"resource": {
				"resourceType": "Observation",
				"id": "obs1",
				"status": "final"
			}
		},
		{
			"resource": {
				"resourceType": "Patient",
				"id": "p2",
				"name": [{"family": "Smith"}]
			}
		}
	]
}`)

func TestOfTypeFunction(t *testing.T) {
	t.Run("ofType filters by Quantity", func(t *testing.T) {
		result, err := fhirpath.Evaluate(observationWithQuantity, "Observation.value.ofType(Quantity)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Empty() {
			t.Fatal("expected non-empty result")
		}
		if result[0].Type() != "Quantity" {
			t.Errorf("expected Quantity type, got %s", result[0].Type())
		}
	})

	t.Run("ofType filters by String", func(t *testing.T) {
		result, err := fhirpath.Evaluate(observationWithString, "Observation.value.ofType(String)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Empty() {
			t.Fatal("expected non-empty result")
		}
		assertStringResult(t, result, "Normal findings")
	})

	t.Run("ofType returns empty for non-matching type", func(t *testing.T) {
		result, err := fhirpath.Evaluate(observationWithQuantity, "Observation.value.ofType(String)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Empty() {
			t.Errorf("expected empty result, got %v", result)
		}
	})

	t.Run("ofType filters CodeableConcept", func(t *testing.T) {
		result, err := fhirpath.Evaluate(observationWithCodeableConcept, "Observation.value.ofType(CodeableConcept)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Empty() {
			t.Fatal("expected non-empty result")
		}
		if result[0].Type() != "CodeableConcept" {
			t.Errorf("expected CodeableConcept type, got %s", result[0].Type())
		}
	})

	t.Run("ofType filters resources in Bundle", func(t *testing.T) {
		result, err := fhirpath.Evaluate(bundleWithMixedResources, "Bundle.entry.resource.ofType(Patient)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Count() != 2 {
			t.Errorf("expected 2 Patient resources, got %d", result.Count())
		}
	})

	t.Run("ofType filters to single resource type", func(t *testing.T) {
		result, err := fhirpath.Evaluate(bundleWithMixedResources, "Bundle.entry.resource.ofType(Observation)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Count() != 1 {
			t.Errorf("expected 1 Observation resource, got %d", result.Count())
		}
	})

	t.Run("ofType on empty returns empty", func(t *testing.T) {
		result, err := fhirpath.Evaluate(simpleJSON, "{}.ofType(String)")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Empty() {
			t.Errorf("expected empty result, got %v", result)
		}
	})
}

// as() answers as is() does for a FHIR primitive asked for as a System type:
// without a model a string read from the resource is taken for a String, and
// as(String) keeps it, where with a model it is a FHIR string and is not.
func TestAsAnswersAsIsWithoutAModel(t *testing.T) {
	doc := []byte(`{"resourceType":"Patient","name":[{"family":"F"}]}`)
	for expr, want := range map[string]string{
		"Patient.name.family is String":          "true",
		"Patient.name.family.as(String).count()": "1",
		"(Patient.name.family is System.String) = Patient.name.family.as(System.String).exists()": "true",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// System.Any is the root of the System types, "an ultimate root type", so
// every value is one and ofType(System.Any) keeps every item.
func TestSystemAnyIsEveryValue(t *testing.T) {
	for expr, want := range map[string]string{
		"1.combine('a').combine(true).ofType(System.Any).count()": "3",
		"@2012-01-01.combine(2.5).ofType(System.Any).count()":     "2",
		"1.ofType(Any).count()":                                   "1",
		// a FHIR resource or element too: the root of every type
		"Patient.is(System.Any)":                      "true",
		"Patient.name.ofType(System.Any).count() > 0": "true",
		"'a'.is(System.Any)":                          "true",
		"(5 'mg').is(System.Any)":                     "true",
		"Patient.name.given.ofType(System.Any).count() = Patient.name.given.count()": "true",
	} {
		if got := evaluateScalar(t, expr, patientJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// TestCastOnFHIRPrimitives covers is() and as()/ofType() over FHIR primitives,
// which behave differently on purpose.
//
// FHIR states that "all primitives are considered to be independent types (so
// markdown is not a subclass of string)". A cast therefore requires the type the
// element was declared with, while is() walks the hierarchy the
// StructureDefinitions describe.
func TestCastOnFHIRPrimitives(t *testing.T) {
	coding := []byte(`{"system":"http://loinc.org","code":"1234-5"}`)

	t.Run("a cast requires the declared type", func(t *testing.T) {
		// code is what the model declares, so that is what casting accepts
		result, err := fhirpath.MustCompile("code.as(code)").EvaluateWithOptions(coding, fhirpath.WithModel(typedChildrenModel{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "1234-5")

		for _, expr := range []string{"code.as(string)", "code.ofType(string)", "code.as(id)"} {
			t.Run(expr, func(t *testing.T) {
				result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(coding, fhirpath.WithModel(typedChildrenModel{}))
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				assertEmptyResult(t, result, expr)
			})
		}
	})

	t.Run("is walks the hierarchy", func(t *testing.T) {
		// code derives from string in the StructureDefinitions, so is() says yes
		// where a cast says no
		result, err := fhirpath.MustCompile("code.is(string)").EvaluateWithOptions(coding, fhirpath.WithModel(typedChildrenModel{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertBooleanResult(t, result, true)
	})

	t.Run("structured values still cast by hierarchy", func(t *testing.T) {
		// An Age is a Quantity, and ofType selects it as one
		age := []byte(`{"resourceType":"Observation","valueAge":{"value":30,"code":"a"}}`)
		result, err := fhirpath.MustCompile("valueAge.ofType(Quantity).count()").
			EvaluateWithOptions(age, fhirpath.WithModel(typedChildrenModel{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 1)
	})

	t.Run("a value with only a system type keeps the permissive match", func(t *testing.T) {
		// Without a model narrowing it, an instant reaches the engine as a
		// DateTime and claims no FHIR type of its own
		observation := []byte(`{"resourceType":"Observation","effectiveInstant":"2015-02-07T13:28:17Z"}`)
		assertIntegerResult(t, evalOrFatal(t, observation, "Observation.effective.ofType(Instant).count()"), 1)
	})
}

// TestFHIRPrimitivesAreDistinctTypes covers FHIR primitives keeping the type the
// model declared for them, rather than being folded into the system type.
//
// FHIR and FHIRPath spell their type names differently — boolean against
// Boolean — and the two are distinct: Patient.active is a FHIR.boolean, which is
// not a System.Boolean even though it converts to one for arithmetic.
func TestFHIRPrimitivesAreDistinctTypes(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","active":true,"gender":"male","birthDate":"1974-12-25"}`)

	withModel := func(t *testing.T, expr string) types.Collection {
		t.Helper()
		result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(patient, fhirpath.WithModel(typedChildrenModel{}))
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", expr, err)
		}
		return result
	}

	t.Run("the value reports its FHIR type", func(t *testing.T) {
		assertStringResult(t, withModel(t, "active.type().namespace"), "FHIR")
		assertStringResult(t, withModel(t, "active.type().name"), "boolean")
		assertStringResult(t, withModel(t, "gender.type().name"), "code")
	})

	t.Run("is() tells FHIR and System apart by case", func(t *testing.T) {
		assertBooleanResult(t, withModel(t, "active.is(boolean)"), true)
		assertBooleanResult(t, withModel(t, "active.is(FHIR.boolean)"), true)
		assertBooleanResult(t, withModel(t, "active.is(Boolean)"), false)
		assertBooleanResult(t, withModel(t, "active.is(System.Boolean)"), false)
	})

	t.Run("System names only the types FHIRPath declares", func(t *testing.T) {
		// There is no System.Patient
		assertBooleanResult(t, withModel(t, "$this.is(System.Patient)"), false)
		assertBooleanResult(t, withModel(t, "$this.is(Patient)"), true)
	})

	t.Run("a literal has no FHIR type", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "true.is(Boolean)"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "1.is(Integer)"), true)
		assertStringResult(t, evalOrFatal(t, simpleJSON, "'x'.type().namespace"), "System")
	})

	t.Run("a polymorphic field name declares the element's type", func(t *testing.T) {
		// valueOid holds an oid, which ofType(url) must not select
		observation := []byte(`{"resourceType":"Observation","valueOid":"urn:oid:1.0"}`)
		assertIntegerResult(t, evalOrFatal(t, observation, "Observation.value.ofType(oid).count()"), 1)
		assertIntegerResult(t, evalOrFatal(t, observation, "Observation.value.ofType(url).count()"), 0)
	})
}

// TestTypeFunction covers the type() reflection function against the examples in
// the FHIRPath specification, section "Types and Reflection > Reflection".
func TestTypeFunction(t *testing.T) {
	t.Run("primitives yield SimpleTypeInfo in the System namespace", func(t *testing.T) {
		// The spec's own example: ('John' | 'Mary').type() yields two
		// SimpleTypeInfo { namespace: 'System', name: 'String', baseType: 'System.Any' }
		names := evalOrFatal(t, simpleJSON, "('John' | 'Mary').type().name")
		if len(names) != 2 {
			t.Fatalf("expected one TypeInfo per element, got %v", names)
		}
		for _, n := range names {
			if n.String() != "String" {
				t.Errorf("expected String, got %s", n.String())
			}
		}

		assertStringResult(t, evalOrFatal(t, simpleJSON, "'John'.type().namespace"), "System")
		assertStringResult(t, evalOrFatal(t, simpleJSON, "'John'.type().baseType"), "System.Any")
	})

	t.Run("each system type reports its own name", func(t *testing.T) {
		cases := map[string]string{
			"'x'.type().name":                   "String",
			"1.type().name":                     "Integer",
			"(1.5).type().name":                 "Decimal",
			"true.type().name":                  "Boolean",
			"@2020-01-01.type().name":           "Date",
			"@2020-01-01T10:00:00Z.type().name": "DateTime",
			"@T10:00:00.type().name":            "Time",
			"(10 'mg').type().name":             "Quantity",
		}
		for expr, expected := range cases {
			t.Run(expr, func(t *testing.T) {
				assertStringResult(t, evalOrFatal(t, simpleJSON, expr), expected)
				assertStringResult(t, evalOrFatal(t, simpleJSON, expr[:len(expr)-len("name")]+"baseType"), "System.Any")
			})
		}
	})

	t.Run("the result is a TypeInfo subtype", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "'x'.type() is SimpleTypeInfo"), true)
		assertBooleanResult(t, evalWithModel(t, typedResource, "code.type() is ClassInfo"), true)
	})

	t.Run("complex types yield ClassInfo in the FHIR namespace", func(t *testing.T) {
		// Mirrors the spec example Patient.maritalStatus.type(), which yields
		// ClassInfo { namespace: 'FHIR', name: 'CodeableConcept', baseType: 'FHIR.Element' }
		assertStringResult(t, evalWithModel(t, typedResource, "code.type().namespace"), "FHIR")
		assertStringResult(t, evalWithModel(t, typedResource, "code.type().name"), "CodeableConcept")
		assertStringResult(t, evalWithModel(t, typedResource, "code.type().baseType"), "FHIR.Element")
	})

	t.Run("resources report their own base type", func(t *testing.T) {
		assertStringResult(t, evalWithModel(t, typedResource, "$this.type().name"), "Observation")
		assertStringResult(t, evalWithModel(t, typedResource, "$this.type().baseType"), "FHIR.DomainResource")
	})

	t.Run("identifier reports Identifier, not Quantity", func(t *testing.T) {
		assertStringResult(t, evalWithModel(t, typedResource, "identifier.first().type().name"), "Identifier")
		assertStringResult(t, evalWithModel(t, typedResource, "identifier.first().type().baseType"), "FHIR.Element")
	})

	t.Run("FHIR primitives keep their FHIR type", func(t *testing.T) {
		// A code is a distinct type from System.String per the FHIR spec
		coding := []byte(`{"system":"http://loinc.org","code":"1234-5"}`)
		result, err := fhirpath.MustCompile("code.type().name").EvaluateWithOptions(coding, fhirpath.WithModel(typedChildrenModel{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "code")

		base, err := fhirpath.MustCompile("code.type().baseType").EvaluateWithOptions(coding, fhirpath.WithModel(typedChildrenModel{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, base, "FHIR.string")
	})

	t.Run("one TypeInfo per input element", func(t *testing.T) {
		// The spec states type() yields a result per element; its ListTypeInfo
		// example contradicts that, and this engine follows the stated rule.
		assertIntegerResult(t, evalWithModel(t, typedResource, "identifier.type().count()"), 1)
		assertIntegerResult(t, evalOrFatal(t, simpleJSON, "items.type().count()"), 5)
	})

	t.Run("empty input yields empty", func(t *testing.T) {
		expr := "nosuchfield.type()"
		assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
	})

	t.Run("baseType is omitted rather than guessed without a model", func(t *testing.T) {
		// No model means no FHIR type hierarchy to report
		expr := "identifier.first().type().baseType"
		assertEmptyResult(t, evalOrFatal(t, typedResource, expr), expr)
		// ...but the name still comes from structural inference
		assertStringResult(t, evalOrFatal(t, typedResource, "identifier.first().type().name"), "Identifier")
	})

	t.Run("type() takes no arguments", func(t *testing.T) {
		if _, err := fhirpath.Evaluate(simpleJSON, "'x'.type('String')"); err == nil {
			t.Error("expected an error for type() with an argument")
		}
	})
}

// TestDelimitedTypeNames checks that a type name written with backticks names
// the same type as one written plainly.
//
// The grammar admits DELIMITEDIDENTIFIER wherever it admits IDENTIFIER, which is
// what lets a type whose name collides with a keyword be written at all. The
// backticks escape the name; they are not part of it.
func TestDelimitedTypeNames(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[{"given":["Jim"]}]}`)

	cases := []struct {
		expr string
		want string
	}{
		{"Patient.is(FHIR.`Patient`)", "true"},
		{"Patient.is(FHIR.Patient)", "true"},
		{"Patient.ofType(FHIR.`Patient`).type().name", "Patient"},
		{"Patient.ofType(FHIR.Patient).type().name", "Patient"},
		{"Patient.`name`.given.first()", "Jim"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestTypeSpecifierMustResolve covers the rule that a type name has to name a
// type: "A type specifier is an identifier that must resolve to the name of a
// type in a model."
//
// The distinction this draws is the point of it. A specifier that names no type
// is an error; a specifier that names a type the value does not have is simply
// empty. Both look like "no result" from the outside, and conflating them turns
// a typo into a silent empty collection — Patient.gender.as(strng) would quietly
// filter everything out rather than say the name is wrong.
func TestTypeSpecifierMustResolve(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","gender":"male"}`)
	withRegistry := []fhirpath.EvalOption{fhirpath.WithModel(typeRegistryModel{})}

	t.Run("a name that resolves to no type is an error", func(t *testing.T) {
		for _, expr := range []string{
			"Patient.gender.as(string1)",
			"Patient.gender.ofType(string1)",
			"Patient.gender.is(string1)",
			"Patient.as(Patint)",
		} {
			if _, err := fhirpath.MustCompile(expr).EvaluateWithOptions(patient, withRegistry...); err == nil {
				t.Errorf("%s: expected an error for a name that resolves to no type", expr)
			}
		}
	})

	t.Run("a type the value does not have is empty, not an error", func(t *testing.T) {
		// gender is a code, so it is not a uri — but uri is a type, and asking
		// is a legitimate question with a legitimate negative answer
		result, err := fhirpath.MustCompile("Patient.gender.as(uri)").EvaluateWithOptions(patient, withRegistry...)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty, got %v", result)
		}
	})

	t.Run("System types are the language's own, not the model's", func(t *testing.T) {
		// The model has no reason to declare String, and is not asked to
		for _, expr := range []string{
			"Patient.gender.is(System.String)",
			"'x'.is(String)",
			"1.is(Integer)",
		} {
			if _, err := fhirpath.MustCompile(expr).EvaluateWithOptions(patient, withRegistry...); err != nil {
				t.Errorf("%s: unexpected error: %v", expr, err)
			}
		}
	})

	t.Run("valid specifiers keep working", func(t *testing.T) {
		cases := []struct {
			expr string
			want string
		}{
			{"Patient.gender.as(code)", "male"},
			{"Patient.gender.ofType(code).exists()", "true"},
			{"Patient.is(FHIR.Patient)", "true"},
			{"Patient.is(FHIR.`Patient`)", "true"},
		}
		for _, tc := range cases {
			result, err := fhirpath.MustCompile(tc.expr).EvaluateWithOptions(patient, withRegistry...)
			if err != nil {
				t.Errorf("%s: unexpected error: %v", tc.expr, err)
				continue
			}
			got := "EMPTY"
			if len(result) > 0 {
				got = result[0].String()
			}
			if got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		}
	})
}

// TestTypeSpecifierWithoutARegistry checks that a model which cannot enumerate
// its types is not treated as a model in which no type exists.
//
// Conflating the two would make every type specifier an error against such a
// model, which is every model published today.
func TestTypeSpecifierWithoutARegistry(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","gender":"male"}`)

	for _, opts := range [][]fhirpath.EvalOption{nil, {fhirpath.WithModel(typedChildrenModel{})}} {
		for _, expr := range []string{
			"Patient.gender.as(string1)",
			"Patient.gender.as(code)",
			"Patient.is(FHIR.Patient)",
		} {
			if _, err := fhirpath.MustCompile(expr).EvaluateWithOptions(patient, opts...); err != nil {
				t.Errorf("%s: unexpected error without a type registry: %v", expr, err)
			}
		}
	}
}

// TestVersionAwareCastRule covers the `as` singleton rule, which applies from R5
// on and not before.
//
// The two readings both come from HL7 and contradict each other: the language
// specification requires an error when the input holds more than one item, while
// FHIR's own dom-3 invariant is written as %resource.descendants().as(canonical),
// which depends on filtering. The reference validator resolves it by version, and
// so does this engine.
func TestVersionAwareCastRule(t *testing.T) {
	patient := []byte(`{
		"resourceType": "Patient",
		"name": [
			{"use": "official", "family": "Chalmers"},
			{"use": "usual", "family": "Windsor"}
		]
	}`)

	t.Run("pre-R5 filters", func(t *testing.T) {
		// A model that does not declare a version is treated as pre-R5
		result, err := fhirpath.MustCompile("name.as(HumanName).count()").
			EvaluateWithOptions(patient, fhirpath.WithModel(typedChildrenModel{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertIntegerResult(t, result, 2)

		// And with no model at all
		assertIntegerResult(t, evalOrFatal(t, patient, "name.as(HumanName).count()"), 2)
	})

	t.Run("R5 requires a single item", func(t *testing.T) {
		for _, expr := range []string{
			"name.as(HumanName)",
			"name as HumanName",
		} {
			t.Run(expr, func(t *testing.T) {
				_, err := fhirpath.MustCompile(expr).EvaluateWithOptions(patient, fhirpath.WithModel(r5Model{}))
				if err == nil {
					t.Error("expected an error for a cast over more than one item")
				}
			})
		}
	})

	t.Run("R5 still casts a single item", func(t *testing.T) {
		result, err := fhirpath.MustCompile("name.first().as(HumanName).use").
			EvaluateWithOptions(patient, fhirpath.WithModel(r5Model{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "official")
	})

	t.Run("dom-3 as R4 publishes it keeps working", func(t *testing.T) {
		// The invariant every DomainResource is validated against
		const dom3 = "contained.where((('#'+id in (%resource.descendants().reference | " +
			"%resource.descendants().as(canonical) | %resource.descendants().as(uri) | " +
			"%resource.descendants().as(url))) or descendants().where(reference = '#').exists())" +
			".not()).empty()"

		orphan := []byte(`{"resourceType":"Observation","id":"o1",
			"contained":[{"resourceType":"Patient","id":"p1"}]}`)
		referenced := []byte(`{"resourceType":"Observation","id":"o1",
			"contained":[{"resourceType":"Patient","id":"p1"}],"subject":{"reference":"#p1"}}`)

		assertBooleanResult(t, evalOrFatal(t, orphan, dom3), false)
		assertBooleanResult(t, evalOrFatal(t, referenced, dom3), true)
	})
}

// fhirTypesModel is a model that also names the FHIR type of its System-typed
// elements, as the generated R5 model does.
type fhirTypesModel struct {
	*testModel
	fhirTypes map[string]string
}

func (m fhirTypesModel) FHIRTypeOf(path string) string { return m.fhirTypes[path] }

func systemTypedModel() *testModel {
	const system = "http://hl7.org/fhirpath/System.String"
	return &testModel{
		typeOf: map[string]string{
			"Patient.id":        system,
			"Patient.name":      "HumanName",
			"Patient.extension": "Extension",
			"HumanName.id":      system,
			"Extension.url":     system,
		},
		parentType: map[string]string{
			"id":     "string",
			"string": "PrimitiveType",
			"uri":    "PrimitiveType",
		},
	}
}

// A model that implements FHIRTypes has an element whose type code is a
// System type read as the FHIR type it names, unless that is string, which
// stays the System type, as the HL7 validator and fhirpath.js read it. A
// model that does not has such an element read as the System type, which is
// none of the FHIR types that convert to it.
func TestFHIRTypesTypeSystemTypedElements(t *testing.T) {
	resource := []byte(`{"resourceType":"Patient","id":"p1","name":[{"id":"n1","family":"F"}],` +
		`"extension":[{"url":"http://example.org/x","valueString":"v"}]}`)
	named := fhirTypesModel{systemTypedModel(), map[string]string{
		"Patient.id":    "id",
		"HumanName.id":  "string",
		"Extension.url": "uri",
	}}

	for _, tt := range []struct {
		expr          string
		named, system string
	}{
		{"Patient.id.is(id)", "[true]", "[false]"},
		{"Patient.id.is(string)", "[true]", "[true]"},
		{"Patient.id.is(System.String)", "[false]", "[true]"},
		{"Patient.id.type().name", "[id]", "[String]"},
		{"Patient.id = 'p1'", "[true]", "[true]"},
		{"Patient.name.id.is(System.String)", "[true]", "[true]"},
		{"Patient.name.id.is(id)", "[false]", "[false]"},
		{"Patient.extension.url.is(uri)", "[true]", "[false]"},
		{"Patient.extension.url.is(string)", "[false]", "[true]"},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			for _, m := range []struct {
				name  string
				model fhirpath.Model
				want  string
			}{{"named", named, tt.named}, {"system", systemTypedModel(), tt.system}} {
				result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions(resource, fhirpath.WithModel(m.model))
				if err != nil {
					t.Fatalf("%s: %v", m.name, err)
				}
				if got := result.String(); got != m.want {
					t.Errorf("%s: %s = %s, want %s", m.name, tt.expr, got, m.want)
				}
			}
		})
	}
}

// Test is() function
func TestIsFunctionIntegration(t *testing.T) {
	bundle := []byte(`{
		"resourceType": "Bundle",
		"type": "document",
		"entry": [
			{
				"resource": {
					"resourceType": "Composition",
					"id": "comp1",
					"status": "final"
				}
			},
			{
				"resource": {
					"resourceType": "Patient",
					"id": "pat1"
				}
			}
		]
	}`)

	tests := []struct {
		name     string
		expr     string
		wantBool bool
	}{
		{
			name:     "first entry is Composition",
			expr:     "Bundle.entry.first().resource.is(Composition)",
			wantBool: true,
		},
		{
			name:     "first entry is not Patient",
			expr:     "Bundle.entry.first().resource.is(Patient)",
			wantBool: false,
		},
		{
			name:     "second entry is Patient",
			expr:     "Bundle.entry[1].resource.is(Patient)",
			wantBool: true,
		},
		{
			name:     "resourceType is Bundle",
			expr:     "Bundle.is(Bundle)",
			wantBool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fhirpath.EvaluateToBoolean(bundle, tt.expr)
			if err != nil {
				t.Fatalf("EvaluateToBoolean(%q) error = %v", tt.expr, err)
			}

			if result != tt.wantBool {
				t.Errorf("got %v, want %v", result, tt.wantBool)
			}
		})
	}
}

// Test as() function
func TestAsFunctionIntegration(t *testing.T) {
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "pat1",
		"name": [{"family": "Doe"}]
	}`)

	t.Run("as matching type returns value", func(t *testing.T) {
		expr := fhirpath.MustCompile("Patient.as(Patient)")
		result, err := expr.Evaluate(patient)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if result.Empty() {
			t.Error("expected non-empty result")
		}
	})

	t.Run("as non-matching type returns empty", func(t *testing.T) {
		expr := fhirpath.MustCompile("Patient.as(Observation)")
		result, err := expr.Evaluate(patient)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !result.Empty() {
			t.Error("expected empty result for non-matching type")
		}
	})

	t.Run("as works on collections without error", func(t *testing.T) {
		// Test that as() works on collections (the issue from GitHub #2)
		resource := []byte(`{
			"resourceType": "Patient",
			"id": "123",
			"contained": [{
				"resourceType": "Organization",
				"id": "org1",
				"name": "Test Org"
			}],
			"managingOrganization": {
				"reference": "#org1"
			}
		}`)

		// This expression returns multiple elements via descendants()
		// as() should filter them by type without throwing SingletonExpectedError
		expr := fhirpath.MustCompile("descendants().as(string)")
		result, err := expr.Evaluate(resource)
		if err != nil {
			t.Fatalf("as() on collection should not error, got: %v", err)
		}
		// Should return string values from descendants
		if result.Empty() {
			t.Error("expected non-empty result for as(string) on descendants")
		}
	})
}

// Test as operator (not function) with collections — GitHub issue #7
func TestAsOperatorIntegration(t *testing.T) {
	resource := []byte(`{
		"resourceType": "Patient",
		"id": "123",
		"contained": [
			{"resourceType": "Practitioner", "id": "p1"},
			{"resourceType": "Organization", "id": "o1"}
		]
	}`)

	t.Run("as operator on collection filters by type", func(t *testing.T) {
		// The operator form "expr as Type" should filter collections,
		// not throw SingletonExpectedError
		expr := fhirpath.MustCompile("contained as Practitioner")
		result, err := expr.Evaluate(resource)
		if err != nil {
			t.Fatalf("as operator on collection should not error, got: %v", err)
		}
		if len(result) != 1 {
			t.Errorf("expected 1 Practitioner, got %d elements", len(result))
		}
	})

	t.Run("as operator on collection returns empty for no matches", func(t *testing.T) {
		expr := fhirpath.MustCompile("contained as Encounter")
		result, err := expr.Evaluate(resource)
		if err != nil {
			t.Fatalf("as operator should not error, got: %v", err)
		}
		if !result.Empty() {
			t.Errorf("expected empty result, got %d elements", len(result))
		}
	})

	t.Run("as operator on singleton still works", func(t *testing.T) {
		patient := []byte(`{"resourceType": "Patient", "id": "p1"}`)
		expr := fhirpath.MustCompile("Patient as Patient")
		result, err := expr.Evaluate(patient)
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if result.Empty() {
			t.Error("expected non-empty result for matching singleton")
		}
	})
}

// Test is operator (not function)
func TestIsOperatorIntegration(t *testing.T) {
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "pat1",
		"active": true,
		"birthDate": "1990-01-15"
	}`)

	tests := []struct {
		name     string
		expr     string
		wantBool bool
	}{
		{
			name:     "string is String",
			expr:     "Patient.id is String",
			wantBool: true,
		},
		{
			name:     "boolean is Boolean",
			expr:     "Patient.active is Boolean",
			wantBool: true,
		},
		{
			name:     "Patient is Patient",
			expr:     "Patient is Patient",
			wantBool: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fhirpath.EvaluateToBoolean(patient, tt.expr)
			if err != nil {
				t.Fatalf("EvaluateToBoolean(%q) error = %v", tt.expr, err)
			}

			if result != tt.wantBool {
				t.Errorf("got %v, want %v", result, tt.wantBool)
			}
		})
	}
}

// TestResourceBaseTypeIntegration tests that Resource and DomainResource
// base types work correctly in FHIRPath expressions.
// This addresses the issue where expressions like "Resource.meta.tag" were
// returning empty results even though all FHIR resources inherit from Resource.
func TestResourceBaseTypeIntegration(t *testing.T) {
	// Patient with meta.tag for testing the _tag search parameter expression
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "test",
		"meta": {
			"tag": [{
				"system": "http://example.org",
				"code": "test"
			}]
		},
		"name": [{"family": "Test"}]
	}`)

	// Bundle to test non-DomainResource behavior
	bundle := []byte(`{
		"resourceType": "Bundle",
		"id": "test-bundle",
		"type": "collection",
		"meta": {
			"tag": [{
				"system": "http://example.org",
				"code": "bundle-tag"
			}]
		}
	}`)

	t.Run("Resource.meta.tag returns tags for Patient", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patient, "Resource.meta.tag")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if result.Empty() {
			t.Error("Resource.meta.tag should return tags, got empty")
		}
		if len(result) != 1 {
			t.Errorf("expected 1 tag, got %d", len(result))
		}
	})

	t.Run("DomainResource.meta.tag returns tags for Patient", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patient, "DomainResource.meta.tag")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if result.Empty() {
			t.Error("DomainResource.meta.tag should return tags, got empty")
		}
	})

	t.Run("Resource.id returns id for Patient", func(t *testing.T) {
		result, err := fhirpath.Evaluate(patient, "Resource.id")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if result.Empty() {
			t.Error("Resource.id should return id, got empty")
		}
		if len(result) != 1 {
			t.Errorf("expected 1 result, got %d", len(result))
		}
		// Get the first item and check its string value
		if first := result[0]; first.String() != "test" {
			t.Errorf("expected 'test', got %q", first.String())
		}
	})

	t.Run("Resource.meta.tag works for Bundle", func(t *testing.T) {
		result, err := fhirpath.Evaluate(bundle, "Resource.meta.tag")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if result.Empty() {
			t.Error("Resource.meta.tag should work for Bundle, got empty")
		}
	})

	t.Run("DomainResource.meta.tag is empty for Bundle", func(t *testing.T) {
		// Bundle inherits from Resource, NOT DomainResource
		result, err := fhirpath.Evaluate(bundle, "DomainResource.meta.tag")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !result.Empty() {
			t.Error("DomainResource.meta.tag should be empty for Bundle (Bundle is not a DomainResource)")
		}
	})

	t.Run("Patient is Resource returns true", func(t *testing.T) {
		result, err := fhirpath.EvaluateToBoolean(patient, "Patient.is(Resource)")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !result {
			t.Error("Patient.is(Resource) should be true")
		}
	})

	t.Run("Patient is DomainResource returns true", func(t *testing.T) {
		result, err := fhirpath.EvaluateToBoolean(patient, "Patient.is(DomainResource)")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !result {
			t.Error("Patient.is(DomainResource) should be true")
		}
	})

	t.Run("Bundle is Resource returns true", func(t *testing.T) {
		result, err := fhirpath.EvaluateToBoolean(bundle, "Bundle.is(Resource)")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if !result {
			t.Error("Bundle.is(Resource) should be true")
		}
	})

	t.Run("Bundle is DomainResource returns false", func(t *testing.T) {
		result, err := fhirpath.EvaluateToBoolean(bundle, "Bundle.is(DomainResource)")
		if err != nil {
			t.Fatalf("error = %v", err)
		}
		if result {
			t.Error("Bundle.is(DomainResource) should be false (Bundle inherits directly from Resource)")
		}
	})
}

// TestOfTypePolymorphic verifies that .ofType() correctly resolves polymorphic
// FHIR fields (value[x], effective[x], identified[x]) for temporal and complex types.
func TestOfTypePolymorphic(t *testing.T) {
	tests := []struct {
		name      string
		resource  string
		expr      string
		wantCount int
		wantFirst string
		wantType  string
	}{
		// A polymorphic field name states the element's FHIR type, and FHIR
		// writes primitive type names in lower camel case: valueDateTime holds a
		// dateTime, which is a distinct type from System.DateTime.
		{
			name:      "ofType(DateTime) on valueDateTime",
			resource:  `{"resourceType":"Observation","valueDateTime":"2023-02-08"}`,
			expr:      "Observation.value.ofType(DateTime)",
			wantCount: 1,
			wantFirst: "2023-02-08",
			wantType:  "dateTime",
		},
		{
			name:      "ofType(Date) on valueDate",
			resource:  `{"resourceType":"Observation","valueDate":"2023-06-15"}`,
			expr:      "Observation.value.ofType(Date)",
			wantCount: 1,
			wantFirst: "2023-06-15",
			wantType:  "date",
		},
		{
			name:      "ofType(Time) on valueTime",
			resource:  `{"resourceType":"Observation","valueTime":"18:12:00"}`,
			expr:      "Observation.value.ofType(Time)",
			wantCount: 1,
			wantFirst: "18:12:00",
			wantType:  "time",
		},
		{
			name:      "ofType(Instant) on effectiveInstant",
			resource:  `{"resourceType":"Observation","effectiveInstant":"2015-02-07T13:28:17.239+02:00"}`,
			expr:      "Observation.effective.ofType(Instant)",
			wantCount: 1,
			wantFirst: "2015-02-07T13:28:17.239+02:00",
			wantType:  "instant",
		},
		{
			name:      "ofType(String) on valueString still works",
			resource:  `{"resourceType":"Observation","valueString":"hello"}`,
			expr:      "Observation.value.ofType(String)",
			wantCount: 1,
			wantFirst: "hello",
			wantType:  "string",
		},
		{
			name:      "ofType(Quantity) on valueQuantity",
			resource:  `{"resourceType":"Observation","valueQuantity":{"value":1.0,"unit":"mg"}}`,
			expr:      "Observation.value.ofType(Quantity)",
			wantCount: 1,
			wantType:  "Quantity",
		},
		{
			name:      "ofType(Quantity).value on valueQuantity",
			resource:  `{"resourceType":"Observation","valueQuantity":{"value":1.0,"unit":"mg"}}`,
			expr:      "Observation.value.ofType(Quantity).value",
			wantCount: 1,
			wantFirst: "1.0",
		},
		{
			name:      "ofType(Date) returns empty when valueString present",
			resource:  `{"resourceType":"Observation","valueString":"hello"}`,
			expr:      "Observation.value.ofType(Date)",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(tt.resource), tt.expr)
			if err != nil {
				t.Fatalf("error: %v", err)
			}

			if len(result) != tt.wantCount {
				t.Fatalf("expected %d results, got %d: %v", tt.wantCount, len(result), result)
			}

			if tt.wantCount > 0 && tt.wantFirst != "" {
				if result[0].String() != tt.wantFirst {
					t.Errorf("expected first=%q, got %q", tt.wantFirst, result[0].String())
				}
			}

			if tt.wantCount > 0 && tt.wantType != "" {
				if result[0].Type() != tt.wantType {
					t.Errorf("expected type=%q, got %q", tt.wantType, result[0].Type())
				}
			}
		})
	}
}

// TestOfTypeURISubtypes verifies that ofType() discriminates between URI subtypes
// (url, oid, uuid, id) instead of treating them all as String.
func TestOfTypeURISubtypes(t *testing.T) {
	tests := []struct {
		name      string
		resource  string
		expr      string
		wantCount int
	}{
		{
			name:      "ofType(url) returns empty when valueOid present",
			resource:  `{"resourceType":"Task","output":[{"valueOid":"urn:oid:1.0"}]}`,
			expr:      "Task.output.first().value.ofType(url)",
			wantCount: 0,
		},
		{
			name:      "ofType(oid) returns value when valueOid present",
			resource:  `{"resourceType":"Task","output":[{"valueOid":"urn:oid:1.0"}]}`,
			expr:      "Task.output.first().value.ofType(oid)",
			wantCount: 1,
		},
		{
			name:      "ofType(uuid) returns empty when valueOid present",
			resource:  `{"resourceType":"Task","output":[{"valueOid":"urn:oid:1.0"}]}`,
			expr:      "Task.output.first().value.ofType(uuid)",
			wantCount: 0,
		},
		{
			name:      "ofType(id) returns empty when valueOid present",
			resource:  `{"resourceType":"Task","output":[{"valueOid":"urn:oid:1.0"}]}`,
			expr:      "Task.output.first().value.ofType(id)",
			wantCount: 0,
		},
		{
			name:      "ofType(url) returns value when valueUrl present",
			resource:  `{"resourceType":"Task","output":[{"valueUrl":"http://example.com"}]}`,
			expr:      "Task.output.first().value.ofType(url)",
			wantCount: 1,
		},
		{
			name:      "ofType(uuid) returns value when valueUuid present",
			resource:  `{"resourceType":"Task","output":[{"valueUuid":"urn:uuid:abc-123"}]}`,
			expr:      "Task.output.first().value.ofType(uuid)",
			wantCount: 1,
		},
		{
			name:      "ofType(id) returns value when valueId present",
			resource:  `{"resourceType":"Task","output":[{"valueId":"my-id"}]}`,
			expr:      "Task.output.first().value.ofType(id)",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(tt.resource), tt.expr)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if len(result) != tt.wantCount {
				t.Errorf("expected %d results, got %d: %v", tt.wantCount, len(result), result)
			}
		})
	}
}

// TestOfTypeComplexTypes verifies that ofType() resolves complex types like Quantity
// even when the JSON object doesn't have enough fields for heuristic detection.
func TestOfTypeComplexTypes(t *testing.T) {
	tests := []struct {
		name      string
		resource  string
		expr      string
		wantCount int
		wantFirst string
	}{
		{
			name:      "ofType(Quantity) on minimal valueQuantity (no unit)",
			resource:  `{"resourceType":"Observation","valueQuantity":{"value":1.0}}`,
			expr:      "Observation.value.ofType(Quantity)",
			wantCount: 1,
		},
		{
			name:      "ofType(Quantity).value on minimal valueQuantity",
			resource:  `{"resourceType":"Observation","valueQuantity":{"value":1.0}}`,
			expr:      "Observation.value.ofType(Quantity).value",
			wantCount: 1,
			wantFirst: "1.0",
		},
		{
			name:      "ofType(CodeableConcept) on valueCodeableConcept",
			resource:  `{"resourceType":"Observation","valueCodeableConcept":{"text":"Normal"}}`,
			expr:      "Observation.value.ofType(CodeableConcept)",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := fhirpath.Evaluate([]byte(tt.resource), tt.expr)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			if len(result) != tt.wantCount {
				t.Fatalf("expected %d results, got %d: %v", tt.wantCount, len(result), result)
			}
			if tt.wantFirst != "" && len(result) > 0 {
				if result[0].String() != tt.wantFirst {
					t.Errorf("expected %q, got %q", tt.wantFirst, result[0].String())
				}
			}
		})
	}
}
