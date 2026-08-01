package fhirpath

import "testing"

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
		result, err := MustCompile("code.type().name").EvaluateWithOptions(coding, WithModel(testModel{}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertStringResult(t, result, "code")

		base, err := MustCompile("code.type().baseType").EvaluateWithOptions(coding, WithModel(testModel{}))
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
		if _, err := Evaluate(simpleJSON, "'x'.type('String')"); err == nil {
			t.Error("expected an error for type() with an argument")
		}
	})
}
