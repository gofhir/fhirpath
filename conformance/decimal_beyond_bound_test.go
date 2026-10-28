package conformance

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// A decimal a resource holds beyond the decimals this engine computes with,
// types.DecimalDigits, is read, not lost (#232). FHIR's decimal, alike in R4,
// R4B and R5, is "Rational numbers that have a decimal representation"; the
// bound is this engine's, and FHIRPath leaves it free: "Implementations can
// provide support for larger decimals and higher precision". fhirpath.js 5.2.0
// and HL7's Java engine (validator 6.10.2) both read such a value: 1e1000 exists
// and 1e1000 > 0 is true (batteries in difftest/batteries/decimal-beyond-bound,
// CONFORMANCE.md rule 1). It is held as written and compared exactly; an
// operation that would compute with it overflows to empty, as #135 decided:
// "Operations that cause arithmetic overflow or underflow will result in
// empty".

// beyondPlacements puts a decimal where a resource holds one, %s standing for
// the JSON number, with the expression that reaches it, each by its own route.
var beyondPlacements = []struct{ resource, expr string }{
	{`{"resourceType":"Patient","extension":[{"url":"http://example.org/d","valueDecimal":%s}]}`, "Patient.extension.value"},
	// the same with its keys in another order and an extension no definition
	// names beside it
	{`{"extension":[{"valueDecimal":%s,"url":"http://example.org/d"},{"url":"http://example.org/u","valueString":"u"}],"resourceType":"Patient"}`, "Patient.extension.where(url = 'http://example.org/d').value"},
	{`{"resourceType":"Patient","extension":[{"url":"http://example.org/a","extension":[{"url":"b","valueDecimal":%s}]}]}`, "Patient.extension.extension.value"},
	{`{"resourceType":"RiskAssessment","status":"final","subject":{"reference":"Patient/p"},"prediction":[{"probabilityDecimal":%s}]}`, "RiskAssessment.prediction.probability"},
	// a primitive with its element beside it
	{`{"resourceType":"RiskAssessment","status":"final","subject":{"reference":"Patient/p"},"prediction":[{"probabilityDecimal":%s,"_probabilityDecimal":{"id":"p1"}}]}`, "RiskAssessment.prediction.probability"},
	{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":{"value":%s,"code":"g"}}`, "Observation.value.value"},
	{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"component":[{"code":{"text":"a"},"valueQuantity":{"value":1}},{"code":{"text":"b"},"valueQuantity":{"value":%s}}]}`, "Observation.component.value.value.where($this != 1)"},
	{`{"resourceType":"Patient","contained":[{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":{"value":%s}}]}`, "Patient.contained.value.value"},
	{`{"resourceType":"Bundle","type":"collection","entry":[{"resource":{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":{"value":%s}}}]}`, "Bundle.entry.resource.value.value"},
	{`{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":{"value":%s}}`, "Observation.descendants().ofType(FHIR.decimal)"},
}

// beyondValues are decimals past the bound on either side of the point, as
// written in JSON: large, negative, with a capital E, small, with a long
// coefficient, and two whose digits written out would be millions and a
// billion, which only reading touches.
var beyondValues = []string{
	"1e1000", "-1e1000", "1E1000", "1e+1000", "1e-1001", "-2.5e-1001",
	"123456789012345678901234567890e990", "1e5000000", "1e999999999",
}

// decimalItems names how a value was read: its Go type, the type it reports
// and its text.
func decimalItems(result fhirpath.Collection) string {
	items := make([]string, len(result))
	for i, v := range result {
		kind := fmt.Sprintf("%T", v)
		if _, ok := v.(types.Decimal); ok {
			kind = "Decimal"
		}
		items[i] = kind + " " + v.Type() + " " + v.String()
	}
	return fmt.Sprint(items)
}

func TestADecimalBeyondTheBoundIsRead(t *testing.T) {
	for _, place := range beyondPlacements {
		for _, value := range beyondValues {
			doc := fmt.Sprintf(place.resource, value)
			for _, v := range versionModels {
				result, err := fhirpath.MustCompile(place.expr).EvaluateWithOptions([]byte(doc), fhirpath.WithModel(v.model))
				if err != nil {
					t.Errorf("%s: %s over %s: %v", v.name, place.expr, doc, err)
					continue
				}
				// the value as written, typed decimal
				want := "[Decimal decimal " + value + "]"
				if got := decimalItems(result); got != want {
					t.Errorf("%s: %s over %s = %s, want %s", v.name, place.expr, doc, got, want)
				}
			}
		}
	}
}

// Without a model the value is read too, by each route as 1.5 is read there.
func TestADecimalBeyondTheBoundIsReadWithoutAModel(t *testing.T) {
	read := func(value, expr string) (string, error) {
		doc := fmt.Sprintf(`{"resourceType":"Patient","extension":[{"url":"http://example.org/d","valueDecimal":%s}]}`, value)
		result, err := fhirpath.Evaluate([]byte(doc), expr)
		return decimalItems(result), err
	}
	for _, expr := range []string{"Patient.extension.value", "Patient.extension.valueDecimal", "Patient.extension.children().ofType(Decimal)"} {
		control, err := read("1.5", expr)
		if err != nil || !strings.HasSuffix(control, " 1.5]") {
			t.Fatalf("%s reads 1.5 as %s (%v)", expr, control, err)
		}
		for _, value := range beyondValues {
			got, err := read(value, expr)
			if want := strings.TrimSuffix(control, "1.5]") + value + "]"; err != nil || got != want {
				t.Errorf("%s with %s = %s (%v), want %s", expr, value, got, err, want)
			}
		}
	}
}

// beyondCase is an expression over the extension holding value, and what it
// answers: the items as typedItems renders them, or "error".
type beyondCase struct{ value, expr, want string }

func checkBeyond(t *testing.T, cases []beyondCase) {
	t.Helper()
	for _, tt := range cases {
		doc := fmt.Sprintf(`{"resourceType":"Patient","extension":[{"url":"http://example.org/d","valueDecimal":%s}]}`, tt.value)
		for _, v := range versionModels {
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(doc), fhirpath.WithModel(v.model))
			got := "error"
			if err == nil {
				got = typedItems(result)
			}
			if got != tt.want {
				t.Errorf("%s: %s with V = %s: %s (%v), want %s", v.name, tt.expr, tt.value, got, err, tt.want)
			}
		}
	}
}

// held names the decimal the extension holds.
const held = "Patient.extension.value"

// writtenOut is digits followed by zeros, as a string toDecimal() converts.
func writtenOut(digits string, zeros int) string {
	return "'" + digits + strings.Repeat("0", zeros) + "'.toDecimal()"
}

// Comparison is exact, as arithmetic gives it: both references agree on every
// row below that a float does not lose (fhirpath.js and Java read 1e-1001 as
// 0, so answer 1e-1001 > 0 false; the value is exact, and so is this engine).
func TestADecimalBeyondTheBoundComparesExactly(t *testing.T) {
	yes, no := "[Boolean true]", "[Boolean false]"
	checkBeyond(t, []beyondCase{
		{"1e1000", held + " = " + held, yes},
		{"1e1000", held + " != " + held, no},
		{"1e1000", held + " = 1", no},
		{"1e1000", held + " != 1", yes},
		{"1e1000", held + " > 0", yes},
		{"1e1000", held + " < 1", no},
		{"1e1000", held + " >= -1", yes},
		{"1e1000", "1 < " + held, yes},
		{"-1e1000", held + " > 0", no},
		{"-1e1000", held + " < -1", yes},
		{"-1e1000", held + " < 1", yes},
		{"1e-1001", held + " > 0", yes},
		{"1e-1001", held + " < 0.0000001", yes},
		{"1e-1001", held + " = 0", no},
		{"-2.5e-1001", held + " < 0", yes},
		{"-2.5e-1001", held + " > -0.0000001", yes},
		// the same value written another way is the same value
		// the same value written out digit by digit, which toDecimal() reads
		// (it takes no exponent: "(\+|-)?\d+(\.\d+)?"), is the same value
		{"1e1000", held + " = " + writtenOut("1", 1000), yes},
		{"1e1000", held + " = " + writtenOut("2", 1000), no},
		{"1e1000", held + " < " + writtenOut("2", 1000), yes},
		{"1e1000", held + " > " + writtenOut("999", 997), yes},
		{"1e1000", held + " < " + writtenOut("1", 1001), yes},
		{"123456789012345678901234567890e990", held + " > " + writtenOut("123456789012345678901234567889", 990), yes},
		{"123456789012345678901234567890e990", held + " = " + writtenOut("12345678901234567890123456789", 991), yes},
		{"1e1000", held + " in (1 | " + held + ")", yes},
		{"1e1000", "(1 | " + held + ") contains " + held, yes},
		{"1e1000", "(" + held + " | " + held + ").count()", "[Integer 1]"},
		{"1e1000", "(" + held + " | 1).distinct().count()", "[Integer 2]"},
		{"1e1000", held + ".combine(" + held + ").isDistinct()", no},
		{"1e1000", "(" + held + " | 1).exclude(" + held + ").count()", "[Integer 1]"},
		{"1e1000", "(" + held + " | 1).intersect(" + held + ").count()", "[Integer 1]"},
		{"1e1000", held + ".subsetOf(" + held + " | 1)", yes},
		{"1e1000", "(" + held + " | 3 | 2).sort().first() = 2", yes},
		{"1e1000", "(" + held + " | 3 | 2).sort().last() = " + held, yes},
		{"-1e1000", "(" + held + " | 3 | 2).sort().first() = " + held, yes},
		{"1e-1001", "(" + held + " | 3 | 2).sort().first() = " + held, yes},
		// min() and max() select an item, which they hand back as it is
		{"1e1000", "(" + held + " | 2).max() = " + held, yes},
		{"1e1000", "(" + held + " | 2).min()", "[Integer 2]"},
		{"-1e1000", "(" + held + " | 2).min() = " + held, yes},
		// equivalence rounds to the less precise operand, "comparison is done
		// on values rounded to the precision of the least precise operand":
		// 0 is given to no place, and 1e-1001 rounds to 0 there
		{"1e1000", held + " ~ " + held, yes},
		{"1e1000", held + " ~ 1", no},
		{"1e1000", held + " !~ 1", yes},
		{"1e-1001", held + " ~ 0", yes},
		{"1e-1001", held + " ~ 1", no},
		// against a quantity, as a quantity of unit '1'
		{"1e1000", held + " = 1 '1'", no},
		{"1e1000", held + " ~ 1 '1'", no},
	})
}

// A decimal beyond the bound compared as two FHIR objects holding it: the
// numbers are compared as decimals, so the same value written two ways is
// equal, and without rescaling one to the other: 1e5000000 against 1 would be
// five million digits.
func TestObjectsHoldingADecimalBeyondTheBoundCompareExactly(t *testing.T) {
	doc := `{"resourceType":"Patient","extension":[
		{"url":"http://example.org/d","valueDecimal":1e1000},
		{"url":"http://example.org/d","valueDecimal":1.0e1000},
		{"url":"http://example.org/d","valueDecimal":2e1000},
		{"url":"http://example.org/d","valueDecimal":1e5000000},
		{"url":"http://example.org/d","valueDecimal":1},
		{"valueDecimal":1,"url":"http://example.org/d"},
		{"valueDecimal":10e999,"url":"http://example.org/d"}]}`
	for _, tt := range []struct{ expr, want string }{
		{"Patient.extension[0] = Patient.extension[1]", "[Boolean true]"},
		{"Patient.extension[0] ~ Patient.extension[1]", "[Boolean true]"},
		{"Patient.extension[0] = Patient.extension[2]", "[Boolean false]"},
		{"Patient.extension[0] ~ Patient.extension[2]", "[Boolean false]"},
		{"Patient.extension[3] = Patient.extension[4]", "[Boolean false]"},
		{"Patient.extension[3] ~ Patient.extension[4]", "[Boolean false]"},
		{"Patient.extension[4] ~ Patient.extension[3]", "[Boolean false]"},
		// the keys in another order, which compares child by child
		{"Patient.extension[3] = Patient.extension[5]", "[Boolean false]"},
		{"Patient.extension[3] ~ Patient.extension[5]", "[Boolean false]"},
		{"Patient.extension[0] = Patient.extension[6]", "[Boolean true]"},
		{"Patient.extension[0] ~ Patient.extension[6]", "[Boolean true]"},
	} {
		for _, v := range versionModels {
			start := time.Now()
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(doc), fhirpath.WithModel(v.model))
			if took := time.Since(start); took > 50*time.Millisecond {
				t.Errorf("%s: %s took %v: it rescaled", v.name, tt.expr, took)
			}
			if err != nil {
				t.Errorf("%s: %s: %v", v.name, tt.expr, err)
				continue
			}
			if got := typedItems(result); got != tt.want {
				t.Errorf("%s: %s = %s, want %s", v.name, tt.expr, got, tt.want)
			}
		}
	}
}

// Two Money values compare by amount and currency, an amount beyond the bound
// too, however each is written and whatever else the object carries: the
// same amounts written apart, with an id on one of them, are equal.
func TestMoneyBeyondTheBoundComparesByAmount(t *testing.T) {
	for _, tt := range []struct {
		net, gross, want string
	}{
		{"1", "1.0", "[Boolean true]"},
		{"1e1000", "10e999", "[Boolean true]"},
		{"1e1000", "2e1000", "[Boolean false]"},
		{"-1e-1001", "-0.1e-1000", "[Boolean true]"},
		{"1e5000000", "1", "[Boolean false]"},
	} {
		doc := `{"resourceType":"Invoice","status":"draft",` +
			`"totalNet":{"value":` + tt.net + `,"currency":"USD"},` +
			`"totalGross":{"id":"g","value":` + tt.gross + `,"currency":"USD"}}`
		for _, v := range versionModels {
			result, err := fhirpath.MustCompile("Invoice.totalNet = Invoice.totalGross").EvaluateWithOptions([]byte(doc), fhirpath.WithModel(v.model))
			if err != nil {
				t.Errorf("%s: %s = %s: %v", v.name, tt.net, tt.gross, err)
				continue
			}
			if got := typedItems(result); got != tt.want {
				t.Errorf("%s: %s USD = %s USD is %s, want %s", v.name, tt.net, tt.gross, got, tt.want)
			}
		}
	}
}

// What does not compute with the value answers as for any decimal; what
// computes with it overflows to empty (plan 010, the classification).
func TestADecimalBeyondTheBoundIsHeldNotComputed(t *testing.T) {
	yes, no, empty := "[Boolean true]", "[Boolean false]", "[]"
	cases := []beyondCase{
		// what holds it, or asks about it
		{"1e1000", held + ".exists()", yes},
		{"1e1000", held + ".hasValue()", yes},
		{"1e1000", held + ".count()", "[Integer 1]"},
		{"1e1000", held + ".type().name", "[String decimal]"},
		{"1e1000", held + " is FHIR.decimal", yes},
		{"1e1000", "iif(" + held + " > 0, 'pos', 'neg')", "[String pos]"},
		{"1e1000", held + ".select($this > 0)", yes},
		{"1e1000", held + ".all($this > 0)", yes},
		{"1e1000", held + ".trace('v') > 0", yes},
		{"1e1000", held + ".defineVariable('x').select(%x > 0)", yes},
		{"1e1000", "coalesce(" + held + ", 1) > 1", yes},
		{"1e1000", held + ".comparable(1 'g')", no},
		// its text as written: Java answers 1e1000, fhirpath.js Infinity
		{"1e1000", held + ".toString()", "[String 1e1000]"},
		{"-1e1000", held + ".toString()", "[String -1e1000]"},
		{"1e-1001", held + ".toString()", "[String 1e-1001]"},
		{"1e1000", held + ".getValue().toString()", "[String 1e1000]"},
		{"1e1000", held + ".toDecimal() = " + held, yes},
		{"1e1000", held + ".convertsToDecimal()", yes},
		// its text has an exponent, which a String converting to a Decimal
		// does not take, "(\+|-)?\d+(\.\d+)?" (fhirpath.js answers so)
		{"1e1000", held + ".toString().toDecimal()", empty},
		{"1e1000", held + ".toString().convertsToDecimal()", no},
		{"1e1000", "'1" + strings.Repeat("0", 1000) + "'.convertsToDecimal()", yes},
		{"1e1000", "'-0." + strings.Repeat("0", 1000) + "1'.convertsToDecimal()", yes},
		{"1e1000", held + ".convertsToString()", yes},
		// precision() reads the text, as today
		{"1e1000", held + ".precision()", "[Integer 0]"},
		// against a quantity of unit '1' it orders as against the number:
		// the same unit converts nothing, so nothing is written out
		{"1e1000", held + " > 1 '1'", yes},
		{"1e1000", held + " <= 1 '1'", no},
		{"1e1000", "1 '1' < " + held, yes},
		{"-1e1000", held + " < 1 '1'", yes},
		{"1e-1001", held + " < 1 '1'", yes},
		{"1e-1001", held + " > 0 '1'", yes},
		{"1e5000000", held + " >= 1 '1'", yes},
		// power(0) is 1 whatever the base, as both references answer
		{"1e1000", held + ".power(0)", "[Decimal 1]"},
		// any other exponent computes with it
		{"1e1000", held + ".power(1)", empty},
		{"1e1000", held + ".power(-1)", empty},
		{"1e1000", held + ".power(-2.5)", empty},
		{"1e-1001", held + ".power(-1)", empty},
		// computing with it overflows
		{"1e1000", held + ".convertsToInteger()", no},
		{"1e1000", held + ".convertsToLong()", no},
		{"1e1000", held + ".convertsToBoolean()", no},
		{"1e1000", held + ".convertsToQuantity()", no},
		{"1e-1001", held + ".convertsToInteger()", no},
	}
	values := []string{"1e1000", "-1e1000", "1e-1001", "1e5000000"}
	overflows := []string{
		held + " + 1", held + " - 1", "1 - " + held, held + " * 2", held + " * 0", held + " / 2",
		held + " / " + held, held + " div 2", held + " mod 2", held + " - " + held, "-" + held,
		held + ".abs()", held + ".ceiling()", held + ".floor()", held + ".truncate()",
		held + ".round()", held + ".round(2)", held + ".sqrt()", held + ".exp()",
		held + ".ln()", held + ".log(10)", "(10.0).log(" + held + ")", held + ".power(2)",
		"(2.0).power(" + held + ")", held + ".sum()", held + ".avg()",
		held + ".aggregate($total + $this, 0)", held + ".lowBoundary()",
		held + ".highBoundary()", held + ".toQuantity()", held + ".toQuantity('g')",
		held + ".toInteger()", held + ".toLong()", held + ".toBoolean()",
	}
	computed := make([]beyondCase, 0, len(values)*len(overflows))
	for _, value := range values {
		for _, expr := range overflows {
			computed = append(computed, beyondCase{value, expr, empty})
		}
	}
	checkBeyond(t, cases)
	checkBeyond(t, computed)
}

// A zero written past the bound, 0e2000, is a zero: it is read at a scale
// within the bound, keeping its text, and computes as any zero, as both
// references answer 0e2000 + 1 = 1.
func TestAZeroWrittenBeyondTheBoundIsAZero(t *testing.T) {
	checkBeyond(t, []beyondCase{
		{"0e2000", held + ".toString()", "[String 0e2000]"},
		{"0e2000", held + " = 0", "[Boolean true]"},
		{"0e2000", held + " + 1", "[Decimal 1]"},
		{"0e2000", held + " * 2 = 0", "[Boolean true]"},
		{"0e-2000", held + " = 0", "[Boolean true]"},
		{"0e-2000", held + " + 1 = 1", "[Boolean true]"},
	})
}

// A zero written beyond the bound in a FHIR Quantity is a zero, as a decimal
// written so is: 0e2000 'mg' computes and compares as 0 'mg' does, with a
// model and without one, where reading the quantity refused it.
func TestAZeroWrittenBeyondTheBoundInAQuantityIsAZero(t *testing.T) {
	const ucum = `,"system":"http://unitsofmeasure.org","code":`
	for _, tt := range []struct{ value, code, expr, want string }{
		{"0e2000", "mg", "Observation.value.abs() = 0 'mg'", "[Boolean true]"},
		{"0e2000", "mg", "Observation.value.round(1) = 0 'mg'", "[Boolean true]"},
		{"0e2000", "mg", "Observation.value = 0 'mg'", "[Boolean true]"},
		{"0e2000", "mg", "Observation.value ~ 0 'mg'", "[Boolean true]"},
		{"0e2000", "mg", "Observation.value + 1 'mg' = 1 'mg'", "[Boolean true]"},
		{"-0e2000", "mg", "Observation.value < 1 'mg'", "[Boolean true]"},
		{"0e-2000", "mg", "Observation.value = 0 'mg'", "[Boolean true]"},
		{"0.0E+1500", "1", "Observation.value = 0", "[Boolean true]"},
	} {
		doc := `{"resourceType":"Observation","status":"final","code":{"text":"c"},` +
			`"valueQuantity":{"value":` + tt.value + ucum + `"` + tt.code + `"}}`
		check := func(name string, opts ...fhirpath.EvalOption) {
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(doc), opts...)
			if err != nil {
				t.Errorf("%s: %s '%s': %s: %v", name, tt.value, tt.code, tt.expr, err)
			} else if got := typedItems(result); got != tt.want {
				t.Errorf("%s: %s '%s': %s = %s, want %s", name, tt.value, tt.code, tt.expr, got, tt.want)
			}
		}
		check("no model")
		for _, v := range versionModels {
			check(v.name, fhirpath.WithModel(v.model))
		}
	}
}

// A Quantity whose value is beyond the bound stays as #135 left it: the
// object is there, its value reads, but as a quantity it holds no amount, so
// computing or comparing with it overflows to empty, without an error
// ("Operations that cause arithmetic overflow or underflow will result in
// empty"). The unit '1' makes it comparable to a decimal, which then
// compares with nothing too; another unit makes it no quantity a number is,
// which its unit alone decides: 2 = 2 'mg' is false, and so is 2 = 1e1000
// 'mg'. Something that is no number at all is unequal to it.
func TestAQuantityWithAValueBeyondTheBoundComparesAsNothing(t *testing.T) {
	for _, value := range []string{"1e1000", "1e5000000", "-1e1000", "1e-1001"} {
		for _, code := range []string{"mg", "1"} {
			unitOne := func(withOne, otherwise string) string {
				if code == "1" {
					return withOne
				}
				return otherwise
			}
			doc := `{"resourceType":"Observation","status":"final","code":{"text":"c"},` +
				`"extension":[{"url":"x","valueDecimal":` + value + `}],` +
				`"valueQuantity":{"value":` + value + `,"code":"` + code + `","system":"http://unitsofmeasure.org"}}`
			for _, tt := range []struct{ expr, want string }{
				{"Observation.value.exists()", "[Boolean true]"},
				{"Observation.value.value = Observation.extension.value", "[Boolean true]"},
				{"Observation.value > 1 'mg'", "[]"},
				{"Observation.value = 1 'mg'", "[]"},
				{"1 'mg' = Observation.value", "[]"},
				{"Observation.value ~ Observation.value", "[Boolean false]"},
				{"Observation.value > 1.5", "[]"},
				{"Observation.value >= 1", "[]"},
				{"1.5 < Observation.value", "[]"},
				{"Observation.value = Observation.extension.value", unitOne("[]", "[Boolean false]")},
				{"Observation.extension.value = Observation.value", unitOne("[]", "[Boolean false]")},
				{"Observation.value = 2", unitOne("[]", "[Boolean false]")},
				{"2 = Observation.value", unitOne("[]", "[Boolean false]")},
				{"Observation.value != 2", unitOne("[]", "[Boolean true]")},
				{"Observation.value = 'a'", "[Boolean false]"},
				{"Observation.value = true", "[Boolean false]"},
				{"Observation.value = @2020", "[Boolean false]"},
				// a number beyond the bound against a quantity within it
				// is unequal, exactly
				{"Observation.extension.value = 2 '1'", "[Boolean false]"},
				{"2 '1' = Observation.extension.value", "[Boolean false]"},
				{"Observation.extension.value != 2 '1'", "[Boolean true]"},
				{"Observation.value + 1.5", "[]"},
				{"Observation.value - 1.5", "[]"},
				{"1.5 + Observation.value", "[]"},
				{"Observation.value * 2", "[]"},
				{"Observation.value / 2", "[]"},
				{"Observation.value div 1.5", "[]"},
				{"Observation.value mod 1.5", "[]"},
				{"-Observation.value", "[]"},
				{"Observation.value.abs()", "[]"},
				{"Observation.value.round()", "[]"},
				{"Observation.value.ceiling()", "[]"},
				{"Observation.value.floor()", "[]"},
				{"Observation.value.truncate()", "[]"},
				{"Observation.value.sqrt()", "[]"},
				{"Observation.value.exp()", "[]"},
				{"Observation.value.ln()", "[]"},
				{"Observation.value.power(0)", "[]"},
				{"Observation.value.sum()", "[]"},
				{"Observation.value.aggregate($total + $this, 0)", "[]"},
				{"Observation.value.lowBoundary()", "[]"},
				{"Observation.value.toQuantity()", "[]"},
			} {
				for _, v := range versionModels {
					result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(doc), fhirpath.WithModel(v.model))
					if err != nil {
						t.Errorf("%s: %s '%s': %s: %v", v.name, value, code, tt.expr, err)
						continue
					}
					if got := typedItems(result); got != tt.want {
						t.Errorf("%s: %s '%s': %s = %s, want %s", v.name, value, code, tt.expr, got, tt.want)
					}
				}
			}
		}
	}
}

