---
title: "Type Checking Functions"
linkTitle: "Type Checking Functions"
weight: 8
description: >
  Functions for inspecting and casting element types in FHIRPath expressions.
---

Type checking functions allow you to test an element's type and to cast elements to specific types. These are essential when working with polymorphic FHIR® elements (like `value[x]`) where the actual type may vary.

---

## is

Returns `true` if the input element is of the specified type.

**Signature:**

```text
is(type : TypeSpecifier) : Boolean
```

**Parameters:**

| Name     | Type              | Description                                                                         |
|----------|-------------------|-------------------------------------------------------------------------------------|
| `type`   | `TypeSpecifier`   | The FHIR® type name to check against (e.g., `Quantity`, `String`, `Patient`)         |

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(observation, "Observation.value.is(Quantity)")
// true if value is a Quantity

result, _ := fhirpath.Evaluate(observation, "Observation.value.is(CodeableConcept)")
// true if value is a CodeableConcept

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().is(HumanName)")
// true
```

**Edge Cases / Notes:**

- Requires a singleton input (exactly one element). If the input contains more than one element, an error is raised.
- Returns empty collection if the input is empty.
- The type name is resolved by the evaluator directly from the expression AST, so type names like `Patient` or `Quantity` are used without quotes.
- Type matching uses the `eval.TypeMatches` function, which supports both simple type names and fully qualified FHIR® type names.
- The function form `value.is(Quantity)` is equivalent to the operator form `value is Quantity`.

---

## as

Casts the input to the specified type. Returns the input if it matches the type, otherwise returns an empty collection.

**Signature:**

```text
as(type : TypeSpecifier) : Collection
```

**Parameters:**

| Name     | Type              | Description                                                                         |
|----------|-------------------|-------------------------------------------------------------------------------------|
| `type`   | `TypeSpecifier`   | The FHIR® type name to cast to (e.g., `Quantity`, `String`, `Patient`)               |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(observation, "Observation.value.as(Quantity)")
// Returns the value as a Quantity, or empty if not a Quantity

result, _ := fhirpath.Evaluate(observation, "Observation.value.as(Quantity).value")
// Accesses the numeric value if value[x] is a Quantity

result, _ := fhirpath.Evaluate(resource, "Bundle.entry.resource.as(Patient)")
// Returns only entries that are Patient resources
```

**Edge Cases / Notes:**

- Returns empty collection if the input is empty.
- Returns empty collection if none of the elements match the specified type.
- Unlike `is()`, the `as()` function works on collections with multiple elements -- it filters and returns only matching elements.
- The function form `value.as(Quantity)` is equivalent to the operator form `value as Quantity`.
- The type name is typically handled specially by the evaluator, extracting it directly from the AST.

---

## ofType

Filters the input collection, returning only elements that are of the specified type. This function is identical in behavior to `as()` but is the preferred form when filtering collections.

**Signature:**

```text
ofType(type : TypeSpecifier) : Collection
```

**Parameters:**

| Name     | Type              | Description                                                                         |
|----------|-------------------|-------------------------------------------------------------------------------------|
| `type`   | `TypeSpecifier`   | The FHIR® type name to filter by (e.g., `Quantity`, `String`, `HumanName`)           |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(observation, "Observation.value.ofType(Quantity)")
// Returns value only if it is a Quantity

result, _ := fhirpath.Evaluate(resource, "Bundle.entry.resource.ofType(Patient)")
// Returns only Patient resources from a Bundle

result, _ := fhirpath.Evaluate(resource, "Bundle.entry.resource.ofType(Observation).status")
// Gets status from all Observation resources in a Bundle
```

**Edge Cases / Notes:**

- This function is also documented under [Filtering Functions]({{< relref "filtering" >}}) since it filters collections by type.
- Type matching compares the element's runtime type name against the specified type name.
- Returns an empty collection if no elements match the type.
- Unlike `is()`, `ofType()` works on collections with multiple elements and never errors on non-singleton input.

---

## Comparison: is vs. as vs. ofType

| Function      | Input        | Returns        | Use Case                                              |
|---------------|--------------|----------------|-------------------------------------------------------|
| `is(T)`       | Singleton    | `Boolean`      | Testing if a single value is a specific type          |
| `as(T)`       | Collection   | `Collection`   | Casting / filtering a collection to a type            |
| `ofType(T)`   | Collection   | `Collection`   | Filtering a collection to elements of a type          |

**Example illustrating the differences:**

```go
// is() -- returns a boolean
result, _ := fhirpath.Evaluate(observation, "Observation.value.is(Quantity)")
// true or false

// as() -- returns the value if it matches, empty otherwise
result, _ := fhirpath.Evaluate(observation, "Observation.value.as(Quantity)")
// The Quantity object, or empty

// ofType() -- filters multiple elements by type
result, _ := fhirpath.Evaluate(resource, "Bundle.entry.resource.ofType(Patient)")
// All Patient resources from the Bundle
```

In practice, `as()` and `ofType()` behave identically in this implementation -- both filter elements by type. The FHIRPath specification recommends using `ofType()` when filtering collections and `as()` when casting a single value.
