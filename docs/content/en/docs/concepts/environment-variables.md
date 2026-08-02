---
title: "Environment Variables"
linkTitle: "Environment Variables"
description: "How to use built-in FHIRPath environment variables (%resource, %context, %ucum) and define custom variables with WithVariable()."
weight: 4
---

FHIRPath environment variables are special identifiers prefixed with `%` that provide access to contextual information during expression evaluation. They are essential for writing FHIR® invariant constraints, referencing the root resource from nested paths, and injecting external data into expressions.

## Built-in Variables

The FHIRPath Go library automatically sets the following environment variables when an evaluation begins.

### %resource

`%resource` refers to the **root resource** being evaluated. It is automatically set to the top-level resource passed to `Evaluate`, `EvaluateCached`, or `Expression.Evaluate`.

This variable is required by many FHIR® StructureDefinition constraints (invariants) that need to reference the root resource from within a nested context. For example, the FHIR® Bundle invariant `bdl-3` uses `%resource` to reference the Bundle from within an entry:

```text
// FHIR invariant bdl-3: fullUrl must be unique within a Bundle
%resource.entry.where(fullUrl.exists()).select(fullUrl).isDistinct()
```

In a Go program:

```go
bundle := []byte(`{
    "resourceType": "Bundle",
    "type": "collection",
    "entry": [
        {"fullUrl": "urn:uuid:1", "resource": {"resourceType": "Patient", "id": "1"}},
        {"fullUrl": "urn:uuid:2", "resource": {"resourceType": "Patient", "id": "2"}}
    ]
}`)

result, err := fhirpath.Evaluate(bundle, "%resource.entry.count()")
// result: [2]
```

### %context

`%context` represents the **original node** passed to the evaluation engine. For top-level evaluation (the most common case), `%context` is the same as `%resource`. The distinction matters in advanced scenarios where an expression is evaluated against a sub-node of a resource.

```text
// For top-level evaluation, these are equivalent:
%resource.id
%context.id
```

Both `%resource` and `%context` are set automatically by the `eval.NewContext` constructor and require no manual configuration.

### %ucum

`%ucum` is a standard FHIRPath constant that resolves to the string `'http://unitsofmeasure.org'`. It is used in expressions that check the system of a Quantity's unit coding:

```text
Observation.value.ofType(Quantity).system = %ucum
```

This is shorthand for:

```text
Observation.value.ofType(Quantity).system = 'http://unitsofmeasure.org'
```

Several FHIR® invariants marked SHALL depend on it, among them `age-1`, `drt-1`, `cnt-3` and `dis-1`:

```text
// FHIR invariant cnt-3 (Count)
(code.exists() or value.empty()) and (system.empty() or system = %ucum) and (code.empty() or code = '1')
```

### %sct and %loinc

Like `%ucum`, these resolve to the canonical URL of a code system:

| Variable  | Value                       |
|-----------|-----------------------------|
| `%sct`    | `'http://snomed.info/sct'`  |
| `%loinc`  | `'http://loinc.org'`        |

### %vs- and %ext-

FHIR® also defines two parameterized forms that expand to a canonical URL:

| Expression                     | Value                                                          |
|--------------------------------|----------------------------------------------------------------|
| `%'vs-administrative-gender'`  | `'http://hl7.org/fhir/ValueSet/administrative-gender'`         |
| `%'ext-patient-birthTime'`     | `'http://hl7.org/fhir/StructureDefinition/patient-birthTime'`  |

The FHIR® specification writes these with double quotes (`%"vs-[name]"`), which the FHIRPath grammar does not accept. Use single quotes or backticks instead: `%'vs-name'` or ``%`vs-name` ``.

### Overriding a fixed constant

`%ucum`, `%sct` and `%loinc` are set by `eval.NewContext` before evaluation, so [`WithVariable`](#custom-variables-with-withvariable) can replace any of them — useful for testing against a different terminology server:

```go
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithVariable("loinc", types.Collection{types.NewString("http://example.org/loinc")}),
)
```

## Custom Variables with WithVariable()

You can inject your own environment variables into an evaluation using the `WithVariable` functional option. Custom variables are accessed in FHIRPath expressions via the `%name` syntax, just like built-in variables.

### Basic Usage

```go
import (
    "fmt"
    "github.com/gofhir/fhirpath"
    "github.com/gofhir/fhirpath/types"
)

expr := fhirpath.MustCompile("Patient.name.where(family = %expectedName).exists()")

patient := []byte(`{
    "resourceType": "Patient",
    "name": [{"family": "Smith", "given": ["Jane"]}]
}`)

result, err := expr.EvaluateWithOptions(patient,
    fhirpath.WithVariable("expectedName", types.Collection{types.NewString("Smith")}),
)
if err != nil {
    panic(err)
}

fmt.Println(result) // [true]
```

### Multiple Variables

You can pass multiple `WithVariable` options to set several variables at once:

```go
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithVariable("minAge", types.Collection{types.NewInteger(18)}),
    fhirpath.WithVariable("maxAge", types.Collection{types.NewInteger(65)}),
    fhirpath.WithVariable("status", types.Collection{types.NewString("active")}),
)
```

### Variable Types

Variable values are `types.Collection` instances, so you can pass any FHIRPath value type:

```go
// String variable
fhirpath.WithVariable("system", types.Collection{types.NewString("http://example.org")})

// Integer variable
fhirpath.WithVariable("threshold", types.Collection{types.NewInteger(100)})

// Boolean variable
fhirpath.WithVariable("strict", types.Collection{types.NewBoolean(true)})

// Decimal variable
d, _ := types.NewDecimal("3.14")
fhirpath.WithVariable("pi", types.Collection{d})

// Empty variable (explicitly empty)
fhirpath.WithVariable("empty", types.Collection{})
```

### Use Cases

Custom variables are particularly useful for:

1. **Parameterized validation rules** -- pass thresholds, expected values, or configuration as variables rather than hard-coding them in expressions.

```go
// Validate that a patient's age exceeds a configurable minimum
expr := fhirpath.MustCompile(
    "Patient.birthDate <= today() - %minAge 'years'",
)
result, _ := expr.EvaluateWithOptions(patientJSON,
    fhirpath.WithVariable("minAge", types.Collection{types.NewInteger(18)}),
)
```

2. **Cross-resource references** -- pass data from one resource as a variable when evaluating another.

```go
// Check if a patient's identifier matches an expected value from another system
expr := fhirpath.MustCompile(
    "Patient.identifier.where(system = %targetSystem and value = %targetId).exists()",
)
result, _ := expr.EvaluateWithOptions(patientJSON,
    fhirpath.WithVariable("targetSystem", types.Collection{types.NewString("http://hospital.example.org")}),
    fhirpath.WithVariable("targetId", types.Collection{types.NewString("MRN-12345")}),
)
```

3. **Dynamic expression evaluation** -- when expressions are loaded from configuration or user input and need runtime parameters.

## Combining with Other Options

`WithVariable` can be combined with other evaluation options such as `WithTimeout`, `WithMaxDepth`, and `WithContext`:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithContext(ctx),
    fhirpath.WithTimeout(3*time.Second),
    fhirpath.WithMaxDepth(50),
    fhirpath.WithVariable("expected", types.Collection{types.NewString("active")}),
)
```

See the [Getting Started]({{< relref "../getting-started" >}}) guide for more on evaluation options.