// Without a model, an object holding a numeric value alone converts as a
// quantity of no unit ({"value":5}.abs() is 5); one beyond the bound holds no
// amount, and computing with it overflows to empty, as with a unit.
func TestAValueAloneBeyondTheBoundWithoutAModelComputesNothing(t *testing.T) {
	for _, value := range []string{"1e1000", "1e5000000", "1e-1001"} {
		doc := `{"resourceType":"Observation","valueQuantity":{"value":` + value + `}}`
		for _, expr := range []string{
			"Observation.valueQuantity.abs()", "Observation.valueQuantity.round()",
			"Observation.valueQuantity.round(1)", "Observation.valueQuantity.ceiling()",
			"Observation.valueQuantity.floor()", "Observation.valueQuantity.truncate()",
			"Observation.valueQuantity + 1 'mg'", "Observation.valueQuantity > 1 'mg'",
			"Observation.valueQuantity.sum()",
		} {
			result, err := fhirpath.MustCompile(expr).Evaluate([]byte(doc))
			if err != nil {
				t.Errorf("%s: %s: %v", value, expr, err)
				continue
			}
			if got := typedItems(result); got != "[]" {
				t.Errorf("%s: %s = %s, want []", value, expr, got)
			}
		}
	}
}

// Without a model a Money is no quantity, beyond the bound or within it:
// computing with it is the type error it is with an amount of 5, not the
// empty of a quantity that overflows; compared as an object it is unequal to
// a number.
func TestAMoneyBeyondTheBoundWithoutAModelIsNoQuantity(t *testing.T) {
	for _, value := range []string{"5", "1e1000"} {
		doc := `{"resourceType":"Claim","total":{"value":` + value + `,"currency":"USD"}}`
		for _, expr := range []string{"Claim.total.abs()", "Claim.total * 2", "-Claim.total", "Claim.total > 1"} {
			if _, err := fhirpath.MustCompile(expr).Evaluate([]byte(doc)); err == nil {
				t.Errorf("%s USD: %s gave no error, want the type error of a Money", value, expr)
			}
		}
		result, err := fhirpath.MustCompile("Claim.total = 5").Evaluate([]byte(doc))
		if err != nil {
			t.Errorf("%s USD: Claim.total = 5: %v", value, err)
		} else if got := typedItems(result); got != "[Boolean false]" {
			t.Errorf("%s USD: Claim.total = 5 is %s, want [Boolean false]", value, got)
		}
	}
}

