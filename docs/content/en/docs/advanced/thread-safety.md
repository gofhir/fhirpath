---
title: "Thread Safety"
linkTitle: "Thread Safety"
weight: 6
description: >
  Understand the concurrency model: which objects are safe to share across goroutines
  and which must remain per-evaluation.
---

## What Is Thread-Safe

The following objects are designed to be **shared safely** across multiple goroutines:

### Compiled Expression (`*Expression`)

A compiled `*Expression` is immutable after creation. It holds only the parsed AST
and the original expression string. You can call `Evaluate()` or
`EvaluateWithOptions()` from any number of goroutines simultaneously:

```go
// Safe: one expression, many goroutines.
expr := fhirpath.MustCompile("Patient.name.family")

var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(resource []byte) {
        defer wg.Done()
        result, err := expr.Evaluate(resource)
        // handle result...
    }(resources[i])
}
wg.Wait()
```

### Expression Cache (`*ExpressionCache`)

The `ExpressionCache` (including the global `DefaultCache`) uses a `sync.RWMutex`
internally. All methods -- `Get()`, `MustGet()`, `Clear()`, `Size()`, `Stats()`,
and `HitRate()` -- are safe for concurrent use:

```go
// Safe: concurrent cache access from multiple goroutines.
cache := fhirpath.NewExpressionCache(500)

var wg sync.WaitGroup
for _, exprStr := range expressions {
    wg.Add(1)
    go func(e string) {
        defer wg.Done()
        compiled, err := cache.Get(e)
        // use compiled...
    }(exprStr)
}
wg.Wait()
```

### Convenience Functions

The top-level functions `EvaluateCached()`, `GetCached()`, and `MustGetCached()` all
delegate to `DefaultCache` and are therefore safe for concurrent use.

## What Is Not Shared

### Evaluation Context (`eval.Context`)

Each call to `Evaluate()` or `EvaluateWithOptions()` creates a **new** `eval.Context`
internally. The context holds mutable evaluation state: the current `$this` value,
`$index`, variables, and limits. It must **never** be shared between concurrent
evaluations.

You normally do not need to worry about this because the public API creates a fresh
context per call. However, if you create an `eval.Context` manually, do not reuse
it across goroutines:

```go
// WRONG -- sharing a context between goroutines.
ctx := eval.NewContext(resource)
go func() { expr1.EvaluateWithContext(ctx) }() // DATA RACE
go func() { expr2.EvaluateWithContext(ctx) }() // DATA RACE
```

```go
// CORRECT -- each goroutine gets its own context.
go func() {
    ctx := eval.NewContext(resource)
    expr1.EvaluateWithContext(ctx)
}()
go func() {
    ctx := eval.NewContext(resource)
    expr2.EvaluateWithContext(ctx)
}()
```

### Resource Byte Slices

The `[]byte` resource data passed to `Evaluate()` is **read-only** during evaluation.
It is safe to pass the same byte slice to multiple concurrent evaluations as long as
no goroutine mutates it during evaluation.

## Concurrent Evaluation Pattern

The most common concurrent pattern is a fan-out where multiple resources are
evaluated in parallel using the same compiled expression:

```go
package main

import (
    "fmt"
    "sync"

    "github.com/gofhir/fhirpath"
)

func main() {
    // Compile once.
    expr := fhirpath.MustCompile("Patient.name.family")

    resources := [][]byte{
        []byte(`{"resourceType":"Patient","name":[{"family":"Smith"}]}`),
        []byte(`{"resourceType":"Patient","name":[{"family":"Johnson"}]}`),
        []byte(`{"resourceType":"Patient","name":[{"family":"Williams"}]}`),
        []byte(`{"resourceType":"Patient","name":[{"family":"Brown"}]}`),
    }

    results := make([]fhirpath.Collection, len(resources))
    errors := make([]error, len(resources))

    var wg sync.WaitGroup
    for i, res := range resources {
        wg.Add(1)
        go func(idx int, resource []byte) {
            defer wg.Done()
            results[idx], errors[idx] = expr.Evaluate(resource)
        }(i, res)
    }
    wg.Wait()

    for i, result := range results {
        if errors[i] != nil {
            fmt.Printf("resource %d: error: %v\n", i, errors[i])
        } else {
            fmt.Printf("resource %d: %v\n", i, result)
        }
    }
}
```

## Production Patterns

### HTTP Handler

In an HTTP server, each request handler runs in its own goroutine. Compiled
expressions and the expression cache can be shared across all handlers:

