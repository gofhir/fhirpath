---
title: "Subsetting Functions"
linkTitle: "Subsetting Functions"
weight: 5
description: >
  Functions for extracting subsets of elements from collections in FHIRPath expressions.
---

Subsetting functions allow you to select specific elements or ranges of elements from a collection. They are essential for navigating ordered FHIR® data such as name entries, telecom contacts, or list resources.

---

## first

Returns a collection containing only the first element of the input collection.

**Signature:**

```text
first() : Collection
```

**Return Type:** `Collection` (containing at most one element)

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.first()")
// Returns the first name entry

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family")
// Returns the family name from the first name entry

result, _ := fhirpath.Evaluate(resource, "{}.first()")
// { } (empty collection)
```

**Edge Cases / Notes:**

- Returns an empty collection if the input is empty.
- Equivalent to `take(1)`.
- The result is still a collection (with zero or one elements), not a scalar.

---

## last

Returns a collection containing only the last element of the input collection.

**Signature:**

```text
last() : Collection
```

**Return Type:** `Collection` (containing at most one element)

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.last()")
// Returns the last name entry

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.last().value")
// Returns the value from the last telecom entry

result, _ := fhirpath.Evaluate(resource, "{}.last()")
// { } (empty collection)
```

**Edge Cases / Notes:**

- Returns an empty collection if the input is empty.
- The result is still a collection (with zero or one elements), not a scalar.

---

## tail

Returns all elements except the first. Equivalent to `skip(1)`.

**Signature:**

```text
tail() : Collection
```

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.tail()")
// Returns all name entries except the first

result, _ := fhirpath.Evaluate(patient, "Patient.name.tail().count()")
// Number of names minus 1

result, _ := fhirpath.Evaluate(resource, "{}.tail()")
// { } (empty collection)
```

**Edge Cases / Notes:**

- Returns an empty collection if the input has zero or one elements.
- Equivalent to `skip(1)`.

---

## take

Returns the first `n` elements of the input collection.

**Signature:**

```text
take(num : Integer) : Collection
```

**Parameters:**

| Name    | Type      | Description                                            |
|---------|-----------|--------------------------------------------------------|
| `num`   | `Integer` | The number of elements to take from the beginning      |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.take(2)")
// Returns the first 2 name entries

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.take(1)")
// Equivalent to first()

result, _ := fhirpath.Evaluate(patient, "Patient.name.take(100)")
// Returns all names (if fewer than 100 exist)
```

**Edge Cases / Notes:**

- If `n` is greater than the collection size, all elements are returned.
- If `n` is zero or negative, returns an empty collection.
- Returns an empty collection if the input is empty.

---

## skip

Returns all elements except the first `n` elements.

**Signature:**

```text
skip(num : Integer) : Collection
```

**Parameters:**

| Name    | Type      | Description                                             |
|---------|-----------|---------------------------------------------------------|
| `num`   | `Integer` | The number of elements to skip from the beginning       |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.skip(1)")
// Equivalent to tail() - skips the first name

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.skip(2)")
// Returns all telecom entries after the second

result, _ := fhirpath.Evaluate(patient, "Patient.name.skip(0)")
// Returns all names (skip nothing)
```

**Edge Cases / Notes:**

- If `n` is greater than or equal to the collection size, returns an empty collection.
- If `n` is zero or negative, all elements are returned.
- Returns an empty collection if the input is empty.

---

## single

Returns the single element from the input collection. If the collection does not contain exactly one element, an error is raised.

**Signature:**

```text
single() : Collection
```

**Return Type:** `Collection` (containing exactly one element)

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.single()")
// Returns the birth date (exactly one)

result, _ := fhirpath.Evaluate(patient, "Patient.active.single()")
// Returns the active flag (exactly one)

result, err := fhirpath.Evaluate(patient, "Patient.name.single()")
// Error if patient has more than one name entry
```

**Edge Cases / Notes:**

- Returns an error of type `ErrSingletonExpected` if the collection contains zero or more than one element.
- Use this function when you expect exactly one result and want to enforce that constraint.
- This is stricter than `first()`, which silently returns empty or the first element.

---

## intersect

Returns the set intersection of the input collection and another collection -- elements that appear in both.

**Signature:**

```text
intersect(other : Collection) : Collection
```

**Parameters:**

| Name      | Type           | Description                           |
|-----------|----------------|---------------------------------------|
| `other`   | `Collection`   | The collection to intersect with      |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).intersect(2 | 3 | 4)")
// { 2, 3 }

result, _ := fhirpath.Evaluate(resource, "(1 | 2).intersect(3 | 4)")
// { } (no common elements)

result, _ := fhirpath.Evaluate(resource, "('a' | 'b' | 'c').intersect('b' | 'd')")
// { 'b' }
```

**Edge Cases / Notes:**

- The result contains no duplicates (set semantics).
- Element equality is determined by FHIRPath's equality rules.
- Returns an empty collection if either input is empty or there are no common elements.

---

## exclude

Returns elements from the input collection that are **not** in the other collection.

**Signature:**

```text
exclude(other : Collection) : Collection
```

**Parameters:**

| Name      | Type           | Description                              |
|-----------|----------------|------------------------------------------|
| `other`   | `Collection`   | The collection of elements to exclude    |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).exclude(2)")
// { 1, 3 }

result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).exclude(4 | 5)")
// { 1, 2, 3 } (nothing to exclude)

result, _ := fhirpath.Evaluate(resource, "('a' | 'b' | 'c').exclude('a' | 'c')")
// { 'b' }
```

**Edge Cases / Notes:**

- This is the set difference operation: `input - other`.
- Element equality is determined by FHIRPath's equality rules.
- If the other collection is empty, the input is returned unchanged.
- Returns an empty collection if the input is empty.
