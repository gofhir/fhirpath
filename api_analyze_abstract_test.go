package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// abstractAnalysisModel describes Resource and DomainResource, as a model
// built from the StructureDefinition snapshots does, over analysisModel.
type abstractAnalysisModel struct{ analysisModel }

var abstractAnalysisTypes = map[string]string{
	"Resource.id":              "id",
	"Resource.meta":            "Meta",
	"DomainResource.text":      "Narrative",
	"DomainResource.contained": "Resource",
	"Patient.contained":        "Resource",
	"Meta.id":                  "string",
	"Meta.versionId":           "id",
}

func (m abstractAnalysisModel) TypeOf(path string) string {
	if t, ok := abstractAnalysisTypes[path]; ok {
		return t
	}
	return m.analysisModel.TypeOf(path)
}

// abstractSetModel adds IsAbstract, as StructureDefinition.abstract gives it:
// BackboneElement is abstract too.
type abstractSetModel struct{ abstractAnalysisModel }

func (abstractSetModel) IsAbstract(typeName string) bool {
	switch typeName {
	case "Resource", "DomainResource", "Element", "BackboneElement":
		return true
	}
	return false
}

// A missing element of an abstract type proves nothing: an instance there is
// of a concrete type derived from it. The model has to say which types are
// abstract; one that does not keeps the fault. A backbone element is typed
// with an abstract type and defines its own elements, so it keeps the fault.
func TestAnalyzeThroughAnAbstractTypeNeedsTheModelToSayIt(t *testing.T) {
	for _, tt := range []struct {
		expr, context string
		fault         bool // with IsAbstract
		faultWithout  bool
	}{
		{"contained.contained.empty()", "Patient", false, true},
		{"contained.text.empty()", "Patient", false, true},
		{"contained.contained.contained.id", "Patient", false, true},
		{"contained.where(nme.exists())", "Patient", false, true},
		// The step through an abstract type lands on no type, so what follows
		// is unchecked: text is a Narrative in the instance, not a Resource,
		// and Resource's meta is not to be read off it
		{"contained.text.meta.nme", "Patient", false, true},
		{"contained.meta.versionId", "Patient", false, false},
		{"contained.meta.nme", "Patient", true, true},
		{"Patient.nme", "Patient", true, true},
		{"Patient.name.foo", "Patient", true, true},
		{"Patient.contact.nme", "Patient", true, true},
		{"contained.Patient.name", "Patient", true, true},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			if err := fhirpath.MustCompile(tt.expr).Analyze(abstractSetModel{}, tt.context); (err != nil) != tt.fault {
				t.Errorf("with IsAbstract: %v, want fault %v", err, tt.fault)
			}
			if err := fhirpath.MustCompile(tt.expr).Analyze(abstractAnalysisModel{}, tt.context); (err != nil) != tt.faultWithout {
				t.Errorf("without IsAbstract: %v, want fault %v", err, tt.faultWithout)
			}
		})
	}
}
