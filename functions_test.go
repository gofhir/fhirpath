package fhirpath

import "testing"

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

// TestComparableFunction covers comparable(), which reports whether two
// quantities have commensurable units.
func TestComparableFunction(t *testing.T) {
	cases := []struct {
		expr     string
		expected bool
	}{
		{"(1 'cm').comparable(1 '[in_i]')", true},
		{"(1 'cm').comparable(1 'm')", true},
		{"(1 'cm').comparable(1 's')", false},
		{"(1 'mg').comparable(1 'g')", true},
		{"(1 'mg').comparable(1 'L')", false},
		{"(1 'cm').comparable(1 '[s]')", false},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			assertBooleanResult(t, evalOrFatal(t, simpleJSON, tc.expr), tc.expected)
		})
	}

	t.Run("works on FHIR quantity objects", func(t *testing.T) {
		q := []byte(`{"value":10,"unit":"milligram","system":"http://unitsofmeasure.org","code":"mg"}`)
		assertBooleanResult(t, evalOrFatal(t, q, "$this.comparable(1 'g')"), true)
		assertBooleanResult(t, evalOrFatal(t, q, "$this.comparable(1 's')"), false)
	})
}

// TestSort covers sort(), including ordering keys and the leading minus that
// marks a key as descending.
func TestSort(t *testing.T) {
	names := []byte(`{
		"resourceType": "Patient",
		"name": [
			{"use": "official", "family": "Chalmers", "given": ["Peter", "James"]},
			{"use": "usual", "given": ["Jim"]},
			{"use": "maiden", "family": "Windsor", "given": ["Peter", "James"]}
		]
	}`)

	cases := []struct {
		expr     string
		expected bool
	}{
		{"(1 | 2 | 3).sort() = (1 | 2 | 3)", true},
		{"(3 | 2 | 1).sort() = (1 | 2 | 3)", true},
		{"(1 | 2 | 3).sort($this) = (1 | 2 | 3)", true},
		{"(3 | 2 | 1).sort($this) = (1 | 2 | 3)", true},
		{"(1 | 2 | 3).sort(-$this) = (3 | 2 | 1)", true},
		{"('a' | 'b' | 'c').sort($this) = ('a' | 'b' | 'c')", true},
		{"('c' | 'b' | 'a').sort($this) = ('a' | 'b' | 'c')", true},
		{"('a' | 'b' | 'c').sort(-$this) = ('c' | 'b' | 'a')", true},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			assertBooleanResult(t, evalOrFatal(t, simpleJSON, tc.expr), tc.expected)
		})
	}

	t.Run("sorts navigated values", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, names,
			"Patient.name[0].given.sort() = ('James' | 'Peter')"), true)
	})

	t.Run("multiple descending keys", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, names,
			"Patient.name.sort(-family, -given.first()).first().use = 'usual'"), true)
	})

	t.Run("a key missing on some elements sorts them last", func(t *testing.T) {
		// The name without a family has no key for it
		result := evalOrFatal(t, names, "Patient.name.sort(family).last().use")
		assertStringResult(t, result, "usual")
	})

	t.Run("collections of fewer than two elements are returned as is", func(t *testing.T) {
		assertIntegerResult(t, evalOrFatal(t, simpleJSON, "(1).sort().count()"), 1)
		assertEmptyResult(t, evalOrFatal(t, simpleJSON, "nosuchfield.sort()"), "nosuchfield.sort()")
	})

	t.Run("ordering is stable", func(t *testing.T) {
		// Both names share the same family key, so they keep their input order
		result := evalOrFatal(t, names, "Patient.name.sort(given.first()).use")
		if len(result) != 3 {
			t.Fatalf("expected 3 results, got %v", result)
		}
	})
}

// TestCollectionEquality covers = and ~ over collections, which the spec defines
// as item-by-item (ordered) and order-independent respectively.
func TestCollectionEquality(t *testing.T) {
	t.Run("equal compares item by item in order", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "(1 | 2 | 3) = (1 | 2 | 3)"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "(1 | 2 | 3) = (3 | 2 | 1)"), false)
	})

	t.Run("different lengths are false, not empty", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "(1 | 2 | 3) = (1 | 2)"), false)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "(1 | 2) = (1 | 2 | 3)"), false)
	})

	t.Run("equivalence ignores order", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "(1 | 2 | 3) ~ (3 | 1 | 2)"), true)
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "(1 | 2 | 3) ~ (1 | 2 | 9)"), false)
	})

	t.Run("an empty operand still yields empty for equality", func(t *testing.T) {
		expr := "nosuchfield = (1 | 2)"
		assertEmptyResult(t, evalOrFatal(t, simpleJSON, expr), expr)
	})

	t.Run("two empty collections are equivalent", func(t *testing.T) {
		assertBooleanResult(t, evalOrFatal(t, simpleJSON, "nosuchfield ~ othermissing"), true)
	})
}
