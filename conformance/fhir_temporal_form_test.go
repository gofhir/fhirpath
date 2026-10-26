package conformance

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// A value a model declares dateTime, instant, date or time is read by FHIR's
// form for that type, not by the FHIRPath literal grammar (#231). FHIR's
// datatypes page, the same in R4, R4B and R5:
//
//   - dateTime: "The format is YYYY, YYYY-MM, YYYY-MM-DD or
//     YYYY-MM-DDThh:mm:ss+zz:zz … Seconds must be provided due to schema type
//     constraints";
//   - instant: "in the format YYYY-MM-DDThh:mm:ss.sss+zz:zz … The time SHALL
//     specified at least to the second and SHALL include a time zone";
//   - date: "YYYY, YYYY-MM, or YYYY-MM-DD";
//   - time: "in the format hh:mm:ss".
//
// A value outside its form is the string it is, typed as declared, as an
// off-calendar date already is (#145). Where the text and the engines part,
// the engines decide (CONFORMANCE.md, rule 1; measured with make three-engine,
// fhirpath.js 5.2.0 and validator 6.10.2, batteries in
// difftest/batteries/fhir-temporal-form): a dateTime with a time and no offset
// and a time without seconds are read by all three, so they are read here.

// temporalPlacements puts a value of each FHIR temporal type where a
// resource holds it, %s standing for the JSON value, with the expression that
// reaches it. Each path reaches the value by its own route: a choice, a
// declared element, a nested datatype, an extension, a repeating element,
// a contained resource, a Bundle entry, descendants().
var temporalPlacements = map[string][]struct{ resource, expr string }{
	"dateTime": {
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"effectiveDateTime":%s}`, "Observation.effective"},
		// the same with its keys in another order and an extension no
		// definition names beside it
		{`{"effectiveDateTime":%s,"extension":[{"url":"http://example.org/u","valueString":"u"}],"code":{"text":"c"},"status":"final","resourceType":"Observation"}`, "Observation.effective"},
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"effectivePeriod":{"start":%s}}`, "Observation.effective.start"},
		{`{"resourceType":"Patient","extension":[{"url":"http://example.org/d","valueDateTime":%s}]}`, "Patient.extension.value"},
		{`{"resourceType":"Patient","extension":[{"url":"http://example.org/a","extension":[{"url":"b","valueDateTime":%s}]}]}`, "Patient.extension.extension.value"},
		{`{"resourceType":"MedicationRequest","status":"active","intent":"order","subject":{"reference":"Patient/p"},"dosageInstruction":[{"timing":{"event":[null,%s]}}]}`, "MedicationRequest.dosageInstruction.timing.event"},
		{`{"resourceType":"Patient","contained":[{"resourceType":"Observation","status":"final","code":{"text":"c"},"effectiveDateTime":%s}]}`, "Patient.contained.effective"},
		{`{"resourceType":"Bundle","type":"collection","entry":[{"resource":{"resourceType":"Observation","status":"final","code":{"text":"c"},"effectiveDateTime":%s}}]}`, "Bundle.entry.resource.effective"},
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"effectiveDateTime":%s}`, "Observation.descendants().ofType(FHIR.dateTime)"},
	},
	"instant": {
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"issued":%s}`, "Observation.issued"},
		{`{"resourceType":"Patient","meta":{"lastUpdated":%s}}`, "Patient.meta.lastUpdated"},
		{`{"resourceType":"Patient","extension":[{"url":"http://example.org/i","valueInstant":%s}]}`, "Patient.extension.value"},
	},
	"date": {
		{`{"resourceType":"Patient","birthDate":%s}`, "Patient.birthDate"},
		{`{"resourceType":"Patient","extension":[{"url":"http://example.org/d","valueDate":%s}]}`, "Patient.extension.value"},
	},
	"time": {
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueTime":%s}`, "Observation.value"},
		{`{"resourceType":"Patient","extension":[{"url":"http://example.org/t","valueTime":%s}]}`, "Patient.extension.value"},
	},
}

