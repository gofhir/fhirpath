package types

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/buger/jsonparser"
)

// Plan 023 (#265): comparar dos objetos con = o ~ no agrega un sentido propio.
// Los nombres de estas pruebas son los que encuentra el lector de un objeto: el
// nombre de un primitivo, su elemento _name, un elemento de un elemento __name, y
// las dos claves que ningún elemento FHIR tiene, "" y "_".

// reversedMembers escribe los miembros de primer nivel de un objeto en el orden
// opuesto.
func reversedMembers(text string) string {
	var members []string
	jsonparser.ObjectEach([]byte(text), func(key, value []byte, dataType jsonparser.ValueType, _ int) error {
		raw := string(value)
		if dataType == jsonparser.String {
			raw = `"` + raw + `"`
		}
		members = append(members, `"`+string(key)+`":`+raw)
		return nil
	})
	for i, j := 0, len(members)-1; i < j; i, j = i+1, j-1 {
		members[i], members[j] = members[j], members[i]
	}
	return "{" + strings.Join(members, ",") + "}"
}

// symmetryChildren son los valores que toma un miembro, los primeros los más
// reveladores: las formas del barrido del plan, un nombre de recurso sin valor, y las cantidades cuya
// comparación tiene un sentido propio.
var symmetryChildren = []string{
	`"x"`, `{}`, `{"id":"e"}`, `[{}]`, `null`, `[{"id":"e"},null]`, `["x"]`, `[null,"x"]`, `[]`,
	`{"resourceType":"X"}`, `1`, `{"value":1,"code":"1"}`, `{"value":1.5,"code":"Cel"}`, `{"value":274.6,"code":"K"}`,
}

// symmetryKeys son las claves de un objeto generado.
var symmetryKeys = []string{"a", "_a", "__a", "b", "", "_"}

// symmetricChildren deja fuera los valores hijos cuyas propias comparaciones no
// son simétricas, por = y por ~: un objeto compara a sus hijos con sus
// comparaciones, y así hereda esos sentidos, que casos propios
// fijan. Dice qué dejó fuera.
func symmetricChildren(t *testing.T) []string {
	t.Helper()
	var kept []string
	left := map[string][]string{}
	for _, v := range symmetryChildren {
		ok := true
		for _, w := range symmetryChildren {
			a, b := object(`{"k":`+v+`}`), object(`{"k":`+w+`}`)
			if a.Equal(b) != b.Equal(a) {
				ok = false
				left[v] = append(left[v], "= against "+w)
			}
			if a.Equivalent(b) != b.Equivalent(a) {
				ok = false
				left[v] = append(left[v], "~ against "+w)
			}
		}
		if ok {
			kept = append(kept, v)
		}
	}
	for v, why := range left {
		t.Logf("child %s left out: %s", v, strings.Join(why, ", "))
	}
	t.Logf("%d of %d child values kept", len(kept), len(symmetryChildren))
	return kept
}

// writeMembers escribe los miembros de un objeto, con las claves en el orden
// dado.
func writeMembers(keys, values []string) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%q:%s", k, values[i])
	}
	return "{" + strings.Join(parts, ",") + "}"
}

// symmetryObjects genera objetos a partir de las claves: todos los de un miembro con
// cualquier hijo, todos los de dos miembros con los primeros hijos, y una muestra
// sembrada de tres. Los pares de un barrido crecen con el cuadrado de los objetos.
func symmetryObjects(children []string, pairs int) []string {
	var out []string
	out = append(out, `{}`)
	for i, k := range symmetryKeys {
		for _, v := range children {
			out = append(out, writeMembers([]string{k}, []string{v}))
		}
		for _, k2 := range symmetryKeys[i+1:] {
			for _, v := range children[:min(pairs, len(children))] {
				for _, w := range children[:min(pairs, len(children))] {
					out = append(out, writeMembers([]string{k, k2}, []string{v, w}))
				}
			}
		}
	}
	rng := rand.New(rand.NewSource(23))
	for range 500 {
		perm := rng.Perm(len(symmetryKeys))[:3]
		keys := make([]string, 3)
		values := make([]string, 3)
		for i, p := range perm {
			keys[i], values[i] = symmetryKeys[p], children[rng.Intn(len(children))]
		}
		out = append(out, writeMembers(keys, values))
	}
	return out
}

// symmetryForm escribe el par n en una de cuatro formas: tal cual, el segundo
// invertido, ambos anidados un nivel, y ambos anidados e invertidos.
func symmetryForm(a, b string, n int) (first, second string) {
	switch n % 4 {
	case 1:
		return a, reversedMembers(b)
	case 2:
		return `{"n":` + a + `}`, `{"n":` + b + `}`
	case 3:
		return `{"n":` + reversedMembers(a) + `}`, `{"n":` + reversedMembers(b) + `}`
	}
	return a, b
}

