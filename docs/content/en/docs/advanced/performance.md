---
title: "Performance Guide"
linkTitle: "Performance Guide"
weight: 5
description: >
  Practical patterns for high-throughput FHIRPath evaluation: compile-once, expression
  caching, resource pre-serialization, early filtering, and type conversion tips.
---

## Compile Once Pattern

The single most impactful optimization is to **compile each expression once** and
reuse the resulting `*Expression` object. Parsing is 10-50x more expensive than
evaluation.

### Bad: Compile on Every Call

```go
// BAD -- parses the expression on every iteration.
for _, resource := range resources {
    result, err := fhirpath.Evaluate(resource, "Patient.name.family")
    if err != nil {
        log.Fatal(err)
    }
    process(result)
}
```

`fhirpath.Evaluate()` calls `Compile()` internally every time. For a loop over
10 000 resources, you pay the parse cost 10 000 times.

### Good: Compile Once, Evaluate Many

```go
// GOOD -- compile once, evaluate many times.
expr := fhirpath.MustCompile("Patient.name.family")

for _, resource := range resources {
    result, err := expr.Evaluate(resource)
    if err != nil {
        log.Fatal(err)
    }
    process(result)
}
```

The compiled `*Expression` is immutable and safe to share across goroutines (see
[Thread Safety](../thread-safety/)).

### Best: Package-Level Variables

For expressions known at development time, compile them once at package
initialization:

```go
package myvalidator

import "github.com/gofhir/fhirpath"

var (
    exprFamilyName = fhirpath.MustCompile("Patient.name.family")
    exprBirthDate  = fhirpath.MustCompile("Patient.birthDate")
    exprMRN        = fhirpath.MustCompile(
        "Patient.identifier.where(system = 'http://hospital.example.org/mrn').value",
    )
)

func GetFamilyName(patient []byte) (string, error) {
    return fhirpath.EvaluateToString(patient, "Patient.name.family")
}
```

`MustCompile` panics if the expression is invalid, which surfaces syntax errors
immediately at startup rather than at runtime.

## Expression Caching

When expressions are not known at compile time (for example, user-supplied search
expressions or expressions loaded from configuration), use the expression cache:

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func evaluateUserExpression(resource []byte, userExpr string) (fhirpath.Collection, error) {
    // GetCached compiles on the first call and returns the cached
    // *Expression on subsequent calls.
    expr, err := fhirpath.GetCached(userExpr)
    if err != nil {
        return nil, fmt.Errorf("invalid expression: %w", err)
    }
    return expr.Evaluate(resource)
}
```

See [Expression Caching](../caching/) for details on cache sizing, warming, and
monitoring.

### When to Use Which

| Scenario                                    | Approach                    |
|---------------------------------------------|-----------------------------|
| Hard-coded expression, known at compile time | `MustCompile` as package var |
| Expression from config, loaded once          | `Compile` at startup         |
| Dynamic expression, many distinct values     | `GetCached` / `ExpressionCache` |
| One-off expression, never reused             | `Evaluate` (no caching)     |

## Resource Pre-serialization

If you have a Go struct that you need to evaluate multiple expressions against,
serialize it to JSON **once** using `ResourceJSON` instead of letting each
evaluation call `json.Marshal`:

### Bad: Marshal on Every Evaluation

```go
type MyPatient struct {
    ResourceType string `json:"resourceType"`
    ID           string `json:"id"`
    // ... many fields
}

func (p *MyPatient) GetResourceType() string { return p.ResourceType }

