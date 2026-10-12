package conformance

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r4b/v2"
	"github.com/gofhir/models/r5/v2"
)

// With the generated models, = and ~ read as quantities only objects typed
// Quantity or one of its profiles, as the HL7 validator 6.10.2 does. An
// adjudication has a numeric value too: read as a quantity, two with the same
// value were equal whatever their category.
func TestOnlyAQuantityComparesAsOneWithTheGeneratedModels(t *testing.T) {
	claim := `{"resourceType":"ClaimResponse","item":[{"itemSequence":1,"adjudication":[` +
		`{"category":{"text":"a"},"value":1},{"category":{"text":"b"},"value":1}]}]}`
	tests := []struct{ expr, want string }{
		{"ClaimResponse.item.adjudication[0] = ClaimResponse.item.adjudication[1]", "[false]"},
		{"ClaimResponse.item.adjudication[0] ~ ClaimResponse.item.adjudication[1]", "[false]"},
		{"ClaimResponse.item.adjudication[0] = ClaimResponse.item.adjudication[0]", "[true]"},
	}

	for _, model := range []struct {
		name  string
		model fhirpath.Model
	}{{"r4", r4.FHIRPathModel()}, {"r4b", r4b.FHIRPathModel()}, {"r5", r5.FHIRPathModel()}} {
		for _, tt := range tests {
			t.Run(model.name+"/"+tt.expr, func(t *testing.T) {
				result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(claim), fhirpath.WithModel(model.model))
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
				}
			})
		}
	}
}