// TestObjectEqualityIsSymmetric comprueba que comparar objetos no agrega un
// sentido propio: en cada par de los objetos generados, = y ~ dan la
// misma respuesta en ambos sentidos.
func TestObjectEqualityIsSymmetric(t *testing.T) {
	objects := symmetryObjects(symmetricChildren(t), 7)
	var pairs, equal, equivalent, violations int
	for i, a := range objects {
		for j := i; j < len(objects); j++ {
			x, y := symmetryForm(a, objects[j], i+j)
			l, r := object(x), object(y)
			eq, eqBack := l.Equal(r), r.Equal(l)
			equiv, equivBack := l.Equivalent(r), r.Equivalent(l)
			pairs++
			if eq {
				equal++
			}
			if equiv {
				equivalent++
			}
			if eq != eqBack || equiv != equivBack {
				violations++
				if violations <= 10 {
					t.Errorf("%s and %s: = %v and %v, ~ %v and %v", x, y, eq, eqBack, equiv, equivBack)
				}
			}
		}
	}
	t.Logf("%d objects, %d pairs, %d equal, %d equivalent, %d asymmetric", len(objects), pairs, equal, equivalent, violations)
	if violations > 0 {
		t.Errorf("%d asymmetric pairs", violations)
	}
	if equal < 1000 || equivalent < 1000 {
		t.Errorf("only %d equal and %d equivalent pairs: the sweep tests too little", equal, equivalent)
	}
}

// objectCase es un par de objetos y lo que responden = y ~, en ambos sentidos
// y con los miembros de cada uno en el orden escrito o invertido.
type objectCase struct {
	name, a, b       string
	eq, eqBack       bool
	equiv, equivBack bool
}

func both(name, a, b string, eq, equiv bool) objectCase {
	return objectCase{name: name, a: a, b: b, eq: eq, eqBack: eq, equiv: equiv, equivBack: equiv}
}

// objectCases toma sus valores esperados del texto del plan y de sus
// mediciones.
func objectCases() []objectCase {
	return []objectCase{
		// Lo que cambia
		both("an element that holds no value against nothing", `{"_a":{}}`, `{}`, false, false),
		both("an element that holds no value beside a value", `{"a":"c","_a":{}}`, `{"a":"c"}`, true, true),
		both("an array of one such element against the element", `{"_a":[{}]}`, `{"_a":{}}`, true, true),
		both("a null against an element that holds no value", `{"a":null}`, `{"_a":{}}`, false, false),
		both("a member _ against one named empty and _", `{"_":{}}`, `{"":"x","_":{}}`, false, false),
		both("the same beside an element", `{"_a":{},"_":{}}`, `{"_a":{},"_":{},"":"x"}`, false, false),
		both("an element of an element against the element", `{"__a":{}}`, `{"_a":{}}`, false, false),
		both("an element of an element against nothing", `{"__a":{}}`, `{}`, false, false),
		both("the element against nothing", `{"_a":{}}`, `{}`, false, false),
		// Lo que no debe cambiar
		both("an element with content against nothing", `{"_a":{"id":"e"}}`, `{}`, false, false),
		both("an element with content beside a value against the value", `{"a":"c","_a":{"id":"e"}}`, `{"a":"c"}`, false, false),
		both("an element that holds no value in the second place against nothing", `{"_a":[null,{}]}`, `{}`, false, false),
		both("the same against one value", `{"a":["x",null],"_a":[null,{}]}`, `{"a":["x"]}`, false, false),
		both("an element and the element of an element against the element", `{"_a":{},"__a":{}}`, `{"_a":{}}`, false, false),
		both("an element and the element of an element against itself", `{"_a":{},"__a":{}}`, `{"__a":{},"_a":{}}`, true, true),
		both("one element against one", `{"_a":{}}`, `{"_a":{}}`, true, true),
		both("an element of an element against itself", `{"__a":{}}`, `{"__a":{}}`, true, true),
		both("a member _ against itself", `{"_":{}}`, `{"_":{}}`, true, true),
		both("a member _ against an empty name", `{"":"x"}`, `{"_":{}}`, false, false),
		both("a member _ against nothing", `{"_":{}}`, `{}`, false, false),
		both("a value that holds no value against nothing", `{"a":{}}`, `{}`, false, false),
		both("a null against nothing", `{"a":null}`, `{}`, true, true),
		both("an empty array against nothing", `{"a":[]}`, `{}`, true, true),
		both("nulls beside an element, against the element", `{"a":[null,null],"_a":[{},null]}`, `{"_a":{}}`, true, true),
		both("one empty element against two", `{"_a":[{}]}`, `{"_a":[{},{}]}`, false, false),
		both("values in another order", `{"a":["x","y"]}`, `{"a":["y","x"]}`, false, true),
		both("elements in another order", `{"a":["x","y"],"_a":[{"id":"1"},null]}`, `{"a":["x","y"],"_a":[null,{"id":"1"}]}`, false, false),
		both("elements with another id", `{"_a":{"id":"e"}}`, `{"_a":{"id":"f"}}`, false, false),
		both("elements with the same id", `{"_a":{"id":"e"}}`, `{"_a":{"id":"e"}}`, true, true),
		both("an element that holds no value beside a value, another beside a null", `{"a":["x","y"],"_a":[{},{"id":"1"}]}`, `{"a":["x","y"],"_a":[null,{"id":"1"}]}`, true, true),
		both("an element before its value", `{"_a":{},"a":"c"}`, `{"a":"c"}`, true, true),
		both("another name beside an element", `{"b":"x","_a":{}}`, `{"b":"x"}`, false, false),
	}
}

