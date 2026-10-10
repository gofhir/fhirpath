package fhirpath

import (
	"context"
	"fmt"
	"testing"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// A reference is resolved from where it is written, not from the root of the
// expression. bundle.html#references: a relative reference takes the base of
// the RESTful fullUrl of the entry that holds it; a versioned one also matches
// meta.versionId; several matches are ambiguous, and the first is taken, as
// it always was. references.html#contained: a
// fragment names a resource contained in the resource that makes the
// reference, and one written inside a contained resource follows the rules of
// the resource that contains it.

func entriesBundle(entries string) []byte {
	return []byte(`{"resourceType":"Bundle","type":"collection","entry":[` + entries + `]}`)
}

func observationEntry(fullURL, id, status, extra string) string {
	if fullURL != "" {
		fullURL = `"fullUrl":"` + fullURL + `",`
	}
	return fmt.Sprintf(`{%s"resource":{"resourceType":"Observation","id":%q,"status":%q,"code":{"text":"c"}%s}}`,
		fullURL, id, status, extra)
}

func TestResolveFromTheReferringEntry(t *testing.T) {
	member := `,"hasMember":[{"reference":"%s"}]`
	resolved := "Bundle.entry.resource.where(id = 'root').hasMember.resolve()"

	for _, tc := range []struct {
		name    string
		entries []string
		want    string
	}{
		{"the base of the referring entry, not the first type and id", []string{
			observationEntry("http://a.org/fhir/Observation/x", "x", "preliminary", ""),
			observationEntry("http://b.org/fhir/Observation/root", "root", "final", fmt.Sprintf(member, "Observation/x")),
			observationEntry("http://b.org/fhir/Observation/x", "x", "final", ""),
		}, "final"},
		{"the same, the other way round", []string{
			observationEntry("http://b.org/fhir/Observation/x", "x", "final", ""),
			observationEntry("http://a.org/fhir/Observation/root", "root", "final", fmt.Sprintf(member, "Observation/x")),
			observationEntry("http://a.org/fhir/Observation/x", "x", "preliminary", ""),
		}, "preliminary"},
		{"a RESTful base with no match resolves to nothing in the Bundle", []string{
			observationEntry("http://a.org/fhir/Observation/x", "x", "final", ""),
			observationEntry("http://b.org/fhir/Observation/root", "root", "final", fmt.Sprintf(member, "Observation/x")),
		}, "EMPTY"},
		{"an absolute reference by fullUrl", []string{
			observationEntry("http://a.org/fhir/Observation/x", "x", "preliminary", ""),
			observationEntry("http://b.org/fhir/Observation/root", "root", "final", fmt.Sprintf(member, "http://a.org/fhir/Observation/x")),
		}, "preliminary"},
		{"a urn:uuid reference by fullUrl", []string{
			observationEntry("urn:uuid:7c5e7a3e-0000-4000-8000-000000000001", "x", "amended", ""),
			observationEntry("urn:uuid:7c5e7a3e-0000-4000-8000-000000000002", "root", "final",
				fmt.Sprintf(member, "urn:uuid:7c5e7a3e-0000-4000-8000-000000000001")),
		}, "amended"},
		{"a relative reference from a non-RESTful fullUrl, by the ids in the Bundle", []string{
			observationEntry("urn:uuid:7c5e7a3e-0000-4000-8000-000000000001", "x", "amended", ""),
			observationEntry("urn:uuid:7c5e7a3e-0000-4000-8000-000000000002", "root", "final", fmt.Sprintf(member, "Observation/x")),
		}, "amended"},
		{"a relative reference with no fullUrl, by the ids in the Bundle", []string{
			observationEntry("", "x", "amended", ""),
			observationEntry("", "root", "final", fmt.Sprintf(member, "Observation/x")),
		}, "amended"},
		{"of several matches by id, the first", []string{
			observationEntry("", "x", "amended", ""),
			observationEntry("", "x", "final", ""),
			observationEntry("", "root", "final", fmt.Sprintf(member, "Observation/x")),
		}, "amended"},
		{"an entry whose fullUrl is the reference as written", []string{
			`{"fullUrl":"Observation/x","resource":{"resourceType":"Observation","status":"amended","code":{"text":"c"}}}`,
			observationEntry("", "root", "final", fmt.Sprintf(member, "Observation/x")),
		}, "amended"},
		{"a versioned fullUrl that is the reference as written", []string{
			`{"fullUrl":"http://a.org/fhir/Observation/x/_history/2","resource":{"resourceType":"Observation","id":"x","status":"amended","code":{"text":"c"}}}`,
			observationEntry("http://a.org/fhir/Observation/root", "root", "final", fmt.Sprintf(member, "http://a.org/fhir/Observation/x/_history/2")),
		}, "amended"},
		{"a fullUrl with a trailing slash", []string{
			observationEntry("http://a.org/fhir/Observation/x/", "x", "amended", ""),
			observationEntry("http://a.org/fhir/Observation/root", "root", "final", fmt.Sprintf(member, "Observation/x")),
		}, "amended"},
		{"a versioned reference matches meta.versionId", []string{
			`{"fullUrl":"http://a.org/fhir/Observation/x","resource":{"resourceType":"Observation","id":"x","meta":{"versionId":"1"},"status":"preliminary","code":{"text":"c"}}}`,
			`{"fullUrl":"http://a.org/fhir/Observation/x","resource":{"resourceType":"Observation","id":"x","meta":{"versionId":"2"},"status":"final","code":{"text":"c"}}}`,
			observationEntry("http://a.org/fhir/Observation/root", "root", "final", fmt.Sprintf(member, "Observation/x/_history/2")),
		}, "final"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := tc.entries[0]
			for _, e := range tc.entries[1:] {
				entries += "," + e
			}
			got := evaluateScalar(t, resolved+".status", entriesBundle(entries))
			if got != tc.want {
				t.Errorf("%s.status = %s, want %s", resolved, got, tc.want)
			}
		})
	}

	// The case of #123, written as an invariant over the Bundle
	bundle := entriesBundle(observationEntry("http://a.org/fhir/Observation/x", "x", "preliminary", "") + "," +
		observationEntry("http://b.org/fhir/Observation/root", "root", "final", fmt.Sprintf(member, "Observation/x")) + "," +
		observationEntry("http://b.org/fhir/Observation/x", "x", "final", ""))
	expr := "entry.resource.ofType(Observation).hasMember.all(resolve().status = 'final')"
	if got := evaluateScalar(t, expr, bundle); got != "true" {
		t.Errorf("%s = %s, want true", expr, got)
	}
}

