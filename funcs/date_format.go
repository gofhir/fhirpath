package funcs

import (
	"fmt"
	"strconv"
	"strings"
)

// 3.0.0 lets toDate() and toDateTime() read a string by a template of format
// codes (trial use): "When the optional format parameter is provided, it is
// used as a template instead of the default format." Codes are runs of one
// letter, "case-sensitive", and "Any character in the format string parameter
// not represented by a format code is treated as a literal and must match
// exactly in the input string."
//
// The codes the text marks required are read, yyyy, MM, dd, hh, HH, mm, ss,
// S (one per fractional digit), a and Z, and of the optional ones those that
// need no locale: yy, M, d, h, H, m and s. The localized month names, MMM and
// MMMM, and a zone by name, z, are not, and a template using them reads
// nothing.

// formatted is what a template read from a string: the components it named,
// each -1 where it named none.
type formatted struct {
	year, month, day     int
	hour, minute, second int
	fraction             string
	pm, meridiem, ampm   bool
	hours                int
	offset               string
}

// parseFormatted reads value by template, reporting false where it does not
// match: a literal that differs, a component of the wrong width, a code the
// engine does not read, or text left over.
func parseFormatted(value, template string) (formatted, bool) {
	f := formatted{year: -1, month: -1, day: -1, hour: -1, minute: -1, second: -1}
	pos := 0
	for i := 0; i < len(template); {
		c := template[i]
		run := 1
		for i+run < len(template) && template[i+run] == c {
			run++
		}
		if !strings.ContainsRune("yMdhHmsSaZz", rune(c)) {
			// A literal, which matches only itself
			if pos >= len(value) || value[pos] != c {
				return f, false
			}
			pos++
			i++
			continue
		}
		i += run
		var ok bool
		if pos, ok = f.read(value, pos, c, run); !ok {
			return f, false
		}
	}
	if pos != len(value) {
		return f, false
	}
	ok := f.toDayClock()
	return f, ok
}

// read reads one code, a run of the letter c, at pos.
func (f *formatted) read(value string, pos int, c byte, run int) (int, bool) {
	if strings.IndexByte("yMd", c) >= 0 {
		return f.readDate(value, pos, c, run)
	}
	return f.readClock(value, pos, c, run)
}

// readDate reads a code of the date: a year, a month or a day.
func (f *formatted) readDate(value string, pos int, c byte, run int) (int, bool) {
	var ok bool
	switch {
	case c == 'y' && run == 4:
		f.year, pos, ok = digits(value, pos, 4, 4)
	case c == 'y' && run == 2:
		var yy int
		yy, pos, ok = digits(value, pos, 2, 2)
		// "A common approach is to interpret values 00-49 as 2000-2049 and
		// 50-99 as 1950-1999"
		f.year = 1900 + yy
		if yy < 50 {
			f.year = 2000 + yy
		}
	case c == 'M' && run <= 2:
		f.month, pos, ok = digits(value, pos, run, 2)
	case c == 'd' && run <= 2:
		f.day, pos, ok = digits(value, pos, run, 2)
	}
	return pos, ok
}

// readClock reads a code of the time of day, AM/PM or the offset.
func (f *formatted) readClock(value string, pos int, c byte, run int) (int, bool) {
	var ok bool
	switch {
	case (c == 'H' || c == 'h') && run <= 2:
		f.hour, pos, ok = digits(value, pos, run, 2)
		f.meridiem = f.meridiem || c == 'h'
		f.hours++
	case c == 'm' && run <= 2:
		f.minute, pos, ok = digits(value, pos, run, 2)
	case c == 's' && run <= 2:
		f.second, pos, ok = digits(value, pos, run, 2)
	case c == 'S':
		f.fraction, pos, ok = fraction(value, pos, run)
	case c == 'a' && run == 1:
		f.pm, pos, ok = meridiem(value, pos)
		f.ampm = true
	case c == 'Z' && run == 1:
		f.offset, pos, ok = zoneOffset(value, pos)
	}
	return pos, ok
}