// temporalForms lists, for each type, values in its form, which read as the
// System type FHIR maps it to, and values outside it, which read as a String.
var temporalForms = []struct {
	fhirType string
	system   string // the kind a value in form reads as
	valid    []string
	invalid  []string
}{
	{
		fhirType: "dateTime", system: "DateTime",
		valid: []string{
			"2020", "2020-01", "2020-01-15", "2020-01-15T10:30:00Z",
			"2020-01-15T10:30:00.123+05:00", "2016-12-31T23:59:60Z",
			"2020-01-15T10:30:00+14:00", "2020-01-15T10:30:00-12:00",
			// no offset: the three engines read it (FHIR: "SHALL be populated")
			"2020-01-15T10:30:00",
		},
		invalid: []string{
			// a time after a partial date: refused by fhirpath.js and Java
			"2020-01T10:30:00Z", "2020T10:30:00Z", "2020-01T10:30Z",
			// a trailing T, a time without seconds: refused by Java and the text
			"2020T", "2020-01T", "2020-01-15T", "2020-01-15T10:30Z",
			"2020-01-15T10", "2020-01-15T10Z", "2020-01-15T10:30", "2020-01-15T10:30+05:00",
			// an offset on a date: refused by the three engines in R4 and R5,
			// though R5's definition comments "UTC offset is allowed for dates
			// and partial dates"
			"2020-01-15Z", "2020-01+05:00",
			// a sign with no offset, which R5's regex accepts
			"2020-01-15T10:30:00+",
		},
	},
	{
		fhirType: "instant", system: "DateTime",
		valid: []string{"2020-01-15T10:30:00Z", "2020-01-15T10:30:00.123+05:00"},
		invalid: []string{
			// to the second and with an offset: fhirpath.js and the text refuse
			// these, the Java engine reads them
			"2020", "2020-01", "2020-01-15", "2020-01-15T10:30Z", "2020-01-15T10:30:00",
			"2020T", "2020-01-15T",
		},
	},
	{
		fhirType: "date", system: "Date",
		valid: []string{"2020", "2020-01", "2020-02-29"},
		// a time is no part of a date: fhirpath.js and the text refuse it,
		// the Java engine reads it
		invalid: []string{"2020-01-15T10:30:00Z", "2020T", "2020-01-15T"},
	},
	{
		fhirType: "time", system: "Time",
		// without seconds: the three engines read it (FHIR: "hh:mm:ss")
		valid: []string{"10:30:00", "10:30:00.123", "10:30", "10"},
		// a date is no time: fhirpath.js and Java refuse it
		invalid: []string{"2020-01-15", "2020-01-15T10:30:00Z", "2020", "T10:30:00"},
	},
}

// temporalKind names how a value was read: its Go type, the type it reports
// and its text. Under a model `is DateTime` and `is String` are false for a
// FHIR dateTime whichever way it was read, so the Go type is the oracle.
func temporalKind(result fhirpath.Collection) string {
	items := make([]string, len(result))
	for i, v := range result {
		kind := fmt.Sprintf("%T", v)
		switch v.(type) {
		case types.DateTime:
			kind = "DateTime"
		case types.Date:
			kind = "Date"
		case types.Time:
			kind = "Time"
		case types.String:
			kind = "String"
		}
		items[i] = kind + " " + v.Type() + " " + v.String()
	}
	return "[" + strings.Join(items, ", ") + "]"
}

func TestAFHIRTemporalValueIsReadByItsFHIRForm(t *testing.T) {
	for _, form := range temporalForms {
		for _, place := range temporalPlacements[form.fhirType] {
			for _, v := range versionModels {
				check := func(value, kind string) {
					doc := fmt.Sprintf(place.resource, `"`+value+`"`)
					result, err := fhirpath.MustCompile(place.expr).EvaluateWithOptions([]byte(doc), fhirpath.WithModel(v.model))
					if err != nil {
						t.Errorf("%s: %s over %s: %v", v.name, place.expr, doc, err)
						return
					}
					want := "[" + kind + " " + form.fhirType + " " + value + "]"
					if got := temporalKind(result); got != want {
						t.Errorf("%s: %s over %s = %s, want %s", v.name, place.expr, doc, got, want)
					}
				}
				for _, value := range form.valid {
					check(value, form.system)
				}
				for _, value := range form.invalid {
					check(value, "String")
				}
			}
		}
	}
}

