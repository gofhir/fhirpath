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
		} {
			err := fhirpath.MustCompile(tt.expr).Analyze(model.model, tt.context)
			if (err != nil) != tt.fault {
				t.Errorf("%s: %s on %s: fault %v, want one: %v", model.name, tt.expr, tt.context, err, tt.fault)
			}
		}
	}
}