type mapResolver map[string]string

func (r mapResolver) Resolve(_ context.Context, reference string) ([]byte, error) {
	if resource, ok := r[reference]; ok {
		return []byte(resource), nil
	}
	return nil, fmt.Errorf("no %s", reference)
}

func TestResolveFragmentInTheReferringResource(t *testing.T) {
	observation := []byte(`{"resourceType":"Observation","id":"o","status":"final","code":{"text":"c"},
	  "subject":{"reference":"Patient/p"},
	  "contained":[{"resourceType":"Practitioner","id":"pr","name":[{"family":"Wrong"}]}]}`)
	resolver := mapResolver{"Patient/p": `{"resourceType":"Patient","id":"p",
	  "contained":[{"resourceType":"Practitioner","id":"pr","name":[{"family":"Right"}]},
	               {"resourceType":"Organization","id":"org","contact":[{"name":{"text":"back"}}],"partOf":{"reference":"#"}}],
	  "generalPractitioner":[{"reference":"#pr"}],
	  "managingOrganization":{"reference":"#org"}}`}

	for expr, want := range map[string]string{
		// #pr names the Patient's contained Practitioner, not the Observation's
		"subject.resolve().generalPractitioner.resolve().name.family": "Right",
		"subject.resolve().generalPractitioner.resolve().exists()":    "true",
		// # from a contained resource names its container
		"subject.resolve().managingOrganization.resolve().partOf.resolve().id": "p",
		// The Observation's own fragment still names its own contained resource
		"contained.where(id = 'pr').name.family": "Wrong",
	} {
		got, err := MustCompile(expr).EvaluateWithOptions(observation, WithResolver(resolver))
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if g := got.String(); g != "["+want+"]" {
			t.Errorf("%s = %s, want [%s]", expr, g, want)
		}
	}
}

