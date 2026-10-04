package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// A validator evaluates an element's invariants with the element as the root,
// and gives the root's path with SetPath so that a model can type it. A
// primitive root used to be typed by guessing anyway: "2019-12-08" at
// Observation.effectiveDateTime was a Date, and AU Core's au-core-obs-02,
// which starts with $this is dateTime, failed on a day-precision dateTime.
func TestARootTakesTheTypeTheModelGivesItsPath(t *testing.T) {
	model := &testModel{
		typeOf: map[string]string{
			"Observation.effectiveDateTime": "dateTime",
			"Observation.status":            "code",
			"Observation.valueQuantity":     "Quantity",
			"Quantity.value":                "decimal",
			"Patient.birthDate":             "date",
			"Patient.name":                  "HumanName",
			"HumanName.given":               "string",
		},
		// A model may know a choice element only by its base name.
		choiceTypes: map[string][]string{"Procedure.performed": {"dateTime", "Period", "string"}},
	}

	tests := []struct {
		root, path, expr, want string
	}{
		{`"2019-12-08"`, "Observation.effectiveDateTime", "$this is dateTime", "[true]"},
		{`"2019-12-08"`, "Observation.effectiveDateTime", "$this.type().name", "[dateTime]"},
		{`"2019-12-08T10:00:00Z"`, "Observation.effectiveDateTime", "$this is dateTime", "[true]"},
		{`"2019-12-08"`, "Procedure.performedDateTime", "$this is dateTime", "[true]"},
		{`"2019-12-08"`, "Patient.birthDate", "$this.type().name", "[date]"},
		{`"final"`, "Observation.status", "$this.type().name", "[code]"},
		{`{"value":5,"unit":"mg"}`, "Observation.valueQuantity", "$this.type().name", "[Quantity]"},
		{`{"value":5,"unit":"mg"}`, "Observation.valueQuantity", "value.type().name", "[decimal]"},
		{`{"given":["2019"]}`, "Patient.name", "given.type().name", "[string]"},
		// A path the model does not know leaves the root as it was read.
		{`"2019-12-08"`, "Basic.unknown", "$this.type().name", "[Date]"},
	}

	for _, tt := range tests {
		t.Run(tt.path+" "+tt.expr, func(t *testing.T) {
			expr := fhirpath.MustCompile(tt.expr)

			// The model and the path can be given in either order.
			for _, modelFirst := range []bool{true, false} {
				ctx := eval.NewContext([]byte(tt.root))
				if modelFirst {
					ctx.SetModel(model)
					ctx.SetPath(tt.path)
				} else {
					ctx.SetPath(tt.path)
					ctx.SetModel(model)
				}
				result, err := expr.EvaluateWithContext(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s at %s (model first: %v) = %s, want %s", tt.expr, tt.path, modelFirst, got, tt.want)
				}
			}
		})
	}
}

// Without a model, or without a path, the root is read as it always was.
func TestARootWithoutAModelOrAPathIsReadAsBefore(t *testing.T) {
	model := &testModel{typeOf: map[string]string{"Observation.effectiveDateTime": "dateTime"}}

	noModel := eval.NewContext([]byte(`"2019-12-08"`))
	noModel.SetPath("Observation.effectiveDateTime")
	noPath := eval.NewContext([]byte(`"2019-12-08"`))
	noPath.SetModel(model)

	for name, ctx := range map[string]*eval.Context{"no model": noModel, "no path": noPath} {
		result, err := fhirpath.MustCompile("$this.type().name").EvaluateWithContext(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != "[Date]" {
			t.Errorf("%s: $this.type().name = %s, want [Date]", name, got)
		}
	}
}

// A context reused for another element follows the path it is given now: a
// root typed for one path is read as it was once the path, or the model, no
// longer types it.
func TestARootFollowsTheCurrentPath(t *testing.T) {
	model := &testModel{typeOf: map[string]string{
		"Observation.effectiveDateTime": "dateTime",
		"Patient.birthDate":             "date",
	}}
	expr := fhirpath.MustCompile("$this.type().name")

	ctx := eval.NewContext([]byte(`"2019-12-08"`))
	ctx.SetModel(model)

	for _, step := range []struct {
		set  func()
		want string
	}{
		{func() { ctx.SetPath("Observation.effectiveDateTime") }, "[dateTime]"},
		{func() { ctx.SetPath("Patient.birthDate") }, "[date]"},
		{func() { ctx.SetPath("Basic.unknown") }, "[Date]"},
		{func() { ctx.SetPath("Observation.effectiveDateTime") }, "[dateTime]"},
		{func() { ctx.SetPath("") }, "[Date]"},
		{func() { ctx.SetPath("Patient.birthDate") }, "[date]"},
		{func() { ctx.SetModel(nil) }, "[Date]"},
	} {
		step.set()
		result, err := expr.EvaluateWithContext(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != step.want {
			t.Errorf("at path %q = %s, want %s", ctx.Path(), got, step.want)
		}
	}
}

// A caller building the root itself can type it the same way.
func TestJSONToCollectionWithType(t *testing.T) {
	root, err := types.JSONToCollectionWithType([]byte(`"2019-12-08"`), "dateTime")
	if err != nil {
		t.Fatal(err)
	}
	result, err := fhirpath.MustCompile("$this is dateTime").EvaluateWithContext(eval.NewContextForRoot(root))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.String(); got != "[true]" {
		t.Errorf("$this is dateTime = %s, want [true]", got)
	}
}

// A context made for a checked document places its root under the model's type
// as NewContext's does, reading it again without the scan NewContext's makes.
func TestAValidJSONRootTakesTheModelsTypeAsNewContextsDoes(t *testing.T) {
	model := &testModel{typeOf: map[string]string{
		"Observation.valueQuantity":     "Quantity",
		"Quantity.value":                "decimal",
		"Observation.effectiveDateTime": "dateTime",
	}}

	for _, tt := range []struct {
		root, path, expr string
	}{
		{` {"value":5,"unit":"mg"} `, "Observation.valueQuantity", "$this.type().name"},
		{` {"value":5,"unit":"mg"} `, "Observation.valueQuantity", "value.type().name"},
		{`"2019-12-08"`, "Observation.effectiveDateTime", "$this is dateTime"},
	} {
		t.Run(tt.path+" "+tt.expr, func(t *testing.T) {
			expr := fhirpath.MustCompile(tt.expr)
			evaluate := func(ctx *eval.Context) string {
				ctx.SetModel(model)
				ctx.SetPath(tt.path)
				result, err := expr.EvaluateWithContext(ctx)
				if err != nil {
					t.Fatal(err)
				}
				return result.String()
			}
			want := evaluate(eval.NewContext([]byte(tt.root)))
			if got := evaluate(eval.NewContextForValidJSON([]byte(tt.root))); got != want {
				t.Errorf("%s at %s = %s, want %s as NewContext answers", tt.expr, tt.path, got, want)
			}
		})
	}
}
