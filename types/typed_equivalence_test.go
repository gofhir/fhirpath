package types

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// Plan 025: EquivalentIn compara dos objetos leyendo cada hijo como lo tipa el
// modelo, igual que la navegación. La API todavía no existe: este archivo no
// compila en main hasta que se agregue types.EquivalentIn. Los valores
// esperados vienen de N1 §6.1.3 ("For complex types, equivalence requires all
// child properties to be equivalent, recursively") y del plan 025 §2; donde se
// compara con la navegación, el oráculo es TypedChildren con el mismo resolutor,
// nunca una respuesta del motor escrita a mano.

// pathTypes es un resolutor mínimo: solo TypeOf, un mapa de ruta a tipo.
type pathTypes map[string]string

func (p pathTypes) TypeOf(path string) string { return p[path] }

// fullTypes agrega los métodos opcionales que la navegación detecta:
// ChoiceTypes(path) y ResolvePath(path).
type fullTypes struct {
	pathTypes
	choices  map[string][]string
	resolved map[string]string // ruta alcanzada -> ruta que define el modelo
}

func (f fullTypes) ChoiceTypes(path string) []string { return f.choices[path] }

func (f fullTypes) ResolvePath(path string) string {
	if r, ok := f.resolved[path]; ok {
		return r
	}
	return path
}

// navigationOracle compara dos objetos con lo que la navegación les da:
// TypedChildren con el mismo resolutor, hijo por hijo, recursivamente. Sirve
// para objetos sin repeticiones desordenadas; los repetidos se parean por orden.
func navigationOracle(a, b *ObjectValue, pa, pb string, res ElementTypeResolver) bool {
	ca, cb := childrenByName(a.TypedChildren(pa, res)), childrenByName(b.TypedChildren(pb, res))
	if len(ca) != len(cb) {
		return false
	}
	for name, xs := range ca {
		ys, ok := cb[name]
		if !ok || len(xs) != len(ys) {
			return false
		}
		for i := range xs {
			oa, aIsObject := xs[i].Value.(*ObjectValue)
			ob, bIsObject := ys[i].Value.(*ObjectValue)
			switch {
			case aIsObject && bIsObject:
				if !navigationOracle(oa, ob, xs[i].Path, ys[i].Path, res) {
					return false
				}
			case aIsObject != bIsObject:
				return false
			default:
				if !xs[i].Value.Equivalent(ys[i].Value) {
					return false
				}
			}
		}
	}
	return true
}

func childrenByName(children []TypedChild) map[string][]TypedChild {
	byName := map[string][]TypedChild{}
	for _, c := range children {
		name := c.Path[strings.LastIndex(c.Path, ".")+1:]
		byName[name] = append(byName[name], c)
	}
	return byName
}

func obj(s string) *ObjectValue { return NewObjectValue([]byte(s)) }

// Texto idéntico bajo tipos distintos: el mismo "2020" es string a un lado y
// date al otro, y '2020' ~ @2020 es falso. Ninguna vía rápida por bytes puede
// responder antes de tipar (plan §3, A2).
func TestEquivalentInIdenticalBytesUnderDifferentTypes(t *testing.T) {
	res := pathTypes{
		"L.x": "string", "R.x": "date",
		"L.n": "TL", "R.n": "TR", "TL.x": "string", "TR.x": "date",
		"L.v": "string", "R.v": "date",
	}
	tests := []struct {
		name         string
		a, b         string
		pathA, pathB string
		want         bool
	}{
		{"root level, string against date", `{"x":"2020"}`, `{"x":"2020"}`, "L", "R", false},
		{"root level, the same type", `{"x":"2020"}`, `{"x":"2020"}`, "L", "L", true},
		{"root level, date on both sides", `{"x":"2020"}`, `{"x":"2020"}`, "R", "R", true},
		{"nested object", `{"n":{"x":"2020"}}`, `{"n":{"x":"2020"}}`, "L", "R", false},
		{"nested object, the same type", `{"n":{"x":"2020"}}`, `{"n":{"x":"2020"}}`, "R", "R", true},
		{"inside an array", `{"n":[{"x":"2020"},{"x":"2021"}]}`, `{"n":[{"x":"2020"},{"x":"2021"}]}`, "L", "R", false},
		{"inside an array, the same type", `{"n":[{"x":"2020"},{"x":"2021"}]}`, `{"n":[{"x":"2020"},{"x":"2021"}]}`, "L", "L", true},
		{"element pairing, x with _x", `{"v":"2020","_v":{"id":"i"}}`, `{"v":"2020","_v":{"id":"i"}}`, "L", "R", false},
		{"element pairing, the same type", `{"v":"2020","_v":{"id":"i"}}`, `{"v":"2020","_v":{"id":"i"}}`, "L", "L", true},
		{"a string array, string against date", `{"x":["2020"]}`, `{"x":["2020"]}`, "L", "R", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EquivalentIn(obj(tt.a), obj(tt.b), tt.pathA, tt.pathB, res); got != tt.want {
				t.Errorf("EquivalentIn(%s at %s, %s at %s) = %v, want %v", tt.a, tt.pathA, tt.b, tt.pathB, got, tt.want)
			}
			if got := EquivalentIn(obj(tt.b), obj(tt.a), tt.pathB, tt.pathA, res); got != tt.want {
				t.Errorf("swapped: = %v, want %v", got, tt.want)
			}
		})
	}
}

