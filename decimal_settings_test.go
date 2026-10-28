package fhirpath_test

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/gofhir/fhirpath"
)

// decimalSettingEnv names, in a re-executed test binary, the decimal
// library's setting to change before evaluating.
const decimalSettingEnv = "FHIRPATH_TEST_DECIMAL_SETTING"

// decimalSettings changes each process-wide setting of shopspring/decimal
// v1.6.0 that a program embedding this engine may set (#237), and "all" all
// at once. A setting the engine read would change one of settingCases.
var decimalSettings = map[string]func(){
	"TrimTrailingZeros":            func() { decimal.TrimTrailingZeros = false },
	"UseScientificNotation":        func() { decimal.UseScientificNotation = true },
	"DivisionPrecision4":           func() { decimal.DivisionPrecision = 4 },
	"DivisionPrecision40":          func() { decimal.DivisionPrecision = 40 },
	"PowPrecisionNegativeExponent": func() { decimal.PowPrecisionNegativeExponent = 2 },
	"ExpMaxIterations":             func() { decimal.ExpMaxIterations = 1 },
	"MarshalJSONWithoutQuotes":     func() { decimal.MarshalJSONWithoutQuotes = true },
	"MaxDecodeExponent":            func() { decimal.MaxDecodeExponent = 1 },
	"Zero":                         func() { decimal.Zero = decimal.New(1, 0) },
	"all": func() {
		decimal.TrimTrailingZeros = false
		decimal.UseScientificNotation = true
		decimal.DivisionPrecision = 4
		decimal.PowPrecisionNegativeExponent = 2
		decimal.ExpMaxIterations = 1
		decimal.MarshalJSONWithoutQuotes = true
		decimal.MaxDecodeExponent = 1
		decimal.Zero = decimal.New(1, 0)
	},
}

// settingsResource carries decimals as a resource writes them: trailing
// zeros, exponents, and Quantities in UCUM and calendar units.
const settingsResource = `{"resourceType":"Observation","valueQuantity":{"value":1.50,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"},"component":[{"valueDecimal":2.500},{"valueDecimal":1e2},{"valueDecimal":5e1},{"valueQuantity":{"value":2.540,"system":"http://unitsofmeasure.org","code":"cm"}},{"valueQuantity":{"value":1e10,"system":"http://unitsofmeasure.org","code":"a"}},{"valueDecimal":0e2000},{"valueQuantity":{"value":3e2,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}]}`

