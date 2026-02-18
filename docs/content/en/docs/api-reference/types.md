---
title: "Types Package"
linkTitle: "Types"
weight: 7
description: >
  FHIRPath type system: Value, Collection, and all primitive types.
---

The `github.com/gofhir/fhirpath/types` package defines the FHIRPath type system. Every value returned by a FHIRPath evaluation is a `Value`, and every evaluation result is a `Collection` (an ordered slice of `Value`). This page documents all interfaces, types, and their methods.

```go
import "github.com/gofhir/fhirpath/types"
```

---

## Core Interfaces

### Value

The base interface for all FHIRPath values. Every type in this package implements `Value`.

```go
type Value interface {
    // Type returns the FHIRPath type name (e.g., "Boolean", "Integer", "String").
    Type() string

    // Equal compares exact equality (the = operator in FHIRPath).
    Equal(other Value) bool

    // Equivalent compares equivalence (the ~ operator in FHIRPath).
    // For strings: case-insensitive, normalized whitespace.
    Equivalent(other Value) bool

    // String returns a human-readable string representation of the value.
    String() string

    // IsEmpty indicates if this value represents an empty value.
    IsEmpty() bool
}
```

### Comparable

Implemented by types that support ordering (less than, greater than). Extends `Value`.

```go
type Comparable interface {
    Value
    // Compare returns -1 if less than, 0 if equal, 1 if greater than.
    // Returns error if types are incompatible.
    Compare(other Value) (int, error)
}
```

Types that implement `Comparable`: `Integer`, `Decimal`, `String`, `Date`, `DateTime`, `Time`, `Quantity`.

### Numeric

Implemented by numeric types. Provides a conversion to `Decimal` for cross-type arithmetic.

```go
type Numeric interface {
    Value
    // ToDecimal converts the numeric value to a Decimal.
    ToDecimal() Decimal
}
```

Types that implement `Numeric`: `Integer`, `Decimal`.

---

## Collection

`Collection` is the fundamental return type for all FHIRPath expressions. It is an ordered sequence of `Value` elements.

```go
type Collection []Value
```

### Querying Methods

#### Empty

Returns `true` if the collection has no elements.

```go
func (c Collection) Empty() bool
```

#### Count

Returns the number of elements.

```go
func (c Collection) Count() int
```

#### First

Returns the first element and `true`, or `nil` and `false` if the collection is empty.

```go
func (c Collection) First() (Value, bool)
```

#### Last

Returns the last element and `true`, or `nil` and `false` if the collection is empty.

```go
func (c Collection) Last() (Value, bool)
```

#### Single

Returns the single element if the collection has exactly one element. Returns an error if the collection is empty or has more than one element.

```go
func (c Collection) Single() (Value, error)
```

#### Contains

Returns `true` if the collection contains a value equal to `v` (using `Equal`).

```go
func (c Collection) Contains(v Value) bool
```

### Subsetting Methods

#### Tail

Returns all elements except the first.

```go
func (c Collection) Tail() Collection
```

#### Skip

Returns a new collection with the first `n` elements removed.

```go
func (c Collection) Skip(n int) Collection
```

#### Take

Returns a new collection with only the first `n` elements.

```go
func (c Collection) Take(n int) Collection
```

### Set Operations

#### Distinct

Returns a new collection with duplicate values removed, preserving the order of first occurrence.

```go
func (c Collection) Distinct() Collection
```

#### IsDistinct

Returns `true` if all elements in the collection are unique.

```go
func (c Collection) IsDistinct() bool
```

#### Union

Returns the union of two collections with duplicates removed.

```go
func (c Collection) Union(other Collection) Collection
```

#### Combine

Returns a new collection that concatenates both collections. Unlike `Union`, duplicates are preserved.

```go
func (c Collection) Combine(other Collection) Collection
```

#### Intersect

Returns elements that exist in both collections.