// TestResolveInTheNearestBundle checks that a reference inside a Bundle held
// by another — a document in a collection — is resolved in the Bundle it is
// written in, from its own entry.
func TestResolveInTheNearestBundle(t *testing.T) {
	outer := []byte(`{"resourceType":"Bundle","type":"collection","entry":[
	  {"fullUrl":"http://outer.org/fhir/Bundle/inner","resource":{"resourceType":"Bundle","type":"document","entry":[
	    {"fullUrl":"http://inner.org/fhir/Composition/c","resource":{"resourceType":"Composition","id":"c","subject":{"reference":"Patient/p"}}},
	    {"fullUrl":"http://inner.org/fhir/Patient/p","resource":{"resourceType":"Patient","id":"p","gender":"female"}}]}},
	  {"fullUrl":"http://outer.org/fhir/Patient/p","resource":{"resourceType":"Patient","id":"p","gender":"male"}}]}`)

	expr := "Bundle.entry.resource.ofType(Bundle).entry.resource.ofType(Composition).subject.resolve().gender"
	if got := evaluateScalar(t, expr, outer); got != "female" {
		t.Errorf("%s = %s, want female", expr, got)
	}
}

// TestResolveContainerOnlyFromContained checks that "#" names the container
// only from one of its contained resources: from a resource that is not
// contained it names nothing.
func TestResolveContainerOnlyFromContained(t *testing.T) {
	observation := []byte(`{"resourceType":"Observation","id":"o","status":"final","code":{"text":"c"},
	  "subject":{"reference":"#"}}`)
	for _, expr := range []string{"subject.resolve().exists()", "subject.reference.resolve().exists()"} {
		if got := evaluateScalar(t, expr, observation); got != "false" {
			t.Errorf("%s = %s, want false", expr, got)
		}
	}
}