// A quantity written with no unit takes the other's, as Comparable and the
// conversion have it: a decimal beyond the bound orders against it as
// against a number, exactly.
func TestADecimalBeyondTheBoundOrdersAgainstAQuantityWithNoUnit(t *testing.T) {
	for _, tt := range []struct{ value, expr, want string }{
		{"1e1000", "Observation.extension.value > Observation.value", "[Boolean true]"},
		{"1e1000", "Observation.value < Observation.extension.value", "[Boolean true]"},
		{"-1e1000", "Observation.extension.value > Observation.value", "[Boolean false]"},
		{"1e-1001", "Observation.extension.value < Observation.value", "[Boolean true]"},
		{"1e1000", "Observation.extension.value = Observation.value", "[Boolean false]"},
	} {
		doc := `{"resourceType":"Observation","status":"final","code":{"text":"c"},` +
			`"extension":[{"url":"x","valueDecimal":` + tt.value + `}],"valueQuantity":{"value":2}}`
		for _, v := range versionModels {
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(doc), fhirpath.WithModel(v.model))
			if err != nil {
				t.Errorf("%s: %s: %s: %v", v.name, tt.value, tt.expr, err)
				continue
			}
			if got := typedItems(result); got != tt.want {
				t.Errorf("%s: %s: %s = %s, want %s", v.name, tt.value, tt.expr, got, tt.want)
			}
		}
	}
}

