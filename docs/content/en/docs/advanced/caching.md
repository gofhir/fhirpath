---
title: "Expression Caching"
linkTitle: "Expression Caching"
weight: 1
description: >
  Use the built-in LRU expression cache to avoid redundant parsing and dramatically
  improve throughput in production workloads.
---

## Why Cache Expressions

Every FHIRPath expression must be **parsed** into an AST before it can be evaluated.
Parsing involves lexical analysis and grammar matching, which is orders of magnitude
more expensive than the subsequent tree-walk evaluation.

```text
Compile("Patient.name.family")   ~250 us   (parse + build AST)
expr.Evaluate(resource)          ~5 us     (walk the cached AST)
```

In a typical FHIR server you will evaluate the same handful of expressions (validation
constraints, search parameters, extraction rules) millions of times against different
resources. Caching the compiled `*Expression` objects eliminates the parse cost for
every call after the first.

## The Default Cache

The library ships with a ready-to-use global cache:

```go
// DefaultCache is a global expression cache with a 1 000-entry LRU limit.
var DefaultCache = NewExpressionCache(1000)
```

You can use it through the convenience functions:

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func main() {
    patient := []byte(`{
        "resourceType": "Patient",
        "name": [{"family": "Doe", "given": ["John"]}]
    }`)

    // EvaluateCached compiles (with caching) and evaluates in one call.
    result, err := fhirpath.EvaluateCached(patient, "Patient.name.family")
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // [Doe]

    // Or retrieve the compiled expression directly:
    expr, err := fhirpath.GetCached("Patient.name.given")
    if err != nil {
        panic(err)
    }
    fmt.Println(expr.Evaluate(patient)) // [John] <nil>
}
```

The `DefaultCache` is safe for concurrent use. On the first call for a given
expression string the cache compiles it and stores the result; subsequent calls
return the cached `*Expression` without parsing.

### MustGetCached

When you know the expression is syntactically valid (for example, a hard-coded
literal), you can skip error handling:

```go
expr := fhirpath.MustGetCached("Patient.name.family")
```

`MustGetCached` panics if the expression cannot be compiled. Use it only for
expressions whose syntax is guaranteed at development time.

## Custom Caches

If you need independent cache namespaces or different size limits, create your own
`ExpressionCache`:

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func main() {
    // A small cache for hot-path validation rules.
    validationCache := fhirpath.NewExpressionCache(100)

    // A larger cache for ad-hoc search parameter extraction.
    searchCache := fhirpath.NewExpressionCache(5000)

    patient := []byte(`{
        "resourceType": "Patient",
        "active": true
    }`)

    // Each cache tracks its own entries and statistics.
    expr, _ := validationCache.Get("Patient.active")
    result, _ := expr.Evaluate(patient)
    fmt.Println(result) // [true]

    expr2, _ := searchCache.Get("Patient.active")
    result2, _ := expr2.Evaluate(patient)
    fmt.Println(result2) // [true]

    fmt.Println(validationCache.Size()) // 1
    fmt.Println(searchCache.Size())     // 1
}
```

### Unbounded Cache

Pass `0` (or any non-positive value) as the limit to create a cache that never
evicts entries:

```go
// This cache will grow without bound -- only use when you know
// the set of possible expressions is finite and small.
cache := fhirpath.NewExpressionCache(0)
```

## Cache Statistics

The cache tracks hits and misses so you can monitor its effectiveness:

```go
package main

import (
    "fmt"
    "log"
    "github.com/gofhir/fhirpath"
)

func main() {
    cache := fhirpath.NewExpressionCache(500)

    expressions := []string{
        "Patient.name.family",
        "Patient.birthDate",
        "Patient.name.family", // duplicate -- will be a hit
        "Patient.active",
        "Patient.birthDate",   // duplicate -- will be a hit
    }

    for _, expr := range expressions {
        _, err := cache.Get(expr)
        if err != nil {
            log.Fatal(err)
        }
    }

    // Retrieve aggregate statistics.
    stats := cache.Stats()
    fmt.Printf("Size:   %d\n", stats.Size)   // 3
    fmt.Printf("Limit:  %d\n", stats.Limit)  // 500
    fmt.Printf("Hits:   %d\n", stats.Hits)    // 2
    fmt.Printf("Misses: %d\n", stats.Misses)  // 3

    // Or get the hit rate directly as a percentage (0-100).
    fmt.Printf("Hit rate: %.1f%%\n", cache.HitRate()) // 40.0%
}
```

