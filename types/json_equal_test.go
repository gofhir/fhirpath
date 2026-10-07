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
		{"key order", `{"a":1,"b":{"c":"x","d":[true,null]}}`, `{"b":{"d":[true,null],"c":"x"},"a":1}`, true},
		{"key order in an array item", `{"a":[{"x":1,"y":2}]}`, `{"a":[{"y":2,"x":1}]}`, true},
		{"layout inside a string", `{"a":"x y"}`, `{"a":"x  y"}`, false},
		{"a string holding layout and braces", `{"a":"{ \"b\" : 1 }"}`, `{"a":"{ \"b\" : 1 }"}`, true},
		{"an escaped quote, laid out", "{ \"a\" : \"x\\\"y\" }", `{"a":"x\"y"}`, true},
		{"an escaped backslash before a quote, laid out", "{ \"a\" : \"x\\\\\" }", `{"a":"x\\"}`, true},
		{"array order", `{"a":[1,2]}`, `{"a":[2,1]}`, false},
		{"an extra key", `{"a":1}`, `{"a":1,"b":2}`, false},
		{"a missing key", `{"a":1,"b":2}`, `{"a":1}`, false},
		{"the same bytes in other values", `{"a":"xy","b":"z"}`, `{"a":"xz","b":"y"}`, false},
		{"the same bytes under other keys", `{"ab":"c","d":"e"}`, `{"ab":"e","d":"c"}`, false},
		{"a value of another type", `{"a":"1"}`, `{"a":1}`, false},
		{"a number written another way", `{"a":1}`, `{"a":1.0}`, false},
		{"null against absent", `{"a":null,"b":1}`, `{"b":1}`, false},
		{"booleans", `{"a":true,"b":false}`, `{"b":false,"a":true}`, true},
		{"nested arrays", `{"a":[[1,{"x":1,"y":2}],[]]}`, `{"a":[[1,{"y":2,"x":1}],[]]}`, true},
		{"nested arrays, other length", `{"a":[[1],[]]}`, `{"a":[[1]]}`, false},
		// Found by FuzzSameJSON: a key written twice made up for one it lacked
		{"a duplicated key", `{"":0,"":0}`, `{"0":[],"":0}`, false},
		{"a duplicated key with another value", `{"x":1,"x":1}`, `{"x":1,"x":2}`, false},
		{"a duplicated key against one written once", `{"a":1,"a":1,"b":[]}`, `{"b":[],"a":1}`, false},
		{"an escape, keys reordered", `{"a":"x\"y","b":1}`, `{"b":1,"a":"x\"y"}`, false},
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

// FuzzSameJSON holds the property the comparison is built on: it never says
// two texts are the same when their content differs. The reference decodes
// both, with numbers kept as written, as sameJSON compares them. A false
// where the reference says true is allowed — a number or a string written
// differently, a key written twice, keys reordered around an escape — a true
// where it says false is not. Nor may two texts it finds the same have
// different fingerprints, which would keep both in a union.
func FuzzSameJSON(f *testing.F) {
	for _, seed := range [][2]string{
		{`{"a":1,"b":[1,2]}`, `{"b":[1,2],"a":1}`},
		{"{\n \"a\" : \"x y\" }", `{"a":"x  y"}`},
		{`{"a":"x\"y","b":{"c":null}}`, `{"b":{"c":null},"a":"x\"y"}`},
		{`{"a":"xy","b":"z"}`, `{"a":"xz","b":"y"}`},
		{`{"a":[{"x":1},{"y":2}]}`, `{"a":[{"y":2},{"x":1}]}`},
	} {
		f.Add(seed[0], seed[1])
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		// Whatever it is given, it answers without panicking
		_ = sameJSON([]byte(a), []byte(b))

		x, xok := decodeObject(a)
		y, yok := decodeObject(b)
		if !xok || !yok {
			return
		}
		same := sameJSON([]byte(a), []byte(b))
		if same && !reflect.DeepEqual(x, y) {
			t.Errorf("sameJSON(%q, %q) is true, but their content differs", a, b)
		}
		if same && jsonFingerprint([]byte(a)) != jsonFingerprint([]byte(b)) {
			t.Errorf("sameJSON(%q, %q) is true, but their fingerprints differ", a, b)
		}
	})
}

func decodeObject(s string) (map[string]any, bool) {
	if !json.Valid([]byte(s)) {
		return nil, false
	}
	decoder := json.NewDecoder(strings.NewReader(s))
	decoder.UseNumber()
	var v map[string]any
	if decoder.Decode(&v) != nil || v == nil || decoder.More() {
		return nil, false
	}
	return v, true
}

// FuzzSameJSONLayout holds the other half: what differs only in layout and in
// the order of keys is the same. Each input is rewritten with every object's
// keys reversed and laid out over lines, its values kept byte for byte, and the
// two must compare equal. An object with a key written twice is left out, since
// reversing it changes which value comes first.
func FuzzSameJSONLayout(f *testing.F) {
	for _, seed := range []string{
		`{"a":1,"b":[1,{"c":"x","d":null}]}`,
		`{"resourceType":"Patient","name":[{"family":"F","given":["G","H"]}],"active":true}`,
		`{"a":{"b":{"c":{"d":[[],{}]}}}}`,
		`{"":0,"x y":"a: b, c","q":"{\"k\":1}"}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, a string) {
		// A string is compared as written, escapes included, and ObjectEach
		// hands keys over unescaped: an input with an escape is left out
		if !json.Valid([]byte(a)) || !strings.HasPrefix(strings.TrimLeft(a, " \t\r\n"), "{") ||
			strings.Contains(a, "\\") {
			return
		}
		var b strings.Builder
		if !relayout(&b, []byte(a), jsonparser.Object, 0) {
			return
		}
		if !sameJSON([]byte(a), []byte(b.String())) {
			t.Errorf("sameJSON(%q, %q) is false, but they differ only in layout and key order", a, b.String())
		}
		if jsonFingerprint([]byte(a)) != jsonFingerprint([]byte(b.String())) {
			t.Errorf("fingerprints of %q and %q differ, but they differ only in layout and key order", a, b.String())
		}
	})
}

// relayout writes a JSON value with every object's keys reversed and one
// member per line, scalars as written. It reports false for an object with a
// key written twice.
func relayout(out *strings.Builder, value []byte, valueType jsonparser.ValueType, depth int) bool {
	indent := "\n" + strings.Repeat("  ", depth+1)
	switch valueType {
	case jsonparser.Object:
		type member struct {
			key, value []byte
			valueType  jsonparser.ValueType
		}
		var members []member
		seen := map[string]bool{}
		duplicate := false
		if jsonparser.ObjectEach(value, func(key, v []byte, vt jsonparser.ValueType, _ int) error {
			duplicate = duplicate || seen[string(key)]
			seen[string(key)] = true
			members = append(members, member{key, v, vt})
			return nil
		}) != nil || duplicate {
			return false
		}
		out.WriteString("{")
		for i := len(members) - 1; i >= 0; i-- {
			out.WriteString(indent + `"` + string(members[i].key) + `" : `)
			if !relayout(out, members[i].value, members[i].valueType, depth+1) {
				return false
			}
			if i > 0 {
				out.WriteString(",")
			}
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
		out.WriteString("]")
	case jsonparser.String:
		out.WriteString(`"` + string(value) + `"`)
	default:
		out.Write(value)
	}
	return true
}
