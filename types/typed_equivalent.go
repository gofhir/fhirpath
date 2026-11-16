package types

import (
	"bytes"
	"strings"

	"github.com/buger/jsonparser"
)

// EquivalentIn compares two objects as ~ does when a model types them: each
// child is read as the type the model gives it, as navigation reads it, and
// compared with the equivalence of that type, recursively. N1 §6.1.3: "For
// complex types, equivalence requires all child properties to be equivalent,
// recursively."
//
// Each side is typed on its own, by what navigation reads: the object's type
// and the path it was reached by, leftPath and rightPath. A backbone element
// types its children by that path first, any other object by its type first
// and then by the path; see typeField. Two values of different types compare
// as ~ compares those values, so the same text under two types may not be
// equivalent. A child whose type the model does not know is read by its
// shape, as ObjectValue.Equivalent reads every child, and a child that reads
// as nothing under its type, such as a number outside the decimal grammar, is
// absent on its side, as navigation leaves it out.
//
// An answer that skips reading, bytes written alike, stands only where both
// sides have the same typing state, the same type and the same path; with
// another state the pair is read typed. The comparison is lazy: a field is
// typed only when it is read, once per comparison however many repeats it is
// read for, and no value is built for a child that is paired unread. Nested
// Quantities are told by their shape, as ObjectValue.Equivalent tells them.
//
// With no resolver it is ObjectValue.Equivalent, the reading without a model.
// Like it, it reads the two objects and writes to neither.
func EquivalentIn(left, right *ObjectValue, leftPath, rightPath string, res ElementTypeResolver) bool {
	if res == nil {
		return left.Equivalent(right)
	}
	var e Equivalence
	e.pair.res = res
	return e.Equivalent(left, right, leftPath, rightPath)
}

// Equivalence is EquivalentIn under one resolver for many comparisons, which
// keeps what it works out of how the fields of each state are typed from one
// comparison to the next: a `~` between collections compares each of their
// items with each, and types each field of a state once, not once per pair.
// The typing it keeps is the resolver's answer for a type, the path it was
// reached by and a field's name, so it is the same for any pair of objects.
// It is for one evaluation, as a Context is: not for use by several
// goroutines at once, and not past a change of the resolver's model.
type Equivalence struct {
	pair typedPair
}

// NewEquivalence returns an Equivalence under a resolver. It must not be nil.
func NewEquivalence(res ElementTypeResolver) *Equivalence {
	e := &Equivalence{}
	e.pair.res = res
	return e
}

// Equivalent is EquivalentIn under the Equivalence's resolver.
func (e *Equivalence) Equivalent(left, right *ObjectValue, leftPath, rightPath string) bool {
	leftType, rightType := left.Type(), right.Type()
	if leftType == rightType && leftPath == rightPath && sameJSON(left.data, right.data) {
		return true
	}
	if lq, ok := left.typedQuantity(); ok {
		if rq, ok := right.typedQuantity(); ok {
			return lq.Equivalent(rq)
		}
	}
	p := &e.pair
	l, ok := p.newSide(left.data, leftType, leftPath)
	if !ok {
		return false
	}
	r, ok := p.newSide(right.data, rightType, rightPath)
	if !ok {
		return false
	}
	return p.objects(&l, &r)
}

// typedPair is one comparison of two objects under a resolver, with the
// typing of the fields of every state it has met.
//
// The states of the first few types are kept in the comparison itself, which
// stays on the stack: most comparisons meet no more.
type typedPair struct {
	res    ElementTypeResolver
	states []*typingState
	index  map[[2]string]*typingState // the states once there are many
	ptrs   [4]*typingState
	buf    [4]typingState
}

// typingState is what types the fields of an object: its type and the path it
// was reached by, and nothing else about it. The typing of each field is
// worked out the first time any object in that state is read for it, and kept
// for every other, so a field is typed once per comparison, never by path
// alone.
type typingState struct {
	typ, path string
	fields    []namedTyping
	index     map[string]int // the fields once there are many
	kept      [3]namedTyping
}

// namedTyping is the typing of a field, by the field's name.
type namedTyping struct {
	name   string
	typing fieldTyping
}

// typedSide is one object of a typed comparison, its children read once, and
// the state it is typed under.
type typedSide struct {
	object jsonObject
	state  *typingState
}

// newSide reads an object's children as written, as an object of a type that
// was reached by a path.
func (p *typedPair) newSide(data []byte, typ, path string) (typedSide, bool) {
	object, ok := parseJSONObject(data)
	if !ok {
		return typedSide{}, false
	}
	return typedSide{object: object, state: p.stateOf(typ, path)}, true
}

