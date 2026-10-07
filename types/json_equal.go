package types

import (
	"bytes"
	"encoding/binary"
)

// sameJSON reports whether two JSON texts hold the same content as written,
// apart from the whitespace between tokens: "For complex types, equality
// requires all child properties to be equal, recursively." A document read
// from one serializer writes the same value the same way wherever it stands,
// but not at the same indentation, and comparing bytes made two identical
// Codings at two depths unequal.
//
// Only layout is set aside. Keys in another order, 1 and 1.0, or µ and µ
// stay apart: one serializer writes a value's keys in one order — FHIR's,
// the order of the definition — and its numbers and strings alike throughout a
// document. And layout separates tokens: 1 2 is not 12.
//
// Equality is asked of every pair in a union, distinct(), intersect() or in,
// and nearly every pair differs, so a differing pair must cost no more than
// comparing bytes did. The texts are compared eight bytes at a time up to the
// first that differs; unless layout stands there, they differ, and only a pair
// laid out differently is read a byte at a time.
func sameJSON(a, b []byte) bool {
	if bytes.Equal(a, b) {
		return true
	}
	k := commonPrefix(a, b)
	if k < len(a) && k < len(b) && !isJSONSpace(a[k]) && !isJSONSpace(b[k]) {
		// The same text up to two different tokens' bytes
		return false
	}
	return equalIgnoringLayout(a, b)
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

// isTokenByte reports a byte of a number or a literal, which layout separates
// from the next: 1 2 is two numbers, 12 one.
func isTokenByte(c byte) bool {
	switch c {
	case '{', '}', '[', ']', ',', ':', '"':
		return false
	}
	return !isJSONSpace(c)
}

// equalIgnoringLayout compares two JSON texts byte for byte, skipping the
// whitespace between tokens but not inside strings. While the two agree they
// are inside a string or outside one together, so one state serves both.
// Layout on one side only may stand between punctuation, never between two
// bytes of a number or a literal, where it would split one token into two.
func equalIgnoringLayout(a, b []byte) bool {
	i, j := 0, 0
	inString, escaped := false, false
	previous := byte(',') // nothing yet, which layout separates from nothing
	for {
		if !inString {
			ii, jj := skipLayout(a, i), skipLayout(b, j)
			if splitsToken(previous, ii > i, jj > j, a, ii) {
				return false
			}
			i, j = ii, jj
		}
		if i == len(a) || j == len(b) {
			return i == len(a) && j == len(b)
		}
		c := a[i]
		if c != b[j] {
			return false
		}
		switch {
		case escaped:
			escaped = false
		case inString && c == '\\':
			escaped = true
		case c == '"':
			inString = !inString
		}
		previous = c
		i++
		j++
	}
}

// splitsToken reports layout on one side only that stands between two bytes of
// a number or a literal, after previous and before a[next]: there it splits
// one token into two, which the other side keeps whole.
func splitsToken(previous byte, spacedA, spacedB bool, a []byte, next int) bool {
	return spacedA != spacedB && isTokenByte(previous) && next < len(a) && isTokenByte(a[next])
}

// skipLayout returns the first index at or after i that is not layout.
func skipLayout(data []byte, i int) int {
	for i < len(data) && isJSONSpace(data[i]) {
		i++
	}
	return i
}
