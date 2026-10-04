package types

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/shopspring/decimal"
)

// ObjectValue represents a FHIR resource or complex type as a JSON object.
type ObjectValue struct {
	data []byte
	// Cache of accessed fields, created when the first field is read. Most
	// objects an expression walks past are never read through Get, and a
	// navigation over a Bundle creates one ObjectValue per entry, so the map
	// is worth its own allocation only for the ones that are.
	fields map[string]Value
	// Cache of fields read as collections, which is how navigation reads them.
	// Kept only when caching is on. See cachedCollection.
	collections map[fieldKey]Collection
	// caching is off by default: an object read for a single evaluation is
	// discarded with it, so keeping what it read would cost memory that nothing
	// goes on to use. It is turned on for an object that outlives one
	// evaluation, which is what a Document is for.
	caching bool
	// private records that one goroutine reads the object, so it can keep
	// what it works out about itself. See MarkPrivate.
	private      bool
	reads        uint8          // how many times a field was read by scanning. See readField.
	index        []indexedField // the object's fields once indexed. See readField.
	explicitType string         // optional explicit FHIR type from polymorphic resolution
	elementPath  string         // the element the object was reached as. See ElementPath.
	typeName     string         // the type once answered, which does not change. See Type.
}

// EnableCaching makes the object keep the fields it reads, and the objects it
// reads them into keep theirs.
//
// This is for a resource that several expressions are evaluated against, where
// what one expression navigates is worth keeping for the next. It trades memory
// for that, and an object that caches must not be read from two goroutines at
// once.
func (o *ObjectValue) EnableCaching() {
	o.caching = true
}

// ElementPath is the path of the element the object was reached as —
// "Claim.diagnosis" for an entry of a Claim's diagnosis — which is what a model
// resolves the object's own fields beneath. It is "" for an object a model did
// not place, and a resource needs none: its type is its path.
//
// It belongs to the object rather than to the evaluation because it is a fact
// about where the object sits in the resource, which nothing evaluated before
// or after it changes.
func (o *ObjectValue) ElementPath() string {
	return o.elementPath
}

// SetElementPath records the path of the element the object was reached as.
func (o *ObjectValue) SetElementPath(path string) {
	o.elementPath = path
}

// MarkPrivate records that only one goroutine reads the object, which lets it
// keep what it works out about itself — its type, the fields read through Get
// — instead of working them out again each time.
//
// An object that is not private is only ever read, so a root a caller builds
// can be evaluated against from several goroutines at once. An object that
// caches is private as well. Objects read out of a private or caching one are
// private in turn: each is created by the read, not shared.
func (o *ObjectValue) MarkPrivate() {
	o.private = true
}

// MarkShared undoes MarkPrivate: the object may be read from several goroutines
// from now on, so it no longer writes what it works out to itself. What it
// wrote while private stays, and is only read.
//
// An evaluation marks what it returns shared: the objects it read privately
// are handed to a caller who may share them. An object that caches stays
// single-goroutine, as caching documents.
func (o *ObjectValue) MarkShared() {
	// An object already shared is only read: other goroutines may be reading
	// it now, so it is not written even to the value it already has.
	if o.private {
		o.private = false
	}
}

// keeps reports whether the object may write what it works out to itself.
func (o *ObjectValue) keeps() bool {
	return o.private || o.caching
}

// NewObjectValue creates a new ObjectValue from JSON bytes.
func NewObjectValue(data []byte) *ObjectValue {
	return &ObjectValue{
		data: data,
	}
}

// NewObjectValueWithType creates a new ObjectValue with an explicit FHIR type.
// Used when the type is known from polymorphic field resolution (e.g., valueQuantity → "Quantity").
func NewObjectValueWithType(data []byte, typeName string) *ObjectValue {
	return &ObjectValue{
		data:         data,
		explicitType: typeName,
	}
}

// FHIR type constants for type inference.
const (
	typeQuantity        = "Quantity"
	typeCoding          = "Coding"
	typeCodeableConcept = "CodeableConcept"
	typeReference       = "Reference"
	typePeriod          = "Period"
	typeIdentifier      = "Identifier"
	typeRange           = "Range"
	typeRatio           = "Ratio"
	typeAttachment      = "Attachment"
	typeHumanName       = "HumanName"
	typeAddress         = "Address"
	typeContactPoint    = "ContactPoint"
	typeAnnotation      = "Annotation"
	typeObject          = "Object"
)

// Type returns the FHIR type of this object.
// Checks explicit type (from polymorphic resolution), then resourceType, then infers from structure.
//
// The answer is kept: an object's data does not change, and navigation asks for
// the type of every object it passes — for an inferred type that means the
// structural checks below, each of which scans the object.
func (o *ObjectValue) Type() string {
	// First, check for explicit type set during polymorphic resolution
	if o.explicitType != "" {
		return o.explicitType
	}

	if o.typeName != "" {
		return o.typeName
	}

	// Kept only where no other goroutine reads the object; a shared one works
	// it out each time rather than write to it.
	typeName := o.readType()
	if o.keeps() {
		o.typeName = typeName
	}
	return typeName
}

// readType reads the object once and answers what it is.
//
// A resource says so in a resourceType field, and anything else is inferred
// from the fields it carries. Both were read separately, which meant scanning
// the object twice for everything that is not a resource — and a navigation
// passes far more elements than resources.
func (o *ObjectValue) readType() string {
	summary, resourceType := o.fieldSummary()
	if resourceType != "" {
		return resourceType
	}
	return summary.inferType()
}

// fields is which of the fields that distinguish a complex type the object
// carries, and of what kind. The set is small and fixed, so membership is a bit
// in a word rather than a map an inference would have to allocate.
type fields struct {
	present    uint32
	valueKind  jsonparser.ValueType
	givenKind  jsonparser.ValueType
	codingKind jsonparser.ValueType
}

// The fields the inference rules ask about.
const (
	fieldValue uint32 = 1 << iota
	fieldUnit
	fieldCode
	fieldSystem
	fieldCoding
	fieldReference
	fieldStart
	fieldEnd
	fieldLow
	fieldHigh
	fieldNumerator
	fieldDenominator
	fieldContentType
	fieldFamily
	fieldGiven
	fieldCity
	fieldPostalCode
	fieldUse
	fieldText
	fieldTime
	fieldAuthorReference
	fieldAuthorString
)

