package types

import (
	"errors"
	"testing"
	"time"
)

// placed returns a DateTime written without an offset and given a default,
// the way a CQL engine places a literal at the evaluation request's offset.
func placed(t *testing.T, written string, offset time.Duration) DateTime {
	t.Helper()
	return mustDateTime(t, written).WithDefaultOffset(offset)
}

// compareOutcome folds an order and its unknown answers into one comparable
// string, so a table can name "unknown" without caring which sentinel says it.
func compareOutcome(t *testing.T, left, right Value) string {
	t.Helper()

	cmp, err := compareTemporalValues(left, right)
	switch {
	case err == nil && cmp < 0:
		return "<"
	case err == nil && cmp > 0:
		return ">"
	case err == nil:
		return "="
	case IsUnknownTemporalComparison(err):
		return "unknown"
	}
	t.Fatalf("comparing %v and %v: %v", left, right, err)
	return ""
}

func mirror(outcome string) string {
	switch outcome {
	case "<":
		return ">"
	case ">":
		return "<"
	}
	return outcome
}

// A placed value coarser than the hour against a finer one at another offset:
// the finer value is re-expressed at the coarser one's offset and the two are
// compared precision by precision, so @2012-01-01T placed at -03:00 is the day
// 2012-01-01 in that zone, and 05:00Z, which is 02:00 there, falls within it.
// Both operand orders must give the mirrored answer.
func TestCoarsePlacedValueAgainstAFinerValueAtAnotherOffset(t *testing.T) {
	h := time.Hour
	tests := []struct {
		name   string
		coarse DateTime
		fine   string
		want   string // coarse against fine
	}{
		{"day, same day in its zone", placed(t, "2012-01-01", -3*h), "2012-01-01T05:00:00.000Z", "unknown"},
		{"day, previous day in its zone", placed(t, "2012-01-01", -3*h), "2012-01-01T01:00:00.000Z", ">"},
		{"day, next day in its zone", placed(t, "2012-01-01", -3*h), "2012-01-02T03:00:00.000Z", "<"},
		{"day, last hour of its day", placed(t, "2012-01-01", -3*h), "2012-01-02T02:59:59.999Z", "unknown"},
		{"day, hour precision", placed(t, "2012-01-01", -3*h), "2012-01-01T05Z", "unknown"},
		{"day, minute precision", placed(t, "2012-01-01", -3*h), "2012-01-01T05:00Z", "unknown"},
		{"day, second precision", placed(t, "2012-01-01", -3*h), "2012-01-01T05:00:00Z", "unknown"},
		{"month, last hour of its month in its zone", placed(t, "2012-01", -3*h), "2012-02-01T02:00:00.000Z", "unknown"},
		{"month, first hour of the next in its zone", placed(t, "2012-01", -3*h), "2012-02-01T03:00:00.000Z", "<"},
		{"year, next year in its zone", placed(t, "2012", -3*h), "2013-01-01T02:00:00.000Z", "unknown"},
		{"year, earlier year in its zone", placed(t, "2012", -3*h), "2012-01-01T02:00:00.000Z", ">"},
		{"year +14, the year starts the day before in UTC", placed(t, "2012", 14*h), "2012-01-01T00:00:00.000Z", "unknown"},
		{"year +14, before it starts at UTC", placed(t, "2012", 14*h), "2011-12-31T09:00:00.000Z", ">"},
		{"year +14, in the next year in its zone", placed(t, "2012", 14*h), "2012-12-31T10:00:00.000Z", "<"},
		{"year +14, last hour of its zone's year", placed(t, "2012", 14*h), "2012-12-31T09:59:59.999Z", "unknown"},
		{"year -14, the year starts in its zone later", placed(t, "2012", -14*h), "2012-01-01T13:59:59.999Z", ">"},
		{"year -14, ends at the zone's next year", placed(t, "2012", -14*h), "2013-01-01T14:00:00.000Z", "<"},
		{"year -14, inside", placed(t, "2012", -14*h), "2012-06-01T00:00:00.000Z", "unknown"},
		{"day, a written offset is converted too", placed(t, "2012-01-01", -3*h), "2012-01-01T10:00:00.000+14:00", ">"},
		{"day, written +14 on the day after", placed(t, "2012-01-01", -3*h), "2012-01-02T18:00:00.000+14:00", "<"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fine, err := NewDateTime(tt.fine)
			if err != nil {
				t.Fatal(err)
			}
			if got := compareOutcome(t, tt.coarse, fine); got != tt.want {
				t.Errorf("coarse against fine: got %s, want %s", got, tt.want)
			}
			if got := compareOutcome(t, fine, tt.coarse); got != mirror(tt.want) {
				t.Errorf("fine against coarse: got %s, want %s", got, mirror(tt.want))
			}
		})
	}
}

