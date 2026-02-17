---
title: "Existence Functions"
linkTitle: "Existence Functions"
weight: 3
description: >
  Functions for testing the existence and properties of elements within collections.
---

Existence functions allow you to test whether collections contain elements, whether those elements meet certain criteria, and to retrieve distinct values. These are fundamental to FHIRPath expressions and are used extensively in FHIR validation and data extraction.

---

## empty

Returns `true` if the input collection is empty, `false` otherwise.

**Signature:**

```text
empty() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.empty()")
// false (patient has at least one name)

result, _ := fhirpath.Evaluate(patient, "Patient.contact.empty()")
// true (if patient has no contacts)

result, _ := fhirpath.Evaluate(resource, "{}.empty()")
// true
```

**Edge Cases / Notes:**

- Always returns `true` or `false`, never an empty collection.
- This is the only existence function guaranteed to return a boolean even for empty input.

---

## exists

Returns `true` if the input collection contains any elements. With an optional criteria expression, returns `true` if any element satisfies the criteria.

**Signature:**

```text
exists([criteria : Expression]) : Boolean
```

**Parameters:**

| Name         | Type         | Description                                                   |
|--------------|--------------|---------------------------------------------------------------|
| `criteria`   | `Expression` | (Optional) A filter expression evaluated for each element     |

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.exists()")
// true (patient has at least one name)

result, _ := fhirpath.Evaluate(patient, "Patient.name.exists(use = 'official')")
// true if any name has use = 'official'

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.exists(system = 'phone')")
// true if patient has a phone telecom entry
```

**Edge Cases / Notes:**

- Without criteria, `exists()` is the inverse of `empty()`.
- With criteria, it is equivalent to `where(criteria).exists()`.
- Returns `false` for an empty input collection.
- The criteria expression is evaluated with `$this` set to each element.

---

## all

Returns `true` if **all** elements in the collection satisfy the given criteria expression. Returns `true` for an empty collection (vacuous truth).

**Signature:**

```text
all(criteria : Expression) : Boolean
```

**Parameters:**

| Name         | Type         | Description                                                      |
|--------------|--------------|------------------------------------------------------------------|
| `criteria`   | `Expression` | A filter expression that must be true for every element          |

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.all(use.exists())")
// true if every name entry has a 'use' field

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.all(system = 'phone')")
// true only if every telecom entry is a phone

result, _ := fhirpath.Evaluate(patient, "Patient.contact.all(name.exists())")
// true if patient has no contacts (vacuous truth)
```

**Edge Cases / Notes:**

- An empty collection returns `true` (vacuous truth per FHIRPath specification).
- The criteria expression is evaluated with `$this` set to each element.
- This function is commonly used in FHIR invariant definitions.

---

## allTrue

Returns `true` if all items in the collection are boolean `true`.

**Signature:**

```text
allTrue() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "Patient.active.allTrue()")
// true if active is true

result, _ := fhirpath.Evaluate(resource, "(true | true | true).allTrue()")
// true

result, _ := fhirpath.Evaluate(resource, "(true | false | true).allTrue()")
// false
```

**Edge Cases / Notes:**

- An empty collection returns `true` (vacuous truth).
- Non-boolean elements cause the function to return `false`.
- Commonly used after mapping a collection to boolean values.

---

## anyTrue

Returns `true` if **any** item in the collection is boolean `true`.

**Signature:**

```text
anyTrue() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(true | false | false).anyTrue()")
// true

result, _ := fhirpath.Evaluate(resource, "(false | false).anyTrue()")
// false

result, _ := fhirpath.Evaluate(resource, "{}.anyTrue()")
// false
```

**Edge Cases / Notes:**

- An empty collection returns `false`.
- Returns `true` as soon as one boolean `true` is found.

---

## allFalse

Returns `true` if all items in the collection are boolean `false`.

**Signature:**

