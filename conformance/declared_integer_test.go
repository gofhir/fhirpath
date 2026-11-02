package conformance

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// What a declared integer type reads (#251). json.html §2.1.6.4.2: "The FHIR
// types integer, unsignedInt, positiveInt and decimal are represented as a JSON
// number … and all other types (including integer64) are represented as a JSON
// string". The R5 datatypes page: integer64 is "A signed integer in the range
// -9,223,372,036,854,775,808 to +9,223,372,036,854,775,807 (64-bit)", JSON
// "JSON String". So an integer64 is read from its string as the integer it
// writes (plan 016, D2 and D3: strconv.ParseInt's form), and a value outside
// that form, or a number outside the form of an integer type, is read as it is
// written and typed as declared (D1, as #231 reads a date outside its form).

const (
	attachmentSize = `{"resourceType":"DocumentReference","status":"current","content":[{"attachment":{"contentType":"text/plain","size":%s}}]}`
	sizePath       = "DocumentReference.content.attachment.size"
	observationInt = `{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueInteger":%s}`
	imagingStudy   = `{"resourceType":"ImagingStudy","status":"available","subject":{"reference":"Patient/p"},"numberOfSeries":%s}`
	telecomRank    = `{"resourceType":"Patient","telecom":[{"system":"phone","value":"1","rank":%s}]}`
)

// plainRender evaluates and renders what came back as the values' text joined
// by ";", empty for no value and "error" when the evaluation ends with one.
func plainRender(doc, expr string, opts ...fhirpath.EvalOption) string {
	result, err := fhirpath.MustCompile(expr).EvaluateWithOptions([]byte(doc), opts...)
	if err != nil {
		return "error"
	}
	items := make([]string, len(result))
	for i, item := range result {
		items[i] = item.String()
	}
	return strings.Join(items, ";")
}

// only keeps the model options named, "no model" among them.
func only(names ...string) []struct {
	name string
	opts []fhirpath.EvalOption
} {
	var kept []struct {
		name string
		opts []fhirpath.EvalOption
	}
	for _, m := range modelOptions() {
		for _, name := range names {
			if m.name == name {
				kept = append(kept, m)
			}
		}
	}
	return kept
}

type integerRow struct{ value, expr, want string }

func checkRows(t *testing.T, template, path string, rows []integerRow, models ...string) {
	t.Helper()
	for _, row := range rows {
		doc := fmt.Sprintf(template, row.value)
		expr := strings.ReplaceAll(row.expr, "X", path)
		for _, m := range only(models...) {
			if got := plainRender(doc, expr, m.opts...); got != row.want {
				t.Errorf("%s, %s over %s = %q, want %q", m.name, expr, row.value, got, row.want)
			}
		}
	}
}

// An integer64 written as the JSON string FHIR requires is the Integer it
// writes (D2, D3), typed integer64: R5 only, which is the only version with it.
func TestAnInteger64StringIsRead(t *testing.T) {
	rows := make([]integerRow, 0, 22)
	for expr, want := range map[string]string{
		"X + 1": "6", "X * 2": "10", "X div 1": "5", "X > 1": "true", "X = 5": "true",
		"X ~ 5": "true", "X.toInteger()": "5", "X.toString()": "5",
		"X is FHIR.integer64": "true", "X.type().name": "integer64", "X.hasValue()": "true",
		// A FHIR integer64 is not System.Integer, as a FHIR integer is not in
		// any of the three engines
		"X is System.Integer": "false",
	} {
		rows = append(rows, integerRow{`"5"`, expr, want})
	}
	rows = append(rows,
		// An Integer keeps no source text
		integerRow{`"+5"`, "X = 5", "true"}, integerRow{`"+5"`, "X.toString()", "5"},
		integerRow{`"-0"`, "X = 0", "true"}, integerRow{`"-0"`, "X + 1", "1"},
		integerRow{`"01"`, "X = 1", "true"},
		integerRow{`"-5"`, "X * 2", "-10"},
		integerRow{`"9223372036854775807"`, "X = 9223372036854775807", "true"},
		integerRow{`"-9223372036854775808"`, "X.toString()", "-9223372036854775808"},
		integerRow{`"-9223372036854775808"`, "X is FHIR.integer64", "true"},
		integerRow{`"-9223372036854775808"`, "X + 0", "-9223372036854775808"},
	)
	checkRows(t, attachmentSize, sizePath, rows, "r5")
}

