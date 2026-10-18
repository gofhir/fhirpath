package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// TestStringLiteralEscapes covers every escape sequence the FHIRPath String
// section defines, plus its rule that a backslash beginning a non-escape
// sequence is dropped.
//
// These depend on both halves of literal handling: the grammar's STRING rule
// must accept the sequence, and unquoteString must resolve it.
func TestStringLiteralEscapes(t *testing.T) {
	cases := []struct {
		expr     string
		expected string
	}{
		{`'a\'b'`, "a'b"},
		{`'a\"b'`, `a"b`},
		{"'a\\`b'", "a`b"},
		{`'a\\b'`, `a\b`},
		{`'a\rb'`, "a\rb"},
		{`'a\nb'`, "a\nb"},
		{`'a\tb'`, "a\tb"},
		{`'a\fb'`, "a\fb"},
		{`'a\/b'`, "a/b"},
		{`'abc'`, "abc"},
		{`'é'`, "é"},
		// A backslash before a non-escape is dropped, per the spec's examples
		{`'\p'`, "p"},
		{`'\3'`, "3"},
		{`'\u005'`, "u005"},
		// Resolved in one pass: '\\n' is a backslash and an n, not a line feed
		{`'\\p'`, `\p`},
		{`'\\n'`, `\n`},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			assertStringResult(t, evalOrFatal(t, simpleJSON, tc.expr), tc.expected)
		})
	}

	t.Run("the suite's full escape literal parses", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, simpleJSON,
			`'\\\/\f\r\n\t\"\`+"`"+`\'*'.convertsToString()`), true)
	})
}

// TestEncodeDecode covers encode()/decode() per the FHIRPath specification,
// section "String Manipulation": formats hex, base64 and urlbase64.
func TestEncodeDecode(t *testing.T) {
	cases := []struct {
		expr     string
		expected string
	}{
		{"'test'.encode('hex')", "74657374"},
		{"'test'.encode('base64')", "dGVzdA=="},
		{"'subjects?_d'.encode('base64')", "c3ViamVjdHM/X2Q="},
		{"'subjects?_d'.encode('urlbase64')", "c3ViamVjdHM_X2Q="},
		{"'74657374'.decode('hex')", "test"},
		{"'dGVzdA=='.decode('base64')", "test"},
		{"'c3ViamVjdHM/X2Q='.decode('base64')", "subjects?_d"},
		{"'c3ViamVjdHM_X2Q='.decode('urlbase64')", "subjects?_d"},
		// Round trip
		{"'Ünïcödé ✓'.encode('base64').decode('base64')", "Ünïcödé ✓"},
		{"'Ünïcödé ✓'.encode('hex').decode('hex')", "Ünïcödé ✓"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			assertStringResult(t, evalOrFatal(t, simpleJSON, tc.expr), tc.expected)
		})
	}

	t.Run("empty for unknown format", func(t *testing.T) {
		for _, expr := range []string{"'test'.encode('rot13')", "'test'.decode('rot13')"} {
			assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
		}
	})

	t.Run("empty for empty input", func(t *testing.T) {
		expr := "nosuchfield.encode('hex')"
		assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
	})

	t.Run("empty when the input is not valid for the encoding", func(t *testing.T) {
		expr := "'not hex!'.decode('hex')"
		assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
	})
}

