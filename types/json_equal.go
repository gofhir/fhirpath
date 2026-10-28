package types

import (
	"bytes"
	"encoding/binary"
	"math/bits"
	"strings"
)

// sameJSON reports whether two JSON texts hold the same content as written,
// apart from the whitespace between tokens. It is equalJSON's first fast
// path: a document read from one serializer writes the same value the same
// way wherever it stands, but not at the same indentation, so most values
// that are equal compare alike here. Keys in another order, 1 and 1.0, or an
// escaped character and the character itself are not alike as written, and
// equalJSON reads them on. Layout separates tokens: 1 2 is not 12.
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

// orderedEqual compares two JSON texts token by token while their keys stand
// in the same order, which is how one serializer writes two values of one
// type, and decides what that walk can: a pair whose keys agree so far and
// that first differs in a value, a string, a number that is another number,
// a Boolean, or a value of another kind, is unequal, read no further and
// without building anything. A pair whose texts agree to the end, apart from
// numbers written differently, is equal. Anything else leaves it undecided,
// for equalJSON to read child by child: keys in another order, an escape in a
// string that differs, a null or an array where the other side has something
// else, which navigation may read alike.
func orderedEqual(a, b []byte) (equal, decided bool) {
	i, j := 0, 0
	for {
		i, j = skipJSONSpace(a, i), skipJSONSpace(b, j)
		if i == len(a) || j == len(b) {
			done := i == len(a) && j == len(b)
			return done, done
		}
		var verdict tokenVerdict
		i, j, verdict = compareTokens(a, i, b, j)
		switch verdict {
		case tokensUnequal:
			return false, true
		case tokensUndecided:
			return false, false
		}
	}
}

// tokenVerdict is what comparing one token on each side tells orderedEqual.
type tokenVerdict int

const (
	tokensAlike tokenVerdict = iota
	tokensUnequal
	tokensUndecided
)

// compareTokens compares the tokens that start at i in a and at j in b, and
// returns the offsets past them.
func compareTokens(a []byte, i int, b []byte, j int) (nextA, nextB int, verdict tokenVerdict) {
	ca, cb := a[i], b[j]
	switch {
	case ca == '"' && cb == '"':
		return compareStrings(a, i, b, j)
	case isNumberStart(ca) && isNumberStart(cb):
		return compareNumbers(a, i, b, j)
	case isLiteralStart(ca) && isLiteralStart(cb):
		return compareLiterals(a, i, b, j)
	case ca == cb && strings.IndexByte("{}[],:", ca) >= 0:
		return i + 1, j + 1, tokensAlike
	case isScalarStart(ca) && isScalarStart(cb) && !startsNull(a, i) && !startsNull(b, j):
		// A string against a number or a Boolean, at a key both have
		return i, j, tokensUnequal
	}
	return i, j, tokensUndecided
}

// compareStrings compares two strings. Keys that differ may be keys in
// another order, and a string with an escape may read as the other: both are
// left undecided. Two values that differ otherwise are unequal.
func compareStrings(a []byte, i int, b []byte, j int) (nextA, nextB int, verdict tokenVerdict) {
	ea, eb := stringEnd(a, i), stringEnd(b, j)
	if ea < 0 || eb < 0 {
		return i, j, tokensUndecided
	}
	keyA, keyB := followedByColon(a, ea), followedByColon(b, eb)
	if keyA != keyB {
		return i, j, tokensUndecided
	}
	if bytes.Equal(a[i:ea], b[j:eb]) {
		return ea, eb, tokensAlike
	}
	if keyA || bytes.IndexByte(a[i:ea], '\\') >= 0 || bytes.IndexByte(b[j:eb], '\\') >= 0 {
		return i, j, tokensUndecided
	}
	return i, j, tokensUnequal
}