### Using Statistics for Monitoring

In a production system you might expose cache stats as Prometheus metrics or
periodic log lines:

```go
func reportCacheMetrics(cache *fhirpath.ExpressionCache) {
    stats := cache.Stats()
    log.Printf(
        "fhirpath_cache size=%d limit=%d hits=%d misses=%d hit_rate=%.1f%%",
        stats.Size, stats.Limit, stats.Hits, stats.Misses, cache.HitRate(),
    )
}
```

If the hit rate is consistently low, your cache limit may be too small and the LRU
is evicting entries that are still needed. Consider increasing the limit.

## Cache Warming

For latency-sensitive applications you can **pre-compile** your known expressions at
startup so that the first real request does not pay the parse cost:

```go
package main

import (
    "log"
    "github.com/gofhir/fhirpath"
)

// expressions lists every FHIRPath expression the application uses.
var expressions = []string{
    "Patient.name.family",
    "Patient.name.given",
    "Patient.birthDate",
    "Patient.identifier.where(system = 'http://hl7.org/fhir/sid/us-ssn').value",
    "Patient.telecom.where(system = 'phone').value",
    "Patient.address.where(use = 'home')",
    "Observation.code.coding.where(system = 'http://loinc.org').code",
    "Observation.value.ofType(Quantity).value",
}

func warmCache(cache *fhirpath.ExpressionCache) {
    for _, expr := range expressions {
        if _, err := cache.Get(expr); err != nil {
            log.Fatalf("invalid expression during cache warm-up: %s -- %v", expr, err)
        }
    }
    log.Printf("Cache warmed with %d expressions", cache.Size())
}

func main() {
    cache := fhirpath.NewExpressionCache(1000)
    warmCache(cache)

    // The cache now contains compiled ASTs for all known expressions.
    // Subsequent Get() calls for these expressions will be instant cache hits.
}
```

Warming also serves as an **early validation** step: if any expression has a syntax
error, the application fails immediately at startup rather than at runtime when a
request arrives.

## Memory Considerations

Each cached `*Expression` holds a parse tree (AST) in memory. The size depends on
the complexity of the expression, but a typical expression consumes a few kilobytes.

| Cache Limit | Approximate Memory |
|-------------|-------------------|
| 100         | ~0.5 MB           |
| 1 000       | ~5 MB             |
| 10 000      | ~50 MB            |

These are rough estimates. Actual usage depends on expression complexity.

**Guidelines for choosing a cache limit:**

1. **Start with the default (1 000).** This is sufficient for most applications
   that evaluate a fixed set of expressions.
2. **Increase the limit** if your hit rate is below 90% and you have memory to spare.
3. **Use separate caches** when different subsystems have very different expression
   sets (for example, validation rules vs. search parameter extraction). This
   prevents one subsystem from evicting entries that another needs.
4. **Call `Clear()`** if you need to release memory or reset statistics:

   ```go
   cache.Clear() // Removes all entries and resets hit/miss counters.
   ```

## Summary

| Function / Method              | Description                                       |
|-------------------------------|---------------------------------------------------|
| `DefaultCache`                | Global `ExpressionCache` with a 1 000-entry limit  |
| `NewExpressionCache(limit)`   | Create a custom cache with the given LRU limit     |
| `cache.Get(expr)`             | Retrieve or compile an expression                  |
| `cache.MustGet(expr)`         | Like `Get` but panics on error                     |
| `cache.Clear()`               | Remove all entries and reset counters              |
| `cache.Size()`                | Number of entries currently cached                 |
| `cache.Stats()`               | Returns `CacheStats{Size, Limit, Hits, Misses}`   |
| `cache.HitRate()`             | Hit rate as a float64 percentage (0--100)          |
| `GetCached(expr)`             | Shorthand for `DefaultCache.Get(expr)`             |
| `MustGetCached(expr)`         | Shorthand for `DefaultCache.MustGet(expr)`         |
| `EvaluateCached(resource, expr)` | Compile with cache + evaluate in one call       |
