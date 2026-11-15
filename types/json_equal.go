package types

import (
	"bytes"
	"encoding/binary"
	"math"
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
	if !shortcuts {
		shortcutEntries++
	}
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

// shortcuts turns every shortcut of equalJSON and writtenAlike on or off at
// any depth. Only tests set it, to read every pair by child as the reference
// (plan 024), and never while another goroutine compares.
var shortcuts = true

// shortcutEntries counts the entries to a shortcut made while shortcuts is
// off, which must be none: a test reads it to check the switch reaches every
// shortcut. It is written only with the switch off, in tests, so that
// comparing stays free of shared writes.
var shortcutEntries int

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
//
// It reports where it stopped in each text: the start of the token that
// decided or left it undecided, and the end of the text when it read to the end.
func orderedEqual(a, b []byte) (equal, decided bool, stopA, stopB int) {
	i, j := 0, 0
	for {
		i, j = skipJSONSpace(a, i), skipJSONSpace(b, j)
		if i == len(a) || j == len(b) {
			done := i == len(a) && j == len(b)
			return done, done, i, j
		}
		var verdict tokenVerdict
		i, j, verdict = compareTokens(a, i, b, j)
		switch verdict {
		case tokensUnequal:
			return false, true, i, j
		case tokensUndecided:
			return false, false, i, j
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
	if nextA, nextB, verdict, bad := compareBadNumbers(a, i, b, j); bad {
		return nextA, nextB, verdict
	}
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

// badNumberAt reports a number outside JSON's grammar at i.
func badNumberAt(data []byte, i int) bool {
	return isNumberStart(data[i]) && !jsonNumber(data[i:tokenEnd(data, i)])
}

// compareBadNumbers compares tokens either of which is a number outside JSON's
// grammar, which reads as null (#238): two such tokens are alike as two nulls
// are, and one against another value is left to the full reading, as a null
// is. It reports false for tokens that are not.
func compareBadNumbers(a []byte, i int, b []byte, j int) (nextA, nextB int, verdict tokenVerdict, bad bool) {
	badA, badB := badNumberAt(a, i), badNumberAt(b, j)
	switch {
	case !badA && !badB:
		return i, j, tokensAlike, false
	case (badA || startsNull(a, i)) && (badB || startsNull(b, j)):
		return tokenOrLiteralEnd(a, i, badA), tokenOrLiteralEnd(b, j, badB), tokensAlike, true
	}
	return i, j, tokensUndecided, true
}

// tokenOrLiteralEnd is the end of the number outside the grammar at i, when it
// is one, and else of the literal there.
func tokenOrLiteralEnd(data []byte, i int, bad bool) int {
	if bad {
		return tokenEnd(data, i)
	}
	return literalEnd(data, i)
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
	if !shortcuts {
		shortcutEntries++
	}
	d := firstDifference(a, b)
	// One pass over the shared bytes finds where the token that holds the
	// difference starts, and whether they are plain
	var keyBuf [plainStackKeys]keyRange
	var markBuf [plainStackMarks]int32
	shared, start, plain := plainScan{keyBuf[:0], markBuf[:0]}.prefix(a, d)
	if !plain {
		start = tokenStart(a, d)
	}
	equal, decided, stopA, stopB := orderedEqual(a[start:], b[start:])
	if decided && !equal {
		// A positive answer is trusted: texts that agree token by token read
		// alike. A negative one stands only if the reader would read the text
		// it read as written, up to the end of the token that decided (plan 024)
		return false, plain && shared.tailsPlain(a, b, start, start+stopA+tokenSpan(a[start+stopA:]), start+stopB+tokenSpan(b[start+stopB:]))
	}
	return equal, decided
}

// tokenStart returns where the token that holds a[d] starts, or d. Inside a
// string that is its opening quote. Inside a number or a literal it is the
// token's first character; the walk back stops at a delimiter, because a token
// outside the number grammar can be made of any bytes.
func tokenStart(a []byte, d int) int {
	if inString, opening := stringAt(a[:d]); inString {
		return opening
	}
	start := d
	for start > 0 && !endsToken[a[start-1]] && strings.IndexByte("[{:\"", a[start-1]) < 0 {
		start--
	}
	return start
}

// tokenSpan returns the length of the scalar token that starts data.
func tokenSpan(data []byte) int {
	if len(data) > 0 && data[0] == '"' {
		if end := stringClose(data, 0); end >= 0 {
			return end + 1
		}
		return len(data)
	}
	return tokenEnd(data, 0)
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

// keyRange is where an object's key stands in the text read.
type keyRange struct{ start, end int32 }

// maxPlainKeys is how many keys one object may hold for plainScan to compare
// them in pairs, which costs the square of that: an object with more is not
// plain, and is read by child.
const maxPlainKeys = 128

// plainStackKeys and plainStackMarks size the stacks of keys and of open
// objects a scan keeps on the stack. A text that goes beyond them is read
// all the same, with the stacks on the heap.
const (
	plainStackKeys  = 96
	plainStackMarks = 8
)

// plainScan reads a text for the gate (P1 to P3 of plan 024), and can stop at a token and
// go on from there, over the same text or over another that shares the bytes
// so far.
type plainScan struct {
	keys  []keyRange // the keys of the open objects, each object's after the last mark
	marks []int32    // where the keys of each open object begin
}

// prefix reads the first d bytes of a, which two texts share, and returns the
// scan at the token that holds a[d], where that token starts, and whether the
// bytes are plain so far.
func (s plainScan) prefix(a []byte, d int) (plainScan, int, bool) {
	s, stop, ok := s.scan(a, 0, d)
	if !ok {
		return s, 0, false
	}
	if stop < d {
		// Inside a string, which starts at stop
		return s, stop, true
	}
	// Inside a number or a literal, or between tokens: the bytes of a bare
	// token change nothing the scan keeps
	return s, tokenStart(a, d), true
}

// tailsPlain continues the scan that read the shared bytes over what each
// text read after them, a[start:endA] and b[start:endB], each with the scope
// the shared bytes left open.
func (s plainScan) tailsPlain(a, b []byte, start, endA, endB int) bool {
	var forkKeys [plainStackKeys]keyRange
	var forkMarks [plainStackMarks]int32
	fork := plainScan{append(forkKeys[:0], s.keys...), append(forkMarks[:0], s.marks...)}
	if _, stop, ok := s.scan(a, start, endA); !ok || stop != endA {
		return false
	}
	_, stop, ok := fork.scan(b, start, endB)
	return ok && stop == endB
}

// scan reads data[from:to], which begins between tokens, and returns the scan
// after it, where it stopped, and whether the text is plain so far. It stops
// before to at the opening quote of a string that closes at or after to. A key
// is told by the colon that follows it, which may stand past to. It takes and
// returns the scan by value so that the buffers it grows in stay on the
// caller's stack.
func (s plainScan) scan(data []byte, from, to int) (plainScan, int, bool) {
	if len(data) > math.MaxInt32 {
		return s, from, false
	}
	for i := from; i < to; {
		c := data[i]
		if !plainInteresting[c] {
			i++
			continue
		}
		switch c {
		case '"':
			end := stringClose(data, i)
			if end < 0 || end >= to {
				return s, i, true
			}
			if next := skipJSONSpace(data, end+1); next < len(data) && data[next] == ':' {
				if len(s.marks) == 0 || !plainKey(data, i+1, end, s.keys[s.marks[len(s.marks)-1]:]) {
					return s, i, false
				}
				key := keyRange{int32(i + 1), int32(end)} //nolint:gosec // len(data) is at most MaxInt32, checked on entry
				s.keys = append(s.keys, key)
			}
			i = end + 1
		case '{':
			s.marks = append(s.marks, int32(len(s.keys))) //nolint:gosec // at most the keys of one text, below MaxInt32
			i++
		case '}':
			if len(s.marks) == 0 {
				return s, i, false
			}
			s.keys = s.keys[:s.marks[len(s.marks)-1]]
			s.marks = s.marks[:len(s.marks)-1]
			i++
		default: // n: a null
			return s, i, false
		}
	}
	return s, to, true
}

// plainInteresting marks the bytes outside a string that plainScan stops at.
var plainInteresting = [256]bool{'"': true, '{': true, '}': true, 'n': true}

// stringClose returns the offset of the quote that closes the string that
// opens at i, or -1 when it does not close. A quote after an odd number of
// backslashes is escaped.
func stringClose(data []byte, i int) int {
	from := i + 1
	for {
		k := bytes.IndexByte(data[from:], '"')
		if k < 0 {
			return -1
		}
		q := from + k
		slashes := 0
		for b := q - 1; b > i && data[b] == '\\'; b-- {
			slashes++
		}
		if slashes%2 == 0 {
			return q
		}
		from = q + 1
	}
}

// plainKey reports the key at data[start:end] plain: it does not start with _,
// holds no backslash, and is none of the keys already read of its object.
func plainKey(data []byte, start, end int, seen []keyRange) bool {
	key := data[start:end]
	if len(key) > 0 && key[0] == '_' || bytes.IndexByte(key, '\\') >= 0 || len(seen) >= maxPlainKeys {
		return false
	}
	for _, k := range seen {
		if int(k.end-k.start) == len(key) && bytes.Equal(data[k.start:k.end], key) {
			return false
		}
	}
	return true
}
