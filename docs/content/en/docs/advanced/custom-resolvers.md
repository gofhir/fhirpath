---
title: "Custom Reference Resolvers"
linkTitle: "Custom Reference Resolvers"
weight: 3
description: >
  Implement the ReferenceResolver interface to let the resolve() function fetch
  referenced FHIR resources from HTTP endpoints, in-memory bundles, or any data source.
---

## The ReferenceResolver Interface

FHIR resources frequently reference other resources. The FHIRPath `resolve()` function
follows those references and returns the referenced resource as part of the evaluation
result. To make `resolve()` work, you need to provide a `ReferenceResolver` that knows
how to fetch resources given a reference string.

The interface is intentionally minimal:

```go
// ReferenceResolver resolves FHIR references for the resolve() function.
type ReferenceResolver interface {
    // Resolve takes a reference string (e.g., "Patient/123") and returns
    // the resource as raw JSON bytes.
    Resolve(ctx context.Context, reference string) ([]byte, error)
}
```

Key points:

- The `reference` parameter is the raw string extracted from a FHIR `Reference.reference`
  field. It may be a relative reference (`"Patient/123"`), an absolute URL
  (`"http://example.org/fhir/Patient/123"`), or a fragment (`"#contained-1"`).
- The resolver must return the resource as **JSON bytes** (`[]byte`).
- The `ctx` parameter carries the evaluation timeout and cancellation signal.
  Respect it in any I/O operations.
- If the reference cannot be resolved, return an error. The `resolve()` function
  will silently skip unresolvable references and continue with the next item.

## Simple HTTP Resolver

The most common use case is resolving references against a remote FHIR server:

```go
package main

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/gofhir/fhirpath"
)

// HTTPResolver resolves FHIR references by making HTTP GET requests.
type HTTPResolver struct {
    BaseURL    string       // e.g., "http://hapi.fhir.org/baseR4"
    HTTPClient *http.Client
}

func (r *HTTPResolver) Resolve(ctx context.Context, reference string) ([]byte, error) {
    // Build the full URL.
    var url string
    if strings.HasPrefix(reference, "http://") || strings.HasPrefix(reference, "https://") {
        url = reference
    } else {
        url = strings.TrimRight(r.BaseURL, "/") + "/" + reference
    }

    // Create request with context for timeout propagation.
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := r.HTTPClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("HTTP GET %s: %w", url, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("HTTP GET %s returned %d", url, resp.StatusCode)
    }

    return io.ReadAll(resp.Body)
}

func main() {
    resolver := &HTTPResolver{
        BaseURL:    "http://hapi.fhir.org/baseR4",
        HTTPClient: &http.Client{Timeout: 10 * time.Second},
    }

    // An Observation that references a Patient.
    observation := []byte(`{
        "resourceType": "Observation",
        "subject": {
            "reference": "Patient/example"
        },
        "code": {
            "coding": [{"system": "http://loinc.org", "code": "29463-7"}]
        }
    }`)

    expr := fhirpath.MustCompile("Observation.subject.resolve().name.family")

    result, err := expr.EvaluateWithOptions(observation,
        fhirpath.WithResolver(resolver),
        fhirpath.WithTimeout(5 * time.Second),
    )
    if err != nil {
        fmt.Println("evaluation error:", err)
        return
    }
    fmt.Println(result) // The patient's family name, if the reference resolves.
}
```

## In-Memory Bundle Resolver

When working with FHIR Bundles, references are often internal to the bundle. An
in-memory resolver avoids any network calls:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/gofhir/fhirpath"
)

// BundleResolver resolves references within a pre-parsed FHIR Bundle.
type BundleResolver struct {
    // resources maps "ResourceType/id" to raw JSON bytes.
    resources map[string][]byte
}

// NewBundleResolver builds an index from a raw FHIR Bundle.
func NewBundleResolver(bundleJSON []byte) (*BundleResolver, error) {
    var bundle struct {
        Entry []struct {
            FullURL  string          `json:"fullUrl"`
            Resource json.RawMessage `json:"resource"`
        } `json:"entry"`
    }
    if err := json.Unmarshal(bundleJSON, &bundle); err != nil {
        return nil, fmt.Errorf("unmarshal bundle: %w", err)
    }

    resources := make(map[string][]byte, len(bundle.Entry))
    for _, entry := range bundle.Entry {
        // Index by fullUrl.
        if entry.FullURL != "" {
            resources[entry.FullURL] = entry.Resource
        }

        // Also index by "ResourceType/id" for relative references.
        var meta struct {
            ResourceType string `json:"resourceType"`
            ID           string `json:"id"`
        }
        if err := json.Unmarshal(entry.Resource, &meta); err == nil && meta.ID != "" {
            key := meta.ResourceType + "/" + meta.ID
            resources[key] = entry.Resource
        }
    }

    return &BundleResolver{resources: resources}, nil
}