// toDayClock reads an hour of AM/PM, h or hh, which is 1 to 12, as an hour of
// the day; a means it is the afternoon.
func (f *formatted) toDayClock() bool {
	// One hour, and an AM/PM specifier exactly where the hour is one of
	// AM/PM: hh alone does not say which half of the day, and a beside HH
	// could contradict it
	if f.hours > 1 || f.meridiem != f.ampm {
		return false
	}
	if !f.meridiem {
		return true
	}
	if f.hour < 1 || f.hour > 12 {
		return false
	}
	f.hour %= 12
	if f.pm {
		f.hour += 12
	}
	return true
}

// fraction reads a fraction of a second, one digit per S: "consecutive
// fractional seconds are grouped together, e.g. SSS for milliseconds".
func fraction(value string, pos, run int) (digitsRead string, next int, ok bool) {
	end := pos + run
	if end > len(value) || !allDigits(value[pos:end]) {
		return "", pos, false
	}
	return value[pos:end], end, true
}

// digits reads from minimum to maximum digits at pos.
func digits(value string, pos, minimum, maximum int) (n, next int, ok bool) {
	end := pos
	for end < len(value) && end-pos < maximum && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	if end-pos < minimum {
		return 0, pos, false
	}
	n, err := strconv.Atoi(value[pos:end])
	return n, end, err == nil
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// meridiem reads an AM/PM specifier as en-US writes it, A, AM, P or PM,
// reporting whether it is the afternoon.
func meridiem(value string, pos int) (pm bool, next int, ok bool) {
	// Two letters at most are read, not the rest of the input
	rest := strings.ToUpper(value[pos:min(pos+2, len(value))])
	for _, spec := range []struct {
		text string
		pm   bool
	}{{"AM", false}, {"PM", true}, {"A", false}, {"P", true}} {
		if strings.HasPrefix(rest, spec.text) {
			return spec.pm, pos + len(spec.text), true
		}
	}
	return false, pos, false
}

// zoneOffset reads an offset, "+0200, -0500" or "Z literal for UTC", as an
// offset FHIRPath writes, +02:00 or Z. +02:00 is read too.
func zoneOffset(value string, pos int) (offset string, next int, ok bool) {
	if pos < len(value) && value[pos] == 'Z' {
		return "Z", pos + 1, true
	}
	if pos >= len(value) || (value[pos] != '+' && value[pos] != '-') {
		return "", pos, false
	}
	sign := value[pos : pos+1]
	hours, next, ok := digits(value, pos+1, 2, 2)
	if !ok {
		return "", pos, false
	}
	if next < len(value) && value[next] == ':' {
		next++
	}
	minutes, next, ok := digits(value, next, 2, 2)
	if !ok {
		return "", pos, false
	}
	return fmt.Sprintf("%s%02d:%02d", sign, hours, minutes), next, true
}

// date writes what the template read as a date literal's text, to the
// finest component it named; false where it named no year, or a day without a
// month.
func (f formatted) date() (string, bool) {
	if f.year < 0 || f.day >= 0 && f.month < 0 {
		return "", false
	}
	text := fmt.Sprintf("%04d", f.year)
	if f.month >= 0 {
		text += fmt.Sprintf("-%02d", f.month)
		if f.day >= 0 {
			text += fmt.Sprintf("-%02d", f.day)
		}
	}
	return text, true
}

// dateTime writes what the template read as a DateTime's text, to the finest
// component it named, with the offset it read.
func (f formatted) dateTime() (string, bool) {
	text, ok := f.date()
	if !ok {
		return "", false
	}
	// A component without the one above it names no time: a minute without
	// an hour, a second without a minute, a fraction without a second
	if f.minute >= 0 && f.hour < 0 || f.second >= 0 && f.minute < 0 || f.fraction != "" && f.second < 0 {
		return "", false
	}
	if f.hour < 0 || f.month < 0 || f.day < 0 {
		return text, f.hour < 0 && f.offset == ""
	}
	text += fmt.Sprintf("T%02d", f.hour)
	if f.minute >= 0 {
		text += fmt.Sprintf(":%02d", f.minute)
		if f.second >= 0 {
			text += fmt.Sprintf(":%02d", f.second)
			if f.fraction != "" {
				text += "." + f.fraction
			}
		}
	}
	return text + f.offset, true
}
