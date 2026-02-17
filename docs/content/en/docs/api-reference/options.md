---
title: "Evaluation Options"
linkTitle: "Options"
weight: 6
description: >
  Configure evaluation behavior with timeouts, depth limits, variables, and reference resolution.
---

The options API lets you customize how FHIRPath expressions are evaluated. Options are applied using Go's functional options pattern, giving you fine-grained control over timeouts, recursion limits, external variables, and reference resolution.

## EvalOptions

The `EvalOptions` struct holds all configuration for an evaluation run. You rarely construct this directly; instead, use the functional option functions to build options incrementally.

```go
type EvalOptions struct {
    // Ctx is the context for cancellation and timeout.
    Ctx context.Context

    // Timeout for evaluation (0 means no timeout).
    Timeout time.Duration

    // MaxDepth limits recursion depth for descendants() (0 means default of 100).
    MaxDepth int

    // MaxCollectionSize limits output collection size (0 means no limit).
    MaxCollectionSize int

    // Variables are external variables accessible via %name in expressions.
    Variables map[string]types.Collection

    // Resolver handles reference resolution for the resolve() function.
    Resolver ReferenceResolver
}
```

---

## DefaultOptions

Returns a new `EvalOptions` pre-configured with sensible production defaults.

```go
func DefaultOptions() *EvalOptions
```

**Default values:**

| Field | Default Value |
|-------|---------------|
| `Ctx` | `context.Background()` |
| `Timeout` | `5 * time.Second` |
| `MaxDepth` | `100` |
| `MaxCollectionSize` | `10000` |
| `Variables` | Empty map |
| `Resolver` | `nil` |

When you call `Expression.EvaluateWithOptions` without any options, these defaults are used.

---

## Functional Options

Each functional option is a function of type `EvalOption` that modifies `EvalOptions`:

```go
type EvalOption func(*EvalOptions)
```

### WithContext

Sets the `context.Context` for cancellation support. If the context is canceled during evaluation, the evaluation returns immediately with the context's error.

```go
func WithContext(ctx context.Context) EvalOption
```

**Example:**

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithContext(ctx),
)
```

---

### WithTimeout

Sets the maximum time allowed for a single evaluation. If the evaluation takes longer than the specified duration, it is canceled and an error is returned. A zero value disables the timeout.

```go
func WithTimeout(d time.Duration) EvalOption
```

**Example:**

```go
// Allow at most 2 seconds for evaluation
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithTimeout(2*time.Second),
)
```

{{% alert title="Note" color="info" %}}
When both `WithContext` and `WithTimeout` are provided, the effective deadline is the earlier of the two. `WithTimeout` internally creates a child context with the given timeout.
{{% /alert %}}

---

### WithMaxDepth

Sets the maximum recursion depth for functions like `descendants()` that traverse the resource tree. This prevents stack overflows on deeply nested or circular structures. The default is 100.

```go
func WithMaxDepth(depth int) EvalOption
```

**Example:**

```go
// Limit recursion to 50 levels
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithMaxDepth(50),
)
```

---

### WithMaxCollectionSize

Sets the maximum number of values allowed in a result collection. This prevents excessive memory usage when expressions produce very large result sets. The default is 10000.

```go
func WithMaxCollectionSize(size int) EvalOption
```

**Example:**

```go
// Limit results to 1000 values
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithMaxCollectionSize(1000),
)
```

---

### WithVariable

Defines an external variable that can be referenced in the FHIRPath expression using the `%name` syntax. Multiple `WithVariable` calls can be chained.

```go
func WithVariable(name string, value types.Collection) EvalOption
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `name` | `string` | Variable name (referenced as `%name` in expressions) |
| `value` | `types.Collection` | The variable's value as a Collection |

**Example:**

```go
import "github.com/gofhir/fhirpath/types"

// Define a variable %maxAge that can be used in the expression
expr := fhirpath.MustCompile("Patient.birthDate < today() - %maxAge")

result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithVariable("maxAge", types.Collection{types.NewString("65 years")}),
)
```

---

### WithResolver

Provides a `ReferenceResolver` implementation for the FHIRPath `resolve()` function. When an expression calls `resolve()` on a FHIR reference, the resolver fetches the referenced resource.

