---
title: "Aggregate Functions"
linkTitle: "Aggregate Functions"
weight: 12
description: >
  Functions for reducing collections to single values through aggregation, summation, averaging, and finding extremes.
---

Aggregate functions reduce a collection of values to a single result. They are essential for performing calculations across multiple elements, such as summing quantities, computing averages, or finding minimum and maximum values.

---

## aggregate

Performs a general-purpose aggregation (fold/reduce) over the input collection. This is the most flexible aggregation function, allowing custom accumulation logic.

**Signature:**
```
aggregate(aggregator : Expression [, init : Value]) : Value
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `aggregator` | `Expression` | An expression evaluated for each element. Within the expression, `$this` refers to the current element and `$total` refers to the accumulated value |
| `init` | `Value` | (Optional) The initial value for `$total`. Defaults to empty collection |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3 | 4).aggregate($total + $this, 0)")
// 10 (sum: 0 + 1 + 2 + 3 + 4)

result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3 | 4).aggregate($total * $this, 1)")
// 24 (product: 1 * 1 * 2 * 3 * 4)

result, _ := fhirpath.Evaluate(resource, "('a' | 'b' | 'c').aggregate($total + $this, '')")
// 'abc' (string concatenation)
```

**Edge Cases / Notes:**
- The `aggregator` expression has access to two special variables:
  - `$this` -- the current element being processed.
  - `$total` -- the accumulated result so far.
- If no `init` value is provided, `$total` starts as an empty collection.
- This function requires special handling in the evaluator for proper lambda/expression support.
- This is equivalent to a functional `fold` or `reduce` operation.
- Returns the `init` value (or empty) if the input collection is empty.

---

## sum

Returns the sum of all numeric values in the input collection.

**Signature:**
```
sum() : Integer | Decimal
```

**Return Type:** `Integer` if all values are `Integer`, `Decimal` if any value is `Decimal`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3 | 4).sum()")
// 10 (Integer)

result, _ := fhirpath.Evaluate(resource, "(1.5 | 2.5 | 3.0).sum()")
// 7.0 (Decimal)

result, _ := fhirpath.Evaluate(resource, "{}.sum()")
// 0 (empty collection sums to 0)
```

**Edge Cases / Notes:**
- An empty collection returns `0` (as an `Integer`).
- If all elements are `Integer`, the result is `Integer`. If any element is `Decimal`, the result is `Decimal`.
- Returns an empty collection if any element is non-numeric (per FHIRPath specification).
- Supports cancellation for large collections via context checking.
- Uses precise decimal arithmetic via the `shopspring/decimal` library.

---

## avg

Returns the arithmetic mean (average) of all numeric values in the input collection.

**Signature:**
```
avg() : Decimal
```

**Return Type:** `Decimal`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3 | 4).avg()")
// 2.5

result, _ := fhirpath.Evaluate(resource, "(10 | 20 | 30).avg()")
// 20.0

result, _ := fhirpath.Evaluate(resource, "(5).avg()")
// 5.0 (single element)
```

**Edge Cases / Notes:**
- Always returns a `Decimal`, even if all inputs are `Integer`.
- Returns an empty collection if the input is empty.
- Returns an empty collection if any element is non-numeric.
- Computed as `sum() / count()` using precise decimal arithmetic.
- Supports cancellation for large collections.

---

## min

Returns the minimum value from the input collection. Works with numeric types, strings, dates, date-times, and times.

**Signature:**
```
min() : Value
```

**Return Type:** The same type as the input elements

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(3 | 1 | 4 | 1 | 5).min()")
// 1

result, _ := fhirpath.Evaluate(resource, "('cherry' | 'apple' | 'banana').min()")
// 'apple' (lexicographic comparison)

result, _ := fhirpath.Evaluate(resource, "(@2024-01-01 | @2024-06-15 | @2024-03-20).min()")
// @2024-01-01
```

**Edge Cases / Notes:**
- Returns an empty collection if the input is empty.
- Returns an empty collection if the collection contains unsupported types.
- Supported types for comparison:
  - `Integer` and `Decimal` (numeric comparison)
  - `String` (lexicographic comparison)
  - `Date` (chronological comparison)
  - `DateTime` (chronological comparison)
  - `Time` (chronological comparison)
- All elements in the collection should be of the same type for meaningful results.
- Supports cancellation for large collections.

---

## max

Returns the maximum value from the input collection. Works with numeric types, strings, dates, date-times, and times.

**Signature:**
```
max() : Value
```

**Return Type:** The same type as the input elements

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(3 | 1 | 4 | 1 | 5).max()")
// 5

result, _ := fhirpath.Evaluate(resource, "('cherry' | 'apple' | 'banana').max()")
// 'cherry' (lexicographic comparison)

result, _ := fhirpath.Evaluate(resource, "(@2024-01-01 | @2024-06-15 | @2024-03-20).max()")
// @2024-06-15
```

**Edge Cases / Notes:**
- Returns an empty collection if the input is empty.
- Returns an empty collection if the collection contains unsupported types.
- Supported types are the same as `min()`: `Integer`, `Decimal`, `String`, `Date`, `DateTime`, `Time`.
- All elements in the collection should be of the same type for meaningful results.
- Supports cancellation for large collections.

---

## Comparison of Aggregate Functions

| Function | Input | Empty Collection | Non-Numeric Elements | Return Type |
|----------|-------|------------------|---------------------|-------------|
| `sum()` | Numeric | `0` | Empty collection | `Integer` or `Decimal` |
| `avg()` | Numeric | Empty collection | Empty collection | `Decimal` |
| `min()` | Any comparable | Empty collection | Empty collection | Same as input |
| `max()` | Any comparable | Empty collection | Empty collection | Same as input |
| `aggregate()` | Any | `init` value | N/A (custom logic) | Any |

### Using aggregate for Custom Calculations

The `aggregate` function can express any reduction that `sum`, `avg`, `min`, or `max` perform, plus custom logic:

```go
// Custom: running maximum
result, _ := fhirpath.Evaluate(resource,
    "(3 | 1 | 4 | 1 | 5).aggregate(iif($this > $total, $this, $total), 0)")
// 5

// Custom: count of values greater than 2
result, _ := fhirpath.Evaluate(resource,
    "(3 | 1 | 4 | 1 | 5).aggregate(iif($this > 2, $total + 1, $total), 0)")
// 3
```