// Precedencia de la navegación (plan §2, B1/B2): un objeto de tipo
// BackboneElement tipa a sus hijos primero por la ruta alcanzada; cualquier
// otro, primero por su tipo (Type().name) y luego por la ruta.
func TestEquivalentInBackbonePrecedence(t *testing.T) {
	res := pathTypes{
		"L.x":               "string", // ruta alcanzada del backbone: gana a su tipo
		"BackboneElement.x": "date",   // el tipo del backbone: pierde ante la ruta
		"Other.x":           "string", // el tipo del otro objeto: gana a su ruta
		"R.x":               "date",   // la ruta del otro objeto: pierde ante su tipo
		"Plain.x":           "date",   // un objeto que no es backbone, con tipo
		"L2.x":              "string",
	}
	data := `{"x":"2020"}`
	backbone := func() *ObjectValue { return NewObjectValueWithType([]byte(data), "BackboneElement") }
	other := func() *ObjectValue { return NewObjectValueWithType([]byte(data), "Other") }
	plain := func() *ObjectValue { return NewObjectValueWithType([]byte(data), "Plain") }

	// backbone en L: ruta primero, string. other en R: tipo primero, string. Iguales.
	if !EquivalentIn(backbone(), other(), "L", "R", res) {
		t.Error("a backbone reached by L (string) against an Other typed string through its type: want true")
	}
	if !EquivalentIn(other(), backbone(), "R", "L", res) {
		t.Error("swapped: want true")
	}
	// plain: su tipo (date) gana a su ruta L2 (string); contra el backbone en L (string) difieren.
	if EquivalentIn(backbone(), plain(), "L", "L2", res) {
		t.Error("a backbone typed by its path (string) against a Plain typed by its type (date): want false")
	}
	if !EquivalentIn(plain(), plain(), "L", "R", res) {
		t.Error("two Plain objects are typed by their type on both sides, whatever their paths: want true")
	}
}

// Un resolutor mínimo (solo TypeOf) y uno completo: EquivalentIn debe tipar lo
// que la navegación tiparía con el mismo resolutor. El oráculo es TypedChildren.
func TestEquivalentInAgreesWithNavigationTypes(t *testing.T) {
	minimal := pathTypes{
		"L.a": "string", "R.a": "string",
		"L.x": "string", "R.x": "date",
		"E.value": "", "E.valueString": "string", "E.valueDate": "date",
		"T.item.text": "string", "U.item.text": "date",
	}
	full := fullTypes{
		pathTypes: pathTypes{
			"L.a": "string", "R.a": "string",
			"L.x": "string", "R.x": "date",
			"E.valueString": "string", "E.valueDate": "date",
			"T.item": "BackboneElement", "U.item": "BackboneElement",
			"T.item.text": "string", "U.item.text": "date",
		},
		choices:  map[string][]string{"E.value": {"string", "date"}},
		resolved: map[string]string{"T.item.item": "T.item", "U.item.item": "U.item"},
	}
	tests := []struct {
		name         string
		a, b         string
		pathA, pathB string
	}{
		{"a choice name with no suffix", `{"a":"x","value":"2020"}`, `{"a":"x"}`, "E", "E"},
		{"a choice key with its suffix", `{"valueString":"2020"}`, `{"valueString":"2020"}`, "E", "E"},
		{"a choice with different suffixes", `{"valueString":"2020"}`, `{"valueDate":"2020"}`, "E", "E"},
		{"a recursive backbone", `{"item":{"text":"2020"}}`, `{"item":{"text":"2020"}}`, "T.item", "U.item"},
		{"a recursive backbone, the same side", `{"item":{"text":"2020"}}`, `{"item":{"text":"2020"}}`, "T.item", "T.item"},
		{"plain children", `{"x":"2020","a":"y"}`, `{"x":"2020","a":"y"}`, "L", "R"},
	}
	for _, resolver := range []struct {
		name string
		res  ElementTypeResolver
	}{{"minimal", minimal}, {"full", full}} {
		for _, tt := range tests {
			t.Run(resolver.name+"/"+tt.name, func(t *testing.T) {
				a, b := obj(tt.a), obj(tt.b)
				want := navigationOracle(a, b, tt.pathA, tt.pathB, resolver.res)
				if got := EquivalentIn(a, b, tt.pathA, tt.pathB, resolver.res); got != want {
					t.Errorf("EquivalentIn = %v, navigation types (TypedChildren) give %v", got, want)
				}
			})
		}
	}

	// La diferencia entre los dos resolutores existe: el mínimo no conoce la
	// elección, el completo sí (si no, la prueba no distinguiría nada).
	a, b := obj(`{"a":"x","value":"2020"}`), obj(`{"a":"x"}`)
	if navigationOracle(a, b, "E", "E", minimal) == navigationOracle(a, b, "E", "E", full) {
		t.Error("fixture: the minimal and the full resolver must type `value` differently")
	}
	ra, rb := obj(`{"item":{"text":"2020"}}`), obj(`{"item":{"text":"2020"}}`)
	if navigationOracle(ra, rb, "T.item", "U.item", minimal) == navigationOracle(ra, rb, "T.item", "U.item", full) {
		t.Error("fixture: ResolvePath must change how the recursive path is typed")
	}
}

