package types

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// equalJSONByChildren is equalJSON without its fast paths: every pair read
// child by child. The fast paths decide what they can without building
// anything, and must decide it as this does.
func equalJSONByChildren(a, b []byte) bool {
	left, ok := parseJSONObject(a)
	if !ok {
		return false
	}
	right, ok := parseJSONObject(b)
	return ok && equalObjects(&left, &right)
}

// TestEqualJSONFastPathsAgreeWithReadingByChild compares equalJSON with
// reading by child over pairs made to differ the ways the fast paths take
// apart: keys in another order, a number written another way, a null member,
// an escape, a value changed early or late, a nested object.
func TestEqualJSONFastPathsAgreeWithReadingByChild(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	values := []string{`"a"`, `"b"`, `"a b"`, `"\u0061"`, `"\u00e9"`, `"é"`, `"a\"b"`, `"x\\y"`, `1`, `1.0`, `2`, `-0`, `0`, `1e2`, `100`, `true`, `false`, `null`,
		`{"x":"a"}`, `{"x":"b"}`, `{"y":1,"x":"a"}`, `{"x":"a","y":1.0}`, `{"x":{"z":"a\"b"}}`, `{"x":{"z":"a\"c"}}`,
		`[]`, `["a"]`, `["a","b"]`, `["b","a"]`, `[null,"a"]`, `["a",null]`, `[{"id":"e"},null]`, `[null,{"id":"e"}]`, `[{}]`}
	narratives := []string{`{"div":"<div xmlns=\"http://www.w3.org/1999/xhtml\">a</div>"}`, `{"div":"<p class=\"x\">b</p>"}`}
	// A narrative first, full of escaped quotes, as most resources begin;
	// given beside its element _given, nulls holding positions.
	keys := []string{"text", "code", "given", "_given", "system", "display", "version", "extension", "id"}

	object := func() map[string]string {
		m := map[string]string{}
		for _, k := range keys {
			switch {
			case k == "text":
				if rng.Intn(2) == 0 {
					m[k] = narratives[rng.Intn(len(narratives))]
				}
			case rng.Intn(2) == 0:
				m[k] = values[rng.Intn(len(values))]
			}
		}
		return m
	}
	write := func(m map[string]string, shuffle bool) string {
		order := append([]string{}, keys...)
		if shuffle {
			rng.Shuffle(len(order), func(i, j int) { order[i], order[j] = order[j], order[i] })
		}
		var parts []string
		for _, k := range order {
			if v, ok := m[k]; ok {
				parts = append(parts, fmt.Sprintf("%q:%s", k, v))
			}
		}
		return "{" + strings.Join(parts, ",") + "}"
	}

	for n := 0; n < 20000; n++ {
		m := object()
		other := m
		if rng.Intn(2) == 0 {
			other = map[string]string{}
			for k, v := range m {
				other[k] = v
			}
			k := keys[rng.Intn(len(keys))]
			if rng.Intn(3) == 0 {
				delete(other, k)
			} else {
				other[k] = values[rng.Intn(len(values))]
			}
		}
		a, b := write(m, rng.Intn(2) == 0), write(other, rng.Intn(2) == 0)
		if got, want := equalJSON([]byte(a), []byte(b)), equalJSONByChildren([]byte(a), []byte(b)); got != want {
			t.Fatalf("equalJSON(%s, %s) = %v, reading by child says %v", a, b, got, want)
		}
	}
}

// A primitive's value and the element beside it are one item: two names whose
// given values match but whose extensions sit on different givens are not
// equal, nor are two whose positions a null holds differently.
func TestEqualJSONPairsAPrimitiveWithItsElement(t *testing.T) {
	const ext = `{"extension":[{"url":"u","valueString":"x"}]}`
	for _, tc := range []struct {
		a, b  string
		equal bool
	}{
		{`{"given":[null,"b"],"_given":[{"id":"x"},null]}`, `{"given":["b"],"_given":[{"id":"x"}]}`, false},
		{`{"given":["a","b"],"_given":[null,` + ext + `]}`, `{"given":["a","b"],"_given":[` + ext + `,null]}`, false},
		{`{"given":["a","b"],"_given":[null,` + ext + `]}`, `{"_given":[null,` + ext + `],"given":["a","b"]}`, true},
		{`{"given":["a"],"_given":[{}]}`, `{"given":["a"]}`, true},
		{`{"given":[null,"a"]}`, `{"given":["a"]}`, true},
		{`{"_active":` + ext + `}`, `{"_active":` + ext + `}`, true},
		{`{"_active":` + ext + `}`, `{"active":true}`, false},
	} {
		if got := equalJSON([]byte(tc.a), []byte(tc.b)); got != tc.equal {
			t.Errorf("equalJSON(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.equal)
		}
	}
}
