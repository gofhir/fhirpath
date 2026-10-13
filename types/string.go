package types

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// String represents a FHIRPath string value.
type String struct {
	value    string
	fhirType string // optional FHIR type (e.g., "Oid", "Url", "Uuid", "Id") for subtype discrimination
	read     bool   // read from the input, so a FHIR primitive. See IsFHIRPrimitive.

	// The FHIR element this value was read with, when it carried one
	primitiveElement
}

// NewString creates a new String value.
func NewString(v string) String {
	return String{value: v}
}

// NewStringWithFHIRType creates a new String value with an explicit FHIR type.
// Used for URI subtypes (id, oid, url, uuid, code, etc.) to preserve type identity
// so that ofType() can discriminate between them.
func NewStringWithFHIRType(v, fhirType string) String {
	return String{value: v, fhirType: fhirType}
}

// Value returns the underlying string value.
func (s String) Value() string {
	return s.value
}

// Type returns the FHIR type if set (e.g., "Oid", "Url"), otherwise "String".
func (s String) Type() string {
	if s.fhirType != "" {
		return s.fhirType
	}
	return TypeNameString
}

// Equal returns true if other is a String with the same value.
func (s String) Equal(other Value) bool {
	if o, ok := other.(String); ok {
		return s.value == o.value
	}
	return false
}

// Equivalent compares strings "ignoring case and locale, and normalizing
// whitespace": character by character, a letter in either case, and any
// whitespace character, of the Unicode White_Space class FHIRPath 3.0.0
// names, for any other. "Multiple whitespace characters in sequence are not
// collapsed", nor are the ends trimmed: 'a b' ~ 'a\tb' is true, 'a b' ~ 'a  b'
// and ' a' ~ 'a' false, as fhirpath.js answers. Case is set aside one
// character for another, by Unicode's simple folding; fhirpath.js, comparing
// upper case, also takes ß for SS.
func (s String) Equivalent(other Value) bool {
	if o, ok := other.(String); ok {
		return stringsEquivalent(s.value, o.value)
	}
	return false
}

// stringsEquivalent compares two strings as String.Equivalent does.
func stringsEquivalent(a, b string) bool {
	if a == b {
		return true
	}
	for a != "" && b != "" {
		if a[0] < utf8.RuneSelf && b[0] < utf8.RuneSelf {
			if !asciiEquivalent(a[0], b[0]) {
				return false
			}
			a, b = a[1:], b[1:]
			continue
		}
		ra, na := utf8.DecodeRuneInString(a)
		rb, nb := utf8.DecodeRuneInString(b)
		if !runesEquivalent(ra, rb) {
			return false
		}
		a, b = a[na:], b[nb:]
	}
	return a == "" && b == ""
}

// asciiEquivalent is String.Equivalent's rule for two ASCII characters: the
// same, the same letter in another case, or both whitespace. The comparison
// of JSON texts applies it too.
func asciiEquivalent(a, b byte) bool {
	return a == b || isASCIIWhiteSpace(a) && isASCIIWhiteSpace(b) || lowerASCII(a) == lowerASCII(b)
}

// isASCIIWhiteSpace reports the ASCII characters of Unicode's White_Space
// class: tab, line feed, vertical tab, form feed, carriage return and space.
func isASCIIWhiteSpace(c byte) bool {
	return c == ' ' || c >= '\t' && c <= '\r'
}

// lowerASCII lowercases an ASCII letter.
func lowerASCII(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

// runesEquivalent reports two characters equal, the same letter in another
// case, or both whitespace.
func runesEquivalent(a, b rune) bool {
	if a == b {
		return true
	}
	if unicode.Is(unicode.White_Space, a) && unicode.Is(unicode.White_Space, b) {
		return true
	}
	for folded := unicode.SimpleFold(a); folded != a; folded = unicode.SimpleFold(folded) {
		if folded == b {
			return true
		}
	}
	return false
}

// String returns the string value.
func (s String) String() string {
	return s.value
}

// IsEmpty returns true if the string is empty.
func (s String) IsEmpty() bool {
	return s.value == ""
}

// Length returns the number of characters.
func (s String) Length() int {
	return len([]rune(s.value))
}

// Contains returns true if the string contains the substring.
func (s String) Contains(substr string) bool {
	return strings.Contains(s.value, substr)
}

// StartsWith returns true if the string starts with the prefix.
func (s String) StartsWith(prefix string) bool {
	return strings.HasPrefix(s.value, prefix)
}

// EndsWith returns true if the string ends with the suffix.
func (s String) EndsWith(suffix string) bool {
	return strings.HasSuffix(s.value, suffix)
}

// Upper returns a new String with all characters uppercase.
func (s String) Upper() String {
	return NewString(strings.ToUpper(s.value))
}

// Lower returns a new String with all characters lowercase.
func (s String) Lower() String {
	return NewString(strings.ToLower(s.value))
}

// Compare compares two strings lexicographically.
func (s String) Compare(other Value) (int, error) {
	if o, ok := other.(String); ok {
		return strings.Compare(s.value, o.value), nil
	}
	return 0, NewTypeError("String", other.Type(), "comparison")
}

// IndexOf returns the index of the first occurrence of substr, or -1.
func (s String) IndexOf(substr string) int {
	return strings.Index(s.value, substr)
}

// Substring returns a substring starting at start with the given length.
func (s String) Substring(start, length int) String {
	runes := []rune(s.value)
	if start < 0 || start >= len(runes) {
		return NewString("")
	}
	end := start + length
	if end > len(runes) {
		end = len(runes)
	}
	return NewString(string(runes[start:end]))
}

// Replace returns a new String with all occurrences of old replaced by replacement.
func (s String) Replace(old, replacement string) String {
	return NewString(strings.ReplaceAll(s.value, old, replacement))
}

// ToChars returns a collection of single-character strings.
func (s String) ToChars() Collection {
	runes := []rune(s.value)
	result := make(Collection, len(runes))
	for i, r := range runes {
		result[i] = NewString(string(r))
	}
	return result
}

// WithFHIRType returns a copy that reports the FHIR type it was declared with.
// FHIR primitives are types in their own right — a FHIR.code is not a
// System.String — so a value keeps the name the model gave it.
func (s String) WithFHIRType(fhirType string) String {
	s.fhirType = fhirType
	return s
}

// WithElement returns a copy carrying the FHIR element that accompanied the
// value in the JSON, which is where its extensions and id live.
func (s String) WithElement(element *ObjectValue) String {
	s.element = element
	return s
}