```text
allFalse() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(false | false | false).allFalse()")
// true

result, _ := fhirpath.Evaluate(resource, "(false | true | false).allFalse()")
// false

result, _ := fhirpath.Evaluate(resource, "{}.allFalse()")
// true (vacuous truth)
```

**Edge Cases / Notes:**

- An empty collection returns `true` (vacuous truth).
- Non-boolean elements cause the function to return `false`.

---

## anyFalse

Returns `true` if **any** item in the collection is boolean `false`.

**Signature:**

```text
anyFalse() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(true | false | true).anyFalse()")
// true

result, _ := fhirpath.Evaluate(resource, "(true | true).anyFalse()")
// false

result, _ := fhirpath.Evaluate(resource, "{}.anyFalse()")
// false
```

**Edge Cases / Notes:**

- An empty collection returns `false`.
- Returns `true` as soon as one boolean `false` is found.

---

## count

Returns the number of items in the input collection.

**Signature:**

```text
count() : Integer
```

**Return Type:** `Integer`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.count()")
// Number of name entries (e.g., 2)

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.count()")
// Number of telecom entries

result, _ := fhirpath.Evaluate(resource, "{}.count()")
// 0
```

**Edge Cases / Notes:**

- Always returns a non-negative integer, never an empty collection.
- An empty collection returns `0`.

---

## distinct

Returns a collection containing only the distinct (unique) elements from the input.

**Signature:**

```text
distinct() : Collection
```

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 2 | 3 | 3 | 3).distinct()")
// { 1, 2, 3 }

result, _ := fhirpath.Evaluate(resource, "('a' | 'b' | 'a').distinct()")
// { 'a', 'b' }

result, _ := fhirpath.Evaluate(resource, "{}.distinct()")
// { } (empty)
```

**Edge Cases / Notes:**

- The order of elements in the result is implementation-dependent.
- Element equality is determined by FHIRPath's equality rules.
- An empty collection returns an empty collection.

---

## isDistinct

Returns `true` if all elements in the collection are distinct (no duplicates).

**Signature:**

```text
isDistinct() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).isDistinct()")
// true

result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 2).isDistinct()")
// false

result, _ := fhirpath.Evaluate(resource, "{}.isDistinct()")
// true (empty collection is trivially distinct)
```

**Edge Cases / Notes:**

- Equivalent to `count() = distinct().count()`.
- An empty collection returns `true`.

---

## subsetOf

Returns `true` if all elements in the input collection are also present in the other collection.

**Signature:**

```text
subsetOf(other : Collection) : Boolean
```

**Parameters:**

| Name      | Type           | Description                         |
|-----------|----------------|-------------------------------------|
| `other`   | `Collection`   | The collection to check against     |

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2).subsetOf(1 | 2 | 3)")
// true

result, _ := fhirpath.Evaluate(resource, "(1 | 4).subsetOf(1 | 2 | 3)")
// false

result, _ := fhirpath.Evaluate(resource, "{}.subsetOf(1 | 2)")
// true (empty set is a subset of any set)
```

**Edge Cases / Notes:**

- An empty input collection is always a subset of any collection.
- Element comparison follows FHIRPath's equality rules.

---

## supersetOf

Returns `true` if all elements in the other collection are also present in the input collection.

**Signature:**

```text
supersetOf(other : Collection) : Boolean
```

**Parameters:**

| Name      | Type           | Description                         |
|-----------|----------------|-------------------------------------|
| `other`   | `Collection`   | The collection to check against     |

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).supersetOf(1 | 2)")
// true

result, _ := fhirpath.Evaluate(resource, "(1 | 2).supersetOf(1 | 2 | 3)")
// false

result, _ := fhirpath.Evaluate(resource, "(1 | 2).supersetOf({})")
// true (any set is a superset of the empty set)
```

**Edge Cases / Notes:**

- `a.supersetOf(b)` is equivalent to `b.subsetOf(a)`.
- Any collection is a superset of the empty collection.