```go
func (c Collection) Intersect(other Collection) Collection
```

#### Exclude

Returns elements in `c` that are not in `other`.

```go
func (c Collection) Exclude(other Collection) Collection
```

### Boolean Aggregation

#### AllTrue

Returns `true` if every element is a Boolean with value `true`. Returns `true` for an empty collection (vacuous truth).

```go
func (c Collection) AllTrue() bool
```

#### AnyTrue

Returns `true` if at least one element is a Boolean with value `true`.

```go
func (c Collection) AnyTrue() bool
```

#### AllFalse

Returns `true` if every element is a Boolean with value `false`. Returns `true` for an empty collection.

```go
func (c Collection) AllFalse() bool
```

#### AnyFalse

Returns `true` if at least one element is a Boolean with value `false`.

```go
func (c Collection) AnyFalse() bool
```

### Conversion

#### ToBoolean

Converts a singleton Boolean collection to a Go `bool`. Returns an error if the collection is empty, has more than one element, or the single element is not a Boolean.

```go
func (c Collection) ToBoolean() (bool, error)
```

#### String

Returns a string representation of the collection in the form `[val1, val2, ...]`.

```go
func (c Collection) String() string
```

### Collection Example

```go
import "github.com/gofhir/fhirpath/types"

c := types.Collection{
    types.NewString("alpha"),
    types.NewString("beta"),
    types.NewString("gamma"),
}

fmt.Println(c.Count())       // 3
fmt.Println(c.Empty())       // false

first, ok := c.First()
fmt.Println(first, ok)       // alpha true

tail := c.Tail()
fmt.Println(tail)            // [beta, gamma]

top2 := c.Take(2)
fmt.Println(top2)            // [alpha, beta]

without1 := c.Skip(1)
fmt.Println(without1)        // [beta, gamma]

single, err := c.Take(1).Single()
fmt.Println(single, err)     // alpha <nil>
```

---

## Boolean

Represents a FHIRPath boolean value (`true` or `false`).

```go
type Boolean struct {
    // unexported fields
}
```

**Implements:** `Value`

### NewBoolean

```go
func NewBoolean(v bool) Boolean
```

### Key Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Bool` | `func (b Boolean) Bool() bool` | Returns the underlying `bool` value |
| `Not` | `func (b Boolean) Not() Boolean` | Returns the logical negation |
| `Type` | `func (b Boolean) Type() string` | Returns `"Boolean"` |
| `String` | `func (b Boolean) String() string` | Returns `"true"` or `"false"` |

**Example:**

```go
t := types.NewBoolean(true)
f := t.Not()

fmt.Println(t.Bool())   // true
fmt.Println(f.Bool())   // false
fmt.Println(t.Type())   // Boolean
fmt.Println(t.Equal(f)) // false
```

---

## Integer

Represents a FHIRPath integer value (backed by `int64`).

```go
type Integer struct {
    // unexported fields
}
```

**Implements:** `Value`, `Comparable`, `Numeric`

### NewInteger

```go
func NewInteger(v int64) Integer
```

### Key Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Value` | `func (i Integer) Value() int64` | Returns the underlying `int64` |
| `ToDecimal` | `func (i Integer) ToDecimal() Decimal` | Converts to `Decimal` |
| `Add` | `func (i Integer) Add(other Integer) Integer` | Addition |
| `Subtract` | `func (i Integer) Subtract(other Integer) Integer` | Subtraction |
| `Multiply` | `func (i Integer) Multiply(other Integer) Integer` | Multiplication |
| `Divide` | `func (i Integer) Divide(other Integer) (Decimal, error)` | Division (returns `Decimal`) |
| `Div` | `func (i Integer) Div(other Integer) (Integer, error)` | Integer division |
| `Mod` | `func (i Integer) Mod(other Integer) (Integer, error)` | Modulo |
| `Negate` | `func (i Integer) Negate() Integer` | Negation |
| `Abs` | `func (i Integer) Abs() Integer` | Absolute value |
| `Power` | `func (i Integer) Power(exp Integer) Decimal` | Exponentiation (returns `Decimal`) |
| `Sqrt` | `func (i Integer) Sqrt() (Decimal, error)` | Square root (returns `Decimal`) |
| `Compare` | `func (i Integer) Compare(other Value) (int, error)` | Comparison (works with `Integer` and `Decimal`) |

