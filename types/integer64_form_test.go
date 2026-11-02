package types

import (
	"strconv"
	"strings"
	"testing"
)

// FuzzInteger64Form holds fhirString to the form of an integer64 (plan 016,
// D3): a string declared integer64 is read as an Integer exactly when
// strconv.ParseInt(s, 10, 64) reads it, and then to that value; otherwise it is
// a String typed integer64 with the same text. The model's name and the one a
// choice's key gives without a model (Integer64) read alike.
func FuzzInteger64Form(f *testing.F) {
	for _, seed := range []string{
		"5", "+5", "-0", "01", "-5", "9223372036854775807", "-9223372036854775808",
		"9223372036854775808", "-9223372036854775809", " 5", "", "1e2", "1.5", "5x",
		"+", "-", "+-5", "0x5", "5_0", "٥",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if strings.ContainsAny(text, "\\\"") {
			return
		}
		want, err := strconv.ParseInt(text, 10, 64)
		for _, fhirType := range []string{"integer64", "Integer64"} {
			value, ok := fhirString([]byte(text), fhirType)
			if !ok {
				t.Fatalf("%s %q not read as a FHIR type", fhirType, text)
			}
			if !strings.EqualFold(value.Type(), fhirInteger64) {
				t.Fatalf("%s %q typed %s, want integer64", fhirType, text, value.Type())
			}
			if err == nil {
				i, isInteger := value.(Integer)
				if !isInteger || i.Value() != want {
					t.Fatalf("%s %q read as %#v, want the Integer %d", fhirType, text, value, want)
				}
				continue
			}
			s, isString := value.(String)
			if !isString || s.Value() != text {
				t.Fatalf("%s %q read as %#v, want the String with the same text", fhirType, text, value)
			}
		}
	})
}
