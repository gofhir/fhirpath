package conformance

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r5/v2"
)

// Static analysis navigates each operand of an operator and each argument of
// a function from where the expression stands, and a path defined elsewhere
// as the model resolves it, as evaluation does (#180). It still finds the
// element that is not there.
func TestAnalysisFollowsPathsAsEvaluationDoes(t *testing.T) {
	for _, model := range []struct {
		name  string
		model fhirpath.Model
	}{{"r4", r4.FHIRPathModel()}, {"r5", r5.FHIRPathModel()}} {
		for _, tt := range []struct {
			context, expr string
			fault         bool
		}{
			// one operand does not move where the next starts
			{"Questionnaire", "Questionnaire.repeat(item | item.item).count()", false},
			{"CapabilityStatement", "document.select(profile & mode).isDistinct()", false},
			// nor one argument where the next starts
			{"Bundle", "entry.select(fullUrl & iif(resource.meta.versionId.exists(), resource.meta.versionId, ''))", false},
			// a path defined elsewhere, from the context or reached
			{"Questionnaire.item", "type = 'group' implies item.empty().not()", false},
			{"Questionnaire", "item.item.item.linkId.exists()", false},
			{"TestReport.test.action", "operation.exists() xor assert.exists()", false},
			// an element that is not there
			{"Questionnaire", "item.item.nonexistent", true},
			{"Questionnaire.item", "nonexistent.exists()", true},
			{"Questionnaire", "(item | nonexistent).count()", true},
			// a function that keeps its input's type keeps its path
			{"Patient", "contact.ofType(BackboneElement).name.exists()", false},
			{"Patient", "contact.where(name.exists()).first().name.family.exists()", false},
			// an index is evaluated where the indexer stands, and leaves the
			// base's path for what follows (#188)
			{"Patient", "Patient.contact[link.count()].name.exists()", false},
			{"Patient", "Patient.contact[0].nonexistent.exists()", true},
			{"Patient", "Patient.contact[nonexistent.count()].name.exists()", true},
			// a context that is itself a path defined elsewhere
			{"Questionnaire.item.item", "foo.exists()", true},
			{"Questionnaire.item.item", "linkId.exists() and item.linkId.exists()", false},
			// a choice element by its type suffix, under a backbone element too
			{"Observation", "Observation.component.valueQuantity.exists()", true},
			{"Questionnaire", "Questionnaire.item.answerOption.valueCoding.exists()", true},
			{"Observation", "Observation.component.value.ofType(Quantity).exists()", false},
			{"Observation", "Observation.component.code.coding.exists()", false},
			// a System value has no elements; a FHIR primitive ofType() or as()
			// returns has its id and extensions
			{"Patient", "Patient.name.count().given", true},
			{"Patient", "Patient.name.exists().value", true},
			{"", "'a'.length", true},
			{"Patient", "Patient.name.given.ofType(string).extension.exists()", false},
			{"Patient", "(Patient.birthDate as date).id.exists()", false},
			{"Patient", "Patient.name.count().toString().length() > 1", false},
			{"Patient", "Patient.name.where(given.count() > 1).family.exists()", false},
			{"Patient", "(5 'mg').value.exists() and '5 mg'.toQuantity().unit.exists()", false},
			{"Patient", "'a'.upper().length() = 1 and today().toString().length() > 0", false},
			{"Patient", "Patient.name.where(given[0] = 'x') | Patient.contact[0].telecom[0].system", false},
			// union() and combine() join items of other types, as | does
			{"Patient", "'a'.combine(Patient.name.given).extension.exists()", false},
			{"Patient", "Patient.gender.toString().union(Patient.gender).id.exists()", false},
			{"Patient", "Patient.name.union(Patient.telecom).system.exists()", false},
			// aggregate()'s init is read on the outer context
			{"Patient", "Patient.name.aggregate($total + 1, telecom.count())", false},
			{"Patient", "Patient.name.count().aggregate($total + $this, name.count())", false},
			{"Patient", "Patient.name.aggregate($total + 1, nonexistent.count())", true},
		} {
			err := fhirpath.MustCompile(tt.expr).Analyze(model.model, tt.context)
			if (err != nil) != tt.fault {
				t.Errorf("%s: %s on %s: fault %v, want one: %v", model.name, tt.expr, tt.context, err, tt.fault)
			}
		}
	}
}