// A string outside ParseInt's form stays the string it is, typed integer64, as
// #231 reads a value outside its form: no arithmetic, its text as written.
func TestAnInteger64OutsideTheFormStaysAString(t *testing.T) {
	for _, text := range []string{" 5", "", "1e2", "1.5", "9223372036854775808", "-9223372036854775809", "5x"} {
		doc := fmt.Sprintf(attachmentSize, `"`+text+`"`)
		for _, m := range only("r5") {
			if got := plainRender(doc, sizePath+" + 1", m.opts...); got != "error" {
				t.Errorf("%s: %q + 1 = %q, want an evaluation error", m.name, text, got)
			}
			if got := plainRender(doc, sizePath+".type().name", m.opts...); got != "integer64" {
				t.Errorf("%s: %q type().name = %q, want integer64", m.name, text, got)
			}
			if text == "" {
				continue
			}
			if got := plainRender(doc, sizePath+".toString()", m.opts...); got != text {
				t.Errorf("%s: %q toString() = %q, want the text as written", m.name, text, got)
			}
			if got := plainRender(doc, sizePath+".hasValue()", m.opts...); got != "true" {
				t.Errorf("%s: %q hasValue() = %q, want true", m.name, text, got)
			}
		}
	}
}

// The element beside an integer64 is kept.
func TestAnInteger64KeepsItsElement(t *testing.T) {
	doc := `{"resourceType":"DocumentReference","status":"current","content":[{"attachment":{"size":"7","_size":{"extension":[{"url":"http://example.org/x","valueString":"kept"}]}}}]}`
	for expr, want := range map[string]string{
		sizePath + " + 1":             "8",
		sizePath + ".extension.value": "kept",
	} {
		for _, m := range only("r5") {
			if got := plainRender(doc, expr, m.opts...); got != want {
				t.Errorf("%s: %s = %q, want %q", m.name, expr, got, want)
			}
		}
	}
}

// An Extension's valueInteger64 is read with the R5 model and without a model,
// where the key's suffix types it.
func TestAnExtensionInteger64IsRead(t *testing.T) {
	doc := `{"resourceType":"Patient","extension":[{"url":"http://e/i64","valueInteger64":"9"}]}`
	for expr, want := range map[string]string{
		"Patient.extension('http://e/i64').value + 1":         "10",
		"Patient.extension('http://e/i64').value.type().name": "integer64",
	} {
		for _, m := range only("r5", "no model") {
			if got := plainRender(doc, expr, m.opts...); got != want {
				t.Errorf("%s: %s = %q, want %q", m.name, expr, got, want)
			}
		}
	}
}

// Known split, out of scope (plan 016): the sizes of two Attachments are
// equal, while the Attachments are not, as whole-object equality compares the
// JSON as written.
func TestAnInteger64SplitsFromItsObject(t *testing.T) {
	doc := `{"resourceType":"DocumentReference","status":"current","content":[{"attachment":{"size":"5"}},{"attachment":{"size":"+5"}}]}`
	for expr, want := range map[string]string{
		"DocumentReference.content[0].attachment.size = DocumentReference.content[1].attachment.size": "true",
		"DocumentReference.content[0].attachment = DocumentReference.content[1].attachment":           "false",
	} {
		for _, m := range only("r5") {
			if got := plainRender(doc, expr, m.opts...); got != want {
				t.Errorf("%s: %s = %q, want %q", m.name, expr, got, want)
			}
		}
	}
}

// R4 and R4B have no integer64: Attachment.size is an unsignedInt, which a
// string leaves a String (plan 016, scope; SPEC-8).
func TestAnUnsignedIntStringStaysAString(t *testing.T) {
	checkRows(t, attachmentSize, sizePath, []integerRow{
		{`"5"`, "X + 1", "error"},
		{`"5"`, "X.type().name", "unsignedInt"},
	}, "r4", "r4b")
}

// D4: an integer written as a JSON string stays the string it is, typed
// integer; the references part on it (`= 1`: the validator true).
func TestAnIntegerWrittenAsAStringStaysAString(t *testing.T) {
	checkRows(t, observationInt, "Observation.value", []integerRow{
		{`"1"`, "X + 1", "error"},
		{`"1"`, "X = 1", "false"},
		{`"1"`, "X.toString()", "1"},
		{`"1"`, "X.type().name", "integer"},
	}, "r4", "r4b", "r5", "no model")
}

// D1: a number outside an integer type's form is read as written, typed as
// declared, in every version and without a model.
func TestANumberOutsideAnIntegerFormIsReadAsWritten(t *testing.T) {
	all := []string{"r4", "r4b", "r5", "no model"}
	checkRows(t, observationInt, "Observation.value", []integerRow{
		{"1.5", "X + 1", "2.5"}, {"1.5", "X * 2", "3"}, {"1.5", "X div 1", "1"},
		{"1.5", "X > 1", "true"}, {"1.5", "X = 1.5", "true"}, {"1.5", "X.toInteger()", ""},
		{"1.5", "X.toDecimal()", "1.5"}, {"1.5", "X is FHIR.integer", "true"},
		{"1.5", "X.type().name", "integer"}, {"1.5", "X.hasValue()", "true"},
		{"1e2", "X + 1", "101"}, {"1e2", "X.toString()", "1e2"},
		{"1.0", "X = 1", "true"}, {"1.0", "X + 1", "2"}, {"1.0", "X.toInteger()", ""},
		{"-1.5e1", "X + 1", "-14"},
		{"2147483648", "X + 1", "2147483649"},
		{"9223372036854775808", "X + 1", "9223372036854775809"},
	}, all...)
	checkRows(t, telecomRank, "Patient.telecom.rank", []integerRow{
		{"0", "X + 1", "1"}, {"1.5", "X + 1", "2.5"}, {"-1", "X + 1", "0"},
	}, all...)
	checkRows(t, imagingStudy, "ImagingStudy.numberOfSeries", []integerRow{
		{"-1", "X + 1", "0"}, {"1.5", "X + 1", "2.5"},
	}, all...)
}