var inferenceFields = map[string]uint32{
	"value": fieldValue, "unit": fieldUnit, "code": fieldCode,
	"system": fieldSystem, "coding": fieldCoding, "reference": fieldReference,
	"start": fieldStart, "end": fieldEnd, "low": fieldLow, "high": fieldHigh,
	"numerator": fieldNumerator, "denominator": fieldDenominator,
	"contentType": fieldContentType, "family": fieldFamily, "given": fieldGiven,
	"city": fieldCity, "postalCode": fieldPostalCode, "use": fieldUse,
	"text": fieldText, "time": fieldTime,
	"authorReference": fieldAuthorReference, "authorString": fieldAuthorString,
}

func (f fields) has(field uint32) bool { return f.present&field != 0 }

// inferType attempts to infer the FHIR type from the object's structure.
//
// The shape of a complex type is decided by which fields it carries and of what
// kind, so the object is read once into a summary of that and the rules below
// consult the summary. Asking the object one field at a time meant a scan per
// question, a dozen of them per object, which navigation over a Bundle pays for
// every element it passes.
func (f fields) inferType() string {
	if t := f.quantityType(); t != "" {
		return t
	}
	if t := f.codingType(); t != "" {
		return t
	}
	if t := f.complexType(); t != "" {
		return t
	}
	return typeObject
}

// fieldSummary reads the object once, recording the fields the rules ask about.
func (o *ObjectValue) fieldSummary() (summary fields, resourceType string) {
	//nolint:errcheck // The only error is errFieldsFound, which ends the scan
	jsonparser.ObjectEach(o.data, func(key, entry []byte, entryType jsonparser.ValueType, _ int) error {
		// A resource names its type in a string. A resourceType that is null,
		// a number or an object names nothing, and neither does an empty one,
		// so the object is read on as though the field were not there — which
		// is what looking the name up separately did, since that lookup failed
		// on anything but a string.
		if string(key) == "resourceType" && entryType == jsonparser.String {
			if name := decodeJSONString(entry); name != "" {
				// Nothing else the object carries can change the answer now,
				// so the scan is over. FHIR writes the field first, which is
				// what the separate lookup took advantage of and what this
				// keeps.
				resourceType = name
				return errFieldsFound
			}
		}

		field, ok := inferenceFields[string(key)]
		if !ok {
			return nil
		}

		summary.present |= field
		switch field {
		case fieldValue:
			summary.valueKind = entryType
		case fieldGiven:
			summary.givenKind = entryType
		case fieldCoding:
			summary.codingKind = entryType
		}
		return nil
	})

	return summary, resourceType
}

// quantityType checks if the object is a Quantity type.
// The value must be numeric: an Identifier also carries "value" and "system",
// but its value is a string, and misreading it as a Quantity made
// identifier.ofType(Identifier) return nothing.
func (f fields) quantityType() string {
	if f.valueKind == jsonparser.Number {
		if f.has(fieldUnit) || f.has(fieldCode) || f.has(fieldSystem) {
			return typeQuantity
		}
	}
	return ""
}

// codingType checks if the object is a Coding type.
func (f fields) codingType() string {
	if f.has(fieldSystem) && f.has(fieldCode) && !f.has(fieldValue) {
		return typeCoding
	}
	return ""
}

// complexType checks for various FHIR complex types, in the order the rules
// have to be read: an object carrying the fields of two of them is the first
// one named here.
func (f fields) complexType() string {
	if t := f.codedOrReferencedType(); t != "" {
		return t
	}
	return f.namedPartyType()
}

// codedOrReferencedType covers the types built around a code, a reference or a
// pair of bounds.
func (f fields) codedOrReferencedType() string {
	switch {
	case f.codingKind == jsonparser.Array:
		return typeCodeableConcept
	case f.has(fieldReference):
		return typeReference
	case f.has(fieldStart) || f.has(fieldEnd):
		return typePeriod
	case f.has(fieldSystem) && f.valueKind == jsonparser.String:
		return typeIdentifier
	case f.has(fieldLow) || f.has(fieldHigh):
		return typeRange
	case f.has(fieldNumerator) || f.has(fieldDenominator):
		return typeRatio
	}
	return ""
}

// namedPartyType covers the types that describe a party or an attachment.
func (f fields) namedPartyType() string {
	switch {
	case f.has(fieldContentType):
		return typeAttachment
	case f.has(fieldFamily) || f.givenKind == jsonparser.Array:
		return typeHumanName
	case f.has(fieldCity) || f.has(fieldPostalCode):
		return typeAddress
	case f.has(fieldSystem) && f.has(fieldUse):
		return typeContactPoint
	case f.has(fieldText) && (f.has(fieldTime) || f.has(fieldAuthorReference) || f.has(fieldAuthorString)):
		return typeAnnotation
	}
	return ""
}

// Equal returns true if the JSON data is identical.
func (o *ObjectValue) Equal(other Value) bool {
	if ov, ok := other.(*ObjectValue); ok {
		return bytes.Equal(o.data, ov.data)
	}
	return false
}

// Equivalent is the same as Equal for objects.
func (o *ObjectValue) Equivalent(other Value) bool {
	return o.Equal(other)
}

// String returns the JSON representation.
func (o *ObjectValue) String() string {
	return string(o.data)
}

// IsEmpty returns false for object values.
func (o *ObjectValue) IsEmpty() bool {
	return false
}

// Data returns the raw JSON data.
func (o *ObjectValue) Data() []byte {
	return o.data
}

// Get retrieves a field value, caching the result.
func (o *ObjectValue) Get(field string) (Value, bool) {
	// Check cache first
	if v, ok := o.fields[field]; ok {
		return v, true
	}

	// Parse from JSON
	value, dataType, _, err := jsonparser.Get(o.data, field)
	if err != nil {
		return nil, false
	}

	// Convert to Value, and keep it where no other goroutine reads the object.
	// What it reads is new, so it is private either way.
	v := jsonValueToFHIRValue(value, dataType)
	markPrivate(Collection{v})
	if o.keeps() {
		if o.fields == nil {
			o.fields = make(map[string]Value, 4)
		}
		o.fields[field] = v
	}

	return v, true
}

