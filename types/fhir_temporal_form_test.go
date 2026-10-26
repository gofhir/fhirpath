package types

import (
	"regexp"
	"strings"
	"testing"
)

// The forms FHIR gives its temporal types, as the regex extension of
// <type>.value writes them in hl7.fhir.r4.core 4.0.1 (the same in R4B), the
// oracle for fhirString. Two are adapted by hand where the HL7 validator and
// fhirpath.js both read more than the regex, as the engine does
// (CONFORMANCE.md, rule 1): a dateTime's offset is optional, and a time may
// stop at the hour or the minute.
var (
	r4Year         = `([0-9]([0-9]([0-9][1-9]|[1-9]0)|[1-9]00)|[1-9]000)`
	r4Clock        = `([01][0-9]|2[0-3]):[0-5][0-9]:([0-5][0-9]|60)(\.[0-9]+)?`
	r4Offset       = `(Z|(\+|-)((0[0-9]|1[0-3]):[0-5][0-9]|14:00))`
	r4MonthDay     = `-(0[1-9]|1[0-2])-(0[1-9]|[1-2][0-9]|3[0-1])`
	fhirDateForm   = regexp.MustCompile(`^` + r4Year + `(-(0[1-9]|1[0-2])(-(0[1-9]|[1-2][0-9]|3[0-1]))?)?$`)
	fhirDTForm     = regexp.MustCompile(`^` + r4Year + `(-(0[1-9]|1[0-2])(-(0[1-9]|[1-2][0-9]|3[0-1])(T` + r4Clock + r4Offset + `?)?)?)?$`)
	fhirInstForm   = regexp.MustCompile(`^` + r4Year + r4MonthDay + `T` + r4Clock + r4Offset + `$`)
	fhirTimeForm   = regexp.MustCompile(`^([01][0-9]|2[0-3])(:[0-5][0-9](:([0-5][0-9]|60)(\.[0-9]+)?)?)?$`)
	temporalOracle = map[string]struct {
		form  *regexp.Regexp
		parse func(string) error
		kind  string
	}{
		"date":     {fhirDateForm, func(s string) error { _, err := NewDate(s); return err }, "Date"},
		"dateTime": {fhirDTForm, func(s string) error { _, err := NewDateTime(s); return err }, "DateTime"},
		"instant":  {fhirInstForm, func(s string) error { _, err := NewDateTime(s); return err }, "DateTime"},
		"time":     {fhirTimeForm, func(s string) error { _, err := NewTime(s); return err }, "Time"},
	}
)

func temporalKindOf(v Value) string {
	switch v.(type) {
	case Date:
		return "Date"
	case DateTime:
		return "DateTime"
	case Time:
		return "Time"
	case String:
		return "String"
	}
	return "other"
}

// FuzzFHIRTemporalForm holds fhirString to FHIR's forms both ways: a text is
// read as its type's System value exactly when it is in the form and on the
// calendar and the clock, which the parser decides, and as a String
// otherwise, never as another temporal type.
func FuzzFHIRTemporalForm(f *testing.F) {
	for _, seed := range []string{
		"2020", "2020-01", "2020-01-15", "2020-01-15T10:30:00Z", "2020-01-15T10:30:00.123+05:00",
		"2020-01-15T10:30:00", "2016-12-31T23:59:60Z", "2020-02-30", "2020T", "2020-01T",
		"2020-01-15T", "2020-01T10:30:00Z", "2020-01-15T10:30Z", "2020-01-15T10", "2020-01-15T10+05:00",
		"2020-01-15T10:30+05:00", "2020-01-15Z", "10", "10:30", "10:30:00", "T10:30:00", "24:00:00",
		"2020-01-15T10:30:00+14:00", "2020-01-15T10:30:00+14:01", "0000", "2020-01-15T10:30:00+",
		"2020-01-15T10:30", "2020-01-15T10:30:0", "2020-01-15T10:30:00.",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if strings.ContainsAny(text, "\\\"") {
			return
		}
		for fhirType, oracle := range temporalOracle {
			want := "String"
			if oracle.form.MatchString(text) && oracle.parse(text) == nil {
				want = oracle.kind
			}
			value, ok := fhirString([]byte(text), fhirType)
			if !ok {
				t.Fatalf("%s %q not read as a FHIR type", fhirType, text)
			}
			if got := temporalKindOf(value); got != want {
				t.Fatalf("%s %q read as %s, want %s", fhirType, text, got, want)
			}
		}
	})
}
