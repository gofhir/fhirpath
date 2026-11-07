package conformance

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r4b/v2"
	"github.com/gofhir/models/r5/v2"
)

// Plan 023 (#265): = y ~ entre dos objetos responden igual en ambos
// sentidos, y comparan como elementos lo que la navegación cuenta como elementos.

var symmetryModels = []struct {
	name  string
	model fhirpath.Model
}{{"r4", r4.FHIRPathModel()}, {"r4b", r4b.FHIRPathModel()}, {"r5", r5.FHIRPathModel()}}

func evaluateWith(t *testing.T, model fhirpath.Model, resource, expr string) string {
	t.Helper()
	result, err := fhirpath.MustCompile(expr).EvaluateWithOptions([]byte(resource), fhirpath.WithModel(model))
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	return result.String()
}

// Pares de extensión, pues una Extension guarda cualquier propiedad hija que lee la
// comparación, escritos con sus miembros en ambos órdenes. Los valores esperados
// vienen del plan: un elemento escrito como {} es un item, así que no es
// nada, y un elemento junto a un valor no le agrega nada.
func TestObjectsOfAnyShapeCompareTheSameInBothDirections(t *testing.T) {
	tests := []struct {
		name, a, b string
		eq, equiv  bool
	}{
		{"an element that holds no value against nothing", `{"url":"u","_a":{}}`, `{"url":"u"}`, false, false},
		{"an element that holds no value beside a value", `{"url":"u","a":"c","_a":{}}`, `{"url":"u","a":"c"}`, true, true},
		{"an array of one such element against the element", `{"url":"u","_a":[{}]}`, `{"url":"u","_a":{}}`, true, true},
		{"a null against an element that holds no value", `{"url":"u","a":null}`, `{"url":"u","_a":{}}`, false, false},
		{"a member _ against one named empty and _", `{"url":"u","_":{}}`, `{"url":"u","":"x","_":{}}`, false, false},
		{"the same beside an element", `{"url":"u","_a":{},"_":{}}`, `{"url":"u","_a":{},"_":{},"":"x"}`, false, false},
		{"an element of an element against the element", `{"url":"u","__a":{}}`, `{"url":"u","_a":{}}`, false, false},
		{"an element of an element against nothing", `{"url":"u","__a":{}}`, `{"url":"u"}`, false, false},
		{"an element and the element of an element against the element", `{"url":"u","_a":{},"__a":{}}`, `{"url":"u","_a":{}}`, false, false},
		{"an element with content against nothing", `{"url":"u","_a":{"id":"e"}}`, `{"url":"u"}`, false, false},
		{"an element in the second place against nothing", `{"url":"u","_a":[null,{}]}`, `{"url":"u"}`, false, false},
		{"one element against itself", `{"url":"u","_a":{}}`, `{"url":"u","_a":{}}`, true, true},
	}
	reverse := func(text string) string {
		inner := strings.TrimSuffix(strings.TrimPrefix(text, "{"), "}")
		var members []string
		depth, start := 0, 0
		for i, c := range inner {
			switch c {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			case ',':
				if depth == 0 {
					members = append(members, inner[start:i])
					start = i + 1
				}
			}
		}
		members = append(members, inner[start:])
		for i, j := 0, len(members)-1; i < j; i, j = i+1, j-1 {
			members[i], members[j] = members[j], members[i]
		}
		return "{" + strings.Join(members, ",") + "}"
	}

	for _, model := range symmetryModels {
		for _, tt := range tests {
			for _, form := range []struct{ name, a, b string }{
				{"as written", tt.a, tt.b},
				{"reversed", reverse(tt.a), reverse(tt.b)},
			} {
				t.Run(model.name+"/"+tt.name+"/"+form.name, func(t *testing.T) {
					resource := `{"resourceType":"Basic","extension":[` + form.a + `,` + form.b + `]}`
					for _, op := range []struct {
						sign string
						want bool
					}{{"=", tt.eq}, {"~", tt.equiv}} {
						for _, expr := range []string{"extension[0] " + op.sign + " extension[1]", "extension[1] " + op.sign + " extension[0]"} {
							if got, want := evaluateWith(t, model.model, resource, expr), fmt.Sprintf("[%v]", op.want); got != want {
								t.Errorf("%s over %s = %s, want %s", expr, resource, got, want)
							}
						}
					}
				})
			}
		}
	}
}

// Seis codings, cuyas clases por = y por ~ fija el plan: [0] y [1] son
// una sola, pues un elemento sin valor junto a un valor no agrega nada; [2] es
// un code sin valor, [3] no tiene code, [4] es un code con un id, [5] es un
// display sin valor.
func TestCodingsWithElementsFallIntoFiveClasses(t *testing.T) {
	const resource = `{"resourceType":"Basic","code":{"coding":[` +
		`{"system":"s","code":"c"},` +
		`{"system":"s","code":"c","_code":{}},` +
		`{"system":"s","_code":{}},` +
		`{"system":"s"},` +
		`{"system":"s","code":"c","_code":{"id":"e"}},` +
		`{"system":"s","_display":{}}]}}`
	class := []int{0, 0, 1, 2, 3, 4}

	for _, model := range symmetryModels {
		t.Run(model.name, func(t *testing.T) {
			if got := evaluateWith(t, model.model, resource, "code.coding.distinct().count()"); got != "[5]" {
				t.Errorf("distinct().count() = %s, want [5]", got)
			}
			for _, op := range []string{"=", "~"} {
				for i := range class {
					for j := range class {
						expr := fmt.Sprintf("code.coding[%d] %s code.coding[%d]", i, op, j)
						if got, want := evaluateWith(t, model.model, resource, expr), fmt.Sprintf("[%v]", class[i] == class[j]); got != want {
							t.Errorf("%s = %s, want %s", expr, got, want)
						}
					}
				}
			}
		})
	}
}

// Un item que solo tiene un elemento es un item del nombre, que = y ~
// comparan como lo cuenta la navegación: las formas de la tabla del plan.
func TestAnElementAloneIsAnItemWhereNavigationCountsIt(t *testing.T) {
	tests := []struct {
		name  string
		count int
	}{
		{`{"_given":[{}]}`, 1},
		{`{"given":[null],"_given":[{}]}`, 1},
		{`{"given":["x",null],"_given":[null,{}]}`, 2},
		{`{"given":[null,"x"],"_given":[{},null]}`, 2},
		{`{"_given":[{"id":"e"}]}`, 1},
		{`{"given":["x"],"_given":[{}]}`, 1},
		{`{"_given":{}}`, 1},
		{`{"_given":[{},{}]}`, 2},
		{`{"given":[null,null],"_given":[{},null]}`, 1},
	}
	for _, model := range symmetryModels {
		for _, tt := range tests {
			t.Run(model.name+"/"+tt.name, func(t *testing.T) {
				resource := `{"resourceType":"Patient","name":[` + tt.name + `]}`
				if got, want := evaluateWith(t, model.model, resource, "Patient.name[0].given.count()"), fmt.Sprintf("[%d]", tt.count); got != want {
					t.Errorf("given.count() = %s, want %s", got, want)
				}
				// y un item con solo un elemento es igual a sí mismo, y
				// distinto de nada
				if tt.name == `{"_given":[{}]}` {
					for _, expr := range []string{"Patient.name[0] = Patient.name[0]", "Patient.name[0] ~ Patient.name[0]"} {
						if got := evaluateWith(t, model.model, resource, expr); got != "[true]" {
							t.Errorf("%s = %s, want [true]", expr, got)
						}
					}
				}
			})
		}
	}
}
