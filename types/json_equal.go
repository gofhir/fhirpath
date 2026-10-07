package types

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/buger/jsonparser"
)

// sameJSON reports whether two JSON values hold the same content, however each
// was written: "For complex types, equality requires all child properties to
// be equal, recursively." Keys may come in any order and whitespace between
// tokens is layout; arrays keep their order, and a string or number is the same
// only as written, so 1 and 1.0, or \u00b5 and µ, stay apart — one serializer
// writes them alike throughout a document.
//
// Equality is asked of every pair in a union, distinct() or in, and nearly
// every pair differs, so the answer must cost a differing pair no more than
// comparing bytes did. The texts are compared token by token, skipping layout,
// up to the first byte that differs. Where that byte falls decides: in a value,
// the same key holds another value; in the structure, one object has a key the
// other lacks; either way they differ. Only a difference in the name of a key
// can be keys in another order, and only then are the objects walked key by key.
func sameJSON(a, b []byte) bool {
	if bytes.Equal(a, b) {
		return true
	}
	switch firstDifference(a, b) {
	case differInValue:
		return false
	case differInKey:
		return sameReordered(a, b)
	}
	equal, inKey := compareIgnoringLayout(a, b)
	if equal {
		return true
	}
	if !inKey {
		return false
	}
	return sameReordered(a, b)
}

// sameReordered compares two objects whose keys may be in another order, key
// by key. It holds to texts it can read exactly as written: an escape, which
// the parser undoes in a key's name, or a key written twice, leaves the two
// apart unless they differed only in layout. What it finds the same then holds
// the same bytes in another order, which jsonFingerprint relies on.
func sameReordered(a, b []byte) bool {
	if bytes.IndexByte(a, '\\') >= 0 || bytes.IndexByte(b, '\\') >= 0 {
		return false
	}
	return sameValue(a, jsonparser.Object, b, jsonparser.Object)
}

// difference classifies where two texts first differ, as far as can be told
// without walking them.
type difference int

const (
	differUnknown difference = iota // layout, an escape, or one text ends: walk them
	differInValue                   // in a value or in the structure: they differ
	differInKey                     // in the name of a key: keys may be in another order
)

// firstDifference classifies the first byte at which two texts differ, for the
// common case of two values written by one serializer: no layout differs before
// it. The texts agree up to it, so it sits at the same place in both, and only
// one place can hide keys in another order: the name of a key. A key's name is
// followed, past its closing quote, by a colon, so the next quote in a, and
// what follows it, tell a key from anything else without reading the prefix.
//
// Reading a key where there is none only sends the pair to the exact walk; the
// one mistake that would matter, missing a key, cannot happen, since inside a
// key the next quote is the one that closes it. An escape before that quote is
// left to compareIgnoringLayout.
func firstDifference(a, b []byte) difference {
	k := commonPrefix(a, b)
	if k == len(a) || k == len(b) || isLayout(a[k]) || isLayout(b[k]) {
		return differUnknown
	}
	end := bytes.IndexByte(a[k:], '"')
	if end < 0 {
		return differInValue
	}
	if bytes.IndexByte(a[k:k+end], '\\') >= 0 {
		return differUnknown
	}
	for _, c := range a[k+end+1:] {
		if !isLayout(c) {
			if c == ':' {
				return differInKey
			}
			return differInValue
		}
	}
	return differInValue
}

// commonPrefix returns the length of the longest prefix two texts share.
func commonPrefix(a, b []byte) int {
	n := min(len(a), len(b))
	k := 0
	for k+8 <= n && binary.LittleEndian.Uint64(a[k:]) == binary.LittleEndian.Uint64(b[k:]) {
		k += 8
	}
	for k < n && a[k] == b[k] {
		k++
	}
	return k
}

// isLayout reports the whitespace JSON allows between tokens.
func isLayout(c byte) bool {
	return c == ' ' || c == '\n' || c == '\r' || c == '\t'
}

// maxTrackedDepth bounds the containers compareIgnoringLayout tracks, one bit
// each. Deeper than that, a difference is taken as possibly in a key, which
// only costs the walk.
const maxTrackedDepth = 64

// layoutScan is where a JSON text stands, read a byte at a time: inside a
// string or not, at a key or a value, in which containers.
type layoutScan struct {
	objects           uint64 // bit d set: the container at depth d is an object
	depth             int
	inString, escaped bool
	isKey, expectKey  bool
}

// inKeyName reports whether the scan stands inside the name of a key, or deeper
// than it can tell.
func (s *layoutScan) inKeyName() bool {
	return (s.inString && s.isKey) || s.depth > maxTrackedDepth
}

// step reads one byte. It reports false for a container closed that was never
// opened, which is not JSON.
func (s *layoutScan) step(c byte) bool {
	if s.inString {
		switch {
		case s.escaped:
			s.escaped = false
		case c == '\\':
			s.escaped = true
		case c == '"':
			s.inString, s.isKey = false, false
		}
		return true
	}
	switch c {
	case '"':
		s.inString, s.isKey, s.expectKey = true, s.expectKey, false
	case '{', '[':
		if s.depth < maxTrackedDepth {
			bit := uint64(1) << uint(s.depth)
			if c == '{' {
				s.objects |= bit
			} else {
				s.objects &^= bit
			}
		}
		s.depth++
		s.expectKey = c == '{'
	case '}', ']':
		if s.depth == 0 {
			return false
		}
		s.depth--
		s.expectKey = false
	case ',':
		s.expectKey = s.depth > 0 && s.depth <= maxTrackedDepth && s.objects&(1<<uint(s.depth-1)) != 0
	default:
		s.expectKey = false
	}
	return true
}