func TestObjectEqualityCases(t *testing.T) {
	for _, tt := range objectCases() {
		t.Run(tt.name, func(t *testing.T) {
			for _, form := range []struct {
				name string
				a, b string
			}{
				{"as written", tt.a, tt.b},
				{"first reversed", reversedMembers(tt.a), tt.b},
				{"second reversed", tt.a, reversedMembers(tt.b)},
				{"both reversed", reversedMembers(tt.a), reversedMembers(tt.b)},
			} {
				l, r := object(form.a), object(form.b)
				if got := l.Equal(r); got != tt.eq {
					t.Errorf("%s: %s = %s is %v, want %v", form.name, form.a, form.b, got, tt.eq)
				}
				if got := r.Equal(l); got != tt.eqBack {
					t.Errorf("%s: %s = %s is %v, want %v", form.name, form.b, form.a, got, tt.eqBack)
				}
				if got := l.Equivalent(r); got != tt.equiv {
					t.Errorf("%s: %s ~ %s is %v, want %v", form.name, form.a, form.b, got, tt.equiv)
				}
				if got := r.Equivalent(l); got != tt.equivBack {
					t.Errorf("%s: %s ~ %s is %v, want %v", form.name, form.b, form.a, got, tt.equivBack)
				}
			}
		})
	}
}

// TestObjectEqualityKeepsTheDirectionsOfItsChildren fija los sentidos que los
// objetos heredan de la comparación de sus hijos, que el 023 no
// cambia: el desempate de cantidades que manda el texto, FP-21, FP-25 y FP-26.
func TestObjectEqualityKeepsTheDirectionsOfItsChildren(t *testing.T) {
	// "If the conversion factors are 1 (the units are equal), then choose the
	// unit of the operator's left argument"
	celsius, kelvin := object(`{"q":{"value":1.5,"code":"Cel"}}`), object(`{"q":{"value":274.6,"code":"K"}}`)
	if !celsius.Equivalent(kelvin) || kelvin.Equivalent(celsius) {
		t.Errorf("1.5 Cel ~ 274.6 K is %v and the reverse %v, want true and false", celsius.Equivalent(kelvin), kelvin.Equivalent(celsius))
	}
	// FP-25
	number, quantity := object(`{"q":1}`), object(`{"q":{"value":1,"code":"1"}}`)
	if !number.Equivalent(quantity) || quantity.Equivalent(number) {
		t.Errorf("{q:1} ~ {q:quantity} is %v and the reverse %v, want true and false", number.Equivalent(quantity), quantity.Equivalent(number))
	}
	// FP-21
	years, month := mustQuantity("0.0833333333333333 year"), mustQuantity("1 month")
	if !years.Equal(month) || month.Equal(years) {
		t.Errorf("0.0833333333333333 year = 1 month is %v and the reverse %v, want true and false", years.Equal(month), month.Equal(years))
	}
	// FP-26
	bel, one := mustQuantity("1 'B'"), mustQuantity("14 '1'")
	if !bel.Equivalent(one) || one.Equivalent(bel) {
		t.Errorf("1 'B' ~ 14 '1' is %v and the reverse %v, want true and false", bel.Equivalent(one), one.Equivalent(bel))
	}
}

