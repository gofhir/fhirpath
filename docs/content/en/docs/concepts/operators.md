---
title: "Operators"
linkTitle: "Operators"
description: "Complete reference for all FHIRPath operators: arithmetic, comparison, equality, equivalence, Boolean (with three-valued truth tables), collection, type, and string operators, plus precedence rules."
weight: 3
---

FHIRPath defines a rich set of operators for arithmetic, comparison, logic, and collection manipulation. This page documents every operator supported by the FHIRPath Go library, along with their behavior under three-valued (empty-propagating) logic.

## Arithmetic Operators

Arithmetic operators work on `Integer`, `Decimal`, and (where noted) `Quantity`, `String`, `Date`, and `DateTime` values.

| Operator | Name | Left Types | Right Types | Result Type |
|----------|------|------------|-------------|-------------|
| `+` | Addition | Integer, Decimal, String, Date, DateTime, Quantity | Integer, Decimal, String, Quantity | Varies (see below) |
| `-` | Subtraction | Integer, Decimal, Date, DateTime, Quantity | Integer, Decimal, Quantity | Varies |
| `*` | Multiplication | Integer, Decimal | Integer, Decimal | Integer or Decimal |
| `/` | Division | Integer, Decimal | Integer, Decimal | Decimal (always) |
| `div` | Integer division | Integer | Integer | Integer |
| `mod` | Modulo | Integer | Integer | Integer |

**Type promotion:** When one operand is `Integer` and the other is `Decimal`, the `Integer` is promoted to `Decimal` automatically.

**Division always returns Decimal:** Even `6 / 3` returns the Decimal `2.0`, not Integer `2`. This matches the FHIRPath specification. Use `div` for integer division.

