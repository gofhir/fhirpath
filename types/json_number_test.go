package types

import (
	"encoding/json"
	"math/rand/v2"
	"testing"
)

// isJSONNumber is RFC 8259 §6's number grammar, the one a resource's number
// is read by (#238). encoding/json recognizes that grammar on its own, so it
// answers for every token: a lone number is a valid JSON text exactly when
// it is a number, whitespace aside, which ends a token before the engine sees
// it. It says nothing about what a value can hold, which
// decodeDecimal decides apart (an exponent past int32).
func TestIsJSONNumberIsRFC8259(t *testing.T) {
	rows := []string{
		"0", "-0", "1", "-1", "10", "0.5", "-0.5", "1.50", "1e5", "1E5", "1e+5",
		"1e-5", "1.5e2", "0e0", "1e2147483648", "123456789012345678901234567890",
		"", "-", "+1", ".5", "-.5", "5.", "1.", "-0.", "01", "00", "-01", "007.50",
		"1.e5", "1e", "1e+", "-e5", "e5", "1.5.5", "1e5e5", "1-2", "0x1", "1_000",
		"١", "Infinity", "NaN", "--1", "1E", "0.e1",
	}
	for _, s := range rows {
		if got, want := isJSONNumber(s), json.Valid([]byte(s)); got != want {
			t.Errorf("isJSONNumber(%q) = %v, encoding/json %v", s, got, want)
		}
	}
	// Random tokens over the characters a number is written with, and a few
	// it is not; no whitespace, which ends a token before the engine sees it
	const alphabet = "-+.eE0123456789x_"
	r := rand.New(rand.NewPCG(238, 1))
	for range 200000 {
		b := make([]byte, 1+r.IntN(8))
		for i := range b {
			b[i] = alphabet[r.IntN(len(alphabet))]
		}
		s := string(b)
		if got, want := isJSONNumber(s), json.Valid(b); got != want {
			t.Fatalf("isJSONNumber(%q) = %v, encoding/json %v", s, got, want)
		}
	}
}

// A token ends where jsonparser ends it, at whitespace, a comma or the close
// of an array or an object, and at nothing else.
func TestTokenEnd(t *testing.T) {
	for _, tc := range []struct {
		data string
		from int
		want int
	}{
		{"0x1,1", 0, 3},
		{"1_000]", 0, 5},
		{"-.5}", 0, 3},
		{"01 ", 0, 2},
		{"01\t,", 0, 2},
		{"01\n", 0, 2},
		{"01\r", 0, 2},
		{"[01]", 1, 3},
		{"1.5.5", 0, 5},
		{"", 0, 0},
		{"1e5,2", 2, 3},
	} {
		if got := tokenEnd([]byte(tc.data), tc.from); got != tc.want {
			t.Errorf("tokenEnd(%q, %d) = %d, want %d", tc.data, tc.from, got, tc.want)
		}
	}
}

// Each number outside the grammar is written as null, and nothing else is.
// The expectations are written by hand.
func TestNullBadNumbers(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{`{"a":1}`, `{"a":1}`},
		{`{"a":-.5}`, `{"a":null}`},
		{`{"a":01}`, `{"a":null}`},
		{`{"a":0x1}`, `{"a":null}`},
		{`{"a":1_000,"b":1}`, `{"a":null,"b":1}`},
		{`[-.5]`, `[null]`},
		{`[1,5.,2]`, `[1,null,2]`},
		{`[[-.5],[[01]]]`, `[[null],[[null]]]`},
		{`{"a":{"b":1e}}`, `{"a":{"b":null}}`},
		{`{"a":[1e,1e5]}`, `{"a":[null,1e5]}`},
		{"[ -.5 , 01\t,\n1.5.5\r]", "[ null , null\t,\nnull\r]"},
		{`{"a":-}`, `{"a":null}`},
		{`{"a":-,"b":1}`, `{"a":null,"b":1}`},
		{`{"a":"-.5"}`, `{"a":"-.5"}`},
		{`{"a":"a\"1.\"b","c":01}`, `{"a":"a\"1.\"b","c":null}`},
		{`{"a":"x\\","c":01}`, `{"a":"x\\","c":null}`},
		{`{"a":"x\\\"01","c":01}`, `{"a":"x\\\"01","c":null}`},
		{`{"a":"\"","b":"01"}`, `{"a":"\"","b":"01"}`},
		{`{"a":"x\`, `{"a":"x\`},
		{`{"a":"01`, `{"a":"01`},
		{`{"01":01}`, `{"01":null}`},
		{`{"a":1.50,"b":-0,"c":1E+5,"d":0.5}`, `{"a":1.50,"b":-0,"c":1E+5,"d":0.5}`},
		{`{"a":true,"b":null,"c":false}`, `{"a":true,"b":null,"c":false}`},
		{``, ``},
	} {
		if got := nullBadNumbers([]byte(tc.in)); got != tc.want {
			t.Errorf("nullBadNumbers(%s) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

// Text with nothing to rewrite comes back as the same bytes, not a copy.
func TestNullBadNumbersBytesKeepsTextWithNothingToRewrite(t *testing.T) {
	data := []byte(`{"a":[1,2.5,"x\"01"],"b":{"c":-0}}`)
	if got := nullBadNumbersBytes(data); &got[0] != &data[0] || len(got) != len(data) {
		t.Errorf("nullBadNumbersBytes copied %s", data)
	}
	if got := string(nullBadNumbersBytes([]byte(`[01]`))); got != `[null]` {
		t.Errorf("nullBadNumbersBytes([01]) = %s, want [null]", got)
	}
}

// An object or array whose only content is a number outside the grammar holds
// no value, ending where the token ends, as one holding null does.
func TestHoldsNoValueAtABadNumber(t *testing.T) {
	for _, tc := range []struct {
		data    string
		noValue bool
		end     int
	}{
		{`01`, true, 2},
		{`-.5}`, true, 3},
		{`0x1,1`, true, 3},
		{`1_000 `, true, 5},
		{`[0x1]`, true, 5},
		{`[01,-.5]`, true, 8},
		{`{"a":01}`, true, 8},
		{`{"a":{"b":1e},"c":null}`, true, 23},
		{`{"a":0x1,"b":1}`, false, 0},
		{`[0x1,1]`, false, 0},
		{`1`, false, 0},
		{`-1.5e2]`, false, 0},
		{`0`, false, 0},
	} {
		noValue, end := holdsNoValueAt([]byte(tc.data), 0)
		if noValue != tc.noValue || end != tc.end {
			t.Errorf("holdsNoValueAt(%s) = %v, %d, want %v, %d", tc.data, noValue, end, tc.noValue, tc.end)
		}
	}
}

// badNumberAt checks a token whole: 0x1 is no 0.
func TestBadNumberAtReadsTheWholeToken(t *testing.T) {
	for data, want := range map[string]bool{
		"0x1,1": true, "1_000]": true, "-.5}": true, "01 ": true, "0,1": false,
		"1.5e2]": false, "-0}": false, "1 x": false, "null": false, `"01"`: false,
	} {
		if got := badNumberAt([]byte(data), 0); got != want {
			t.Errorf("badNumberAt(%q) = %v, want %v", data, got, want)
		}
	}
}
