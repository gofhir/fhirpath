package eval

import "testing"

// fhirHierarchyModel carries the primitive hierarchy FHIR publishes in its
// StructureDefinitions: uuid, oid, url and canonical specialize uri; code, id
// and markdown specialize string; the sized integers specialize integer.
func fhirHierarchyModel() *mockModel {
	return &mockModel{
		parentType: map[string]string{
			"uuid":            "uri",
			"oid":             "uri",
			"url":             "uri",
			"canonical":       "uri",
			"code":            "string",
			"id":              "string",
			"markdown":        "string",
			"positiveInt":     "integer",
			"unsignedInt":     "integer",
			"Patient":         "DomainResource",
			"DomainResource":  "Resource",
			"Age":             "Quantity",
			"BackboneElement": "Element",
		},
	}
}

// TestNamespaceDoesNotChangeTheType covers the invariant the FHIR namespace has
// to satisfy: it says which namespace a name is read in, and never names a
// different type.
//
// So FHIR.uri and uri are one type, and every question asked of one must get the
// answer given for the other. Before this held, is(uri) on a uuid was true while
// is(FHIR.uri) was false — and it is the qualified form the specification's own
// examples write, as in Patient.name.given.is(FHIR.string).
//
// The names here are FHIR's alone. Those it shares a spelling with System —
// string, integer and the rest — are a separate matter, covered below.
func TestNamespaceDoesNotChangeTheType(t *testing.T) {
	model := fhirHierarchyModel()

	cases := []struct {
		actual string
		bare   string
	}{
		// A primitive and the primitive it specializes
		{"uuid", "uri"},
		{"oid", "uri"},
		{"url", "uri"},
		{"canonical", "uri"},

		// The same type, named twice
		{"uuid", "uuid"},
		{"Patient", "Patient"},

		// Complex types and resources
		{"Patient", "DomainResource"},
		{"Patient", "Resource"},
		{"Age", "Quantity"},

		// And pairs that match neither way
		{"uuid", "oid"},
		{"Patient", "Observation"},
	}

	for _, tc := range cases {
		qualified := "FHIR." + tc.bare

		t.Run(tc.actual+"/"+tc.bare, func(t *testing.T) {
			withModel := TypeMatchesWithModel(tc.actual, tc.bare, model)
			if got := TypeMatchesWithModel(tc.actual, qualified, model); got != withModel {
				t.Errorf("with a model: is(%s) is %v but is(%s) is %v",
					tc.bare, withModel, qualified, got)
			}

			heuristic := TypeMatches(tc.actual, tc.bare)
			if got := TypeMatches(tc.actual, qualified); got != heuristic {
				t.Errorf("without a model: is(%s) is %v but is(%s) is %v",
					tc.bare, heuristic, qualified, got)
			}
		})
	}
}

// TestWritingTheNamespaceIsWhatDisambiguates covers where the qualified and bare
// forms legitimately part company, which is the one place the invariant above
// does not reach.
//
// FHIR's string and System's String differ only in case, so a bare `string` is
// two questions at once: does this derive from FHIR.string, and does it convert
// to System.String. Several primitives convert to System.String without deriving
// from FHIR.string — a uuid derives from uri — so the two answers differ.
//
// Bare, the engine gives the conversion answer, which CONFORMANCE.md records as
// a deliberate guess: "a System.String might be a code, a uri or an id.
// Answering yes is the useful guess while the engine does not carry the declared
// type on every primitive value." Qualified, there is nothing to guess: FHIR.string
// asks about derivation alone, and only a model can answer it.
func TestWritingTheNamespaceIsWhatDisambiguates(t *testing.T) {
	model := fhirHierarchyModel()

	t.Run("bare answers the conversion, and needs no model", func(t *testing.T) {
		for _, actual := range []string{"code", "id", "markdown", "uuid", "uri", "oid"} {
			if !TypeMatches(actual, "string") {
				t.Errorf("%s converts to System.String, so is(string) should be true", actual)
			}
		}
	})

	t.Run("qualified answers the derivation, and takes a model", func(t *testing.T) {
		// Derived from FHIR.string
		for _, actual := range []string{"code", "id", "markdown"} {
			if !TypeMatchesWithModel(actual, "FHIR.string", model) {
				t.Errorf("%s derives from string, so is(FHIR.string) should be true", actual)
			}
		}
		// Converts to System.String, but derives from uri
		for _, actual := range []string{"uuid", "oid", "url", "canonical"} {
			if TypeMatchesWithModel(actual, "FHIR.string", model) {
				t.Errorf("%s derives from uri, not string, so is(FHIR.string) should be false", actual)
			}
			if !TypeMatchesWithModel(actual, "FHIR.uri", model) {
				t.Errorf("%s derives from uri, so is(FHIR.uri) should be true", actual)
			}
		}
	})
}

// TestModelResolvesThePrimitiveHierarchy covers what the invariant above is
// worth: the answers themselves, which only a model can give.
//
// FHIR states that "all primitives are considered to be independent types (so
// markdown is not a subclass of string)", and says it in the section on
// ofType(), which is restricted to concrete types. The derivation is real
// nonetheless — uuid names uri as its base definition — and both the official
// suite and fhirpath.js 5.1.0 expect is() to follow it: the suite's testTypeA4
// asserts that a valueUuid is(FHIR.uri).
func TestModelResolvesThePrimitiveHierarchy(t *testing.T) {
	model := fhirHierarchyModel()

	for _, tc := range []struct {
		actual, typeName string
		want             bool
	}{
		{"uuid", "FHIR.uri", true},
		{"uuid", "uri", true},
		{"markdown", "FHIR.string", true},
		{"positiveInt", "FHIR.integer", true},

		// Derivation runs one way only
		{"uri", "FHIR.uuid", false},
		{"string", "FHIR.markdown", false},

		// And not between unrelated primitives. A uuid converts to System.String
		// without deriving from FHIR.string — see the test above
		{"uuid", "FHIR.string", false},
		{"boolean", "FHIR.integer", false},
	} {
		t.Run(tc.actual+" is "+tc.typeName, func(t *testing.T) {
			if got := TypeMatchesWithModel(tc.actual, tc.typeName, model); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSystemNamespaceStaysDistinct covers the namespace that does change the
// type, so that unqualifying FHIR does not unqualify this one too.
//
// "Patient.active is a FHIR.boolean, so is(Boolean) is false while is(boolean)
// is true." The suite requires both.
func TestSystemNamespaceStaysDistinct(t *testing.T) {
	model := fhirHierarchyModel()

	for _, tc := range []struct {
		actual, typeName string
		want             bool
	}{
		{"boolean", "boolean", true},
		{"boolean", "FHIR.boolean", true},
		{"boolean", "Boolean", false},
		{"boolean", "System.Boolean", false},
		{"code", "String", false},
	} {
		t.Run(tc.actual+" is "+tc.typeName, func(t *testing.T) {
			if got := TypeMatchesWithModel(tc.actual, tc.typeName, model); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