// Two values of one coarse unit at different defaults name spans, and the spans
// order them when one ends before the other starts. Otherwise they overlap
// without being the same and the order is unknown.
func TestCoarsePlacedValuesAtDifferentDefaultsAreComparedAsSpans(t *testing.T) {
	h := time.Hour
	tests := []struct {
		name        string
		left, right DateTime
		want        string
	}{
		{"years eight apart at the extremes", placed(t, "2012", 14*h), placed(t, "2020", -14*h), "<"},
		{"years one apart at the extremes", placed(t, "2012", 14*h), placed(t, "2013", -14*h), "<"},
		{"adjacent years, the earlier ends before the later starts", placed(t, "2012", -3*h), placed(t, "2013", -3*h-30*time.Minute), "<"},
		{"the same year at two defaults", placed(t, "2012", -3*h), placed(t, "2012", 14*h), "unknown"},
		{"the same day at two defaults", placed(t, "2012-01-01", -3*h), placed(t, "2012-01-01", 5*h), "unknown"},
		// The day at -03:00 ends at 03:00Z on the 2nd, an hour before the
		// next day at -04:00 starts: a span one unit too long would overlap it
		{"adjacent days, an hour apart in UTC", placed(t, "2012-01-01", -3*h), placed(t, "2012-01-02", -4*h), "<"},
		{"adjacent months, an hour apart in UTC", placed(t, "2012-01", -3*h), placed(t, "2012-02", -4*h), "<"},
		// January at -03:00 ends at 03:00Z on February 1st, six hours after
		// February at +03:00 starts: a month held as thirty days would end
		// before it
		{"adjacent months that overlap by six hours", placed(t, "2012-01", -3*h), placed(t, "2012-02", 3*h), "unknown"},
		// The day at +14:00 ends at 10:00Z, where the same day at -10:00
		// starts: spans that touch are apart
		{"spans that touch", placed(t, "2012-01-01", 14*h), placed(t, "2012-01-01", -10*h), "<"},
		{"days a week apart", placed(t, "2012-01-01", -3*h), placed(t, "2012-01-08", 14*h), "<"},
		{"adjacent days, far apart in UTC", placed(t, "2012-01-01", 14*h), placed(t, "2012-01-02", -14*h), "<"},
		{"adjacent days that overlap in UTC", placed(t, "2012-01-02", 14*h), placed(t, "2012-01-01", -14*h), "unknown"},
		{"months a year apart", placed(t, "2012-01", 14*h), placed(t, "2013-01", -14*h), "<"},
		{"the same month at two defaults", placed(t, "2012-01", 14*h), placed(t, "2012-01", -14*h), "unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareOutcome(t, tt.left, tt.right); got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
			if got := compareOutcome(t, tt.right, tt.left); got != mirror(tt.want) {
				t.Errorf("reversed: got %s, want %s", got, mirror(tt.want))
			}
		})
	}
}

// A coarse value that has an offset has nothing to be compared with in a value
// that has none: the order is unknown, whichever the precisions and the order
// of the operands. Without that, a bare day would equal two placed days that
// differ from each other.
func TestCoarsePlacedValueAgainstAValueWithoutAnOffset(t *testing.T) {
	day := placed(t, "2012-01-01", -3*time.Hour)

	bare := []Value{
		mustDateTime(t, "2012-01-01"),
		mustDateTime(t, "2012-01-01T10:00:00.000"),
		mustDateTime(t, "2012-01-01T10"),
		mustDateTime(t, "2012"),
		mustDate(t, "2012-01-01"),
		mustDate(t, "2012"),
		mustDate(t, "2011-01-01"),
		mustDateTime(t, "2013-06-01"),
	}
	for _, other := range bare {
		t.Run(other.String(), func(t *testing.T) {
			if _, err := compareTemporalValues(day, other); !errors.Is(err, ErrOffsetMismatch) {
				t.Errorf("placed against bare: got %v, want ErrOffsetMismatch", err)
			}
			if _, err := compareTemporalValues(other, day); !errors.Is(err, ErrOffsetMismatch) {
				t.Errorf("bare against placed: got %v, want ErrOffsetMismatch", err)
			}
			if equal, err := EqualTemporal(day, other); equal || !errors.Is(err, ErrOffsetMismatch) {
				t.Errorf("EqualTemporal: got %v, %v", equal, err)
			}
		})
	}
}

