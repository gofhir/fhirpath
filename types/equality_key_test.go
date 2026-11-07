package types

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// Cómo se reescribe un objeto sin cambiar lo que dice. Cada bit de una máscara
// agrega un cambio a la escritura; dos escrituras de un mismo objeto con
// máscaras distintas deben ser iguales por =, y su clave la misma.
const (
	rewriteReorder    = 1 << iota // las claves en otro orden
	rewriteNumbers                // 1 como 1.0 y 1e0
	rewriteEscape                 // un carácter escapado
	rewriteNullMember             // un nombre con null
	rewriteEmptyArray             // un nombre con un arreglo vacío
	rewriteNullItem               // un null dentro de un arreglo
	rewriteDuplicate              // un nombre escrito dos veces
	rewriteElement                // un _elemento sin valor al lado de un primitivo
	rewriteBadNumber              // un número fuera de la gramática de JSON
	rewriteNest                   // el objeto como hijo de otro
)

// rewriter escribe un árbol de JSON leído con encoding/json.
type rewriter struct {
	mask uint
	rng  *rand.Rand
}

func (w rewriter) write(sb *strings.Builder, v any) {
	switch v := v.(type) {
	case map[string]any:
		w.writeObject(sb, v)
	case []any:
		sb.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				sb.WriteByte(',')
			}
			if w.mask&rewriteNullItem != 0 && w.rng.Intn(2) == 0 {
				sb.WriteString("null,")
			}
			w.write(sb, item)
		}
		sb.WriteByte(']')
	case string:
		w.writeString(sb, v)
	case json.Number:
		sb.WriteString(w.spell(string(v)))
	case bool:
		sb.WriteString(strconv.FormatBool(v))
	default:
		sb.WriteString("null")
	}
}

// spell escribe un número de otra forma que vale lo mismo.
func (w rewriter) spell(n string) string {
	if w.mask&rewriteNumbers == 0 || strings.ContainsAny(n, "eE") {
		return n
	}
	switch w.rng.Intn(3) {
	case 0:
		if !strings.Contains(n, ".") {
			return n + ".0"
		}
	case 1:
		return n + "e0"
	}
	return n
}

func (w rewriter) writeString(sb *strings.Builder, s string) {
	sb.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			sb.WriteString(`\"`)
		case r == '\\':
			sb.WriteString(`\\`)
		case r < 0x20:
			fmt.Fprintf(sb, `\u%04x`, r)
		case w.mask&rewriteEscape != 0 && w.rng.Intn(3) == 0:
			if r > 0xffff {
				sb.WriteRune(r)
			} else {
				fmt.Fprintf(sb, `\u%04x`, r)
			}
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
}

func (w rewriter) writeObject(sb *strings.Builder, m map[string]any) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if w.mask&rewriteReorder != 0 {
		w.rng.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
	}
	var parts []string
	member := func(key string, write func(*strings.Builder)) {
		var part strings.Builder
		w.writeString(&part, key)
		part.WriteByte(':')
		write(&part)
		parts = append(parts, part.String())
	}
	for _, k := range keys {
		member(k, func(sb *strings.Builder) { w.write(sb, m[k]) })
	}
	extra := func(key, value string) { member(key, func(sb *strings.Builder) { sb.WriteString(value) }) }
	if w.mask&rewriteNullMember != 0 {
		extra("zn", "null")
	}
	if w.mask&rewriteEmptyArray != 0 {
		extra("ze", "[]")
	}
	if w.mask&rewriteBadNumber != 0 {
		extra("zb", "0x1")
	}
	if w.mask&rewriteElement != 0 {
		for _, k := range keys {
			if _, isObject := m[k].(map[string]any); !isObject && !strings.HasPrefix(k, "_") {
				if _, exists := m["_"+k]; !exists {
					extra("_"+k, "{}")
				}
			}
		}
	}
	if w.mask&rewriteDuplicate != 0 {
		// la segunda aparición no cuenta: la lectura conserva la primera
		for _, k := range keys {
			extra(k, "null")
		}
		if len(keys) > 0 {
			extra(keys[0], `"duplicate"`)
		}
	}
	if w.mask&rewriteReorder != 0 {
		w.rng.Shuffle(len(parts), func(i, j int) { parts[i], parts[j] = parts[j], parts[i] })
	}
	sb.WriteString("{" + strings.Join(parts, ",") + "}")
}

// rewrite escribe el JSON de seed con los cambios de la máscara, o dice que
// seed no es un objeto de JSON.
func rewrite(seed string, mask uint, rngSeed int64) (string, bool) {
	decoder := json.NewDecoder(strings.NewReader(seed))
	decoder.UseNumber()
	var tree map[string]any
	if decoder.Decode(&tree) != nil || tree == nil {
		return "", false
	}
	w := rewriter{mask: mask, rng: rand.New(rand.NewSource(rngSeed))}
	var sb strings.Builder
	if mask&rewriteNest != 0 {
		tree = map[string]any{"n": tree}
	}
	w.write(&sb, tree)
	return sb.String(), true
}

