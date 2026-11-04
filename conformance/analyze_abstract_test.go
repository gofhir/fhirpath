package conformance

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r4b/v2"
	"github.com/gofhir/models/r5/v2"
)

// What the StructureDefinition snapshots say of the abstract types, which the
// published models leave out: every resource has an id, a meta, implicit
// rules and a language, every domain resource a narrative, contained
// resources and extensions, and every element an id and extensions. The
// elements are the same in R4, R4B and R5; the type of id is not (R4 gives
// string where R5 gives id), and no row depends on it.
var abstractElements = map[string]string{
	"Resource.id":                       "id",
	"Resource.meta":                     "Meta",
	"Resource.implicitRules":            "uri",
	"Resource.language":                 "code",
	"DomainResource.id":                 "id",
	"DomainResource.meta":               "Meta",
	"DomainResource.implicitRules":      "uri",
	"DomainResource.language":           "code",
	"DomainResource.text":               "Narrative",
	"DomainResource.contained":          "Resource",
	"DomainResource.extension":          "Extension",
	"DomainResource.modifierExtension":  "Extension",
	"BackboneElement.id":                "id",
	"BackboneElement.extension":         "Extension",
	"BackboneElement.modifierExtension": "Extension",
	"Element.id":                        "id",
	"Element.extension":                 "Extension",
}

// abstractTypes is the set each version's core package marks
// `abstract: true` on a type's own definition (derivation: specialization),
// as StructureDefinition.abstract defines it: "Whether structure this
// definition describes is abstract or not - that is, whether the structure is
// not intended to be instantiated" (structuredefinition.html). DataType is
// there from R4B; R5 adds five.
var abstractTypes = map[string][]string{
	"r4":  {"Resource", "DomainResource", "Element", "BackboneElement"},
	"r4b": {"Resource", "DomainResource", "Element", "BackboneElement", "DataType"},
	"r5": {"Resource", "DomainResource", "Element", "BackboneElement", "DataType",
		"Base", "BackboneType", "PrimitiveType", "CanonicalResource", "MetadataResource"},
}

// snapshotModel is a published model that also describes the abstract types,
// as a model built from the snapshots does.
type snapshotModel struct{ fhirpath.Model }

func (m snapshotModel) TypeOf(path string) string {
	if t, ok := abstractElements[path]; ok {
		return t
	}
	return m.Model.TypeOf(path)
}

// abstractModel adds the answer to IsAbstract.
type abstractModel struct {
	snapshotModel
	abstract map[string]bool
}

func (m abstractModel) IsAbstract(typeName string) bool { return m.abstract[typeName] }

// An element an abstract type does not have proves nothing about the instance:
// it holds a concrete type derived from it, whose elements are its own, and
// the model has to say the type is abstract for the analysis to know
// (StructureDefinition.abstract). A backbone element is typed BackboneElement,
// which is abstract too, and its path defines its elements, so a misspelling
// under it is still a fault.
func TestAnalyzeThroughAnAbstractType(t *testing.T) {
	models := map[string]fhirpath.Model{
		"r4": r4.FHIRPathModel(), "r4b": r4b.FHIRPathModel(), "r5": r5.FHIRPathModel(),
	}
	for _, version := range []string{"r4", "r4b", "r5"} {
		set := map[string]bool{}
		for _, name := range abstractTypes[version] {
			set[name] = true
		}
		base := snapshotModel{models[version]}
		with := abstractModel{base, set}

		for _, tt := range []struct {
			context, expr string
			// whether it faults when the model says which types are abstract,
			// and when it does not
			faultWith, faultWithout bool
		}{
			// what the issue reports
			{"Patient", "contained.contained.empty()", false, true},
			{"Patient", "contained.text.empty()", false, true},
			{"Bundle", "Bundle.entry.resource.name", false, true},
			{"Parameters", "Parameters.parameter.resource.name", false, true},
			// an element the abstract type has still resolves
			{"Patient", "contained.meta.exists()", false, false},
			{"Patient", "contained.id.exists()", false, false},
			// what follows an abstract type's missing element is unchecked
			{"Patient", "contained.contained.contained.id", false, true},
			// ofType(DomainResource) lands on DomainResource, which describes its
			// own elements: foo is missing, unless the model says the type is
			// abstract
			{"Patient", "contained.ofType(DomainResource).foo", false, true},
			// an abstract context: its own elements are not the instance's
			{"DomainResource", "nme", false, true},
			{"Patient", "contained.where(nme.exists())", false, true},
			// a concrete type, or a backbone element, keeps its fault
			{"Patient", "Patient.nme", true, true},
			{"Patient", "Patient.name.foo", true, true},
			{"Questionnaire", "Questionnaire.item.lnkId", true, true},
			{"Questionnaire.item", "item.item.lnkId", true, true},
			{"Patient", "Patient.contact.nme", true, true},
			{"Parameters", "Parameters.parameter.nme", true, true},
			// a resource type that is not the context is not reachable, from
			// an abstract type too
			{"Patient", "contained.Patient.name", true, true},
		} {
			for _, v := range []struct {
				name  string
				model fhirpath.Model
				fault bool
			}{
				{"with IsAbstract", with, tt.faultWith},
				{"without IsAbstract", base, tt.faultWithout},
			} {
				err := fhirpath.MustCompile(tt.expr).Analyze(v.model, tt.context)
				if (err != nil) != v.fault {
					t.Errorf("%s %s: %s on %s: error %v, want fault %v",
						version, v.name, tt.expr, tt.context, err, v.fault)
				}
			}
		}
	}
}

// The published models alone, which describe no abstract type, keep faulting
// on a backbone misspelling.
func TestAnalyzeBackboneMisspellingWithPublishedModels(t *testing.T) {
	for name, model := range map[string]fhirpath.Model{
		"r4": r4.FHIRPathModel(), "r4b": r4b.FHIRPathModel(), "r5": r5.FHIRPathModel(),
	} {
		for _, tt := range []struct{ context, expr string }{
			{"Questionnaire", "Questionnaire.item.lnkId"},
			{"Questionnaire.item", "item.item.lnkId"},
			{"Patient", "Patient.contact.nme"},
			{"Parameters", "Parameters.parameter.nme"},
		} {
			if err := fhirpath.MustCompile(tt.expr).Analyze(model, tt.context); err == nil {
				t.Errorf("%s: %s on %s: no fault", name, tt.expr, tt.context)
			}
		}
	}
}
