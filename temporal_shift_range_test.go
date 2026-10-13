package fhirpath_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// A time-valued quantity is read as a decimal before it shifts a date or a
// time, so that no amount wraps past the int range on the way. On a date, an
// amount longer than the calendar reaches past the years 1 to 9999 from any
// start: "If the resulting year is out of range, an error is thrown". A time
// of day is cyclic, so a clock amount is taken modulo a day. They wrapped:
// @2020-01-01 + 18446744073709551617 days was @2020-01-02. (#163)
func TestATemporalShiftDoesNotWrap(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient"}`)

	for _, expr := range []string{
		"@2020-01-01 + 18446744073709551617 days",
		"@2020-01-01 + 9223372036854775807 days",
		"@2020-01-01 + 9223372036854775807 weeks",
		"@2020-01-01 + 9223372036854775807 years",
		"@2020-01-01 + 9223372036854775807 months",
		"@2020-01-01 + 9223372036854775807 'h'",
		"@2020-01-01T10:00:00Z + 9300000000000000 seconds",
		"@2020-01-01T10:00:00Z + 100000000000000000000 'ms'",
		"@2020-01-01T10:00:00Z - 9223372036854775807 'min'",
		"@2020-01-01 + 7980 years",
		"@2020-01-01 - 2020 years",
	} {
		_, err := fhirpath.Evaluate(patient, expr)
		if !errors.Is(err, types.ErrInvalidTemporal) || !strings.Contains(err.Error(), "1 to 9999") {
			t.Errorf("%s: error %v, want the calendar's range", expr, err)
		}
	}

	for expr, want := range map[string]string{
		// Within the calendar, as before
		"@2020-01-01 + 7979 years":      "[9999-01-01]",
		"@2020-01-01 - 2019 years":      "[0001-01-01]",
		"@2020-01-01 + 7.9 days":        "[2020-01-08]",
		"@2000-01-01T00:00:00 - 1 'ms'": "[1999-12-31T23:59:59]",
		"@T23:30:00 + 1 hour":           "[00:30:00]",
		"@T01:00:00 + 48 hours":         "[01:00:00]",
		"@T10:00 + 0 days":              "[10:00]",
		// A time of day takes a clock amount modulo a day
		"@T10:00 + 9223372036854775807 hours":       "[17:00]",
		"@T10:00 - 9223372036854775807 hours":       "[03:00]",
		"@T10:00 + 100000000000000000000.5 seconds": "[19:46]",
	} {
		result, err := fhirpath.Evaluate(patient, expr)
		if err != nil {
			t.Errorf("%s: %v", expr, err)
			continue
		}
		if got := result.String(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}

	// A date component still cannot shift a time, whatever its amount
	for _, expr := range []string{"@T10:00 + 1 day", "@T10:00 - 1 week", "@T10:00 + 9223372036854775808000 days"} {
		if _, err := fhirpath.Evaluate(patient, expr); !errors.Is(err, types.ErrDateComponentOnTime) {
			t.Errorf("%s: error %v, want a date component refused on a time", expr, err)
		}
	}
}
