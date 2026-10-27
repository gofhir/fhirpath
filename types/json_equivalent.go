package types

import (
	"bytes"

	"github.com/buger/jsonparser"
	"github.com/shopspring/decimal"
)

// equivalentJSON reports whether two JSON objects are equivalent: "For complex
// types, equivalence requires all child properties to be equivalent,
// recursively." Each child is read as navigation reads it and compared with
// the equivalence of its type — a string ignoring case, any whitespace
// character for any other, no run collapsed, a decimal as decimals are
// equivalent, a date as dates are — and a child that repeats is compared as ~
// compares collections, in any order.
//
// A child present on one side only is not equivalent, as an empty collection
// is not equivalent to a non-empty one; a null, or an array of nothing but
// nulls, counts as absent, as navigation reads nothing there. An element written as {} is read
// as an item, and is a child. The element beside a primitive that holds no
// value adds nothing to the primitive, as in the HL7 validator, and counts as
// absent too.
//
// The comparison reads the two texts and writes to no object: either may be
// shared with other goroutines. A child written alike on both sides is
// equivalent unread, and a string that is no date compares as written, so
// most pairs, alike or not, are told apart without reading a value.
func equivalentJSON(a, b []byte) bool {
	if sameJSON(a, b) {
		return true
	}
	left, ok := parseJSONObject(a)
	if !ok {
		return false
	}
	right, ok := parseJSONObject(b)
	return ok && equivalentObjects(&left, &right)
}

// equivalentObjects compares two objects' children, read once. A primitive
// and the element FHIR writes beside it, given and _given, are one child,
// whose items, each value with the element at its position, pair in any
// order: see equivalentNamed.
func equivalentObjects(left, right *jsonObject) bool {
	if left.hasElements() || right.hasElements() {
		return equivalentWithElements(left, right)
	}
	if len(left.children) != len(right.children) {
		return false
	}
	for i, l := range left.children {
		r, found := right.find(l.key, i)
		if !found || !equivalentChildren(l, r) {
			return false
		}
	}
	return true
}

// equivalentWithElements compares two objects either of which writes an
// element beside a primitive, by name as equalObjects does.
func equivalentWithElements(left, right *jsonObject) bool {
	for _, l := range left.children {
		name, isElement := childName(l.key)
		if isElement && left.has(name) {
			continue // compared with its primitive
		}
		if !left.has("_"+name) && !right.has("_"+name) {
			r, found := right.find(l.key, len(right.children))
			if !found || !equivalentChildren(l, r) {
				return false
			}
			continue
		}
		if !equivalentNamed(left, right, name) {
			return false
		}
	}
	for _, r := range right.children {
		name, _ := childName(r.key)
		if !left.has(name) && !left.has("_"+name) {
			return false
		}
	}
	return true
}

