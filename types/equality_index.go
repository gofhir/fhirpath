package types

import (
	"bytes"
	"hash/maphash"
	"slices"
	"strconv"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/shopspring/decimal"
)

// indexFrom es el tamaño desde el que un índice reemplaza al recorrido de la
// colección: bajo él, el recorrido no pide el mapa ni calcula una clave por
// elemento. Es una variable solo para que las pruebas la midan; nada la cambia.
var indexFrom = 32

// hashSeed es la semilla de las claves del índice, fija por proceso.
var hashSeed = maphash.MakeSeed()

// UnknownQueries es lo que se pasa a NewEqualityIndex cuando no se sabe
// cuántas preguntas vendrán.
const UnknownQueries = -1

// EqualityIndex guarda elementos y responde si alguno es igual, por =, a un
// valor. Bajo cierto tamaño revisa los elementos, como Collection.Contains;
// desde ahí los reparte por una clave de igualdad, de modo que un valor se
// compara solo con los de su cubeta y con los que no tienen clave: ver
// equalityKey.
//
// La respuesta es la misma en los dos casos: la clave solo elige a quién se
// pregunta, y Equal decide. Cada comparación conserva el receptor y el
// argumento, guardado.Equal(valor), porque = entre unidades de calendario no es
// simétrica (0.0833333333333333 year = 1 month, y al revés no).
//
// Los valores guardados no deben cambiar mientras se use el índice: cada uno
// queda en la cubeta de la clave que tenía al entrar. Ver ObjectValue.Data.
//
// Un índice no es seguro para varias goroutines.
type EqualityIndex struct {
	items Collection
	// worth dice que habrá preguntas de sobra para pagar el índice; hint es lo
	// que cabe en el arreglo de elementos, para dimensionar el mapa
	worth   bool
	hint    int
	indexed bool
	head    map[uint64]chain
	next    []int // por elemento: el siguiente de su cubeta, o -1
	unkeyed []int // los elementos sin clave, en orden de inserción
	writer  keyWriter
}

// chain es la cubeta de una clave: su primer y su último elemento.
type chain struct{ first, last int }

// NewEqualityIndex crea un índice vacío, que se llena con AddIfAbsent. queries
// es cuántas preguntas se harán, o UnknownQueries: con menos que el umbral
// recorrer sale más barato que construir el índice, que cuesta una clave por
// elemento.
func NewEqualityIndex(queries int) *EqualityIndex {
	x := new(EqualityIndex)
	x.init(nil, queries)
	return x
}

// init prepara un índice con los elementos dados, todos conservados, los
// iguales también. Las operaciones de este paquete lo declaran como variable y
// llaman a init, de modo que el índice, que bajo el umbral no es más que un
// recorrido, no pide memoria al montón. No copia items: quien llama no los
// cambia mientras use el índice, y a uno que se le agregan elementos le da un
// arreglo propio.
func (x *EqualityIndex) init(items Collection, queries int) {
	*x = EqualityIndex{items: items, worth: queries < 0 || queries >= indexFrom, hint: cap(items)}
	x.activate()
}

// activate construye el índice cuando los elementos llegan al umbral.
func (x *EqualityIndex) activate() {
	if x.indexed || !x.worth || len(x.items) < indexFrom {
		return
	}
	x.indexed = true
	x.head = make(map[uint64]chain, max(x.hint, len(x.items)))
	x.next = make([]int, 0, cap(x.items))
	for i, item := range x.items {
		hash, keyed := x.keyOf(item)
		x.insert(i, hash, keyed)
	}
}

// insert agrega a su cubeta el elemento i, que es el siguiente que el índice
// no tenía.
func (x *EqualityIndex) insert(i int, hash uint64, keyed bool) {
	x.next = append(x.next, -1)
	if !keyed {
		x.unkeyed = append(x.unkeyed, i)
		return
	}
	if c, found := x.head[hash]; found {
		x.next[c.last] = i
		x.head[hash] = chain{c.first, i}
		return
	}
	x.head[hash] = chain{i, i}
}

// keyOf es el resumen de la clave de v: dos valores con la misma clave caen en
// la misma cubeta, y uno de otra clave no se pregunta. Un resumen igual para
// claves distintas solo suma comparaciones.
func (x *EqualityIndex) keyOf(v Value) (hash uint64, keyed bool) {
	if !x.writer.write(v) {
		return 0, false
	}
	return maphash.Bytes(hashSeed, x.writer.buf), true
}

// containsScan es Collection.Contains.
func containsScan(items Collection, v Value) bool {
	for _, item := range items {
		if item.Equal(v) {
			return true
		}
	}
	return false
}