// TestResolveFromSharedReferences resolves references read by one evaluation
// from several goroutines at once: the walk up to the resource that makes them
// reads it without writing to it. Run with -race.
func TestResolveFromSharedReferences(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","id":"p",
	  "contained":[{"resourceType":"Practitioner","id":"pr","name":[{"family":"Right"}]}],
	  "generalPractitioner":[{"reference":"#pr"},{"reference":"#pr"},{"reference":"#pr"},{"reference":"#pr"}]}`)
	refs, err := MustCompile("Patient.generalPractitioner").Evaluate(patient)
	if err != nil {
		t.Fatal(err)
	}
	expr := MustCompile("resolve().name.family")

	done := make(chan string, len(refs))
	for _, ref := range refs {
		go func() {
			got, err := expr.EvaluateWithContext(eval.NewContextForRoot(types.Collection{ref}))
			if err != nil {
				done <- err.Error()
				return
			}
			done <- got.String()
		}()
	}
	for range refs {
		if got := <-done; got != "[Right]" {
			t.Errorf("resolve().name.family = %s, want [Right]", got)
		}
	}
}

// TestResolveIdsThatLookLikeDates checks that an id or a version is the string
// the JSON writes, even where it looks like a date: 2024 is an id, not a year.
func TestResolveIdsThatLookLikeDates(t *testing.T) {
	member := `,"hasMember":[{"reference":"%s"}]`
	resolved := "Bundle.entry.resource.where(id = 'root').hasMember.resolve().status"
	for _, tc := range []struct{ name, entries string }{
		{"an id of four digits", observationEntry("", "2024", "amended", "") + "," +
			observationEntry("", "root", "final", fmt.Sprintf(member, "Observation/2024"))},
		{"a versionId of four digits", `{"fullUrl":"http://a.org/fhir/Observation/x","resource":{"resourceType":"Observation","id":"x","meta":{"versionId":"1234"},"status":"amended","code":{"text":"c"}}},` +
			observationEntry("http://a.org/fhir/Observation/root", "root", "final", fmt.Sprintf(member, "Observation/x/_history/1234"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluateScalar(t, resolved, entriesBundle(tc.entries)); got != "amended" {
				t.Errorf("%s = %s, want amended", resolved, got)
			}
		})
	}

	patient := []byte(`{"resourceType":"Patient","id":"p","contained":[{"resourceType":"Practitioner","id":"2024"}],
	  "generalPractitioner":[{"reference":"#2024"}]}`)
	if got := evaluateScalar(t, "Patient.generalPractitioner.resolve().id", patient); got != "2024" {
		t.Errorf("#2024 resolved to %s, want 2024", got)
	}
}

// TestResolveTransactionByTypeAndID checks that a relative reference from an
// entry with a RESTful fullUrl still finds, by type and id, a resource whose
// entry names no server — a urn:uuid fullUrl, or none — as a transaction
// writes the resources it creates. An entry of another server is not one.
func TestResolveTransactionByTypeAndID(t *testing.T) {
	bundle := []byte(`{"resourceType":"Bundle","type":"transaction","entry":[
	  {"fullUrl":"urn:uuid:aaa","resource":{"resourceType":"Patient","id":"p1","gender":"male"}},
	  {"resource":{"resourceType":"Organization","id":"o1","name":"org"}},
	  {"fullUrl":"http://other.org/fhir/Patient/p2","resource":{"resourceType":"Patient","id":"p2","gender":"other"}},
	  {"fullUrl":"http://a.org/fhir/Observation/r","resource":{"resourceType":"Observation","id":"r","status":"final","code":{"text":"c"},
	    "subject":{"reference":"Patient/p1"},"performer":[{"reference":"Organization/o1"},{"reference":"Patient/p2"}]}}]}`)

	for expr, want := range map[string]string{
		"Bundle.entry.resource.ofType(Observation).subject.resolve().gender":            "male",
		"Bundle.entry.resource.ofType(Observation).performer.first().resolve().name":    "org",
		"Bundle.entry.resource.ofType(Observation).performer.last().resolve().exists()": "false",
	} {
		if got := evaluateScalar(t, expr, bundle); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// TestResolveFallsBackToTheRootBundle checks that a reference in a nested
// Bundle that names nothing there is looked for in the Bundle being evaluated,
// as it always was.
func TestResolveFallsBackToTheRootBundle(t *testing.T) {
	outer := []byte(`{"resourceType":"Bundle","type":"collection","entry":[
	  {"fullUrl":"http://outer.org/fhir/Patient/p","resource":{"resourceType":"Patient","id":"p","gender":"male"}},
	  {"resource":{"resourceType":"Bundle","type":"document","entry":[
	    {"resource":{"resourceType":"Composition","id":"c","subject":{"reference":"http://outer.org/fhir/Patient/p"}}}]}}]}`)

	expr := "Bundle.entry.resource.ofType(Bundle).entry.resource.ofType(Composition).subject.resolve().gender"
	if got := evaluateScalar(t, expr, outer); got != "male" {
		t.Errorf("%s = %s, want male", expr, got)
	}
}

// TestResolveFromSharedObjects resolves, from several goroutines, references
// whose walk returns the container itself ("#") or starts from an object
// Parent returned. Run with -race.
func TestResolveFromSharedObjects(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","id":"p","name":[{"family":"Self"}],
	  "contained":[{"resourceType":"Organization","id":"org","partOf":{"reference":"#"}}]}`)
	refs, err := MustCompile("Patient.contained.partOf").Evaluate(patient)
	if err != nil || len(refs) != 1 {
		t.Fatal(refs, err)
	}

	bundle := chainBundle(4, true)
	resources, err := MustCompile("Bundle.entry.resource").Evaluate(bundle)
	if err != nil || len(resources) != 4 {
		t.Fatal(resources, err)
	}
	entry := resources[0].(*types.ObjectValue).Parent()

	cases := []struct {
		root types.Value
		expr string
		want string
	}{
		{refs[0], "resolve().name.family", "[Self]"},
		{entry, "resource.hasMember.resolve().id", "[o1]"},
	}
	for _, tc := range cases {
		expr := MustCompile(tc.expr)
		done := make(chan string, 8)
		for range 8 {
			go func() {
				got, err := expr.EvaluateWithContext(eval.NewContextForRoot(types.Collection{tc.root}))
				if err != nil {
					done <- err.Error()
					return
				}
				done <- got.String()
			}()
		}
		for range 8 {
			if got := <-done; got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		}
	}
}
