---
title: "Evaluation Options"
linkTitle: "Evaluation Options"
weight: 2
description: >
  Control timeouts, recursion depth, collection size limits, and custom variables
  through the functional options API.
---

## Overview of EvalOptions

When you call `Evaluate()` or `EvaluateCached()`, the library uses sensible defaults
for every safety limit. For fine-grained control, compile the expression first and
then call `EvaluateWithOptions()` with one or more functional options.

```go
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithTimeout(2 * time.Second),
    fhirpath.WithMaxDepth(50),
)
```

The underlying `EvalOptions` struct contains these fields:

| Field              | Type                        | Default               | Description                                          |
|--------------------|-----------------------------|-----------------------|------------------------------------------------------|
| `Ctx`              | `context.Context`           | `context.Background()`| Context for cancellation and deadline propagation     |
| `Timeout`          | `time.Duration`             | 5 s                   | Maximum wall-clock time for one evaluation            |
| `MaxDepth`         | `int`                       | 100                   | Recursion limit for `descendants()` and nested paths  |
| `MaxCollectionSize`| `int`                       | 10 000                | Maximum number of elements in any intermediate result |
| `Variables`        | `map[string]types.Collection`| empty map             | External variables accessible via `%name`             |
| `Resolver`         | `ReferenceResolver`         | nil                   | Handler for the `resolve()` function                  |

All options are applied on top of the defaults returned by `DefaultOptions()`, so
you only need to specify the values you want to override.

## Timeout Protection

The `WithTimeout` option wraps the evaluation in a `context.WithTimeout`. If the
expression takes longer than the specified duration, the evaluation is cancelled and
returns an error.

This is essential when evaluating **user-supplied expressions**, because a
pathological expression like `Patient.descendants().descendants()` could otherwise
run for a very long time.

```go
package main

import (
    "fmt"
    "time"

    "github.com/gofhir/fhirpath"
)

func main() {
    patient := []byte(`{
        "resourceType": "Patient",
        "name": [{"family": "Doe", "given": ["John", "James"]}]
    }`)

    expr := fhirpath.MustCompile("Patient.name.given")

    // Allow at most 500 ms for this evaluation.
    result, err := expr.EvaluateWithOptions(patient,
        fhirpath.WithTimeout(500 * time.Millisecond),
    )
    if err != nil {
        fmt.Println("evaluation timed out:", err)
        return
    }
    fmt.Println(result) // [John, James]
}
```

### Using an Existing Context

If your application already carries a request-scoped context (for example, from an
HTTP handler), pass it with `WithContext` so that the evaluation respects the
caller's cancellation signal:

```go
func handleRequest(ctx context.Context, resource []byte) (fhirpath.Collection, error) {
    expr := fhirpath.MustGetCached("Patient.name.family")
    return expr.EvaluateWithOptions(resource,
        fhirpath.WithContext(ctx),
        fhirpath.WithTimeout(2 * time.Second),
    )
}
```

When both `WithContext` and `WithTimeout` are specified, the timeout is applied as
a child of the provided context. If the parent context is cancelled first, the
evaluation stops immediately.

## Recursion Limits

The `WithMaxDepth` option limits how deeply the evaluator will recurse when
traversing nested structures. This protects against stack overflows caused by
deeply nested resources or expressions that use `descendants()`.

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func main() {
    // A deeply nested Questionnaire with items inside items.
    questionnaire := []byte(`{
        "resourceType": "Questionnaire",
        "item": [{
            "linkId": "1",
            "item": [{
                "linkId": "1.1",
                "item": [{
                    "linkId": "1.1.1"
                }]
            }]
        }]
    }`)

    expr := fhirpath.MustCompile("Questionnaire.descendants().ofType(Questionnaire.item)")

    // Restrict recursion to 50 levels instead of the default 100.
    result, err := expr.EvaluateWithOptions(questionnaire,
        fhirpath.WithMaxDepth(50),
    )
    if err != nil {
        fmt.Println("depth exceeded:", err)
        return
    }
    fmt.Println("items found:", len(result))
}
```

Set `MaxDepth` to `0` to use the default of 100.

## Collection Size Limits

The `WithMaxCollectionSize` option caps the number of elements in any intermediate
collection. If an expression produces more elements than the limit, the evaluation
returns an error rather than consuming unbounded memory.

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func main() {
    // A Bundle with many entries.
    bundle := []byte(`{
        "resourceType": "Bundle",
        "entry": [
            {"resource": {"resourceType": "Patient", "id": "1"}},
            {"resource": {"resourceType": "Patient", "id": "2"}},
            {"resource": {"resourceType": "Patient", "id": "3"}}
        ]
    }`)

    expr := fhirpath.MustCompile("Bundle.entry.resource")

    // Limit intermediate collections to 500 elements.
    result, err := expr.EvaluateWithOptions(bundle,
        fhirpath.WithMaxCollectionSize(500),
    )
    if err != nil {
        fmt.Println("collection too large:", err)
        return
    }
    fmt.Println("resources:", len(result))
}
```

