package conformance

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/models/r4"
	"github.com/gofhir/models/r4b"
	"github.com/gofhir/models/r5"
)

// With the generated models, a root that is an element takes the type the model
// gives the path set with SetPath. AU Core's au-core-obs-02 starts with
// $this is dateTime, evaluated on Observation.effective[x] as the root, and a
// dateTime of day precision used to be guessed a Date.
func TestARootTakesTheTypeTheModelGivesItsPathWithTheGeneratedModels(t *testing.T) {
	tests := []struct {
		root, path, expr, want string
	}{
		{`"2019-12-08"`, "Observation.effectiveDateTime", "$this is dateTime", "[true]"},
		{`"2019-12-08"`, "Observation.effectiveDateTime", "$this.type().name", "[dateTime]"},
		{`"2019-12-08"`, "Patient.birthDate", "$this.type().name", "[date]"},
		{`"final"`, "Observation.status", "$this.type().name", "[code]"},
		{`{"value":5,"unit":"mg"}`, "Observation.valueQuantity", "value.type().name", "[decimal]"},
	}

	for _, model := range []struct {
		name  string
		model fhirpath.Model
	}{{"r4", r4.FHIRPathModel()}, {"r4b", r4b.FHIRPathModel()}, {"r5", r5.FHIRPathModel()}} {
		for _, tt := range tests {
			t.Run(model.name+"/"+tt.path+" "+tt.expr, func(t *testing.T) {
				ctx := eval.NewContext([]byte(tt.root))
				ctx.SetModel(model.model)
				ctx.SetPath(tt.path)
				result, err := fhirpath.MustCompile(tt.expr).EvaluateWithContext(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s at %s = %s, want %s", tt.expr, tt.path, got, tt.want)
				}
			})
		}
	}
}