```go
func WithResolver(r ReferenceResolver) EvalOption
```

**Example:**

```go
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithResolver(myResolver),
)
```

See [ReferenceResolver](#referenceresolver-interface) below for details.

---

## ReferenceResolver Interface

The `ReferenceResolver` interface is implemented by your application to provide reference resolution for the FHIRPath `resolve()` function. When an expression evaluates `Reference.resolve()`, the library calls your resolver with the reference string.

```go
type ReferenceResolver interface {
    Resolve(ctx context.Context, reference string) ([]byte, error)
}
```

**Parameters passed to Resolve:**

| Name | Type | Description |
|------|------|-------------|
| `ctx` | `context.Context` | The evaluation context (respects cancellation/timeout) |
| `reference` | `string` | The FHIR reference string (e.g., `"Patient/123"`, `"http://example.com/fhir/Patient/123"`) |

**Returns:**

| Type | Description |
|------|-------------|
| `[]byte` | Raw JSON bytes of the referenced resource |
| `error` | Non-nil if the reference cannot be resolved |

### Example Implementation: HTTP Resolver

```go
type HTTPResolver struct {
    BaseURL    string
    HTTPClient *http.Client
}

func (r *HTTPResolver) Resolve(ctx context.Context, reference string) ([]byte, error) {
    url := r.BaseURL + "/" + reference

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := r.HTTPClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("fetching %s: %w", reference, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, reference)
    }

    return io.ReadAll(resp.Body)
}
```

### Example Implementation: In-Memory Bundle Resolver

```go
type BundleResolver struct {
    resources map[string][]byte
}

func NewBundleResolver(bundle []byte) (*BundleResolver, error) {
    resolver := &BundleResolver{
        resources: make(map[string][]byte),
    }

    // Parse bundle and index resources by their reference key
    // (e.g., "Patient/123")
    var b struct {
        Entry []struct {
            Resource json.RawMessage `json:"resource"`
        } `json:"entry"`
    }
    if err := json.Unmarshal(bundle, &b); err != nil {
        return nil, err
    }

    for _, entry := range b.Entry {
        var meta struct {
            ResourceType string `json:"resourceType"`
            ID           string `json:"id"`
        }
        if err := json.Unmarshal(entry.Resource, &meta); err == nil {
            key := meta.ResourceType + "/" + meta.ID
            resolver.resources[key] = entry.Resource
        }
    }

    return resolver, nil
}

func (r *BundleResolver) Resolve(ctx context.Context, reference string) ([]byte, error) {
    if data, ok := r.resources[reference]; ok {
        return data, nil
    }
    return nil, fmt.Errorf("resource not found: %s", reference)
}
```

---

## Combining Multiple Options

Options are variadic, so you can pass as many as needed. They are applied in order, with later options overriding earlier ones for the same field.

```go
expr := fhirpath.MustCompile(
    "Observation.subject.resolve().name.family",
)

resolver := &HTTPResolver{
    BaseURL:    "https://fhir.example.com",
    HTTPClient: &http.Client{Timeout: 10 * time.Second},
}

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := expr.EvaluateWithOptions(observationJSON,
    fhirpath.WithContext(ctx),
    fhirpath.WithTimeout(10*time.Second),
    fhirpath.WithMaxDepth(50),
    fhirpath.WithMaxCollectionSize(500),
    fhirpath.WithVariable("today", types.Collection{types.NewString("2025-01-15")}),
    fhirpath.WithResolver(resolver),
)
if err != nil {
    log.Fatal(err)
}
fmt.Println(result)
```

---

## Options Summary

| Option | Default | Description |
|--------|---------|-------------|
| `WithContext` | `context.Background()` | Sets the cancellation context |
| `WithTimeout` | `5s` | Maximum evaluation time |
| `WithMaxDepth` | `100` | Maximum recursion depth for `descendants()` |
| `WithMaxCollectionSize` | `10000` | Maximum result collection size |
| `WithVariable` | None | Defines external variables accessible via `%name` |
| `WithResolver` | `nil` | Provides reference resolution for `resolve()` |