**Example:**

```go
a := types.NewInteger(10)
b := types.NewInteger(3)

fmt.Println(a.Add(b).Value())       // 13
fmt.Println(a.Subtract(b).Value())  // 7
fmt.Println(a.Multiply(b).Value())  // 30

div, _ := a.Div(b)
fmt.Println(div.Value())            // 3

mod, _ := a.Mod(b)
fmt.Println(mod.Value())            // 1

result, _ := a.Divide(b)
fmt.Println(result)                 // 3.3333333333333333
```

---

## Decimal

Represents a FHIRPath decimal value with arbitrary precision (backed by `github.com/shopspring/decimal`).

```go
type Decimal struct {
    // unexported fields
}
```

**Implements:** `Value`, `Comparable`, `Numeric`

### Constructors

| Function | Signature | Description |
|----------|-----------|-------------|
| `NewDecimal` | `func NewDecimal(s string) (Decimal, error)` | Creates from a string like `"3.14"` |
| `NewDecimalFromInt` | `func NewDecimalFromInt(v int64) Decimal` | Creates from an `int64` |
| `NewDecimalFromFloat` | `func NewDecimalFromFloat(v float64) Decimal` | Creates from a `float64` |
| `MustDecimal` | `func MustDecimal(s string) Decimal` | Like `NewDecimal`, panics on error |

### Key Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Value` | `func (d Decimal) Value() decimal.Decimal` | Returns the underlying `decimal.Decimal` |
| `ToDecimal` | `func (d Decimal) ToDecimal() Decimal` | Returns itself |
| `Add` | `func (d Decimal) Add(other Decimal) Decimal` | Addition |
| `Subtract` | `func (d Decimal) Subtract(other Decimal) Decimal` | Subtraction |
| `Multiply` | `func (d Decimal) Multiply(other Decimal) Decimal` | Multiplication |
| `Divide` | `func (d Decimal) Divide(other Decimal) (Decimal, error)` | Division (16-digit precision) |
| `Negate` | `func (d Decimal) Negate() Decimal` | Negation |
| `Abs` | `func (d Decimal) Abs() Decimal` | Absolute value |
| `Ceiling` | `func (d Decimal) Ceiling() Integer` | Smallest integer >= d |
| `Floor` | `func (d Decimal) Floor() Integer` | Largest integer <= d |
| `Truncate` | `func (d Decimal) Truncate() Integer` | Integer part |
| `Round` | `func (d Decimal) Round(precision int32) Decimal` | Round to precision |
| `Power` | `func (d Decimal) Power(exp Decimal) Decimal` | Exponentiation |
| `Sqrt` | `func (d Decimal) Sqrt() (Decimal, error)` | Square root |
| `Exp` | `func (d Decimal) Exp() Decimal` | e^d |
| `Ln` | `func (d Decimal) Ln() (Decimal, error)` | Natural logarithm |
| `Log` | `func (d Decimal) Log(base Decimal) (Decimal, error)` | Logarithm with custom base |
| `IsInteger` | `func (d Decimal) IsInteger() bool` | True if no fractional part |
| `ToInteger` | `func (d Decimal) ToInteger() (Integer, bool)` | Convert to Integer if whole number |
| `Compare` | `func (d Decimal) Compare(other Value) (int, error)` | Comparison (works with `Integer` and `Decimal`) |

**Example:**

