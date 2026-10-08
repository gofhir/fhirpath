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
// Only layout is set aside. Keys in another order, 1 and 1.0, or an escaped
// character and the character itself stay apart: one serializer writes a
// value's keys in one order — FHIR's, the order of the definition — and its
// numbers and strings alike throughout a document. And layout separates
// tokens: 1 2 is not 12.
//
// The texts are read a word at a time over their first headBytes bytes, where
// most pairs that differ do, as resources differ at their id. Texts of
// different lengths that agree that far, which comparing bytes told apart
// unread, are read from the end: a difference outside layout among their last
// endBytes bytes, as where two objects differ in a trailing field, tells them
// apart at once. Otherwise they are read on, a block at a time. Only a pair
// laid out differently is read a byte at a time. A pair of different lengths
// that agrees at both ends costs the bytes read up to its first difference.
func sameJSON(a, b []byte) bool {
	if len(a) == len(b) {
		// The same bytes in memory, as one object reached twice is: equal at
		// once, as bytes.Equal answers
		if len(a) == 0 || &a[0] == &b[0] {
			return true
		}
	}
	n := min(len(a), len(b))
	// Most pairs differ within their first words, as resources differ at their
	// id: read those first
	k := commonWords(a, b, 0, min(n, headBytes))
	if k < min(n, headBytes) {
		k = commonBytes(a, b, k, min(n, headBytes))
	}
	if k == min(n, headBytes) {
		// They agree that far. Texts of different lengths may differ in a
		// trailing field, which their ends tell at once; else read on
		if len(a) != len(b) && differAtEnd(a, b) {
			return false
		}
		k = commonPrefix(a, b, k)
	}
	if k == len(a) && k == len(b) {
		return true
	}
	if k < len(a) && k < len(b) && !isJSONSpace(a[k]) && !isJSONSpace(b[k]) {
		// The same text up to two different tokens' bytes
		return false
	}
	return equalIgnoringLayout(a, b)
}

// endBytes is how far from their ends differAtEnd reads two texts.
const endBytes = 64

// differAtEnd reports two texts whose last bytes, their trailing layout left
// out, differ outside layout within endBytes of the end. As with a first
// difference, the texts agree from there to the end, so the difference stands
// at the same place in both, and a difference outside layout is one in content.
func differAtEnd(a, b []byte) bool {
	i, j := len(a), len(b) // one past the last byte still to compare
	for i > 0 && isJSONSpace(a[i-1]) {
		i--
	}
	for j > 0 && isJSONSpace(b[j-1]) {
		j--
	}
	// Whole words first, eight bytes at a time, while they agree
	for n := 0; n < endBytes && i >= 8 && j >= 8; n += 8 {
		if binary.LittleEndian.Uint64(a[i-8:]) != binary.LittleEndian.Uint64(b[j-8:]) {
			break
		}
		i -= 8
		j -= 8
	}
	// Then the bytes of the word that does not, or of what is left
	for n := 0; n < 8 && i > 0 && j > 0; n++ {
		if c, d := a[i-1], b[j-1]; c != d {
			return !isJSONSpace(c) && !isJSONSpace(d)
		}
		i--
		j--
	}
	return false
}

// headBytes is how much of two texts sameJSON reads a word at a time before
// anything else: most pairs that differ do so within it.
const headBytes = 64

// commonPrefix returns the length of the longest prefix two texts share, from
// k on, comparing blocks with bytes.Equal, which is vectorized, before words
// and bytes.
func commonPrefix(a, b []byte, k int) int {
	n := min(len(a), len(b))
	for k+512 <= n && bytes.Equal(a[k:k+512], b[k:k+512]) {
		k += 512
	}
	for k+64 <= n && bytes.Equal(a[k:k+64], b[k:k+64]) {
		k += 64
	}
	return commonBytes(a, b, commonWords(a, b, k, n), n)
}

// commonBytes advances k past the bytes a and b share, up to limit.
func commonBytes(a, b []byte, k, limit int) int {
	for k < limit && a[k] == b[k] {
		k++
	}
	return k
}

// commonWords advances k past the eight-byte words a and b share, up to limit.
func commonWords(a, b []byte, k, limit int) int {
	for k+8 <= limit && binary.LittleEndian.Uint64(a[k:]) == binary.LittleEndian.Uint64(b[k:]) {
		k += 8
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