// GetCollection retrieves a field as a Collection.
// If the field is an array, returns all elements.
// If the field is a single value, returns a singleton collection.
//
// When caching is on, the result is kept, so reading the same field again — a
// second expression over the same document, or a second mention of the field in
// one expression — costs a map lookup rather than another scan of the object.
// The objects read out of the field cache in turn, so the saving compounds down
// a path: the invariants of one resource share the whole of what they navigate.
//
// The collection handed back cannot grow into the cached one: it is capped at
// its length, so appending to it copies. Its values are immutable.
func (o *ObjectValue) GetCollection(field string) Collection {
	return o.cachedCollection(fieldKey{field: field}, func() Collection {
		return o.fieldCollection(field, jsonValueToFHIRValue)
	})
}

// markPrivate marks the objects in a collection, and the elements of the
// primitives in it, private: they were just read, and nothing else holds them.
func markPrivate(col Collection) {
	for _, value := range col {
		if obj, ok := ElementOf(value); ok {
			obj.private = true
		}
	}
}

// fieldKey identifies a field together with the way it was read: the same field
// parsed under a type hint is not the same collection, and a name whose type
// was read off its spelling is different again, and so is a choice element
// looked up by its base name. A struct rather than a composed string, so that a
// lookup costs nothing to build.
type fieldKey struct {
	field    string
	fhirType string
	parsedAs bool
	choice   bool
}

// cachedCollection answers a field from the cache when caching is on, and
// builds it otherwise.
//
// The objects in a cached collection cache in turn: what one expression
// navigated is what the next one starts from.
func (o *ObjectValue) cachedCollection(key fieldKey, build func() Collection) Collection {
	if !o.caching {
		// What was just read is created by the read and belongs to whoever
		// reads this object, even when this object is shared.
		col := build()
		markPrivate(col)
		return col
	}

	if col, ok := o.collections[key]; ok {
		return col[:len(col):len(col)]
	}

	col := build()
	for _, value := range col {
		// A primitive's element is read through as an object is, so it keeps
		// what it reads as well.
		if child, ok := ElementOf(value); ok {
			child.caching = true
			child.private = true
		}
	}

	if o.collections == nil {
		o.collections = make(map[fieldKey]Collection, 4)
	}
	o.collections[key] = col

	return col[:len(col):len(col)]
}

// fieldCollection reads a field together with the FHIR element stored under the
// same name prefixed with an underscore, which is how FHIR serializes a
// primitive that carries extensions or an id.
//
//	"birthDate": "1974-12-25",
//	"_birthDate": {"extension": [{"url": ".../patient-birthTime", ...}]}
//
// The two are one element, and the pairing is positional when both are arrays:
//
//	"given": [null, "James"],
//	"_given": [{"extension": [...]}]
//
// A null in the value array is a position that has extensions but no value —
// exactly what a data-absent-reason records. It is still an item of the
// collection, so it is kept as its element alone, and hasValue() answers false
// for it. Positions with neither a value nor an element are dropped.
func (o *ObjectValue) fieldCollection(field string, parse func([]byte, jsonparser.ValueType) Value) Collection {
	value, element := o.readField(field)
	return pairedCollection(value, element, parse)
}

// pairedCollection is the collection a field holds, from its value and the
// element beside it, as fieldCollection describes.
func pairedCollection(value, element jsonField, parse func([]byte, jsonparser.ValueType) Value) Collection {
	if value.missing() && element.missing() {
		return Collection{}
	}

	// A single value, with or without its element beside it
	if !value.isArray() && !element.isArray() {
		var elementValue *ObjectValue
		if element.found && element.dataType == jsonparser.Object {
			elementValue = NewObjectValue(element.data)
		}
		return pairValueWithElement(value, elementValue, parse)
	}

	values := positionalEntries(value)
	elements := positionalEntries(element)

	length := len(values)
	if len(elements) > length {
		length = len(elements)
	}

	result := make(Collection, 0, length)
	for i := 0; i < length; i++ {
		var elementValue *ObjectValue
		if i < len(elements) && elements[i].dataType == jsonparser.Object {
			elementValue = NewObjectValue(elements[i].data)
		}

		var entry jsonField
		if i < len(values) {
			entry = values[i]
		}

		result = append(result, pairValueWithElement(entry, elementValue, parse)...)
	}

	return result
}

// errFieldsFound stops a scan that has nothing left to look for. jsonparser
// ends ObjectEach when the callback returns an error, and this one is never
// reported.
var errFieldsFound = errors.New("fields found")

// readField finds a field and the element stored beside it in one pass over the
// object.
//
// Looking each of them up separately means scanning the object twice, and
// naming the element means building the string "_" + field on every access.
// Both are paid per field of every element an expression walks over, which over
// a Bundle is the greater part of the work.
func (o *ObjectValue) readField(field string) (value, element jsonField) {
	if o.index != nil {
		return o.lookup(field)
	}

	// A field read scans the object to its end: the element beside a value is
	// rarely there, and only finding both stops early. An object read more
	// than once — a resource every constraint reads, an element whose fields
	// an expression compares — is therefore scanned whole each time, and
	// indexing it on the second read answers every read after that without
	// scanning. An object read once, as most are, pays nothing for it. Only an
	// object one goroutine reads is indexed, since indexing writes to it.
	if o.keeps() && o.reads != unindexed {
		if o.reads > 0 {
			o.buildIndex()
			if o.index != nil {
				return o.lookup(field)
			}
			o.reads = unindexed
		} else {
			o.reads++
		}
	}

	//nolint:errcheck // The only error is errFieldsFound, which ends the scan
	jsonparser.ObjectEach(o.data, func(key, entry []byte, entryType jsonparser.ValueType, _ int) error {
		switch {
		// A repeated key is not valid JSON to rely on, but if one appears the
		// first occurrence is what jsonparser.Get would have returned.
		case !value.found && string(key) == field:
			value = jsonField{data: entry, dataType: entryType, found: true}
		case !element.found && len(key) > 1 && key[0] == '_' && string(key[1:]) == field:
			element = jsonField{data: entry, dataType: entryType, found: true}
		default:
			return nil
		}

		if value.found && element.found {
			return errFieldsFound
		}
		return nil
	})

	return value, element
}

// unindexed marks, in reads, an object whose fields cannot be indexed, so that
// it is not tried again.
const unindexed = math.MaxUint8

// indexedField is a field of an object as its index keeps it: where its key
// and value lie in the object's JSON, rather than the slices themselves, which
// would take three times the room for every field of every indexed object.
type indexedField struct {
	keyStart, keyEnd     uint32
	valueStart, valueEnd uint32
	dataType             jsonparser.ValueType
}

