---
title: "Boundary Functions"
linkTitle: "Boundary Functions"
weight: 13
description: >
  FHIRPath 2.0 boundary functions for determining the lowest and highest possible values based on precision.
---

Boundary functions return the lowest or highest possible value for a given input based on its precision. They are defined in the [FHIRPath 2.0 specification](http://hl7.org/fhirpath/) and operate on `Date`, `DateTime`, `Time`, `Decimal`, `Integer`, and `Quantity` types.

---

## lowBoundary

Returns the lowest possible value that the input could represent, given its precision.

**Signature:**

```text
lowBoundary([precision : Integer]) : Date | DateTime | Time | Decimal | Integer | Quantity
```

**Parameters:**

| Name        | Type      | Description                                                                                          |
|-------------|-----------|------------------------------------------------------------------------------------------------------|
| `precision` | `Integer` | (Optional) Number of decimal digits for `Decimal` and `Quantity`. Inferred from representation if omitted |

**Return Type:** Same type as input

**Examples:**

```go
// Date -- fills missing components with their lowest values
result, _ := fhirpath.Evaluate(resource, "@2024.lowBoundary()")
// @2024-01-01

result, _ := fhirpath.Evaluate(resource, "@2024-06.lowBoundary()")
// @2024-06-01

// DateTime -- fills to millisecond precision with +14:00 (earliest timezone)
result, _ := fhirpath.Evaluate(resource, "@2024-06-15.lowBoundary()")
// Note: as a DateTime literal, this returns @2024-06-15T00:00:00.000+14:00

// DateTime with existing timezone -- preserves TZ
result, _ := fhirpath.Evaluate(resource, "@2024-06-15T10:00:00+02:00.lowBoundary()")
// @2024-06-15T10:00:00.000+02:00

// Time -- fills missing components with zeros
result, _ := fhirpath.Evaluate(resource, "@T12.lowBoundary()")
// @T12:00:00.000

// Decimal -- subtracts half the precision unit
result, _ := fhirpath.Evaluate(resource, "(1.0).lowBoundary()")
// 0.95 (= 1.0 - 0.05, inferred precision 1)

result, _ := fhirpath.Evaluate(resource, "(1.0).lowBoundary(1)")
// 0.95 (= 1.0 - 0.05, explicit precision 1)

// Integer -- returns itself (no precision-based range)
result, _ := fhirpath.Evaluate(resource, "(42).lowBoundary()")
// 42

// Quantity -- subtracts half the precision unit from the value
result, _ := fhirpath.Evaluate(resource, "(1.0 'mg').lowBoundary(1)")
// 0.95 'mg'
```

**Edge Cases / Notes:**

- For `Date`: fills missing month with `01` and missing day with `01`.
- For `DateTime` without timezone: appends `+14:00` (the earliest UTC offset) per the FHIRPath specification. If a timezone is already present, it is preserved.
- For `DateTime` already at millisecond precision: returns the value unchanged.
- For `Time`: fills missing minute, second, and millisecond with `0`.
- For `Decimal`: if no explicit precision is given, it is inferred from the original string representation (e.g., `"1.0"` has implicit precision 1). If the decimal has no fractional digits (e.g., `"1"`), returns an empty collection.
- For `Integer`: returns the input value unchanged.
- For `Quantity`: works like `Decimal` but preserves the unit. Precision is inferred from the value's exponent when not provided explicitly.
- Returns empty collection if the input is empty.

---

## highBoundary

Returns the highest possible value that the input could represent, given its precision.

**Signature:**

```text
highBoundary([precision : Integer]) : Date | DateTime | Time | Decimal | Integer | Quantity
```

**Parameters:**

| Name        | Type      | Description                                                                                          |
|-------------|-----------|------------------------------------------------------------------------------------------------------|
| `precision` | `Integer` | (Optional) Number of decimal digits for `Decimal` and `Quantity`. Inferred from representation if omitted |

**Return Type:** Same type as input

**Examples:**

```go
// Date -- fills missing components with their highest values
result, _ := fhirpath.Evaluate(resource, "@2024.highBoundary()")
// @2024-12-31

result, _ := fhirpath.Evaluate(resource, "@2024-02.highBoundary()")
// @2024-02-29 (leap year)

result, _ := fhirpath.Evaluate(resource, "@2023-02.highBoundary()")
// @2023-02-28 (non-leap year)

// DateTime -- fills to millisecond precision with -12:00 (latest timezone)
result, _ := fhirpath.Evaluate(resource, "@2024-06-15.highBoundary()")
// Note: as a DateTime literal, this returns @2024-06-15T23:59:59.999-12:00

// DateTime with existing timezone -- preserves TZ
result, _ := fhirpath.Evaluate(resource, "@2024-06-15T10:00:00+02:00.highBoundary()")
// @2024-06-15T10:00:00.999+02:00

// Time -- fills missing components with maximum values
result, _ := fhirpath.Evaluate(resource, "@T12.highBoundary()")
// @T12:59:59.999

// Decimal -- adds half the precision unit
result, _ := fhirpath.Evaluate(resource, "(1.0).highBoundary()")
// 1.05 (= 1.0 + 0.05, inferred precision 1)

result, _ := fhirpath.Evaluate(resource, "(1.0).highBoundary(1)")
// 1.05 (= 1.0 + 0.05, explicit precision 1)

// Integer -- returns itself
result, _ := fhirpath.Evaluate(resource, "(42).highBoundary()")
// 42

// Quantity -- adds half the precision unit to the value
result, _ := fhirpath.Evaluate(resource, "(1.0 'mg').highBoundary(1)")
// 1.05 'mg'
```

**Edge Cases / Notes:**

- For `Date`: fills missing month with `12` and missing day with the last day of the resolved month (accounts for leap years).
- For `DateTime` without timezone: appends `-12:00` (the latest UTC offset) per the FHIRPath specification. If a timezone is already present, it is preserved.
- For `DateTime` already at millisecond precision: returns the value unchanged.
- For `Time`: fills missing minute and second with `59`, missing millisecond with `999`.
- For `Decimal`: if no explicit precision is given, it is inferred from the original string representation. If the decimal has no fractional digits, returns an empty collection.
- For `Integer`: returns the input value unchanged.
- For `Quantity`: works like `Decimal` but preserves the unit.
- Returns empty collection if the input is empty.

---

## Precision Inference

When `lowBoundary()` or `highBoundary()` are called on `Decimal` or `Quantity` values without an explicit `precision` argument, the precision is inferred automatically:

- **Decimal**: Precision is determined from the original string representation. For example, `"1.0"` has 1 decimal place, `"3.14"` has 2, and `"42"` has 0.
- **Quantity**: Precision is inferred from the decimal exponent of the numeric value.

If the inferred precision is `0` (no fractional digits), the functions return an empty collection -- matching the FHIRPath specification behavior that integer-like values without fractional precision have no meaningful boundary range.

```go
// Precision inferred from string representation
result, _ := fhirpath.Evaluate(resource, "(1.0).lowBoundary()")
// 0.95 (precision 1 inferred from "1.0")

result, _ := fhirpath.Evaluate(resource, "(3.14).lowBoundary()")
// 3.135 (precision 2 inferred from "3.14")

// Integer-like decimal returns empty
result, _ := fhirpath.Evaluate(resource, "(1).lowBoundary()")
// {} (empty -- no fractional precision)
```

---

## Timezone Behavior for DateTime

The FHIRPath specification defines specific timezone offset rules for boundary functions on `DateTime` values:

| Function        | No TZ present   | TZ present       |
|-----------------|-----------------|------------------|
| `lowBoundary`   | Adds `+14:00`   | Preserves TZ     |
| `highBoundary`  | Adds `-12:00`   | Preserves TZ     |

The rationale is that `+14:00` represents the earliest possible point in time (farthest ahead of UTC), while `-12:00` represents the latest possible point in time (farthest behind UTC). This ensures the boundary range covers all possible instants the DateTime could represent.
