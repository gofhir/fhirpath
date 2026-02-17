---
title: "Expression Cache"
linkTitle: "Cache"
weight: 5
description: >
  Thread-safe LRU caching of compiled FHIRPath expressions with monitoring.
---

Compiling a FHIRPath expression involves lexing and parsing. When the same expressions are evaluated repeatedly (e.g., in an HTTP handler or data pipeline), caching the compiled form avoids redundant work. The `ExpressionCache` provides a thread-safe, LRU-evicting cache with built-in statistics.

## ExpressionCache

```go
type ExpressionCache struct {
    // unexported fields
}
```

`ExpressionCache` stores compiled `*Expression` objects keyed by their source string. It uses LRU (Least Recently Used) eviction when the cache reaches its size limit. All methods are safe for concurrent use.

### NewExpressionCache

Creates a new cache with the given maximum number of entries. If `limit` is 0 or negative, the cache grows without bound (not recommended for production).

```go
func NewExpressionCache(limit int) *ExpressionCache
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `limit` | `int` | Maximum number of cached expressions. Use 0 for unbounded. |

**Returns:**

| Type | Description |
|------|-------------|
| `*ExpressionCache` | A new, empty cache |

**Example:**

```go
// Create a cache that holds up to 500 compiled expressions
cache := fhirpath.NewExpressionCache(500)
```

---

### ExpressionCache.Get

Retrieves a compiled expression from the cache. If the expression is not cached, it is compiled, stored, and returned. On cache hit, the entry is promoted to the front of the LRU list.

```go
func (c *ExpressionCache) Get(expr string) (*Expression, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `expr` | `string` | A FHIRPath expression string |

**Returns:**

| Type | Description |
|------|-------------|
| `*Expression` | The compiled expression (from cache or freshly compiled) |
| `error` | Non-nil if the expression is syntactically invalid |

**Example:**

```go
cache := fhirpath.NewExpressionCache(100)

// First call: compiles and caches
expr, err := cache.Get("Patient.name.family")
if err != nil {
    log.Fatal(err)
}

// Second call: returns from cache (no compilation)
expr2, err := cache.Get("Patient.name.family")
if err != nil {
    log.Fatal(err)
}
// expr and expr2 point to the same *Expression
```

---

### ExpressionCache.MustGet

Like `Get`, but panics on error.

```go
func (c *ExpressionCache) MustGet(expr string) *Expression
```

**Panics** if the expression is syntactically invalid.

---

### ExpressionCache.Clear

Removes all entries from the cache and resets hit/miss counters to zero.

```go
func (c *ExpressionCache) Clear()
```

**Example:**

```go
cache.Clear()
fmt.Println(cache.Size()) // 0
```

---

### ExpressionCache.Size

Returns the current number of cached expressions.

```go
func (c *ExpressionCache) Size() int
```

---

### ExpressionCache.Stats

Returns a snapshot of cache performance statistics.

```go
func (c *ExpressionCache) Stats() CacheStats
```

**Returns:**

| Type | Description |
|------|-------------|
| `CacheStats` | A struct containing size, limit, hit count, and miss count |

---

### ExpressionCache.HitRate

Returns the cache hit rate as a percentage between 0 and 100. Returns 0 if no lookups have been performed.

```go
func (c *ExpressionCache) HitRate() float64
```

**Example:**

```go
fmt.Printf("Cache hit rate: %.1f%%\n", cache.HitRate())
```

---

## CacheStats

A snapshot of cache performance metrics.

```go
type CacheStats struct {
    Size   int   // Current number of cached entries
    Limit  int   // Maximum cache capacity
    Hits   int64 // Total number of cache hits
    Misses int64 // Total number of cache misses
}
```

**Example:**

```go
stats := cache.Stats()
fmt.Printf("Cache: %d/%d entries, %d hits, %d misses\n",
    stats.Size, stats.Limit, stats.Hits, stats.Misses)
```

---

## DefaultCache

The package provides a pre-configured global cache with a limit of 1000 entries. Functions like `EvaluateCached`, `GetCached`, and `MustGetCached` all use this cache.

```go
var DefaultCache = NewExpressionCache(1000)
```

---

## GetCached

Retrieves or compiles an expression using `DefaultCache`. This is a convenience wrapper around `DefaultCache.Get(expr)`.

```go
func GetCached(expr string) (*Expression, error)
```

**Example:**

```go
expr, err := fhirpath.GetCached("Patient.name.family")
if err != nil {
    log.Fatal(err)
}
result, err := expr.Evaluate(patientJSON)
```

---

## MustGetCached

Like `GetCached`, but panics on error. Wraps `DefaultCache.MustGet(expr)`.

```go
func MustGetCached(expr string) *Expression
```

---

## Monitoring and Observability

### Exposing Cache Metrics

You can periodically log or export cache statistics to your monitoring system:

```go
func logCacheStats() {
    stats := fhirpath.DefaultCache.Stats()
    log.Printf("FHIRPath cache: size=%d/%d hits=%d misses=%d hitRate=%.1f%%",
        stats.Size, stats.Limit, stats.Hits, stats.Misses,
        fhirpath.DefaultCache.HitRate())
}
```

### Prometheus Metrics Example

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/gofhir/fhirpath"
)

var (
    cacheSize = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
        Name: "fhirpath_cache_size",
        Help: "Number of cached FHIRPath expressions",
    }, func() float64 {
        return float64(fhirpath.DefaultCache.Size())
    })

    cacheHitRate = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
        Name: "fhirpath_cache_hit_rate",
        Help: "FHIRPath expression cache hit rate (0-100)",
    }, func() float64 {
        return fhirpath.DefaultCache.HitRate()
    })
)

func init() {
    prometheus.MustRegister(cacheSize, cacheHitRate)
}
```

### Health Check Endpoint

```go
func cacheHealthHandler(w http.ResponseWriter, r *http.Request) {
    stats := fhirpath.DefaultCache.Stats()
    hitRate := fhirpath.DefaultCache.HitRate()

    response := map[string]interface{}{
        "size":     stats.Size,
        "limit":    stats.Limit,
        "hits":     stats.Hits,
        "misses":   stats.Misses,
        "hit_rate": hitRate,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

---

## Using a Custom Cache

For applications that need multiple isolated caches or different size limits:

```go
// Separate caches for different workloads
var (
    validationCache = fhirpath.NewExpressionCache(200)
    extractionCache = fhirpath.NewExpressionCache(500)
)

func validateResource(resource []byte) error {
    expr, err := validationCache.Get("Patient.name.exists()")
    if err != nil {
        return err
    }
    result, err := expr.Evaluate(resource)
    if err != nil {
        return err
    }
    if result.Empty() {
        return fmt.Errorf("patient must have a name")
    }
    return nil
}

func extractFamilyName(resource []byte) (string, error) {
    expr, err := extractionCache.Get("Patient.name.first().family")
    if err != nil {
        return "", err
    }
    result, err := expr.Evaluate(resource)
    if err != nil {
        return "", err
    }
    if first, ok := result.First(); ok {
        return first.String(), nil
    }
    return "", nil
}
```

---

## LRU Eviction Behavior

When the cache is full and a new expression is compiled:

1. The **least recently used** entry (the one that has gone the longest without a `Get` call) is evicted.
2. The new entry is placed at the front of the LRU list.
3. Every `Get` call (hit or miss) updates the entry's position in the LRU list.

This means frequently used expressions stay in the cache while rarely used ones are evicted first. If your application has a stable set of expressions smaller than the cache limit, the cache will eventually reach a 100% hit rate.