// An integer64 written as a string answers as the same value written as an
// integer number: the arithmetic and the comparisons, for each form read. And
// the attachment's keys reordered, or an unknown extension and field added,
// change nothing.
func TestAnInteger64AnswersAsAnIntegerNumber(t *testing.T) {
	forms := map[string]string{
		`"5"`: "5", `"+5"`: "5", `"-0"`: "0", `"01"`: "1", `"-5"`: "-5",
		`"9223372036854775807"`: "9223372036854775807", `"-9223372036854775808"`: "-9223372036854775808",
	}
	exprs := []string{"X + 1", "X * 2", "X > 1", "X = 5", "X div 1"}
	attachments := []string{
		`{"contentType":"text/plain","size":%s}`,
		`{"size":%s,"contentType":"text/plain"}`,
		`{"contentType":"text/plain","extension":[{"url":"http://e/u","valueString":"z"}],"u":1,"size":%s}`,
	}
	for written, number := range forms {
		for _, expr := range exprs {
			want := plainRender(fmt.Sprintf(observationInt, number), strings.ReplaceAll(expr, "X", "Observation.value"), only("r5")[0].opts...)
			for _, attachment := range attachments {
				doc := `{"resourceType":"DocumentReference","status":"current","content":[{"attachment":` + fmt.Sprintf(attachment, written) + `}]}`
				for _, m := range only("r5") {
					if got := plainRender(doc, strings.ReplaceAll(expr, "X", sizePath), m.opts...); got != want {
						t.Errorf("%s: %s over %s = %q, want %q as for the number %s", m.name, expr, written, got, want, number)
					}
				}
			}
		}
	}
}

// A "5" integer64 is the Integer 5 however it is reached (D2): through
// children(), descendants(), repeat(), a cast, a Bundle's entry or a
// Parameters part.
func TestAnInteger64IsReadWhereverItIsReached(t *testing.T) {
	two := `{"resourceType":"DocumentReference","status":"current","content":[{"attachment":{"size":"5"}},{"attachment":{"size":"6"}}]}`
	one := `{"resourceType":"DocumentReference","status":"current","content":[{"attachment":{"size":"5"}}]}`
	bundle := `{"resourceType":"Bundle","type":"collection","entry":[{"resource":` + one + `}]}`
	params := `{"resourceType":"Parameters","parameter":[{"name":"p","part":[{"name":"q","valueInteger64":"5"}]}]}`
	for _, c := range []struct{ doc, expr, want string }{
		{two, "DocumentReference.content.attachment.children().ofType(integer64).select($this + 1)", "6;7"},
		{two, "DocumentReference.descendants().ofType(integer64).select($this + 1)", "6;7"},
		{two, "DocumentReference.content.repeat(attachment).size.select($this + 0)", "5;6"},
		{one, "(DocumentReference.content.attachment.size as integer64) + 1", "6"},
		{one, "DocumentReference.content.attachment.size is integer64", "true"},
		{bundle, "Bundle.entry.resource.content.attachment.size + 1", "6"},
		{params, "Parameters.parameter.part.value + 1", "6"},
	} {
		for _, m := range only("r5") {
			if got := plainRender(c.doc, c.expr, m.opts...); got != c.want {
				t.Errorf("%s: %s = %q, want %q", m.name, c.expr, got, c.want)
			}
		}
	}
}

// Without a model a choice's .value is typed by its suffix, and a value outside
// the form stays the string it is (D3). A choice key named directly
// (valueInteger64) is read untyped without a model, as valueDecimal is
// (pre-existing), so it is a String there.
func TestANoModelInteger64OutsideTheFormStaysAString(t *testing.T) {
	for _, text := range []string{"1e2", " 5"} {
		doc := `{"resourceType":"Patient","extension":[{"url":"http://e/i64","valueInteger64":"` + text + `"}]}`
		value := "Patient.extension('http://e/i64').value"
		for expr, want := range map[string]string{
			value + ".type().name": "integer64", value + " + 1": "error", value + ".toString()": text,
		} {
			for _, m := range only("no model") {
				if got := plainRender(doc, expr, m.opts...); got != want {
					t.Errorf("%s: %q %s = %q, want %q", m.name, text, expr, got, want)
				}
			}
		}
	}
	doc := `{"resourceType":"Patient","extension":[{"url":"http://e/i64","valueInteger64":"5"}]}`
	for _, m := range only("no model") {
		if got := plainRender(doc, "Patient.extension.valueInteger64.type().name", m.opts...); got != "String" {
			t.Errorf("%s: valueInteger64 named directly type().name = %q, want String", m.name, got)
		}
	}
}
