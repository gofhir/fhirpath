---
title: "FHIR Version-Specific Models"
linkTitle: "FHIR Models"
weight: 4
description: >
  Use the Model interface to provide FHIR version-specific type metadata for precise
  polymorphic resolution, type hierarchy checking, and path-based inference.
---

## The Model Interface

The FHIRPath engine can operate in two modes:

- **Without a model** (default): uses built-in heuristics for type resolution. This works
  for most common cases but can produce incorrect results for advanced type hierarchy
  queries (e.g., `Age is Quantity` returns `false`).
- **With a model**: uses precise, version-specific metadata. The model is **authoritative** ---
  its answers override the built-in heuristics.

The `Model` interface provides seven methods:

```go
type Model interface {
    // ChoiceTypes returns the allowed types for a polymorphic element.
    // Example: ChoiceTypes("Observation.value") returns ["Quantity", "string", "boolean", ...]
    ChoiceTypes(path string) []string

    // TypeOf returns the FHIR type of an element.
    // Example: TypeOf("Patient.name") returns "HumanName"
    TypeOf(path string) string

    // ReferenceTargets returns the allowed target resource types for a Reference element.
    // Example: ReferenceTargets("Observation.subject") returns ["Patient", "Group", ...]
    ReferenceTargets(path string) []string

    // ParentType returns the parent type in the FHIR type hierarchy.
    // Example: ParentType("Patient") returns "DomainResource"
    ParentType(typeName string) string

    // IsSubtype returns true if child is a subtype of parent (transitive).
    // Example: IsSubtype("Patient", "Resource") returns true
    IsSubtype(child, parent string) bool

    // ResolvePath resolves content references to their canonical path.
    // Example: ResolvePath("Questionnaire.item.item") returns "Questionnaire.item"
    ResolvePath(path string) string

    // IsResource returns true if the type is a known FHIR resource type.
    // Example: IsResource("Patient") returns true, IsResource("HumanName") returns false
    IsResource(typeName string) bool
}
```

The interface uses **Go structural typing** (duck typing): any type that implements these
seven methods satisfies `Model`. No import dependency is required between the model
package and the FHIRPath engine.

## Why Use a Model?

| Feature | Without Model | With Model |
|---------|--------------|------------|
| Polymorphic resolution (`value[x]`) | Tries 39 hardcoded suffixes | Uses `ChoiceTypes()` for precise resolution |
| Type hierarchy (`is`, `as`, `ofType`) | Heuristic: PascalCase = resource | Uses `IsSubtype()` with full type chain |
| `Age is Quantity` | `false` | `true` (via `ParentType` chain) |
| `HumanName is Resource` | `true` (incorrect heuristic) | `false` (correct) |
| Content references | No resolution | Uses `ResolvePath()` |

## Using gofhir/models

The [`gofhir/models`](https://github.com/gofhir/models) package provides pre-generated
models for FHIR R4, R4B, and R5:

```go
package main

import (
    "fmt"

    "github.com/gofhir/fhirpath"
    "github.com/gofhir/models/r4"
)

func main() {
    observation := []byte(`{
        "resourceType": "Observation",
        "status": "final",
        "code": {"coding": [{"system": "http://loinc.org", "code": "29463-7"}]},
        "valueQuantity": {"value": 72, "unit": "kg"}
    }`)

    expr := fhirpath.MustCompile("Observation.value")

    // With R4 model --- precise polymorphic resolution
    result, err := expr.EvaluateWithOptions(observation,
        fhirpath.WithModel(r4.FHIRPathModel()),
    )
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // The Quantity value
}
```

For other FHIR versions, use the corresponding package:

```go
import "github.com/gofhir/models/r5"

result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithModel(r5.FHIRPathModel()),
)
```

## Custom Model Implementation

You can implement a custom model for testing or for FHIR profiles:

```go
type myModel struct{}

func (m *myModel) ChoiceTypes(path string) []string {
    if path == "Observation.value" {
        return []string{"Quantity", "string", "boolean"}
    }
    return nil
}

func (m *myModel) TypeOf(path string) string             { return "" }
func (m *myModel) ReferenceTargets(path string) []string  { return nil }
func (m *myModel) ParentType(typeName string) string      { return "" }
func (m *myModel) IsSubtype(child, parent string) bool    { return child == parent }
func (m *myModel) ResolvePath(path string) string         { return path }
func (m *myModel) IsResource(typeName string) bool        { return false }
```

{{< callout type="info" >}}
Methods that return zero values (empty string, nil slice, false) signal "no information
available". The engine uses its built-in heuristics as fallback for those specific lookups.
However, for type hierarchy queries (`IsSubtype`), the model is **authoritative** when
present --- the heuristic fallback is skipped entirely.
{{< /callout >}}

## Without a Model

When no model is provided, the engine uses built-in heuristics:

- **Polymorphic resolution**: tries all 39 known type suffixes (e.g., `valueQuantity`,
  `valueString`, `valueBoolean`, etc.)
- **Type hierarchy**: assumes PascalCase names that are not primitives are resource types
- **Resource/DomainResource**: uses a hardcoded list of non-DomainResource types
  (Bundle, Binary, Parameters)

This mode is fully backward compatible and works correctly for the majority of FHIRPath
expressions. Use a model when you need precise type hierarchy checking or work with
polymorphic elements across multiple FHIR versions.

## Summary

| Concept | Description |
|---------|-------------|
| `Model` interface | 7 methods for FHIR version-specific metadata |
| `WithModel(m)` | Functional option to attach a model to an evaluation |
| `gofhir/models` | Pre-generated models for R4, R4B, and R5 |
| Duck typing | No import dependency between model and engine |
| Authoritative | Model overrides heuristics for type hierarchy queries |
| Backward compatible | No model = same behavior as before |