```go
pi, _ := types.NewDecimal("3.14159")
two := types.NewDecimalFromInt(2)

fmt.Println(pi.Add(two))           // 5.14159
fmt.Println(pi.Multiply(two))      // 6.28318
fmt.Println(pi.Round(2))           // 3.14
fmt.Println(pi.Ceiling())          // 4
fmt.Println(pi.Floor())            // 3
fmt.Println(pi.IsInteger())        // false

// MustDecimal for constants
half := types.MustDecimal("0.5")
fmt.Println(half)                  // 0.5
```

---

## String

Represents a FHIRPath string value.

```go
type String struct {
    // unexported fields
}
```

**Implements:** `Value`, `Comparable`

### NewString

```go
func NewString(v string) String
```

### Key Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Value` | `func (s String) Value() string` | Returns the underlying Go string |
| `Length` | `func (s String) Length() int` | Number of characters (rune count) |
| `Contains` | `func (s String) Contains(substr string) bool` | Substring check |
| `StartsWith` | `func (s String) StartsWith(prefix string) bool` | Prefix check |
| `EndsWith` | `func (s String) EndsWith(suffix string) bool` | Suffix check |
| `Upper` | `func (s String) Upper() String` | Uppercase |
| `Lower` | `func (s String) Lower() String` | Lowercase |
| `IndexOf` | `func (s String) IndexOf(substr string) int` | First occurrence index (-1 if not found) |
| `Substring` | `func (s String) Substring(start, length int) String` | Extract substring |
| `Replace` | `func (s String) Replace(old, replacement string) String` | Replace all occurrences |
| `ToChars` | `func (s String) ToChars() Collection` | Split into single-character strings |
| `Compare` | `func (s String) Compare(other Value) (int, error)` | Lexicographic comparison |

**Equivalence behavior:** `Equivalent()` for strings is case-insensitive and normalizes whitespace (trims leading/trailing whitespace, collapses internal whitespace to single spaces).

**Example:**

```go
s := types.NewString("Hello, World!")

fmt.Println(s.Length())                  // 13
fmt.Println(s.Contains("World"))         // true
fmt.Println(s.StartsWith("Hello"))       // true
fmt.Println(s.Upper())                   // HELLO, WORLD!
fmt.Println(s.Lower())                   // hello, world!
fmt.Println(s.IndexOf("World"))          // 7
fmt.Println(s.Substring(0, 5))           // Hello
fmt.Println(s.Replace("World", "Go"))    // Hello, Go!

// Equivalence is case-insensitive
a := types.NewString("  hello  world  ")
b := types.NewString("Hello World")
fmt.Println(a.Equivalent(b))             // true
fmt.Println(a.Equal(b))                  // false
```

---

## Date

Represents a FHIRPath date value with variable precision (year, year-month, or year-month-day).

```go
type Date struct {
    // unexported fields
}
```

**Implements:** `Value`, `Comparable`

### Constructors

| Function | Signature | Description |
|----------|-----------|-------------|
| `NewDate` | `func NewDate(s string) (Date, error)` | Parses `"2024"`, `"2024-03"`, or `"2024-03-15"` |
| `NewDateFromTime` | `func NewDateFromTime(t time.Time) Date` | Creates from `time.Time` with day precision |

### Precision Constants

```go
type DatePrecision int

const (
    YearPrecision  DatePrecision = iota // e.g., "2024"
    MonthPrecision                       // e.g., "2024-03"
    DayPrecision                         // e.g., "2024-03-15"
)
```

### Key Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Year` | `func (d Date) Year() int` | Year component |
| `Month` | `func (d Date) Month() int` | Month component (0 if not specified) |
| `Day` | `func (d Date) Day() int` | Day component (0 if not specified) |
| `Precision` | `func (d Date) Precision() DatePrecision` | The precision level |
| `ToTime` | `func (d Date) ToTime() time.Time` | Convert to `time.Time` (defaults for missing components) |
| `AddDuration` | `func (d Date) AddDuration(value int, unit string) Date` | Add a temporal duration |
| `SubtractDuration` | `func (d Date) SubtractDuration(value int, unit string) Date` | Subtract a temporal duration |
| `Compare` | `func (d Date) Compare(other Value) (int, error)` | Compare (may return error for ambiguous precision) |