// buildIndex reads every field of the object once, keeping where each is.
func (o *ObjectValue) buildIndex() {
	// Offsets are kept in 32 bits; an object too large for them is read by
	// scanning, as it always was.
	if uint64(len(o.data)) > math.MaxUint32 {
		return
	}

	index := make([]indexedField, 0, 8)
	base := cap(o.data)
	unindexable := false
	//nolint:errcheck // ObjectEach only returns errors for non-objects; o.data is always a valid object
	jsonparser.ObjectEach(o.data, func(key, entry []byte, entryType jsonparser.ValueType, end int) error {
		// A key written with escapes is handed over unescaped, from a buffer of
		// its own, and is then not where its capacity says. FHIR does not
		// write one, but an object that does is read by scanning.
		keyStart := base - cap(key)
		if keyStart < 0 || keyStart+len(key) > len(o.data) || !bytes.Equal(o.data[keyStart:keyStart+len(key)], key) {
			unindexable = true
			return errFieldsFound
		}

		// A key is a slice of o.data, so its offset follows from its capacity,
		// as above. A value is handed over capped at its length, so its offset
		// follows from where it ends instead: end is past it, and past the
		// closing quote of a string, which the value leaves out.
		valueEnd := end
		if entryType == jsonparser.String {
			valueEnd--
		}
		index = append(index, indexedField{
			keyStart:   uint32(keyStart),              //nolint:gosec // within len(o.data), checked above
			keyEnd:     uint32(keyStart + len(key)),   //nolint:gosec // within len(o.data), checked above
			valueStart: uint32(valueEnd - len(entry)), //nolint:gosec // within len(o.data), checked above
			valueEnd:   uint32(valueEnd),              //nolint:gosec // within len(o.data), checked above
			dataType:   entryType,
		})
		return nil
	})
	if unindexable {
		return
	}
	o.index = index
}

// lookup answers readField from the index, the first occurrence of a repeated
// key as a scan would.
func (o *ObjectValue) lookup(field string) (value, element jsonField) {
	for i := range o.index {
		f := &o.index[i]
		key := o.data[f.keyStart:f.keyEnd]
		switch {
		case !value.found && string(key) == field:
			value = jsonField{data: o.data[f.valueStart:f.valueEnd:f.valueEnd], dataType: f.dataType, found: true}
		case !element.found && len(key) == len(field)+1 && key[0] == '_' && string(key[1:]) == field:
			element = jsonField{data: o.data[f.valueStart:f.valueEnd:f.valueEnd], dataType: f.dataType, found: true}
		}
	}
	return value, element
}

// jsonField is a field as it was found in the object, or the absence of one.
// The zero value is absent, which is what readField starts from.
type jsonField struct {
	data     []byte
	dataType jsonparser.ValueType
	found    bool
}

// missing reports that the field was not in the object.
func (f jsonField) missing() bool { return !f.found }

// isArray reports that the field holds a JSON array.
func (f jsonField) isArray() bool { return f.dataType == jsonparser.Array }

// pairValueWithElement turns one value and its element into the item, or items,
// they stand for: the value carrying the element, the element alone when there
// is no value, or nothing when there is neither.
func pairValueWithElement(
	value jsonField,
	element *ObjectValue,
	parse func([]byte, jsonparser.ValueType) Value,
) Collection {
	if value.missing() || value.dataType == jsonparser.Null {
		if element == nil {
			return Collection{}
		}
		return Collection{element}
	}

	parsed := parse(value.data, value.dataType)
	if parsed == nil {
		return Collection{}
	}
	if element != nil {
		parsed = withElement(parsed, element)
	}
	return Collection{parsed}
}

// positionalEntries lists a JSON array's entries in order, preserving nulls so
// that a position with extensions but no value can be told from an absent one.
func positionalEntries(field jsonField) []jsonField {
	if field.missing() {
		return nil
	}
	if !field.isArray() {
		return []jsonField{field}
	}

	var entries []jsonField
	//nolint:errcheck // ArrayEach only errors on non-arrays, and this is one
	jsonparser.ArrayEach(field.data, func(entry []byte, entryType jsonparser.ValueType, _ int, _ error) {
		entries = append(entries, jsonField{data: entry, dataType: entryType, found: true})
	})
	return entries
}

// withElement returns the value carrying the given element, for the primitive
// types that can hold one. Anything else is returned unchanged: a complex type
// already holds its extensions among its own fields.
func withElement(value Value, element *ObjectValue) Value {
	switch v := value.(type) {
	case String:
		return v.WithElement(element)
	case Boolean:
		return v.WithElement(element)
	case Integer:
		return v.WithElement(element)
	case Decimal:
		return v.WithElement(element)
	case Date:
		return v.WithElement(element)
	case DateTime:
		return v.WithElement(element)
	case Time:
		return v.WithElement(element)
	}
	return value
}

// Keys returns all field names in the object.
func (o *ObjectValue) Keys() []string {
	var keys []string
	//nolint:errcheck // ObjectEach only returns errors for non-objects; o.data is always a valid object
	jsonparser.ObjectEach(o.data, func(key []byte, _ []byte, _ jsonparser.ValueType, _ int) error {
		keys = append(keys, string(key))
		return nil
	})
	return keys
}

// Children returns a collection of all child values.
func (o *ObjectValue) Children() Collection {
	var result Collection
	//nolint:errcheck // ObjectEach only returns errors for non-objects; o.data is always a valid object
	jsonparser.ObjectEach(o.data, func(_ []byte, value []byte, dataType jsonparser.ValueType, _ int) error {
		if dataType == jsonparser.Array {
			result = append(result, jsonArrayToCollection(value)...)
		} else {
			v := jsonValueToFHIRValue(value, dataType)
			if v != nil {
				result = append(result, v)
			}
		}
		return nil
	})
	return result
}

// ElementTypeResolver resolves a FHIR element path to its type, e.g.
// "Observation.subject" to "Reference". It is the single slice of the engine's
// FHIR model that type-aware child navigation needs, declared here so that this
// package stays independent of the evaluator.
type ElementTypeResolver interface {
	TypeOf(path string) string
}

// TypedChild is a child value together with the FHIR path it was reached by, so
// that a recursive walk (descendants()) can keep resolving types as it descends.
type TypedChild struct {
	Value Value
	Path  string
}