// A value in form compares as a date or a time; one outside it is a string,
// and comparing it with a date is a type error, as fhirpath.js has it.
func TestAFHIRTemporalValueOutsideItsFormDoesNotCompareAsOne(t *testing.T) {
	for _, tt := range []struct{ doc, expr, want string }{
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"effectiveDateTime":"2020-01-15T10:30:00Z"}`, "Observation.effective > @1900", "[Boolean true]"},
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"effectiveDateTime":"2020-01T10:30:00Z"}`, "Observation.effective > @1900", "error"},
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"effectiveDateTime":"2020T"}`, "Observation.effective > @1900", "error"},
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"issued":"2020"}`, "Observation.issued > @1900", "error"},
		{`{"resourceType":"Patient","birthDate":"2020-01-15T10:30:00Z"}`, "Patient.birthDate > @1900", "error"},
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueTime":"2020-01-15"}`, "Observation.value > @1900", "error"},
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueTime":"10:30"}`, "Observation.value > @T09", "[Boolean true]"},
	} {
		for _, v := range versionModels {
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(tt.doc), fhirpath.WithModel(v.model))
			got := "error"
			if err == nil {
				got = typedItems(result)
			}
			if got != tt.want {
				t.Errorf("%s: %s over %s = %s (%v), want %s", v.name, tt.expr, tt.doc, got, err, tt.want)
			}
		}
	}
}

// The element FHIR writes beside a primitive stays with a value outside its
// form, as it does beside any other: its id, its extensions, and hasValue().
func TestAFHIRTemporalValueOutsideItsFormKeepsItsElement(t *testing.T) {
	doc := `{"resourceType":"Patient","birthDate":"2020-01-15T10:30:00Z",
		"_birthDate":{"id":"b","extension":[{"url":"http://example.org/u","valueString":"s"}]},
		"extension":[{"url":"http://example.org/e","_valueDateTime":{"id":"e"}}]}`
	for _, tt := range []struct{ expr, want string }{
		{"Patient.birthDate", "[String date 2020-01-15T10:30:00Z]"},
		{"Patient.birthDate.id", "(b)"},
		{"Patient.birthDate.hasValue()", "(true)"},
		{"Patient.birthDate.extension('http://example.org/u').value", "(s)"},
		// an extension whose value is only the element FHIR writes beside it
		{"Patient.extension.value.hasValue()", "(false)"},
		{"Patient.extension.value.id", "(e)"},
	} {
		for _, v := range versionModels {
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(doc), fhirpath.WithModel(v.model))
			got := temporalKind(result)
			if strings.HasPrefix(tt.want, "(") {
				got = "(" + strings.Trim(result.String(), "[]") + ")"
			}
			if err != nil {
				got = "error: " + err.Error()
			}
			if got != tt.want {
				t.Errorf("%s: %s = %s, want %s", v.name, tt.expr, got, tt.want)
			}
		}
	}
}