// A number against a FHIR Quantity: within the bound it compares as a
// quantity of unit '1' or of no unit would, beyond it of a dimensionless unit
// it is unknown, as an overflow is empty, and of a dimension it is not,
// unequal. 2 = {"value":2} is true; 2 = 1e1000 '%' has no answer.
func TestANumberAgainstAFHIRQuantityByItsUnit(t *testing.T) {
	for _, tt := range []struct{ quantity, want string }{
		{`{"value":2}`, "[Boolean true]"},
		{`{"value":2,"code":"1","system":"http://unitsofmeasure.org"}`, "[Boolean true]"},
		{`{"value":3,"code":"1","system":"http://unitsofmeasure.org"}`, "[Boolean false]"},
		{`{"value":2,"code":"mg","system":"http://unitsofmeasure.org"}`, "[Boolean false]"},
		{`{"value":1e1000}`, "[]"},
		{`{"value":1e1000,"code":"1","system":"http://unitsofmeasure.org"}`, "[]"},
		{`{"value":1e1000,"code":"%","system":"http://unitsofmeasure.org"}`, "[]"},
		{`{"value":1e1000,"code":"{count}","system":"http://unitsofmeasure.org"}`, "[]"},
		{`{"value":1e1000,"code":"mg","system":"http://unitsofmeasure.org"}`, "[Boolean false]"},
	} {
		doc := `{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":` + tt.quantity + `}`
		for _, v := range versionModels {
			result, err := fhirpath.MustCompile("Observation.value = 2").EvaluateWithOptions([]byte(doc), fhirpath.WithModel(v.model))
			if err != nil {
				t.Errorf("%s: %s = 2: %v", v.name, tt.quantity, err)
				continue
			}
			if got := typedItems(result); got != tt.want {
				t.Errorf("%s: %s = 2 is %s, want %s", v.name, tt.quantity, got, tt.want)
			}
		}
	}
}