// settingsCases reach every place the engine formats, divides, re-reads or
// compares with zero a decimal. The answers were measured with the engine
// at 2e1f2b1, built on shopspring/decimal v1.4.0, which had none of these
// settings (#237); an error is written "error: " and its text.
var settingsCases = []struct{ expr, want string }{
	{"(10.0 / 4).toString()", "[2.5]"},
	{"(1.0 + 1.00).toString()", "[2]"},
	{"(0.0010 * 1000000).toString()", "[1000]"},
	{"(1.0 / 3).toString()", "[0.3333333333333333]"},
	{"(10 / 3)", "[3.3333333333333333]"},
	{"(100000000000.0 * 100000000000.0).toString()", "[10000000000000000000000]"},
	{"(1.0 * 10000000000000000000000).toString()", "[10000000000000000000000]"},
	{"(1.10 + 0).toString()", "[1.1]"},
	{"(1.10).toString()", "[1.10]"},
	{"(0.000).toString()", "[0.000]"},
	{"(-0.0).toString()", "[-0.0]"},
	{"(10.0 * 10).toString()", "[100]"},
	{"(1.0 * 1000).toString()", "[1000]"},
	{"(3.14159).round(2).toString()", "[3.14]"},
	{"(5.0).round(-1)", "[10]"},
	{"(5.0).round(-1).toString()", "[10]"},
	{"(1234.5678).round(2)", "[1234.57]"},
	{"(1.0 | 2.00 | 3.000).sum().toString()", "[6]"},
	{"(1.0 | 2.0).avg().toString()", "[1.5]"},
	{"(1 | 2 | 2).avg().toString()", "[1.5]"},
	{"(1.0 | 2.0 | 4.0).avg()", "[2.3333333333333333]"},
	{"(10.0 'mg' / 4).toString()", "[2.5 'mg']"},
	{"(10 'mg' / 3).toString()", "[3.3333333333333333 'mg']"},
	{"(1.50 'mg').round(1).toString()", "[1.5 'mg']"},
	{"(1.25 'mg').round(1)", "[1.3 'mg']"},
	{"(1234.5 'mg').round(-2)", "[1200 'mg']"},
	{"(1 'kg' + 1 'g').toString()", "[1001 'g']"},
	{"1 'g' = 1000 'mg'", "[true]"},
	{"1.0 'g' ~ 1000.0 'mg'", "[true]"},
	{"(1 'kg').toQuantity('g').toString()", "[1000 'g']"},
	{"(1 'kg').toQuantity('mg').toString()", "[1000000 'mg']"},
	{"(1 'h').toQuantity('min').toString()", "[60 'min']"},
	{"(1 '[in_i]').toQuantity('cm').toString()", "[2.54 'cm']"},
	{"(1 'cm').toQuantity('[in_i]').toString()", "[0.3937007874015748031496062992125984 '[in_i]']"},
	{"(30 'd').toQuantity('mo')", "[0.9856262833675564681724845995893224 'mo']"},
	{"(1 'Mm').toQuantity('m').toString()", "[1000000 'm']"},
	{"(1000 'g').toQuantity('kg')", "[1 'kg']"},
	{"(10 months).toQuantity('year')", "[0.8333333333333333 year]"},
	{"(18 months).toQuantity('year').toString()", "[1.5 year]"},
	{"(1 year).toQuantity('mo').toString()", "[12 'mo']"},
	{"1.0 ~ 1.04", "[true]"},
	{"1.0 ~ 1.06", "[true]"},
	{"0.0.toBoolean()", "[false]"},
	{"1.0.toBoolean()", "[true]"},
	{"0.0.convertsToBoolean()", "[true]"},
	{"(1.587).lowBoundary(8).toString()", "[1.58650000]"},
	{"1.58712345.lowBoundary(10).toString()", "[1.5871234450]"},
	{"1.58712345.highBoundary(12)", "[1.587123455000]"},
	{"(1.0).lowBoundary().toString()", "[0.95000000]"},
	{"@2020-01-01T10:00:00+05:20.timezoneOffsetOf()", "[5.3333333333333333]"},
	{"@2020-01-01T10:00:00+05:00.timezoneOffsetOf().toString()", "[5.0]"},
	{"@2020-01-01T10:00:00-03:30.timezoneOffsetOf()", "[-3.5]"},
	{"'1.50'.toDecimal().toString()", "[1.50]"},
	{"'1.50 \\'mg\\''.toQuantity().toString()", "[1.50 'mg']"},
	{"1.50.toQuantity().toString()", "[1.50 '1']"},
	{"'1e2'.toDecimal()", "[]"},
	{"@2020-01-01 + 1000000 years", "error: not a date or time on the calendar and the clock: date arithmetic by 1000000 years reaches past the years 1 to 9999"},
	{"@2020-01-01 + 1.5 'a'", "error: UCUM year and month durations require explicit conversion to calendar units: \"a\""},
	{"(@2020-01-01T10:00:00 + 90.5 'min').toString()", "[2020-01-01T11:30:00]"},
	{"value.value.toString()", "[1.50]"},
	{"(value + 1.5 'mg').toString()", "[3.00 'mg']"},
	{"value.toQuantity('g').toString()", "[0.0015 'g']"},
	{"component.value.ofType(decimal).sum().toString()", "[152.5]"},
	{"component.value.ofType(decimal).avg()", "[38.125]"},
	{"component[1].value.toString()", "[1e2]"},
	{"(component[1].value * 10).toString()", "[1000]"},
	{"component[1].value * 10 = 1000", "[true]"},
	{"component[2].value.round(-1)", "[50]"},
	{"(component[3].value / 3).toString()", "[0.8466666666666667 'cm']"},
	{"component[3].toQuantity('[in_i]').toString()", "[]"},
	{"component[3].value.toQuantity().toString()", "[2.540 'cm']"},
	{"@2020-01-01 + component[4].value", "error: not a date or time on the calendar and the clock: date arithmetic by 10000000000 year reaches past the years 1 to 9999"},
	{"component.value.ofType(decimal).avg().toString()", "[38.125]"},
	{"component[5].value + 1", "[1]"},
	{"0 ~ 0.001", "[true]"},
	{"component[6].value.toString()", "[300 'mg']"},
	{"(10 seconds).toQuantity('minutes')", "[0.1666666666666667 minutes]"},
}

func evaluateSettingsCases(t *testing.T) {
	t.Helper()
	for _, tc := range settingsCases {
		result, err := fhirpath.Evaluate([]byte(settingsResource), tc.expr)
		got := result.String()
		if err != nil {
			got = "error: " + err.Error()
		}
		if got != tc.want {
			t.Errorf("%s = %s, want %s", tc.expr, got, tc.want)
		}
	}
}

// What a program sets in the decimal library does not change a result: the
// engine formats, divides and reads its decimals itself (#237). Each setting
// is changed in a process of its own, so no other test sees it.
func TestResultsDoNotDependOnTheDecimalLibrarysSettings(t *testing.T) {
	if name := os.Getenv(decimalSettingEnv); name != "" {
		decimalSettings[name]()
		evaluateSettingsCases(t)
		return
	}
	evaluateSettingsCases(t)
	names := make([]string, 0, len(decimalSettings))
	for name := range decimalSettings {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestResultsDoNotDependOnTheDecimalLibrarysSettings$", "-test.count=1", "-test.v")
		cmd.Env = append(os.Environ(), decimalSettingEnv+"="+name)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("with %s changed: %v\n%s", name, err, out)
		} else if !strings.Contains(string(out), "--- PASS: TestResultsDoNotDependOnTheDecimalLibrarysSettings") {
			t.Errorf("with %s changed, the test did not run:\n%s", name, out)
		}
	}
}
