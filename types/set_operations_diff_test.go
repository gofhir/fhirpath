package types

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
)

// Las operaciones de conjunto como estaban antes del índice (#243), copiadas
// aquí congeladas: la referencia con la que se miden las indexadas. Llevan su
// propio containsRef para no alcanzar nada del índice.

func containsRef(c Collection, v Value) bool {
	for _, item := range c {
		if item.Equal(v) {
			return true
		}
	}
	return false
}

func distinctRef(c Collection) Collection {
	if len(c) <= 1 {
		return c
	}
	result := make(Collection, 0, len(c))
	for _, item := range c {
		if !containsRef(result, item) {
			result = append(result, item)
		}
	}
	return result
}

func isDistinctRef(c Collection) bool {
	return len(c) == len(distinctRef(c))
}

func unionRef(c, other Collection) Collection {
	result := make(Collection, 0, len(c)+len(other))
	for _, item := range append(append(Collection{}, c...), other...) {
		if !containsRef(result, item) {
			result = append(result, item)
		}
	}
	return result
}

func intersectRef(c, other Collection) Collection {
	result := make(Collection, 0)
	for _, item := range c {
		if containsRef(other, item) && !containsRef(result, item) {
			result = append(result, item)
		}
	}
	return result
}

func excludeRef(c, other Collection) Collection {
	result := make(Collection, 0)
	for _, item := range c {
		if !containsRef(other, item) {
			result = append(result, item)
		}
	}
	return result
}

func subsetOfRef(c, other Collection) bool {
	for _, item := range c {
		if !containsRef(other, item) {
			return false
		}
	}
	return true
}

func supersetOfRef(c, other Collection) bool {
	for _, item := range other {
		if !containsRef(c, item) {
			return false
		}
	}
	return true
}

// sameRepresentative dice si dos resultados son el mismo elemento: el
// mismo valor de Go, con la misma metadata, y el mismo objeto cuando es un
// objeto. Comparar con = no lo vería: 1 y 1.0 son iguales.
func sameRepresentative(got, want Value) bool {
	if reflect.TypeOf(got) != reflect.TypeOf(want) {
		return false
	}
	if got.String() != want.String() || got.Type() != want.Type() || IsFHIRPrimitive(got) != IsFHIRPrimitive(want) {
		return false
	}
	if g, ok := got.(interface{ Element() *ObjectValue }); ok {
		if g.Element() != want.(interface{ Element() *ObjectValue }).Element() {
			return false
		}
	}
	if reflect.TypeOf(got).Comparable() {
		return got == want
	}
	return reflect.DeepEqual(got, want)
}

