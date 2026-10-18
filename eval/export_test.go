package eval

import "time"

// SetNowSource replaces the source evaluations read the current time from,
// for the tests, and returns what restores it.
func SetNowSource(source func() time.Time) (restore func()) {
	previous := nowSource
	nowSource = source
	return func() { nowSource = previous }
}
