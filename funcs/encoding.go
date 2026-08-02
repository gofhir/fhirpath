// String encoding and escaping functions.
//
// Per the FHIRPath specification, section "String Manipulation" (Standard for
// Trial Use):
//
//	encode(format : String) : String    // hex, base64, urlbase64
//	decode(format : String) : String    // same formats as encode
//	escape(target : String) : String    // html, json
//	unescape(target : String) : String  // same targets as escape
//
// All four return empty for empty input, and empty when no format or target is
// given. An input that is not valid for the requested decoding also yields
// empty rather than an error, since the specification defines no error result.

package funcs

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"strings"
	"unicode/utf16"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

func init() {
	Register(FuncDef{Name: "encode", MinArgs: 1, MaxArgs: 1, Fn: fnEncode})
	Register(FuncDef{Name: "decode", MinArgs: 1, MaxArgs: 1, Fn: fnDecode})
	Register(FuncDef{Name: "escape", MinArgs: 1, MaxArgs: 1, Fn: fnEscape})
	Register(FuncDef{Name: "unescape", MinArgs: 1, MaxArgs: 1, Fn: fnUnescape})
}

// Encoding formats and escaping targets named by the specification.
const (
	formatHex       = "hex"
	formatBase64    = "base64"
	formatURLBase64 = "urlbase64"

	targetHTML = "html"
	targetJSON = "json"
)

// stringFuncArgs resolves the singleton string input and the single string
// argument shared by all four functions. ok is false when either is absent, in
// which case the caller returns empty.
func stringFuncArgs(input types.Collection, args []interface{}) (value, arg string, ok bool) {
	if input.Empty() {
		return "", "", false
	}
	value, ok, err := toString(input)
	if err != nil || !ok {
		return "", "", false
	}
	if len(args) == 0 {
		return "", "", false
	}
	arg, ok = toStringArg(args[0])
	if !ok {
		return "", "", false
	}
	return value, arg, true
}

// fnEncode encodes the input string in the given format.
func fnEncode(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	value, format, ok := stringFuncArgs(input, args)
	if !ok {
		return types.Collection{}, nil
	}

	switch format {
	case formatHex:
		return types.Collection{types.NewString(hex.EncodeToString([]byte(value)))}, nil
	case formatBase64:
		return types.Collection{types.NewString(base64.StdEncoding.EncodeToString([]byte(value)))}, nil
	case formatURLBase64:
		return types.Collection{types.NewString(base64.URLEncoding.EncodeToString([]byte(value)))}, nil
	}
	return types.Collection{}, nil
}

// fnDecode decodes the input string from the given format.
func fnDecode(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	value, format, ok := stringFuncArgs(input, args)
	if !ok {
		return types.Collection{}, nil
	}

	var (
		decoded []byte
		err     error
	)
	switch format {
	case formatHex:
		decoded, err = hex.DecodeString(value)
	case formatBase64:
		decoded, err = base64.StdEncoding.DecodeString(value)
	case formatURLBase64:
		decoded, err = base64.URLEncoding.DecodeString(value)
	default:
		return types.Collection{}, nil
	}

	if err != nil {
		// Not valid for this encoding: empty, per the absence of an error result
		return types.Collection{}, nil
	}
	return types.Collection{types.NewString(string(decoded))}, nil
}

// fnEscape escapes the input string for the given target.
func fnEscape(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	value, target, ok := stringFuncArgs(input, args)
	if !ok {
		return types.Collection{}, nil
	}

	switch target {
	case targetHTML:
		return types.Collection{types.NewString(escapeHTML(value))}, nil
	case targetJSON:
		return types.Collection{types.NewString(escapeJSON(value))}, nil
	}
	return types.Collection{}, nil
}

// fnUnescape reverses escaping of the input string for the given target.
func fnUnescape(_ *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
	value, target, ok := stringFuncArgs(input, args)
	if !ok {
		return types.Collection{}, nil
	}

	switch target {
	case targetHTML:
		// html.UnescapeString also resolves numeric and named references beyond
		// the ones escapeHTML produces, which is the lenient direction.
		return types.Collection{types.NewString(html.UnescapeString(value))}, nil
	case targetJSON:
		return types.Collection{types.NewString(unescapeJSON(value))}, nil
	}
	return types.Collection{}, nil
}

// escapeHTML escapes the characters that must not appear literally in HTML
// content. Named references are used rather than numeric ones ("&quot;" over
// "&#34;"), which is the form the specification's examples show.
// The ampersand goes first so that the escapes introduced are not re-escaped.
func escapeHTML(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(s)
}

// escapeJSON escapes a string so that it is valid as the contents of a JSON
// string. It escapes the contents only; no surrounding quotes are added.
func escapeJSON(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}

// unescapeJSON resolves JSON escape sequences. An unrecognized escape is kept
// verbatim, so that unescaping never loses characters.
func unescapeJSON(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] != '\\' || i+1 >= len(runes) {
			b.WriteRune(runes[i])
			continue
		}

		i++
		switch runes[i] {
		case '"', '\\', '/':
			b.WriteRune(runes[i])
		case 'b':
			b.WriteRune('\b')
		case 'f':
			b.WriteRune('\f')
		case 'n':
			b.WriteRune('\n')
		case 'r':
			b.WriteRune('\r')
		case 't':
			b.WriteRune('\t')
		case 'u':
			if r, consumed, ok := decodeJSONUnicode(runes[i+1:]); ok {
				b.WriteRune(r)
				i += consumed
				continue
			}
			b.WriteString(`\u`)
		default:
			// Not an escape sequence: keep both characters
			b.WriteRune('\\')
			b.WriteRune(runes[i])
		}
	}
	return b.String()
}

// decodeJSONUnicode reads a \uXXXX escape, joining a surrogate pair when one
// follows. consumed counts the runes read after the "u".
func decodeJSONUnicode(runes []rune) (r rune, consumed int, ok bool) {
	first, ok := parseHex4(runes)
	if !ok {
		return 0, 0, false
	}
	consumed = 4

	if utf16.IsSurrogate(first) && len(runes) >= 10 &&
		runes[4] == '\\' && runes[5] == 'u' {
		if second, ok := parseHex4(runes[6:]); ok {
			if joined := utf16.DecodeRune(first, second); joined != '�' {
				return joined, 10, true
			}
		}
	}
	return first, consumed, true
}

// parseHex4 parses exactly four hexadecimal digits.
//
// Four hex digits span 0x0000..0xFFFF, so the result is a code unit and is typed
// as one: every caller wants a rune, and none of them has to widen an int and
// hope it fits.
func parseHex4(runes []rune) (rune, bool) {
	if len(runes) < 4 {
		return 0, false
	}
	value := rune(0)
	for _, r := range runes[:4] {
		digit := rune(0)
		switch {
		case r >= '0' && r <= '9':
			digit = r - '0'
		case r >= 'a' && r <= 'f':
			digit = r - 'a' + 10
		case r >= 'A' && r <= 'F':
			digit = r - 'A' + 10
		default:
			return 0, false
		}
		value = value*16 + digit
	}
	return value, true
}