// manyStates and manyFields are how many states a comparison, and fields a
// state, have before they are found by an index rather than one by one.
const (
	manyStates = 8
	manyFields = 12
)

// stateOf is the state of the objects of a type reached by a path, made the
// first time it is asked for.
func (p *typedPair) stateOf(typ, path string) *typingState {
	if p.index != nil {
		if state := p.index[[2]string{typ, path}]; state != nil {
			return state
		}
	} else {
		for _, state := range p.states {
			if state.typ == typ && state.path == path {
				return state
			}
		}
	}
	if p.states == nil {
		p.states = p.ptrs[:0]
	}
	var state *typingState
	if n := len(p.states); n < len(p.buf) {
		state = &p.buf[n]
	} else {
		state = new(typingState)
	}
	state.typ, state.path, state.fields = typ, path, state.kept[:0]
	p.states = append(p.states, state)
	switch {
	case p.index != nil:
		p.index[[2]string{typ, path}] = state
	case len(p.states) > manyStates:
		p.index = make(map[[2]string]*typingState, 2*len(p.states))
		for _, s := range p.states {
			p.index[[2]string{s.typ, s.path}] = s
		}
	}
	return state
}

// typing is the typing of a field of a side, worked out the first time it is
// asked for in its state.
func (p *typedPair) typing(s *typedSide, name []byte) fieldTyping {
	state := s.state
	if state.index != nil {
		if i, ok := state.index[string(name)]; ok {
			return state.fields[i].typing
		}
	} else {
		for i := range state.fields {
			if state.fields[i].name == string(name) {
				return state.fields[i].typing
			}
		}
	}
	field := string(name)
	typing := typeFieldOf(state.typ, state.path, field, p.res)
	state.fields = append(state.fields, namedTyping{name: field, typing: typing})
	switch {
	case state.index != nil:
		state.index[field] = len(state.fields) - 1
	case len(state.fields) > manyFields:
		state.index = make(map[string]int, 2*len(state.fields))
		for i, f := range state.fields {
			state.index[f.name] = i
		}
	}
	return typing
}

// equalTo reports two fields typed alike, which reads their text alike.
func (t fieldTyping) equalTo(o fieldTyping) bool {
	return t.fhirType == o.fhirType && t.path == o.path
}

// findIndex is find giving the position of the child.
func (o *jsonObject) findIndex(key []byte, at int) int {
	if at < len(o.children) && bytes.Equal(o.children[at].key, key) {
		return at
	}
	if o.index != nil {
		if i, ok := o.index[string(key)]; ok {
			return i
		}
		return -1
	}
	for i := range o.children {
		if bytes.Equal(o.children[i].key, key) {
			return i
		}
	}
	return -1
}

// lookup finds the value and the element of a name.
func (o *jsonObject) lookup(name []byte) (value jsonChild, valueFound bool, element jsonChild, elementFound bool) {
	if _, isElement := elementOf(name); !isElement {
		if i := o.findIndex(name, len(o.children)); i >= 0 {
			value, valueFound = o.children[i], true
		}
	}
	if len(name) > 0 {
		var buf [64]byte
		if i := o.findIndex(append(append(buf[:0], '_'), name...), len(o.children)); i >= 0 {
			element, elementFound = o.children[i], true
		}
	}
	return value, valueFound, element, elementFound
}

// alikeChild reports a child written alike on both sides, or on neither.
func alikeChild(l jsonChild, lok bool, r jsonChild, rok bool) bool {
	if lok != rok {
		return false
	}
	return !lok || writtenAlike(l.data, l.dataType, r.data, r.dataType)
}

// objects compares two sides' children, name by name, each name either side
// writes, once. Where both sides have the same state, a child written alike on
// both is equivalent unread, and its field is not typed.
func (p *typedPair) objects(l, r *typedSide) bool {
	same := l.state == r.state
	if l.object.hasElements() || r.object.hasElements() {
		return p.everyName(l, r, same)
	}
	for i := range l.object.children {
		lc := l.object.children[i]
		j := r.object.findIndex(lc.key, i)
		if j < 0 {
			if !p.named(l, r, lc.key, same) {
				return false
			}
			continue
		}
		rc := r.object.children[j]
		if same && writtenAlike(lc.data, lc.dataType, rc.data, rc.dataType) {
			continue
		}
		lt, rt := p.typing(l, lc.key), p.typing(r, lc.key)
		if lt.choice || rt.choice {
			if !p.named(l, r, lc.key, same) {
				return false
			}
			continue
		}
		if !p.childPair(lc, lt, rc, rt, same) {
			return false
		}
	}
	for j := range r.object.children {
		key := r.object.children[j].key
		if l.object.findIndex(key, j) < 0 && !p.named(l, r, key, same) {
			return false
		}
	}
	return true
}

