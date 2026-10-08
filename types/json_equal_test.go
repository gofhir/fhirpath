package types

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/buger/jsonparser"
)

func TestSameJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		a, b string
		want bool
	}{
		{"identical", `{"a":1}`, `{"a":1}`, true},
		{"layout", "{\n  \"a\" : 1,\n\t\"b\": [1, 2]\n}", `{"a":1,"b":[1,2]}`, true},
		{"layout at the edges", " {\"a\":1} \n", `{"a":1}`, true},
		{"layout around every punctuation", "{ \"a\" : [ 1 , { } , [ ] ] , \"b\" : null }", `{"a":[1,{},[]],"b":null}`, true},
		{"layout inside a string", `{"a":"x y"}`, `{"a":"x  y"}`, false},
		{"a string holding layout and braces", `{"a":"{ \"b\" : 1 }"}`, "{ \"a\" : \"{ \\\"b\\\" : 1 }\" }", true},
		{"an escaped quote, laid out", "{ \"a\" : \"x\\\"y\" }", `{"a":"x\"y"}`, true},
		{"an escaped backslash before a quote, laid out", "{ \"a\" : \"x\\\\\" }", `{"a":"x\\"}`, true},
		{"key order", `{"a":1,"b":2}`, `{"b":2,"a":1}`, false},
		{"array order", `{"a":[1,2]}`, `{"a":[2,1]}`, false},
		{"an extra key", `{"a":1}`, `{"a":1,"b":2}`, false},
		{"a number that is a prefix", `{"a":1}`, `{"a":10}`, false},
		{"a number written another way", `{"a":1}`, `{"a":1.0}`, false},
		{"a value of another type", `{"a":"1"}`, `{"a":1}`, false},
		{"empty and absent", `{"a":[]}`, `{}`, false},
		// Layout separates tokens: what is not JSON is not made equal to what is
		{"layout splitting a number", `{"a":1 2}`, `{"a":12}`, false},
		{"layout splitting a literal", `{"a":tr ue}`, `{"a":true}`, false},
		{"layout between a number and a key", `{"a":1 ,"b":2}`, `{"a":1,"b":2}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameJSON([]byte(tc.a), []byte(tc.b)); got != tc.want {
				t.Errorf("sameJSON(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
			if got := sameJSON([]byte(tc.b), []byte(tc.a)); got != tc.want {
				t.Errorf("sameJSON(%s, %s) = %v, want %v (swapped)", tc.b, tc.a, got, tc.want)
			}
		})
	}
}

// FuzzSameJSON holds sameJSON to a reference written the slow way, on any
// input, JSON or not: two texts are the same when they read alike once the
// layout between tokens is dropped, keeping one space where it separates two
// bytes of numbers or literals. On valid JSON, the same is also what
// encoding/json decodes alike, so sameJSON never says true where the content
// differs.
func FuzzSameJSON(f *testing.F) {
	for _, seed := range [][2]string{
		{`{"a":1,"b":[1,2]}`, "{ \"a\" : 1 , \"b\" : [ 1 , 2 ] }"},
		{"{\n \"a\" : \"x y\" }", `{"a":"x  y"}`},
		{`{"a":"x\"y","b":{"c":null}}`, "{\"a\":\"x\\\"y\",\n\"b\":{\"c\":null}}"},
		{`{"a":1 2}`, `{"a":12}`},
		{`{"a":[{"x":1},{"y":2}]}`, `{"a":[{"y":2},{"x":1}]}`},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		got := sameJSON([]byte(a), []byte(b))
		if want := withoutLayout(a) == withoutLayout(b); got != want {
			t.Fatalf("sameJSON(%q, %q) = %v, want %v", a, b, got, want)
		}
		if !got {
			return
		}
		x, xok := decodeObject(a)
		y, yok := decodeObject(b)
		if xok && yok && !reflect.DeepEqual(x, y) {
			t.Errorf("sameJSON(%q, %q) is true, but their content differs", a, b)
		}
	})
}

// withoutLayout drops the whitespace between tokens, keeping one space where
// it separates two bytes of numbers or literals.
func withoutLayout(s string) string {
	var out []byte
	inString, escaped, spaced := false, false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		if isJSONSpace(c) {
			spaced = true
			continue
		}
		if spaced && len(out) > 0 && isTokenByte(out[len(out)-1]) && isTokenByte(c) {
			out = append(out, ' ')
		}
		spaced = false
		if c == '"' {
			inString = true
		}
		out = append(out, c)
	}
	return string(out)
}

func decodeObject(s string) (map[string]any, bool) {
	if !json.Valid([]byte(s)) {
		return nil, false
	}
	decoder := json.NewDecoder(strings.NewReader(s))
	decoder.UseNumber()
	var v map[string]any
	if decoder.Decode(&v) != nil || v == nil {
		return nil, false
	}
	return v, true
}

// FuzzSameJSONLayout holds the other half on valid JSON: an object laid out
// over lines and indented, keys and values as written, is the same.
func FuzzSameJSONLayout(f *testing.F) {
	for _, seed := range []string{
		`{"a":1,"b":[1,{"c":"x","d":null}]}`,
		`{"resourceType":"Patient","name":[{"family":"F","given":["G","H"]}],"active":true}`,
		`{"a":{"b":{"c":{"d":[[],{}]}}}}`,
		`{"":0,"x y":"a: b, c","q":"{ k : 1 }","n":-1.5e3}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, a string) {
		// ObjectEach hands keys over unescaped, so an input with an escape
		// cannot be written back as it was: it is left out
		if !json.Valid([]byte(a)) || !strings.HasPrefix(strings.TrimLeft(a, " \t\r\n"), "{") ||
			strings.Contains(a, "\\") {
			return
		}
		var b strings.Builder
		if !relayout(&b, []byte(a), jsonparser.Object, 0) {
			return
		}
		if !sameJSON([]byte(a), []byte(b.String())) {
			t.Errorf("sameJSON(%q, %q) is false, but they differ only in layout", a, b.String())
		}
	})
}

// relayout writes a JSON value with one member or item per line, indented,
// keys and scalars as written. It reports false where it cannot.
func relayout(out *strings.Builder, value []byte, valueType jsonparser.ValueType, depth int) bool {
	indent := "\n" + strings.Repeat("  ", depth+1)
	switch valueType {
	case jsonparser.Object:
		out.WriteString("{")
		first := true
		if jsonparser.ObjectEach(value, func(key, v []byte, vt jsonparser.ValueType, _ int) error {
			if !first {
				out.WriteString(",")
			}
			first = false
			out.WriteString(indent + `"` + string(key) + `" : `)
			if !relayout(out, v, vt, depth+1) {
				return errStop
			}
			return nil
		}) != nil {
			return false
		}
		out.WriteString("\n" + strings.Repeat("  ", depth) + "}")
	case jsonparser.Array:
		out.WriteString("[")
		first, ok := true, true
		if _, err := jsonparser.ArrayEach(value, func(v []byte, vt jsonparser.ValueType, _ int, _ error) {
			if !first {
				out.WriteString(",")
			}
			first = false
			out.WriteString(indent)
			ok = ok && relayout(out, v, vt, depth+1)
		}); err != nil || !ok {
			return false
		}
		out.WriteString(" ]")
	case jsonparser.String:
		out.WriteString(`"` + string(value) + `"`)
	default:
		out.Write(value)
	}
	return true
}

var errStop = jsonparser.MalformedObjectError