The default limit is 10 000, which is generous for most workloads. Lower it when
evaluating untrusted expressions to prevent denial-of-service through memory
exhaustion.

## Custom Variables

FHIRPath supports external variables referenced with the `%` prefix. The library
automatically sets `%resource` and `%context` to the root resource, but you can
inject your own variables with `WithVariable`.

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
    "github.com/gofhir/fhirpath/types"
)

func main() {
    patient := []byte(`{
        "resourceType": "Patient",
        "identifier": [
            {"system": "http://hospital.example.org/mrn", "value": "MRN-12345"},
            {"system": "http://hl7.org/fhir/sid/us-ssn",  "value": "123-45-6789"}
        ]
    }`)

    // Find identifiers matching a system provided at runtime.
    expr := fhirpath.MustCompile("Patient.identifier.where(system = %targetSystem).value")

    targetSystem := types.Collection{types.NewString("http://hl7.org/fhir/sid/us-ssn")}

    result, err := expr.EvaluateWithOptions(patient,
        fhirpath.WithVariable("targetSystem", targetSystem),
    )
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // [123-45-6789]
}
```

### Multiple Variables

You can pass as many `WithVariable` options as you need:

```go
result, err := expr.EvaluateWithOptions(patient,
    fhirpath.WithVariable("minAge", types.Collection{types.NewInteger(18)}),
    fhirpath.WithVariable("system", types.Collection{types.NewString("http://loinc.org")}),
    fhirpath.WithVariable("today", types.Collection{todayDate}),
)
```

### Built-in Variables

The library automatically provides these variables for every evaluation:

| Variable      | Value                                      |
|---------------|--------------------------------------------|
| `%resource`   | The root resource being evaluated          |
| `%context`    | Same as `%resource` for top-level evaluation |

These are required by FHIR® constraint expressions (such as `bdl-3` and `bdl-4`)
and should not be overridden unless you have a specific reason to do so.

## Combining Options

In production code you will often combine several options. The functional option
pattern makes this clean and readable:

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/gofhir/fhirpath"
    "github.com/gofhir/fhirpath/types"
)

func evaluateExpression(
    ctx context.Context,
    resource []byte,
    expression string,
    targetSystem string,
) (fhirpath.Collection, error) {
    expr, err := fhirpath.GetCached(expression)
    if err != nil {
        return nil, fmt.Errorf("compile: %w", err)
    }

    return expr.EvaluateWithOptions(resource,
        // Propagate the caller's context for cancellation.
        fhirpath.WithContext(ctx),

        // Hard timeout for this single evaluation.
        fhirpath.WithTimeout(2 * time.Second),

        // Safety limits.
        fhirpath.WithMaxDepth(50),
        fhirpath.WithMaxCollectionSize(5000),

        // Runtime variable.
        fhirpath.WithVariable("targetSystem",
            types.Collection{types.NewString(targetSystem)},
        ),
    )
}

func main() {
    patient := []byte(`{
        "resourceType": "Patient",
        "identifier": [
            {"system": "http://hospital.example.org/mrn", "value": "MRN-001"}
        ]
    }`)

    result, err := evaluateExpression(
        context.Background(),
        patient,
        "Patient.identifier.where(system = %targetSystem).value",
        "http://hospital.example.org/mrn",
    )
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // [MRN-001]
}
```

## Quick Reference

| Function                        | Description                                               |
|---------------------------------|-----------------------------------------------------------|
| `WithContext(ctx)`              | Set the parent `context.Context`                          |
| `WithTimeout(d)`               | Set the evaluation timeout                                |
| `WithMaxDepth(n)`              | Set the maximum recursion depth                           |
| `WithMaxCollectionSize(n)`     | Set the maximum intermediate collection size              |
| `WithVariable(name, value)`    | Inject an external variable accessible via `%name`        |
| `WithResolver(r)`              | Set a `ReferenceResolver` (see [Custom Resolvers](../custom-resolvers/)) |
| `DefaultOptions()`             | Returns a new `EvalOptions` with all defaults applied     |
