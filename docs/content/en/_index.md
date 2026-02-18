---
title: "FHIRPath for Go"
description: "A complete FHIRPath 2.0 expression evaluator for FHIR resources in Go, with 95+ functions, UCUM normalization, LRU caching, and thread-safe evaluation."
---

{{< blocks/cover title="FHIRPath for Go" image_anchor="top" height="full" >}}
<a class="btn btn-lg btn-primary me-3 mb-4" href="{{< relref "/docs" >}}">
Documentation <i class="fas fa-arrow-alt-circle-right ms-2"></i>
</a>
<a class="btn btn-lg btn-secondary me-3 mb-4" href="https://github.com/gofhir/fhirpath">
GitHub <i class="fab fa-github ms-2"></i>
</a>
<p class="lead mt-5">A fully compliant FHIRPath 2.0 expression evaluator built for Go &mdash; evaluate, validate, and extract data from FHIR resources with ease.</p>
{{< blocks/link-down color="info" >}}
{{< /blocks/cover >}}


{{% blocks/lead color="primary" %}}
**FHIRPath Go** is a production-ready, open-source library that implements the
[FHIRPath 2.0 specification](http://hl7.org/fhirpath/) for evaluating expressions
against FHIR resources in Go applications. It ships with 95+ built-in functions,
automatic UCUM quantity normalization, an LRU expression cache, and a fully
thread-safe evaluation engine.
{{% /blocks/lead %}}


{{% blocks/section color="dark" type="row" %}}

{{% blocks/feature icon="fa-solid fa-puzzle-piece" title="95+ Built-in Functions" url="/docs/getting-started/" %}}
Comprehensive function library covering existence, filtering, subsetting,
string manipulation, math, type checking, date/time operations, aggregation,
and more. Every function from the FHIRPath specification is implemented.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-certificate" title="Full Spec Compliance" url="/docs/concepts/" %}}
Implements the complete FHIRPath 2.0 specification including three-valued
Boolean logic, partial date/time precision, UCUM unit normalization, and all
operator categories. Validated against the official FHIRPath test suite.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-bolt" title="Production Ready" url="/docs/getting-started/" %}}
Thread-safe concurrent evaluation, LRU expression caching, configurable
timeouts and depth limits, memory-efficient object pooling, and zero
external FHIR model dependencies. Ready for high-throughput services.
{{% /blocks/feature %}}

{{% /blocks/section %}}


{{< blocks/section color="light" >}}
<div class="col-lg-8 mx-auto">
  <h2 class="text-center mb-4">Quick Start</h2>

  <h5><i class="fa-solid fa-download me-2 text-primary"></i>Install</h5>

{{< highlight bash >}}
go get github.com/gofhir/fhirpath
{{< /highlight >}}

  <h5 class="mt-4"><i class="fa-solid fa-code me-2 text-primary"></i>Evaluate a FHIRPath expression in five lines</h5>

{{< highlight go >}}
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
{{< /highlight >}}

  <div class="text-center mt-4">
    <a class="btn btn-lg btn-primary" href="{{< relref "/docs/getting-started" >}}">
      Read the full guide <i class="fas fa-arrow-right ms-2"></i>
    </a>
  </div>
</div>
{{< /blocks/section >}}
