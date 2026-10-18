package fhirpath_test

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// A resource names its own type. An element declared as the abstract Resource
// — Bundle.entry.resource, every contained — says only what it must derive
// from, which is why FHIR writes resourceType on every resource. With a model
// the declared type used to win: entry.resource was a Resource, is Patient was
// false, and bdl-11 failed on every document Bundle.
func TestAResourceTakesTheTypeItNames(t *testing.T) {
	model := &testModel{
		typeOf: map[string]string{
			"Bundle.type":           "code",
			"Bundle.entry":          "BackboneElement",
			"Bundle.entry.resource": "Resource",
			"Observation.contained": "Resource",
			"Observation.status":    "code",
			"Patient.id":            "http://hl7.org/fhirpath/System.String",
			"Patient.birthDate":     "date",
		},
		parentType: map[string]string{"Patient": "DomainResource", "DomainResource": "Resource"},
		resources:  map[string]bool{"Bundle": true, "Observation": true, "Patient": true, "Composition": true},
	}

	bundle := `{"resourceType":"Bundle","type":"document","entry":[{"resource":` +
		`{"resourceType":"Patient","id":"2020","birthDate":"2020-01-01"}}]}`
	observation := `{"resourceType":"Observation","status":"final",` +
		`"contained":[{"resourceType":"Patient","id":"2020","birthDate":"2020-01-01"}]}`

	tests := []struct {
		resource string
		expr     string
		want     string
	}{
		{bundle, "entry.resource.type().name", "[Patient]"},
		{bundle, "entry.resource is Patient", "[true]"},
		{bundle, "entry.resource is Resource", "[true]"},
		{bundle, "entry.resource.as(Patient).count()", "[1]"},
		{bundle, "entry.resource.ofType(Patient).count()", "[1]"},
		{bundle, "entry.resource.ofType(Patient).birthDate.type().name", "[date]"},
		{bundle, "entry.first().resource.is(Patient)", "[true]"},
		{observation, "contained.type().name", "[Patient]"},
		{observation, "contained.birthDate.type().name", "[date]"},
		{observation, "contained.descendants().where($this is FHIR.date).count()", "[1]"},
		// An id the model declares a System.String is a String, not the Date
		// "2020" would be guessed as, and its type is named as a System type.
		{observation, "contained.id.type().name", "[String]"},
		{observation, "contained.id.type().namespace", "[System]"},
		{observation, "contained.id = '2020'", "[true]"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			compiled := fhirpath.MustCompile(tt.expr)
			result, err := compiled.EvaluateWithOptions([]byte(tt.resource), fhirpath.WithModel(model))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
			}

			doc := fhirpath.MustNewDocument([]byte(tt.resource))
			for i := 0; i < 2; i++ {
				result, err = doc.EvaluateWithOptions(compiled, fhirpath.WithModel(model))
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s on a Document (pass %d) = %s, want %s", tt.expr, i+1, got, tt.want)
				}
			}
		})
	}
}

// A root read once and shared between evaluations does not write what it
// works out, so it worked its type out each time it was asked: for a Bundle
// whose resourceType comes after its entries, as Go's encoding/json writes a
// map, by reading past every entry. %resource is Bundle, asked once per entry,
// then cost the size of the Bundle per entry. A root read without a declared
// type has its type worked out when it is read, so per entry a Bundle of 4000
// costs about what one of 500 does, wherever its resourceType is. The ratio is
// of the best of several passes, so it holds on a slow or busy machine, and a
// margin of three keeps noise from failing it while catching a cost that
// grows with the Bundle.
func TestASharedRootIsTypedOnceWhereverItsResourceTypeIs(t *testing.T) {
	compiled := fhirpath.MustCompile("Bundle.entry.where(%resource is Bundle).count()")

	perEntry := func(n int, last bool) time.Duration {
		var bundle strings.Builder
		bundle.WriteString("{")
		if !last {
			bundle.WriteString(`"resourceType":"Bundle",`)
		}
		bundle.WriteString(`"entry":[`)
		for i := 0; i < n; i++ {
			if i > 0 {
				bundle.WriteByte(',')
			}
			fmt.Fprintf(&bundle, `{"fullUrl":"urn:uuid:%d","resource":{"resourceType":"Patient","id":"p%d"}}`, i, i)
		}
		bundle.WriteString("]")
		if last {
			bundle.WriteString(`,"resourceType":"Bundle"`)
		}
		bundle.WriteString("}")

		root, err := types.JSONToCollection([]byte(bundle.String()))
		if err != nil {
			t.Fatal(err)
		}

		best := time.Duration(math.MaxInt64)
		for range 10 {
			start := time.Now()
			result, err := compiled.EvaluateWithContext(eval.NewContextForRoot(root))
			elapsed := time.Since(start)
			if err != nil || result.String() != fmt.Sprintf("[%d]", n) {
				t.Fatal(err, result)
			}
			best = min(best, elapsed)
			if elapsed > 2*time.Second {
				break // already far past linear; no need to wait out the rest
			}
		}
		return best / time.Duration(n)
	}

	for _, last := range []bool{false, true} {
		small, large := perEntry(500, last), perEntry(4000, last)
		ratio := float64(large) / float64(max(small, 1))
		t.Logf("resourceType last=%v: %v per entry at 500, %v at 4000 (%.1fx)", last, small, large, ratio)
		if ratio > 3 {
			t.Errorf("resourceType last=%v: an entry of a Bundle of 4000 costs %.1fx what one of 500 does, want about the same", last, ratio)
		}
	}
}

// A root read without a declared type is typed when it is read, as Type()
// would have typed it, and one read under a declared type keeps that type.
func TestARootReadWithoutATypeIsTypedAsItWouldHaveBeen(t *testing.T) {
	for _, tt := range []struct {
		json, fhirType, want string
	}{
		{`{"resourceType":"Patient","id":"a"}`, "", "Patient"},
		{`{"id":"a","resourceType":"Patient"}`, "", "Patient"},
		{`{"value":5,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}`, "", "Quantity"},
		{`{"value":5,"unit":"mg"}`, "SimpleQuantity", "SimpleQuantity"},
		{`{"resourceType":"Patient"}`, "DomainResource", "Patient"},
	} {
		readers := map[string]func([]byte) (types.Collection, error){
			"JSONToCollection": func(data []byte) (types.Collection, error) {
				return types.JSONToCollectionWithType(data, tt.fhirType)
			},
			"ReadRoot": func(data []byte) (types.Collection, error) {
				return types.ReadRootWithType(data, tt.fhirType)
			},
		}
		for how, read := range readers {
			col, err := read([]byte(tt.json))
			if err != nil {
				t.Fatal(err)
			}
			if got := col[0].Type(); got != tt.want {
				t.Errorf("%s(%s, %q) is a %s, want %s", how, tt.json, tt.fhirType, got, tt.want)
			}
		}
	}
}

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
