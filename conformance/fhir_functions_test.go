package conformance

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r5/v2"
)

// The FHIR functions the fhir-functions spec cases found missing or wrong
// (#167), each as its version's FHIRPath page has it.
var fhirFunctionsPatient = []byte(`{"resourceType":"Patient","id":"p",` +
	`"text":{"status":"generated","div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">x</div>"},` +
	`"modifierExtension":[{"url":"http://example.org/a","valueBoolean":true}],` +
	`"name":[{"family":"F"},{"family":"G"}],"gender":"male",` +
	`"maritalStatus":{"coding":[{"system":"http://s","code":"M"},{"system":"http://t","code":"X"}]}}`)

func evaluateIn(t *testing.T, version, expr string, opts ...fhirpath.EvalOption) (string, error) {
	t.Helper()
	var model fhirpath.Model = r4.FHIRPathModel()
	if version == "r5" {
		model = r5.FHIRPathModel()
	}
	result, err := fhirpath.MustCompile(expr).EvaluateWithOptions(fhirFunctionsPatient,
		append([]fhirpath.EvalOption{fhirpath.WithModel(model)}, opts...)...)
	if err != nil {
		return "", err
	}
	return result.String(), nil
}

func TestFHIRFunctionsAnswerAsTheirVersionSays(t *testing.T) {
	for _, tt := range []struct {
		version, expr, want string // want "error" for an error
	}{
		// subsumes() and subsumedBy(): equivalence needs no terminology
		{"r4", "Patient.maritalStatus.subsumes(Patient.maritalStatus.coding[1])", "[true]"},
		{"r4", "Patient.maritalStatus.coding[0].subsumedBy(Patient.maritalStatus)", "[true]"},
		{"r4", "Patient.maritalStatus.coding[0].subsumes(Patient.maritalStatus.coding[1])", "[]"},
		{"r4", "{}.subsumes(Patient.maritalStatus)", "[]"},
		// what is no Coding or CodeableConcept stands for no code
		{"r4", "Patient.subsumes(Patient.maritalStatus)", "[]"},
		{"r4", "Patient.name.first().subsumes(Patient.maritalStatus)", "[]"},
		{"r4", "Patient.maritalStatus.coding.subsumes(Patient.maritalStatus)", "error"},
		{"r5", "Patient.maritalStatus.coding.subsumes(Patient.maritalStatus)", "[]"},
		// htmlChecks(): empty on what is no single xhtml element or string;
		// without a checker, one of those ends the evaluation
		{"r5", "Patient.text.status.htmlChecks()", "[]"},
		{"r5", "Patient.text.`div`.combine(Patient.text.`div`).htmlChecks()", "[]"},
		{"r5", "Patient.text.`div`.htmlChecks()", "error"},
		{"r5", "'<b>x</b>'.htmlChecks()", "error"},
		{"r4", "1.htmlChecks()", "[]"},
		// checkModifiers(): the input, or an error for a modifier not allowed
		{"r4", "Patient.checkModifiers('http://example.org/a').id", "[p]"},
		{"r5", "Patient.checkModifiers('http://example.org/b,http://example.org/a').id", "[p]"},
		{"r4", "Patient.checkModifiers('http://example.org/b')", "error"},
		{"r5", "Patient.checkModifiers({})", "error"},
		{"r5", "Patient.name.checkModifiers('http://example.org/b').count()", "[2]"},
		// elementDefinition() and slice() need definitions the engine lacks
		{"r4", "{}.elementDefinition()", "[]"},
		{"r4", "Patient.name.elementDefinition()", "error"},
		{"r4", "Patient.name.slice('not a uri', 'x')", "error"},
		{"r5", "Patient.name.slice('not a uri', 'x')", "[]"},
		{"r5", "Patient.name.slice({}, 'x')", "[]"},
		{"r5", "Patient.name.slice('http://hl7.org/fhir/StructureDefinition/x', 'x')", "error"},
		// conformsTo() of several items: an error in R4, empty in R5
		{"r4", "Patient.name.conformsTo('http://hl7.org/fhir/StructureDefinition/HumanName')", "error"},
		{"r5", "Patient.name.conformsTo('http://hl7.org/fhir/StructureDefinition/HumanName')", "[]"},
		// memberOf() of a value set that is no URI: an error in R4, empty in R5
		{"r4", "Patient.gender.memberOf('not a uri')", "error"},
		{"r5", "Patient.gender.memberOf('not a uri')", "[]"},
		{"r4", "Patient.gender.memberOf('http://hl7.org/fhir/ValueSet/administrative-gender')", "[]"},
		{"r4", "Patient.gender.memberOf('ValueSet/administrative-gender')", "[]"},
		// trace()'s projection is evaluated per item
		{"r4", "Patient.name.trace('n', family).family", "[F, G]"},
		{"r4", "(1 | 2).trace('x', $this * 10)", "[1, 2]"},
		// comparable(): a number is a quantity of unit '1'; several items empty
		{"r4", "1.comparable(2)", "[true]"},
		{"r4", "2 '1'.comparable(3)", "[true]"},
		{"r4", "1.comparable(2 'mg')", "[false]"},
		{"r4", "(1 'mg' | 2 'mg').comparable(1 'g')", "[]"},
		// hasValue() of a System value of a primitive type
		{"r4", "'a'.hasValue()", "[true]"},
		{"r4", "@2020.hasValue()", "[true]"},
		{"r4", "1 'mg'.hasValue()", "[false]"},
		{"r4", "('a' | 'b').hasValue()", "[false]"},
		{"r4", "''.hasValue()", "[true]"},
		{"r4", "'a'.getValue()", "[]"},
	} {
		got, err := evaluateIn(t, tt.version, tt.expr)
		switch {
		case tt.want == "error" && err == nil:
			t.Errorf("%s %s = %s, want an error", tt.version, tt.expr, got)
		case tt.want != "error" && err != nil:
			t.Errorf("%s %s: %v", tt.version, tt.expr, err)
		case tt.want != "error" && got != tt.want:
			t.Errorf("%s %s = %s, want %s", tt.version, tt.expr, got, tt.want)
		}
	}
}

