package types

import (
	"bytes"

	"github.com/buger/jsonparser"
)

// equivalentJSON reports whether two JSON objects are equivalent: "For complex
// types, equivalence requires all child properties to be equivalent,
// recursively." Each child is read as navigation reads it and compared with
// the equivalence of its type — a string ignoring case and leading and
// trailing whitespace, with whitespace normalized, a decimal as decimals are
// equivalent, a date by its value — and a child that repeats is compared as ~
// compares collections, in any order.
//
// A child present on one side only is not equivalent, as an empty collection
// is not equivalent to a non-empty one; an empty array or a null counts as
// absent, as navigation reads nothing there.
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

// equivalentObjects compares two objects' children, read once.
func equivalentObjects(left, right *jsonObject) bool {
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
// leaving out the ones navigation reads nothing from: a null and an empty
// array. A key written twice, which no valid resource has, is read where it
// first stands, as navigation reads it.
func parseJSONObject(data []byte) (jsonObject, bool) {
	object := jsonObject{children: make([]jsonChild, 0, 8)}
	err := jsonparser.ObjectEach(data, func(key, value []byte, dataType jsonparser.ValueType, _ int) error {
		if _, seen := object.find(key, len(object.children)); seen {
			return nil
		}
		switch dataType {
		case jsonparser.Null:
			return nil
		case jsonparser.Array:
			if isEmptyArray(value) {
				return nil
			}
		}
		object.add(jsonChild{key: key, data: value, dataType: dataType})
		return nil
	})
	return object, err == nil
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

// isEmptyArray reports a JSON array with no elements.
func isEmptyArray(data []byte) bool {
	empty := true
	//nolint:errcheck // An array that does not parse is compared as written
	jsonparser.ArrayEach(data, func([]byte, jsonparser.ValueType, int, error) {
		empty = false
	})
	return empty
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

// valuesEquivalent compares two values of a repeating child, a missing place
// in a primitive array matching only another.
func valuesEquivalent(left, right childValue) bool {
	leftMissing, rightMissing := left.dataType == jsonparser.Null, right.dataType == jsonparser.Null
	if leftMissing || rightMissing {
		return leftMissing && rightMissing
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

// equivalentASCII compares two strings as String.Equivalent does — case,
// leading and trailing whitespace and runs of whitespace set aside — without
// reading them into strings. It decides only texts in ASCII without escapes,
// where a byte is a character; for any other it reports false, undecided.
func equivalentASCII(a, b []byte) (equivalent, decided bool) {
	for _, text := range [][]byte{a, b} {
		for _, c := range text {
			if c >= 0x80 || c == '\\' {
				return false, false
			}
		}
	}
	a, b = trimASCIISpace(a), trimASCIISpace(b)
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ca, cb := a[i], b[j]
		if isASCIISpace(ca) && isASCIISpace(cb) {
			// A run of whitespace on each side is one space
			for i < len(a) && isASCIISpace(a[i]) {
				i++
			}
			for j < len(b) && isASCIISpace(b[j]) {
				j++
			}
			continue
		}
		if lowerASCII(ca) != lowerASCII(cb) {
			return false, true
		}
		i++
		j++
	}
	return i == len(a) && j == len(b), true
}

// isASCIISpace reports the ASCII characters unicode.IsSpace counts as space.
func isASCIISpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}

// trimASCIISpace trims ASCII whitespace from both ends.
func trimASCIISpace(s []byte) []byte {
	for len(s) > 0 && isASCIISpace(s[0]) {
		s = s[1:]
	}
	for len(s) > 0 && isASCIISpace(s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}

// lowerASCII lowercases an ASCII letter.
func lowerASCII(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// childValue is one value of a child property, as written.
type childValue struct {
	data     []byte
	dataType jsonparser.ValueType
}

// childValues returns the values of a child property, an array's elements or
// the single value; reading them is left to the comparison, which reads only
// what it must.
func childValues(child jsonChild) ([]childValue, bool) {
	if child.dataType != jsonparser.Array {
		return []childValue{{child.data, child.dataType}}, true
	}
	var values []childValue
	_, err := jsonparser.ArrayEach(child.data, func(value []byte, dataType jsonparser.ValueType, _ int, _ error) {
		values = append(values, childValue{value, dataType})
	})
	return values, err == nil
}

// valueEquivalent compares two values read from JSON; a value that does not
// read is equivalent to nothing.
func valueEquivalent(left, right Value) bool {
	return left != nil && right != nil && left.Equivalent(right)
}
