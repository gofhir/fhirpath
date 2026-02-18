---
title: "API Reference"
linkTitle: "API Reference"
weight: 3
description: >
  Complete reference for the FHIRPath Go library public API.
---

The `github.com/gofhir/fhirpath` package provides a complete FHIRPath 2.0 expression evaluator for FHIR® resources in Go. This section documents every public function, type, and interface available in the library.

## Package Overview

The library is organized into two packages:

| Package | Import Path | Description |
|---------|-------------|-------------|
| **fhirpath** | `github.com/gofhir/fhirpath` | Core evaluation engine, compilation, caching, and options |
| **types** | `github.com/gofhir/fhirpath/types` | FHIRPath type system: Value, Collection, and all primitive types |

## Quick Navigation

### Core Evaluation

- **[Evaluate Functions](evaluate/)** -- `Evaluate`, `MustEvaluate`, and `EvaluateCached` for one-shot expression evaluation against JSON resources.
- **[Compile and Expression](compile/)** -- `Compile`, `MustCompile`, and the `Expression` type for precompiling expressions and evaluating them many times.
- **[Typed Evaluation](typed-evaluation/)** -- Convenience functions that return Go-native types: `EvaluateToBoolean`, `EvaluateToString`, `EvaluateToStrings`, `Exists`, and `Count`.

### Resource Handling

- **[Resource Interface](resource/)** -- The `Resource` interface, `EvaluateResource`, `EvaluateResourceCached`, and `ResourceJSON` for evaluating Go structs directly.

### Performance and Configuration

- **[Expression Cache](cache/)** -- `ExpressionCache` with LRU eviction, `DefaultCache`, cache statistics, and monitoring.
- **[Evaluation Options](options/)** -- `EvalOptions`, functional options (`WithTimeout`, `WithContext`, `WithVariable`, etc.), and the `ReferenceResolver` interface.

### Type System

- **[Types Package](types/)** -- The `Value` interface, `Collection` type with all its methods, and all FHIRPath primitive types (`Boolean`, `Integer`, `Decimal`, `String`, `Date`, `DateTime`, `Time`, `Quantity`, `ObjectValue`).

## Choosing the Right Function

Use the following decision tree to pick the best entry point for your use case:

```text
Do you have a Go struct implementing Resource?
  YES --> Do you evaluate many expressions on it?
            YES --> Use ResourceJSON (serialize once, evaluate many)
            NO  --> Use EvaluateResource / EvaluateResourceCached
  NO  --> (You have raw JSON bytes)
          Do you reuse the same expression many times?
            YES --> Use Compile + Expression.Evaluate
                    (or ExpressionCache for automatic caching)
            NO  --> Do you need a specific Go type back?
                      YES --> Use EvaluateToBoolean / EvaluateToString / Exists / Count
                      NO  --> Use Evaluate or EvaluateCached
```
