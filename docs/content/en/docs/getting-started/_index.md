---
title: "Getting Started"
linkTitle: "Getting Started"
description: "Install the FHIRPath Go library, evaluate your first expression, and learn the core API patterns for compiling, caching, and extracting data from FHIR® resources."
weight: 1
---

This guide walks you through installing the library, running your first FHIRPath evaluation, and adopting the patterns you will use in production code.

## Prerequisites

- **Go 1.23** or later.
- A Go module (`go.mod`) in your project. If you do not have one yet, run `go mod init <your-module>`.

## Installation

Add the library to your project with `go get`:

```bash
go get github.com/gofhir/fhirpath
```

Then import it in your Go source files:

```go
import "github.com/gofhir/fhirpath"
```

## Your First Evaluation

The simplest way to evaluate a FHIRPath expression is the top-level `Evaluate` function. It accepts raw JSON bytes representing a FHIR® resource and a FHIRPath expression string, and returns a `Collection` of results.

```go
package main

import (
    "fmt"
    "log"

    "github.com/gofhir/fhirpath"
)

func main() {
    // Define a FHIR Patient resource as JSON
    patient := []byte(`{
        "resourceType": "Patient",
        "id": "123",
        "name": [{"family": "Doe", "given": ["John"]}],
        "birthDate": "1990-05-15"
    }`)

    // Evaluate a FHIRPath expression
    result, err := fhirpath.Evaluate(patient, "Patient.name.family")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result) // [Doe]
}
```

`Evaluate` compiles and evaluates the expression in a single call. It returns a `types.Collection` (an alias for `[]types.Value`), which holds the evaluation result. Every FHIRPath expression produces a collection -- even a single scalar value is wrapped in a one-element collection, and a missing path yields an empty collection.

## Compiling Expressions

If you plan to evaluate the same expression against many resources, compile it once with `Compile` or `MustCompile` and then reuse the resulting `*Expression`:

```go
// Compile once (returns an error if the expression is invalid)
expr, err := fhirpath.Compile("Patient.name.given")
if err != nil {
    log.Fatal(err)
}

// Evaluate against multiple resources
result1, _ := expr.Evaluate(patient1JSON)
result2, _ := expr.Evaluate(patient2JSON)
```

`MustCompile` is a convenience variant that panics instead of returning an error. It is useful for package-level variables where the expression is known at compile time:

```go
var nameExpr = fhirpath.MustCompile("Patient.name.family")
```

## Expression Cache

For production workloads where expressions may arrive at runtime (for example, from configuration or user input), use `EvaluateCached`. It maintains a global, thread-safe LRU cache of compiled expressions so that repeated evaluations of the same expression string do not pay the compilation cost more than once:

```go
result, err := fhirpath.EvaluateCached(patientJSON, "Patient.birthDate")
```

The default cache holds up to 1,000 expressions. You can create a custom cache for finer control:

```go
cache := fhirpath.NewExpressionCache(500) // custom size

expr, err := cache.Get("Patient.name.family")
if err != nil {
    log.Fatal(err)
}

result, err := expr.Evaluate(patientJSON)
```

You can inspect cache performance at any time:

```go
stats := cache.Stats()
fmt.Printf("Size: %d, Hits: %d, Misses: %d, Hit Rate: %.1f%%\n",
    stats.Size, stats.Hits, stats.Misses, cache.HitRate())
```

## Convenience Functions

The library provides several typed convenience functions that evaluate an expression and extract the result in a single call. All of these use the expression cache internally.

### EvaluateToBoolean

Returns a Go `bool`. Useful for FHIRPath expressions that produce a single Boolean value, such as validation constraints:

```go
active, err := fhirpath.EvaluateToBoolean(patientJSON, "Patient.active")
if err != nil {
    log.Fatal(err)
}
fmt.Println(active) // true or false
```

### EvaluateToString

Returns a single Go `string`:

```go
family, err := fhirpath.EvaluateToString(patientJSON, "Patient.name.first().family")
if err != nil {
    log.Fatal(err)
}
fmt.Println(family) // Doe
```

### EvaluateToStrings

Returns a `[]string` containing the string representation of every value in the result collection:

```go
givenNames, err := fhirpath.EvaluateToStrings(patientJSON, "Patient.name.given")
if err != nil {
    log.Fatal(err)
}
fmt.Println(givenNames) // [John]
```

### Exists

Returns `true` if the expression produces a non-empty collection:

```go
hasPhone, err := fhirpath.Exists(patientJSON, "Patient.telecom.where(system='phone')")
if err != nil {
    log.Fatal(err)
}
fmt.Println(hasPhone) // true or false
```

### Count

Returns the number of elements in the result collection:

```go
nameCount, err := fhirpath.Count(patientJSON, "Patient.name")
if err != nil {
    log.Fatal(err)
}
fmt.Println(nameCount) // 1
```

## Error Handling

The library reports two categories of errors:

1. **Compilation errors** -- returned by `Compile` (or raised by `MustCompile` as a panic) when the expression string contains invalid FHIRPath syntax.

2. **Evaluation errors** -- returned by `Evaluate` and related functions when a runtime error occurs (for example, comparing incompatible types).

A typical error-handling pattern looks like this:

```go
result, err := fhirpath.Evaluate(resource, expr)
if err != nil {
    // Handle or log the error
    return fmt.Errorf("fhirpath evaluation failed: %w", err)
}

if result.Empty() {
    // The path resolved but produced no values
    fmt.Println("No results found")
} else {
    fmt.Println("Result:", result)
}
```

Empty collections are not errors. In FHIRPath, navigating to a path that does not exist simply returns an empty collection (`{}`). Always check `result.Empty()` before extracting values.

## Evaluation with Options

For advanced use cases, you can pass functional options to control timeouts, recursion limits, and custom variables:

```go
expr := fhirpath.MustCompile("Patient.name.family")

result, err := expr.EvaluateWithOptions(patientJSON,
    fhirpath.WithTimeout(2*time.Second),
    fhirpath.WithMaxDepth(50),
    fhirpath.WithVariable("name", types.Collection{types.NewString("test")}),
)
```

See [Environment Variables]({{< relref "../concepts/environment-variables" >}}) for more detail on custom variables.

## Next Steps

- **[Type System]({{< relref "../concepts/type-system" >}})** -- learn about the eight FHIRPath types and how they map to Go types.
- **[Collections]({{< relref "../concepts/collections" >}})** -- understand empty propagation, singleton evaluation, and collection operations.
- **[Operators]({{< relref "../concepts/operators" >}})** -- reference for arithmetic, comparison, Boolean, and collection operators.
- **[Environment Variables]({{< relref "../concepts/environment-variables" >}})** -- use built-in and custom variables in your expressions.