**String concatenation:** The `+` operator concatenates two strings: `'Hello' + ' World'` produces `'Hello World'`. If either operand is empty, the result is empty. For null-safe concatenation, use the `&` operator instead (see [String Operators](#string-operators)).

**Date/DateTime arithmetic:** You can add or subtract a `Quantity` with a temporal unit to/from a `Date` or `DateTime`:

```text
@2024-01-15 + 30 days        --> @2024-02-14
@2024-01-15T10:00:00Z - 2 hours  --> @2024-01-15T08:00:00Z
```

**Quantity arithmetic:** Quantities with the same unit can be added or subtracted:

```text
10 'mg' + 5 'mg'  --> 15 'mg'
10 'mg' - 3 'mg'  --> 7 'mg'
```

**Empty propagation:** If either operand is empty, arithmetic operators return empty.

### Examples

```text
2 + 3           --> 5          (Integer)
2.0 + 3         --> 5.0        (Decimal, due to promotion)
10 / 3          --> 3.3333...  (Decimal)
10 div 3        --> 3          (Integer)
10 mod 3        --> 1          (Integer)
'Hello' + ' '   --> 'Hello '   (String)
```

## Comparison Operators

Comparison operators work on any two values of the same `Comparable` type (Integer, Decimal, String, Date, DateTime, Time, Quantity). They return a singleton Boolean collection.

| Operator | Name | Description |
|----------|------|-------------|
| `<` | Less than | True if left is strictly less than right |
| `>` | Greater than | True if left is strictly greater than right |
| `<=` | Less or equal | True if left is less than or equal to right |
| `>=` | Greater or equal | True if left is greater than or equal to right |

**Empty propagation:** If either operand is empty, comparison operators return empty.

**Cross-type comparison:** Integer and Decimal can be compared directly (the Integer is promoted). Comparing incompatible types (e.g., String vs Integer) returns an error.

**Partial precision:** Comparing dates or times with different precisions may be **ambiguous**. For example, `@2024 < @2024-06-15` cannot be determined because `@2024` could represent any day in 2024. In this case, the comparison returns empty (signaling ambiguity) rather than an incorrect result.

### Examples

```text
3 < 5              --> true
'apple' < 'banana' --> true   (lexicographic)
@2024-01 > @2023-12 --> true
10 'kg' > 5 'kg'   --> true
{} < 5             --> {}     (empty propagation)
```

## Equality and Equivalence

FHIRPath distinguishes between **equality** and **equivalence**.

### Equality (`=`, `!=`)

| Operator | Name | Description |
|----------|------|-------------|
| `=` | Equals | Strict value comparison |
| `!=` | Not equals | Negation of `=` |

**Empty propagation:** If either operand is empty, `=` returns **empty** (not `false`). This is a critical difference from most programming languages.

```text
5 = 5         --> true
5 = 6         --> false
{} = 5        --> {}      (empty, NOT false)
{} = {}       --> {}      (empty)
5 != 6        --> true
5 != {}       --> {}      (empty)
```

**Singleton evaluation:** Both operands must be singleton collections. If either has more than one element, the result is empty.

### Equivalence (`~`, `!~`)

| Operator | Name | Description |
|----------|------|-------------|
| `~` | Equivalent | Lenient value comparison |
| `!~` | Not equivalent | Negation of `~` |

Equivalence differs from equality in several important ways:

1. **Empty handling:** Two empty collections are **equivalent** (`{} ~ {}` returns `true`). An empty and a non-empty collection are not equivalent (`{} ~ 5` returns `false`). Equivalence never returns empty.
2. **String comparison:** Case-insensitive with normalized whitespace. `'Hello World' ~ 'hello  world'` is `true`.
3. **Quantity comparison:** Uses UCUM normalization. `1000 'mg' ~ 1 'g'` is `true`.

```text
5 ~ 5               --> true
{} ~ {}             --> true   (unlike = which returns {})
{} ~ 5              --> false  (unlike = which returns {})
'Hello' ~ 'hello'   --> true   (case-insensitive)
1000 'mg' ~ 1 'g'   --> true   (UCUM normalization)
```

### Equality vs Equivalence Summary

| Scenario | `=` (Equality) | `~` (Equivalence) |
|----------|:--------------:|:------------------:|
| `5 = 5` / `5 ~ 5` | `true` | `true` |
| `5 = 6` / `5 ~ 6` | `false` | `false` |
| `{} = {}` / `{} ~ {}` | `{}` (empty) | `true` |
| `{} = 5` / `{} ~ 5` | `{}` (empty) | `false` |
| `'Hi' = 'hi'` / `'Hi' ~ 'hi'` | `false` | `true` |
| `1000 'mg' = 1 'g'` / `1000 'mg' ~ 1 'g'` | `true` | `true` |

## Boolean Operators

Boolean operators implement **three-valued logic** where the three states are `true`, `false`, and `{}` (empty/unknown). This is required by the FHIRPath specification to correctly handle missing data in healthcare resources.

### and

Returns `true` only if both operands are `true`.

| `and` | **true** | **false** | **{}** |
|-------|:--------:|:---------:|:------:|
| **true** | `true` | `false` | `{}` |
| **false** | `false` | `false` | `false` |
| **{}** | `{}` | `false` | `{}` |

Key insight: `false and {}` is `false` (not empty), because no matter what the unknown value is, the result must be `false`.

### or

Returns `true` if at least one operand is `true`.

| `or` | **true** | **false** | **{}** |
|------|:--------:|:---------:|:------:|
| **true** | `true` | `true` | `true` |
| **false** | `true` | `false` | `{}` |
| **{}** | `true` | `{}` | `{}` |

Key insight: `true or {}` is `true` (not empty), because no matter what the unknown value is, the result must be `true`.

### xor

Returns `true` if exactly one operand is `true`.

| `xor` | **true** | **false** | **{}** |
|-------|:--------:|:---------:|:------:|
| **true** | `false` | `true` | `{}` |
| **false** | `true` | `false` | `{}` |
| **{}** | `{}` | `{}` | `{}` |

### implies

Logical implication: `A implies B` is equivalent to `(not A) or B`.

| `implies` | **true** | **false** | **{}** |
|-----------|:--------:|:---------:|:------:|
| **true** | `true` | `false` | `{}` |
| **false** | `true` | `true` | `true` |
| **{}** | `true` | `{}` | `{}` |

Key insight: `false implies X` is always `true`, regardless of `X`. This is a standard truth table for material implication.

### not

Unary negation. Returns the logical negation of a singleton Boolean.

| Input | `not` Result |
|-------|:------------:|
| `true` | `false` |
| `false` | `true` |
| `{}` | `{}` |

If the input is not a singleton Boolean, the result is empty.

### Examples

```text
true and false       --> false
true and {}          --> {}
false and {}         --> false   (short-circuit)
true or {}           --> true    (short-circuit)
true xor false       --> true
false implies false   --> true
(not true)           --> false
```

## Collection Operators

| Operator | Name | Description |
|----------|------|-------------|
| `\|` | Union | Returns the union of two collections with duplicates removed |
| `in` | Membership | Returns `true` if the left singleton is in the right collection |
| `contains` | Contains | Returns `true` if the right singleton is in the left collection |

### Union (`|`)

Merges two collections and removes duplicate values. This is the operator form of `Collection.Union()`.

```text
(1 | 2 | 3) | (2 | 3 | 4)  --> (1 | 2 | 3 | 4)
```

### in

Checks if a single value (left) exists in a collection (right). The left operand must be a singleton.

```text
2 in (1 | 2 | 3)     --> true
5 in (1 | 2 | 3)     --> false
{} in (1 | 2 | 3)    --> {}     (empty propagation)
```

### contains

The reverse of `in`. Checks if a collection (left) contains a single value (right). The right operand must be a singleton.

```text
(1 | 2 | 3) contains 2    --> true
(1 | 2 | 3) contains 5    --> false
(1 | 2 | 3) contains {}   --> {}   (empty propagation)
```

## Type Operators

| Operator | Name | Description |
|----------|------|-------------|
| `is` | Type test | Returns `true` if the value is of the given type |
| `as` | Type cast | Returns the value if it is of the given type, otherwise empty |

### is

Tests whether a value is of a specific type:

```text
5 is Integer         --> true
5 is String          --> false
'hello' is String    --> true
@2024-01 is Date     --> true
```

### as

Casts a value to a specific type. If the value is not of that type, returns empty:

```text
5 as Integer         --> 5
5 as String          --> {}
'hello' as String    --> 'hello'
```

The `as` operator is useful in `where` clauses to filter and cast simultaneously.

## String Operators

| Operator | Name | Description |
|----------|------|-------------|
| `&` | Concatenation | Null-safe string concatenation |
| `+` | Addition | String concatenation (empty-propagating) |

The `&` operator differs from `+` in its handling of empty collections. The `+` operator propagates empty (if either side is empty, the result is empty), while `&` treats empty as an empty string:

```text
'Hello' + {}      --> {}            (empty propagation)
'Hello' & {}      --> 'Hello'       (empty treated as '')
{} & {}           --> ''            (both treated as '')
'Hello' & ' ' & 'World'  --> 'Hello World'
```

This makes `&` the preferred operator for building display strings where some parts may be missing.

## Operator Precedence

Operators are listed from **highest** to **lowest** precedence:

| Precedence | Operators | Associativity |
|:----------:|-----------|:-------------:|
| 1 | `.` (path navigation) | Left |
| 2 | `[]` (indexer) | Left |
| 3 | Unary `+`, `-` | Right |
| 4 | `*`, `/`, `div`, `mod` | Left |
| 5 | `+`, `-` | Left |
| 6 | `&` (string concatenation) | Left |
| 7 | `is`, `as` | Left |
| 8 | `\|` (union) | Left |
| 9 | `<`, `>`, `<=`, `>=` | Left |
| 10 | `=`, `!=`, `~`, `!~` | Left |
| 11 | `in`, `contains` | Left |
| 12 | `and` | Left |
| 13 | `xor` | Left |
| 14 | `or` | Left |
| 15 | `implies` | Left |

Use parentheses to override default precedence when needed:

```text
2 + 3 * 4          --> 14     (multiplication first)
(2 + 3) * 4        --> 20     (addition first)
true or false and true  --> true  (and binds tighter than or)
```
