package conformance

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// A resource's number is read by JSON's grammar (#238), RFC 8259 §6:
// "number = [ minus ] int [ frac ] [ exp ]", "int = zero / ( digit1-9
// *DIGIT )", "frac = decimal-point 1*DIGIT", "Leading zeros are not
// allowed". FHIR R4 and R4B's decimal regex is that grammar,
// -?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?. A token outside it is no
// number, and reads as a null does (plan 015, D1): the primitive has no
// value, and its element (_valueDecimal), when there is one, stays.

// nonJSONNumbers are tokens jsonparser hands the engine as numbers that RFC
// 8259 refuses. .5 and +1 are refused by jsonparser itself, and never reach
// it.
var nonJSONNumbers = []string{
	"-.5", "5.", "1.", "-0.", "007.50", "1.e5", "01", "00", "-01", "-",
	"1e", "1e+", "-e5", "1.5.5", "1e5e5", "1-2", "0x1", "1_000", "1_5",
}

// numberPlacements puts a number where a resource holds one, %s standing for
// the JSON value, with the expressions that reach it. Each reaches the value
// by its own route: a declared decimal and its element, a choice in an
// extension and its element, a Quantity, a Money, an integer, a repeating
// element, a contained resource, a Bundle entry, descendants(), and an
// object no model knows, whose type is told by the fields it carries.
var numberPlacements = []struct{ resource, expr string }{
	{`{"resourceType":"RiskAssessment","status":"final","subject":{"reference":"Patient/p"},"prediction":[{"probabilityDecimal":%s,"_probabilityDecimal":{"id":"e","extension":[{"url":"http://example.org/x","valueString":"kept"}]}}]}`, "RiskAssessment.prediction.probability"},
	{`{"resourceType":"RiskAssessment","status":"final","subject":{"reference":"Patient/p"},"prediction":[{"probabilityDecimal":0.25},{"probabilityDecimal":%s}]}`, "RiskAssessment.prediction.probability"},
	{`{"resourceType":"Patient","extension":[{"url":"http://example.org/d","valueDecimal":%s,"_valueDecimal":{"extension":[{"url":"http://example.org/x","valueString":"kept"}]}}]}`, "Patient.extension.value"},
	{`{"resourceType":"Patient","extension":[{"url":"http://example.org/d","valueDecimal":%s}]}`, "Patient.extension.value"},
	{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":{"value":%s,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}`, "Observation.value"},
	{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":{"value":%s,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}`, "Observation.value.value"},
	{`{"resourceType":"Invoice","status":"issued","totalNet":{"value":%s,"currency":"EUR"}}`, "Invoice.totalNet"},
	{`{"resourceType":"Invoice","status":"issued","totalNet":{"value":%s,"currency":"EUR"}}`, "Invoice.totalNet.value"},
	{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueInteger":%s}`, "Observation.value"},
	{`{"resourceType":"Patient","contained":[{"resourceType":"RiskAssessment","status":"final","subject":{"reference":"Patient/p"},"prediction":[{"probabilityDecimal":%s}]}]}`, "Patient.contained.prediction.probability"},
	{`{"resourceType":"Bundle","type":"collection","entry":[{"resource":{"resourceType":"RiskAssessment","status":"final","subject":{"reference":"Patient/p"},"prediction":[{"probabilityDecimal":%s}]}}]}`, "Bundle.entry.resource.prediction.probability"},
	{`{"resourceType":"RiskAssessment","status":"final","subject":{"reference":"Patient/p"},"prediction":[{"probabilityDecimal":%s}]}`, "RiskAssessment.descendants().where($this is FHIR.decimal or $this is System.Decimal)"},
	{`{"resourceType":"Patient","x":{"value":%s,"unit":"mg"}}`, "Patient.x"},
}

// numberQuestions are asked of each placement's expression, X: whether it is
// there, its value and element, how it computes, converts and compares.
var numberQuestions = []string{
	"X", "X.exists()", "X.count()", "X.hasValue()", "X.getValue()",
	"X.extension.value", "X.id", "X.children().count()", "X + 1",
	"X.toString()", "X.toDecimal()", "X.toQuantity()", "X.ofType(FHIR.decimal).count()",
	"X = X", "X ~ X", "X > 0", "X.type().name", "%resource.descendants().count()",
}

// evaluateRendered evaluates and renders what came back, the error included,
// so that two evaluations compare as text.
func evaluateRendered(doc, expr string, opts ...fhirpath.EvalOption) string {
	result, err := fhirpath.MustCompile(expr).EvaluateWithOptions([]byte(doc), opts...)
	if err != nil {
		return "error: " + err.Error()
	}
	items := make([]string, len(result))
	for i, item := range result {
		items[i] = item.Type() + " " + item.String()
	}
	return "[" + strings.Join(items, "; ") + "]"
}

// modelOptions are the ways a resource is read: by each version's model and
// by none.
func modelOptions() []struct {
	name string
	opts []fhirpath.EvalOption
} {
	all := make([]struct {
		name string
		opts []fhirpath.EvalOption
	}, 0, 1+len(versionModels))
	all = append(all, struct {
		name string
		opts []fhirpath.EvalOption
	}{"no model", nil})
	for _, v := range versionModels {
		all = append(all, struct {
			name string
			opts []fhirpath.EvalOption
		}{v.name, []fhirpath.EvalOption{fhirpath.WithModel(v.model)}})
	}
	return all
}

// A number outside JSON's grammar reads as null in its place does, by every
// route and for every question, R4, R4B and R5 and without a model.
func TestANonJSONNumberReadsAsNull(t *testing.T) {
	for _, place := range numberPlacements {
		withNull := fmt.Sprintf(place.resource, "null")
		for _, number := range nonJSONNumbers {
			doc := fmt.Sprintf(place.resource, number)
			for _, question := range numberQuestions {
				expr := strings.ReplaceAll(question, "X", "("+place.expr+")")
				for _, m := range modelOptions() {
					got, want := evaluateRendered(doc, expr, m.opts...), evaluateRendered(withNull, expr, m.opts...)
					if got != want {
						t.Errorf("%s, %s written %s: %s, want %s as with null", m.name, expr, number, got, want)
					}
				}
			}
		}
	}
}

// The element beside a number outside the grammar is kept, with its id and
// extensions, as beside a null: FHIR JSON, "the value and the extensions are
// represented as properties"; the value is absent, the element is not.
func TestANonJSONNumberKeepsItsElement(t *testing.T) {
	doc := fmt.Sprintf(numberPlacements[0].resource, "-.5")
	for expr, want := range map[string]string{
		"RiskAssessment.prediction.probability.exists()":                "[Boolean true]",
		"RiskAssessment.prediction.probability.hasValue()":              "[Boolean false]",
		"RiskAssessment.prediction.probability.extension.value":         "[string kept]",
		"RiskAssessment.prediction.probability.id":                      "[String e]",
		"RiskAssessment.prediction.probability.getValue().exists()":     "[Boolean false]",
		"RiskAssessment.prediction.probability.ofType(FHIR.decimal).id": "[String e]",
	} {
		for _, v := range versionModels {
			if got := evaluateRendered(doc, expr, fhirpath.WithModel(v.model)); got != want {
				t.Errorf("%s: %s = %s, want %s", v.name, expr, got, want)
			}
		}
	}
	// Without an element, a path to it is empty
	doc = fmt.Sprintf(numberPlacements[3].resource, "007.50")
	for _, m := range modelOptions() {
		if got := evaluateRendered(doc, "Patient.extension.value.exists()", m.opts...); got != "[Boolean false]" {
			t.Errorf("%s: Patient.extension.value.exists() = %s, want false", m.name, got)
		}
	}
}

// Objects holding a number outside the grammar compare as they would holding
// null there: alike when both do, whatever each wrote, and unlike one holding
// a number. Identical, rewritten and reordered, in an extension and in a
// Money.
func TestObjectsWithANonJSONNumberCompareAsWithNull(t *testing.T) {
	pairs := [][2]string{
		{`{"url":"http://example.org/d","valueDecimal":%s}`, `{"url":"http://example.org/d","valueDecimal":%s}`},
		{`{"url":"http://example.org/d","valueDecimal":%s}`, `{"valueDecimal":%s,"url":"http://example.org/d"}`},
		{`{"url":"http://example.org/d","valueDecimal":%s}`, `{"url":"http://example.org/d","valueDecimal":-0.5}%.0s`},
		{`{"url":"http://example.org/d","valueDecimal":%s}`, `{"url":"http://example.org/d","valueDecimal":null}%.0s`},
		{`{"url":"http://example.org/d","valueDecimal":-0.5}%.0s`, `{"url":"http://example.org/d","valueDecimal":%s}`},
		{`{"url":"http://example.org/m","valueMoney":{"value":%s,"currency":"EUR"}}`, `{"url":"http://example.org/m","valueMoney":{"value":%s,"currency":"EUR"}}`},
		{`{"url":"http://example.org/m","valueMoney":{"value":%s,"currency":"EUR"}}`, `{"url":"http://example.org/m","valueMoney":{"value":-0.5,"currency":"EUR"}}%.0s`},
	}
	questions := []string{
		"Patient.extension[0] = Patient.extension[1]",
		"Patient.extension[0] ~ Patient.extension[1]",
		"Patient.extension.distinct().count()",
		"(Patient.extension[0] | Patient.extension[1]).count()",
		"Patient.extension[0].value = Patient.extension[1].value",
	}
	for _, pair := range pairs {
		for _, left := range nonJSONNumbers {
			// 1_6 differs from 1_5 only past a byte no number holds, where
			// reading from the difference starts mid-token, at a valid 6
			for _, right := range []string{left, "-.50", "01", "1_6"} {
				doc := `{"resourceType":"Patient","extension":[` + fmt.Sprintf(pair[0], left) + "," + fmt.Sprintf(pair[1], right) + `]}`
				withNull := `{"resourceType":"Patient","extension":[` + fmt.Sprintf(pair[0], "null") + "," + fmt.Sprintf(pair[1], "null") + `]}`
				for _, question := range questions {
					for _, m := range modelOptions() {
						got, want := evaluateRendered(doc, question, m.opts...), evaluateRendered(withNull, question, m.opts...)
						if got != want {
							t.Errorf("%s, %s over %s: %s, want %s as with null", m.name, question, doc, got, want)
						}
					}
				}
			}
		}
	}
}

// Every JSON number is read as before: the value it writes, computed by
// FHIRPath's arithmetic and compared by value, R4, R4B and R5 and without a
// model.
func TestAJSONNumberIsReadAsWritten(t *testing.T) {
	for number, plusOne := range map[string]string{
		"-0":     "1",
		"0":      "1",
		"0.0":    "1.0",
		"-0.0":   "1.0",
		"1.5":    "2.5",
		"1E5":    "100001",
		"1e+05":  "100001",
		"1e-5":   "1.00001",
		"-1.5E1": "-14",
		"10":     "11",
	} {
		doc := fmt.Sprintf(numberPlacements[3].resource, number)
		expr := "Patient.extension.value + 1 = " + plusOne
		for _, m := range modelOptions() {
			if got := evaluateRendered(doc, expr, m.opts...); got != "[Boolean true]" {
				t.Errorf("%s: %s over %s = %s, want true", m.name, expr, number, got)
			}
		}
	}
}

// numberAtToken replaces NUM in a template by the token, which is how the
// tests below write a number where a resource holds one.
func numberAtToken(template, token string) string { return strings.ReplaceAll(template, "NUM", token) }

// An object whose only content is a number outside the grammar holds no value,
// as one holding null does, for exists(), empty() and hasValue() (#238).
func TestAnObjectHoldingOnlyANonJSONNumberHoldsNoValue(t *testing.T) {
	for _, place := range []struct{ resource, expr string }{
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":{"value":NUM}}`, "Observation.value"},
		{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":{"value":NUM}}`, "Observation.valueQuantity"},
		{`{"resourceType":"Patient","x":{"a":NUM}}`, "Patient.x"},
		{`{"resourceType":"Patient","x":{"a":{"b":NUM}}}`, "Patient.x"},
		{`{"resourceType":"Patient","x":{"a":[NUM]}}`, "Patient.x"},
		{`{"resourceType":"Patient","x":{"a":[NUM,NUM]}}`, "Patient.x"},
		{`{"resourceType":"Patient","x":[{"a":NUM}]}`, "Patient.x"},
		{`{"resourceType":"Patient","x":{"a":NUM }}`, "Patient.x"},
		{`{"resourceType":"Patient","x":{"a":NUM,"b":null}}`, "Patient.x"},
	} {
		for _, number := range nonJSONNumbers {
			doc, withNull := numberAtToken(place.resource, number), numberAtToken(place.resource, "null")
			for _, question := range []string{"X.exists()", "X.empty()", "X.hasValue()", "X.count()", "X.children().count()", "X | X"} {
				expr := strings.ReplaceAll(question, "X", place.expr)
				for _, m := range modelOptions() {
					got, want := evaluateRendered(doc, expr, m.opts...), evaluateRendered(withNull, expr, m.opts...)
					if got != want {
						t.Errorf("%s, %s over %s: %s, want %s as with null", m.name, expr, doc, got, want)
					}
				}
			}
		}
	}
}

// A number outside the grammar inside an array, or in arrays nested in it,
// compares as null does, however the arrays are laid out and whatever follows
// it: by =, ~, distinct() and union (#238).
func TestANonJSONNumberInAnArrayComparesAsNull(t *testing.T) {
	for _, pair := range [][2]string{
		{`[NUM,1]`, `[1]`},
		{`[1,NUM]`, `[1]`},
		{`[NUM,1]`, `[null,1]`},
		{`[NUM]`, `[null]`},
		{`[NUM]`, `[NUM]`},
		{`[NUM,1]`, `[NUM,1]`},
		{`[[NUM]]`, `[[null]]`},
		{`[[NUM]]`, `[[NUM]]`},
		{`[[NUM,1]]`, `[[null,1]]`},
		{`[[NUM,1]]`, `[[1]]`},
		{`[[1,NUM]]`, `[[1,null]]`},
		{`[[[NUM]]]`, `[[[null]]]`},
		{`[[NUM],"a"]`, `[[null],"\u0061"]`},
		{`[["a",NUM],"a"]`, `[["a",null],"\u0061"]`},
		{`[[NUM],"a"]`, `[[null],"A"]`},
		{`[1,[NUM]]`, `[1,[null]]`},
		{`[[{"b":NUM}]]`, `[[{"b":null}]]`},
		{`[[NUM],[1]]`, `[[null],[1.0]]`},
	} {
		for _, number := range nonJSONNumbers {
			for _, shape := range []string{`{"resourceType":"Patient","x":[{"a":LEFT},{"a":RIGHT}]}`, `{"resourceType":"Patient","x":[{"a":RIGHT},{"a":LEFT}]}`} {
				// Each NUM is the token, in the pair's left and right alike
				doc := strings.NewReplacer("LEFT", numberAtToken(pair[0], number), "RIGHT", numberAtToken(pair[1], number)).Replace(shape)
				withNull := strings.NewReplacer("LEFT", numberAtToken(pair[0], "null"), "RIGHT", numberAtToken(pair[1], "null")).Replace(shape)
				for _, question := range []string{
					"Patient.x[0] = Patient.x[1]", "Patient.x[0] ~ Patient.x[1]",
					"Patient.x[0].a = Patient.x[1].a", "Patient.x[0] != Patient.x[1]",
					"Patient.x.distinct().count()", "(Patient.x[0] | Patient.x[1]).count()",
					"Patient.x.isDistinct()", "Patient.x[0].a.exists()", "Patient.x.a.count()",
				} {
					for _, m := range modelOptions() {
						got, want := evaluateRendered(doc, question, m.opts...), evaluateRendered(withNull, question, m.opts...)
						if got != want {
							t.Errorf("%s, %s over %s: %s, want %s as with null", m.name, question, doc, got, want)
						}
					}
				}
			}
		}
	}
}

// Arrays of valid numbers nested in objects compare as they did before #238:
// the values here are what main answered, kept as a regression guard and not
// derived from the null relation. = reads a number by value, so [[1.0]] equals
// [[1]], and ~ takes two arrays that are not written alike for different.
func TestNestedArraysOfJSONNumbersCompareAsBefore(t *testing.T) {
	for _, tc := range []struct {
		left, right                  string
		equal, equivalent, distinctN string
	}{
		{"[[1.0]]", "[[1]]", "[Boolean true]", "[Boolean false]", "[Integer 1]"},
		{"[[1]]", "[[2]]", "[Boolean false]", "[Boolean false]", "[Integer 2]"},
		{"[[1,2]]", "[[1,2]]", "[Boolean true]", "[Boolean true]", "[Integer 1]"},
		{"[[1.0,2]]", "[[1,2.0]]", "[Boolean true]", "[Boolean false]", "[Integer 1]"},
		{`[["a"]]`, `[["A"]]`, "[Boolean false]", "[Boolean false]", "[Integer 2]"},
		{"[[1,[2.0]]]", "[[1,[2]]]", "[Boolean true]", "[Boolean false]", "[Integer 1]"},
		{`[[{"b":1.0}]]`, `[[{"b":1}]]`, "[Boolean true]", "[Boolean false]", "[Integer 1]"},
		{"[[null]]", "[[1]]", "[Boolean false]", "[Boolean false]", "[Integer 2]"},
		{"[[1]]", "[[1],[2]]", "[Boolean false]", "[Boolean false]", "[Integer 2]"},
	} {
		doc := `{"resourceType":"Patient","x":[{"a":` + tc.left + `},{"a":` + tc.right + `}]}`
		for _, m := range modelOptions() {
			for expr, want := range map[string]string{
				"Patient.x[0] = Patient.x[1]":           tc.equal,
				"Patient.x[0] ~ Patient.x[1]":           tc.equivalent,
				"Patient.x.distinct().count()":          tc.distinctN,
				"(Patient.x[0] | Patient.x[1]).count()": tc.distinctN,
			} {
				if got := evaluateRendered(doc, expr, m.opts...); got != want {
					t.Errorf("%s, %s over %s = %s, want %s", m.name, expr, doc, got, want)
				}
			}
		}
	}
}

// A number outside the grammar in the array of a primitive that has elements
// beside it, _a, or in the array of those elements, is no value there, as null
// is: the element beside it stays and is compared at its position (#238).
func TestANonJSONNumberBesideAnElementComparesAsNull(t *testing.T) {
	for _, pair := range [][2]string{
		{`{"a":[NUM,1],"_a":[{"id":"e"},null]}`, `{"a":[null,1],"_a":[{"id":"e"},null]}`},
		{`{"a":[NUM,1],"_a":[{"id":"e"},null]}`, `{"a":[1]}`},
		{`{"a":[NUM,1],"_a":[{"id":"e"},{"id":"f"}]}`, `{"a":[null,1],"_a":[{"id":"e"},{"id":"f"}]}`},
		{`{"a":[NUM,1],"_a":[{"id":"e"},{"id":"f"}]}`, `{"a":[1],"_a":[{"id":"e"},{"id":"f"}]}`},
		{`{"a":[1,NUM],"_a":[null,{"id":"e"}]}`, `{"a":[1,null],"_a":[null,{"id":"e"}]}`},
		{`{"a":[1],"_a":[NUM,{"id":"e"}]}`, `{"a":[1],"_a":[null,{"id":"e"}]}`},
		{`{"a":[1],"_a":[null,NUM]}`, `{"a":[1]}`},
		{`{"a":[1],"_a":[NUM]}`, `{"a":[1]}`},
		{`{"a":[1,null],"_a":[NUM,NUM]}`, `{"a":[1]}`},
		{`{"a":[NUM],"_a":[NUM]}`, `{"a":[1],"_a":[null]}`},
		{`{"a":[NUM],"_a":[{"id":"e"}]}`, `{"a":[null],"_a":[{"id":"e"}]}`},
		{`{"a":[NUM],"_a":[{"id":"e"}]}`, `{"a":[1],"_a":[{"id":"e"}]}`},
		{`{"a":[null],"_a":[NUM]}`, `{"a":[1]}`},
		{`{"a":NUM,"_a":{"id":"e"}}`, `{"a":null,"_a":{"id":"e"}}`},
		{`{"a":NUM,"_a":{"id":"e"}}`, `{"a":1,"_a":{"id":"e"}}`},
		{`{"a":NUM,"_a":NUM}`, `{}`},
		{`{"_a":[NUM]}`, `{}`},
		{`{"_a":[NUM,NUM]}`, `{"_a":[null]}`},
		{`{"a":[1],"_a":[NUM,NUM]}`, `{"a":[1]}`},
		{`{"a":[NUM],"_a":[{}]}`, `{"a":[null],"_a":[{}]}`},
		{`{"a":[NUM],"_a":[{}]}`, `{"a":[1],"_a":[{}]}`},
		{`{"a":[NUM,1],"_a":[{},{}]}`, `{"a":[null,1],"_a":[{},{}]}`},
		{`{"a":[1,NUM],"_a":[{},{}]}`, `{"a":[1],"_a":[{},{}]}`},
		{`{"a":[1,NUM],"_a":[{},NUM]}`, `{"a":[1,null],"_a":[{},null]}`},
		{`{"a":[1,NUM],"_a":[NUM,{}]}`, `{"a":[1]}`},
		{`{"a":[1],"_a":[{},NUM]}`, `{"a":[1]}`},
		{`{"a":[1,NUM],"_a":[{},{},NUM]}`, `{"a":[1,null]}`},
		{`{"a":[1],"_a":[{},NUM]}`, `{"a":[1],"_a":[{}]}`},
		{`{"a":[NUM,2],"_a":[{"id":"e"},{"id":"f"}],"b":1}`, `{"b":1,"a":[null,2],"_a":[{"id":"e"},{"id":"f"}]}`},
	} {
		for _, number := range nonJSONNumbers {
			for _, shape := range []string{`{"resourceType":"Patient","x":[LEFT,RIGHT]}`, `{"resourceType":"Patient","x":[RIGHT,LEFT]}`} {
				doc := strings.NewReplacer("LEFT", numberAtToken(pair[0], number), "RIGHT", numberAtToken(pair[1], number)).Replace(shape)
				withNull := strings.NewReplacer("LEFT", numberAtToken(pair[0], "null"), "RIGHT", numberAtToken(pair[1], "null")).Replace(shape)
				for _, question := range []string{
					"Patient.x[0] = Patient.x[1]", "Patient.x[0] ~ Patient.x[1]",
					"Patient.x.distinct().count()", "(Patient.x[0] | Patient.x[1]).count()",
					"Patient.x.a.count()", "Patient.x.a.hasValue()", "Patient.x.a.extension.count()",
				} {
					for _, m := range modelOptions() {
						got, want := evaluateRendered(doc, question, m.opts...), evaluateRendered(withNull, question, m.opts...)
						if got != want {
							t.Errorf("%s, %s over %s: %s, want %s as with null", m.name, question, doc, got, want)
						}
					}
				}
			}
		}
	}
}
