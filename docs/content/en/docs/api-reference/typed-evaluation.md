---
title: "Typed Evaluation"
linkTitle: "Typed Evaluation"
weight: 3
description: >
  Convenience functions that return Go-native types instead of Collections.
---

The typed evaluation functions wrap `EvaluateCached` and convert the result to a specific Go type. They simplify common patterns like checking existence, counting results, or extracting a single string or boolean value.

All of these functions use the `DefaultCache` internally, so repeated calls with the same expression benefit from automatic caching.

## EvaluateToBoolean

Evaluates a FHIRPath expression and returns the result as a Go `bool`. Returns `false` if the result is empty. Returns an error if the result contains more than one value or if the single value is not a Boolean.

```go
func EvaluateToBoolean(resource []byte, expr string) (bool, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `[]byte` | Raw JSON bytes of a FHIR® resource |
| `expr` | `string` | A FHIRPath expression that should yield a single Boolean |

**Returns:**

| Type | Description |
|------|-------------|
| `bool` | The boolean result, or `false` if the result is empty |
| `error` | Non-nil on compilation/evaluation errors, multiple results, or non-Boolean result |

**Example:**

```go
patient := []byte(`{
    "resourceType": "Patient",
    "active": true,
    "name": [{"family": "Smith"}]
}`)

// Check a boolean field
active, err := fhirpath.EvaluateToBoolean(patient, "Patient.active")
if err != nil {
    log.Fatal(err)
}
fmt.Println(active) // true

// Boolean expressions also work
hasName, err := fhirpath.EvaluateToBoolean(patient, "Patient.name.exists()")
if err != nil {
    log.Fatal(err)
}
fmt.Println(hasName) // true
```

---

## EvaluateToString

Evaluates a FHIRPath expression and returns the result as a Go `string`. Returns an empty string if the result is empty. If the single result is a `types.String`, its raw value is returned; otherwise, the value's `String()` representation is used. Returns an error if the result contains more than one value.

```go
func EvaluateToString(resource []byte, expr string) (string, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `[]byte` | Raw JSON bytes of a FHIR® resource |
| `expr` | `string` | A FHIRPath expression that should yield a single value |

**Returns:**

| Type | Description |
|------|-------------|
| `string` | The string result, or `""` if the result is empty |
| `error` | Non-nil on compilation/evaluation errors, or if the result has more than one value |

**Example:**

```go
patient := []byte(`{
    "resourceType": "Patient",
    "name": [{"family": "Johnson", "given": ["Alice"]}],
    "birthDate": "1985-03-22"
}`)

family, err := fhirpath.EvaluateToString(patient, "Patient.name.first().family")
if err != nil {
    log.Fatal(err)
}
fmt.Println(family) // Johnson

birthDate, err := fhirpath.EvaluateToString(patient, "Patient.birthDate")
if err != nil {
    log.Fatal(err)
}
fmt.Println(birthDate) // 1985-03-22
```

---

## EvaluateToStrings

Evaluates a FHIRPath expression and returns all results as a `[]string`. Each element is converted to its string representation. Unlike `EvaluateToString`, this function handles collections of any size.

```go
func EvaluateToStrings(resource []byte, expr string) ([]string, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `[]byte` | Raw JSON bytes of a FHIR® resource |
| `expr` | `string` | A FHIRPath expression |

**Returns:**

| Type | Description |
|------|-------------|
| `[]string` | All result values as strings |
| `error` | Non-nil on compilation or evaluation errors |

**Example:**

```go
patient := []byte(`{
    "resourceType": "Patient",
    "name": [
        {"family": "Williams", "given": ["Robert", "James"]},
        {"family": "Bill", "given": ["Bob"]}
    ]
}`)

// Get all given names across all name entries
names, err := fhirpath.EvaluateToStrings(patient, "Patient.name.given")
if err != nil {
    log.Fatal(err)
}
fmt.Println(names) // [Robert James Bob]
```

---

## Exists

Evaluates a FHIRPath expression and returns `true` if the result collection is non-empty. This is equivalent to calling `Evaluate` and checking `!result.Empty()`, but more concise.

```go
func Exists(resource []byte, expr string) (bool, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `[]byte` | Raw JSON bytes of a FHIR® resource |
| `expr` | `string` | A FHIRPath expression |

**Returns:**

| Type | Description |
|------|-------------|
| `bool` | `true` if at least one result exists |
| `error` | Non-nil on compilation or evaluation errors |

**Example:**

```go
patient := []byte(`{
    "resourceType": "Patient",
    "telecom": [
        {"system": "phone", "value": "555-0100"}
    ]
}`)

hasPhone, err := fhirpath.Exists(patient, "Patient.telecom.where(system = 'phone')")
if err != nil {
    log.Fatal(err)
}
fmt.Println(hasPhone) // true

hasEmail, err := fhirpath.Exists(patient, "Patient.telecom.where(system = 'email')")
if err != nil {
    log.Fatal(err)
}
fmt.Println(hasEmail) // false
```

---

## Count

Evaluates a FHIRPath expression and returns the number of values in the result collection.

```go
func Count(resource []byte, expr string) (int, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `[]byte` | Raw JSON bytes of a FHIR® resource |
| `expr` | `string` | A FHIRPath expression |

**Returns:**

| Type | Description |
|------|-------------|
| `int` | The number of result values |
| `error` | Non-nil on compilation or evaluation errors |

**Example:**

```go
patient := []byte(`{
    "resourceType": "Patient",
    "name": [
        {"family": "Smith", "given": ["John", "Jacob"]},
        {"family": "Doe"}
    ],
    "address": [
        {"city": "Springfield"},
        {"city": "Shelbyville"},
        {"city": "Capital City"}
    ]
}`)

nameCount, err := fhirpath.Count(patient, "Patient.name")
if err != nil {
    log.Fatal(err)
}
fmt.Println(nameCount) // 2

addressCount, err := fhirpath.Count(patient, "Patient.address")
if err != nil {
    log.Fatal(err)
}
fmt.Println(addressCount) // 3
```

---

## Summary

| Function | Return Type | Empty Result | Multiple Results | Caching |
|----------|-------------|--------------|------------------|---------|
| `EvaluateToBoolean` | `bool` | `false` | Error | Yes |
| `EvaluateToString` | `string` | `""` | Error | Yes |
| `EvaluateToStrings` | `[]string` | `[]string{}` | All converted | Yes |
| `Exists` | `bool` | `false` | `true` | Yes |
| `Count` | `int` | `0` | Returns count | Yes |

All functions use `EvaluateCached` internally, so the first call for a given expression incurs compilation cost, and all subsequent calls are served from the `DefaultCache`.

## Practical Patterns

### Validation with EvaluateToBoolean

```go
func validatePatient(resource []byte) error {
    // Check required fields
    hasName, err := fhirpath.EvaluateToBoolean(resource, "Patient.name.exists()")
    if err != nil {
        return fmt.Errorf("validation error: %w", err)
    }
    if !hasName {
        return fmt.Errorf("Patient must have at least one name")
    }
    return nil
}
```

### Extracting Lists with EvaluateToStrings

```go
func getAllIdentifiers(resource []byte) ([]string, error) {
    return fhirpath.EvaluateToStrings(resource, "Patient.identifier.value")
}
```

### Conditional Logic with Exists

```go
func isDeceased(resource []byte) (bool, error) {
    return fhirpath.Exists(resource, "Patient.deceased.where($this = true)")
}
```
