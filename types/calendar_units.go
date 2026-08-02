package types

import "github.com/shopspring/decimal"

// The calendar unit keywords the grammar accepts as an unquoted unit, from the
// rules dateTimePrecision and pluralDateTimePrecision.
//
// Three tables in this package key off these names: how a duration is applied to
// a date, whether a unit renders quoted or bare, and which precisions
// difference() and duration() accept. They have to agree, and naming the words
// once is what keeps them agreeing — a table that quietly spelled one of them
// differently would fail only on the input that used it.
const (
	unitNameYear        = "year"
	unitNameMonth       = "month"
	unitNameWeek        = "week"
	unitNameDay         = "day"
	unitNameHour        = "hour"
	unitNameMinute      = "minute"
	unitNameSecond      = "second"
	unitNameMillisecond = "millisecond"
)

// calendarUnitNames lists the keywords from coarsest to finest.
var calendarUnitNames = []string{
	unitNameYear, unitNameMonth, unitNameWeek, unitNameDay,
	unitNameHour, unitNameMinute, unitNameSecond, unitNameMillisecond,
}

// withPlurals returns the given unit names along with their plural forms, which
// the grammar accepts interchangeably: 1 year and 1 years are the same quantity.
func withPlurals(names []string) []string {
	both := make([]string, 0, len(names)*2)
	for _, name := range names {
		both = append(both, name, name+"s")
	}
	return both
}

// UCUMSystem identifies UCUM as the code system of a FHIR Quantity.
const UCUMSystem = "http://unitsofmeasure.org"

// fhirQuantityCalendarUnits is the map FHIR applies when a FHIR Quantity is
// evaluated as a FHIRPath System.Quantity.
//
// FHIR R5, "Using FHIRPath with FHIR", Use of FHIR Quantity: "As part of the
// mapping, time-valued UCUM units are mapped to the calendar duration units
// defined in FHIRPath, according to the following map".
//
// The map is FHIR's, and it is narrower than the set of UCUM durations this
// package understands: 'wk' and 'ms' are absent from it, so a Quantity carrying
// either keeps its UCUM code. That is not an oversight to correct here — a UCUM
// week and a calendar week are the same span, so nothing turns on it, and a
// mapping FHIR does not specify is not ours to invent.
var fhirQuantityCalendarUnits = map[string]string{
	"a":   unitNameYear,
	"mo":  unitNameMonth,
	"d":   unitNameDay,
	"h":   unitNameHour,
	"min": unitNameMinute,
	"s":   unitNameSecond,
}

// CalendarUnitForUCUMCode returns the calendar duration keyword FHIR maps a
// time-valued UCUM code onto, and whether the code is one it maps.
//
// This is what lets a Quantity read from FHIR data take part in date arithmetic
// at all: a Quantity of 1 'a' would be refused, since a UCUM year is a definite
// 365.25 days and cannot be added to a calendar, while the 1 year it maps to is
// exactly what the calendar can add.
func CalendarUnitForUCUMCode(code string) (string, bool) {
	keyword, ok := fhirQuantityCalendarUnits[code]
	return keyword, ok
}

// SecondUnitMilliseconds converts a quantity given in seconds to whole
// milliseconds, reporting false when the unit is not a second.
//
// This is the one place a duration's fractional part survives: "The decimal
// portion of the time-valued quantity is only applied for second or millisecond
// precisions; for all other precisions, the decimal portion is ignored, since
// date/time arithmetic is performed with calendar duration semantics." So
// 0.1 's' shifts by 100 milliseconds, while 7.9 days shifts by seven.
func SecondUnitMilliseconds(value decimal.Decimal, unit string) (int, bool) {
	switch unit {
	case unitNameSecond, unitNameSecond + "s", "s":
		return int(value.Mul(decimal.NewFromInt(1000)).IntPart()), true
	}
	return 0, false
}
