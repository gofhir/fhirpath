package fhirpath

import (
	"context"
	"fmt"
	"testing"
)

// A reference is resolved from where it is written, not from the root of the
// expression. bundle.html#references: a relative reference takes the base of
// the RESTful fullUrl of the entry that holds it; a versioned one also matches
// meta.versionId; several matches are ambiguous. references.html#contained: a
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
		{"several matches by id are ambiguous", []string{
			observationEntry("", "x", "amended", ""),
			observationEntry("", "x", "final", ""),
			observationEntry("", "root", "final", fmt.Sprintf(member, "Observation/x")),
		}, "EMPTY"},
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