// TestEscapeUnescape covers escape()/unescape() for the html and json targets.
func TestEscapeUnescape(t *testing.T) {
	cases := []struct {
		expr     string
		expected string
	}{
		{`'"1<2"'.escape('html')`, "&quot;1&lt;2&quot;"},
		{`'"1<2"'.escape('json')`, `\"1<2\"`},
		{`'&quot;1&lt;2&quot;'.unescape('html')`, `"1<2"`},
		{`'a & b'.escape('html')`, "a &amp; b"},
		{`'&amp;'.unescape('html')`, "&"},
		// Numeric character references resolve too
		{`'&#34;x&#34;'.unescape('html')`, `"x"`},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			assertStringResult(t, evalOrFatal(t, simpleJSON, tc.expr), tc.expected)
		})
	}

	t.Run("html escaping round trips", func(t *testing.T) {
		assertStringResult(t, evalOrFatal(t, simpleJSON,
			`'"1<2" & more'.escape('html').unescape('html')`), `"1<2" & more`)
	})

	t.Run("json escaping handles control characters", func(t *testing.T) {
		// A tab is escaped as \t and restored
		assertStringResult(t, evalOrFatal(t, simpleJSON,
			`'a\tb'.escape('json')`), `a\tb`)
		assertStringResult(t, evalOrFatal(t, simpleJSON,
			`'a\tb'.escape('json').unescape('json')`), "a\tb")
	})

	t.Run("empty for unknown target", func(t *testing.T) {
		for _, expr := range []string{"'x'.escape('yaml')", "'x'.unescape('yaml')"} {
			assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
		}
	})
}

// TestMatchesFull covers matchesFull(), which anchors the pattern to the whole
// input rather than matching a substring like matches() does.
func TestMatchesFull(t *testing.T) {
	const url = "'http://fhir.org/guides/cqf/common/Library/FHIR-ModelInfo|4.0.1'"

	cases := []struct {
		expr     string
		expected bool
	}{
		{url + ".matchesFull('library')", false},
		{url + ".matchesFull('Library')", false},
		{url + ".matchesFull('^Library$')", false},
		{url + ".matchesFull('.*Library.*')", true},
		{url + ".matchesFull('Measure')", false},
		{"'abc'.matchesFull('abc')", true},
		{"'abc'.matchesFull('a')", false},
		{"'abc'.matchesFull('a.c')", true},
		// matches() succeeds on a substring where matchesFull() does not
		{"'abc'.matches('b')", true},
		{"'abc'.matchesFull('b')", false},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			assertBooleanResult(t, evalOrFatal(t, simpleJSON, tc.expr), tc.expected)
		})
	}

	t.Run("empty input yields empty", func(t *testing.T) {
		expr := "nosuchfield.matchesFull('x')"
		assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
	})
}

