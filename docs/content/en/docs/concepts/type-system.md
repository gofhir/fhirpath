---
title: "Type System"
linkTitle: "Type System"
description: "Complete reference for the eight FHIRPath primitive types, their Go representations, core interfaces, and UCUM quantity normalization."
weight: 1
---

FHIRPath defines eight primitive types. The FHIRPath Go library maps each of them to a concrete Go struct in the `github.com/gofhir/fhirpath/types` package.

## Primitive Types

| FHIRPath Type | Go Type | FHIRPath Literal Examples |
|---------------|-----------------|--------------------------|
| Boolean | `types.Boolean` | `true`, `false` |
| Integer | `types.Integer` | `42`, `-17`, `0` |
| Decimal | `types.Decimal` | `3.14159`, `-0.5` |
| String | `types.String` | `'hello'`, `'FHIRPath'` |
| Date | `types.Date` | `@2024-01-15`, `@2024-01`, `@2024` |
| DateTime | `types.DateTime` | `@2024-01-15T10:30:00Z`, `@2024-01-15T10:30:00+05:00` |
| Time | `types.Time` | `@T14:30:00`, `@T08:00` |
| Quantity | `types.Quantity` | `10 'mg'`, `5.5 'km'`, `1000 'ms'` |

Every FHIRPath value in the library implements the `Value` interface. Collections of values are represented as `[]Value` (aliased as `Collection`).

## The Value Interface

All FHIRPath types implement the `Value` interface defined in `types/value.go`:

```go
type Value interface {
    // Type returns the FHIRPath type name (e.g., "Boolean", "Integer").
    Type() string

    // Equal compares exact equality (the = operator).
    Equal(other Value) bool

    // Equivalent compares equivalence (the ~ operator).
    // For strings: case-insensitive, normalizes whitespace.
    Equivalent(other Value) bool

    // String returns a human-readable string representation.
    String() string

    // IsEmpty indicates if this value represents empty.
    IsEmpty() bool
}
```

The distinction between `Equal` and `Equivalent` is important. Equality (`=`) is a strict comparison: `'Hello'` is not equal to `'hello'`. Equivalence (`~`) is a more lenient comparison: for strings it is case-insensitive and normalizes whitespace; for quantities it uses UCUM normalization so that `1000 'mg'` is equivalent to `1 'g'`.

## The Comparable Interface

Types that support ordering implement `Comparable`:

```go
type Comparable interface {
    Value
    // Compare returns -1, 0, or 1.
    // Returns error if types are incompatible.
    Compare(other Value) (int, error)
}
```

The following types implement `Comparable`: `Integer`, `Decimal`, `String`, `Date`, `DateTime`, `Time`, and `Quantity`.

`Boolean` does **not** implement `Comparable` because the FHIRPath specification does not define an ordering for Boolean values.

## The Numeric Interface

Numeric types (`Integer` and `Decimal`) implement the `Numeric` interface, which enables cross-type arithmetic:

```go
type Numeric interface {
    Value
    // ToDecimal converts the numeric value to a Decimal.
    ToDecimal() Decimal
}
```

When an arithmetic operator receives an `Integer` and a `Decimal`, the `Integer` is promoted to `Decimal` via `ToDecimal()` before the operation is performed.

## Type Details

### Boolean

`types.Boolean` wraps a Go `bool`.

```go
b := types.NewBoolean(true)
fmt.Println(b.Bool())   // true
fmt.Println(b.Type())   // Boolean
fmt.Println(b.Not())    // false
```

### Integer

`types.Integer` wraps an `int64` and provides arithmetic methods: `Add`, `Subtract`, `Multiply`, `Divide`, `Div` (integer division), `Mod`, `Negate`, `Abs`, `Power`, and `Sqrt`.

```go
i := types.NewInteger(42)
fmt.Println(i.Value())            // 42
fmt.Println(i.Add(types.NewInteger(8)))  // 50
fmt.Println(i.ToDecimal())        // 42
```

### Decimal

