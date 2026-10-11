package types

import "testing"

// FuzzEquivalentASCII holds the comparison of strings as written to the one
// String.Equivalent makes, on every text it decides.
func FuzzEquivalentASCII(f *testing.F) {
	for _, seed := range [][2]string{
		{"ABC", "abc"},
		{"  a \t b ", "a b"},
		{"a b", "ab"},
		{"", " "},
		{"x\ny", "X Y"},
		{"é", "É"},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		got, decided := equivalentASCII([]byte(a), []byte(b))
		if !decided {
			return
		}
		if want := NewString(a).Equivalent(NewString(b)); got != want {
			t.Fatalf("equivalentASCII(%q, %q) = %v, String.Equivalent says %v", a, b, got, want)
		}
	})
}

// FuzzEquivalentJSON checks that equivalence of objects is reflexive and
// symmetric, and holds wherever equality does.
func FuzzEquivalentJSON(f *testing.F) {
	for _, seed := range [][2]string{
		{`{"code":"ABC"}`, `{"code":"abc"}`},
		{`{"a":[{"x":1},{"y":2}]}`, `{"a":[{"y":2},{"x":1}]}`},
		{`{"a":1.0,"b":"x"}`, `{"b":"X","a":1}`},
		{`{"given":["A",null],"_given":[null,{"id":"x"}]}`, `{"given":["a",null],"_given":[null,{"id":"X"}]}`},
		{`{"d":"2020-01-01T10:00:00Z"}`, `{"d":"2020-01-01T10:00:00+00:00"}`},
		{`{"a":[]}`, `{}`},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		x, y := []byte(a), []byte(b)
		if !equivalentJSON(x, x) {
			t.Fatalf("equivalentJSON(%q, itself) is false", a)
		}
		ab, ba := equivalentJSON(x, y), equivalentJSON(y, x)
		if ab != ba {
			t.Fatalf("equivalentJSON(%q, %q) = %v, but the other way round %v", a, b, ab, ba)
		}
		if sameJSON(x, y) && !ab {
			t.Fatalf("%q and %q are equal but not equivalent", a, b)
		}
	})
}
