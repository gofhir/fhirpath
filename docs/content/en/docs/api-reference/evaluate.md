---
title: "Evaluate Functions"
linkTitle: "Evaluate"
weight: 1
description: >
  One-shot evaluation of FHIRPath expressions against JSON FHIR resources.
---

The evaluate functions are the simplest way to run a FHIRPath expression. They accept raw JSON bytes and an expression string, and return a `Collection` of results. Choose the variant that best matches your needs.

## Evaluate

Parses and evaluates a FHIRPath expression against a JSON resource in a single call. The expression is compiled each time, so this is best suited for one-off evaluations.

```go
func Evaluate(resource []byte, expr string) (Collection, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `[]byte` | Raw JSON bytes of a FHIR resource |
| `expr` | `string` | A FHIRPath expression to evaluate |

**Returns:**

| Type | Description |
|------|-------------|
| `Collection` | An ordered sequence of FHIRPath values (alias for `types.Collection`) |
| `error` | Non-nil if the expression is invalid or evaluation fails |

**Example:**

```go
package main

import (
    "fmt"
    "log"

    "github.com/gofhir/fhirpath"
)

func main() {
    patient := []byte(`{
        "resourceType": "Patient",
        "name": [{"family": "Smith", "given": ["John", "Jacob"]}],
        "birthDate": "1990-01-15"
    }`)

    // Extract the family name
    result, err := fhirpath.Evaluate(patient, "Patient.name.family")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result) // [Smith]

    // Use a more complex expression
    result, err = fhirpath.Evaluate(patient, "Patient.name.given.count()")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result) // [2]
}
```

{{% alert title="Performance Note" color="warning" %}}
`Evaluate` compiles the expression on every call. If you evaluate the same expression repeatedly, use `EvaluateCached` or pre-compile with `Compile` instead.
{{% /alert %}}

---

## MustEvaluate

Like `Evaluate`, but panics instead of returning an error. Use this in tests or initialization code where a failure is unrecoverable.

```go
func MustEvaluate(resource []byte, expr string) Collection
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `[]byte` | Raw JSON bytes of a FHIR resource |
| `expr` | `string` | A FHIRPath expression to evaluate |

**Returns:**

| Type | Description |
|------|-------------|
| `Collection` | An ordered sequence of FHIRPath values |

**Panics** if the expression is invalid or evaluation fails.

**Example:**

```go
// In a test
func TestPatientName(t *testing.T) {
    patient := []byte(`{"resourceType": "Patient", "name": [{"family": "Doe"}]}`)
    result := fhirpath.MustEvaluate(patient, "Patient.name.family")

    if result.Count() != 1 {
        t.Errorf("expected 1 name, got %d", result.Count())
    }
}
```

---

## EvaluateCached

Compiles (with automatic caching) and evaluates a FHIRPath expression. Subsequent calls with the same expression string skip compilation entirely and reuse the cached parse tree. This is the **recommended function for production use**.

```go
func EvaluateCached(resource []byte, expr string) (Collection, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `[]byte` | Raw JSON bytes of a FHIR resource |
| `expr` | `string` | A FHIRPath expression to evaluate |

**Returns:**

| Type | Description |
|------|-------------|
| `Collection` | An ordered sequence of FHIRPath values |
| `error` | Non-nil if the expression is invalid or evaluation fails |

`EvaluateCached` uses the package-level `DefaultCache` (an LRU cache with a limit of 1000 entries). For custom cache settings, create your own `ExpressionCache`.

**Example:**

```go
func extractNames(patients [][]byte) ([]string, error) {
    var names []string
    for _, p := range patients {
        // The expression is compiled only on the first call;
        // subsequent iterations reuse the cached compilation.
        result, err := fhirpath.EvaluateCached(p, "Patient.name.family")
        if err != nil {
            return nil, err
        }
        if first, ok := result.First(); ok {
            names = append(names, first.String())
        }
    }
    return names, nil
}
```

---

## When to Use Each Function

| Function | Compilation | Panics | Best For |
|----------|-------------|--------|----------|
| `Evaluate` | Every call | No | One-off evaluations, scripts, exploratory work |
| `MustEvaluate` | Every call | Yes | Tests, init code, guaranteed-valid expressions |
| `EvaluateCached` | Once (cached) | No | Production workloads, loops, HTTP handlers |

For even more control, see [Compile and Expression](../compile/) to pre-compile expressions, or [Expression Cache](../cache/) to manage cache size and monitor hit rates.

## Error Handling

All non-`Must` functions return an `error` as the second value. Errors fall into two categories:

1. **Compilation errors** -- The expression string is syntactically invalid.
2. **Evaluation errors** -- The expression is valid but fails at runtime (e.g., type mismatch, division by zero).

```go
result, err := fhirpath.Evaluate(resource, "Patient.name.family")
if err != nil {
    // Handle error: check err.Error() for details
    log.Printf("FHIRPath evaluation failed: %v", err)
    return
}
// Use result safely
```