// A decimal written past the bound with zeros alone, 1 with 1001 zeros after
// the point, is 1, and is read at the nearest scale within the bound, as a
// zero written so is: it computes and compares as 1 does, as a decimal and as
// a FHIR Quantity's value, with a model and without one. Written with 5000
// zeros it is read so too, and 100.0e-1001, whose zeros stand on both sides of
// the point, is 1e-999.
func TestADecimalPastTheBoundByZerosAloneIsReadWithin(t *testing.T) {
	yes, no := "[Boolean true]", "[Boolean false]"
	checkBeyond(t, []beyondCase{{"100.0e-1001", held + " + 1 > 1", yes}})
	for _, value := range []string{"1." + strings.Repeat("0", 1001), "1." + strings.Repeat("0", 5000)} {
		checkBeyond(t, []beyondCase{
			{value, held + " + 1 = 2", yes},
			{value, held + " = 1", yes},
			{value, held + " = 1 '1'", yes},
			{value, "1 '1' = " + held, yes},
			{value, held + " ~ 1 '1'", yes},
			{value, held + " ~ 100 '%'", yes},
			{value, held + " in (1 '1' | 2 '1')", yes},
			{value, held + " = 1 'mg'", no},
			{value, held + ".round(2) = 1", yes},
		})
	}
	one := "1." + strings.Repeat("0", 1001)
	doc := `{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":{"value":` + one +
		`,"system":"http://unitsofmeasure.org","code":"mg"}}`
	for _, tt := range []struct{ expr, want string }{
		{"Observation.value = 1 'mg'", yes},
		{"Observation.value ~ 1 'mg'", yes},
		{"Observation.value < 2 'mg'", yes},
		{"Observation.value.abs() = 1 'mg'", yes},
		{"Observation.value + 1 'mg' = 2 'mg'", yes},
	} {
		check := func(name string, opts ...fhirpath.EvalOption) {
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(doc), opts...)
			if err != nil {
				t.Errorf("%s: %s: %v", name, tt.expr, err)
			} else if got := typedItems(result); got != tt.want {
				t.Errorf("%s: %s = %s, want %s", name, tt.expr, got, tt.want)
			}
		}
		check("no model")
		for _, v := range versionModels {
			check(v.name, fhirpath.WithModel(v.model))
		}
	}
}