Supported duration units for `AddDuration`/`SubtractDuration`: `"year"`, `"years"`, `"month"`, `"months"`, `"week"`, `"weeks"`, `"day"`, `"days"`.

**Example:**

```go
d, _ := types.NewDate("2024-03-15")
fmt.Println(d.Year())       // 2024
fmt.Println(d.Month())      // 3
fmt.Println(d.Day())        // 15
fmt.Println(d.Precision())  // DayPrecision

// Partial date
partial, _ := types.NewDate("2024-03")
fmt.Println(partial)          // 2024-03
fmt.Println(partial.Day())   // 0 (not specified)

// Date arithmetic
next := d.AddDuration(1, "month")
fmt.Println(next)            // 2024-04-15
```

---

## DateTime

Represents a FHIRPath datetime value with variable precision from year to millisecond, with optional timezone.

```go
type DateTime struct {
    // unexported fields
}
```

**Implements:** `Value`, `Comparable`

### Constructors

| Function | Signature | Description |
|----------|-----------|-------------|
| `NewDateTime` | `func NewDateTime(s string) (DateTime, error)` | Parses ISO 8601 datetime strings |
| `NewDateTimeFromTime` | `func NewDateTimeFromTime(t time.Time) DateTime` | Creates from `time.Time` with millisecond precision |

Accepted formats include: `"2024"`, `"2024-03"`, `"2024-03-15"`, `"2024-03-15T10:30"`, `"2024-03-15T10:30:00"`, `"2024-03-15T10:30:00.000"`, `"2024-03-15T10:30:00Z"`, `"2024-03-15T10:30:00+05:00"`.

### Precision Constants

```go
type DateTimePrecision int

const (
    DTYearPrecision   DateTimePrecision = iota
    DTMonthPrecision
    DTDayPrecision
    DTHourPrecision
    DTMinutePrecision
    DTSecondPrecision
    DTMillisPrecision
)
```

### Key Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Year` | `func (dt DateTime) Year() int` | Year component |
| `Month` | `func (dt DateTime) Month() int` | Month component |
| `Day` | `func (dt DateTime) Day() int` | Day component |
| `Hour` | `func (dt DateTime) Hour() int` | Hour component |
| `Minute` | `func (dt DateTime) Minute() int` | Minute component |
| `Second` | `func (dt DateTime) Second() int` | Second component |
| `Millisecond` | `func (dt DateTime) Millisecond() int` | Millisecond component |
| `ToTime` | `func (dt DateTime) ToTime() time.Time` | Convert to `time.Time` |
| `AddDuration` | `func (dt DateTime) AddDuration(value int, unit string) DateTime` | Add a temporal duration |
| `SubtractDuration` | `func (dt DateTime) SubtractDuration(value int, unit string) DateTime` | Subtract a temporal duration |
| `Compare` | `func (dt DateTime) Compare(other Value) (int, error)` | Compare (may return error for ambiguous precision) |

Supported duration units: `"year"`, `"years"`, `"month"`, `"months"`, `"week"`, `"weeks"`, `"day"`, `"days"`, `"hour"`, `"hours"`, `"minute"`, `"minutes"`, `"second"`, `"seconds"`, `"millisecond"`, `"milliseconds"`, `"ms"`.

**Example:**

```go
dt, _ := types.NewDateTime("2024-03-15T14:30:00Z")
fmt.Println(dt.Year())        // 2024
fmt.Println(dt.Hour())        // 14
fmt.Println(dt.Minute())      // 30
fmt.Println(dt)               // 2024-03-15T14:30:00Z

// DateTime arithmetic
later := dt.AddDuration(2, "hours")
fmt.Println(later)             // 2024-03-15T16:30:00Z

// From time.Time
now := types.NewDateTimeFromTime(time.Now())
fmt.Println(now.Type())        // DateTime
```