// TypedChildren returns the object's children with their FHIR types resolved
// through res, which makes the model — not structural inference — decide what
// each child is. A child whose type the model does not know falls back to
// inference, exactly like [ObjectValue.Children].
//
// basePath is this object's FHIR path (e.g. "Observation.component"); it may be
// empty, in which case only the object's own type is used to resolve children.
func (o *ObjectValue) TypedChildren(basePath string, res ElementTypeResolver) []TypedChild {
	// A primitive and the element FHIR writes beside it under _name are one
	// child, read together as navigation reads them, not two: birthDate carries
	// the id and extensions of _birthDate, and _birthDate is not a node of its
	// own. A name held only as an element is still a child.
	//
	// Most objects have no such element, and are read field by field as they
	// stand; only one that does is read again to pair its fields.
	if children, unpaired := o.unpairedChildren(basePath, res); unpaired {
		return children
	}

	fields := o.pairedFields()
	result := make([]TypedChild, 0, len(fields))
	for i := range fields {
		result = o.appendPairedChild(result, &fields[i], basePath, res)
	}
	return result
}

// pairedField is a field of an object read with the element beside it.
type pairedField struct {
	name           string
	value, element jsonField
}

// pairedFields reads an object's fields, each with the element FHIR writes
// beside it under _name, in the order the names first appear. They are paired
// in a slice rather than a map: an element has few fields, and descendants()
// pairs those of every node it walks.
func (o *ObjectValue) pairedFields() []pairedField {
	var fields []pairedField

	//nolint:errcheck // ObjectEach only returns errors for non-objects; o.data is always a valid object
	jsonparser.ObjectEach(o.data, func(key []byte, value []byte, dataType jsonparser.ValueType, _ int) error {
		entry := jsonField{data: value, dataType: dataType, found: true}

		// Only what FHIR writes beside a primitive is its element; any other
		// field named with an underscore is a field like any other.
		isElement := len(key) > 1 && key[0] == '_' && elementShaped(entry)
		if isElement {
			key = key[1:]
		}

		i := 0
		for i < len(fields) && fields[i].name != string(key) {
			i++
		}
		switch {
		case i == len(fields):
			fields = append(fields, pairedField{name: string(key)})
		case !isElement && fields[i].value.found:
			// A repeated key is invalid JSON to rely on, but every occurrence
			// is a child, as it is for an object without elements.
			fields = append(fields, pairedField{name: string(key), value: entry})
			return nil
		case isElement && fields[i].element.found:
			fields = append(fields, pairedField{name: string(key), element: entry})
			return nil
		}

		switch {
		case isElement && !fields[i].element.found:
			fields[i].element = entry
		case !isElement:
			fields[i].value = entry
		}
		return nil
	})

	return fields
}

// elementShaped reports whether a field holds what FHIR writes beside a
// primitive: an object, or an array of objects and nulls.
func elementShaped(field jsonField) bool {
	switch field.dataType {
	case jsonparser.Object:
		return true
	case jsonparser.Array:
		shaped := true
		//nolint:errcheck // ArrayEach only returns errors for non-arrays; the field is an array
		jsonparser.ArrayEach(field.data, func(_ []byte, itemType jsonparser.ValueType, _ int, _ error) {
			if itemType != jsonparser.Object && itemType != jsonparser.Null {
				shaped = false
			}
		})
		return shaped
	}
	return false
}

// appendPairedChild appends the children a paired field holds: one per value,
// each carrying its element, or the element alone where there is no value.
func (o *ObjectValue) appendPairedChild(result []TypedChild, f *pairedField, basePath string, res ElementTypeResolver) []TypedChild {
	childPath, fhirType := o.childElement(basePath, f.name, res)
	parse := jsonValueToFHIRValue
	if fhirType != "" {
		parse = typedParser(fhirType)
	}

	for _, v := range pairedCollection(f.value, f.element, parse) {
		// A resource resolves its fields beneath its own type, so it is not
		// placed at the element that holds it.
		if obj, ok := v.(*ObjectValue); ok && fhirType != "" && !IsAbstractResourceType(fhirType) {
			obj.elementPath = childPath
		}
		markPrivate(Collection{v})
		result = append(result, TypedChild{Value: v, Path: childPath})
	}
	return result
}

// unpairedChildren reads the children of an object that holds no element
// beside a primitive: each field as it stands, the items of an array each. It
// stops at the first _name field, and reports that the object has to be read
// with its fields paired.
func (o *ObjectValue) unpairedChildren(basePath string, res ElementTypeResolver) (result []TypedChild, unpaired bool) {
	unpaired = true

	//nolint:errcheck // The only error is errFieldsFound, which ends the scan
	jsonparser.ObjectEach(o.data, func(key []byte, value []byte, dataType jsonparser.ValueType, _ int) error {
		if len(key) > 1 && key[0] == '_' {
			unpaired = false
			return errFieldsFound
		}
		name := string(key)
		childPath, fhirType := o.childElement(basePath, name, res)

		appendChild := func(data []byte, dt jsonparser.ValueType) {
			var v Value
			if fhirType != "" {
				v = jsonValueToFHIRValueWithType(data, dt, fhirType)
			} else {
				v = jsonValueToFHIRValue(data, dt)
			}
			if v != nil {
				// A resource resolves its fields beneath its own type, so it
				// is not placed at the element that holds it.
				if obj, ok := v.(*ObjectValue); ok && fhirType != "" && !IsAbstractResourceType(fhirType) {
					obj.elementPath = childPath
				}
				markPrivate(Collection{v})
				result = append(result, TypedChild{Value: v, Path: childPath})
			}
		}

		if dataType == jsonparser.Array {
			//nolint:errcheck // ArrayEach only returns errors for non-arrays; value is already an array
			jsonparser.ArrayEach(value, func(item []byte, itemType jsonparser.ValueType, _ int, _ error) {
				appendChild(item, itemType)
			})
			return nil
		}
		appendChild(value, dataType)
		return nil
	})

	if !unpaired {
		return nil, false
	}
	return result, true
}

// childElement resolves a named child's FHIR type and the path it should carry
// onward. It tries both ways a FHIR model indexes elements: a complex type or
// resource indexes its own elements ("Identifier.system", "Observation.subject"),
// while a backbone element only exists beneath the path it was reached by
// ("Observation.component.valueQuantity").
//
// The returned path is the one that resolved, so a recursive walk keeps a path
// the model can still answer for. fhirType is "" when there is no resolver or
// the model knows neither form, leaving the caller on structural inference.
func (o *ObjectValue) childElement(basePath, name string, res ElementTypeResolver) (childPath, fhirType string) {
	candidates := make([]string, 0, 2)

	// The object's own type covers complex types and contained resources, whose
	// type comes from resourceType.
	if t := o.Type(); t != "" && t != typeObject {
		candidates = append(candidates, t+"."+name)
	}
	if basePath != "" {
		candidates = append(candidates, basePath+"."+name)
	}

	if res != nil {
		for _, candidate := range candidates {
			if t := res.TypeOf(candidate); t != "" {
				return candidate, t
			}
		}
	}

	if len(candidates) > 0 {
		return candidates[0], ""
	}
	return name, ""
}

