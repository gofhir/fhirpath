---
title: "Function Reference"
linkTitle: "Function Reference"
weight: 4
description: >
  Complete reference for all FHIRPath functions supported by the Go FHIRPath library.
---

The FHIRPath specification defines a rich set of functions for navigating, filtering, and transforming FHIR® data. This library implements the full function catalog from the [FHIRPath 2.0 specification](http://hl7.org/fhirpath/), along with FHIR®-specific extensions.

Functions are invoked using dot notation on a collection:

```
Patient.name.where(use = 'official').first().given
```

All functions operate on collections and return collections, following FHIRPath's consistent collection-based model. When a function is called on an empty collection, it typically returns an empty collection (propagating emptiness).

## Function Categories

| Category | Functions | Description |
|----------|-----------|-------------|
| [String Functions]({{< relref "strings" >}}) | 16 | Text manipulation: `startsWith`, `contains`, `replace`, `matches`, `substring`, `lower`, `upper`, `split`, `join`, and more |
| [Math Functions]({{< relref "math" >}}) | 10 | Numeric operations: `abs`, `ceiling`, `floor`, `round`, `sqrt`, `power`, `ln`, `log`, `exp`, `truncate` |
| [Existence Functions]({{< relref "existence" >}}) | 12 | Collection testing: `empty`, `exists`, `all`, `count`, `distinct`, `allTrue`, `anyTrue`, `subsetOf`, `supersetOf`, and more |
| [Filtering Functions]({{< relref "filtering" >}}) | 4 | Collection filtering: `where`, `select`, `repeat`, `ofType` |
| [Subsetting Functions]({{< relref "subsetting" >}}) | 8 | Collection slicing: `first`, `last`, `tail`, `take`, `skip`, `single`, `intersect`, `exclude` |
| [Combining Functions]({{< relref "combining" >}}) | 2 | Merging collections: `union`, `combine` |
| [Conversion Functions]({{< relref "conversion" >}}) | 17 | Type conversion: `iif`, `toBoolean`, `toInteger`, `toDecimal`, `toString`, `toDate`, `toDateTime`, `toTime`, `toQuantity`, and `convertsTo*` variants |
| [Type Checking Functions]({{< relref "type-checking" >}}) | 3 | Type inspection: `is`, `as`, `ofType` |
| [Temporal Functions]({{< relref "temporal" >}}) | 10 | Date/time operations: `now`, `today`, `timeOfDay`, `year`, `month`, `day`, `hour`, `minute`, `second`, `millisecond` |
| [Utility Functions]({{< relref "utility" >}}) | 3 | Debugging and navigation: `trace`, `children`, `descendants` |
| [FHIR®-Specific Functions]({{< relref "fhir-specific" >}}) | 8 | FHIR® extensions: `extension`, `hasExtension`, `resolve`, `memberOf`, `conformsTo`, `hasValue`, `getValue`, `getReferenceKey` |
| [Aggregate Functions]({{< relref "aggregate" >}}) | 5 | Reduction operations: `aggregate`, `sum`, `avg`, `min`, `max` |

## Common Patterns

### Empty Collection Propagation

Most functions return an empty collection when called on an empty input:

```go
result, _ := fhirpath.Evaluate(resource, "Patient.deceased.startsWith('abc')")
// If Patient.deceased is absent, result is empty -- not an error
```

### Singleton Evaluation

Functions that expect a single value (such as string functions) operate on the first element of the input collection. If the collection contains more than one element, some functions may return an error or operate only on the first element.

### Type Safety

The library performs runtime type checking. If a function receives an input of an unexpected type, it returns an empty collection rather than raising an error, consistent with the FHIRPath specification.