func describe(c Collection) string {
	parts := make([]string, len(c))
	for i, v := range c {
		parts[i] = fmt.Sprintf("%T(%s)", v, v.String())
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func sameItems(t *testing.T, label string, got, want, from, other Collection) {
	t.Helper()
	if len(got) != len(want) || (got == nil) != (want == nil) {
		t.Errorf("%s: %d items (nil %v), want %d (nil %v)\n c = %s\n other = %s\n got = %s\n want = %s",
			label, len(got), got == nil, len(want), want == nil, describe(from), describe(other), describe(got), describe(want))
		return
	}
	for i := range got {
		if !sameRepresentative(got[i], want[i]) {
			t.Errorf("%s: item %d is %T(%s), want %T(%s)\n c = %s\n other = %s",
				label, i, got[i], got[i].String(), want[i], want[i].String(), describe(from), describe(other))
			return
		}
	}
}

// checkSetOperations compara cada operación con su referencia, para c y other
// en los dos órdenes.
func checkSetOperations(t *testing.T, label string, c, other Collection) {
	t.Helper()
	for _, pair := range [2][2]Collection{{c, other}, {other, c}} {
		a, b := pair[0], pair[1]
		sameItems(t, label+" Union", a.Union(b), unionRef(a, b), a, b)
		sameItems(t, label+" Distinct", a.Distinct(), distinctRef(a), a, b)
		sameItems(t, label+" Intersect", a.Intersect(b), intersectRef(a, b), a, b)
		sameItems(t, label+" Exclude", a.Exclude(b), excludeRef(a, b), a, b)
		if got, want := a.IsDistinct(), isDistinctRef(a); got != want {
			t.Errorf("%s IsDistinct = %v, want %v\n c = %s", label, got, want, describe(a))
		}
		if got, want := a.SubsetOf(b), subsetOfRef(a, b); got != want {
			t.Errorf("%s SubsetOf = %v, want %v\n c = %s\n other = %s", label, got, want, describe(a), describe(b))
		}
		if got, want := a.SupersetOf(b), supersetOfRef(a, b); got != want {
			t.Errorf("%s SupersetOf = %v, want %v\n c = %s\n other = %s", label, got, want, describe(a), describe(b))
		}
	}
}

func mustValue[T Value](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

func mustQuantity(s string) Quantity      { return mustValue(NewQuantity(s)) }
func mustDecimal(s string) Decimal        { return mustValue(NewDecimal(s)) }
func mustReadDecimal(s string) Decimal    { return mustValue(ReadDecimal(s)) }
func setDateTime(s string) DateTime       { return mustValue(NewDateTime(s)) }
func setDate(s string) Date               { return mustValue(NewDate(s)) }
func mustTime(s string) Time              { return mustValue(NewTime(s)) }
func object(data string) *ObjectValue     { return NewObjectValue([]byte(data)) }
func typed(data, t string) *ObjectValue   { return NewObjectValueWithType([]byte(data), t) }
func quantityObject(v, unit string) Value { return object(`{"value":` + v + `,"unit":"` + unit + `"}`) }

// setFamilies son grupos de valores que se parecen entre sí a propósito: los
// que son iguales con otra escritura, otro tipo o otra metadata, y los que lo
// son solo por conversión. Un conjunto generado toma varios de una familia, de
// modo que casi toda colección tiene duplicados por igualdad.
func setFamilies() [][]Value {
	element := object(`{"extension":[{"url":"http://example.org/x","valueString":"x"}]}`)
	other := object(`{"id":"e1"}`)
	return [][]Value{
		// el 1 en todas sus escrituras, y como cantidad de unidad 1
		{NewInteger(1), mustDecimal("1.0"), mustDecimal("1"), mustDecimal("1.00"), mustQuantity("1 '1'"), quantityObject("1", "1"),
			NewInteger(1).WithElement(element), NewInteger(1).WithFHIRType("positiveInt")},
		{NewInteger(2), mustDecimal("2.0"), mustQuantity("2 '1'"), mustDecimal("2.000")},
		{NewInteger(0), mustDecimal("0.0"), mustDecimal("0"), mustDecimal("-0.0")},
		{NewInteger(10), mustDecimal("10.0"), mustDecimal("10"), NewInteger(10).WithFHIRType("unsignedInt")},
		{mustDecimal("0.5"), mustDecimal("0.50"), mustDecimal("0.500")},
		{mustDecimal("-1"), NewInteger(-1)},
		{mustReadDecimal("1e5000000"), mustReadDecimal("10e4999999"), mustReadDecimal("100e4999998")},
		{mustReadDecimal("1e5000001")},
		// la cantidad de gramos en sus otras unidades, y sus objetos FHIR
		{mustQuantity("1 'g'"), mustQuantity("1000 'mg'"), quantityObject("1", "g"), quantityObject("1000", "mg"),
			typed(`{"value":1,"unit":"g"}`, "Quantity")},
		{mustQuantity("5 '1'"), NewInteger(5), mustDecimal("5.0"), quantityObject("5", "1")},
		{mustQuantity("5 'mg'"), mustQuantity("0.005 'g'"), quantityObject("5", "mg")},
		// igualdad de unidades de calendario, que no es simétrica
		{mustQuantity("0.0833333333333333 year"), mustQuantity("1 month")},
		{mustQuantity("1 year"), mustQuantity("12 months")},
		// fechas y horas
		{setDateTime("2020-01-01T10:00:00+00:00"), setDateTime("2020-01-01T11:00:00+01:00"), setDateTime("2020-01-01T10:00:00Z")},
		{setDate("2020-01-01"), setDateTime("2020-01-01")},
		{setDateTime("2020-01-01T10:00:00"), setDateTime("2020-01-01T10:00:00.000")},
		{setDate("2020-01"), setDateTime("2020-01")},
		{mustTime("10:00:00"), mustTime("10:00:00.000")},
		// textos: lo que se escribe distinto y se lee igual, y la metadata
		{NewString("a"), NewStringWithFHIRType("a", "code"), NewStringWithFHIRType("a", "uri"), NewString("a").WithElement(element),
			NewStringWithFHIRType("a", "code").WithElement(other)},
		{NewString("b"), NewStringWithFHIRType("b", "id")},
		{NewString("é"), NewString("é"), NewString("é")},
		{NewString(`a"b`), NewString("a\"b")},
		{NewString("1"), NewString("1.0")},
		{NewString(""), NewStringWithFHIRType("", "code")},
		{NewString("2020-01-01"), NewStringWithFHIRType("2020-01-01", "date")},
		{NewBoolean(true), NewBoolean(true).WithElement(element)},
		{NewBoolean(false), NewBoolean(false).WithFHIRType("boolean")},
		// objetos escritos distinto y leídos igual
		{object(`{"a":1,"b":"x"}`), object(`{"b":"x","a":1.0}`), object(`{ "a" : 1 , "b" : "x" }`), object(`{"a":1,"b":"x","c":null}`),
			object(`{"a":1,"b":"x","d":[]}`), object(`{"a":[1],"b":["x"]}`), object(`{"b":"x","a":1}`), object(`{"a":1e0,"b":"x"}`),
			object(`{"a":[null,1],"b":"x"}`), object(`{"a":1,"a":2,"b":"x"}`), object(`{"a":1,"b":"x","z":0x1}`)},
		{object(`{"a":1,"b":"y"}`), object(`{"a":1.0,"b":"y"}`)},
		{object(`{"a":2,"b":"x"}`)},
		{object(`{}`), object(`{"a":null}`), object(`{"a":[]}`), object(`{"a":[null]}`), object(`{"a":0x1}`)},
		// la misma clave, distintos por = e iguales por ~: un índice que compare por ~
		// los juntaría
		{object(`{"given":["J"],"_given":[{"id":"x"}]}`), object(`{"_given":[{"id":"x"}],"given":["J"]}`),
			object(`{"given":["J"],"_given":[{"id":"X"}]}`)},
		{object(`{"given":["J"],"_given":[{"id":"y"}]}`)},
		{object(`{"given":["J"]}`), object(`{"given":["J"],"_given":[{}]}`), object(`{"given":"J"}`)},
		{object(`{"_given":[{"id":"x"}]}`), object(`{"_given":{"id":"x"}}`)},
		{object(`{"o":{"y":2,"x":1}}`), object(`{"o":{"x":1,"y":2}}`), object(`{"o":{"x":1.0,"y":2,"w":null}}`)},
		{object(`{"o":[{"x":1},{"x":2}]}`), object(`{"o":[{"x":1.0},{"x":2}]}`)},
		{object(`{"o":[{"x":2},{"x":1}]}`)},
		{object(`{"n":[[1],[2]]}`), object(`{"n":[[1],[2]]}`)},
		{object(`{"n":[[1],[3]]}`)},
		{object(`{"t":"a\"b"}`), object(`{"t":"a"b"}`)},
		{object(`{"t":"é"}`), object(`{"t":"é"}`)},
		{object(`{"a":1`), object(`{"a":1 `), object(`not json`)},
		// "" y "_" son nombres como los demás: distintos entre sí y de {}
		{object(`{"_":{}}`)},
		{object(`{"":"x","_":{}}`)},
		{object(`{"":[],"_":{}}`), object(`{"":[],"_":{},"":null,"":"duplicate"}`)},
		{object(`{"text":"x","extension":[{"url":"u","valueInteger":1}]}`), object(`{"extension":[{"valueInteger":1,"url":"u"}],"text":"x"}`)},
		// R5: un integer64 de un objeto, escrito como cadena, como entero y como decimal
		{object(`{"v":"9007199254740993"}`)},
		{object(`{"v":9007199254740993}`), object(`{"v":9007199254740993.0}`), object(`{"v":9.007199254740993e15}`)},
		{mustDecimal("9007199254740993.0"), mustDecimal("9007199254740993"), NewInteger(9007199254740993)},
		{typed(`{"value":9007199254740993}`, "Quantity"), mustQuantity("9007199254740993 '1'")},
	}
}

// fillerStrings son cadenas distintas entre sí, para llevar una colección más
// allá del umbral sin cambiar lo que dicen los demás.
func fillerStrings(prefix string, n int) Collection {
	out := make(Collection, n)
	for i := range out {
		out[i] = NewString(fmt.Sprintf("%s-%d", prefix, i))
	}
	return out
}

// drawSet toma n valores de las familias, varios de la misma.
func drawSet(rng *rand.Rand, families [][]Value, n int) Collection {
	out := make(Collection, n)
	for i := range out {
		family := families[rng.Intn(len(families))]
		out[i] = family[rng.Intn(len(family))]
	}
	return out
}

func TestSetOperationsMatchTheReference(t *testing.T) {
	families := setFamilies()
	sizes := []int{0, 1, 2, 3, 7, indexFrom - 1, indexFrom, indexFrom + 1, 2 * indexFrom, 40}
	rng := rand.New(rand.NewSource(243))
	cases := 0
	for _, n := range sizes {
		for _, m := range sizes {
			for round := 0; round < 4; round++ {
				c, other := drawSet(rng, families, n), drawSet(rng, families, m)
				checkSetOperations(t, fmt.Sprintf("n=%d m=%d round=%d", n, m, round), c, other)
				cases++
			}
		}
	}
	// una mitad de sí misma, y consigo misma
	for _, n := range sizes {
		c := drawSet(rng, families, n)
		checkSetOperations(t, fmt.Sprintf("half n=%d", n), c, c[:len(c)/2])
		checkSetOperations(t, fmt.Sprintf("self n=%d", n), c, c)
		cases += 2
	}
	t.Logf("%d cases, threshold %d", cases, indexFrom)
}

// TestSetOperationsOverEveryFamilyAboveTheThreshold toma una familia entera
// con relleno que la lleva sobre el umbral, en todas las posiciones del
// relleno, para que ninguna se compare solo bajo él.
func TestSetOperationsOverEveryFamilyAboveTheThreshold(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	for i, family := range setFamilies() {
		for _, size := range []int{indexFrom - 1, indexFrom, indexFrom + 1} {
			filler := fillerStrings("f", size)
			// la familia al comienzo, en medio y al final del relleno
			for _, at := range []int{0, len(filler) / 2, len(filler)} {
				c := append(append(append(Collection{}, filler[:at]...), family...), filler[at:]...)
				other := append(Collection{}, family...)
				rng.Shuffle(len(other), func(a, b int) { other[a], other[b] = other[b], other[a] })
				other = append(other, fillerStrings("g", size)...)
				checkSetOperations(t, fmt.Sprintf("family %d size %d at %d", i, size, at), c, other)
			}
		}
	}
}

// orders llama a f con cada orden de values.
func orders(values Collection, f func(Collection)) {
	var rec func(k int)
	rec = func(k int) {
		if k == len(values) {
			f(append(Collection{}, values...))
			return
		}
		for i := k; i < len(values); i++ {
			values[k], values[i] = values[i], values[k]
			rec(k + 1)
			values[k], values[i] = values[i], values[k]
		}
	}
	rec(0)
}

// TestAsymmetricAndNontransitiveGroups corre cada grupo en todos los órdenes,
// solo y con relleno sobre el umbral, donde las operaciones de pertenencia
// pasan por el índice.
func TestAsymmetricAndNontransitiveGroups(t *testing.T) {
	a, b, c := quantityObject("1", "g"), mustQuantity("1 'g'"), quantityObject("1000", "mg")
	groups := map[string]Collection{
		"year and month":    {mustQuantity("0.0833333333333333 year"), mustQuantity("1 month")},
		"one":               {NewInteger(1), mustDecimal("1.0"), mustQuantity("1 '1'"), quantityObject("1", "1")},
		"datetime and date": {setDateTime("2020-01-01T10:00:00+00:00"), setDateTime("2020-01-01T11:00:00+01:00"), setDate("2020-01-01")},
		"A, B and C":        {a, b, c},
	}
	for name, group := range groups {
		for _, pad := range []int{0, indexFrom - 1, indexFrom, indexFrom + 1} {
			count := 0
			orders(append(Collection{}, group...), func(order Collection) {
				// cada orden contra todos los demás de las dos formas: solo, y rodeado de relleno
				filler := fillerStrings("p", pad)
				left := append(append(Collection{}, filler...), order...)
				orders(append(Collection{}, group...), func(second Collection) {
					right := append(append(Collection{}, second...), fillerStrings("q", pad)...)
					checkSetOperations(t, fmt.Sprintf("%s pad %d", name, pad), left, right)
					count++
				})
			})
			if count == 0 {
				t.Fatalf("%s: nothing ran", name)
			}
		}
	}

	// other = [A, B] contra C, sobre el umbral: C es igual a B y no a A, de modo
	// que quitar a B del índice de other, por igual a A, lo perdería
	for _, pad := range []int{indexFrom - 1, indexFrom, indexFrom + 1} {
		other := append(fillerStrings("o", pad), a, b)
		input := append(fillerStrings("o", pad), c)
		checkSetOperations(t, fmt.Sprintf("A,B against C pad %d", pad), input, other)
		if !input.Intersect(other).Contains(c) {
			t.Errorf("pad %d: Intersect lost C, which equals B in other", pad)
		}
		if input.SubsetOf(other) != subsetOfRef(input, other) {
			t.Errorf("pad %d: SubsetOf differs", pad)
		}
	}
}

func TestGeneratedObjectsMatchTheReference(t *testing.T) {
	rng := rand.New(rand.NewSource(1243))
	names := []string{"a", "b", "c", "given", "_given", "_a", "code", "n"}
	values := []string{`1`, `1.0`, `1e0`, `2`, `"x"`, `"x"`, `"é"`, `true`, `null`, `[]`, `[1]`, `[1,null]`, `[null,1]`, `[[1]]`, `{"k":1}`, `{"k":1.0,"j":null}`,
		`{"id":"e"}`, `{}`, `[{}]`, `0x1`, `-0`, `0`, `[{"k":2},{"k":1}]`, `"1"`}
	write := func() string {
		var parts []string
		for _, name := range names {
			if rng.Intn(3) == 0 {
				parts = append(parts, fmt.Sprintf("%q:%s", name, values[rng.Intn(len(values))]))
			}
		}
		rng.Shuffle(len(parts), func(i, j int) { parts[i], parts[j] = parts[j], parts[i] })
		return "{" + strings.Join(parts, ",") + "}"
	}
	// un conjunto pequeño de objetos, para que se repitan
	pool := make([]Value, 30)
	for i := range pool {
		pool[i] = object(write())
	}
	for _, n := range []int{indexFrom - 1, indexFrom, indexFrom + 1, 3 * indexFrom} {
		for round := 0; round < 20; round++ {
			c, other := make(Collection, n), make(Collection, n)
			for i := range c {
				c[i], other[i] = pool[rng.Intn(len(pool))], pool[rng.Intn(len(pool))]
			}
			checkSetOperations(t, fmt.Sprintf("objects n=%d round=%d", n, round), c, other)
		}
	}
}
