package conformance

import (
	"slices"
	"testing"

	"github.com/gofhir/fhirpath"
)

// FHIR's JSON writes a choice element only with its type suffix, valueString
// for value[x]: a key "value" where the model says the element is a choice is
// no element, and a suffix the version does not allow is no value of it. HL7's
// Java engine and fhirpath.js both answer so (#219; measured with
// make three-engine, validator 6.10.2 and fhirpath.js 5.2.0). Choices inside
// backbone elements and contentReference paths are read by the model as those
// of a resource are.

var choiceResources = map[string]string{
	"extension": `{"resourceType":"Patient","id":"p","extension":[
		{"url":"lit","value":"lit"},
		{"url":"s","valueString":"x","value":"bare"}]}`,
	"component": `{"resourceType":"Observation","id":"o","status":"final","code":{"text":"c"},
		"component":[
			{"code":{"text":"k"},"valueCode":"compcode"},
			{"code":{"text":"k2"},"value":"barecomp","valueString":"vs"}]}`,
	"observation": `{"resourceType":"Observation","id":"o","status":"final","code":{"text":"c"},
		"value":"bareobs","valueString":"vs"}`,
	"deceased": `{"resourceType":"Patient","id":"p","deceased":"bare","deceasedBoolean":true}`,
	"dosage": `{"resourceType":"MedicationRequest","id":"m","status":"active","intent":"order",
		"subject":{"reference":"Patient/p"},
		"dosageInstruction":[{
			"timing":{"repeat":{"boundsCode":"x"}},
			"doseAndRate":[{"doseCode":"x"},{"doseQuantity":{"value":2}}]},{
			"timing":{"repeat":{"boundsDuration":{"value":3}}}}]}`,
	"questionnaire": `{"resourceType":"Questionnaire","id":"q","status":"draft","item":[
		{"linkId":"1","type":"group","item":[
			{"linkId":"1.1","type":"string","enableWhen":[
				{"question":"1","operator":"=","answerCode":"top"},
				{"question":"1","operator":"=","answerBoolean":true}]}]}]}`,
	"parameters": `{"resourceType":"Parameters","parameter":[
		{"name":"a","part":[{"name":"b","value":"bare","valueString":"ps"}]}]}`,
	"contained": `{"resourceType":"Patient","id":"p","contained":[
		{"resourceType":"Observation","id":"o","status":"final","code":{"text":"c"},
		 "value":"bareincont","valueInteger":3}]}`,
	"bundle": `{"resourceType":"Bundle","type":"collection","entry":[
		{"resource":{"resourceType":"Observation","id":"o","status":"final","code":{"text":"c"},
		 "value":"bareinbundle"}}]}`,
	"unchanged": `{"resourceType":"Observation","id":"o","status":"final","code":{"text":"c"},
		"valueQuantity":{"value":5},"subject":{"identifier":{"value":"idv"}}}`,
	"primitive": `{"resourceType":"Patient","id":"p","birthDate":"1970-01-01","_birthDate":{"id":"b"},
		"name":[{"given":["a"],"_given":[{"id":"g"}]}]}`,
	"invalid suffix": `{"resourceType":"Observation","id":"o","status":"final","code":{"text":"c"},
		"valueCode":"topcode"}`,
	// an object that also holds the element beside a primitive is read with
	// its fields paired, another path to the same rule
	"paired": `{"resourceType":"Observation","id":"o","status":"final","_status":{"id":"s"},
		"code":{"text":"c"},"value":"bare","valueString":"vs"}`,
	"bare with element": `{"resourceType":"Patient","id":"p","extension":[
		{"url":"s","value":"x","_value":{"id":"e"}}]}`,
}

func TestAChoiceIsReadOnlyByTheSuffixesTheModelAllows(t *testing.T) {
	all := []string{"r4", "r4b", "r5"}
	for _, tt := range []struct {
		resource string
		versions []string
		expr     string
		want     string
	}{
		// an extension's value[x]
		{"extension", all, "Patient.extension('lit').value", "[]"},
		{"extension", all, "Patient.extension('s').value", "[string x]"},
		{"extension", all, "Patient.extension('s').children()", "[uri s, string x]"},
		{"extension", all, "Patient.extension.descendants().count()", "[Integer 3]"},
		// a backbone element's: a suffix the version does not allow, and a bare
		// key beside an allowed one
		{"component", all, "Observation.component.value", "[string vs]"},
		{"component", all, "Observation.component[1].children().count()", "[Integer 2]"},
		// backbone elements of datatypes
		{"dosage", all, "MedicationRequest.dosageInstruction.doseAndRate.dose", "[Quantity {\"value\":2}]"},
		{"dosage", all, "MedicationRequest.dosageInstruction.timing.repeat.bounds", "[Duration {\"value\":3}]"},
		// contentReference paths
		{"questionnaire", all, "Questionnaire.item.item.enableWhen.answer", "[boolean true]"},
		{"parameters", all, "Parameters.parameter.part.value", "[string ps]"},
		// a resource's, alone and in a contained resource or a Bundle entry
		{"observation", all, "Observation.value", "[string vs]"},
		{"observation", all, "Observation.children().count()", "[Integer 4]"},
		{"deceased", all, "Patient.deceased", "[boolean true]"},
		{"contained", all, "Patient.contained.value", "[integer 3]"},
		{"bundle", all, "Bundle.entry.resource.value", "[]"},
		// unchanged: elements named value that are no choice, the element FHIR
		// writes beside a primitive, a suffix no definition allows in children()
		{"unchanged", all, "Observation.value.value", "[decimal 5]"},
		{"unchanged", all, "Observation.subject.identifier.value", "[string idv]"},
		{"primitive", all, "Patient._birthDate.id.count()", "[Integer 1]"},
		{"primitive", all, "Patient.name._given.count()", "[Integer 1]"},
		{"invalid suffix", all, "Observation.value", "[]"},
		{"invalid suffix", all, "Observation.children().count()", "[Integer 4]"},
		// the same where the object's fields are read paired
		{"paired", all, "Observation.children().count()", "[Integer 4]"},
		{"paired", all, "Observation.value", "[string vs]"},
		{"bare with element", all, "Patient.extension.value", "[]"},
		{"bare with element", all, "Patient.extension.children()", "[uri s]"},
	} {
		for _, v := range versionModels {
			if !slices.Contains(tt.versions, v.name) {
				continue
			}
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(choiceResources[tt.resource]), fhirpath.WithModel(v.model))
			got := typedItems(result)
			if err != nil {
				got = "error: " + err.Error()
			}
			if got != tt.want {
				t.Errorf("%s: %s over %s = %s, want %s", v.name, tt.expr, tt.resource, got, tt.want)
			}
		}
	}
}

// Without a model there is no type to tell a choice by: a bare key is read as
// it stands, as fhirpath.js reads it, and suffixes are taken from the field
// name, the engine's documented way to evaluate without one.
func TestWithoutAModelAChoiceIsReadAsItStands(t *testing.T) {
	for _, tt := range []struct{ resource, expr, want string }{
		{"extension", "Patient.extension('lit').value.count()", "[Integer 1]"},
		{"observation", "Observation.value.count()", "[Integer 1]"},
		{"component", "Observation.component.value.count()", "[Integer 2]"},
	} {
		result, err := fhirpath.MustCompile(tt.expr).Evaluate([]byte(choiceResources[tt.resource]))
		got := typedItems(result)
		if err != nil {
			got = "error: " + err.Error()
		}
		if got != tt.want {
			t.Errorf("%s over %s = %s, want %s", tt.expr, tt.resource, got, tt.want)
		}
	}
}
