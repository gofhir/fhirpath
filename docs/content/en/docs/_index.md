---
title: "Documentation"
linkTitle: "Documentation"
description: "Complete documentation for the FHIRPath Go library -- a FHIRPath 2.0 expression evaluator for FHIR resources."
weight: 1
---

Welcome to the **FHIRPath Go** documentation. This library provides a complete, production-ready implementation of the [FHIRPath 2.0 specification](http://hl7.org/fhirpath/) for evaluating expressions against FHIR resources in Go.

## Where to Start

<div class="row">
<div class="col-md-6 mb-4">

### [Getting Started]({{< relref "getting-started" >}})
Install the library, write your first evaluation, learn about compiling and caching expressions, and explore the convenience functions.

</div>
<div class="col-md-6 mb-4">

### [Core Concepts]({{< relref "concepts" >}})
Understand the FHIRPath type system, collections and empty propagation, operators and three-valued logic, and environment variables.

</div>
</div>

## Key Features

- **95+ built-in functions** covering existence, filtering, subsetting, string manipulation, math, type checking, date/time operations, aggregation, and more.
- **Full FHIRPath 2.0 compliance** including three-valued Boolean logic, partial date/time precision, and UCUM quantity normalization.
- **Production ready** with thread-safe evaluation, LRU expression caching, configurable timeouts, and memory-efficient pooling.
- **Zero FHIR model dependency** -- works directly with raw JSON bytes, so you can use any FHIR model library or none at all.

## Package Overview

| Package | Description |
|---------|-------------|
| `github.com/gofhir/fhirpath` | Top-level API: `Evaluate`, `Compile`, `EvaluateCached`, convenience helpers |
| `github.com/gofhir/fhirpath/types` | FHIRPath type system: `Value`, `Collection`, `Boolean`, `Integer`, `Decimal`, `String`, `Date`, `DateTime`, `Time`, `Quantity` |
| `github.com/gofhir/fhirpath/eval` | Internal evaluation engine and operator implementations |
| `github.com/gofhir/fhirpath/funcs` | Built-in function registry (existence, filtering, strings, math, etc.) |
