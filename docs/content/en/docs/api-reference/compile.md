---
title: "Compile and Expression"
linkTitle: "Compile"
weight: 2
description: >
  Pre-compile FHIRPath expressions for efficient repeated evaluation.
---

The compile functions parse a FHIRPath expression once and return an `Expression` object that can be evaluated many times against different resources. This is the "compile once, evaluate many" pattern and offers the best performance for hot paths.

## Compile

Parses a FHIRPath expression string and returns a compiled `Expression`. Returns an error if the expression is syntactically invalid.

```go
func Compile(expr string) (*Expression, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `expr` | `string` | A FHIRPath expression to compile |

**Returns:**

| Type | Description |
|------|-------------|
| `*Expression` | A compiled, reusable expression object |
| `error` | Non-nil if the expression has syntax errors |

**Example:**

```go
expr, err := fhirpath.Compile("Patient.name.where(use = 'official').family")
if err != nil {
    log.Fatalf("invalid expression: %v", err)
}

// Use expr.Evaluate() against many resources
for _, patient := range patients {
    result, err := expr.Evaluate(patient)
    if err != nil {
        log.Printf("evaluation error: %v", err)
        continue
    }
    fmt.Println(result)
}
```

---

## MustCompile

Like `Compile`, but panics on error. Ideal for package-level variables or initialization where a bad expression is a programming error.

```go
func MustCompile(expr string) *Expression
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `expr` | `string` | A FHIRPath expression to compile |

**Returns:**

| Type | Description |
|------|-------------|
| `*Expression` | A compiled, reusable expression object |

**Panics** if the expression is syntactically invalid.

**Example:**

```go
// Package-level compiled expressions -- compiled once at startup.
var (
    exprFamilyName = fhirpath.MustCompile("Patient.name.family")
    exprBirthDate  = fhirpath.MustCompile("Patient.birthDate")
    exprActive     = fhirpath.MustCompile("Patient.active")
)

func getPatientInfo(resource []byte) (string, error) {
    result, err := exprFamilyName.Evaluate(resource)
    if err != nil {
        return "", err
    }
    if first, ok := result.First(); ok {
        return first.String(), nil
    }
    return "", nil
}
```

---

## Expression Type

`Expression` represents a compiled FHIRPath expression. It holds the parsed AST (abstract syntax tree) and can be evaluated against any FHIR® resource.

```go
type Expression struct {
    // unexported fields
}
```

### Expression.Evaluate

Executes the compiled expression against a JSON FHIR® resource.

```go
func (e *Expression) Evaluate(resource []byte) (Collection, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `[]byte` | Raw JSON bytes of a FHIR® resource |

**Returns:**

| Type | Description |
|------|-------------|
| `Collection` | The evaluation result |
| `error` | Non-nil if evaluation fails |

**Example:**

```go
expr := fhirpath.MustCompile("Patient.telecom.where(system = 'phone').value")

patient := []byte(`{
    "resourceType": "Patient",
    "telecom": [
        {"system": "phone", "value": "555-0100"},
        {"system": "email", "value": "john@example.com"}
    ]
}`)

result, err := expr.Evaluate(patient)
if err != nil {
    log.Fatal(err)
}
fmt.Println(result) // [555-0100]
```

---

### Expression.EvaluateWithContext

Executes the expression with a custom evaluation context. This is a lower-level method that gives full control over the evaluation environment.

```go
func (e *Expression) EvaluateWithContext(ctx *eval.Context) (Collection, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `ctx` | `*eval.Context` | An evaluation context created by `eval.NewContext` |

**Returns:**

| Type | Description |
|------|-------------|
| `Collection` | The evaluation result |
| `error` | Non-nil if evaluation fails |

This method is intended for advanced use cases where you need direct access to the internal evaluation context (for example, setting variables or limits at a lower level). For most cases, prefer `EvaluateWithOptions`.

---

### Expression.EvaluateWithOptions

Executes the expression against a JSON resource with configurable options. Options are applied using the functional options pattern.

```go
func (e *Expression) EvaluateWithOptions(resource []byte, opts ...EvalOption) (Collection, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `[]byte` | Raw JSON bytes of a FHIR® resource |
| `opts` | `...EvalOption` | Zero or more functional options |

**Returns:**

| Type | Description |
|------|-------------|
| `Collection` | The evaluation result |
| `error` | Non-nil if evaluation fails |

When no options are provided, `DefaultOptions()` values are used (5-second timeout, max depth 100, max collection size 10000).

**Example:**

```go
expr := fhirpath.MustCompile("Patient.name.family")

result, err := expr.EvaluateWithOptions(patient,
    fhirpath.WithTimeout(2*time.Second),
    fhirpath.WithMaxDepth(50),
)
if err != nil {
    log.Fatal(err)
}
fmt.Println(result)
```

See [Evaluation Options](../options/) for the full list of available options.

---

### Expression.String

Returns the original FHIRPath expression string that was compiled.

```go
func (e *Expression) String() string
```

**Returns:**

| Type | Description |
|------|-------------|
| `string` | The original expression source text |

**Example:**

```go
expr := fhirpath.MustCompile("Patient.name.family")
fmt.Println(expr.String()) // Patient.name.family
```

---

## Compile Once, Evaluate Many

The recommended pattern for production code is to compile expressions at package initialization time and reuse them throughout the application lifetime:

```go
package patient

import "github.com/gofhir/fhirpath"

// Compiled once when the package loads.
var (
    nameExpr   = fhirpath.MustCompile("Patient.name.where(use = 'official').family")
    phoneExpr  = fhirpath.MustCompile("Patient.telecom.where(system = 'phone').value")
    activeExpr = fhirpath.MustCompile("Patient.active")
)

// GetOfficialName evaluates the pre-compiled expression against any Patient resource.
func GetOfficialName(patient []byte) (string, error) {
    result, err := nameExpr.Evaluate(patient)
    if err != nil {
        return "", err
    }
    if first, ok := result.First(); ok {
        return first.String(), nil
    }
    return "", nil
}

// GetPhoneNumbers returns all phone numbers for a Patient.
func GetPhoneNumbers(patient []byte) ([]string, error) {
    result, err := phoneExpr.Evaluate(patient)
    if err != nil {
        return nil, err
    }
    phones := make([]string, 0, result.Count())
    for _, v := range result {
        phones = append(phones, v.String())
    }
    return phones, nil
}
```

This avoids the overhead of parsing and compiling the expression on every call, which is especially important in request handlers, data pipelines, and anywhere expressions are evaluated at high frequency.
