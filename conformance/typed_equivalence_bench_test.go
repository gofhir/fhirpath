package conformance

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r4b/v2"
	"github.com/gofhir/models/r5/v2"
)

// Plan 025: costo de `~` entre dos objetos con un modelo. Cuando la
// equivalencia tipe cada hijo por el modelo, como lo hace la navegación,
// estas cargas muestran si se construyen valores para hijos que nunca se
// leen. Se miden antes del cambio (línea base en main) y después.

// equivalenceWorkload es una carga: un recurso por versión y una expresión `~`
// entre objetos.
type equivalenceWorkload struct {
	name     string
	expr     string
	resource func(version string) string
}

// equivalenceVersions son las formas de cada carga; "none" es la ruta sin modelo.
var equivalenceVersions = []struct {
	name  string
	model fhirpath.Model // nil: sin modelo
}{
	{"R4", r4.FHIRPathModel()},
	{"R4B", r4b.FHIRPathModel()},
	{"R5", r5.FHIRPathModel()},
	{"none", nil},
}

// repeatJoin escribe n elementos f(0)..f(n-1) separados por coma, en el orden dado.
func repeatJoin(n int, reversed bool, f func(i int) string) string {
	parts := make([]string, n)
	for i := range parts {
		k := i
		if reversed {
			k = n - 1 - i
		}
		parts[i] = f(k)
	}
	return strings.Join(parts, ",")
}

// stringChildren escribe n hijos de texto p00.. y un último, p99, con el valor indicado.
func stringChildren(n int, last string) string {
	return repeatJoin(n, false, func(i int) string { return fmt.Sprintf(`"p%02d":"v%02d"`, i, i) }) + `,"p99":"` + last + `"`
}

var equivalenceWorkloads = []equivalenceWorkload{
	{"digits", "Observation.identifier[0] ~ Observation.identifier[1]", func(string) string {
		identifier := func(value string) string {
			return `{"id":"i","use":"official","type":{"text":"t"},"system":"urn:s","value":"` + value +
				`","period":{"start":"2020-01-01","end":"2020-12-31"},"assigner":{"display":"a","reference":"Organization/1"}}`
		}
		return `{"resourceType":"Observation","status":"final","code":{"text":"c"},"identifier":[` +
			identifier("2020-01-01T10:00:00+01:00") + `,` + identifier("2020-01-01T09:00:00Z") + `]}`
	}},
	{"integer64", "DocumentReference.content[0].attachment ~ DocumentReference.content[1].attachment", func(version string) string {
		first, second := `1`, `1` // en R4 y R4B size es un número
		if version == "R5" {
			first, second = `"+1"`, `"1"` // en R5 size es integer64: una cadena JSON
		}
		attachment := func(size string) string {
			return `{"attachment":{"contentType":"text/plain","size":` + size + `,"title":"t"}}`
		}
		return `{"resourceType":"DocumentReference","status":"current","content":[` + attachment(first) + `,` + attachment(second) + `]}`
	}},
	{"escaped", "Observation.identifier[0] ~ Observation.identifier[1]", func(string) string {
		return `{"resourceType":"Observation","status":"final","code":{"text":"c"},"identifier":[` +
			`{"system":"urn:s","value":"2020-01-01"},{"system":"urn:s","value":"2020-01-02"}]}`
	}},
	{"nested", "Observation.identifier[0] ~ Observation.identifier[1]", func(string) string {
		identifier := func(value string) string {
			return `{"value":"x","assigner":{"identifier":{"value":"y","assigner":{"identifier":{"value":"` + value + `"}}}}}`
		}
		return `{"resourceType":"Observation","status":"final","code":{"text":"c"},"identifier":[` +
			identifier("deep-a") + `,` + identifier("deep-b") + `]}`
	}},
	{"elements", "Patient.name[0] ~ Patient.name[1]", func(string) string {
		given := `"given":["a","b","c","d"]`
		elements := `"_given":[{"id":"g0"},{"extension":[{"url":"u","valueString":"x"}]},{"id":"g2"},null]`
		reversed := `"_given":[null,{"id":"g2"},{"extension":[{"url":"u","valueString":"x"}]},{"id":"g0"}]`
		return `{"resourceType":"Patient","name":[{` + elements + `,` + given + `,"family":"f"},` +
			`{"family":"f",` + reversed + `,"given":["d","c","b","a"]}]}`
	}},
	{"repeats", "Patient.name[0] ~ Patient.name[1]", func(string) string {
		givens := func(reversed bool) string {
			return `[` + repeatJoin(40, reversed, func(i int) string { return fmt.Sprintf(`"g%02d"`, i) }) + `]`
		}
		return `{"resourceType":"Patient","name":[{"family":"f","given":` + givens(false) + `},{"family":"f","given":` + givens(true) + `}]}`
	}},
	{"identical", "Patient.extension[0] ~ Patient.extension[1]", func(string) string {
		return `{"resourceType":"Patient","extension":[{"url":"u",` + stringChildren(40, "a") + `},{"url":"u",` + stringChildren(40, "b") + `}]}`
	}},
	{"lateElement", "Patient.extension[0] ~ Patient.extension[1]", func(string) string {
		children := repeatJoin(30, false, func(i int) string { return fmt.Sprintf(`"c%02d":"v%02d"`, i, i) })
		return `{"resourceType":"Patient","extension":[{"url":"u",` + children + `,"_x":{"id":"1"}},{"url":"u",` + children + `,"_x":{"id":"2"}}]}`
	}},
	{"reversedObjects", "Bundle.entry[0].resource.contact ~ Bundle.entry[1].resource.contact", func(string) string {
		patient := func(reversed bool) string {
			contacts := repeatJoin(40, reversed, func(i int) string {
				return fmt.Sprintf(`{"name":{"text":"c%02d"},"gender":"male"}`, i)
			})
			return `{"resource":{"resourceType":"Patient","contact":[` + contacts + `]}}`
		}
		return `{"resourceType":"Bundle","type":"collection","entry":[` + patient(false) + `,` + patient(true) + `]}`
	}},
	{"differentTypes", "Observation.identifier[0] ~ Observation.code.coding[0]", func(string) string {
		return `{"resourceType":"Observation","status":"final","identifier":[{"system":"urn:s","value":"v"}],` +
			`"code":{"coding":[{"system":"urn:s","code":"v"}]}}`
	}},
}

