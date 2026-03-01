---
title: "FHIRPath for Go"
description: "A complete FHIRPath 2.0 expression evaluator for FHIR® resources in Go, with 95+ functions, UCUM normalization, LRU caching, and thread-safe evaluation."
layout: hextra-home
---

<div class="hx:text-center hx:mt-24 hx:mb-6">
{{< hextra/hero-badge >}}
  <span>Open Source</span>
  {{< icon name="github" attributes="height=14" >}}
{{< /hextra/hero-badge >}}
</div>

<div class="hx:text-center hx:mt-8 hx:mb-6">
{{< hextra/hero-headline >}}
  FHIRPath for Go
{{< /hextra/hero-headline >}}
</div>

<div class="hx:text-center hx:mt-6 hx:mb-20">
{{< hextra/hero-subtitle >}}
  A fully compliant FHIRPath 2.0 expression evaluator built for Go.&nbsp;<br class="sm:hx:block hx:hidden" />Evaluate, validate, and extract data from FHIR® resources with ease.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:text-center hx:mb-32">
{{< hextra/hero-button text="Get Started" link="docs/getting-started" >}}
{{< hextra/hero-button text="View on GitHub" link="https://github.com/gofhir/fhirpath" style="background: transparent; border: 1px solid #e5e7eb; color: inherit;" >}}
</div>

<div class="hx:mt-32"></div>

## Features

<div class="hx:mt-8"></div>

{{< cards >}}
  {{< card link="docs/functions" title="95+ Built-in Functions" icon="puzzle" subtitle="Complete function library: existence, filtering, subsetting, strings, math, type checking, date/time, aggregation, and more." >}}
  {{< card link="docs/concepts" title="Full Spec Compliance" icon="badge-check" subtitle="FHIRPath 2.0 with three-valued Boolean logic, partial date/time precision, UCUM unit normalization, and all operator categories." >}}
  {{< card link="docs/advanced/performance" title="Production Ready" icon="lightning-bolt" subtitle="Thread-safe evaluation, LRU caching, configurable timeouts, memory-efficient pooling, and zero FHIR® model dependencies." >}}
{{< /cards >}}

<div class="hx:mt-32"></div>

## Quick Start

<div class="hx:mt-8"></div>

{{< callout type="info" >}}
Requires **Go 1.21** or later.
{{< /callout >}}

<div class="hx:mt-8"></div>

**Install the library:**

```bash
go get github.com/gofhir/fhirpath
```

<div class="hx:mt-8"></div>

**Evaluate a FHIRPath expression:**

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

    result, err := fhirpath.Evaluate(patient, "Patient.name.family")
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // [Doe]
}
```

<div class="hx:text-center hx:mt-16 hx:mb-32">
{{< hextra/hero-button text="Read the full guide →" link="docs/getting-started" >}}
</div>