// BAD -- marshals the struct to JSON on every call.
func validatePatient(p *MyPatient) error {
    _, err := fhirpath.EvaluateResource(p, "Patient.name.exists()")
    if err != nil {
        return err
    }
    _, err = fhirpath.EvaluateResource(p, "Patient.birthDate.exists()")
    return err
}
```

### Good: Serialize Once with ResourceJSON

```go
// GOOD -- serialize once, evaluate many times.
func validatePatient(p *MyPatient) error {
    rj, err := fhirpath.NewResourceJSON(p)
    if err != nil {
        return fmt.Errorf("serialize: %w", err)
    }

    // Each call reuses the pre-serialized JSON bytes.
    _, err = rj.EvaluateCached("Patient.name.exists()")
    if err != nil {
        return err
    }
    _, err = rj.EvaluateCached("Patient.birthDate.exists()")
    return err
}
```

For even better performance, keep the `[]byte` JSON around when you already have it
(for example, from an HTTP request body) and evaluate directly against that:

```go
func handleCreatePatient(body []byte) error {
    // body is already JSON -- no marshalling needed.
    result, err := fhirpath.EvaluateCached(body, "Patient.name.exists()")
    if err != nil {
        return err
    }
    // ...
}
```

## Filter Early

When an expression operates on a large collection, use `where()` to reduce its size
as early as possible. This minimizes the number of elements that downstream
functions must process.

### Bad: Process Everything, Filter Late

```go
// BAD -- descendants() expands the entire resource tree, then filters.
expr := fhirpath.MustCompile(
    "Bundle.entry.resource.descendants().ofType(Coding).where(system = 'http://loinc.org')",
)
```

### Good: Filter at Each Level

```go
// GOOD -- filter entries first, then navigate to the specific element.
expr := fhirpath.MustCompile(
    "Bundle.entry.resource.ofType(Observation).code.coding.where(system = 'http://loinc.org')",
)
```

The second expression avoids calling `descendants()` entirely. Instead, it narrows
down to Observation resources first, then navigates directly to the code element.

### Collection Size Limits

As a safety net, set `WithMaxCollectionSize` when evaluating untrusted expressions
to prevent pathological queries from consuming unbounded memory:

```go
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithMaxCollectionSize(5000),
)
```

## Avoid Unnecessary Conversions

The library works with `types.Collection` (a slice of `types.Value`). Avoid
round-tripping through Go native types when you can work with the FHIRPath values
directly.

### Bad: Convert to String Just to Compare

```go
// BAD -- unnecessary string conversion.
result, _ := expr.Evaluate(patient)
for _, v := range result {
    str := v.String()
    if str == "active" {
        // ...
    }
}
```

### Good: Use Type-Aware Comparison

```go
// GOOD -- compare at the FHIRPath type level.
result, _ := expr.Evaluate(patient)
for _, v := range result {
    if s, ok := v.(types.String); ok && s.Value() == "active" {
        // ...
    }
}
```

### Use Convenience Functions

For common extraction patterns, use the built-in convenience functions that handle
type conversion correctly:

```go
// Extract a single boolean.
active, err := fhirpath.EvaluateToBoolean(patient, "Patient.active")

// Extract a single string.
family, err := fhirpath.EvaluateToString(patient, "Patient.name.first().family")

// Extract multiple strings.
givens, err := fhirpath.EvaluateToStrings(patient, "Patient.name.first().given")

// Check existence.
hasName, err := fhirpath.Exists(patient, "Patient.name")

// Count results.
nameCount, err := fhirpath.Count(patient, "Patient.name")
```

## Best Practices Summary

1. **Compile expressions once.** Use `MustCompile` for hard-coded expressions or
   `GetCached` for dynamic ones. Never call `Evaluate()` in a hot loop.

2. **Use the expression cache** for dynamic expressions. Size it appropriately and
   monitor the hit rate.

3. **Pre-serialize resources** when evaluating multiple expressions against the same
   Go struct. Use `ResourceJSON` or keep the raw `[]byte` around.

4. **Filter early.** Use `where()` and `ofType()` to narrow collections before
   applying expensive operations like `descendants()`.

5. **Set safety limits.** Use `WithTimeout`, `WithMaxDepth`, and
   `WithMaxCollectionSize` when evaluating untrusted expressions.

6. **Avoid unnecessary type conversions.** Work with `types.Value` directly and use
   the convenience functions (`EvaluateToString`, `EvaluateToBoolean`, etc.) when
   you need Go native types.

7. **Warm the cache at startup** for latency-sensitive applications. This also
   validates expression syntax early.

8. **Profile before optimizing.** Use Go's built-in benchmarking and profiling tools
   (`go test -bench`, `pprof`) to identify actual bottlenecks before applying
   optimizations.
