package fhirpath

import "testing"

// The string functions measured across this engine, fhirpath.js and HL7's
// Java engine (#167): an argument of more than one item is an error, a
// surrogate pair written as two escapes is one character, encode('ascii')
// replaces what is past code 127, decode() refuses bytes that are no UTF-8,
// trim() removes ASCII whitespace alone, and join() of nothing is empty.
func TestStringFunctionsAsMeasured(t *testing.T) {
	for _, expr := range []string{
		"'abc'.startsWith('a' | 'b')",
		"'abc'.indexOf('a' | 'b')",
		"'abc'.substring(0, 1 | 2)",
		"'a,b'.split(',' | ';')",
		"'abc'.encode('hex' | 'base64')",
		"'abc'.replace('a' | 'b', 'x')",
	} {
		if _, err := Evaluate(simpleJSON, expr); err == nil {
			t.Errorf("%s: no error, want one for an argument of several items", expr)
		}
	}
	for expr, want := range map[string]string{
		"'abc'.encode('ascii')":             "abc",
		"'aé'.encode('ascii')":              "a?",
		"'a🔥b'.encode('ascii')":             "a?b",
		"'ff'.decode('hex')":                "EMPTY",
		"'616263'.decode('hex')":            "abc",
		"' \\t a \\n'.trim()":               "a",
		"'\\fa\\f'.trim().length()":         "1",
		"'\\u00a0a\\u00a0'.trim().length()": "3",
		"{}.join(',')":                      "EMPTY",
		"('a' | 'b').join(',').length()":    "3",
	} {
		if got := evaluateScalar(t, expr, simpleJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// A surrogate pair written as two escapes in a literal is one character, as
// the literal itself is. The escapes are built from a backslash rune so that
// no tool turns them into the character before the engine reads them.
func TestASurrogatePairEscapeIsOneCharacter(t *testing.T) {
	bs := string(rune(92))
	fire := bs + "uD83D" + bs + "uDD25"
	for expr, want := range map[string]string{
		"'" + fire + "' = '\U0001F525'":             "true",
		"'a" + fire + "b'.length()":                 "3",
		"'a" + fire + "b'.indexOf('b')":             "2",
		"'a" + fire + "b'.toChars().count()":        "3",
		"'" + bs + "u00e9'.length()":                "1",
		"'" + bs + "uD83D'.length()":                "1",
		"'" + bs + "uDD25" + bs + "uD83D'.length()": "2",
	} {
		if got := evaluateScalar(t, expr, simpleJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// trim() removes the whitespace fhirpath.js and the HL7 validator both trim,
// and keeps what only one of them does: a control character below space (the
// validator) and a Unicode space past ASCII (fhirpath.js).
func TestTrimRemovesWhatBothEnginesDo(t *testing.T) {
	bs := string(rune(92))
	for expr, want := range map[string]string{
		"'" + bs + "u000Ba'.trim().length()": "1",
		"'" + bs + "fa'.trim().length()":     "1",
		"'" + bs + "u001Fa'.trim().length()": "2",
		"'" + bs + "u0000a'.trim().length()": "2",
		"'" + bs + "u2003a'.trim().length()": "2",
		"'" + bs + "u00a0a'.trim().length()": "2",
	} {
		if got := evaluateScalar(t, expr, simpleJSON); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
