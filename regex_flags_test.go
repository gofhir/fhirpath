package fhirpath

import "testing"

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
			if _, err := MustCompile(expr).Evaluate(doc); err == nil {
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