// TestRegexFlagsFromTheSpecification covers the optional flags parameter that
// 3.0.0 adds to matches, matchesFull and replaceMatches.
//
// "i to perform a case-insensitive search (otherwise is case-sensitive)" and
// "m - Matches the start and end of each line using ^ and $ (multi-line) (not
// only begin/end of string)".
//
// Calling any of the three with a flags argument used to fail on arity, so an
// expression written against 3.0.0 could not be evaluated at all.
func TestRegexFlagsFromTheSpecification(t *testing.T) {
	doc := []byte(`{}`)

	cases := []struct {
		expr string
		want string
	}{
		// The specification's own worked examples, verbatim
		{`'first line\nsecond line'.matches('^second', 'm')`, "true"},
		{`'first line\nsecond line'.matches('^second', '')`, "false"},
		{`'first line\nsecond line'.matches('^SECOND', 'im')`, "true"},

		// i, on each of the three functions
		{`'ABC'.matches('abc', 'i')`, "true"},
		{`'ABC'.matches('abc')`, "false"},
		{`'ABC'.matchesFull('abc', 'i')`, "true"},
		{`'ABC'.matchesFull('abc')`, "false"},
		{`'aAbB'.replaceMatches('a', 'x', 'i')`, "xxbB"},
		{`'aAbB'.replaceMatches('a', 'x')`, "xAbB"},

		// Order does not matter, and a repeated flag is not an error
		{`'ABC\nDEF'.matches('^def', 'mi')`, "true"},
		{`'ABC\nDEF'.matches('^def', 'iim')`, "true"},

		// An absent or empty flags argument is the default behavior, and the
		// single line mode the specification fixes still applies: . matches a
		// newline whether or not m is given
		{`'A\nB'.matches('A.B')`, "true"},
		{`'A\nB'.matches('A.B', '')`, "true"},
		{`'A\nB'.matches('A.B', 'm')`, "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, doc); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestRegexFlagsRejectsWhatIsNotAFlag covers the rule the specification states
// for the parameter: "if the flags parameter contains invalid values, the
// evaluation of the expression will end and signal an error to the calling
// environment."
//
// Silently ignoring an unknown flag would be worse than refusing it — an
// expression asking for a mode the engine does not apply would answer as though
// it had been applied.
func TestRegexFlagsRejectsWhatIsNotAFlag(t *testing.T) {
	doc := []byte(`{}`)

	for _, expr := range []string{
		`'abc'.matches('a', 'z')`,
		`'abc'.matches('a', 'ix')`,
		`'abc'.matches('a', 's')`,
		`'abc'.matchesFull('a', 'g')`,
		`'abc'.replaceMatches('a', 'x', 'u')`,
	} {
		t.Run(expr, func(t *testing.T) {
			if _, err := fhirpath.MustCompile(expr).Evaluate(doc); err == nil {
				t.Errorf("expected an error: the flag is not one the specification defines")
			}
		})
	}
}

// TestMatchesIsPartialAndMatchesFullIsNot covers the distinction between the two
// functions, which is the whole reason both exist.
//
// A validator team reported the partial behavior as a possible defect, and it is
// not: "the start/end of line markers ^, $ can be used to match the entire
// string" only means something if the pattern is not anchored already, and
// matchesFull is defined as the form where they always surround it.
//
// The pairs below are the specification's own examples, which state the
// difference by giving the same input and pattern to both functions.
func TestMatchesIsPartialAndMatchesFullIsNot(t *testing.T) {
	doc := []byte(`{}`)

	const canonical = `'http://fhir.org/guides/cqf/common/Library/FHIR-ModelInfo|4.0.1'`

	cases := []struct {
		expr string
		want string
	}{
		{canonical + `.matches('Library')`, "true"},
		{canonical + `.matchesFull('Library')`, "false"},

		// "returns true as the string has an 8 number sequence in it starting
		// with N", against "returns false as the string is not an 8 char number
		// (it has 10)"
		{`'N8000123123'.matches('N[0-9]{8}')`, "true"},
		{`'N8000123123'.matchesFull('N[0-9]{8}')`, "false"},
		{`'N8000123123'.matchesFull('N[0-9]{10}')`, "true"},

		// Anchoring the pattern by hand gives matchesFull's answer from matches
		{`'N8000123123'.matches('^N[0-9]{8}$')`, "false"},

		// Which is the shape FHIR's own invariants need and mostly lack: eld-19
		// and the identifier-shaped constraints are unanchored, so a value that
		// merely embeds something acceptable passes
		{`'  X  '.matches('[A-Z]([A-Za-z0-9_]){0,254}')`, "true"},
		{`'  X  '.matchesFull('[A-Z]([A-Za-z0-9_]){0,254}')`, "false"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, doc); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestMatchesWithinUrlFromTheSuite runs the conformance cases that settle the
// question, so the answer lives here and not only in an issue thread.
//
// Both corpora carry two groups over one input, testMatchesWithinUrl and
// testMatchesFullWithinUrl — "within" is the suite's own word for it. The pair
// that matters is the one where they differ: the same value and the same pattern
// are expected to be true for matches and false for matchesFull.
//
// The engine passes all ten in all four configurations, and anchoring matches
// would fail testMatchesWithinUrl2 in R4 and R5 alike. So this is not a reading
// of the specification that could go either way — it is a published expected
// value.
func TestMatchesWithinUrlFromTheSuite(t *testing.T) {
	doc := []byte(`{}`)

	const url = `'http://fhir.org/guides/cqf/common/Library/FHIR-ModelInfo|4.0.1'`

	for _, tc := range []struct{ name, expr, want string }{
		{"testMatchesWithinUrl1", url + `.matches('library')`, "false"},
		{"testMatchesWithinUrl2", url + `.matches('Library')`, "true"},
		{"testMatchesWithinUrl3", url + `.matches('^Library$')`, "false"},
		{"testMatchesWithinUrl1a", url + `.matches('.*Library.*')`, "true"},
		{"testMatchesWithinUrl4", url + `.matches('Measure')`, "false"},

		{"testMatchesFullWithinUrl1", url + `.matchesFull('library')`, "false"},
		{"testMatchesFullWithinUrl3", url + `.matchesFull('Library')`, "false"},
		{"testMatchesFullWithinUrl4", url + `.matchesFull('^Library$')`, "false"},
		{"testMatchesFullWithinUrl1a", url + `.matchesFull('.*Library.*')`, "true"},
		{"testMatchesFullWithinUrl2", url + `.matchesFull('Measure')`, "false"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, doc); got != tc.want {
				t.Errorf("%s: got %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// The FHIRPath string literals HL7 publishes for these invariants, verbatim.
// They are Go raw strings so that the escaping below is the specification's and
// not this file's: FHIRPath unescapes \\s to the regex \s, and \\' to an
// apostrophe.
const (
	eld19Pattern = `'[A-Za-z][A-Za-z0-9]*(\\.[a-zA-Z0-9]+(\\[x\\])?)*(\\:[^\\s\\.]+)?(\\.[a-zA-Z0-9]+(\\[x\\])?(\\:[^\\s\\.]+)?)*'`
	eld20Pattern = `'[^\\s\\.,:;\\\'"\\/|?!@#$%&*()\\[\\]{}]{1,64}'`
)

// TestPublishedInvariantsEvaluate covers eld-19 and eld-20 through the engine,
// the way a validator reaches them.
//
// Both are invariants HL7 publishes against ElementDefinition, and both were
// refused by a pattern check that ran before the regex was compiled. eld-19 is
// SHALL-level, so while it was refused ElementDefinition.path went unchecked
// and a malformed path passed. The failure was a property of the expression
// rather than of the data, so it repeated once per element: a 200-element
// snapshot raised it some four hundred times.
//
// The assertions use matchesFull, because matches() looks for the pattern
// anywhere in the value — '.leadingDot' contains 'leadingDot', which matches.
func TestPublishedInvariantsEvaluate(t *testing.T) {
	doc := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		pattern string
		value   string
		want    string
	}{
		{eld19Pattern, "Patient.name.given", "true"},
		{eld19Pattern, "Observation.value[x]", "true"},
		{eld19Pattern, "ElementDefinition.path", "true"},
		{eld19Pattern, "Patient.extension:race", "true"},
		{eld19Pattern, ".leadingDot", "false"},
		{eld19Pattern, "has space", "false"},
		{eld19Pattern, "1startsWithADigit", "false"},

		{eld20Pattern, "given", "true"},
		{eld20Pattern, "has space", "false"},
		{eld20Pattern, "has.punctuation", "false"},
	}

	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			expr := "'" + tc.value + "'.matchesFull(" + tc.pattern + ")"
			if got := evaluateScalar(t, expr, doc); got != tc.want {
				t.Errorf("%s = %s, want %s", expr, got, tc.want)
			}
		})
	}
}

// TestLastIndexOf covers the last-occurrence search, which counts characters
// rather than bytes.
func TestLastIndexOf(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		{"'abcdefg'.lastIndexOf('bc')", "1"},
		{"'abcabc'.lastIndexOf('bc')", "4"},
		{"'abcdefg'.lastIndexOf('x')", "-1"},
		{"'abcdefg'.lastIndexOf('abcdefg')", "0"},
		// "If substring is an empty string, the function returns the length"
		{"'abc'.lastIndexOf('')", "3"},
		{"{}.lastIndexOf('a')", "EMPTY"},

		// The index is measured in characters, so a multi-byte prefix counts once
		{"'héllo wörld'.lastIndexOf('ö')", "7"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestStringFunctionsCountCharacters covers the unit FHIRPath measures strings
// in.
//
// "Returns the number of characters (Unicode scalar values) in the input
// string", says length(); "the returned index is measured in characters (Unicode
// scalar values)", says indexOf. Bytes are not characters wherever the data is
// not ASCII, which for a patient name in Spanish, Portuguese or French is the
// ordinary case rather than the exotic one.
//
// The engine used to index bytes in length(), indexOf() and substring(), while
// toChars() and lastIndexOf() counted characters — the same string measured two
// ways depending on which function asked. substring() then returned invalid
// UTF-8, having cut a two-byte character in half: 'ñJosé'.substring(1) answered
// "\xb1José".
func TestStringFunctionsCountCharacters(t *testing.T) {
	doc := []byte(`{}`)

	cases := []struct {
		expr string
		want string
	}{
		// length counts characters
		{"'José'.length()", "4"},
		{"'ñandú'.length()", "5"},
		{"'ñ'.length()", "1"},
		{"''.length()", "0"},
		{"'abc'.length()", "3"},

		// and agrees with toChars, which always did
		{"'ñandú'.toChars().count()", "5"},
		{"'ñandú'.toChars().count() = 'ñandú'.length()", "true"},

		// indexOf returns a character offset. 'ñJosé' is ñ J o s é in
		// characters, and the J sits at byte 2
		{"'ñJosé'.indexOf('J')", "1"},
		{"'ñJosé'.lastIndexOf('J')", "1"},
		{"'áé'.indexOf('é')", "1"},

		// substring takes characters, and never splits one
		{"'ñJosé'.substring(1)", "José"},
		{"'ñJosé'.substring(1, 1)", "J"},
		{"'áé'.substring(1, 1)", "é"},
		{"'ñandú'.substring(0, 2)", "ña"},
		{"'ñandú'.substring(4)", "ú"},

		// A character index past the end is still out of range, even where the
		// byte length would reach
		{"'ñandú'.substring(5).empty()", "true"},
		{"'ñ'.substring(1).empty()", "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, doc); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestSubstringLengthNeverPanics covers what a negative length used to do:
// panic, which is not an error a caller can handle — it unwinds the process, and
// there is no recover on the evaluation path.
//
// It arrives from arithmetic rather than being written down. R5's sdf-24 and
// sdf-25 compute id.substring(0, $this.length()-10), negative for any id shorter
// than ten characters, so validating a StructureDefinition crashed.
//
// The answer is the empty string. "If length is given, will return at most
// length number of characters", and the specification names only an out-of-range
// start, an empty input and an empty start as causes of empty — length bounds
// the result and never refuses the call. At most a negative number of characters
// is none of them. fhirpath.js 5.1.0 answers the same.
func TestSubstringLengthNeverPanics(t *testing.T) {
	doc := []byte(`{}`)

	cases := []struct {
		expr string
		want string
	}{
		{"'abc'.substring(0, -1)", ""},
		{"'abc'.substring(1, -5)", ""},
		{"'abc'.substring(0, 'abc'.length()-10)", ""},
		{"'ñandú'.substring(2, -1)", ""},

		// Still a string, not an empty collection
		{"'abc'.substring(0, -1).count()", "1"},
		{"'abc'.substring(0, -1) = ''", "true"},

		// Which is what a zero length already answered
		{"'abc'.substring(0, 0)", ""},
		{"'abc'.substring(0, 0) = ''", "true"},

		// An enormous length is bounded by what remains, from either direction
		{"'abc'.substring(1, 1000)", "bc"},
		{"'abc'.substring(0, 3)", "abc"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, doc); got != tc.want {
				t.Errorf("%s = %q, want %q", tc.expr, got, tc.want)
			}
		})
	}
}

// TestSubstringWithAnEmptyLength covers a rule stated outright and not
// implemented: "If an empty length is provided, the behavior is the same as if
// length had not been provided."
//
// It used to raise a type error instead.
func TestSubstringWithAnEmptyLength(t *testing.T) {
	doc := []byte(`{}`)

	for _, tc := range []struct{ expr, want string }{
		{"'abcdefg'.substring(3, {})", "defg"},
		{"'abcdefg'.substring(3)", "defg"},
		{"'abcdefg'.substring(3, {}) = 'abcdefg'.substring(3)", "true"},
		{"'ñJosé'.substring(1, {})", "José"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, doc); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestSubstringSpecExamples runs the specification's own worked examples, so the
// rewrite above is anchored to them rather than to a reading of them.
func TestSubstringSpecExamples(t *testing.T) {
	doc := []byte(`{}`)

	for _, tc := range []struct{ expr, want string }{
		{"'abcdefg'.substring(3)", "defg"},
		{"'abcdefg'.substring(1, 2)", "bc"},
		{"'abcdefg'.substring(6, 2)", "g"},
		{"'abcdefg'.substring(7, 1).empty()", "true"},

		// indexOf, from the same section
		{"'abcdefg'.indexOf('bc')", "1"},
		{"'abcdefg'.indexOf('x')", "-1"},
		{"'abcdefg'.indexOf('')", "0"},

		// length
		{"'abc'.length()", "3"},
		{"''.length()", "0"},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, doc); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

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
		if _, err := fhirpath.Evaluate(simpleJSON, expr); err == nil {
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

// TestSubstringWithEmptyStart covers an absent start position, which the
// specification answers rather than refuses: "If the input or start is empty,
// the result is empty."
func TestSubstringWithEmptyStart(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		{"'string'.substring({}).empty()", "true"},
		{"{}.substring(1).empty()", "true"},
		// A start that is present still works, including out of range
		{"'string'.substring(2)", "ring"},
		{"'string'.substring(0, 3)", "str"},
		{"'string'.substring(20).empty()", "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestRegexSingleLineMode covers the mode FHIRPath fixes for regular
// expressions: "should be case-sensitive, use 'single line' mode and allow
// Unicode characters".
//
// Single line mode is what makes . match a newline, which Go leaves off by
// default. Without it, a value that spans lines — an address, a narrative —
// cannot be matched across them.
func TestRegexSingleLineMode(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	cases := []struct {
		expr string
		want string
	}{
		{"'A\nB'.matches('A.*B')", "true"},
		{"'A B'.matches('A.*B')", "true"},
		{"'a\nb'.replaceMatches('a.b', 'x')", "x"},

		// The markers still address the whole string, not each line: multi-line
		// is a flag, and single line mode is not it
		{"'abc'.matches('^abc$')", "true"},
		{"'abc\ndef'.matches('^abc$')", "false"},

		// The specification's worked examples
		{"'N8000123123'.matches('^N[0-9]{8}$')", "false"},
		{"'http://fhir.org/guides/cqf/common/Library/FHIR-ModelInfo|4.0.1'.matches('Library')", "true"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// Test string functions
func TestStringFunctions(t *testing.T) {
	patient := []byte(`{"resourceType": "Patient"}`)

	tests := []struct {
		expr string
		want string
	}{
		{"'Hello'.lower()", "hello"},
		{"'hello'.upper()", "HELLO"},
		{"'hello world'.startsWith('hello')", "true"},
		{"'hello world'.endsWith('world')", "true"},
		{"'hello world'.contains('lo wo')", "true"},
		{"'hello'.length()", "5"},
		{"'hello world'.replace('world', 'there')", "hello there"},
		{"'a,b,c'.split(',').count()", "3"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := fhirpath.Evaluate(patient, tt.expr)
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if got := result[0].String(); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}