// contains busca v entre los elementos con la clave dada, y entre los que no
// tienen clave. Un valor sin clave se compara con todos.
func (x *EqualityIndex) contains(v Value, hash uint64, keyed bool) bool {
	if !keyed {
		return containsScan(x.items, v)
	}
	if c, found := x.head[hash]; found {
		for i := c.first; i >= 0; i = x.next[i] {
			if x.items[i].Equal(v) {
				return true
			}
		}
	}
	for _, i := range x.unkeyed {
		if x.items[i].Equal(v) {
			return true
		}
	}
	return false
}

// Contains dice si algún elemento guardado es igual a v: guardado.Equal(v).
func (x *EqualityIndex) Contains(v Value) bool {
	if !x.indexed {
		return containsScan(x.items, v)
	}
	hash, keyed := x.keyOf(v)
	return x.contains(v, hash, keyed)
}

// AddIfAbsent guarda v si ningún elemento guardado es igual a él, y dice si lo
// guardó.
func (x *EqualityIndex) AddIfAbsent(v Value) bool {
	if !x.indexed {
		if containsScan(x.items, v) {
			return false
		}
		x.items = append(x.items, v)
		x.activate()
		return true
	}
	hash, keyed := x.keyOf(v)
	if x.contains(v, hash, keyed) {
		return false
	}
	x.items = append(x.items, v)
	x.insert(len(x.items)-1, hash, keyed)
	return true
}

// equalityKey es la clave de un valor: si a.Equal(b), los dos con clave,
// tienen la misma. Una clave más gruesa que = solo cuesta tiempo, nunca una
// respuesta; una más fina perdería elementos. Un valor sin clave
// (keyed == false) se compara con todos.
func equalityKey(v Value) (key string, keyed bool) {
	var w keyWriter
	if !w.write(v) {
		return "", false
	}
	return string(w.buf), true
}

// keyWriter escribe claves de igualdad, y reutiliza entre una y otra el búfer
// de la clave y el espacio donde se leen los objetos.
type keyWriter struct {
	buf []byte
	// arena es donde se leen los hijos de los objetos, de a uno dentro de otro;
	// used es lo que ocupan los objetos que se están escribiendo
	arena []jsonChild
	used  int
}

// write escribe en buf la clave de v, y dice si v tiene una.
//
// Tienen clave los valores que solo son iguales a otros de su tipo, o de la
// pareja Integer y Decimal, que lo son por valor. No la tienen la cantidad, ni
// el objeto que se lee como una, ni las fechas y horas: son iguales entre tipos
// y entre unidades (5 '1' = 5, 1 'g' = 1000 'mg', @2020-01-01 = un DateTime, un
// desfase y otro), y una clave tendría que seguir esos cambios de unidad.
func (w *keyWriter) write(v Value) bool {
	w.buf, w.used = w.buf[:0], 0
	switch v := v.(type) {
	case String:
		w.buf = append(append(w.buf, 's'), v.value...)
		return true
	case Boolean:
		if v.value {
			w.buf = append(w.buf, 'b', 't')
		} else {
			w.buf = append(w.buf, 'b', 'f')
		}
		return true
	case Integer:
		w.integer(v.value)
		return true
	case Decimal:
		w.decimal(v.value)
		return true
	case *ObjectValue:
		// Un objeto que se lee como cantidad es igual a un número o a una
		// cantidad: ver ObjectValue.Equal
		if _, isQuantity := v.ToQuantity(); isQuantity {
			return false
		}
		w.buf = append(w.buf, 'o')
		return w.object(v.data)
	}
	return false
}

// integer escribe un entero como el decimal que es: la clave del 10 es la del
// 10.0.
func (w *keyWriter) integer(v int64) {
	w.buf = append(w.buf, 'n')
	if v == 0 {
		w.buf = append(w.buf, '0')
		return
	}
	var digits [20]byte
	text := strconv.AppendInt(digits[:0], v, 10)
	trimmed := bytes.TrimRight(text, "0")
	w.buf = strconv.AppendInt(append(append(w.buf, trimmed...), 'e'), int64(len(text)-len(trimmed)), 10)
}

// decimal escribe un decimal en su forma canónica: signo y cifras sin los
// ceros del final, y el exponente que queda. Es exacta, también más allá de
// DecimalDigits, y dos decimales que CompareDecimals da por iguales la
// comparten.
func (w *keyWriter) decimal(d decimal.Decimal) {
	w.buf = append(w.buf, 'n')
	if d.Sign() == 0 {
		w.buf = append(w.buf, '0')
		return
	}
	text := d.Coefficient().Text(10)
	trimmed := strings.TrimRight(text, "0")
	w.buf = strconv.AppendInt(append(append(w.buf, trimmed...), 'e'), int64(d.Exponent())+int64(len(text)-len(trimmed)), 10)
}

