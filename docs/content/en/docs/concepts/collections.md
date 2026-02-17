---
title: "Collections"
linkTitle: "Collections"
description: "How FHIRPath represents results as ordered collections, the rules for empty propagation and three-valued logic, singleton evaluation, and the full set of collection operations."
weight: 2
---

In FHIRPath, **every expression evaluates to a collection**. A collection is an ordered list of zero or more values. There are no standalone scalar values -- even a single Boolean `true` is represented as a one-element collection containing that Boolean.

In the FHIRPath Go library, a collection is defined as:

```go
type Collection []Value
```

This is the fundamental return type for all FHIRPath expressions.

## Empty Collections

An empty collection (`{}` in FHIRPath notation, `Collection{}` or `nil` in Go) represents the absence of a value. Navigating to a path that does not exist in a resource always produces an empty collection rather than an error.

```go
result, _ := fhirpath.Evaluate(patientJSON, "Patient.deceased")
if result.Empty() {
    fmt.Println("No deceased value present")
}
```

## Empty Propagation (Three-Valued Logic)

FHIRPath uses **three-valued logic** where the three states are `true`, `false`, and **empty** (unknown). When an operand to most operators or functions is an empty collection, the result propagates as empty rather than producing an error.

For example:

```text
{} = 5        --> {}      (empty, not false)
{} and true   --> {}      (empty, not false)
{} + 3        --> {}      (empty, not an error)
```

This is different from many programming languages where a null or missing value would cause an error. FHIRPath is designed for healthcare data where missing values are common and expected.

The Boolean operators (`and`, `or`, `implies`) have special propagation rules. For instance, `false and {}` evaluates to `false` (not empty), because regardless of the unknown value, the result must be `false`. See the [Operators]({{< relref "operators" >}}) page for complete three-valued truth tables.

## Singleton Evaluation

Many operators (such as `=`, `<`, `+`) expect **singleton** collections (collections with exactly one element). When these operators receive a collection, FHIRPath applies **singleton evaluation**:

- If the collection has exactly **one** element, that element is used as the operand.
- If the collection is **empty**, the result is empty (per empty propagation).
- If the collection has **more than one** element, the behavior depends on the operator -- most return empty or raise an error.

```go
// Single-element collection: works as expected
result, _ := fhirpath.Evaluate(patientJSON, "Patient.birthDate = @1990-05-15")
// result: [true]

// Multi-element collection on one side: empty result
result, _ = fhirpath.Evaluate(patientJSON, "Patient.name.given = 'John'")
// If Patient has multiple given names, this may return empty
```

## Collection Methods

The `Collection` type provides a rich set of methods for working with results in Go code.

### Basic Access

| Method | Signature | Description |
|--------|-----------|-------------|
| `Empty()` | `func (c Collection) Empty() bool` | Returns `true` if the collection has no elements. |
| `Count()` | `func (c Collection) Count() int` | Returns the number of elements. |
| `First()` | `func (c Collection) First() (Value, bool)` | Returns the first element and `true`, or `nil` and `false` if empty. |
| `Last()` | `func (c Collection) Last() (Value, bool)` | Returns the last element and `true`, or `nil` and `false` if empty. |
| `Single()` | `func (c Collection) Single() (Value, error)` | Returns the sole element. Errors if empty or more than one element. |

**Examples:**

```go
result, _ := fhirpath.Evaluate(patientJSON, "Patient.name.given")

if result.Empty() {
    fmt.Println("No given names found")
}

fmt.Println("Count:", result.Count())

if first, ok := result.First(); ok {
    fmt.Println("First:", first)
}

if last, ok := result.Last(); ok {
    fmt.Println("Last:", last)
}
```

### Subsetting

| Method | Signature | Description |
|--------|-----------|-------------|
| `Tail()` | `func (c Collection) Tail() Collection` | Returns all elements except the first. |
| `Skip(n)` | `func (c Collection) Skip(n int) Collection` | Returns a collection with the first `n` elements removed. |
| `Take(n)` | `func (c Collection) Take(n int) Collection` | Returns a collection with only the first `n` elements. |

**Examples:**

```go
result, _ := fhirpath.Evaluate(patientJSON, "Patient.name")

// Get everything after the first name
rest := result.Tail()

// Pagination-style operations
page := result.Skip(10).Take(5) // elements 11-15
```

### Set Operations