// decodeJSONString reads the contents of a JSON string, which jsonparser hands
// over without its quotes and with its escapes intact.
//
// A string without a backslash has nothing to unescape, and that is the
// ordinary case in a FHIR resource, so it is taken as it stands rather than
// through the JSON decoder — which would cost two slices and a reflective
// decode per string in the document.
func decodeJSONString(data []byte) string {
	if bytes.IndexByte(data, '\\') < 0 {
		return string(data)
	}

	var s string
	if err := json.Unmarshal(append([]byte{'"'}, append(data, '"')...), &s); err != nil {
		return string(data)
	}
	return s
}

// jsonValueToFHIRValue converts a JSON value to a FHIRPath Value.
func jsonValueToFHIRValue(data []byte, dataType jsonparser.ValueType) Value {
	switch dataType {
	case jsonparser.String:
		s := decodeJSONString(data)
		// Heuristic: try to detect ISO 8601 date/datetime patterns
		if v := tryParseTemporalString(s); v != nil {
			return v
		}
		return NewString(s)

	case jsonparser.Number:
		s := string(data)
		// Check if it's an integer
		if !strings.Contains(s, ".") && !strings.Contains(s, "e") && !strings.Contains(s, "E") {
			if i, err := jsonparser.ParseInt(data); err == nil {
				return NewInteger(i)
			}
		}
		// Parse as decimal
		d, err := NewDecimal(s)
		if err != nil {
			return nil
		}
		return d

	case jsonparser.Boolean:
		b, err := jsonparser.ParseBoolean(data)
		if err != nil {
			return nil
		}
		return NewBoolean(b)

	case jsonparser.Object:
		return NewObjectValue(data)

	case jsonparser.Array:
		// Arrays should be handled separately as collections
		return nil

	case jsonparser.Null:
		return nil
	}

	return nil
}

// tryParseTemporalString attempts to parse a string as a Date or DateTime
// using strict pattern matching. Returns nil if the string doesn't match temporal patterns.
// This provides heuristic type detection when no Model is available.
func tryParseTemporalString(s string) Value {
	if !looksTemporal(s) {
		return nil
	}
	// Try Date first for short strings (4-10 chars: YYYY to YYYY-MM-DD)
	if len(s) <= 10 {
		if d, err := NewDate(s); err == nil {
			return d
		}
	}
	// Try DateTime for anything that could be a datetime (contains T, Z, or TZ offset)
	if strings.ContainsAny(s, "TZ") || (len(s) > 10 && (s[10] == '+' || s[10] == '-')) {
		if dt, err := NewDateTime(s); err == nil {
			return dt
		}
	}
	return nil
}