// compareIgnoringLayout compares two JSON texts byte for byte, skipping the
// whitespace between tokens but not inside strings. It reports whether they
// are equal, and if not, whether the first difference falls in the name of a
// key. While the two agree they share one scan.
func compareIgnoringLayout(a, b []byte) (equal, inKey bool) {
	var scan layoutScan
	i, j := 0, 0
	for {
		if !scan.inString {
			i = skipLayout(a, i)
			j = skipLayout(b, j)
		}
		if i == len(a) || j == len(b) {
			return i == len(a) && j == len(b), false
		}
		if a[i] != b[j] {
			return false, scan.inKeyName()
		}
		if !scan.step(a[i]) {
			// More closed than opened: not JSON, and not the same as anything
			return false, false
		}
		i++
		j++
	}
}

// skipLayout returns the first index at or after i that is not layout.
func skipLayout(data []byte, i int) int {
	for i < len(data) && isLayout(data[i]) {
		i++
	}
	return i
}

// errDiffer stops a walk at the first difference.
var errDiffer = errors.New("values differ")

// sameValue compares two JSON values of the given types by content. A string
// or a number is compared as written.
func sameValue(a []byte, at jsonparser.ValueType, b []byte, bt jsonparser.ValueType) bool {
	if at != bt {
		return false
	}
	switch at {
	case jsonparser.Object:
		return sameObject(a, b)
	case jsonparser.Array:
		return sameArray(a, b)
	default:
		return bytes.Equal(a, b)
	}
}

// sameObject compares two objects key by key, in whatever order each holds
// them: each side's keys are looked up in the other, values included. An
// object that writes a key twice is the same as none here.
func sameObject(a, b []byte) bool {
	return !hasDuplicateKey(a) && !hasDuplicateKey(b) && keysFoundIn(a, b) && keysFoundIn(b, a)
}

// hasDuplicateKey reports an object that writes a key twice, or that cannot
// be read as an object at all, which keeps it apart as surely.
func hasDuplicateKey(data []byte) bool {
	seen := map[string]bool{}
	err := jsonparser.ObjectEach(data, func(key, _ []byte, _ jsonparser.ValueType, _ int) error {
		if seen[string(key)] {
			return errDiffer
		}
		seen[string(key)] = true
		return nil
	})
	return err != nil
}

// keysFoundIn reports whether every key of a holds, in b, the same value.
func keysFoundIn(a, b []byte) bool {
	err := jsonparser.ObjectEach(a, func(key, value []byte, valueType jsonparser.ValueType, _ int) error {
		other, otherType, _, err := jsonparser.Get(b, string(key))
		if err != nil || !sameValue(value, valueType, other, otherType) {
			return errDiffer
		}
		return nil
	})
	return err == nil
}

// sameArray compares two arrays item by item, in order.
func sameArray(a, b []byte) bool {
	type item struct {
		data      []byte
		valueType jsonparser.ValueType
	}
	collect := func(data []byte) ([]item, bool) {
		var items []item
		_, err := jsonparser.ArrayEach(data, func(value []byte, valueType jsonparser.ValueType, _ int, err error) {
			items = append(items, item{value, valueType})
		})
		return items, err == nil
	}
	left, lok := collect(a)
	right, rok := collect(b)
	if !lok || !rok || len(left) != len(right) {
		return false
	}
	for i := range left {
		if !sameValue(left[i].data, left[i].valueType, right[i].data, right[i].valueType) {
			return false
		}
	}
	return true
}

// fingerprintTable gives each byte a fixed 64-bit value, spread by splitmix64,
// so that a sum of them tells apart texts that hold different bytes.
var fingerprintTable = func() (table [256]uint64) {
	x := uint64(0x9e3779b97f4a7c15)
	for i := range table {
		x += 0x9e3779b97f4a7c15
		z := x
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		table[i] = z ^ (z >> 31)
	}
	return table
}()

// jsonFingerprint sums the values of a JSON text's bytes, leaving out the
// whitespace between tokens. A sum does not depend on order, so texts that
// sameJSON finds the same — laid out apart, or with keys in another order —
// always share a fingerprint, and texts that do not share one differ. Two
// that share one may still differ, which only means comparing them.
func jsonFingerprint(data []byte) uint64 {
	// Compact text, which is most of it, has no layout to leave out: one tight
	// pass, and the careful one only where a byte could be layout
	var sum uint64
	spaced := false
	for _, c := range data {
		sum += fingerprintTable[c]
		spaced = spaced || c <= ' '
	}
	if !spaced {
		return sum
	}

	sum = 0
	inString, escaped := false, false
	for _, c := range data {
		switch {
		case escaped:
			escaped = false
		case inString && c == '\\':
			escaped = true
		case c == '"':
			inString = !inString
		case !inString && isLayout(c):
			continue
		}
		sum += fingerprintTable[c]
	}
	return sum
}
