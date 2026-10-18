package fhirpath_test

import (
	"fmt"
	"strings"

	"github.com/gofhir/fhirpath"
)

// chainBundle writes n Observations each of which names the next as a member,
// with RESTful fullUrls or urn:uuid ones, as resolve() meets them over a
// Bundle.
func chainBundle(n int, restful bool) []byte {
	entries := make([]string, n)
	for i := range entries {
		fullURL, ref := fmt.Sprintf("http://example.org/fhir/Observation/o%d", i), fmt.Sprintf("Observation/o%d", i+1)
		if !restful {
			fullURL, ref = fmt.Sprintf("urn:uuid:00000000-0000-4000-8000-%012d", i), fmt.Sprintf("urn:uuid:00000000-0000-4000-8000-%012d", i+1)
		}
		entries[i] = fmt.Sprintf(`{"fullUrl":%q,"resource":{"resourceType":"Observation","id":"o%d","status":"final","code":{"text":"c"},"hasMember":[{"reference":%q}]}}`,
			fullURL, i, ref)
	}
	return []byte(`{"resourceType":"Bundle","type":"collection","entry":[` + strings.Join(entries, ",") + `]}`)
}

// makeBundle builds a Bundle with n Patient entries.
func makeBundle(n int) []byte {
	var b strings.Builder
	b.WriteString(`{"resourceType":"Bundle","type":"searchset","entry":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"fullUrl":"urn:uuid:%d","resource":{"resourceType":"Patient","id":"p%d","active":true,`+
			`"name":[{"use":"official","family":"Fam%d","given":["Given%d","Middle%d"]}],`+
			`"telecom":[{"system":"phone","value":"555-%04d"}],"gender":"male","birthDate":"1974-12-25"}}`,
			i, i, i, i, i, i)
	}
	b.WriteString(`]}`)
	return []byte(b.String())
}

func validationExpressions() []*fhirpath.Expression {
	return []*fhirpath.Expression{
		fhirpath.MustCompile("Bundle.entry.resource.name.exists()"),
		fhirpath.MustCompile("Bundle.entry.resource.name.family.exists()"),
		fhirpath.MustCompile("Bundle.entry.resource.telecom.value.exists()"),
		fhirpath.MustCompile("Bundle.entry.resource.all(gender in ('male' | 'female' | 'other'))"),
		fhirpath.MustCompile("Bundle.entry.resource.birthDate.exists()"),
		fhirpath.MustCompile("Bundle.entry.count() > 0"),
		fhirpath.MustCompile("Bundle.entry.resource.active = true"),
		fhirpath.MustCompile("Bundle.entry.resource.id.exists()"),
	}
}

// testModel implements fhirpath.Model for integration tests.
type testModel struct {
	choiceTypes      map[string][]string
	typeOf           map[string]string
	referenceTargets map[string][]string
	parentType       map[string]string
	resolvePath      map[string]string
	resources        map[string]bool
}

func (m *testModel) ChoiceTypes(path string) []string { return m.choiceTypes[path] }

func (m *testModel) TypeOf(path string) string { return m.typeOf[path] }

func (m *testModel) ReferenceTargets(path string) []string { return m.referenceTargets[path] }

func (m *testModel) ParentType(typeName string) string { return m.parentType[typeName] }

func (m *testModel) IsSubtype(child, parent string) bool {
	if child == parent {
		return true
	}
	current := child
	for {
		p, ok := m.parentType[current]
		if !ok || p == "" {
			return false
		}
		if p == parent {
			return true
		}
		current = p
	}
}

func (m *testModel) ResolvePath(path string) string {
	if resolved, ok := m.resolvePath[path]; ok {
		return resolved
	}
	return path
}

func (m *testModel) IsResource(typeName string) bool { return m.resources[typeName] }
