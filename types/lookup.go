package types

// Lookup holds a collection to search for items equal to others, as union,
// distinct(), intersect(), exclude(), subsetOf() and repeat() do for every item
// they are given.
//
// Each search compares the item with every one held, and nearly every pair of
// objects differs. Once it holds enough items to repay it, a Lookup
// fingerprints each object once, by its content apart from layout, and
// compares two objects only when their fingerprints match: two objects that are
// equal always share one. See jsonFingerprint.
type Lookup struct {
	items  Collection
	prints []uint64 // each object's fingerprint once fingerprinted, nil until then
}

// lookupFingerprintFrom is the number of items from which a Lookup
// fingerprints its objects. Below it the pairs are too few to repay a pass
// over every object; most collections an expression compares hold a handful.
const lookupFingerprintFrom = 8

// NewLookup returns a Lookup holding the items of a collection.
func NewLookup(items Collection) *Lookup {
	l := newLookup(len(items))
	for _, item := range items {
		l.Add(item)
	}
	return l
}

// newLookup returns an empty Lookup with room for capacity items.
func newLookup(capacity int) *Lookup {
	return &Lookup{items: make(Collection, 0, capacity)}
}

// Items returns the items held, in the order added.
func (l *Lookup) Items() Collection {
	return l.items
}

// Add holds one more item.
func (l *Lookup) Add(item Value) {
	l.items = append(l.items, item)
	switch {
	case l.prints != nil:
		l.prints = append(l.prints, fingerprintOf(item))
	case len(l.items) >= lookupFingerprintFrom:
		l.prints = make([]uint64, len(l.items), cap(l.items))
		for i, held := range l.items {
			l.prints[i] = fingerprintOf(held)
		}
	}
}

// Contains reports whether an item equal to v is held.
func (l *Lookup) Contains(v Value) bool {
	object, isObject := v.(*ObjectValue)
	if l.prints == nil || !isObject {
		return l.items.Contains(v)
	}
	mark := jsonFingerprint(object.data)
	for i, held := range l.items {
		if _, heldObject := held.(*ObjectValue); heldObject && l.prints[i] != mark {
			continue
		}
		if held.Equal(v) {
			return true
		}
	}
	return false
}

// fingerprintOf is an object's fingerprint, and zero for any other value,
// which Contains never reads.
func fingerprintOf(v Value) uint64 {
	if object, ok := v.(*ObjectValue); ok {
		return jsonFingerprint(object.data)
	}
	return 0
}

// fingerprintTable gives each byte a fixed 64-bit value, spread by splitmix64,
// so that a sum of them tells apart texts that hold different bytes.
var fingerprintTable = func() (table [256]uint64) {
	x := uint64(0x9e3779b97f4a7c15)
	for i := range table {
		x += 0x9e3779b97f4a7c15
		z := x
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		table[i] = z ^ (z >> 31)
	}
	return table
}()

// jsonFingerprint sums the values of a JSON text's bytes, leaving out the
// whitespace between tokens. Texts that sameJSON finds the same hold the same
// bytes once that whitespace is gone, so they always share a fingerprint, and
// texts that do not share one differ. Two that share one may still differ,
// which only means comparing them.
func jsonFingerprint(data []byte) uint64 {
	// Compact text has no layout to leave out: one tight pass, and the
	// careful one only from the first byte that could be layout
	var sum uint64
	for i, c := range data {
		if c <= ' ' {
			return sum + layoutFingerprint(data[i:], data[:i])
		}
		sum += fingerprintTable[c]
	}
	return sum
}

// layoutFingerprint sums the rest of a text from a byte that could be layout,
// knowing from what came before whether it stands inside a string.
func layoutFingerprint(rest, before []byte) uint64 {
	inString, escaped := false, false
	for _, c := range before {
		switch {
		case escaped:
			escaped = false
		case inString && c == '\\':
			escaped = true
		case c == '"':
			inString = !inString
		}
	}
	var sum uint64
	for _, c := range rest {
		switch {
		case escaped:
			escaped = false
		case inString && c == '\\':
			escaped = true
		case c == '"':
			inString = !inString
		case !inString && isJSONSpace(c):
			continue
		}
		sum += fingerprintTable[c]
	}
	return sum
}