// agree dice si la clave respeta a Equal para a y b: si a.Equal(b) y los dos
// tienen clave, es la misma.
func agree(a, b Value) (agrees bool, why string) {
	ka, oka := equalityKey(a)
	kb, okb := equalityKey(b)
	if oka && okb && a.Equal(b) && ka != kb {
		return false, fmt.Sprintf("%s = %s, but their keys differ:\n %q\n %q", a.String(), b.String(), ka, kb)
	}
	return true, ""
}

func checkKeyAgreement(t *testing.T, a, b Value) {
	t.Helper()
	if ok, why := agree(a, b); !ok {
		t.Fatalf("%s", why)
	}
	if ok, why := agree(b, a); !ok {
		t.Fatalf("(swapped) %s", why)
	}
}

// FuzzEqualityKeyAgreesWithEqual sostiene la propiedad de la clave: dos
// valores con clave que son iguales por = tienen la misma. Los pares
// independientes casi nunca son iguales, de modo que el objetivo deriva los
// suyos de una semilla, escrita de dos formas que dicen lo mismo: claves
// reordenadas, un número como 1, 1.0 o 1e0, un carácter escapado, un null, un
// arreglo vacío o un null dentro de un arreglo, un nombre repetido, un
// _elemento sin valor, un número fuera de la gramática, o el objeto anidado.
// picks: los dos primeros bytes son la máscara de la segunda escritura, los
// dos siguientes la de la primera, y el resto siembra el azar.
func FuzzEqualityKeyAgreesWithEqual(f *testing.F) {
	for _, seed := range equalityKeySeeds() {
		f.Add(seed.object, seed.other, seed.picks)
	}
	f.Fuzz(func(t *testing.T, seed, other string, picks []byte) {
		var left, right uint
		var rngSeed int64
		if len(picks) >= 2 {
			right = uint(picks[0]) | uint(picks[1])<<8
		}
		if len(picks) >= 4 {
			left = uint(picks[2]) | uint(picks[3])<<8
		}
		for _, b := range picks[min(len(picks), 4):] {
			rngSeed = rngSeed*31 + int64(b)
		}
		a, aok := rewrite(seed, left, rngSeed)
		b, bok := rewrite(seed, right, rngSeed+1)
		if aok && bok {
			checkKeyAgreement(t, NewObjectValue([]byte(a)), NewObjectValue([]byte(b)))
		}
		// dos textos cualesquiera, tal como llegan
		checkKeyAgreement(t, NewObjectValue([]byte(seed)), NewObjectValue([]byte(other)))
		// y sus formas de otros tipos
		for _, pair := range scalarPairs(seed) {
			checkKeyAgreement(t, pair[0], pair[1])
		}
	})
}

// scalarPairs son valores iguales por = de tipos o escrituras distintos, que
// salen del texto: un entero y el decimal que vale lo mismo, una cadena y la
// misma con otro tipo FHIR o con metadata.
func scalarPairs(text string) [][2]Value {
	pairs := [][2]Value{
		{NewString(text), NewStringWithFHIRType(text, "code")},
		{NewString(text), NewString(text).WithElement(NewObjectValue([]byte(`{"id":"e"}`)))},
	}
	if n, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64); err == nil {
		d, _ := NewDecimal(strconv.FormatInt(n, 10) + ".0")
		pairs = append(pairs, [2]Value{NewInteger(n), d}, [2]Value{d, NewInteger(n)})
	}
	if d, err := NewDecimal(strings.TrimSpace(text)); err == nil {
		z, _ := NewDecimal(d.String() + "00")
		pairs = append(pairs, [2]Value{d, z})
	}
	return pairs
}

type keySeed struct {
	object, other string
	picks         []byte
}

