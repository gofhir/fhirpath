---
title: "Advanced Topics"
linkTitle: "Advanced Topics"
weight: 5
description: >
  Deep dives into caching, evaluation options, reference resolution, terminology services, performance tuning, and thread safety.
---

This section covers advanced features of the FHIRPath Go library that help you build
production-ready applications. Each topic builds on the core API introduced in the
[Getting Started](/docs/getting-started/) guide.

## What You Will Find Here

- **[Expression Caching](caching/)** -- Avoid redundant parsing with the built-in
  LRU expression cache. Learn how to use the global `DefaultCache`, create custom
  caches, monitor hit rates, and pre-warm caches at startup.

- **[Evaluation Options](options-and-context/)** -- Control evaluation behavior with
  timeouts, recursion limits, collection size caps, and custom variables via the
  functional options API.

- **[Custom Reference Resolvers](custom-resolvers/)** -- Implement the
  `ReferenceResolver` interface to let the `resolve()` function fetch referenced
  FHIR resources from HTTP endpoints, in-memory bundles, or any other data source.

- **[Terminology Services](terminology-services/)** -- Connect the `memberOf()` and
  `conformsTo()` functions to external terminology servers and profile validators
  by implementing the `TerminologyService` and `ProfileValidator` interfaces.

- **[Performance Guide](performance/)** -- Practical patterns for high-throughput
  evaluation: compile-once, expression caching, resource pre-serialization, early
  filtering, and avoiding unnecessary type conversions.

- **[Thread Safety](thread-safety/)** -- Understand the concurrency model: which
  objects are safe to share across goroutines and which must remain per-evaluation.
  Includes HTTP handler and worker pool examples.
