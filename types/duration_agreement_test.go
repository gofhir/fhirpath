package types

import (
	"errors"
	"testing"

	ucumfhir "github.com/gofhir/ucum/v4/fhir"
)

// TestDurationUnitsAgreeWithUCUMLibrary checks this package's duration table
// against the UCUM library's answer for the same question.
//
// Both encode the same rule — which UCUM duration codes may take part in
// date/time arithmetic — and they have to agree. This engine keeps its own table
// because it needs more than a yes or no: a unit resolves to the field it shifts
// and by how much, which the library does not express. So the duplication is
// deliberate, and this test is what makes it safe: if either side changes its
// mind, the disagreement surfaces here rather than as a wrong answer.
//
// The rule itself is not "seconds and below", which is how FHIRPath N1 reads at
// first glance. It is that a calendar duration and its UCUM counterpart have to
// measure the same span: 1 'wk' is exactly 1 week, while 1 'a' is 365.25 days
// and 1 year is a calendar year. The conformance suite settles it — it accepts
// 'wk', 'd', 'h' and 'min' and rejects only 'a' and 'mo' — and FHIRPath 3.0.0
// writes each code into the table explicitly.
func TestDurationUnitsAgreeWithUCUMLibrary(t *testing.T) {
	// Every UCUM code for a duration, whether or not this engine accepts it
	codes := []string{"ms", "s", "min", "h", "d", "wk", "mo", "a"}

	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			_, _, _, _, err := durationParts(1, code)
			weAccept := err == nil
			theyAccept := ucumfhir.AllowedInDateTimeArithmetic(code)

			if weAccept != theyAccept {
				t.Errorf("disagreement on %q: this package accepts=%v, gofhir/ucum accepts=%v",
					code, weAccept, theyAccept)
			}

			// The two we refuse are refused for a stated reason, not by omission
			if !weAccept && !errors.Is(err, ErrCalendarConversionRequired) {
				t.Errorf("%q was refused with %v, want ErrCalendarConversionRequired", code, err)
			}
		})
	}
}

// TestCalendarKeywordsHaveUCUMCounterparts checks the other direction: every
// calendar keyword except year and month maps onto a UCUM code that means the
// same thing, which is what lets the two be used interchangeably.
func TestCalendarKeywordsHaveUCUMCounterparts(t *testing.T) {
	// The UCUM code FHIRPath 3.0.0 names alongside each calendar keyword
	counterparts := map[string]string{
		unitNameWeek:        "wk",
		unitNameDay:         "d",
		unitNameHour:        "h",
		unitNameMinute:      "min",
		unitNameSecond:      "s",
		unitNameMillisecond: "ms",
	}

	for keyword, code := range counterparts {
		t.Run(keyword, func(t *testing.T) {
			years, months, days, millis, err := durationParts(1, keyword)
			if err != nil {
				t.Fatalf("calendar keyword %q was refused: %v", keyword, err)
			}

			ucumYears, ucumMonths, ucumDays, ucumMillis, err := durationParts(1, code)
			if err != nil {
				t.Fatalf("UCUM code %q was refused: %v", code, err)
			}

			if years != ucumYears || months != ucumMonths || days != ucumDays || millis != ucumMillis {
				t.Errorf("1 %s shifts (%d,%d,%d,%d) but 1 '%s' shifts (%d,%d,%d,%d); "+
					"the specification says they are equal and equivalent",
					keyword, years, months, days, millis,
					code, ucumYears, ucumMonths, ucumDays, ucumMillis)
			}
		})
	}

	// Year and month are the exception, and the reason the others are not: a
	// UCUM year is a fixed 365.25 days, a calendar year is not
	for keyword, code := range map[string]string{unitNameYear: "a", unitNameMonth: "mo"} {
		t.Run(keyword+" has no exact UCUM counterpart", func(t *testing.T) {
			if _, _, _, _, err := durationParts(1, keyword); err != nil {
				t.Errorf("calendar keyword %q was refused: %v", keyword, err)
			}
			if _, _, _, _, err := durationParts(1, code); !errors.Is(err, ErrCalendarConversionRequired) {
				t.Errorf("UCUM code %q was accepted, want ErrCalendarConversionRequired", code)
			}
		})
	}
}
