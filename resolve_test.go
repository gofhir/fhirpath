package fhirpath

import "testing"

// TestResolveWithinDocument covers references that point inward, which need no
// resolver because the target is already in the document.
//
// FHIR writes two kinds. A fragment names a resource in the containing
// resource's contained list; a relative reference names an entry of a Bundle.
// Returning empty for either would be wrong rather than merely limited — the
// data is right there, and an invariant that walks contained resources would
// silently pass on documents it should reject.
func TestResolveWithinDocument(t *testing.T) {
	report := []byte(`{
	  "resourceType":"DiagnosticReport",
	  "id":"eric",
	  "contained":[
	    {"resourceType":"Composition","id":"comp","section":[{"entry":[{"reference":"#obs1"}]}]},
	    {"resourceType":"Observation","id":"obs1","status":"final"}
	  ],
	  "status":"final",
	  "result":[{"reference":"#obs1"}],
	  "composition":{"reference":"#comp"}
	}`)

	cases := []struct {
		expr string
		want string
	}{
		{"composition.resolve().type().name", "Composition"},
		{"result.reference.resolve().type().name", "Observation"},
		{"result.resolve().id", "obs1"},

		// resolve() reaches through a chain
		{"composition.resolve().section.entry.reference.resolve().type().name", "Observation"},
		{"composition.resolve().section.entry.reference.where(resolve() is Observation).count()", "1"},

		// A reference to nothing resolves to nothing
		{"composition.resolve().section.entry.reference.where($this = '#absent').resolve().empty()", "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, report); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestResolveWithinBundle covers the other inward reference: an entry of a
// Bundle, matched on its fullUrl or on the type and id its resource carries.
func TestResolveWithinBundle(t *testing.T) {
	bundle := []byte(`{
	  "resourceType":"Bundle",
	  "type":"collection",
	  "entry":[
	    {"fullUrl":"http://example.org/fhir/Patient/p1",
	     "resource":{"resourceType":"Patient","id":"p1","gender":"male"}},
	    {"fullUrl":"http://example.org/fhir/Observation/o1",
	     "resource":{"resourceType":"Observation","id":"o1","status":"final",
	                 "subject":{"reference":"Patient/p1"}}}
	  ]
	}`)

	cases := []struct {
		expr string
		want string
	}{
		// By the type and id of the resource
		{"Bundle.entry.resource.ofType(Observation).subject.resolve().type().name", "Patient"},
		{"Bundle.entry.resource.ofType(Observation).subject.resolve().gender", "male"},

		// By fullUrl
		{"'http://example.org/fhir/Patient/p1'.resolve().id", "p1"},

		// An entry that is not there
		{"'Patient/absent'.resolve().empty()", "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, bundle); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestResolveWithoutAResolver checks that a reference the engine cannot reach is
// still empty rather than an error, which is what leaves an external resolver
// worth supplying.
func TestResolveWithoutAResolver(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","id":"p1","managingOrganization":{"reference":"http://example.org/fhir/Organization/o1"}}`)

	if got := evaluateScalar(t, "managingOrganization.resolve().empty()", patient); got != "true" {
		t.Errorf("an unreachable reference should resolve to nothing, got %s", got)
	}
}