// everyName is the comparison of objects that write an element beside a
// primitive: the names of both sides, each once, as everyName reads them.
func (p *typedPair) everyName(l, r *typedSide, same bool) bool {
	for _, side := range [2]*typedSide{l, r} {
		for _, child := range side.object.children {
			name, isElement := elementOf(child.key)
			if !isElement {
				name = child.key
			} else if _, found := side.object.valueOf(name); found {
				continue // compared with its primitive
			}
			if side == r && l.object.holds(name) {
				continue // compared above
			}
			if !p.named(l, r, name, same) {
				return false
			}
		}
	}
	return true
}

// named compares the child a name stands for on each side, as equivalentNamed
// does, each side under its own typing of the name. A name that is the bare
// name of a choice is no field on its side.
func (p *typedPair) named(l, r *typedSide, name []byte, same bool) bool {
	lv, lok, le, leok := l.object.lookup(name)
	rv, rok, re, reok := r.object.lookup(name)
	if same && alikeChild(lv, lok, rv, rok) && alikeChild(le, leok, re, reok) {
		return true
	}
	var lt, rt fieldTyping
	if lok || leok {
		if lt = p.typing(l, name); lt.choice {
			lok, leok = false, false
		}
	}
	if rok || reok {
		if rt = p.typing(r, name); rt.choice {
			rok, reok = false, false
		}
	}
	if !leok && !reok && lok && rok {
		return p.childPair(lv, lt, rv, rt, same)
	}
	li, ri := p.items(lv, lok, le, leok, lt), p.items(rv, rok, re, reok, rt)
	return len(li) == len(ri) && p.pairItems(li, ri, lt, rt, same)
}

// childPair compares a child written on both sides as a pair of collections,
// a single value being a collection of one.
//
// Where both sides have the same state the caller has told whether they are
// written alike; where only the typing of this field is alike, it is told here.
func (p *typedPair) childPair(lv jsonChild, lt fieldTyping, rv jsonChild, rt fieldTyping, same bool) bool {
	if !same && lt.equalTo(rt) && writtenAlike(lv.data, lv.dataType, rv.data, rv.dataType) {
		return true
	}
	if lv.dataType != jsonparser.Array && rv.dataType != jsonparser.Array {
		l := typedVal{data: lv.data, dt: lv.dataType, typing: lt}
		r := typedVal{data: rv.data, dt: rv.dataType, typing: rt}
		return p.valuesEq(&l, &r)
	}
	l, ok := typedChildValues(lv, lt)
	if !ok {
		return false
	}
	r, ok := typedChildValues(rv, rt)
	return ok && len(l) == len(r) && p.pairValues(l, r, lt, rt, same)
}

// typedChildValues is childValues leaving out the values that read as nothing
// under the field's type, which navigation leaves out: a nested array, a
// number outside the decimal grammar. A field of no known type leaves none out.
func typedChildValues(child jsonChild, typing fieldTyping) ([]childValue, bool) {
	values, ok := childValues(child)
	if !ok || typing.fhirType == "" {
		return values, ok
	}
	kept := values[:0]
	for _, v := range values {
		if v.dataType == jsonparser.Array || v.dataType == jsonparser.Number && !numberReads(v.data) {
			continue
		}
		kept = append(kept, v)
	}
	return kept, true
}

// numberReads reports a number token that reads as a value, as jsonValue
// reads it.
func numberReads(data []byte) bool {
	if !jsonNumber(data) {
		return false
	}
	if _, err := jsonparser.ParseInt(data); err == nil {
		return true
	}
	_, err := ReadDecimal(string(data))
	return err == nil
}