---

## Time

Represents a FHIRPath time value with variable precision from hour to millisecond.

```go
type Time struct {
    // unexported fields
}
```

**Implements:** `Value`, `Comparable`

### Constructors

| Function | Signature | Description |
|----------|-----------|-------------|
| `NewTime` | `func NewTime(s string) (Time, error)` | Parses time strings like `"14:30"`, `"14:30:00"`, `"T14:30:00.000"` |
| `NewTimeFromGoTime` | `func NewTimeFromGoTime(t time.Time) Time` | Creates from `time.Time` with millisecond precision |

### Precision Constants

```go
type TimePrecision int

const (
    HourPrecision   TimePrecision = iota
    MinutePrecision
    SecondPrecision
    MillisPrecision
)
```

### Key Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Hour` | `func (t Time) Hour() int` | Hour component |
| `Minute` | `func (t Time) Minute() int` | Minute component |
| `Second` | `func (t Time) Second() int` | Second component |
| `Millisecond` | `func (t Time) Millisecond() int` | Millisecond component |
| `Compare` | `func (t Time) Compare(other Value) (int, error)` | Compare (may return error for ambiguous precision) |

**Example:**

```go
t, _ := types.NewTime("14:30:00")
fmt.Println(t.Hour())        // 14
fmt.Println(t.Minute())      // 30
fmt.Println(t.Second())      // 0
fmt.Println(t)               // 14:30:00

// With milliseconds
precise, _ := types.NewTime("T08:15:30.500")
fmt.Println(precise.Millisecond()) // 500
```

---

## Quantity

Represents a FHIRPath quantity -- a numeric value paired with a unit string. Supports UCUM unit normalization for comparing quantities with different but compatible units.

```go
type Quantity struct {
    // unexported fields
}
```

**Implements:** `Value`, `Comparable`

### Constructors

| Function | Signature | Description |
|----------|-----------|-------------|
| `NewQuantity` | `func NewQuantity(s string) (Quantity, error)` | Parses strings like `"5.5 'mg'"`, `"100 kg"` |
| `NewQuantityFromDecimal` | `func NewQuantityFromDecimal(value decimal.Decimal, unit string) Quantity` | Creates from a `decimal.Decimal` and unit string |

### Key Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Value` | `func (q Quantity) Value() decimal.Decimal` | Returns the numeric value |
| `Unit` | `func (q Quantity) Unit() string` | Returns the unit string |
| `Add` | `func (q Quantity) Add(other Quantity) (Quantity, error)` | Add (same unit required) |
| `Subtract` | `func (q Quantity) Subtract(other Quantity) (Quantity, error)` | Subtract (same unit required) |
| `Multiply` | `func (q Quantity) Multiply(factor decimal.Decimal) Quantity` | Multiply by a number |
| `Divide` | `func (q Quantity) Divide(divisor decimal.Decimal) (Quantity, error)` | Divide by a number |
| `Normalize` | `func (q Quantity) Normalize() ucum.NormalizedQuantity` | UCUM normalization |
| `Compare` | `func (q Quantity) Compare(other Value) (int, error)` | Compare (supports compatible units via UCUM) |

**Equivalence behavior:** `Equivalent()` for quantities uses UCUM normalization, so `10 'cm'` and `0.1 'm'` are considered equivalent.

**Example:**

```go
q, _ := types.NewQuantity("75.5 'kg'")
fmt.Println(q.Value()) // 75.5
fmt.Println(q.Unit())  // kg
fmt.Println(q)         // 75.5 kg

// Arithmetic
q2, _ := types.NewQuantity("2.5 'kg'")
sum, _ := q.Add(q2)
fmt.Println(sum)       // 78 kg
```

---