// A decimal past the bound by a digit that is no zero, 1 written with 1000
// zeros and a 5 after the point, is no number within it, yet against a
// quantity in the same unit, or with no unit, = and ~ answer as against the
// number: = false, and ~ true, rounded to the places 1 has.
func TestADecimalPastTheBoundByADigitComparesAsAgainstTheNumber(t *testing.T) {
	yes, no := "[Boolean true]", "[Boolean false]"
	v := "1." + strings.Repeat("0", 1000) + "5"
	checkBeyond(t, []beyondCase{
		{v, held + " + 1", "[]"},
		{v, held + " = 1", no},
		{v, held + " ~ 1", yes},
		{v, held + " = 1 '1'", no},
		{v, "1 '1' = " + held, no},
		{v, held + " != 1 '1'", yes},
		{v, held + " ~ 1 '1'", yes},
		{v, "1 '1' ~ " + held, yes},
		{v, held + " !~ 1 '1'", no},
		{v, held + " > 1 '1'", yes},
		{v, held + " in (1 '1' | 2 '1')", no},
		{v, held + " = 1 '%'", "[]"},
		{v, held + " = 1 'mg'", no},
		{"1e1000", held + " = 1 '1'", no},
		{"1e1000", held + " ~ 1 '1'", no},
	})
	doc := `{"resourceType":"Observation","status":"final","code":{"text":"c"},` +
		`"extension":[{"url":"x","valueDecimal":` + v + `}],"valueQuantity":{"value":1}}`
	for _, tt := range []struct{ expr, want string }{
		{"Observation.extension.value = Observation.value", no},
		{"Observation.extension.value ~ Observation.value", yes},
	} {
		for _, m := range versionModels {
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(doc), fhirpath.WithModel(m.model))
			if err != nil {
				t.Errorf("%s: %s: %v", m.name, tt.expr, err)
			} else if got := typedItems(result); got != tt.want {
				t.Errorf("%s: %s = %s, want %s", m.name, tt.expr, got, tt.want)
			}
		}
	}
}

