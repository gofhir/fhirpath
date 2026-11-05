package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/models/r4/v2"
	"github.com/gofhir/models/r4b/v2"
	"github.com/gofhir/models/r5/v2"
)

// Dos component de Observation cuyos hijos fecha/hora son el mismo instante
// escrito de otra forma (otro offset, ".000") no son iguales con `=`: el motor
// compara cada hijo fecha/hora como está escrito, igual que fhirpath.js 5.2.0 y
// el motor Java de HL7 6.10.2. Es una divergencia tomada a propósito del texto
// ("For complex types, equality requires all child properties to be equal,
// recursively", FHIRPath N1 §6.1.1), porque esas dos referencias responden
// `children() = children()` verdadero con los padres distintos. Ver
// CONFORMANCE.md, "A divergence taken on purpose: a complex value's date/time
// children compare as written", y el plan 021.
//
// Los valores esperados son los de las referencias, no los de este motor: cada
// fila de abajo es una respuesta en la que las tres coinciden (filas 33 a 94 de
// difftest/batteries/complex-temporal-equality/battery.tsv, R4 y R5).
//
// R4B está extrapolado: el corredor de la batería no lo ejecuta, así que sus
// filas reusan los valores de R4 y R5. Cada elemento que usan los archivos tiene
// el mismo tipo y cardinalidad en hl7.fhir.r4.core 4.0.1, r4b.core 4.3.0 y
// r5.core 5.0.0, y el motor aplica un solo texto FHIRPath a toda versión
// (plan 021, §3 punto 2).
func TestComplexEqualityComparesDateChildrenAsWritten(t *testing.T) {
	dir := filepath.Join("..", "difftest", "batteries", "complex-temporal-equality")
	files := map[string][]byte{}
	for _, name := range []string{"valid.json", "fallback.json"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = data
	}

	models := []struct {
		name  string
		model fhirpath.Model
	}{
		{"R4", r4.FHIRPathModel()},
		{"R4B (extrapolated)", r4b.FHIRPathModel()},
		{"R5", r5.FHIRPathModel()},
	}

	cases := []struct{ file, expr, want string }{

		// valid.json: valores válidos de Observation; component 0-8, ver el archivo
		// N1 §6.1.1; fila 33 (R4) y 34 (R5) de la batería
		{"valid.json", "Observation.component[0].value = Observation.component[1].value", "[true]"},
		// N1 §6.1.1; fila 35 (R4) y 36 (R5) de la batería
		{"valid.json", "Observation.component[0].value = Observation.component[2].value", "[true]"},
		// N1 §6.1.1; fila 37 (R4) y 38 (R5) de la batería
		{"valid.json", "Observation.component[0].children() = Observation.component[1].children()", "[true]"},
		// N1 §6.1.1; fila 39 (R4) y 40 (R5) de la batería
		{"valid.json", "Observation.component[0].children() = Observation.component[2].children()", "[true]"},
		// N1 §6.1.1; fila 41 (R4) y 42 (R5) de la batería
		{"valid.json", "Observation.component[0] = Observation.component[1]", "[false]"},
		// N1 §6.1.1; fila 43 (R4) y 44 (R5) de la batería
		{"valid.json", "Observation.component[0] = Observation.component[3]", "[false]"},
		// N1 §6.1.1; fila 45 (R4) y 46 (R5) de la batería
		{"valid.json", "Observation.component[1] = Observation.component[3]", "[true]"},
		// N1 §6.1.1; fila 47 (R4) y 48 (R5) de la batería
		{"valid.json", "Observation.component[0] = Observation.component[4]", "[false]"},
		// N1 §6.1.1; fila 49 (R4) y 50 (R5) de la batería
		{"valid.json", "Observation.component[5] = Observation.component[6]", "[false]"},
		// N1 §6.1.1; fila 51 (R4) y 52 (R5) de la batería
		{"valid.json", "Observation.component[5].value = Observation.component[6].value", "[false]"},
		// N1 §6.1.1; fila 53 (R4) y 54 (R5) de la batería
		{"valid.json", "Observation.component[7] = Observation.component[8]", "[false]"},
		// N1 §6.1.1; fila 55 (R4) y 56 (R5) de la batería
		{"valid.json", "Observation.issued = Observation.effective.start", "[true]"},
		// N1 §6.1.1; fila 57 (R4) y 58 (R5) de la batería
		{"valid.json", "Observation.component.take(3).distinct().count()", "[3]"},
		// N1 §6.1.1; fila 59 (R4) y 60 (R5) de la batería
		{"valid.json", "Observation.component.take(3).isDistinct()", "[true]"},
		// N1 §6.1.1; fila 61 (R4) y 62 (R5) de la batería
		{"valid.json", "Observation.component.take(1).intersect(Observation.component.skip(1).take(1)).count()", "[0]"},
		// N1 §6.1.1; fila 63 (R4) y 64 (R5) de la batería
		{"valid.json", "Observation.component.take(2).exclude(Observation.component.take(1)).count()", "[1]"},
		// N1 §6.1.1; fila 65 (R4) y 66 (R5) de la batería
		{"valid.json", "Observation.component.take(1).subsetOf(Observation.component.skip(1).take(1))", "[false]"},
		// N1 §6.1.1; fila 67 (R4) y 68 (R5) de la batería
		{"valid.json", "Observation.component.skip(1).take(1).supersetOf(Observation.component.take(1))", "[false]"},
		// N1 §6.1.1; fila 69 (R4) y 70 (R5) de la batería
		{"valid.json", "Observation.component.take(1) contains Observation.component[2]", "[false]"},
		// N1 §6.1.1; fila 71 (R4) y 72 (R5) de la batería
		{"valid.json", "Observation.component[2] in Observation.component.take(1)", "[false]"},
		// N1 §6.1.1; fila 73 (R4) y 74 (R5) de la batería
		{"valid.json", "(Observation.component.take(1) | Observation.component.skip(3).take(1)).count()", "[2]"},

		// fallback.json: F1 = component 0, 1 y 7; F2 = 2 y 3; F3 = 4, 5 y 6
		// N1 §6.1.1; fila 75 (R4) y 76 (R5) de la batería; F1: llaves externas reordenadas, extension [u, v] contra [v, u]
		{"fallback.json", "Observation.component[0] = Observation.component[1]", "[false]"},
		// N1 §6.1.1; fila 77 (R4) y 78 (R5) de la batería; F1, al revés
		{"fallback.json", "Observation.component[1] = Observation.component[0]", "[false]"},
		// N1 §6.1.1; fila 79 (R4) y 80 (R5) de la batería; F1: el 7 repite las extension del 0 en orden [u, v]; las del 1 van [v, u]
		{"fallback.json", "Observation.component[1] = Observation.component[7]", "[false]"},
		// N1 §6.1.1; fila 81 (R4) y 82 (R5) de la batería; F2: llaves reordenadas, id y code iguales primero, luego valueDateTime en otro offset
		{"fallback.json", "Observation.component[2] = Observation.component[3]", "[false]"},
		// N1 §6.1.1; fila 83 (R4) y 84 (R5) de la batería; F2, al revés
		{"fallback.json", "Observation.component[3] = Observation.component[2]", "[false]"},
		// N1 §6.1.1; fila 85 (R4) y 86 (R5) de la batería; F3: code escrito "\u0061" contra "a", antes de una fecha en otro offset
		{"fallback.json", "Observation.component[4] = Observation.component[5]", "[false]"},
		// N1 §6.1.1; fila 87 (R4) y 88 (R5) de la batería; F3: code "\u0061" contra "a" y la fecha con el mismo texto; igual
		{"fallback.json", "Observation.component[4] = Observation.component[6]", "[true]"},
		// N1 §6.1.1; fila 89 (R4) y 90 (R5) de la batería; F3, al revés
		{"fallback.json", "Observation.component[6] = Observation.component[4]", "[true]"},
		// N1 §6.1.1; fila 91 (R4) y 92 (R5) de la batería; F2 con |: dos component
		{"fallback.json", "(Observation.component[2] | Observation.component[3]).count()", "[2]"},
		// N1 §6.1.1; fila 93 (R4) y 94 (R5) de la batería; F3 con |: el mismo texto se une en uno
		{"fallback.json", "(Observation.component[4] | Observation.component[6]).count()", "[1]"},

		// Ronda 1 del review: !=, union() y cada operación en ambas direcciones;
		// las últimas siete, con los component 1 y 3 (llaves reordenadas, mismo texto), son los casos de control positivo
		// N1 §6.1.1; fila 95 (R4) y 96 (R5) de la batería
		{"valid.json", "Observation.component[0] != Observation.component[1]", "[true]"},
		// N1 §6.1.1; fila 97 (R4) y 98 (R5) de la batería
		{"valid.json", "Observation.component[1] != Observation.component[0]", "[true]"},
		// N1 §6.1.1; fila 99 (R4) y 100 (R5) de la batería
		{"valid.json", "Observation.component[1] != Observation.component[3]", "[false]"},
		// N1 §6.1.1; fila 101 (R4) y 102 (R5) de la batería
		{"valid.json", "Observation.component.take(1).union(Observation.component.skip(3).take(1)).count()", "[2]"},
		// N1 §6.1.1; fila 103 (R4) y 104 (R5) de la batería
		{"valid.json", "Observation.component.skip(3).take(1).union(Observation.component.take(1)).count()", "[2]"},
		// N1 §6.1.1; fila 105 (R4) y 106 (R5) de la batería
		{"valid.json", "(Observation.component.skip(3).take(1) | Observation.component.take(1)).count()", "[2]"},
		// N1 §6.1.1; fila 107 (R4) y 108 (R5) de la batería
		{"valid.json", "Observation.component.skip(1).take(1).intersect(Observation.component.take(1)).count()", "[0]"},
		// N1 §6.1.1; fila 109 (R4) y 110 (R5) de la batería
		{"valid.json", "Observation.component.skip(1).take(1).exclude(Observation.component.take(1)).count()", "[1]"},
		// N1 §6.1.1; fila 137 (R4) y 138 (R5) de la batería; exclude en la otra dirección (ronda 2 del review)
		{"valid.json", "Observation.component.take(1).exclude(Observation.component.skip(1).take(1)).count()", "[1]"},
		// N1 §6.1.1; fila 111 (R4) y 112 (R5) de la batería
		{"valid.json", "Observation.component.skip(1).take(1).subsetOf(Observation.component.take(1))", "[false]"},
		// N1 §6.1.1; fila 113 (R4) y 114 (R5) de la batería
		{"valid.json", "Observation.component.take(1).supersetOf(Observation.component.skip(1).take(1))", "[false]"},
		// N1 §6.1.1; fila 115 (R4) y 116 (R5) de la batería
		{"valid.json", "Observation.component.skip(2).take(1) contains Observation.component[0]", "[false]"},
		// N1 §6.1.1; fila 117 (R4) y 118 (R5) de la batería
		{"valid.json", "Observation.component[0] in Observation.component.skip(2).take(1)", "[false]"},
		// N1 §6.1.1; fila 119 (R4) y 120 (R5) de la batería
		{"valid.json", "Observation.component[1].combine(Observation.component[3]).distinct().count()", "[1]"},
		// N1 §6.1.1; fila 121 (R4) y 122 (R5) de la batería
		{"valid.json", "Observation.component[1].combine(Observation.component[3]).isDistinct()", "[false]"},
		// N1 §6.1.1; fila 123 (R4) y 124 (R5) de la batería
		{"valid.json", "Observation.component.skip(1).take(1).intersect(Observation.component.skip(3).take(1)).count()", "[1]"},
		// N1 §6.1.1; fila 125 (R4) y 126 (R5) de la batería
		{"valid.json", "Observation.component.skip(3).take(1).exclude(Observation.component.skip(1).take(1)).count()", "[0]"},
		// N1 §6.1.1; fila 127 (R4) y 128 (R5) de la batería
		{"valid.json", "Observation.component.skip(1).take(1).subsetOf(Observation.component.skip(3).take(1))", "[true]"},
		// N1 §6.1.1; fila 129 (R4) y 130 (R5) de la batería
		{"valid.json", "Observation.component.skip(3).take(1).supersetOf(Observation.component.skip(1).take(1))", "[true]"},
		// N1 §6.1.1; fila 131 (R4) y 132 (R5) de la batería
		{"valid.json", "Observation.component.skip(1).take(1).union(Observation.component.skip(3).take(1)).count()", "[1]"},
		// N1 §6.1.1; fila 133 (R4) y 134 (R5) de la batería
		{"valid.json", "Observation.component[3] in Observation.component.skip(1).take(1)", "[true]"},
		// N1 §6.1.1; fila 135 (R4) y 136 (R5) de la batería
		{"valid.json", "Observation.component.skip(1).take(1) contains Observation.component[3]", "[true]"},
	}

	// Cada expresión se compila una vez y corre con los tres modelos.
	for _, tc := range cases {
		expr := fhirpath.MustCompile(tc.expr)
		for _, m := range models {
			t.Run(fmt.Sprintf("%s/%s/%s", m.name, tc.file, tc.expr), func(t *testing.T) {
				result, err := expr.EvaluateWithOptions(files[tc.file], fhirpath.WithModel(m.model))
				if err != nil {
					t.Fatalf("%s: unexpected error: %v", tc.expr, err)
				}
				items := make([]string, len(result))
				for i, v := range result {
					items[i] = v.String()
				}
				if got := "[" + strings.Join(items, " ") + "]"; got != tc.want {
					t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
				}
			})
		}
	}
}
