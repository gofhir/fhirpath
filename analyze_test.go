package fhirpath

import "testing"

// analysisModel is a model with enough of Patient and Observation to analyze
// against, including a choice element and a backbone one.
type analysisModel struct{ testModel }

var analysisTypes = map[string]string{
	"Patient.id":                   "string",
	"Patient.name":                 "HumanName",
	"Patient.active":               "boolean",
	"Patient.birthDate":            "date",
	"Patient.telecom":              "ContactPoint",
	"Patient.contact":              "BackboneElement",
	"Patient.contact.id":           "string",
	"Patient.contact.relationship": "CodeableConcept",
	"Patient.deceasedBoolean":      "boolean",
	"HumanName.id":                 "string",
	"HumanName.use":                "code",
	"HumanName.given":              "string",
	"HumanName.family":             "string",
	"ContactPoint.id":              "string",
	"ContactPoint.system":          "code",
	"ContactPoint.value":           "string",
	"CodeableConcept.id":           "string",
	"CodeableConcept.text":         "string",
	"Observation.id":               "string",
	"Observation.status":           "code",
	"Observation.valueQuantity":    "Quantity",
	"Quantity.id":                  "string",
	"Quantity.unit":                "string",
	"Quantity.value":               "decimal",
	"Period.id":                    "string",
	"Period.start":                 "dateTime",
}

func (analysisModel) TypeOf(path string) string { return analysisTypes[path] }

func (analysisModel) ChoiceTypes(path string) []string {
	if path == "Observation.value" {
		return []string{"Quantity", "string", "Period"}
	}
	if path == "Patient.deceased" {
		return []string{"boolean", "dateTime"}
	}
	return nil
}

func (analysisModel) IsResource(name string) bool {
	return name == "Patient" || name == "Observation" || name == "Encounter"
}

// TestAnalyzeRejectsWhatCannotBeValid covers the faults static analysis can
// prove, which evaluation cannot see.
//
// Each of these evaluates to something perfectly reasonable against a document —
// usually empty — and is still wrong, because the model says the expression
// could never mean anything. The conformance suite calls them semantic errors
// for that reason.
func TestAnalyzeRejectsWhatCannotBeValid(t *testing.T) {
	model := analysisModel{}

	cases := []struct {
		expr    string
		context string
		reason  string
	}{
		{"name.given1", "Patient", "given1 is not an element of HumanName"},
		{"Patient.name.family1", "Patient", "nor is family1"},
		{"Encounter.name.given", "Patient", "an expression against a Patient cannot name another resource"},

		// A choice element is navigated through ofType(), not by the name its
		// JSON representation uses
		{"Observation.valueQuantity.unit", "Observation", "valueQuantity is the JSON spelling of value[x]"},
		{"Observation.valueQuantity.exists()", "Observation", "same, even where the result is only tested"},
		{"Patient.deceasedBoolean", "Patient", "same for any choice element"},

		// A type the value cannot have
		{"(Observation.value as Period).unit", "Observation", "Period has no unit"},

		// Reading a collection that has no order
		{"Patient.children().skip(1)", "Patient", "children() is unordered"},
		{"Patient.descendants().first()", "Patient", "so is descendants()"},
		{"Patient.children()[0]", "Patient", "an indexer reads by position too"},

		// A criterion that cannot be a Boolean
		{"iif(1 | 2 | 3, true, false)", "Patient", "a union of three values is not a singleton"},
		{"iif('non boolean criteria', 'yes', 'no')", "Patient", "a String literal is not a Boolean"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if err := MustCompile(tc.expr).Analyze(model, tc.context); err == nil {
				t.Errorf("expected a fault: %s", tc.reason)
			}
		})
	}
}

// TestAnalyzeAcceptsValidExpressions is the half that matters more.
//
// A false positive rejects an expression that works, which is worse than
// missing a fault: the analysis is meant to catch typos, not to become one. So
// it reports only what the model makes certain, and stops following a branch as
// soon as the type stops being known.
func TestAnalyzeAcceptsValidExpressions(t *testing.T) {
	model := analysisModel{}

	cases := []struct {
		expr    string
		context string
	}{
		// Plain navigation, with and without the leading type name
		{"Patient.name.given", "Patient"},
		{"name.given", "Patient"},
		{"Patient.name.family", "Patient"},

		// A scoped function navigates its argument from the item, not from the
		// context: use belongs to HumanName, not to Patient
		{"Patient.name.where(use = 'official').given", "Patient"},
		{"Patient.telecom.where(system = 'phone').value", "Patient"},
		{"Patient.name.select(given.first())", "Patient"},
		{"Patient.name.all(use.exists())", "Patient"},

		// ...and leaves the path where it found it
		{"Patient.name.where(use = 'official').given.first()", "Patient"},
		{"Patient.contact.where(relationship.text = 'x').id", "Patient"},

		// A backbone element holds elements its type does not
		{"Patient.contact.relationship.text", "Patient"},

		// A choice element, navigated the way it is meant to be
		{"Observation.value.ofType(Quantity).unit", "Observation"},
		{"(Observation.value as Quantity).unit", "Observation"},

		// A union with one non-empty side is a singleton
		{"iif({} | true, true, false)", "Patient"},
		{"iif(Patient.active, 'yes', 'no')", "Patient"},

		// Where the type stops being known, so does the checking
		{"Patient.extension('http://x').value.anything", "Patient"},
		{"Patient.descendants().count()", "Patient"},
		{"Patient.children().count()", "Patient"},
		{"%resource.anything.at.all", "Patient"},

		// Operators and literals
		{"Patient.birthDate + 1 year", "Patient"},
		{"Patient.name.given.count() > 1", "Patient"},
		{"Patient.active or Patient.name.exists()", "Patient"},
		{"'a' | 'b'", "Patient"},

		// No context type at all: nothing is provable, nothing is reported
		{"name.given1", ""},
		{"anything.at.all", ""},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if err := MustCompile(tc.expr).Analyze(model, tc.context); err != nil {
				t.Errorf("false positive: %v", err)
			}
		})
	}
}

// TestAnalyzeWithoutAModel checks that analysis is opt-in: without a model there
// is nothing to check against, and the expression is not rejected.
func TestAnalyzeWithoutAModel(t *testing.T) {
	if err := MustCompile("name.given1").Analyze(nil, "Patient"); err != nil {
		t.Errorf("expected no fault without a model, got %v", err)
	}
}

// TestAnalyzeDoesNotAffectEvaluation checks that the strict reading stays a
// separate call.
//
// The engine evaluates leniently by default, which is what the conformance suite
// asks for with mode="lenient/polymorphics": Observation.valueQuantity.exists()
// is a semantic error and also, in that mode, a question with an answer.
func TestAnalyzeDoesNotAffectEvaluation(t *testing.T) {
	observation := []byte(`{"resourceType":"Observation","status":"final","valueQuantity":{"value":6,"unit":"lbs"}}`)

	result, err := MustCompile("Observation.valueQuantity.unit").
		EvaluateWithOptions(observation, WithModel(analysisModel{}))
	if err != nil {
		t.Fatalf("evaluation should not apply the strict reading: %v", err)
	}
	if len(result) != 1 || result[0].String() != "lbs" {
		t.Errorf("got %v, want lbs", result)
	}
}