// A decimal beyond the bound against a quantity within it in another unit,
// '10*3' or '%', which only converting one side to the other's unit decides:
// that writes out its digits, an operation over it that overflows, so = and !=
// are empty, as < is, where a definite false would deny 1e1000 = 1e997 '10*3'
// (#232). ~ stays false, as for a FHIR Quantity beyond the bound. Within the
// bound the same pairs are equal.
func TestADecimalBeyondTheBoundAgainstAQuantityInAnotherUnitIsUnknown(t *testing.T) {
	const ucum = `,"system":"http://unitsofmeasure.org","code":`
	for _, tt := range []struct{ value, quantity, expr, want string }{
		{"1e1000", `{"value":1e997` + ucum + `"10*3"}`, "Observation.extension.value = Observation.value", "[]"},
		{"1e1000", `{"value":1e997` + ucum + `"10*3"}`, "Observation.value = Observation.extension.value", "[]"},
		{"1e1000", `{"value":1e997` + ucum + `"10*3"}`, "Observation.extension.value != Observation.value", "[]"},
		{"2e1000", `{"value":1e997` + ucum + `"10*3"}`, "Observation.extension.value = Observation.value", "[]"},
		{"1e-1001", `{"value":1e-999` + ucum + `"%"}`, "Observation.extension.value = Observation.value", "[]"},
		{"1e1000", `{"value":1e997` + ucum + `"10*3"}`, "Observation.extension.value ~ Observation.value", "[Boolean false]"},
		{"1e1000", `{"value":1e997` + ucum + `"10*3"}`, "Observation.extension.value >= Observation.value", "[]"},
		{"1000", `{"value":1` + ucum + `"10*3"}`, "Observation.extension.value = Observation.value", "[Boolean true]"},
		{"0.01", `{"value":1` + ucum + `"%"}`, "Observation.extension.value != Observation.value", "[Boolean false]"},
	} {
		doc := `{"resourceType":"Observation","status":"final","code":{"text":"c"},` +
			`"extension":[{"url":"x","valueDecimal":` + tt.value + `}],"valueQuantity":` + tt.quantity + `}`
		for _, v := range versionModels {
			result, err := fhirpath.MustCompile(tt.expr).EvaluateWithOptions([]byte(doc), fhirpath.WithModel(v.model))
			if err != nil {
				t.Errorf("%s: %s, %s: %s: %v", v.name, tt.value, tt.quantity, tt.expr, err)
				continue
			}
			if got := typedItems(result); got != tt.want {
				t.Errorf("%s: %s, %s: %s = %s, want %s", v.name, tt.value, tt.quantity, tt.expr, got, tt.want)
			}
		}
	}
}

