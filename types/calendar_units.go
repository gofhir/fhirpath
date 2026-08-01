package types

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