func (r *BundleResolver) Resolve(_ context.Context, reference string) ([]byte, error) {
    // Try exact match first (handles both fullUrl and relative references).
    if data, ok := r.resources[reference]; ok {
        return data, nil
    }

    // Try matching the tail of fullUrl entries.
    for key, data := range r.resources {
        if strings.HasSuffix(key, "/"+reference) {
            return data, nil
        }
    }

    return nil, fmt.Errorf("reference not found in bundle: %s", reference)
}

func main() {
    bundle := []byte(`{
        "resourceType": "Bundle",
        "type": "transaction",
        "entry": [
            {
                "fullUrl": "urn:uuid:patient-1",
                "resource": {
                    "resourceType": "Patient",
                    "id": "patient-1",
                    "name": [{"family": "Smith", "given": ["Jane"]}]
                }
            },
            {
                "fullUrl": "urn:uuid:obs-1",
                "resource": {
                    "resourceType": "Observation",
                    "id": "obs-1",
                    "subject": {"reference": "Patient/patient-1"},
                    "code": {
                        "coding": [{"system": "http://loinc.org", "code": "29463-7"}]
                    }
                }
            }
        ]
    }`)

    resolver, err := NewBundleResolver(bundle)
    if err != nil {
        panic(err)
    }

    // Evaluate on a single entry's resource.
    observation := []byte(`{
        "resourceType": "Observation",
        "subject": {"reference": "Patient/patient-1"},
        "code": {
            "coding": [{"system": "http://loinc.org", "code": "29463-7"}]
        }
    }`)

    expr := fhirpath.MustCompile("Observation.subject.resolve().name.family")

    result, err := expr.EvaluateWithOptions(observation,
        fhirpath.WithResolver(resolver),
    )
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // [Smith]
}
```

## Error Handling

The `resolve()` function handles resolver errors gracefully:

1. If no resolver is configured, `resolve()` returns an **empty collection**.
2. If the resolver returns an error for a specific reference, that reference is
   **silently skipped** and the next item in the collection is tried.
3. If the returned JSON cannot be parsed, the item is skipped.

This design follows the FHIRPath specification, which states that `resolve()`
should not fail the entire expression when a reference cannot be followed.

```go
// A resolver that rejects certain references.
type SelectiveResolver struct {
    inner fhirpath.ReferenceResolver
}

func (r *SelectiveResolver) Resolve(ctx context.Context, ref string) ([]byte, error) {
    // Only resolve Patient references.
    if !strings.HasPrefix(ref, "Patient/") {
        return nil, fmt.Errorf("unsupported reference type: %s", ref)
    }
    return r.inner.Resolve(ctx, ref)
}
```

In the example above, non-Patient references will be silently excluded from the
result. The expression continues to evaluate without error.

### Logging Resolution Failures

If you want visibility into resolution failures, add logging inside your resolver:

```go
func (r *HTTPResolver) Resolve(ctx context.Context, reference string) ([]byte, error) {
    data, err := r.doResolve(ctx, reference)
    if err != nil {
        log.Printf("WARN: failed to resolve reference %q: %v", reference, err)
        return nil, err
    }
    return data, nil
}
```

## Wiring It Up

There are two ways to attach a resolver to an evaluation:

### Option 1: Functional Option (Recommended)

Use `WithResolver` when calling `EvaluateWithOptions`:

```go
expr := fhirpath.MustCompile("Observation.subject.resolve().name.family")

result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithResolver(myResolver),
    fhirpath.WithTimeout(3 * time.Second),
)
```

This is the recommended approach because it keeps the resolver scoped to a single
evaluation and composes cleanly with other options.

### Option 2: Direct Context Setup

For more control, create an `eval.Context` manually and set the resolver directly:

```go
import "github.com/gofhir/fhirpath/eval"

ctx := eval.NewContext(resource)
ctx.SetResolver(myResolverAdapter)
ctx.SetContext(requestCtx)
ctx.SetLimit("maxDepth", 100)
ctx.SetLimit("maxCollectionSize", 10000)

result, err := expr.EvaluateWithContext(ctx)
```

Note that when using the `eval.Context` directly, you must use an adapter that
implements the `eval.Resolver` interface (which has the same signature as
`fhirpath.ReferenceResolver`). The `WithResolver` option handles this adaptation
automatically.

## Summary

| Concept                  | Description                                                   |
|--------------------------|---------------------------------------------------------------|
| `ReferenceResolver`      | Interface with a single `Resolve(ctx, reference) ([]byte, error)` method |
| `WithResolver(r)`        | Functional option to attach a resolver to an evaluation       |
| HTTP Resolver            | Resolves references by fetching from a FHIR REST API         |
| Bundle Resolver          | Resolves references within a pre-indexed FHIR Bundle         |
| Error behavior           | Unresolvable references are silently skipped                 |
| No resolver configured   | `resolve()` returns an empty collection                      |
