package types

// Lookup holds a collection to search for items equal to others, as union,
// distinct(), intersect(), exclude(), subsetOf() and repeat() do for every item
// they are given.
//
// Each search compares the item with every one held, and nearly every pair of
// objects differs. Comparing bytes rejected objects of different lengths at
// once; layout makes length say nothing, so once a Lookup has been searched
// often enough to repay it, it measures each object's content once — its
// length and a sum, apart from whitespace — and compares two objects only when
// those agree. See measureContent.
type Lookup struct {
	items    Collection
	prints   []contentPrint // each item's, once measured; nil until then
	objects  bool           // whether any item held is an object
	searches int
	// The last value searched for and its print, which Add reuses when it
	// holds what was just found absent
	last      Value
	lastPrint contentPrint
}

// contentPrint is what a Lookup knows of an object's content: its length and
// sum apart from whitespace. A value that is not an object has none.
type contentPrint struct {
	sum    uint64
	length int32
	object bool
}

// printOf measures v's content, if v is an object.
func printOf(v Value) contentPrint {
	if object, ok := v.(*ObjectValue); ok {
		length, sum := measureContent(object.data)
		//nolint:gosec // a length; no object read into memory nears 2^31 bytes
		return contentPrint{sum: sum, length: int32(length), object: true}
	}
	return contentPrint{}
}

// lookupMeasureFrom is the number of items held, and of searches made, from
// which a Lookup measures its objects. Below either the pairs are too few to
// repay a pass over every object: most collections an expression compares
// hold a handful, and exclude() of one item searches once.
const lookupMeasureFrom = 8

// NewLookup returns a Lookup holding the items of a collection. It reads the
// collection in place; Add copies it first.
func NewLookup(items Collection) *Lookup {
	l := &Lookup{items: items[:len(items):len(items)]}
	for _, item := range items {
		if _, ok := item.(*ObjectValue); ok {
			l.objects = true
			break
		}
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
	_, isObject := item.(*ObjectValue)
	l.objects = l.objects || isObject
	l.items = append(l.items, item)
	if l.prints == nil {
		return
	}
	if item == l.last {
		l.prints = append(l.prints, l.lastPrint)
	} else {
		l.prints = append(l.prints, printOf(item))
	}
}

// Contains reports whether an item equal to v is held. Objects whose content
// differs in length or sum are not compared.
func (l *Lookup) Contains(v Value) bool {
	l.searches++
	_, isObject := v.(*ObjectValue)
	if !isObject || !l.objects || !l.measured() {
		return l.items.Contains(v)
	}
	probe := printOf(v)
	found := false
	for i, held := range l.items {
		if p := l.prints[i]; p.object && (p.length != probe.length || p.sum != probe.sum) {
			continue
		}
		if held.Equal(v) {
			found = true
			break
		}
	}
	l.last, l.lastPrint = v, probe
	return found
}

// measured reports whether the objects held are measured, measuring them once
// the Lookup holds enough items and has been searched often enough.
func (l *Lookup) measured() bool {
	if l.prints != nil {
		return true
	}
	if len(l.items) < lookupMeasureFrom || l.searches < lookupMeasureFrom {
		return false
	}
	l.prints = make([]contentPrint, len(l.items), cap(l.items))
	for i, held := range l.items {
		l.prints[i] = printOf(held)
	}
	return true
}