// equivalentNamed compares the child a name stands for on each side where
// either writes the element beside it: each item, a value with the element at
// its position, pairs with an unused equivalent item on the other side, in
// any order, as ~ between collections pairs. An extension on the second given
// is not one on the first.
func equivalentNamed(left, right *jsonObject, name string) bool {
	key, elementKey := []byte(name), []byte("_"+name)
	lv, lok := left.find(key, len(left.children))
	rv, rok := right.find(key, len(right.children))
	le, leok := left.find(elementKey, len(left.children))
	re, reok := right.find(elementKey, len(right.children))
	l, r := equivalenceItems(lv, lok, le, leok), equivalenceItems(rv, rok, re, reok)
	if len(l) != len(r) {
		return false
	}
	// Items written alike pair first, as pairInAnyOrder pairs values: with
	// decimals, whose equivalence is not transitive, pairing greedily from
	// the first would miss what pairing the values alone finds
	used := make([]bool, len(r))
	paired := make([]bool, len(l))
	for i, item := range l {
		for j, candidate := range r {
			if !used[j] && optionalAlike(item.value, candidate.value) &&
				optionalAlike(item.element, candidate.element) {
				used[j], paired[i] = true, true
				break
			}
		}
	}
	for i, item := range l {
		if paired[i] {
			continue
		}
		matched := false
		for j, candidate := range r {
			if !used[j] && equivalentOptional(item.value, candidate.value) &&
				equivalentOptional(item.element, candidate.element) {
				used[j], matched = true, true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// optionalAlike reports two values written alike, either of which may be
// absent.
func optionalAlike(left, right *childValue) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return writtenAlike(left.data, left.dataType, right.data, right.dataType)
}

// equivalenceItems lists a primitive's items as ~ reads them, each value with
// the element at its position. An element that holds no value adds nothing
// beside a value, as parseJSONObject has it, but beside a null it is the
// item, a primitive with no value, as in the HL7 validator; a null beside a
// null is no item.
func equivalenceItems(value jsonChild, valueFound bool, element jsonChild, elementFound bool) []pairedItem {
	values, elements := positionalValues(value, valueFound), positionalValues(element, elementFound)
	var items []pairedItem
	for i := range max(len(values), len(elements)) {
		var item pairedItem
		if i < len(values) && values[i].dataType != jsonparser.Null {
			item.value = &values[i]
		}
		if i < len(elements) && elements[i].dataType != jsonparser.Null &&
			(item.value == nil || !holdsNoValue(elements[i].data, elements[i].dataType)) {
			item.element = &elements[i]
		}
		if item.value != nil || item.element != nil {
			items = append(items, item)
		}
	}
	return items
}

// equivalentOptional compares two values either of which may be absent.
func equivalentOptional(left, right *childValue) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return writtenAlike(left.data, left.dataType, right.data, right.dataType) ||
		valuesEquivalent(*left, *right)
}

// jsonChild is one child property as written: its key, JSON text and type.
type jsonChild struct {
	key      []byte
	data     []byte
	dataType jsonparser.ValueType
}

// jsonObject is an object's children as written, with an index by key once it
// has more than a few: the other side's keys may stand in another order.
type jsonObject struct {
	children []jsonChild
	index    map[string]int
}

// indexedChildren is how many children an object has before its children are
// found by an index rather than by reading them in turn.
const indexedChildren = 16

// parseJSONObject reads the children of a JSON object in the order written,
// leaving out the ones navigation reads nothing from, a null and an array of
// nothing but nulls, and the element beside a primitive that adds nothing to its values:
// see addsNothing. A key written twice, which no valid resource has, is read
// where it first stands, as navigation reads it.
func parseJSONObject(data []byte) (jsonObject, bool) {
	object := jsonObject{children: make([]jsonChild, 0, 8)}
	besides := 0
	err := jsonparser.ObjectEach(data, func(key, value []byte, dataType jsonparser.ValueType, _ int) error {
		if _, seen := object.find(key, len(object.children)); seen {
			return nil
		}
		if dataType == jsonparser.Null || dataType == jsonparser.Array && nullsOnly(value) {
			return nil
		}
		if len(key) > 1 && key[0] == '_' {
			besides++
		}
		object.add(jsonChild{key: key, data: value, dataType: dataType})
		return nil
	})
	if err != nil || besides == 0 {
		return object, err == nil
	}

	// The element beside a primitive is told by the value beside it, which
	// the object may hold before it or after: looked up among the children
	// once they are all read, so that each is found in the index rather than
	// by reading the object again
	kept := jsonObject{children: make([]jsonChild, 0, len(object.children))}
	for _, child := range object.children {
		if len(child.key) > 1 && child.key[0] == '_' {
			value, _ := object.find(child.key[1:], len(object.children))
			if addsNothing(child, value) {
				continue
			}
		}
		kept.add(child)
	}
	return kept, true
}

// add appends a child, indexing the children once there are enough of them.
func (o *jsonObject) add(child jsonChild) {
	o.children = append(o.children, child)
	switch {
	case o.index != nil:
		o.index[string(child.key)] = len(o.children) - 1
	case len(o.children) > indexedChildren:
		o.index = make(map[string]int, 2*len(o.children))
		for i, c := range o.children {
			o.index[string(c.key)] = i
		}
	}
}

// find returns the child with a key, looking first where the other side has
// it, as one serializer writes keys in one order.
func (o *jsonObject) find(key []byte, at int) (jsonChild, bool) {
	if at < len(o.children) && bytes.Equal(o.children[at].key, key) {
		return o.children[at], true
	}
	if o.index != nil {
		i, ok := o.index[string(key)]
		if !ok {
			return jsonChild{}, false
		}
		return o.children[i], true
	}
	for _, child := range o.children {
		if bytes.Equal(child.key, key) {
			return child, true
		}
	}
	return jsonChild{}, false
}

// addsNothing reports an element beside a primitive, _given beside given,
// that holds no value and stands only beside values: given "J" with _given
// [{}] is given "J", as in the HL7 validator. One beside a null is the item
// there, and is kept. value is the primitive's child, the zero jsonChild
// where there is none; a null, or an array of nothing but nulls, is none.
func addsNothing(element, value jsonChild) bool {
	if !holdsNoValue(element.data, element.dataType) {
		return false
	}

	if element.dataType != jsonparser.Array {
		return value.key != nil && value.dataType != jsonparser.Array
	}

	var values []jsonparser.ValueType
	if value.key != nil && value.dataType == jsonparser.Array {
		//nolint:errcheck // An array that does not parse stands beside nothing
		jsonparser.ArrayEach(value.data, func(_ []byte, dataType jsonparser.ValueType, _ int, _ error) {
			values = append(values, dataType)
		})
	}
	beside := true
	i := 0
	//nolint:errcheck // An array that does not parse holds no value, as holdsNoValue found
	jsonparser.ArrayEach(element.data, func(_ []byte, dataType jsonparser.ValueType, _ int, _ error) {
		if dataType != jsonparser.Null && (i >= len(values) || values[i] == jsonparser.Null) {
			beside = false
		}
		i++
	})
	return beside
}

// nullsOnly reports a JSON array whose elements, if any, are all null.
func nullsOnly(data []byte) bool {
	nulls := true
	//nolint:errcheck // An array that does not parse is compared as written
	jsonparser.ArrayEach(data, func(_ []byte, dataType jsonparser.ValueType, _ int, _ error) {
		if dataType != jsonparser.Null {
			nulls = false
		}
	})
	return nulls
}

// equivalentChildren compares two child properties, each a single value or an
// array, as collections: a single value is a collection of one.
func equivalentChildren(left, right jsonChild) bool {
	if writtenAlike(left.data, left.dataType, right.data, right.dataType) {
		return true
	}
	if left.dataType != jsonparser.Array && right.dataType != jsonparser.Array {
		return rawEquivalent(left.data, left.dataType, right.data, right.dataType)
	}
	l, ok := childValues(left)
	if !ok {
		return false
	}
	r, ok := childValues(right)
	return ok && len(l) == len(r) && pairInAnyOrder(l, r)
}

// pairInAnyOrder reports whether each value on the left pairs with an unused
// equivalent one on the right, taking the first that is, as ~ between
// collections does. Values written alike pair first, which reads nothing, so
// repeats in another order pair unread. Pairing is greedy: where equivalence
// is not transitive, as between decimals, which round, an order that pairs
// differently can answer differently.
func pairInAnyOrder(l, r []childValue) bool {
	used := make([]bool, len(r))
	paired := make([]bool, len(l))
	for i, item := range l {
		for j, candidate := range r {
			if !used[j] && writtenAlike(item.data, item.dataType, candidate.data, candidate.dataType) {
				used[j], paired[i] = true, true
				break
			}
		}
	}
	// Objects met again and again are read once each
	leftObjects := make([]*jsonObject, len(l))
	rightObjects := make([]*jsonObject, len(r))
	for i, item := range l {
		if paired[i] {
			continue
		}
		matched := false
		for j, candidate := range r {
			if used[j] {
				continue
			}
			var same bool
			if item.dataType == jsonparser.Object && candidate.dataType == jsonparser.Object {
				lo, lok := objectAt(leftObjects, i, item.data)
				ro, rok := objectAt(rightObjects, j, candidate.data)
				same = lok && rok && equivalentObjects(lo, ro)
			} else {
				same = valuesEquivalent(item, candidate)
			}
			if same {
				used[j], matched = true, true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// objectAt returns the object at a place, read the first time it is asked
// for.
func objectAt(objects []*jsonObject, at int, data []byte) (*jsonObject, bool) {
	if objects[at] == nil {
		object, ok := parseJSONObject(data)
		if !ok {
			return nil, false
		}
		objects[at] = &object
	}
	return objects[at], true
}

// valuesEquivalent compares two values of a repeating child that are not
// written alike. A missing place in a primitive array matches only another,
// which pairing written alike has already done, so here it matches nothing.
func valuesEquivalent(left, right childValue) bool {
	if left.dataType == jsonparser.Null || right.dataType == jsonparser.Null {
		return false
	}
	return rawEquivalent(left.data, left.dataType, right.data, right.dataType)
}

// writtenAlike reports two values written alike: a string as the same text,
// anything else as the same JSON apart from layout. A string's text is all
// content, its spaces included, so it is compared byte for byte.
func writtenAlike(a []byte, aType jsonparser.ValueType, b []byte, bType jsonparser.ValueType) bool {
	if aType != bType {
		return false
	}
	if aType == jsonparser.String {
		return bytes.Equal(a, b)
	}
	return sameJSON(a, b)
}

// rawEquivalent compares two single values from their JSON. Two strings that
// are no dates, in ASCII and without escapes, compare as written, which is what
// reading them would compare; anything else is read first, an object compared
// child by child in turn.
func rawEquivalent(a []byte, aType jsonparser.ValueType, b []byte, bType jsonparser.ValueType) bool {
	if aType == jsonparser.String && bType == jsonparser.String &&
		!looksTemporalBytes(a) && !looksTemporalBytes(b) {
		if equivalent, decided := equivalentASCII(a, b); decided {
			return equivalent
		}
	}
	return valueEquivalent(jsonValue(a, aType, false), jsonValue(b, bType, false))
}

// looksTemporalBytes is looksTemporal over a string's JSON text.
func looksTemporalBytes(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	for _, c := range data[:4] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// equivalentASCII compares two strings as String.Equivalent does, a letter in
// either case and any whitespace for any other, without reading them into
// strings. It decides only texts in ASCII without escapes, where a byte is a
// character; for any other it reports false, undecided.
func equivalentASCII(a, b []byte) (equivalent, decided bool) {
	for _, text := range [][]byte{a, b} {
		for _, c := range text {
			if c >= 0x80 || c == '\\' {
				return false, false
			}
		}
	}
	if len(a) != len(b) {
		return false, true
	}
	for i := range a {
		if !asciiEquivalent(a[i], b[i]) {
			return false, true
		}
	}
	return true, true
}

// childValue is one value of a child property, as written.
type childValue struct {
	data     []byte
	dataType jsonparser.ValueType
}

// childValues returns the values of a child property, an array's elements or
// the single value; reading them is left to the comparison, which reads only
// what it must. A null item is left out, as navigation reads nothing there;
// one with an element beside it is compared where the element is.
func childValues(child jsonChild) ([]childValue, bool) {
	if child.dataType != jsonparser.Array {
		return []childValue{{child.data, child.dataType}}, true
	}
	var values []childValue
	_, err := jsonparser.ArrayEach(child.data, func(value []byte, dataType jsonparser.ValueType, _ int, _ error) {
		if dataType == jsonparser.Null {
			return
		}
		values = append(values, childValue{value, dataType})
	})
	return values, err == nil
}

// valueEquivalent compares two values read from JSON; a value that does not
// read is equivalent to nothing.
func valueEquivalent(left, right Value) bool {
	return left != nil && right != nil && left.Equivalent(right)
}

// equalJSON compares two JSON objects as = does: "For complex types,
// equality requires all child properties to be equal, recursively." A child is
// found by its key, wherever the object writes it, as fhirpath.js and HL7's
// Java engine both find it; a child that repeats is compared in order, "Comparison
// is order dependent"; a number is compared as a decimal, "trailing zeroes
// after the decimal are ignored", so 1 and 1.0 are equal, as fhirpath.js has
// it; a string is compared as it reads, an escaped character as the character
// itself. The children navigation reads nothing from are left out, as ~
// leaves them: see parseJSONObject.
//
// Texts alike apart from their layout are equal unread, which is how most
// pairs that are equal compare, and a pair whose keys stand in one order and
// that differs in a value is unequal unread, which is how most pairs that
// differ compare: see orderedEqual. Only the rest, keys in another order among
// them, are read child by child.
func equalJSON(a, b []byte) bool {
	if sameJSON(a, b) {
		return true
	}
	if equal, decided := orderedEqualFromDifference(a, b); decided {
		return equal
	}
	left, ok := parseJSONObject(a)
	if !ok {
		return false
	}
	right, ok := parseJSONObject(b)
	return ok && equalObjects(&left, &right)
}

// equalObjects compares two parsed objects child by child, by name. A
// primitive and the element FHIR writes beside it, given and _given, are one
// child: each value is compared with the element at its own position, so
// that an extension on the second given is not one on the first.
func equalObjects(left, right *jsonObject) bool {
	for _, l := range left.children {
		name, isElement := childName(l.key)
		if isElement && left.has(name) {
			continue // compared with its primitive
		}
		if !equalNamed(left, right, name) {
			return false
		}
	}
	for _, r := range right.children {
		name, isElement := childName(r.key)
		if isElement && right.has(name) {
			continue
		}
		if !left.has(name) && !left.has("_"+name) {
			return false
		}
	}
	return true
}

// childName is the name a key stands for: given for given and for _given,
// which is its element.
func childName(key []byte) (name string, isElement bool) {
	if len(key) > 1 && key[0] == '_' {
		return string(key[1:]), true
	}
	return string(key), false
}

// hasElements reports an object that writes an element beside a primitive,
// a key starting with an underscore.
func (o *jsonObject) hasElements() bool {
	for _, child := range o.children {
		if len(child.key) > 1 && child.key[0] == '_' {
			return true
		}
	}
	return false
}

// has reports a child of an object by its key.
func (o *jsonObject) has(key string) bool {
	_, found := o.find([]byte(key), len(o.children))
	return found
}

// equalNamed compares the child a name stands for on each side: its values,
// and the elements beside them where either side writes any.
func equalNamed(left, right *jsonObject, name string) bool {
	key, elementKey := []byte(name), []byte("_"+name)
	lv, lok := left.find(key, len(left.children))
	rv, rok := right.find(key, len(right.children))
	le, leok := left.find(elementKey, len(left.children))
	re, reok := right.find(elementKey, len(right.children))
	if !leok && !reok {
		return lok && rok && equalChildren(lv, rv)
	}
	l, r := pairedItems(lv, lok, le, leok), pairedItems(rv, rok, re, reok)
	if len(l) != len(r) {
		return false
	}
	for i := range l {
		if !equalOptional(l[i].value, r[i].value) || !equalOptional(l[i].element, r[i].element) {
			return false
		}
	}
	return true
}

// pairedItem is a primitive's value and the element beside it at one
// position, either nil where none is written.
type pairedItem struct {
	value, element *childValue
}

// pairedItems lists a primitive's items, each value with the element at its
// position; a position with neither, a null beside a null or an element that
// holds no value, is no item, as navigation reads it.
func pairedItems(value jsonChild, valueFound bool, element jsonChild, elementFound bool) []pairedItem {
	values, elements := positionalValues(value, valueFound), positionalValues(element, elementFound)
	var items []pairedItem
	for i := range max(len(values), len(elements)) {
		var item pairedItem
		if i < len(values) && values[i].dataType != jsonparser.Null {
			item.value = &values[i]
		}
		if i < len(elements) && !holdsNoValue(elements[i].data, elements[i].dataType) {
			item.element = &elements[i]
		}
		if item.value != nil || item.element != nil {
			items = append(items, item)
		}
	}
	return items
}

// positionalValues lists a child's values by position, nulls kept, a single
// value as one.
func positionalValues(child jsonChild, found bool) []childValue {
	switch {
	case !found:
		return nil
	case child.dataType != jsonparser.Array:
		return []childValue{{child.data, child.dataType}}
	}
	var values []childValue
	//nolint:errcheck // An array that does not parse lists what it read
	jsonparser.ArrayEach(child.data, func(value []byte, dataType jsonparser.ValueType, _ int, _ error) {
		values = append(values, childValue{value, dataType})
	})
	return values
}

// equalOptional compares two values either of which may be absent.
func equalOptional(left, right *childValue) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return equalValues(*left, *right)
}

// equalChildren compares a child's values on each side, in order.
func equalChildren(left, right jsonChild) bool {
	if writtenAlike(left.data, left.dataType, right.data, right.dataType) {
		return true
	}
	l, ok := childValues(left)
	if !ok {
		return false
	}
	r, ok := childValues(right)
	if !ok || len(l) != len(r) {
		return false
	}
	for i := range l {
		if !equalValues(l[i], r[i]) {
			return false
		}
	}
	return true
}

// equalValues compares two values of a child as = does.
func equalValues(left, right childValue) bool {
	if left.dataType != right.dataType {
		return false
	}
	switch left.dataType {
	case jsonparser.Object:
		return equalJSON(left.data, right.data)
	case jsonparser.Number:
		l, lerr := decimal.NewFromString(string(left.data))
		r, rerr := decimal.NewFromString(string(right.data))
		return lerr == nil && rerr == nil && CompareDecimals(l, r) == 0
	case jsonparser.String:
		if bytes.Equal(left.data, right.data) {
			return true
		}
		l, lerr := jsonparser.ParseString(left.data)
		r, rerr := jsonparser.ParseString(right.data)
		return lerr == nil && rerr == nil && l == r
	}
	return sameJSON(left.data, right.data)
}
