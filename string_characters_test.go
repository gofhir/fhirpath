package fhirpath

import "testing"

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