// A Unicode minus is no sign in FHIR's decimal nor in JSON, though the JSON
// reader hands a number written with one in its exponent over as a number:
// it is read as nothing, as shopspring/decimal v1.4.0 had it, where v1.6.0
// would read 1e−2 as 0.01 (#235).
func TestAUnicodeMinusInAResourceIsNoNumber(t *testing.T) {
	checkBeyond(t, []beyondCase{
		{"1e−2", held + ".exists()", "[Boolean false]"},
		{"1.5e−2", held + ".exists()", "[Boolean false]"},
	})
}

// A known loss: an exponent past int32, which the decimal library holds the
// exponent in, is not read. R4's and R4B's regex for decimal allow any exponent
// ("([eE][+-]?[0-9]+)?"); R5's allows nine digits ("[0-9]{1,9}"), all of
// which are held. fhirpath.js and Java read 1e9999999999 (as a float, and
// exists() is true); this engine answers as for no value, which
// docs/upstream-findings.md records (FP-6).
func TestAnExponentPastInt32IsAKnownLoss(t *testing.T) {
	checkBeyond(t, []beyondCase{
		{"1e9999999999", held + ".exists()", "[Boolean false]"},
		{"1e-9999999999", held + ".exists()", "[Boolean false]"},
		// the widest nine-digit exponent R5 allows is held
		{"1e999999999", held + ".exists()", "[Boolean true]"},
		{"1e-999999999", held + ".exists()", "[Boolean true]"},
	})
}
