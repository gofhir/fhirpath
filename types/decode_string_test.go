package types

import (
	"encoding/json"
	"testing"
)

// decodeJSONString reads a JSON string's content exactly as encoding/json
// would, which it used to call: every escape, surrogate pairs and lone
// surrogates included, and content encoding/json refuses read as it stands.
func TestDecodeJSONStringAnswersAsEncodingJSON(t *testing.T) {
	for _, raw := range []string{
		``, `plain`, `é unicode`, `a\"b`, `back\\slash`, `sl\/ash`, `\b\f\n\r\t`,
		`é`, `é`, `😀`, `x\ud83dy`, `\ude00`, `\ud83dA`,
		`<div xmlns=\"http://www.w3.org/1999/xhtml\">\n  <p>text</p>\n</div>`,
		`trailing\\`, `\u0000nul`, `\u001f`,
		// Not valid JSON string content.
		`bad\q`, `\u12`, `\u12zz`, `end\`,
	} {
		want := raw
		var decoded string
		if err := json.Unmarshal([]byte(`"`+raw+`"`), &decoded); err == nil {
			want = decoded
		}
		if got := decodeJSONString([]byte(raw)); got != want {
			t.Errorf("decodeJSONString(%q) = %q, want %q", raw, got, want)
		}
	}
}

func BenchmarkDecodeJSONString(b *testing.B) {
	narrative := []byte(`<div xmlns=\"http://www.w3.org/1999/xhtml\">\n  <p><b>Generated Narrative with Details</b></p>\n  <p><b>id</b>: example</p>\n  <p><b>status</b>: final</p>\n</div>`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = decodeJSONString(narrative)
	}
}
