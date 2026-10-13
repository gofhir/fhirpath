package conformance

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
	"github.com/gofhir/models/r4/v2"
)

// A validator evaluates an element's constraints with the element as the
// root, and builds a primitive with no value, its "_family" sibling, as an
// object typed as the primitive. It answers as navigation does, as the HL7
// validator 6.10.2 does: hasValue() is false, so ele-1 fails on an element
// that holds only an id. A complex value has no value either. (#161)
func TestAPrimitiveWithNoValueBuiltAsTheRootHasNoValue(t *testing.T) {
	tests := []struct{ json, typeName, expr, want string }{
		{`{"id":"x"}`, "string", "hasValue()", "[false]"},
		{`{"id":"x"}`, "string", "hasValue() or (children().count() > id.count())", "[false]"},
		{`{"extension":[{"url":"http://x","valueString":"y"}]}`, "date", "$this.hasValue().not()", "[true]"},
		{`{"extension":[{"url":"http://x","valueString":"y"}]}`, "date", "hasValue() or (children().count() > id.count())", "[true]"},
		// A complex value is no FHIR primitive, and has none, as the text
		// and fhirpath.js have it
		{`{"id":"x"}`, "CodeableConcept", "hasValue()", "[false]"},
		{`{"id":"x"}`, "CodeableConcept", "hasValue() or (children().count() > id.count())", "[false]"},
		// The type as the R4 model gives it for Resource.id, and as others
		// spell a primitive
		{`{"id":"x"}`, "http://hl7.org/fhirpath/System.String", "hasValue()", "[false]"},
		{`{"id":"x"}`, "http://hl7.org/fhirpath/System.String", "hasValue() or (children().count() > id.count())", "[false]"},
		{`{"id":"x"}`, "FHIR.string", "hasValue()", "[false]"},
		{`{"id":"x"}`, "String", "hasValue()", "[false]"},
	}

	for _, tt := range tests {
		t.Run(tt.typeName+"/"+tt.expr, func(t *testing.T) {
			ctx := eval.NewContextForRoot(types.Collection{types.NewObjectValueWithType([]byte(tt.json), tt.typeName)})
			ctx.SetModel(r4.FHIRPathModel())
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithContext(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}
		})
	}
}