| Method | Signature | Description |
|--------|-----------|-------------|
| `Union(other)` | `func (c Collection) Union(other Collection) Collection` | Returns the union of both collections with duplicates removed. |
| `Combine(other)` | `func (c Collection) Combine(other Collection) Collection` | Concatenates both collections, preserving duplicates. |
| `Intersect(other)` | `func (c Collection) Intersect(other Collection) Collection` | Returns elements present in both collections. |
| `Exclude(other)` | `func (c Collection) Exclude(other Collection) Collection` | Returns elements in `c` that are not in `other`. |
| `Distinct()` | `func (c Collection) Distinct() Collection` | Returns a new collection with duplicates removed, preserving order of first occurrence. |

**Examples:**

```go
a, _ := fhirpath.Evaluate(patientJSON, "Patient.name.given")
b, _ := fhirpath.Evaluate(patientJSON, "Patient.contact.name.given")

// All unique given names from patient and contacts
all := a.Union(b)

// All given names including duplicates
combined := a.Combine(b)

// Given names that appear in both
shared := a.Intersect(b)

// Given names only on the patient (not contacts)
patientOnly := a.Exclude(b)

// Remove duplicates from a single collection
unique := a.Distinct()
```

The `Union` and `Combine` distinction is important:
- **Union** (`|` in FHIRPath) merges two collections and removes duplicates.
- **Combine** concatenates two collections and preserves duplicates.

### Boolean Aggregation

These methods evaluate collections of Boolean values:

| Method | Signature | Description |
|--------|-----------|-------------|
| `AllTrue()` | `func (c Collection) AllTrue() bool` | Returns `true` if every element is Boolean `true`. |
| `AnyTrue()` | `func (c Collection) AnyTrue() bool` | Returns `true` if at least one element is Boolean `true`. |
| `AllFalse()` | `func (c Collection) AllFalse() bool` | Returns `true` if every element is Boolean `false`. |
| `AnyFalse()` | `func (c Collection) AnyFalse() bool` | Returns `true` if at least one element is Boolean `false`. |
| `ToBoolean()` | `func (c Collection) ToBoolean() (bool, error)` | Converts a singleton Boolean collection to a Go `bool`. Errors if empty, multi-valued, or not a Boolean. |

**Examples:**

```go
// Check if all validation results are true
results, _ := fhirpath.Evaluate(bundleJSON, "Bundle.entry.resource.active")
if results.AllTrue() {
    fmt.Println("All resources are active")
}

// Extract a single boolean
isActive, _ := fhirpath.Evaluate(patientJSON, "Patient.active")
if active, err := isActive.ToBoolean(); err == nil {
    fmt.Println("Active:", active)
}
```

### Membership

| Method | Signature | Description |
|--------|-----------|-------------|
| `Contains(v)` | `func (c Collection) Contains(v Value) bool` | Returns `true` if the collection contains a value equal to `v`. |
| `IsDistinct()` | `func (c Collection) IsDistinct() bool` | Returns `true` if all elements in the collection are unique. |

**Example:**

```go
names, _ := fhirpath.Evaluate(patientJSON, "Patient.name.given")
if names.Contains(types.NewString("John")) {
    fmt.Println("Patient has given name John")
}
```

## Working with Collections in Go

A common pattern is to iterate over a collection and type-assert each value:

```go
result, _ := fhirpath.Evaluate(patientJSON, "Patient.name.given")
for _, val := range result {
    if s, ok := val.(types.String); ok {
        fmt.Println("Given name:", s.Value())
    }
}
```

For single-value results, use `First()` or `Single()`:

```go
result, _ := fhirpath.Evaluate(patientJSON, "Patient.birthDate")
if val, ok := result.First(); ok {
    if d, ok := val.(types.Date); ok {
        fmt.Println("Birth year:", d.Year())
    }
}
```

Or use the convenience functions for the most common patterns:

```go
// These handle collection unwrapping for you
family, _ := fhirpath.EvaluateToString(patientJSON, "Patient.name.first().family")
active, _ := fhirpath.EvaluateToBoolean(patientJSON, "Patient.active")
names, _  := fhirpath.EvaluateToStrings(patientJSON, "Patient.name.given")
exists, _ := fhirpath.Exists(patientJSON, "Patient.telecom")
count, _  := fhirpath.Count(patientJSON, "Patient.name")
```