// pairValues reports whether each value on the left pairs with an unused
// equivalent one on the right, as pairInAnyOrder pairs them. Values written
// alike pair first, unread, where both sides are typed alike; any other pair
// is read typed.
func (p *typedPair) pairValues(l, r []childValue, lt, rt fieldTyping, same bool) bool {
	used := make([]bool, len(r))
	paired := make([]bool, len(l))
	if same || lt.equalTo(rt) {
		pairAlike(l, r, used, paired)
	}
	// Objects met again and again are read once each
	var leftSides, rightSides []sideSlot
	for i, item := range l {
		if paired[i] {
			continue
		}
		if leftSides == nil {
			leftSides, rightSides = make([]sideSlot, len(l)), make([]sideSlot, len(r))
		}
		matched := false
		for j, candidate := range r {
			if used[j] {
				continue
			}
			lv := typedVal{data: item.data, dt: item.dataType, typing: lt}
			rv := typedVal{data: candidate.data, dt: candidate.dataType, typing: rt}
			var equivalent bool
			if lv.dt == jsonparser.Object && rv.dt == jsonparser.Object {
				equivalent = p.objectsEq(&lv, &rv, &leftSides[i], &rightSides[j], false)
			} else {
				equivalent = p.valuesEq(&lv, &rv)
			}
			if equivalent {
				used[j], matched = true, true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// pairAlike pairs the values written alike, unread.
func pairAlike(l, r []childValue, used, paired []bool) {
	for i, item := range l {
		for j, candidate := range r {
			if !used[j] && writtenAlike(item.data, item.dataType, candidate.data, candidate.dataType) {
				used[j], paired[i] = true, true
				break
			}
		}
	}
}

// typedItem is a primitive's value and the element beside it at one position,
// as pairedItem; the type of the element, which is the primitive's, is told
// from the value.
type typedItem struct {
	value, element       childValue
	hasValue, hasElement bool
	elementType          string
}

// items lists the items of a name on one side, as nameItems does, leaving out
// the ones whose value reads as nothing, as navigation drops the item.
func (p *typedPair) items(value jsonChild, valueFound bool, element jsonChild, elementFound bool, typing fieldTyping) []typedItem {
	paired := nameItems(value, valueFound, element, elementFound)
	items := make([]typedItem, 0, len(paired))
	for _, item := range paired {
		var entry typedItem
		var read *typedVal
		if item.value != nil {
			entry.value, entry.hasValue = *item.value, true
			read = &typedVal{data: item.value.data, dt: item.value.dataType, typing: typing}
			if read.absent() {
				continue
			}
		}
		if item.element != nil {
			entry.element, entry.hasElement = *item.element, true
			entry.elementType = elementTypeOf(read, typing)
		}
		items = append(items, entry)
	}
	return items
}

// elementTypeOf is the type navigation gives the element beside a primitive,
// which has none of its own: the type of the primitive read from the value,
// or, where there is no value, the primitive type the field declares.
func elementTypeOf(value *typedVal, typing fieldTyping) string {
	if typing.fhirType == "" {
		return ""
	}
	if value == nil {
		return primitiveTypeOf(typing.fhirType)
	}
	if value.dt == jsonparser.Object {
		return ""
	}
	// A FHIR primitive's value is read as its own type, which needs no reading
	// to tell
	if !strings.HasPrefix(typing.fhirType, systemTypePrefix) {
		if name := primitiveTypeOf(typing.fhirType); name != "" {
			return name
		}
	}
	parsed := value.reading()
	if parsed == nil {
		return ""
	}
	if _, primitive := SystemValue(parsed); !primitive {
		return ""
	}
	name := parsed.Type()
	if IsSystemTypeName(name) {
		name = lowerFirst(name)
	}
	return name
}

// pairItems reports whether each item on the left pairs with an unused
// equivalent one on the right, as pairItems pairs them.
func (p *typedPair) pairItems(l, r []typedItem, lt, rt fieldTyping, same bool) bool {
	used := make([]bool, len(r))
	paired := make([]bool, len(l))
	if same || lt.equalTo(rt) {
		for i := range l {
			for j := range r {
				if !used[j] && itemsAlike(&l[i], &r[j]) {
					used[j], paired[i] = true, true
					break
				}
			}
		}
	}
	for i := range l {
		if paired[i] {
			continue
		}
		matched := false
		for j := range r {
			if !used[j] && p.itemsEq(&l[i], &r[j], lt, rt, same) {
				used[j], matched = true, true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

// itemsAlike reports two items written alike, value and element each.
func itemsAlike(l, r *typedItem) bool {
	return l.hasValue == r.hasValue && l.hasElement == r.hasElement &&
		(!l.hasValue || writtenAlike(l.value.data, l.value.dataType, r.value.data, r.value.dataType)) &&
		(!l.hasElement || writtenAlike(l.element.data, l.element.dataType, r.element.data, r.element.dataType))
}

// itemsEq compares two items, value and element, each of which may be absent.
func (p *typedPair) itemsEq(l, r *typedItem, lt, rt fieldTyping, same bool) bool {
	if l.hasValue != r.hasValue || l.hasElement != r.hasElement {
		return false
	}
	if l.hasValue {
		lv := typedVal{data: l.value.data, dt: l.value.dataType, typing: lt}
		rv := typedVal{data: r.value.data, dt: r.value.dataType, typing: rt}
		alike := (same || lt.equalTo(rt)) && writtenAlike(l.value.data, l.value.dataType, r.value.data, r.value.dataType)
		if !alike && !p.valuesEq(&lv, &rv) {
			return false
		}
	}
	if l.hasElement {
		lv := typedVal{data: l.element.data, dt: l.element.dataType, typing: lt, element: true, elementType: l.elementType}
		rv := typedVal{data: r.element.data, dt: r.element.dataType, typing: rt, element: true, elementType: r.elementType}
		alike := (same || lt.equalTo(rt)) && writtenAlike(l.element.data, l.element.dataType, r.element.data, r.element.dataType)
		if !alike && !p.valuesEq(&lv, &rv) {
			return false
		}
	}
	return true
}

// typedVal is one value of a child as written, with the typing of the field
// it stands in, and what reading it gave, once it is read. An element beside a
// primitive is an object of the primitive's type, placed at the primitive's
// path.
type typedVal struct {
	data        []byte
	dt          jsonparser.ValueType
	typing      fieldTyping
	element     bool
	elementType string
	value       Value
	read        bool
}

// reading is the value read under the field's type, as navigation reads it,
// or by its shape where the model gives none; nil where it reads as nothing.
func (v *typedVal) reading() Value {
	if !v.read {
		v.read = true
		if v.typing.fhirType != "" {
			v.value = jsonValueToFHIRValueWithType(v.data, v.dt, v.typing.fhirType)
		} else {
			v.value = jsonValue(v.data, v.dt, false)
		}
	}
	return v.value
}

// absent reports a value that reads as nothing under a type the model gives:
// a nested array, a number outside what a decimal reads. Navigation leaves it
// out. Where the model gives no type the value is read by its shape, and one
// that reads as nothing is equivalent to nothing.
func (v *typedVal) absent() bool {
	if v.typing.fhirType == "" {
		return false
	}
	switch v.dt {
	case jsonparser.Array:
		return true
	case jsonparser.Number:
		return v.reading() == nil
	}
	return false
}

// plainString reports a JSON string that reads as a String, whose equivalence
// is ASCII's: any text under a type that is no temporal type and no integer64,
// and under no type one that is no date by its shape.
func (v *typedVal) plainString() bool {
	t := v.typing.fhirType
	if t == "" {
		return !looksTemporalBytes(v.data)
	}
	t = strings.TrimPrefix(t, systemTypePrefix)
	switch strings.ToLower(t) {
	case "date", "datetime", fhirInstant, "time", fhirInteger64:
		return false
	}
	return true
}

// valuesEq compares two values not written alike: absent ones only to each
// other, an object with an object as objectsEq has it, and anything else read
// under its type. Two plain strings in ASCII compare as written, which is what
// reading them would compare.
func (p *typedPair) valuesEq(l, r *typedVal) bool {
	la, ra := l.absent(), r.absent()
	if la || ra {
		return la && ra
	}
	if l.dt == jsonparser.Object || r.dt == jsonparser.Object {
		var ls, rs sideSlot
		return l.dt == r.dt && p.objectsEq(l, r, &ls, &rs, true)
	}
	if l.dt == jsonparser.String && r.dt == jsonparser.String && l.plainString() && r.plainString() {
		if equivalent, decided := equivalentASCII(l.data, r.data); decided {
			return equivalent
		}
	}
	lv, rv := l.reading(), r.reading()
	return lv != nil && rv != nil && lv.Equivalent(rv)
}

// objectsEq compares two objects the model types, each as the type its field
// declares, an abstract resource as its resourceType says, placed at the
// field's path. The sides read are kept in ls and rs, so that a repeat met
// again is read once. A pair where either field has no type is read by its
// shape, as ObjectValue.Equivalent reads it. Two Quantities are told by their
// shape, never by the type the model gives: a nested one stays as it is told
// without a model.
func (p *typedPair) objectsEq(l, r *typedVal, ls, rs *sideSlot, quantity bool) bool {
	if l.typing.fhirType == "" || r.typing.fhirType == "" {
		return p.unknownEq(l, r, ls, rs, quantity)
	}
	lk, lok := l.stateKey()
	rk, rok := r.stateKey()
	if lok && rok && lk == rk && sameJSON(l.data, r.data) {
		return true
	}
	if quantity {
		if equivalent, told := shapedQuantities(l.data, r.data); told {
			return equivalent
		}
	}
	left, ok := ls.of(p, l)
	if !ok {
		return false
	}
	right, ok := rs.of(p, r)
	if !ok {
		return false
	}
	if (!lok || !rok) && left.state == right.state && sameJSON(l.data, r.data) {
		return true
	}
	return p.objects(left, right)
}

// unknownEq compares two objects where the model types at most one field. Two
// untyped ones are read by their shape, as ObjectValue.Equivalent reads them.
// Where only one is typed, each side keeps its own typing: the known one under its type and path,
// the unknown one under none, so that its children are read by their shape. The
// states differ, so no shortcut that skips reading stands between them.
func (p *typedPair) unknownEq(l, r *typedVal, ls, rs *sideSlot, quantity bool) bool {
	if l.typing.fhirType == "" && r.typing.fhirType == "" {
		return NewObjectValue(l.data).Equivalent(NewObjectValue(r.data))
	}
	// Dos Quantities se reconocen por su forma, como sin modelo (plan §2, C2)
	if quantity {
		if equivalent, told := shapedQuantities(l.data, r.data); told {
			return equivalent
		}
	}
	for _, v := range [2]*typedVal{l, r} {
		slot := ls
		if v == r {
			slot = rs
		}
		if slot.built {
			continue
		}
		slot.built = true
		if v.typing.fhirType == "" {
			slot.side, slot.ok = p.newSide(v.data, "", "")
		} else {
			slot.side, slot.ok = p.newSide(v.data, v.stateType(), v.path())
		}
	}
	if !ls.ok || !rs.ok {
		return false
	}
	return p.objects(&ls.side, &rs.side)
}

// shapedQuantities compares two objects that are Quantities by their shape, as
// ObjectValue.Equivalent tells them, whatever type the model gives them.
func shapedQuantities(l, r []byte) (equivalent, told bool) {
	if !bytes.Contains(l, []byte(`"value"`)) || !bytes.Contains(r, []byte(`"value"`)) {
		return false, false
	}
	lq, ok := NewObjectValue(l).typedQuantity()
	if !ok {
		return false, false
	}
	rq, ok := NewObjectValue(r).typedQuantity()
	if !ok {
		return false, false
	}
	return lq.Equivalent(rq), true
}

// stateKey is the typing state an object will have, its type and path, where
// the field says so: not where the type is an abstract resource's, told by the
// object's resourceType, or a System type's, or an element's type that is
// left to its shape.
func (v *typedVal) stateKey() (key [2]string, ok bool) {
	t := v.typing.fhirType
	if v.element {
		t = v.elementType
	}
	if t == "" || IsAbstractResourceType(t) || strings.HasPrefix(t, systemTypePrefix) {
		return key, false
	}
	return [2]string{t, v.path()}, true
}

// path is the path the object is placed at: the field's, except for a
// resource, which resolves its fields beneath its own type.
func (v *typedVal) path() string {
	if v.typing.placed() {
		return v.typing.path
	}
	return ""
}

// stateType is the type navigation gives the object a field holds: the one the
// model declares, an abstract resource type told by its resourceType, and the
// type of the primitive for the element beside one.
func (v *typedVal) stateType() string {
	switch t := v.typing.fhirType; {
	case v.element:
		if v.elementType != "" {
			return v.elementType
		}
		return NewObjectValue(v.data).Type()
	case IsAbstractResourceType(t):
		if resource := asResource(v.data); resource != nil {
			return resource.typeName
		}
		return t
	case strings.HasPrefix(t, systemTypePrefix):
		return NewObjectValue(v.data).Type()
	default:
		return t
	}
}

// sideSlot keeps the side an object was read as, the first time it is asked
// for.
type sideSlot struct {
	side  typedSide
	ok    bool
	built bool
}

// of reads the object a typed value is under the state its field gives it.
func (s *sideSlot) of(p *typedPair, v *typedVal) (*typedSide, bool) {
	if !s.built {
		s.built = true
		s.side, s.ok = p.newSide(v.data, v.stateType(), v.path())
	}
	return &s.side, s.ok
}