// looksTemporal reports whether a string could be a date or a dateTime, both of
// which begin with a four-digit year.
//
// Every string in a resource is offered to this, since which of them are
// temporal is exactly what is being worked out, and the parsers underneath try
// a regular expression per shape. A code, a name or a URL fails on its first
// character instead.
func looksTemporal(s string) bool {
	if len(s) < 4 {
		return false
	}
	for i := 0; i < 4; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// jsonValueToFHIRValueWithType converts a JSON value to a FHIRPath Value,
// using the FHIR type hint to parse strings as Date, DateTime, Time, etc.
func jsonValueToFHIRValueWithType(data []byte, dataType jsonparser.ValueType, fhirType string) Value {
	if system, ok := strings.CutPrefix(fhirType, systemTypePrefix); ok {
		return systemValue(data, dataType, system)
	}

	// Objects carry the type so that Type() reports it
	if dataType == jsonparser.Object && fhirType != "" {
		if IsAbstractResourceType(fhirType) {
			if resource := asResource(data); resource != nil {
				return resource
			}
		}
		return NewObjectValueWithType(data, fhirType)
	}

	value := jsonValueToFHIRValue(data, dataType)
	if value == nil || fhirType == "" {
		return value
	}

	// A string may hold a date, a time or an instant, which the untyped path
	// guesses at; the declared type decides.
	if dataType == jsonparser.String {
		if typed, ok := parseTypedString(data, fhirType); ok {
			value = typed
		}
	}

	return withFHIRType(value, fhirType)
}

// systemTypePrefix is how a StructureDefinition names a System type as an
// element's type code: Resource.id and Extension.url are
// http://hl7.org/fhirpath/System.String in R4.
const systemTypePrefix = "http://hl7.org/fhirpath/System."

// systemValue reads a value the model declares as a System type. It is that
// System type — String in namespace System, not a FHIR type named by a URL —
// and a string is read as it, so an id of "2020" is not taken for a Date.
func systemValue(data []byte, dataType jsonparser.ValueType, system string) Value {
	if dataType == jsonparser.String {
		if typed, ok := parseTypedString(data, system); ok {
			return typed
		}
	}
	return jsonValueToFHIRValue(data, dataType)
}

// IsAbstractResourceType reports whether a declared type is one of the abstract
// resource types an element can hold any resource under: Bundle.entry.resource
// and every contained are Resource.
func IsAbstractResourceType(fhirType string) bool {
	switch fhirType {
	case "Resource", "DomainResource", "CanonicalResource", "MetadataResource":
		return true
	}
	return false
}

// asResource reads an object held by an element of an abstract resource type.
// A resource names its own type in resourceType, which FHIR writes on every
// resource because the element's type does not say which one it is; that type
// is the object's, and the declared one only the base it derives from. An
// object without a resourceType is not a resource, and nil leaves it to the
// declared type.
func asResource(data []byte) *ObjectValue {
	obj := NewObjectValue(data)
	if _, resourceType := obj.fieldSummary(); resourceType != "" {
		obj.typeName = resourceType
		return obj
	}
	return nil
}

// parseTypedString reads a JSON string as the temporal type the model declares,
// rather than leaving it to pattern matching.
func parseTypedString(data []byte, fhirType string) (Value, bool) {
	text := decodeJSONString(data)

	switch strings.ToLower(fhirType) {
	case "date":
		if d, err := NewDate(text); err == nil {
			return d, true
		}
	case "datetime", "instant":
		if dt, err := NewDateTime(text); err == nil {
			return dt, true
		}
	case "time":
		if t, err := NewTime(text); err == nil {
			return t, true
		}
	default:
		// Anything else is a string in FHIR terms, even where the untyped path
		// would have read it as a date
		return NewString(text), true
	}
	return nil, false
}

// withFHIRType tags a primitive with the type FHIR declared for it. FHIR
// primitives are types in their own right — FHIR.boolean is not System.Boolean —
// so the value carries the name rather than being folded into the system type.
func withFHIRType(value Value, fhirType string) Value {
	switch v := value.(type) {
	case String:
		return v.WithFHIRType(fhirType)
	case Boolean:
		return v.WithFHIRType(fhirType)
	case Integer:
		return v.WithFHIRType(fhirType)
	case Decimal:
		return v.WithFHIRType(fhirType)
	case Date:
		return v.WithFHIRType(fhirType)
	case DateTime:
		return v.WithFHIRType(fhirType)
	case Time:
		return v.WithFHIRType(fhirType)
	}
	return value
}

// GetCollectionParsedAs retrieves a field as a Collection from a type read off a
// polymorphic field name, such as the Oid in valueOid.
//
// Such a name is capitalized to form the field, while FHIR writes primitive type
// names in lower camel case and complex ones capitalized. The value itself says
// which it is — a primitive parses to a primitive — so the recorded type is
// corrected accordingly rather than kept in the field's spelling.
func (o *ObjectValue) GetCollectionParsedAs(field, suffix string) Collection {
	return o.cachedCollection(fieldKey{field: field, fhirType: suffix, parsedAs: true}, func() Collection {
		// Built from its own reading of the field rather than from
		// GetCollectionWithType, because the loop below writes into what it is
		// given and that collection may be a cached one.
		collection := o.fieldCollection(field, typedParser(suffix))
		for i, value := range collection {
			if _, isObject := value.(*ObjectValue); isObject {
				continue
			}
			collection[i] = withFHIRType(value, lowerFirst(suffix))
		}
		return collection
	})
}

// GetChoiceCollection reads the variant of a choice element the object holds —
// valueQuantity or valueString for value — trying the given type suffixes in
// order, and reads it as GetCollectionParsedAs does.
//
// Trying each suffix as a field of its own reads the object once per suffix,
// and for a name the object does not hold at all, which is what most lookups of
// this kind are, each of those reads goes to the end of the object. This reads
// the keys once and tries only the suffixes found among them. The first that
// reads as something wins: where the object holds more than one variant, which
// valid FHIR never does, that is the one listed first, and a variant written as
// null, which holds nothing, does not hide one that holds a value.
//
// The answer is kept under the name alone, so a caller passes the same suffixes
// for a name every time it asks.
func (o *ObjectValue) GetChoiceCollection(name string, suffixes []string) Collection {
	return o.cachedCollection(fieldKey{field: name, choice: true}, func() Collection {
		for _, i := range o.choiceSuffixes(name, suffixes) {
			if children := o.GetCollectionParsedAs(name+suffixes[i], suffixes[i]); len(children) > 0 {
				return children
			}
		}
		return Collection{}
	})
}

// GetChoiceCollectionWithType is GetChoiceCollection for the choice types a
// model gives an element: each is tried as name with the type
// capitalized — valueString for string — in the model's order, only where the
// object holds that key, and read as GetCollectionWithType does.
//
// The answer as a whole is not kept, since another model may give the element
// other choice types; each variant read is kept, as GetCollectionWithType keeps
// it, and the object's keys are read from its field index.
func (o *ObjectValue) GetChoiceCollectionWithType(name string, choiceTypes []string) Collection {
	for _, i := range o.choiceSuffixes(name, choiceTypes) {
		choiceType := choiceTypes[i]
		field := name + strings.ToUpper(choiceType[:1]) + choiceType[1:]
		if children := o.GetCollectionWithType(field, choiceType); len(children) > 0 {
			return children
		}
	}
	return Collection{}
}

// choiceSuffixes returns, in the order of suffixes, the index of each suffix
// that appended to name is a key of the object, either as the value or as the
// element beside it. The suffix is matched with its first letter capitalized,
// as a variant is spelled: string matches valueString.
func (o *ObjectValue) choiceSuffixes(name string, suffixes []string) []int {
	var found []int
	o.eachKey(func(key []byte) {
		if len(key) > 0 && key[0] == '_' {
			key = key[1:]
		}
		if len(key) <= len(name) || string(key[:len(name)]) != name {
			return
		}

		rest := key[len(name):]
		for i, suffix := range suffixes {
			if len(rest) == len(suffix) && rest[0] == upperASCII(suffix[0]) && string(rest[1:]) == suffix[1:] {
				if !slices.Contains(found, i) {
					found = append(found, i)
				}
				return
			}
		}
	})

	slices.Sort(found)
	return found
}

// eachKey calls fn with each of the object's keys, in order, from the index
// when the object has one. It indexes an object that caches, which will be
// read again, and one read twice before; looking for a choice's variants
// follows a field read that found nothing, so indexing on that second access
// would build an index most objects read once never use. It counts as a read
// either way. See readField.
func (o *ObjectValue) eachKey(fn func(key []byte)) {
	if o.index == nil && o.keeps() && o.reads != unindexed && (o.caching || o.reads >= 2) {
		o.buildIndex()
		if o.index == nil {
			o.reads = unindexed
		}
	}

	if o.index != nil {
		for i := range o.index {
			fn(o.data[o.index[i].keyStart:o.index[i].keyEnd])
		}
		return
	}

	if o.keeps() && o.reads < 2 {
		o.reads++
	}
	//nolint:errcheck // The callback never fails
	jsonparser.ObjectEach(o.data, func(key, _ []byte, _ jsonparser.ValueType, _ int) error {
		fn(key)
		return nil
	})
}

// upperASCII capitalizes an ASCII letter and leaves any other byte as it is.
func upperASCII(b byte) byte {
	if 'a' <= b && b <= 'z' {
		return b - 'a' + 'A'
	}
	return b
}

// lowerFirst converts a capitalized type name to the lower camel case FHIR uses
// for primitive types: Oid to oid, Base64Binary to base64Binary.
func lowerFirst(name string) string {
	if name == "" {
		return name
	}
	return strings.ToLower(name[:1]) + name[1:]
}

// GetCollectionWithType retrieves a field as a Collection, using the FHIR type hint
// to properly parse string values as Date, DateTime, Time, etc.
func (o *ObjectValue) GetCollectionWithType(field, fhirType string) Collection {
	return o.cachedCollection(fieldKey{field: field, fhirType: fhirType}, func() Collection {
		return o.fieldCollection(field, typedParser(fhirType))
	})
}

// typedParser reads a value as the FHIR type the model declares for it.
func typedParser(fhirType string) func([]byte, jsonparser.ValueType) Value {
	return func(data []byte, dataType jsonparser.ValueType) Value {
		return jsonValueToFHIRValueWithType(data, dataType, fhirType)
	}
}

// jsonArrayToCollection converts a JSON array to a Collection.
func jsonArrayToCollection(data []byte) Collection {
	var result Collection
	//nolint:errcheck // ArrayEach only returns errors for non-arrays; data is already validated as array
	jsonparser.ArrayEach(data, func(value []byte, dataType jsonparser.ValueType, _ int, _ error) {
		v := jsonValueToFHIRValue(value, dataType)
		if v != nil {
			result = append(result, v)
		}
	})
	return result
}

// JSONToCollection converts JSON bytes to a Collection.
func JSONToCollection(data []byte) (Collection, error) {
	// Detect JSON type
	value, dataType, _, err := jsonparser.Get(data)
	if err != nil {
		return nil, err
	}

	switch dataType {
	case jsonparser.Object:
		return Collection{NewObjectValue(value)}, nil
	case jsonparser.Array:
		return jsonArrayToCollection(value), nil
	case jsonparser.Null:
		return Collection{}, nil
	default:
		v := jsonValueToFHIRValue(value, dataType)
		if v == nil {
			return Collection{}, nil
		}
		return Collection{v}, nil
	}
}

// ReadRoot reads a document already known to be well formed, as
// JSONToCollection reads one, without first scanning the whole of it for where
// its value ends. An object — a resource, an element — is taken as it stands
// when it begins with { and ends with }, and its fields are read as they are
// asked for; anything else is read by JSONToCollection.
//
// That scan is what tells a malformed document, and it costs more than many
// evaluations do. Skipping it is only right for a document its caller has
// decoded or checked: one cut off after a nested object, or followed by more,
// would be read as far as it goes. A document that has not been checked is
// read with JSONToCollection.
func ReadRoot(data []byte) (Collection, error) {
	start, end := 0, len(data)
	for start < end && isJSONSpace(data[start]) {
		start++
	}
	for end > start && isJSONSpace(data[end-1]) {
		end--
	}
	if end-start >= 2 && data[start] == '{' && data[end-1] == '}' {
		return Collection{NewObjectValue(data[start:end])}, nil
	}
	return JSONToCollection(data)
}

// isJSONSpace reports whether b is whitespace between JSON tokens.
func isJSONSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// JSONToCollectionWithType converts JSON bytes to a Collection, reading the
// value as the FHIR type a model declares for it, as navigation reads a value
// it reaches: "2019-12-08" declared dateTime is a dateTime, not the Date its
// shape suggests, and an object declared Quantity is a Quantity. The items of
// an array are each read as the type. An empty fhirType reads as
// JSONToCollection does.
//
// This is for a root that is an element rather than a resource — what a
// validator evaluates an element's invariants on.
func JSONToCollectionWithType(data []byte, fhirType string) (Collection, error) {
	if fhirType == "" {
		return JSONToCollection(data)
	}

	value, dataType, _, err := jsonparser.Get(data)
	if err != nil {
		return nil, err
	}

	switch dataType {
	case jsonparser.Array:
		result := Collection{}
		//nolint:errcheck // ArrayEach only returns errors for non-arrays; value is an array
		jsonparser.ArrayEach(value, func(item []byte, itemType jsonparser.ValueType, _ int, _ error) {
			if v := jsonValueToFHIRValueWithType(item, itemType, fhirType); v != nil {
				result = append(result, v)
			}
		})
		return result, nil
	case jsonparser.Null:
		return Collection{}, nil
	default:
		if v := jsonValueToFHIRValueWithType(value, dataType, fhirType); v != nil {
			return Collection{v}, nil
		}
		return Collection{}, nil
	}
}

// ToQuantity attempts to convert an ObjectValue to a Quantity.
// This is used when the object represents a FHIR Quantity type
// (with fields like "value", "unit", "code", "system").
// Returns the Quantity and true if successful, or zero Quantity and false if not.
func (o *ObjectValue) ToQuantity() (Quantity, bool) {
	// Try to get the "value" field (required for Quantity)
	valueBytes, dataType, _, err := jsonparser.Get(o.data, "value")
	if err != nil || dataType == jsonparser.NotExist {
		return Quantity{}, false
	}

	// Parse the numeric value
	var val decimal.Decimal
	if dataType == jsonparser.Number {
		s := string(valueBytes)
		val, err = decimal.NewFromString(s)
		if err != nil {
			return Quantity{}, false
		}
	} else {
		return Quantity{}, false
	}

	// Prefer "code" over "unit": code carries the computable UCUM symbol ("mg"),
	// while unit is a human-readable display ("milligram") that no unit
	// conversion can interpret. Fall back to unit when there is no code.
	unit := ""
	hasCode := false
	if codeBytes, _, _, err := jsonparser.Get(o.data, "code"); err == nil {
		unit = string(codeBytes)
		hasCode = true
	} else if unitBytes, _, _, err := jsonparser.Get(o.data, "unit"); err == nil {
		unit = string(unitBytes)
	}

	// FHIR maps time-valued UCUM codes onto FHIRPath's calendar keywords as part
	// of this conversion, and conditions the mapping on the quantity declaring
	// UCUM as its system: "The Mapping from FHIR Quantity to FHIRPath
	// System.Quantity can only be applied if the FHIR Quantity has a UCUM code —
	// i.e. a system of http://unitsofmeasure.org, and a code is present."
	if hasCode {
		if systemBytes, _, _, err := jsonparser.Get(o.data, "system"); err == nil && string(systemBytes) == UCUMSystem {
			if keyword, mapped := CalendarUnitForUCUMCode(unit); mapped {
				unit = keyword
			}
		}
	}

	return NewQuantityFromDecimal(val, unit), true
}