func mustDate(t *testing.T, s string) Date {
	t.Helper()

	value, err := NewDate(s)
	if err != nil {
		t.Fatalf("NewDate(%q): %v", s, err)
	}
	return value
}

// A leap second is held as the second after it, so a value written 23:59:60 on
// the last day of the year is held in the next year. Against a placed day it is
// converted as the second before that, which keeps it in the minute it was
// written in: two spellings of one moment compare alike.
func TestCoarsePlacedValueAgainstALeapSecond(t *testing.T) {
	day := placed(t, "2016-12-31", -3*time.Hour)

	x := mustDateTime(t, "2016-12-31T23:59:60-03:00")
	y := mustDateTime(t, "2017-01-01T02:59:60Z")
	if !x.Equal(y) {
		t.Fatal("the premise is that X and Y are the same moment")
	}

	if got, want := compareOutcome(t, day, x), "unknown"; got != want {
		t.Errorf("against X (same offset): got %s, want %s", got, want)
	}
	if got, want := compareOutcome(t, day, y), compareOutcome(t, day, x); got != want {
		t.Errorf("against Y: got %s, but X gave %s", got, want)
	}
	if got, want := compareOutcome(t, y, day), mirror(compareOutcome(t, day, x)); got != want {
		t.Errorf("Y against the day: got %s, want %s", got, want)
	}

	// And the year, where the leap second is the last of it.
	year := placed(t, "2016", -3*time.Hour)
	if got, want := compareOutcome(t, year, y), compareOutcome(t, year, x); got != want {
		t.Errorf("year against Y: got %s, but X gave %s", got, want)
	}
	if got := compareOutcome(t, year, y); got != "unknown" {
		t.Errorf("year against Y: got %s, want unknown", got)
	}
}

// Equal is an equivalence over a set of placed, bare and written values, which
// is what lets distinct() and union() answer the same whatever order they are
// given.
func TestPlacedAndBareValuesEqualTransitively(t *testing.T) {
	h := time.Hour
	values := []Value{
		placed(t, "2012-01-01", -3*h),
		mustDateTime(t, "2012-01-01"),
		placed(t, "2012-01-01", 14*h),
		mustDate(t, "2012-01-01"),
		mustDateTime(t, "2012-01-01T00:00:00.000-03:00"),
		mustDateTime(t, "2012-01-01T03:00:00.000Z"),
		placed(t, "2012", -3*h),
		placed(t, "2012-01", 14*h),
	}

	for i, a := range values {
		if !equalsItself(a) {
			t.Errorf("%d is not equal to itself", i)
		}
		for j, b := range values {
			if a.Equal(b) != b.Equal(a) {
				t.Errorf("Equal is not symmetric over %d, %d", i, j)
			}
			for k, c := range values {
				if a.Equal(b) && b.Equal(c) && !a.Equal(c) {
					t.Errorf("Equal is not transitive over %d, %d, %d", i, j, k)
				}
			}
		}
	}

	// The set holds the bare day and the Date as one, the written pair as one,
	// and every other value apart.
	var sizes = map[int]bool{}
	permutations(len(values), func(order []int) {
		c := make(Collection, 0, len(order))
		for _, i := range order {
			c = append(c, values[i])
		}
		sizes[len(c.Distinct())] = true

		var left, right Collection
		for n, i := range order {
			if n%2 == 0 {
				left = append(left, values[i])
			} else {
				right = append(right, values[i])
			}
		}
		sizes[1000+len(left.Union(right))] = true
	})
	if len(sizes) != 2 {
		t.Errorf("distinct and union sizes depend on the input order: %v", sizes)
	}
}

// permutations calls visit with a sample of the orderings of n items: every
// rotation of the identity and of its reversal, and a spread of swaps, which is
// enough to put each value both before and after every other.
func permutations(n int, visit func([]int)) {
	base := make([]int, n)
	for i := range base {
		base[i] = i
	}
	seen := map[string]bool{}
	emit := func(order []int) {
		key := ""
		for _, i := range order {
			key += string(rune('a' + i))
		}
		if !seen[key] {
			seen[key] = true
			visit(append([]int(nil), order...))
		}
	}
	for r := 0; r < n; r++ {
		forward := make([]int, n)
		backward := make([]int, n)
		for i := 0; i < n; i++ {
			forward[i] = (i + r) % n
			backward[i] = (n - 1 - i + r) % n
		}
		emit(forward)
		emit(backward)
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			swapped := append([]int(nil), base...)
			swapped[i], swapped[j] = swapped[j], swapped[i]
			emit(swapped)
		}
	}
}

func equalsItself(v Value) bool {
	copied := v
	return v.Equal(copied)
}