```go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"

    "github.com/gofhir/fhirpath"
)

// expressionCache is shared across all request handlers.
var expressionCache = fhirpath.NewExpressionCache(1000)

// Pre-compiled expressions for known operations.
var (
    exprFamilyName = fhirpath.MustCompile("Patient.name.family")
    exprBirthDate  = fhirpath.MustCompile("Patient.birthDate")
    exprActive     = fhirpath.MustCompile("Patient.active")
)

func handleExtract(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "failed to read body", http.StatusBadRequest)
        return
    }

    // Each evaluation creates its own internal eval.Context --
    // no shared mutable state between requests.
    family, err := exprFamilyName.EvaluateWithOptions(body,
        fhirpath.WithContext(r.Context()),
        fhirpath.WithTimeout(2*time.Second),
    )
    if err != nil {
        http.Error(w, fmt.Sprintf("evaluation error: %v", err),
            http.StatusInternalServerError)
        return
    }

    resp := map[string]interface{}{
        "familyName": family.String(),
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func handleDynamic(w http.ResponseWriter, r *http.Request) {
    // For user-supplied expressions, use the cache.
    exprStr := r.URL.Query().Get("expression")
    if exprStr == "" {
        http.Error(w, "missing expression parameter", http.StatusBadRequest)
        return
    }

    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "failed to read body", http.StatusBadRequest)
        return
    }

    // GetCached is safe for concurrent use.
    compiled, err := expressionCache.Get(exprStr)
    if err != nil {
        http.Error(w, fmt.Sprintf("invalid expression: %v", err),
            http.StatusBadRequest)
        return
    }

    result, err := compiled.EvaluateWithOptions(body,
        fhirpath.WithContext(r.Context()),
        fhirpath.WithTimeout(2*time.Second),
        fhirpath.WithMaxCollectionSize(1000),
    )
    if err != nil {
        http.Error(w, fmt.Sprintf("evaluation error: %v", err),
            http.StatusInternalServerError)
        return
    }

    resp := map[string]interface{}{
        "result": result.String(),
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func main() {
    http.HandleFunc("/extract", handleExtract)
    http.HandleFunc("/evaluate", handleDynamic)
    log.Println("listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Worker Pool

For batch processing (for example, validating all resources in a database), use a
worker pool to bound concurrency:

```go
package main

import (
    "fmt"
    "sync"
    "time"

    "github.com/gofhir/fhirpath"
)

// ValidationResult holds the outcome for one resource.
type ValidationResult struct {
    Index int
    Valid bool
    Error error
}

func validateBatch(
    resources [][]byte,
    expression *fhirpath.Expression,
    workers int,
) []ValidationResult {
    jobs := make(chan int, len(resources))
    results := make([]ValidationResult, len(resources))

    var wg sync.WaitGroup
    for w := 0; w < workers; w++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for idx := range jobs {
                result, err := expression.EvaluateWithOptions(resources[idx],
                    fhirpath.WithTimeout(2*time.Second),
                    fhirpath.WithMaxCollectionSize(5000),
                )
                if err != nil {
                    results[idx] = ValidationResult{Index: idx, Error: err}
                    continue
                }

                // Check if the expression returned true (valid).
                valid := false
                if !result.Empty() {
                    valid = result.String() == "[true]"
                }
                results[idx] = ValidationResult{Index: idx, Valid: valid}
            }
        }()
    }

    // Enqueue all jobs.
    for i := range resources {
        jobs <- i
    }
    close(jobs)

    wg.Wait()
    return results
}

func main() {
    // Compile the validation expression once.
    expr := fhirpath.MustCompile("Patient.name.exists() and Patient.birthDate.exists()")

    resources := [][]byte{
        []byte(`{"resourceType":"Patient","name":[{"family":"Doe"}],"birthDate":"1990-01-15"}`),
        []byte(`{"resourceType":"Patient","name":[{"family":"Smith"}]}`),
        []byte(`{"resourceType":"Patient","birthDate":"1985-03-22"}`),
        []byte(`{"resourceType":"Patient","name":[{"family":"Lee"}],"birthDate":"2000-07-04"}`),
    }

    // Process with 4 worker goroutines.
    results := validateBatch(resources, expr, 4)

    for _, r := range results {
        if r.Error != nil {
            fmt.Printf("resource %d: error: %v\n", r.Index, r.Error)
        } else {
            fmt.Printf("resource %d: valid=%v\n", r.Index, r.Valid)
        }
    }
    // Output:
    // resource 0: valid=true
    // resource 1: valid=false
    // resource 2: valid=false
    // resource 3: valid=true
}
```

## Summary

| Object                        | Thread-Safe? | Notes                                                 |
|-------------------------------|--------------|-------------------------------------------------------|
| `*Expression`                 | Yes          | Immutable after creation; share freely                |
| `*ExpressionCache`            | Yes          | Uses `sync.RWMutex` internally                       |
| `DefaultCache`                | Yes          | Global `*ExpressionCache`                             |
| `EvaluateCached()`, `GetCached()` | Yes      | Delegate to `DefaultCache`                            |
| `eval.Context`                | **No**       | Created per-evaluation; never share between goroutines|
| `[]byte` resource             | Read-only    | Safe if no goroutine mutates it during evaluation     |
| `ReferenceResolver`           | Depends      | Your implementation must be safe for concurrent use   |
| `TerminologyService`          | Depends      | Your implementation must be safe for concurrent use   |
| `ProfileValidator`            | Depends      | Your implementation must be safe for concurrent use   |