// evaluateEquivalence evalúa la carga con el modelo de la versión (o sin él).
func evaluateEquivalence(expr *fhirpath.Expression, data []byte, model fhirpath.Model) (types.Collection, error) {
	if model == nil {
		return expr.Evaluate(data)
	}
	return expr.EvaluateWithOptions(data, fhirpath.WithModel(model))
}

// BenchmarkTypedEquivalence compila una vez, lee el recurso una vez y evalúa
// una expresión `~` entre objetos por iteración.
func BenchmarkTypedEquivalence(b *testing.B) {
	for _, workload := range equivalenceWorkloads {
		for _, version := range equivalenceVersions {
			b.Run(workload.name+"/"+version.name, func(b *testing.B) {
				expr := fhirpath.MustCompile(workload.expr)
				data := []byte(workload.resource(version.name))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := evaluateEquivalence(expr, data, version.model); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// TestTypedEquivalenceWorkloadsAnswer comprueba que cada carga responde un
// único Boolean (ni vacío ni error); no afirma si es verdadero o falso.
func TestTypedEquivalenceWorkloadsAnswer(t *testing.T) {
	for _, workload := range equivalenceWorkloads {
		for _, version := range equivalenceVersions {
			t.Run(workload.name+"/"+version.name, func(t *testing.T) {
				expr := fhirpath.MustCompile(workload.expr)
				result, err := evaluateEquivalence(expr, []byte(workload.resource(version.name)), version.model)
				if err != nil {
					t.Fatal(err)
				}
				value, err := result.Single()
				if err != nil {
					t.Fatalf("expected one value, got %q: %v", result.String(), err)
				}
				if value.Type() != "Boolean" {
					t.Fatalf("expected a Boolean, got %s (%s)", value.Type(), value.String())
				}
			})
		}
	}
}
