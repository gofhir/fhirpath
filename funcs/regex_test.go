package funcs

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestCompileAcceptsPatternsTheSpecificationPublishes covers the patterns a
// pattern validator has no business refusing.
//
// The first two are eld-19 and eld-20, invariants HL7 publishes in the R4
// specification against ElementDefinition. eld-19 is SHALL-level: while it was
// refused, ElementDefinition.path could not be validated at all and a malformed
// path passed unnoticed. The rest are ordinary regular expression syntax.
func TestCompileAcceptsPatternsTheSpecificationPublishes(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
	}{
		{"eld-19", `[A-Za-z][A-Za-z0-9]*(\.[a-zA-Z0-9]+(\[x\])?)*(\:[^\s\.]+)?(\.[a-zA-Z0-9]+(\[x\])?(\:[^\s\.]+)?)*`},
		{"eld-20", `[^\s\.,:;\'"\/|?!@#$%&*()\[\]{}]+(\s[^\s\.,:;\'"\/|?!@#$%&*()\[\]{}]+)*`},

		// A group carrying a quantifier, then a quantifier of its own
		{"optional quantified group", `(a+)?`},
		{"starred quantified group", `(a*)?`},
		{"repeated quantified group", `(a+)*`},

		// The ? after a quantifier makes it lazy; it is not a second quantifier
		{"lazy plus", `a+?`},
		{"lazy star", `a*?`},
		{"lazy brace", `a{1,3}?`},

		// Inside a character class a quantifier character is a literal
		{"quantifiers in a class", `a[*+?]b`},
		{"negated class", `[^*+?]+`},

		// Escaped, they are literals too
		{"escaped quantifiers", `a\+\+`},

		// Depth is not danger
		{"deep nesting", `(((((((a)))))))`},

		// FHIR's own primitive type patterns
		{"FHIR id", `[A-Za-z0-9\-\.]{1,64}`},
		{"FHIR dateTime", `([0-9]([0-9]([0-9][1-9]|[1-9]0)|[1-9]00)|[1-9]000)(-(0[1-9]|1[0-2])(-(0[1-9]|[1-2][0-9]|3[0-1]))?)?`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DefaultRegexCache.Compile(tc.pattern); err != nil {
				t.Errorf("refused a valid pattern: %v", err)
			}
		})
	}
}

// TestCompileStillRefusesMalformedPatterns covers what took the removed guard's
// place: the compiler.
//
// Every shape the guard claimed to catch is caught here, with a message that
// names the actual fault instead of guessing at intent.
func TestCompileStillRefusesMalformedPatterns(t *testing.T) {
	for _, pattern := range []string{
		`a**`,     // nested repetition
		`a*+`,     // same
		`a{2}{3}`, // same, spelled with braces
		`(a`,      // unclosed group
		`[a-`,     // unclosed class
		`a{1001}`, // repeat count past what RE2 allows
	} {
		t.Run(pattern, func(t *testing.T) {
			if _, err := DefaultRegexCache.Compile(pattern); err == nil {
				t.Errorf("accepted a malformed pattern")
			}
		})
	}
}

// TestCompileBoundsPatternLength covers the one check that survives: a limit on
// size, which unlike a limit on shape cannot misread what a pattern means.
func TestCompileBoundsPatternLength(t *testing.T) {
	cache := NewRegexCache(10, 20, time.Second)

	if _, err := cache.Compile(strings.Repeat("a", 10)); err != nil {
		t.Errorf("refused a pattern within the limit: %v", err)
	}
	if _, err := cache.Compile(strings.Repeat("a", 21)); err == nil {
		t.Errorf("accepted a pattern past the limit")
	}
}

// TestMatchingIsLinear is the reason there is nothing to guard against.
//
// (a+)+$ against a run of a's ending in a character that cannot match is the
// textbook ReDoS pattern: a backtracking engine takes time exponential in the
// input on it. Go's regexp is RE2, which does not backtrack, so the same
// pattern that hangs a backtracking matcher returns immediately.
func TestMatchingIsLinear(t *testing.T) {
	// gocritic carries the same heuristic the engine just dropped, and flags
	// this pattern for the same reason: it reads the shape without knowing which
	// matcher will run it. Here the shape is the point of the test.
	re := regexp.MustCompile(`(a+)+$`) //nolint:gocritic // RE2 does not backtrack; that is what this asserts
	input := strings.Repeat("a", 64) + "!"

	done := make(chan bool, 1)
	go func() { done <- re.MatchString(input) }()

	select {
	case matched := <-done:
		if matched {
			t.Errorf("the input was built not to match")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("catastrophic backtracking: the matcher is not RE2")
	}
}