// htmlChecker passes the xhtml unless it holds forbidden, and fails when
// told to.
type htmlChecker struct {
	calls int
	fail  bool
}

func (c *htmlChecker) HTMLChecks(_ context.Context, xhtml string) (bool, error) {
	c.calls++
	if c.fail {
		return false, errors.New("checker failed")
	}
	return !strings.Contains(xhtml, "forbidden"), nil
}

// htmlChecks() asks the caller's checker about a single xhtml element or
// string, as fhirpath.js and the HL7 validator check both, and about nothing
// else: not a code, on which the two part, nor several items.
func TestHTMLChecksAsksTheCallersChecker(t *testing.T) {
	checker := &htmlChecker{}
	for expr, want := range map[string]string{
		"Patient.text.`div`.htmlChecks()":          "[true]",
		"Patient.name.first().family.htmlChecks()": "[true]",
		"'<b>forbidden</b>'.htmlChecks()":          "[false]",
		"Patient.text.status.htmlChecks()":         "[]",
		"Patient.gender.htmlChecks()":              "[]",
		"Patient.htmlChecks()":                     "[]",
		"Patient.descendants().htmlChecks()":       "[]",
	} {
		got, err := evaluateIn(t, "r5", expr, fhirpath.WithHTMLChecker(checker))
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
	if checker.calls != 3 {
		t.Errorf("the checker was asked %d times, want 3", checker.calls)
	}

	// The checker's verdict is the answer, and its failure ends the evaluation
	forbidden := []byte(`{"resourceType":"Patient","text":{"status":"generated",` +
		`"div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">forbidden</div>"}}`)
	result, err := fhirpath.MustCompile("Patient.text.`div`.htmlChecks()").EvaluateWithOptions(forbidden,
		fhirpath.WithModel(r5.FHIRPathModel()), fhirpath.WithHTMLChecker(&htmlChecker{}))
	if err != nil || result.String() != "[false]" {
		t.Errorf("htmlChecks() of a narrative the checker refuses = %v, %v; want [false]", result, err)
	}
	if _, err := evaluateIn(t, "r5", "Patient.text.`div`.htmlChecks()",
		fhirpath.WithHTMLChecker(&htmlChecker{fail: true})); err == nil {
		t.Error("htmlChecks() with a failing checker: no error, want one")
	}
}

// failingService cannot answer membership.
type failingService struct{}

func (failingService) MemberOf(context.Context, interface{}, string) (bool, error) {
	return false, errors.New("service down")
}

// memberOf() of a code the service could not answer for is unknown, empty,
// not "not a member". Up to 1.14.7 it was false.
func TestMemberOfIsUnknownWhenTheServiceFails(t *testing.T) {
	ctx := eval.NewContext(fhirFunctionsPatient)
	ctx.SetTerminologyService(failingService{})
	result, err := fhirpath.MustCompile("Patient.gender.memberOf('http://hl7.org/fhir/ValueSet/administrative-gender')").
		EvaluateWithContext(ctx)
	if err != nil || result.String() != "[]" {
		t.Errorf("memberOf() with a failing service = %v, %v; want []", result, err)
	}
}

// statusService knows two codes, final and M, as members of every value set,
// and a concept as a member when any of its codings is.
type statusService struct{}

func (statusService) MemberOf(_ context.Context, code interface{}, _ string) (bool, error) {
	m, _ := code.(map[string]interface{})
	codings, _ := m["coding"].([]map[string]interface{})
	for _, c := range append(codings, m) {
		if c["code"] == "final" || c["code"] == "M" {
			return true, nil
		}
	}
	return false, nil
}

// memberOf() of several items is empty under both versions, as R5 says and
// fhirpath.js and the Java engine both answer against tx.fhir.org under R4
// too, even where one item is a member.
func TestMemberOfOfSeveralItemsIsEmpty(t *testing.T) {
	for _, tt := range []struct {
		model      eval.Model
		expr, want string
	}{
		{r4.FHIRPathModel(), "'final'.memberOf('http://hl7.org/fhir/ValueSet/observation-status')", "[true]"},
		{r4.FHIRPathModel(), "'bogus'.memberOf('http://hl7.org/fhir/ValueSet/observation-status')", "[false]"},
		{r4.FHIRPathModel(), "('final' | 'bogus').memberOf('http://hl7.org/fhir/ValueSet/observation-status')", "[]"},
		{r5.FHIRPathModel(), "('bogus' | 'final').memberOf('http://hl7.org/fhir/ValueSet/observation-status')", "[]"},
		{r5.FHIRPathModel(), "'final'.memberOf('http://hl7.org/fhir/ValueSet/observation-status')", "[true]"},
		// a concept of two codings is one item, a member when any coding is;
		// its codings are two
		{r4.FHIRPathModel(), "Patient.maritalStatus.memberOf('http://hl7.org/fhir/ValueSet/marital-status')", "[true]"},
		{r4.FHIRPathModel(), "Patient.maritalStatus.coding.memberOf('http://hl7.org/fhir/ValueSet/marital-status')", "[]"},
		{r4.FHIRPathModel(), "Patient.maritalStatus.coding[1].memberOf('http://hl7.org/fhir/ValueSet/marital-status')", "[false]"},
		// several items are empty before the value set is looked at
		{r4.FHIRPathModel(), "('a' | 'b').memberOf('not a uri')", "[]"},
	} {
		ctx := eval.NewContext(fhirFunctionsPatient)
		ctx.SetModel(tt.model)
		ctx.SetTerminologyService(statusService{})
		result, err := fhirpath.MustCompile(tt.expr).EvaluateWithContext(ctx)
		if err != nil || result.String() != tt.want {
			t.Errorf("%s = %v, %v; want %s", tt.expr, result, err, tt.want)
		}
	}
}

type subsumer struct{}

func (subsumer) MemberOf(context.Context, interface{}, string) (bool, error) { return false, nil }

// Subsumes says http://s|A subsumes http://s|B and nothing else.
func (subsumer) Subsumes(_ context.Context, source, given interface{}) (bool, error) {
	// A CodeableConcept comes as its codings, as memberOf() passes it
	code := func(v interface{}) string {
		m, _ := v.(map[string]interface{})
		if codings, ok := m["coding"].([]map[string]interface{}); ok && len(codings) > 0 {
			m = codings[0]
		}
		c, _ := m["code"].(string)
		return c
	}
	return code(source) == "A" && code(given) == "B", nil
}

// subsumes() and subsumedBy() ask a terminology service that can tell
// subsumption once equivalence does not settle it, subsumedBy() with the
// sides swapped.
func TestSubsumptionAsksATerminologyServiceThatCanTell(t *testing.T) {
	data := []byte(`{"resourceType":"Observation","status":"final",` +
		`"code":{"coding":[{"system":"http://s","code":"A"}]},` +
		`"method":{"coding":[{"system":"http://s","code":"B"}]}}`)
	for expr, want := range map[string]string{
		"Observation.code.subsumes(Observation.method)":   "[true]",
		"Observation.method.subsumes(Observation.code)":   "[false]",
		"Observation.method.subsumedBy(Observation.code)": "[true]",
		"Observation.code.subsumedBy(Observation.method)": "[false]",
	} {
		ctx := eval.NewContext(data)
		ctx.SetTerminologyService(subsumer{})
		result, err := fhirpath.MustCompile(expr).EvaluateWithContext(ctx)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if got := result.String(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// An expression may open with a supertype of its context's type, as 2.0.0
// allows, and static analysis accepts it as evaluation does.
func TestAnalysisAcceptsASupertypeAtTheRoot(t *testing.T) {
	for _, tt := range []struct {
		expr  string
		fault bool
	}{
		{"DomainResource.text.status = 'generated'", false},
		{"Resource.id.exists()", false},
		{"DomainResource.nonexistent.exists()", true},
		{"Observation.status.exists()", true},
		{"name.DomainResource.exists()", true},
		{"Patient.contained.ofType(Patient).DomainResource.text.exists()", true},
		{"Patient.where(DomainResource.text.exists()).exists()", false},
		// a FHIR function that returns its input keeps its type
		{"Patient.checkModifiers('http://example.org/a').name.family.exists()", false},
		{"Patient.checkModifiers('http://example.org/a').nmae.exists()", true},
		{"Patient.name.slice('http://example.org/s', 'x').nmae.exists()", true},
	} {
		err := fhirpath.MustCompile(tt.expr).Analyze(r4.FHIRPathModel(), "Patient")
		if (err != nil) != tt.fault {
			t.Errorf("%s: fault %v, want %v", tt.expr, err, tt.fault)
		}
	}
}
