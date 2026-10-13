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
	values := []string{`"a"`, `"b"`, `"a b"`, `"a"`, `1`, `1.0`, `2`, `-0`, `0`, `true`, `false`, `null`,
		`{"x":"a"}`, `{"x":"b"}`, `{"y":1,"x":"a"}`, `{"x":"a","y":1.0}`, `[]`, `["a"]`, `["a","b"]`, `["b","a"]`, `[null,"a"]`}
	keys := []string{"code", "system", "display", "version", "extension", "id"}

	object := func() map[string]string {
		m := map[string]string{}
		for _, k := range keys {
			if rng.Intn(2) == 0 {
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