// object escribe la lectura canónica de un objeto, con los mismos lectores con
// que equalJSON lo compara: parseJSONObject y childValues.
//
// Las claves van ordenadas por nombre; de un nombre escrito dos veces cuenta la
// aparición que la lectura conserva. Cada nombre con algún valor se escribe
// como la lista de sus valores, en orden: un número como decimal, una cadena
// decodificada, un objeto con esta misma lectura, un arreglo anidado como
// "[]". Un nombre sin valores, y todo _elemento según elementOf, quedan fuera:
// la clave es más gruesa que =, que los compara. "" y "_" son nombres como los
// demás. Un objeto o un arreglo que no se lee no tiene clave.
func (w *keyWriter) object(data []byte) bool {
	start := w.used
	ok := w.children(data)
	w.used = start
	return ok
}

// children es object, que deja en w.used lo que ocupan los hijos del objeto.
func (w *keyWriter) children(data []byte) bool {
	if w.arena == nil {
		w.arena = make([]jsonChild, 0, 64)
	}
	start := w.used
	object, ok := parseJSONObjectInto(w.arena[start:start], data)
	if !ok {
		return false
	}
	// Si cupo en la arena, la ocupa hasta que este objeto esté escrito; si no,
	// se leyó aparte
	if start+len(object.children) <= cap(w.arena) {
		w.used = start + len(object.children)
	}

	// Los hijos son nuestros: se filtran y se ordenan donde están
	children := object.children[:0]
	for _, child := range object.children {
		if _, isElement := elementOf(child.key); isElement {
			continue
		}
		children = append(children, child)
	}
	slices.SortFunc(children, func(a, b jsonChild) int { return bytes.Compare(a.key, b.key) })

	for _, child := range children {
		if !w.member(child) {
			return false
		}
	}
	return true
}

// member escribe un hijo de un objeto, con sus valores, o nada si no tiene.
func (w *keyWriter) member(child jsonChild) bool {
	mark := len(w.buf)
	w.buf = append(append(w.buf, 'k'), child.key...)
	values := 0
	if child.dataType != jsonparser.Array {
		if !w.value(child.data, child.dataType) {
			return false
		}
		values++
	} else {
		failed := false
		// Un null, o un número fuera de la gramática, no es un valor: ver childValues
		_, err := jsonparser.ArrayEach(child.data, func(value []byte, dataType jsonparser.ValueType, _ int, _ error) {
			dataType = readAs(value, dataType)
			if dataType == jsonparser.Null || failed {
				return
			}
			failed = !w.value(value, dataType)
			values++
		})
		if err != nil || failed {
			return false
		}
	}
	if values == 0 {
		w.buf = w.buf[:mark]
		return true
	}
	w.buf = append(w.buf, ';')
	return true
}

// value escribe la clave de un valor de un objeto, como lo lee equalValues.
func (w *keyWriter) value(data []byte, dataType jsonparser.ValueType) bool {
	w.buf = append(w.buf, ',')
	switch dataType {
	case jsonparser.Object:
		w.buf = append(w.buf, '{')
		ok := w.object(data)
		w.buf = append(w.buf, '}')
		return ok
	case jsonparser.Number:
		if n, ok := smallInteger(data); ok {
			w.integer(n)
			return true
		}
		d, err := decodeDecimal(string(data))
		if err != nil {
			// Solo es igual a lo escrito igual
			w.buf = append(append(w.buf, 'r'), data...)
			return true
		}
		w.decimal(d)
	case jsonparser.String:
		if bytes.IndexByte(data, '\\') < 0 {
			w.buf = append(append(w.buf, 's'), data...)
			return true
		}
		text, err := jsonparser.ParseString(data)
		if err != nil {
			w.buf = append(append(w.buf, 'r'), data...)
			return true
		}
		w.buf = append(append(w.buf, 's'), text...)
	case jsonparser.Array:
		w.buf = append(w.buf, '[')
	default:
		w.buf = append(append(w.buf, 'l'), data...)
	}
	return true
}

// smallInteger lee un número escrito solo con cifras y signo, que cabe en un
// int64 sin pasar por un decimal: la clave es la misma.
func smallInteger(data []byte) (int64, bool) {
	if len(data) == 0 || len(data) > 18 {
		return 0, false
	}
	for i, c := range data {
		if (c < '0' || c > '9') && (c != '-' || i != 0 || len(data) == 1) {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(string(data), 10, 64)
	return n, err == nil
}
