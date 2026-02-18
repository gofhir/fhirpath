---
title: "Filtering Functions"
linkTitle: "Filtering Functions"
weight: 4
description: >
  Functions for filtering, projecting, and recursively navigating collections in FHIRPath expressions.
---

Filtering functions allow you to narrow down collections based on criteria, project elements to extract specific properties, and recursively navigate through resource structures. These are among the most commonly used functions in FHIRPath.

---

## where

Filters the input collection, returning only elements where the criteria expression evaluates to `true`.

**Signature:**

```text
where(criteria : Expression) : Collection
```

**Parameters:**

| Name         | Type         | Description                                                                                                    |
|--------------|--------------|----------------------------------------------------------------------------------------------------------------|
| `criteria`   | `Expression` | A boolean expression evaluated for each element. Within the expression, `$this` refers to the current element  |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.where(use = 'official')")
// Returns only name entries where use is 'official'

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.where(system = 'phone' and use = 'home')")
// Returns telecom entries that are home phone numbers

result, _ := fhirpath.Evaluate(patient, "Patient.name.where(given.exists())")
// Returns name entries that have at least one given name
```

**Edge Cases / Notes:**

- The criteria expression is evaluated with `$this` set to each element in the input collection.
- If the criteria evaluates to an empty collection or `false` for an element, that element is excluded.
- An empty input collection returns an empty collection.
- Unlike `select`, `where` preserves the original elements -- it does not transform them.

---

## select

Projects each element of the input collection through an expression, returning the flattened results.

**Signature:**

```text
select(projection : Expression) : Collection
```

**Parameters:**

| Name           | Type         | Description                                                                                              |
|----------------|--------------|----------------------------------------------------------------------------------------------------------|
| `projection`   | `Expression` | An expression evaluated for each element. Within the expression, `$this` refers to the current element   |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.telecom.select(value)")
// Returns just the values from each telecom entry

result, _ := fhirpath.Evaluate(patient, "Patient.name.select(given)")
// Returns all given names (flattened from all name entries)

result, _ := fhirpath.Evaluate(patient, "Patient.name.select(family + ', ' + given.first())")
// Returns formatted names like "Smith, John"
```

**Edge Cases / Notes:**

- The results from all elements are **flattened** into a single collection. If each element produces a collection, all items are merged into one.
- An empty input collection returns an empty collection.
- `select` transforms elements, while `where` filters them. Use `select` to extract or compute values.
- The projection expression is evaluated with `$this` set to each element.

---

## repeat

Repeatedly applies an expression to the input collection and its results, collecting all results until no new elements are produced. This enables recursive navigation through hierarchical data.

**Signature:**

```text
repeat(expression : Expression) : Collection
```

**Parameters:**

| Name           | Type         | Description                                      |
|----------------|--------------|--------------------------------------------------|
| `expression`   | `Expression` | An expression that is applied recursively        |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "QuestionnaireResponse.item.repeat(item)")
// Recursively collects all nested items at every level

result, _ := fhirpath.Evaluate(resource, "Observation.component.repeat(component)")
// Recursively navigates nested components

result, _ := fhirpath.Evaluate(resource, "ValueSet.expansion.contains.repeat(contains)")
// Navigates the full hierarchy of a ValueSet expansion
```

**Edge Cases / Notes:**

- The expression is applied to the input, then to the results, and so on until no new elements are produced.
- Duplicate detection prevents infinite loops in cyclic structures.
- An empty input collection returns an empty collection.
- The results include all intermediate results, not just the final level.
- This function requires special handling in the evaluator for proper recursive evaluation.

---

## ofType

Filters the input collection, returning only elements that are of the specified type.

**Signature:**

```text
ofType(type : TypeSpecifier) : Collection
```

**Parameters:**

| Name     | Type              | Description                                                                    |
|----------|-------------------|--------------------------------------------------------------------------------|
| `type`   | `TypeSpecifier`   | The FHIR® type name to filter by (e.g., `Quantity`, `String`, `HumanName`)      |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(observation, "Observation.value.ofType(Quantity)")
// Returns value only if it is a Quantity type

result, _ := fhirpath.Evaluate(observation, "Observation.value.ofType(CodeableConcept)")
// Returns value only if it is a CodeableConcept type

result, _ := fhirpath.Evaluate(resource, "Bundle.entry.resource.ofType(Patient)")
// Returns only Patient resources from a Bundle
```

**Edge Cases / Notes:**

- This function is particularly useful for polymorphic FHIR® elements (e.g., `value[x]`).
- Type matching compares the element's runtime type against the specified type name.
- An empty input collection returns an empty collection.
- This function is also listed under [Type Checking Functions]({{< relref "type-checking" >}}) since it serves a dual purpose.
- Unlike `as()`, `ofType()` works on collections with multiple elements and never errors.
