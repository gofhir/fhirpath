package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
)

// where(), exists() and all() end with an error when their criteria evaluates
// to more than one item, as the HL7 validator does and as where()'s text says:
// "If the result of evaluating the condition is other than a single boolean
// value, the evaluation will end and signal an error". Empty and a single item
// keep their reading. fhirpath.js reads the first item instead.
func TestCriteriaOfSeveralItemsIsAnError(t *testing.T) {
	doc := []byte(`{"resourceType":"Patient","name":[{"family":"F","given":["A","B"]},{"family":"G"}]}`)
	for _, expr := range []string{
		"(1 | 2).exists(true | false)",
		"(1 | 2).where(true | false)",
		"(1 | 2).all(true | false)",
		"Patient.name.exists(given)",
		"Patient.name.where(given)",
	} {
		if _, err := fhirpath.Evaluate(doc, expr); err == nil {
			t.Errorf("%s: no error, want one for a criteria of several items", expr)
		}
	}
	for expr, want := range map[string]string{
		"(1 | 2).exists(true | true)":                "true",
		"(1 | 2).exists({})":                         "false",
		"(1 | 2).where('a').count()":                 "2",
		"(1 | 2).all({})":                            "false",
		"Patient.name.exists(family = 'F')":          "true",
		"Patient.name.where(given.exists()).count()": "1",
		"Patient.name.all(family.exists())":          "true",
	} {
		if got := evaluateScalar(t, expr, doc); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

// TestRepeat covers repeat(), which walks a projection transitively.
func TestRepeat(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		// Items 1, 2, 1.1, 1.2 and 1.1.1 — every level, reached by re-applying
		// the projection to each round of results
		{"repeat(item).linkId.count()", "5"},
		{"repeat(item).linkId.first()", "1"},

		// select() applies the projection once, which is the contrast repeat()
		// exists for: the two children of item 1, and nothing below them
		{"item.select(item).linkId.count()", "2"},

		// The input is not part of the output, only what the projection yields
		{"repeat(item).where(linkId.empty()).empty()", "true"},

		// A projection that yields nothing yields nothing, rather than looping
		{"repeat(linkId).count()", "0"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, nestedQuestionnaire); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestRepeatAllKeepsDuplicates contrasts the two forms.
//
// repeat() adds an item only when the output holds no equal one, so a projection
// that keeps producing the same value stops after the first. repeatAll() adds it
// every time, and terminates only because a round eventually yields nothing.
func TestRepeatAllKeepsDuplicates(t *testing.T) {
	// Two names that share a family, so the projection produces equal values
	patient := []byte(`{"resourceType":"Patient","name":[{"family":"Smith"},{"family":"Smith"}]}`)

	if got := evaluateScalar(t, "name.repeat(family).count()", patient); got != "1" {
		t.Errorf("repeat() = %s, want 1: equal items are collected once", got)
	}
	if got := evaluateScalar(t, "name.repeatAll(family).count()", patient); got != "2" {
		t.Errorf("repeatAll() = %s, want 2: duplicates are kept", got)
	}
}

// TestCoalesce covers the first-non-empty selection, including the
// short-circuit the specification requires.
func TestCoalesce(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[{"use":"usual","text":"Jim"}]}`)

	cases := []struct {
		expr string
		want string
	}{
		{"coalesce({}, 'b')", "b"},
		{"coalesce('a', 'b')", "a"},
		{"coalesce({}, {})", "EMPTY"},
		{"coalesce({}, {}, 'c')", "c"},

		// The shape the specification's example uses: fall through to the first
		// name that exists
		{"coalesce(name.where(use='official'), name.where(use='usual')).text", "Jim"},

		// Arguments after the first non-empty one are not evaluated, so the
		// error in the second argument is never reached
		{"coalesce('a', 1 'kg' + 1 'm').count()", "1"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, patient); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}