// itemsOf cuenta los items de un nombre que = compara.
func itemsOf(data, name string) int {
	o, ok := parseJSONObject([]byte(data))
	if !ok {
		return -1
	}
	value, vok := o.find([]byte(name), len(o.children))
	element, eok := o.find([]byte("_"+name), len(o.children))
	return len(nameItems(value, vok, element, eok))
}

// TestEqualityComparesWhatNavigationCounts: en un objeto cuyas claves son nombres
// FHIR válidos, los items que = compara bajo un nombre son los que la navegación
// cuenta bajo él.
func TestEqualityComparesWhatNavigationCounts(t *testing.T) {
	// Un primitivo guarda escalares y un elemento guarda objetos, como
	// los escribe FHIR
	primitives := []string{`"x"`, `["x"]`, `[null,"x"]`, `["x","y"]`, `[null,null]`, `null`, `[]`}
	elements := []string{`{}`, `{"id":"e"}`, `[{"id":"e"},null]`, `[{}]`, `[{},null]`, `[null,{}]`, `[null,null]`, `null`, `[]`}
	keys := []string{"a", "_a", "b", "_b"}
	var objects []string
	var build func(from int, keys []string, values []string)
	build = func(from int, chosenKeys, chosen []string) {
		objects = append(objects, writeMembers(chosenKeys, chosen))
		if len(chosenKeys) == 3 {
			return
		}
		for i := from; i < len(keys); i++ {
			children := primitives
			if strings.HasPrefix(keys[i], "_") {
				children = elements
			}
			for _, v := range children {
				build(i+1, append(append([]string{}, chosenKeys...), keys[i]), append(append([]string{}, chosen...), v))
			}
		}
	}
	build(0, nil, nil)
	violations := 0
	for _, data := range objects {
		o := object(data)
		for _, name := range []string{"a", "b"} {
			if got, want := itemsOf(data, name), len(o.GetCollection(name)); got != want {
				violations++
				if violations <= 10 {
					t.Errorf("%s under %q: = compares %d items, navigation counts %d", data, name, got, want)
				}
			}
		}
	}
	t.Logf("%d objects, %d disagreements", len(objects), violations)
	if violations > 0 {
		t.Errorf("%d disagreements", violations)
	}
}

// TestEqualKeysAreEqualOverGeneratedObjects: en cada par generado que es
// igual y tiene dos claves, las claves son la misma.
func TestEqualKeysAreEqualOverGeneratedObjects(t *testing.T) {
	objects := symmetryObjects(symmetricChildren(t), 5)
	var equal, keyed, unkeyed int
	for i, a := range objects {
		for j := i; j < len(objects); j++ {
			x, y := symmetryForm(a, objects[j], i+j)
			l, r := object(x), object(y)
			if !l.Equal(r) {
				continue
			}
			equal++
			kl, lok := equalityKey(l)
			kr, rok := equalityKey(r)
			if !lok || !rok {
				unkeyed++
				continue
			}
			keyed++
			if kl != kr {
				t.Errorf("%s = %s, but their keys differ:\n %q\n %q", x, y, kl, kr)
			}
		}
	}
	t.Logf("%d equal pairs, %d keyed, %d with no key", equal, keyed, unkeyed)
	if keyed < 1000 {
		t.Errorf("only %d equal pairs have keys", keyed)
	}
	if unkeyed > 0 {
		t.Errorf("%d equal pairs have no key", unkeyed)
	}
}

// TestEqualityKeyHoldsTheNamesItDoesNotTakeForElements: las claves "" y "_"
// son nombres como cualquier otro, con una clave propia, y {} no recibe la
// clave de {"_":{}}.
func TestEqualityKeyHoldsTheNamesItDoesNotTakeForElements(t *testing.T) {
	keyOf := func(text string) string {
		key, ok := equalityKey(object(text))
		if !ok {
			t.Errorf("%s has no key", text)
		}
		return key
	}
	empty, underscore, blank, other := keyOf(`{}`), keyOf(`{"_":{}}`), keyOf(`{"":"x"}`), keyOf(`{"_":{"id":"e"}}`)
	if empty == underscore {
		t.Errorf("{} and {\"_\":{}} share the key %q", empty)
	}
	if blank == underscore || other == underscore {
		t.Errorf("the keys of {\"\":\"x\"}, {\"_\":{}} and {\"_\":{\"id\":\"e\"}} are %q, %q and %q", blank, underscore, other)
	}
	// un elemento no es un miembro, y un nombre que solo lo parece es
	if key := keyOf(`{"_a":{}}`); key != empty {
		t.Errorf("{\"_a\":{}} has the key %q, want the key of {}: %q", key, empty)
	}
}
