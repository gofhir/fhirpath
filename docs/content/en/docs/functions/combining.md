---
title: "Combining Functions"
linkTitle: "Combining Functions"
weight: 6
description: >
  Functions for merging two collections together in FHIRPath expressions.
---

Combining functions allow you to merge two collections. The key difference between the two available functions is how they handle duplicates: `union` produces a set (no duplicates), while `combine` preserves all elements including duplicates.

---

## union

Returns the set union of the input collection and another collection. Duplicate elements are removed from the result.

**Signature:**

```text
union(other : Collection) : Collection
```

**Parameters:**

| Name      | Type           | Description                    |
|-----------|----------------|--------------------------------|
| `other`   | `Collection`   | The collection to merge with   |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).union(3 | 4 | 5)")
// { 1, 2, 3, 4, 5 } (duplicates removed)

result, _ := fhirpath.Evaluate(resource, "('a' | 'b').union('c' | 'd')")
// { 'a', 'b', 'c', 'd' }

result, _ := fhirpath.Evaluate(resource, "(1 | 2).union(1 | 2)")
// { 1, 2 } (identical sets)
```

**Edge Cases / Notes:**

- The result is a set with no duplicate elements.
- Element equality is determined by FHIRPath's equality rules.
- The `|` operator in FHIRPath is equivalent to calling `union`. For example, `a | b` is the same as `a.union(b)`.
- If either collection is empty, the result is the other collection (with duplicates removed).
- The order of elements in the result is implementation-dependent.
- Uses the `Collection.Union` method internally, which handles deduplication.

---

## combine

Returns the concatenation of the input collection and another collection. Unlike `union`, duplicates are preserved.

**Signature:**

```text
combine(other : Collection) : Collection
```

**Parameters:**

| Name      | Type           | Description                          |
|-----------|----------------|--------------------------------------|
| `other`   | `Collection`   | The collection to concatenate with   |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).combine(3 | 4 | 5)")
// { 1, 2, 3, 3, 4, 5 } (duplicates preserved)

result, _ := fhirpath.Evaluate(resource, "('a' | 'b').combine('b' | 'c')")
// { 'a', 'b', 'b', 'c' }

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().given.combine(Patient.name.last().given)")
// Combines given names from first and last name entries
```

**Edge Cases / Notes:**

- Unlike `union`, `combine` does **not** remove duplicates. It is a simple concatenation.
- The order of elements is preserved: all elements from the input come first, followed by all elements from the other collection.
- If either collection is empty, the result is the other collection.
- Use `combine` when you need to preserve duplicates (e.g., for counting or aggregating). Use `union` when you need set semantics.

---

## Comparison: union vs. combine

| Feature             | `union`                              | `combine`                        |
|---------------------|--------------------------------------|----------------------------------|
| Duplicates          | Removed                              | Preserved                        |
| Set semantics       | Yes                                  | No                               |
| Equivalent operator | `\|`                                 | None                             |
| Use case            | Set operations, deduplication        | Concatenation, aggregation       |

**Example illustrating the difference:**

```go
// Given collections: {1, 2, 3} and {2, 3, 4}

// union removes duplicates
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).union(2 | 3 | 4)")
// { 1, 2, 3, 4 }

// combine preserves duplicates
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).combine(2 | 3 | 4)")
// { 1, 2, 3, 2, 3, 4 }
```