## ObjectValue

Represents a FHIR® resource or complex type as a JSON object. This type is used internally to represent structured data within the evaluation engine. The `Type()` method attempts to infer the FHIR® type from the JSON structure (checking `resourceType` first, then structural patterns for common complex types).

```go
type ObjectValue struct {
    // unexported fields
}
```

**Implements:** `Value`

### NewObjectValue

```go
func NewObjectValue(data []byte) *ObjectValue
```

Creates an `ObjectValue` from raw JSON bytes representing an object.

### Key Methods

| Method | Signature | Description |
|--------|-----------|-------------|
| `Type` | `func (o *ObjectValue) Type() string` | Inferred FHIR® type or `"Object"` |
| `Data` | `func (o *ObjectValue) Data() []byte` | Raw JSON bytes |
| `Get` | `func (o *ObjectValue) Get(field string) (Value, bool)` | Get a field value (cached) |
| `GetCollection` | `func (o *ObjectValue) GetCollection(field string) Collection` | Get a field as a Collection |
| `Keys` | `func (o *ObjectValue) Keys() []string` | All field names |
| `Children` | `func (o *ObjectValue) Children() Collection` | All child values |
| `ToQuantity` | `func (o *ObjectValue) ToQuantity() (Quantity, bool)` | Convert to Quantity if structure matches |

**Type inference:** The `Type()` method recognizes FHIR® resource types (via `resourceType` field) and common complex types including `Quantity`, `Coding`, `CodeableConcept`, `Reference`, `Period`, `Identifier`, `Range`, `Ratio`, `Attachment`, `HumanName`, `Address`, `ContactPoint`, and `Annotation`.

**Example:**

```go
data := []byte(`{"resourceType": "Patient", "id": "123", "active": true}`)
obj := types.NewObjectValue(data)

fmt.Println(obj.Type()) // Patient

if v, ok := obj.Get("id"); ok {
    fmt.Println(v) // 123
}

keys := obj.Keys()
fmt.Println(keys) // [resourceType id active]
```

---

## TypeError

Represents a type mismatch error that can occur during operations on values.

```go
type TypeError struct {
    Expected  string
    Actual    string
    Operation string
}
```

### NewTypeError

```go
func NewTypeError(expected, actual, operation string) *TypeError
```

### Error

```go
func (e *TypeError) Error() string
// Returns: "type error in <operation>: expected <expected>, got <actual>"
```

**Example:**

```go
err := types.NewTypeError("Integer", "String", "comparison")
fmt.Println(err.Error()) // type error in comparison: expected Integer, got String
```

---

## Utility Functions

### JSONToCollection

Converts raw JSON bytes (which can be an object, array, or primitive) to a `Collection`.

```go
func JSONToCollection(data []byte) (Collection, error)
```

**Behavior:**

- JSON object: Returns a singleton collection with an `*ObjectValue`
- JSON array: Returns a collection with one element per array item
- JSON null: Returns an empty collection
- JSON primitive: Returns a singleton collection with the corresponding type

---

## Type Summary

| Type | FHIRPath Name | Go Backing Type | Implements |
|------|---------------|-----------------|------------|
| `Boolean` | Boolean | `bool` | `Value` |
| `Integer` | Integer | `int64` | `Value`, `Comparable`, `Numeric` |
| `Decimal` | Decimal | `decimal.Decimal` | `Value`, `Comparable`, `Numeric` |
| `String` | String | `string` | `Value`, `Comparable` |
| `Date` | Date | year/month/day ints | `Value`, `Comparable` |
| `DateTime` | DateTime | component ints + timezone | `Value`, `Comparable` |
| `Time` | Time | hour/minute/second/millis ints | `Value`, `Comparable` |
| `Quantity` | Quantity | `decimal.Decimal` + unit string | `Value`, `Comparable` |
| `*ObjectValue` | (inferred) | `[]byte` JSON | `Value` |
