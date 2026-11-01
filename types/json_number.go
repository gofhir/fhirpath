package types

import (
	"bytes"

	"github.com/buger/jsonparser"
)

// isJSONNumber reports whether s is a number by RFC 8259 §6:
//
//	number = [ minus ] int [ frac ] [ exp ]
//
// with int = zero / ( digit1-9 *DIGIT ), frac = point 1*DIGIT and
// exp = e [ minus / plus ] 1*DIGIT. FHIR R4 and R4B's decimal regex is the
// same grammar; R5's is informative and malformed, so the grammar is written
// here and not read from the packages (#238). A resource's number outside it
// is no number: it reads as null does.
func isJSONNumber(s string) bool { return jsonNumber(s) }

// jsonNumber is the recognizer, over a string or the bytes of a token alike,
// so that a token is checked without being copied.
func jsonNumber[T string | []byte](s T) bool {
	i := 0
	if i < len(s) && s[i] == '-' {
		i++
	}
	i, ok := scanInt(s, i)
	if !ok {
		return false
	}
	if i < len(s) && s[i] == '.' {
		if i, ok = scanDigits(s, i+1); !ok {
			return false
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if i, ok = scanDigits(s, i); !ok {
			return false
		}
	}
	return i == len(s)
}

// scanInt reads int = zero / ( digit1-9 *DIGIT ) at i.
func scanInt[T string | []byte](s T, i int) (int, bool) {
	switch {
	case i >= len(s):
		return i, false
	case s[i] == '0':
		return i + 1, true
	case s[i] >= '1' && s[i] <= '9':
		return skipDigits(s, i), true
	}
	return i, false
}

// scanDigits reads 1*DIGIT at i.
func scanDigits[T string | []byte](s T, i int) (int, bool) {
	end := skipDigits(s, i)
	return end, end > i
}

// skipDigits returns the index of the first byte at or after i that is no
// ASCII digit.
func skipDigits[T string | []byte](s T, i int) int {
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

// isJSONNumberBytes is isJSONNumber on a token as the parser hands it over,
// without copying it to a string.
func isJSONNumberBytes(b []byte) bool { return jsonNumber(b) }

// noValue reports that a field holds nothing to read: it is absent, null, or a
// number token outside JSON's grammar, which reads as null does (#238).
func (f jsonField) noValue() bool {
	return f.missing() || f.dataType == jsonparser.Null ||
		(f.dataType == jsonparser.Number && !isJSONNumberBytes(f.data))
}

// readAs is the kind a value is read as: a number outside JSON's grammar is
// null (#238).
func readAs(data []byte, dataType jsonparser.ValueType) jsonparser.ValueType {
	if dataType == jsonparser.Number && !jsonNumber(data) {
		return jsonparser.Null
	}
	return dataType
}

// tokenEnd returns the offset just past the token that starts at i, where
// jsonparser ends it: at whitespace, a comma, or the close of an array or an
// object. A number is checked over all of it, so 0x1 is not read as 0 (#238).
func tokenEnd(data []byte, i int) int {
	for i < len(data) && !endsToken[data[i]] {
		i++
	}
	return i
}

// endsToken marks the bytes that end a token.
var endsToken = [256]bool{',': true, ']': true, '}': true, ' ': true, '\t': true, '\r': true, '\n': true}

// startsNumber marks the bytes a number token starts with.
var startsNumber = func() (marks [256]bool) {
	marks['-'] = true
	for c := '0'; c <= '9'; c++ {
		marks[c] = true
	}
	return marks
}()

// nullBadNumbers is the JSON text with each number outside JSON's grammar
// written as null, which is how it is read (#238). Text with none is returned
// as it is, without a copy beyond the string's own.
func nullBadNumbers(data []byte) string { return string(nullBadNumbersBytes(data)) }

// nullBadNumbersBytes is nullBadNumbers over bytes: the text itself, not a
// copy, when there is nothing to write as null.
func nullBadNumbersBytes(data []byte) []byte {
	var out []byte
	last := 0
	for i := 0; i < len(data); {
		// Most bytes start nothing: skip them without a branch per kind
		for i < len(data) && !startsNumber[data[i]] && data[i] != '"' {
			i++
		}
		if i == len(data) {
			break
		}
		switch data[i] {
		case '"':
			i = skipString(data, i)
		default:
			end := tokenEnd(data, i)
			if !jsonNumber(data[i:end]) {
				out = append(out, data[last:i]...)
				out = append(out, "null"...)
				last = end
			}
			i = end
		}
	}
	if out == nil {
		return data
	}
	return append(out, data[last:]...)
}

// skipString returns the offset just past the string that opens at i, or the
// end of the text when it does not close. A quote closes it unless an odd
// number of backslashes stands before it.
func skipString(data []byte, i int) int {
	start := i + 1
	for at := start; ; {
		q := bytes.IndexByte(data[at:], '"')
		if q < 0 {
			return len(data)
		}
		q += at
		backslashes := 0
		for k := q - 1; k >= start && data[k] == '\\'; k-- {
			backslashes++
		}
		if backslashes%2 == 0 {
			return q + 1
		}
		at = q + 1
	}
}
