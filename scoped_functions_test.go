package fhirpath

import "testing"

// A questionnaire nested three levels deep, which is the shape repeat() exists
// for: the items form a tree of unbounded depth that no fixed path can walk.
var nestedQuestionnaire = []byte(`{
  "resourceType":"Questionnaire",
  "item":[
    {"linkId":"1","item":[
      {"linkId":"1.1","item":[{"linkId":"1.1.1"}]},
      {"linkId":"1.2"}
    ]},
    {"linkId":"2"}
  ]
}`)

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

// TestDefineVariable covers the one function that exists for its effect on the
// context rather than its result.
func TestDefineVariable(t *testing.T) {
	cases := []struct {
		expr string
		want string
	}{
		// The output is the input, so the function is transparent to its chain
		{"item.defineVariable('x').linkId.count()", "2"},

		// The name is visible further along the same expression
		{"defineVariable('root', 'Q').item.select(%root).count()", "2"},

		// The shape the specification's own example uses: a name bound at one
		// level and read from inside a nested projection
		{"item.first().defineVariable('top', linkId).item.select(%top & '/' & linkId).first()", "1/1.1"},
	}

	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			if got := evaluateScalar(t, tc.expr, nestedQuestionnaire); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
			}
		})
	}
}

// TestDefineVariablePerIterationScope checks that each element of an iteration
// starts from the same scope.
//
// Without that, the second name would find 'n' already defined and the
// evaluation would fail — which is why the specification describes the variable
// as popped off the stack when the expression completes.
func TestDefineVariablePerIterationScope(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[{"family":"A"},{"family":"B"}]}`)

	result, err := MustCompile("name.select(defineVariable('n', family).select(%n))").Evaluate(patient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 || result[0].String() != "A" || result[1].String() != "B" {
		t.Errorf("got %v, want [A B]", result)
	}
}

// TestDefineVariableRedefinition checks the error the specification requires:
// "If the name already exists in the current expression scope, the evaluation
// will end and signal an error to the calling environment."
func TestDefineVariableRedefinition(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[{"family":"A"}]}`)

	for _, expr := range []string{
		"defineVariable('x').defineVariable('x').count()",
		// The environment's own variables are in scope, so they cannot be shadowed
		"defineVariable('resource').count()",
		"defineVariable('ucum').count()",
	} {
		t.Run(expr, func(t *testing.T) {
			if _, err := MustCompile(expr).Evaluate(patient); err == nil {
				t.Errorf("%s: expected an error", expr)
			}
		})
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
