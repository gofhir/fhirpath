package fhirpath

import "testing"

// The examples the specification gives for each extractor, run as written.
func TestComponentExtractorsFromTheSpecification(t *testing.T) {
	tests := []struct {
		expr string
		want string
	}{
		{"@2014-01-05T10:30:00.000.yearOf()", "2014"},
		{"@2014-01-05T10:30:00.000.monthOf()", "1"},
		{"@2014-01-05T10:30:00.000.dayOf()", "5"},
		{"@2012-01-01T03:30:40.002-07:00.hourOf()", "3"},
		{"@2012-01-01T16:30:40.002-07:00.hourOf()", "16"},
		{"@2012-01-01T12:30:40.002-07:00.minuteOf()", "30"},
		{"@2012-01-01T12:30:40.002-07:00.secondOf()", "40"},
		{"@2012-01-01T12:30:00.002-07:00.millisecondOf()", "2"},
		{"@2012-01-01T12:30:00.000-07:00.timezoneOffsetOf()", "-7.0"},
		{"@2012-01-01T12:30:00.000+08:45.timezoneOffsetOf()", "8.75"},
		{"@2012-01-01T12:30:00.000-07:00.dateOf()", "2012-01-01"},
		{"@2012-01-01T12:30:00.000-07:00.timeOf()", "12:30:00.000"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate([]byte(`{"resourceType":"Patient"}`), tt.expr)
			if err != nil {
				t.Fatalf("%s: %v", tt.expr, err)
			}
			if len(result) != 1 {
				t.Fatalf("got %d results, want 1", len(result))
			}
			if got := result[0].String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// A component the value does not carry is empty, not zero. The specification
// gives one of these — @2012.monthOf() — and the same rule decides the rest.
func TestComponentAbsentIsEmpty(t *testing.T) {
	for _, expr := range []string{
		"@2012.monthOf()",
		"@2012.dayOf()",
		"@2012-01.dayOf()",
		"@2012-01-01.hourOf()",
		"@2012-01-01T12.minuteOf()",
		"@2012-01-01T12:30.secondOf()",
		"@2012-01-01T12:30:40.millisecondOf()",
		"@2012-01-01T12:30:40.timezoneOffsetOf()",
		"@2012-01-01.timeOf()",
		// A time without a day: the month and day are absent even though the
		// value carries a time.
		"@2012T12:30:00.monthOf()",
		"@2012T12:30:00.dayOf()",
		"@2012-05T12:30:00.dayOf()",
		"@T12:30.secondOf()",
		"@T12:30:40.millisecondOf()",
	} {
		t.Run(expr, func(t *testing.T) {
			result, err := Evaluate([]byte(`{"resourceType":"Patient"}`), expr)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if !result.Empty() {
				t.Errorf("got %v, want an empty collection", result)
			}
		})
	}
}

// An offset in whole quarters of an hour divides exactly; twenty minutes is a
// third of an hour and does not, and the result says so rather than stopping at
// a few places and claiming to be exact.
func TestTimezoneOffsetDivision(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"@2012-01-01T12:30:00.000Z.timezoneOffsetOf()", "0.0"},
		{"@2012-01-01T12:30:00.000+05:45.timezoneOffsetOf()", "5.75"},
		{"@2012-01-01T12:30:00.000-03:30.timezoneOffsetOf()", "-3.5"},
		{"@2012-01-01T12:30:00.000-07:00.timezoneOffsetOf()", "-7.0"},
		{"@2012-01-01T12:30:00.000+00:20.timezoneOffsetOf()", "0.3333333333333333"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate([]byte(`{"resourceType":"Patient"}`), tt.expr)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if len(result) != 1 {
				t.Fatalf("got %d results, want 1", len(result))
			}
			if got := result[0].String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// dateOf keeps the precision the value was written with, rather than filling in
// what it does not say.
func TestDateOfKeepsPrecision(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"@2012.dateOf()", "2012"},
		{"@2012-05.dateOf()", "2012-05"},
		{"@2012-05-07.dateOf()", "2012-05-07"},
		{"@2012T12:30:00.dateOf()", "2012"},
		{"@2012-05T12:30:00.dateOf()", "2012-05"},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := Evaluate([]byte(`{"resourceType":"Patient"}`), tt.expr)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if len(result) != 1 {
				t.Fatalf("got %d results, want 1", len(result))
			}
			if got := result[0].String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// A value that is not temporal at all gives empty, and the wrong kind of
// temporal value does too: timeOf asks for a DateTime, and a Date has no time
// in it.
func TestComponentOfWrongTypeIsEmpty(t *testing.T) {
	for _, expr := range []string{
		"'abc'.yearOf()",
		"5.monthOf()",
		"true.dayOf()",
		"@T12:30:00.yearOf()",
		"@2012-01-01.timezoneOffsetOf()",
		"@T12:30:00.dateOf()",
		"@T12:30:00.timeOf()",
	} {
		t.Run(expr, func(t *testing.T) {
			result, err := Evaluate([]byte(`{"resourceType":"Patient"}`), expr)
			if err != nil {
				t.Fatalf("%v", err)
			}
			if !result.Empty() {
				t.Errorf("got %v, want an empty collection", result)
			}
		})
	}
}

// "If the input collection contains multiple items, the evaluation of the
// expression will end and signal an error to the calling environment."
func TestComponentOfManyItemsIsAnError(t *testing.T) {
	patient := []byte(`{"resourceType":"Patient","name":[
		{"family":"a"},{"family":"b"}]}`)

	// Two dates reached through a collection.
	if _, err := Evaluate(patient, "(@2012-01-01 | @2013-01-01).yearOf()"); err == nil {
		t.Error("expected an error for a collection of two dates, got none")
	}

	if _, err := Evaluate(patient, "Patient.name.family.yearOf()"); err == nil {
		t.Error("expected an error for a collection of two strings, got none")
	}
}

// An empty input gives empty rather than an error, which is what makes these
// safe to chain onto a path that may find nothing.
func TestComponentOfEmptyIsEmpty(t *testing.T) {
	result, err := Evaluate([]byte(`{"resourceType":"Patient"}`), "Patient.birthDate.yearOf()")
	if err != nil {
		t.Fatalf("%v", err)
	}
	if !result.Empty() {
		t.Errorf("got %v, want an empty collection", result)
	}
}

// The calendar-unit spellings answer the same thing, which is what keeps the
// two names from drifting. They need backticks: the grammar reads a bare month
// after a dot as the unit.
func TestCalendarUnitSpellingsAgree(t *testing.T) {
	pairs := [][2]string{
		{"@2014-01-05.`year`()", "@2014-01-05.yearOf()"},
		{"@2014-01-05.`month`()", "@2014-01-05.monthOf()"},
		{"@2014-01-05.`day`()", "@2014-01-05.dayOf()"},
		{"@2014-01-05T10:30:40.123.`hour`()", "@2014-01-05T10:30:40.123.hourOf()"},
		{"@2014-01-05T10:30:40.123.`minute`()", "@2014-01-05T10:30:40.123.minuteOf()"},
		{"@2014-01-05T10:30:40.123.`second`()", "@2014-01-05T10:30:40.123.secondOf()"},
		{"@2014-01-05T10:30:40.123.`millisecond`()", "@2014-01-05T10:30:40.123.millisecondOf()"},
		{"@2012-01-01.`month`()", "@2012-01-01.monthOf()"},
		{"@2012.`month`()", "@2012.monthOf()"},
	}

	for _, pair := range pairs {
		t.Run(pair[0], func(t *testing.T) {
			old, err := Evaluate([]byte(`{"resourceType":"Patient"}`), pair[0])
			if err != nil {
				t.Fatalf("%s: %v", pair[0], err)
			}
			current, err := Evaluate([]byte(`{"resourceType":"Patient"}`), pair[1])
			if err != nil {
				t.Fatalf("%s: %v", pair[1], err)
			}

			if len(old) != len(current) {
				t.Fatalf("%s gave %d results, %s gave %d", pair[0], len(old), pair[1], len(current))
			}
			for i := range old {
				if old[i].String() != current[i].String() {
					t.Errorf("%s gave %q, %s gave %q", pair[0], old[i].String(), pair[1], current[i].String())
				}
			}
		})
	}
}