// compareNumbers compares two numbers as decimals: 1 and 1.0 are alike.
func compareNumbers(a []byte, i int, b []byte, j int) (nextA, nextB int, verdict tokenVerdict) {
	ea, eb := numberEnd(a, i), numberEnd(b, j)
	if bytes.Equal(a[i:ea], b[j:eb]) {
		return ea, eb, tokensAlike
	}
	// Read as a resource holds them, one beyond the bound too, and compared
	// without rescaling
	l, lerr := decodeDecimal(string(a[i:ea]))
	r, rerr := decodeDecimal(string(b[j:eb]))
	switch {
	case lerr != nil || rerr != nil:
		return i, j, tokensUndecided
	case CompareDecimals(l, r) != 0:
		return i, j, tokensUnequal
	}
	return ea, eb, tokensAlike
}

// compareLiterals compares true, false and null. A null against something
// else may be no child against none: left undecided.
func compareLiterals(a []byte, i int, b []byte, j int) (nextA, nextB int, verdict tokenVerdict) {
	ea, eb := literalEnd(a, i), literalEnd(b, j)
	switch {
	case bytes.Equal(a[i:ea], b[j:eb]):
		return ea, eb, tokensAlike
	case startsNull(a, i) || startsNull(b, j):
		return i, j, tokensUndecided
	}
	return i, j, tokensUnequal
}

// followedByColon reports a string that ends at end and is a key.
func followedByColon(data []byte, end int) bool {
	k := skipJSONSpace(data, end)
	return k < len(data) && data[k] == ':'
}

func isNumberStart(c byte) bool  { return c == '-' || c >= '0' && c <= '9' }
func isLiteralStart(c byte) bool { return c == 't' || c == 'f' || c == 'n' }
func isScalarStart(c byte) bool  { return c == '"' || isNumberStart(c) || isLiteralStart(c) }

func startsNull(data []byte, i int) bool { return bytes.HasPrefix(data[i:], []byte("null")) }

// numberEnd returns the offset just past the JSON number that starts at i.
func numberEnd(data []byte, i int) int {
	for i < len(data) && strings.IndexByte("+-0123456789.eE", data[i]) >= 0 {
		i++
	}
	return i
}

// literalEnd returns the offset just past the literal, true, false or null,
// that starts at i.
func literalEnd(data []byte, i int) int {
	for i < len(data) && data[i] >= 'a' && data[i] <= 'z' {
		i++
	}
	return i
}

// orderedEqualFromDifference is orderedEqual started at the token where the
// two texts first differ, everything before it being the same bytes: the
// common prefix is found a word at a time, which is what keeps a pair of large
// objects that differ late as cheap to tell apart as comparing bytes was.
func orderedEqualFromDifference(a, b []byte) (equal, decided bool) {
	d := firstDifference(a, b)
	prefix := a[:d]
	start := d
	if inString, opening := stringAt(prefix); inString {
		// Inside a string: from its opening quote
		start = opening
	} else {
		// Inside a number or a literal: from its first character
		for start > 0 && strings.IndexByte("+-0123456789.eEabcdefghijklmnopqrstuvwxyz", a[start-1]) >= 0 {
			start--
		}
	}
	return orderedEqual(a[start:], b[start:])
}

// firstDifference returns the offset of the first byte where two texts
// differ, or the length of the shorter where one is the other's prefix,
// comparing a word at a time.
func firstDifference(a, b []byte) int {
	n := min(len(a), len(b))
	i := 0
	for ; i+8 <= n; i += 8 {
		if x := binary.LittleEndian.Uint64(a[i:]) ^ binary.LittleEndian.Uint64(b[i:]); x != 0 {
			return i + bits.TrailingZeros64(x)/8
		}
	}
	for ; i < n && a[i] == b[i]; i++ {
	}
	return i
}

// stringAt reports whether the end of a JSON text stands inside a string, and
// where that string opens. It reads only the quotes and backslashes, an
// escaped quote, \", no quote: a resource's narrative is full of them.
func stringAt(text []byte) (inString bool, opening int) {
	for i := 0; i < len(text); {
		k := bytes.IndexAny(text[i:], "\"\\")
		if k < 0 {
			break
		}
		i += k
		if text[i] == '\\' {
			i += 2
			continue
		}
		if !inString {
			opening = i
		}
		inString = !inString
		i++
	}
	return inString, opening
}
