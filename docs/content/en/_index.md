---
title: "FHIRPath for Go"
description: "A complete FHIRPath 2.0 expression evaluator for FHIR® resources in Go, with 95+ functions, UCUM normalization, LRU caching, and thread-safe evaluation."
layout: hextra-home
---

<div class="hx-mt-6 hx-mb-6">
{{< hextra/hero-badge >}}
  <span>Open Source</span>
  {{< icon name="github" attributes="height=14" >}}
{{< /hextra/hero-badge >}}
</div>

<div class="hx-mt-6 hx-mb-6">
{{< hextra/hero-headline >}}
  FHIRPath for Go
{{< /hextra/hero-headline >}}
</div>

<div class="hx-mb-12">
{{< hextra/hero-subtitle >}}
  A fully compliant FHIRPath 2.0 expression evaluator built for Go&mdash;evaluate, validate, and extract data from FHIR® resources with ease.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx-mb-6">
{{< hextra/hero-button text="Get Started" link="docs/getting-started" >}}
</div>

## Why FHIRPath Go? {.hx-mt-6}

{{< cards >}}
  {{< card link="docs/getting-started" title="95+ Built-in Functions" icon="puzzle" subtitle="Comprehensive function library covering existence, filtering, subsetting, string manipulation, math, type checking, date/time operations, aggregation, and more." >}}
  {{< card link="docs/concepts" title="Full Spec Compliance" icon="badge-check" subtitle="Implements the complete FHIRPath 2.0 specification including three-valued Boolean logic, partial date/time precision, UCUM unit normalization, and all operator categories." >}}
  {{< card link="docs/getting-started" title="Production Ready" icon="lightning-bolt" subtitle="Thread-safe concurrent evaluation, LRU expression caching, configurable timeouts and depth limits, memory-efficient object pooling, and zero external FHIR® model dependencies." >}}
{{< /cards >}}

## Quick Start {.hx-mt-6}

### Install

```bash
go get github.com/gofhir/fhirpath
```

### Evaluate a FHIRPath expression in five lines

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func main() {
    patient := []byte(`{"resourceType":"Patient","name":[{"family":"Doe","given":["John"]}]}`)
    result, err := fhirpath.Evaluate(patient, "Patient.name.family")
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // [Doe]
}
```

{{< hextra/hero-button text="Read the full guide" link="docs/getting-started" >}}
