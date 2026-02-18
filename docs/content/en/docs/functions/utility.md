---
title: "Utility Functions"
linkTitle: "Utility Functions"
weight: 10
description: >
  Functions for debugging, logging, and navigating the element tree in FHIRPath expressions.
---

Utility functions help with debugging FHIRPath expressions and navigating the hierarchical structure of FHIR® resources. The `trace` function provides observability during expression evaluation, while `children` and `descendants` enable tree traversal.

---

## trace

Logs the input collection for debugging purposes and returns it unchanged. This function is a pass-through that adds observability without affecting the evaluation result.

**Signature:**

```text
trace(name : String [, projection : Expression]) : Collection
```

**Parameters:**

| Name           | Type         | Description                                                                  |
|----------------|--------------|------------------------------------------------------------------------------|
| `name`         | `String`     | A label to identify this trace point in the log output                       |
| `projection`   | `Expression` | (Optional) An additional expression to evaluate and log alongside the input  |

**Return Type:** `Collection` (the input collection, unchanged)

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.trace('names').where(use = 'official')")
// Logs: [trace] names: { ... }
// Returns: the filtered names (trace does not alter the pipeline)

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.trace('telecom').select(value)")
// Logs the telecom entries before selecting values

result, _ := fhirpath.Evaluate(patient, "Patient.name.trace('names', given)")
// Logs both the name entries and their 'given' projection
```

**Edge Cases / Notes:**

- The function does **not** modify the result. It is purely a side effect for logging.
- By default, trace output is written to `stderr` in plain text format.
- The trace logger can be customized using `funcs.SetTraceLogger()`:
  - `funcs.NewDefaultTraceLogger(writer, false)` for plain text output.
  - `funcs.NewDefaultTraceLogger(writer, true)` for JSON-structured output.
  - `funcs.NullTraceLogger{}` to disable trace output entirely (recommended for production).
- Each trace entry includes a timestamp, the name label, the input collection, count, and an optional projection.

### Configuring the Trace Logger

```go
import "github.com/gofhir/fhirpath/funcs"

// JSON-structured logging to stdout
funcs.SetTraceLogger(funcs.NewDefaultTraceLogger(os.Stdout, true))

// Disable trace output in production
funcs.SetTraceLogger(funcs.NullTraceLogger{})

// Custom logger implementing the funcs.TraceLogger interface
funcs.SetTraceLogger(myCustomLogger)
```

---

## children

Returns all direct child elements of each element in the input collection.

**Signature:**

```text
children() : Collection
```

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.children()")
// Returns all direct child elements: name, telecom, birthDate, gender, etc.

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().children()")
// Returns children of the first name: use, family, given, etc.

result, _ := fhirpath.Evaluate(patient, "Patient.children().count()")
// Number of direct child elements
```

**Edge Cases / Notes:**

- Only works on complex types (`ObjectValue`). Primitive values (strings, integers, etc.) have no children and produce no output.
- Returns the values of all fields in the object, regardless of field name.
- An empty input collection returns an empty collection.
- The order of children depends on the underlying object structure.

---

## descendants

Returns all descendant elements of each element in the input collection, recursively. This includes children, grandchildren, and so on at every nesting level.

**Signature:**

```text
descendants() : Collection
```

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.descendants()")
// Returns ALL nested elements at every level of the Patient resource

result, _ := fhirpath.Evaluate(patient, "Patient.descendants().ofType(HumanName)")
// Finds all HumanName elements anywhere in the resource

result, _ := fhirpath.Evaluate(patient, "Patient.name.descendants()")
// Returns all nested elements within name entries
```

**Edge Cases / Notes:**

- This function recursively traverses the entire element tree beneath the input.
- Cycle detection is built in: elements that have already been visited are skipped to prevent infinite loops.
- Only complex types (`ObjectValue`) are traversed. Primitive values are included in the result but do not produce further descendants.
- An empty input collection returns an empty collection.
- For large resources, `descendants()` may return a very large collection. Consider using more targeted path expressions when possible.
- The result includes intermediate nodes, not just leaf nodes. Both complex and primitive descendants are returned.

---

## Practical Usage Patterns

### Finding All Elements of a Specific Type

```go
// Find all CodeableConcept elements anywhere in a resource
result, _ := fhirpath.Evaluate(resource, "Resource.descendants().ofType(CodeableConcept)")
```

### Debugging Complex Expressions

```go
// Trace intermediate values in a chain
result, _ := fhirpath.Evaluate(patient,
    "Patient.name.trace('all-names').where(use = 'official').trace('official-names').first().given")
```

### Exploring Resource Structure

```go
// Count all nested elements
result, _ := fhirpath.Evaluate(patient, "Patient.descendants().count()")

// Get all direct child element names
result, _ := fhirpath.Evaluate(patient, "Patient.children()")
```