// Un hijo declarado decimal con un número fuera de la gramática no se lee: en
// ese lado está ausente, como lo omite la navegación (plan §2, B4).
func TestEquivalentInAChildThatFailsToDecodeIsAbsent(t *testing.T) {
	res := pathTypes{"L.a": "string", "L.d": "decimal"}
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"absent on one side", `{"a":"x","d":01}`, `{"a":"x"}`, true},
		{"absent on the other side", `{"a":"x"}`, `{"a":"x","d":01}`, true},
		{"absent on both sides, written differently", `{"a":"x","d":01}`, `{"a":"x","d":02}`, true},
		{"a decimal that reads, against none", `{"a":"x","d":1.0}`, `{"a":"x"}`, false},
		{"decimals that read", `{"a":"x","d":1.0}`, `{"a":"x","d":1}`, true},
		{"another child differs", `{"a":"x","d":01}`, `{"a":"y"}`, false},
		// Con el elemento al lado: el valor que no se lee falta, y el ítem es
		// el elemento solo, como lo cuenta la navegación
		{"absent beside its element", `{"a":"x","d":01,"_d":{"id":"i"}}`, `{"a":"x","_d":{"id":"i"}}`, true},
		{"absent beside its element, against a value", `{"a":"x","d":01,"_d":{"id":"i"}}`, `{"a":"x","d":1,"_d":{"id":"i"}}`, false},
		// Un número de la gramática JSON que el decimal no lee por su
		// exponente: la navegación tampoco lo lee (FP-6)
		{"a decimal whose exponent does not read", `{"a":"x","d":1e2147483648}`, `{"a":"x"}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EquivalentIn(obj(tt.a), obj(tt.b), "L", "L", res); got != tt.want {
				t.Errorf("EquivalentIn(%s, %s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
			if got := EquivalentIn(obj(tt.b), obj(tt.a), "L", "L", res); got != tt.want {
				t.Errorf("swapped = %v, want %v", got, tt.want)
			}
		})
	}
}

// Reflexividad y simetría sobre un conjunto pequeño generado, con un resolutor
// falso que tipa las mismas claves distinto en cada ruta.
func TestEquivalentInReflexiveAndSymmetric(t *testing.T) {
	res := pathTypes{
		"P.a": "string", "Q.a": "date",
		"P.b": "date", "Q.b": "dateTime",
		"P.c": "dateTime", "Q.c": "string",
		"P.d": "decimal", "Q.d": "decimal",
		"P.e": "time", "Q.e": "string",
		"P.n": "Thing", "Q.n": "Thing",
		"Thing.x": "string", "Thing.y": "date",
	}
	values := []string{
		`"2020"`, `"2020-01-01"`, `"2020-01-01T10:00:00Z"`, `"2020-01-01T12:00:00+02:00"`,
		`"2020-01-01T10:00:00z"`, `"10:00:00"`, `"10:00:00.000"`, `"x"`, `"X"`, `1`, `1.0`, `01`,
	}
	rng := rand.New(rand.NewSource(25))
	generated := make([]*ObjectValue, 0, 40)
	for len(generated) < 40 {
		var parts []string
		for _, key := range []string{"a", "b", "c", "d", "e"} {
			if rng.Intn(2) == 0 {
				parts = append(parts, fmt.Sprintf(`%q:%s`, key, values[rng.Intn(len(values))]))
			}
		}
		if rng.Intn(3) == 0 {
			parts = append(parts, fmt.Sprintf(`"n":{"x":%s,"y":%s}`, values[rng.Intn(len(values))], values[rng.Intn(len(values))]))
		}
		if rng.Intn(4) == 0 {
			parts = append(parts, fmt.Sprintf(`"_a":{"id":"i%d"}`, rng.Intn(2)))
		}
		generated = append(generated, obj("{"+strings.Join(parts, ",")+"}"))
	}
	paths := [][2]string{{"P", "P"}, {"P", "Q"}, {"Q", "Q"}, {"Q", "P"}}
	for _, a := range generated {
		for _, p := range []string{"P", "Q"} {
			if !EquivalentIn(a, a, p, p, res) {
				t.Fatalf("a ~ a is false at %s for %s", p, a.data)
			}
		}
	}
	for _, a := range generated {
		for _, b := range generated {
			for _, p := range paths {
				ab := EquivalentIn(a, b, p[0], p[1], res)
				ba := EquivalentIn(b, a, p[1], p[0], res)
				if ab != ba {
					t.Fatalf("asymmetric at (%s,%s): %s ~ %s = %v, swapped %v", p[0], p[1], a.data, b.data, ab, ba)
				}
			}
		}
	}
}