// Without a model no type is declared, and nothing here changes: a key's
// type suffix is a guess at the field, and a value is read by each route,
// the choice's name, the key itself or children(), as it was before FHIR's
// forms were applied to declared types (1.16.6). Where those routes agreed
// they still do; two cases where they already parted are pinned as they were,
// since reading them alike is no part of this change.
func TestWithoutAModelEveryRouteReadsAValueAsBefore(t *testing.T) {
	for _, tt := range []struct {
		key, value string
		want       []string // nil: the three routes agree
	}{
		{"valueDateTime", "2020T", nil},
		{"valueDateTime", "2020-01-15T10:30Z", nil},
		{"valueDateTime", "2020-01-15T10:30:00Z", nil},
		{"valueDate", "2020-01-15T10:30:00Z", nil},
		{"valueInstant", "2020", []string{"types.DateTime 2020", "types.Date 2020", "types.Date 2020"}},
		{"valueTime", "10:30", []string{"types.Time 10:30", "types.String 10:30", "types.String 10:30"}},
	} {
		doc := []byte(`{"resourceType":"Patient","extension":[{"url":"u","` + tt.key + `":"` + tt.value + `"}]}`)
		var kinds []string
		for _, expr := range []string{"Patient.extension.value", "Patient.extension." + tt.key, "Patient.extension.children().last()"} {
			result, err := fhirpath.MustCompile(expr).Evaluate(doc)
			if err != nil || len(result) != 1 {
				t.Fatalf("%s over %s: %v, %v", expr, doc, result, err)
			}
			kinds = append(kinds, fmt.Sprintf("%T %s", result[0], result[0].String()))
		}
		switch {
		case tt.want == nil && (kinds[0] != kinds[1] || kinds[1] != kinds[2]):
			t.Errorf("%s: the routes read %q", doc, kinds)
		case tt.want != nil && !slices.Equal(kinds, tt.want):
			t.Errorf("%s: the routes read %q, want %q", doc, kinds, tt.want)
		}
	}
}

// systemTypedModel types Patient.birthDate as a System type, as a custom model
// may: a System type is FHIRPath's, and keeps the literal grammar.
type systemTypedModel struct {
	fhirpath.Model
	system string
}

func (m systemTypedModel) TypeOf(path string) string {
	if path == "Patient.birthDate" {
		return "http://hl7.org/fhirpath/System." + m.system
	}
	return m.Model.TypeOf(path)
}

// FHIRPath's own grammar is untouched: its literals, conversions from a
// string, and a System type a model declares, which "@2015-02T" and
// "2015T10:30:00Z" are values of (fhirpath.g4: '@' DATEFORMAT 'T'
// (TIMEFORMAT TIMEZONEOFFSETFORMAT?)?).
func TestTheFHIRPathGrammarIsUnchanged(t *testing.T) {
	model := versionModels[0].model
	obs := `{"resourceType":"Observation","status":"final","code":{"text":"c"},"effectiveDateTime":"2020-01-15T10:30Z"}`
	for _, tt := range []struct {
		model     fhirpath.Model
		doc, expr string
		want      string
	}{
		{model, obs, "@2015-02T", "[DateTime DateTime 2015-02]"},
		{model, obs, "@2015T10:30:00Z", "[DateTime DateTime 2015T10:30:00Z]"},
		{model, obs, "'2020T'.toDateTime()", "[DateTime DateTime 2020]"},
		// a value outside its FHIR form still converts as a string does
		{model, obs, "Observation.effective.toDateTime()", "[DateTime DateTime 2020-01-15T10:30Z]"},
		{systemTypedModel{model, "DateTime"}, `{"resourceType":"Patient","birthDate":"2015-02T"}`, "Patient.birthDate", "[DateTime DateTime 2015-02]"},
		// and a value outside that grammar is a string, as a FHIR one is
		{systemTypedModel{model, "Date"}, `{"resourceType":"Patient","birthDate":"2020-01-15T10:30:00Z"}`, "Patient.birthDate", "[String String 2020-01-15T10:30:00Z]"},
		{systemTypedModel{model, "Time"}, `{"resourceType":"Patient","birthDate":"2020-01-15"}`, "Patient.birthDate", "[String String 2020-01-15]"},
		// still a primitive read from the resource, whose value getValue() gives
		{systemTypedModel{model, "Date"}, `{"resourceType":"Patient","birthDate":"2020-01-15T10:30:00Z"}`, "Patient.birthDate.getValue()", "[String String 2020-01-15T10:30:00Z]"},
		{systemTypedModel{model, "Time"}, `{"resourceType":"Patient","birthDate":"2020-01-15"}`, "Patient.birthDate.getValue()", "[String String 2020-01-15]"},
	} {
		result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(tt.doc), fhirpath.WithModel(tt.model))
		got := temporalKind(result)
		if err != nil {
			got = "error: " + err.Error()
		}
		if got != tt.want {
			t.Errorf("%s = %s, want %s", tt.expr, got, tt.want)
		}
	}
}