`types.Decimal` uses `shopspring/decimal` for arbitrary-precision arithmetic. It supports all the same arithmetic methods as `Integer` plus `Ceiling`, `Floor`, `Truncate`, `Round`, `Exp`, `Ln`, and `Log`.

```go
d, _ := types.NewDecimal("3.14159")
fmt.Println(d.Value())      // 3.14159
fmt.Println(d.Round(2))     // 3.14
fmt.Println(d.Ceiling())    // 4
fmt.Println(d.Floor())      // 3
```

Division always returns a `Decimal` (even for `Integer / Integer`), matching the FHIRPath specification.

### String

`types.String` wraps a Go `string` and provides `Length`, `Contains`, `StartsWith`, `EndsWith`, `Upper`, `Lower`, `IndexOf`, `Substring`, `Replace`, and `ToChars`.

```go
s := types.NewString("FHIRPath")
fmt.Println(s.Length())           // 8
fmt.Println(s.Lower())           // fhirpath
fmt.Println(s.Contains("Path"))  // true
```

Equivalence for strings is case-insensitive and normalizes whitespace:

```go
a := types.NewString("Hello  World")
b := types.NewString("hello world")
fmt.Println(a.Equal(b))      // false
fmt.Println(a.Equivalent(b)) // true
```

### Date

`types.Date` supports partial precision: year-only (`@2024`), year-month (`@2024-01`), or full date (`@2024-01-15`).

```go
d, _ := types.NewDate("2024-01-15")
fmt.Println(d.Year())   // 2024
fmt.Println(d.Month())  // 1
fmt.Println(d.Day())    // 15
```

Comparing dates with different precisions may be **ambiguous**. For example, `@2024` vs `@2024-06-15` is not clearly less than or greater than, so `Compare` returns an error to signal ambiguity (matching FHIRPath's empty-propagation semantics for incomparable values).

Date arithmetic is supported through `AddDuration` and `SubtractDuration` with temporal quantity units (`year`, `month`, `week`, `day`).

### DateTime

`types.DateTime` extends `Date` with time components (hour, minute, second, millisecond) and an optional timezone offset. It supports seven levels of precision, from year-only to millisecond.

```go
dt, _ := types.NewDateTime("2024-01-15T10:30:00Z")
fmt.Println(dt.Year())   // 2024
fmt.Println(dt.Hour())   // 10
fmt.Println(dt.Minute()) // 30
```

DateTime arithmetic supports all temporal units including `hour`, `minute`, `second`, and `millisecond`.

### Time

`types.Time` represents a time-of-day without a date component. It supports precision from hour to millisecond.

```go
t, _ := types.NewTime("14:30:00")
fmt.Println(t.Hour())   // 14
fmt.Println(t.Minute()) // 30
fmt.Println(t.Second()) // 0
```

### Quantity

`types.Quantity` pairs a `decimal.Decimal` value with a UCUM unit string. Quantities support arithmetic (`Add`, `Subtract`, `Multiply`, `Divide`) and comparison.

```go
q, _ := types.NewQuantity("10 'mg'")
fmt.Println(q.Value()) // 10
fmt.Println(q.Unit())  // mg
```

## UCUM Normalization

One of the most powerful features of the Quantity type is automatic UCUM (Unified Code for Units of Measure) normalization. When comparing or testing equivalence of quantities with different but compatible units, the library normalizes both quantities to their canonical UCUM form before comparing.

This means the following equivalences hold:

```text
1000 'mg' ~ 1 'g'      // true -- both normalize to grams
100 'cm'  ~ 1 'm'      // true -- both normalize to meters
1000 'ms' ~ 1 's'      // true -- both normalize to seconds
```

Normalization is performed automatically by the `Equal`, `Equivalent`, and `Compare` methods on `Quantity`. You can also call `Normalize()` directly to obtain the canonical form:

```go
q, _ := types.NewQuantity("1000 'mg'")
norm := q.Normalize()
fmt.Printf("Value: %f, Unit: %s\n", norm.Value, norm.Code) // Value: 1.000000, Unit: g
```

If two quantities have incompatible units (for example, `'mg'` and `'m'`), comparison returns an error rather than an incorrect result.