// equalityKeySeeds es el corpus de partida: un objeto de cada forma, con los
// cambios que lo reescriben de a uno y todos juntos.
func equalityKeySeeds() []keySeed {
	objects := []string{
		`{"a":1,"b":"x"}`,
		`{"a":1.50,"b":[1,2,3],"c":{"d":"é","e":true}}`,
		`{"given":["J","K"],"_given":[{"id":"x"},null],"family":"F"}`,
		`{"code":{"coding":[{"system":"s","code":"c"}],"text":"t"},"valueQuantity":{"value":5,"unit":"mg"}}`,
		`{"n":[[1],[2]],"o":[{"k":1},{"k":2.0}]}`,
		`{"t":"a\"b\\c\n","u":"é"}`,
		`{"v":9007199254740993,"w":"9007199254740993"}`,
		`{"a":1e5000000,"b":0.000000000000000000000000000000000000001}`,
		`{"a":null,"b":[],"c":[null],"d":{}}`,
		`{"a":1,"a":2,"b":null,"b":3}`,
	}
	var seeds []keySeed
	for i, object := range objects {
		other := objects[(i+3)%len(objects)]
		for bit := 0; bit < 10; bit++ {
			mask := uint(1) << bit
			picks := []byte{byte(mask), byte(mask >> 8), 0, 0, byte(i), byte(bit)}
			if mask == rewriteNest {
				// anidado de los dos lados; de uno solo no son iguales
				picks[2], picks[3] = picks[0], picks[1]
			}
			seeds = append(seeds, keySeed{object, other, picks})
		}
		seeds = append(seeds,
			keySeed{object, other, []byte{0xff, 0x03, 0, 0, byte(i)}},
			keySeed{object, other, []byte{0xff, 0x03, 0x55, 0x01, byte(i)}})
	}
	return seeds
}

// TestEqualityKeySeedsAreEqualPairs dice que el corpus hace lo que el objetivo
// pretende: la mayor parte de lo que reescribe es igual por =, de modo que la
// propiedad se pone a prueba y no se cumple por vacío.
func TestEqualityKeySeedsAreEqualPairs(t *testing.T) {
	var total, equal, keyed int
	for _, seed := range equalityKeySeeds() {
		right := uint(seed.picks[0]) | uint(seed.picks[1])<<8
		left := uint(seed.picks[2]) | uint(seed.picks[3])<<8
		a, aok := rewrite(seed.object, left, 1)
		b, bok := rewrite(seed.object, right, 2)
		if !aok || !bok {
			t.Fatalf("%s does not rewrite", seed.object)
		}
		total++
		x, y := NewObjectValue([]byte(a)), NewObjectValue([]byte(b))
		if x.Equal(y) {
			equal++
			if _, ok := equalityKey(x); ok {
				keyed++
			}
		}
		checkKeyAgreement(t, x, y)
	}
	t.Logf("%d rewrites, %d equal, %d of those with a key", total, equal, keyed)
	if equal*10 < total*7 {
		t.Errorf("only %d of %d rewrites are equal; the seeds do not test the property", equal, total)
	}
	if keyed != equal {
		t.Errorf("%d of %d equal rewrites have no key", equal-keyed, equal)
	}
}

func TestEqualityKeyOfScalars(t *testing.T) {
	d := mustDecimal
	same := [][2]Value{
		{NewInteger(1), d("1.0")},
		{NewInteger(10), d("10")},
		{NewInteger(10), d("1e1")},
		{NewInteger(0), d("-0.00")},
		{NewInteger(-100), d("-1e2")},
		{d("0.5"), d("0.50")},
		{mustReadDecimal("1e5000000"), mustReadDecimal("10e4999999")},
		{NewString("a"), NewStringWithFHIRType("a", "code")},
		{NewBoolean(true), NewBoolean(true).WithFHIRType("boolean")},
	}
	for _, pair := range same {
		ka, oka := equalityKey(pair[0])
		kb, okb := equalityKey(pair[1])
		if !oka || !okb || ka != kb || !pair[0].Equal(pair[1]) {
			t.Errorf("%v and %v: keys %q (%v), %q (%v), equal %v", pair[0], pair[1], ka, oka, kb, okb, pair[0].Equal(pair[1]))
		}
	}
	different := [][2]Value{
		{NewInteger(1), NewInteger(10)},
		{NewInteger(10), d("1")},
		{NewString("1"), NewInteger(1)},
		{NewBoolean(true), NewBoolean(false)},
		{mustReadDecimal("1e5000000"), mustReadDecimal("1e5000001")},
	}
	for _, pair := range different {
		ka, _ := equalityKey(pair[0])
		kb, _ := equalityKey(pair[1])
		if ka == kb {
			t.Errorf("%v and %v share the key %q", pair[0], pair[1], ka)
		}
	}
	for _, v := range []Value{mustQuantity("1 'g'"), quantityObject("1", "g"), setDate("2020-01-01"), setDateTime("2020-01-01T10:00:00Z"), mustTime("10:00:00")} {
		if key, ok := equalityKey(v); ok {
			t.Errorf("%T(%s) has the key %q, and should have none", v, v, key)
		}
	}
	// "" y "_" son nombres como cualquier otro, y tienen una clave
	for _, text := range []string{`{"_":{}}`, `{"":"x"}`, `{"a":{"_":1}}`, `{"a":[{"":1}]}`} {
		if _, ok := equalityKey(object(text)); !ok {
			t.Errorf("%s has no key", text)
		}
	}
	if key, ok := equalityKey(object(`{"a":`)); ok {
		t.Errorf("an object that does not read has the key %q", key)
	}
}
