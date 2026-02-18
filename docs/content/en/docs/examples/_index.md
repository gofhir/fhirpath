---
title: "Examples"
linkTitle: "Examples"
weight: 6
description: >
  Practical examples and cookbook recipes for common FHIRPath evaluation tasks in Go.
---

This section contains hands-on examples that demonstrate how to use the FHIRPath Go library in real-world scenarios. Each page includes complete, runnable Go code alongside realistic FHIR® JSON resources so you can copy, paste, and adapt them to your own projects.

## What You Will Find Here

| Page | Description |
|------|-------------|
| [Basic Queries](basic-queries/) | Extract demographics, navigate nested structures, work with arrays, and use path syntax |
| [Filtering Data](filtering-data/) | Use `where()`, `select()`, `exists()`, `count()`, and `empty()` to filter and project FHIR® data |
| [FHIR® Validation](fhir-validation/) | Evaluate FHIR® constraints and invariants using Boolean expressions |
| [Working with Extensions](working-with-extensions/) | Access, check, and extract values from FHIR® extensions |
| [Quantities and Units](quantities-and-units/) | Compare and manipulate UCUM quantities in Observation values |
| [Real-World Patterns](real-world-patterns/) | Production patterns including HTTP middleware, batch pipelines, and error handling |

## Prerequisites

All examples assume you have the library installed:

```bash
go get github.com/gofhir/fhirpath
```

And imported in your Go files:

```go
import "github.com/gofhir/fhirpath"
```

Most examples work with raw JSON bytes (`[]byte`) as input, which means you do not need any FHIR® model library. You can load resources from files, HTTP responses, or databases -- anything that gives you the JSON as bytes.
