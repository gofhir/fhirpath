---
title: "Resource Interface"
linkTitle: "Resource"
weight: 4
description: >
  Evaluate FHIRPath expressions against Go structs implementing the Resource interface.
---

The resource API lets you evaluate FHIRPath expressions directly against Go structs, without manually serializing them to JSON first. The library handles JSON marshaling internally. For repeated evaluations against the same resource, `ResourceJSON` pre-serializes the struct once for optimal performance.

## Resource Interface

Any Go struct that implements the `Resource` interface can be used with `EvaluateResource`, `EvaluateResourceCached`, and `ResourceJSON`.

```go
type Resource interface {
    GetResourceType() string
}
```

The `GetResourceType()` method should return the FHIR resource type name (e.g., `"Patient"`, `"Observation"`, `"Bundle"`). This is typically the same value as the `resourceType` field in the JSON representation.

**Implementing the interface:**

```go
type Patient struct {
    ResourceType string `json:"resourceType"`
    ID           string `json:"id"`
    Active       bool   `json:"active"`
    Name         []HumanName `json:"name,omitempty"`
    BirthDate    string `json:"birthDate,omitempty"`
}

func (p *Patient) GetResourceType() string {
    return "Patient"
}

type HumanName struct {
    Family string   `json:"family,omitempty"`
    Given  []string `json:"given,omitempty"`
    Use    string   `json:"use,omitempty"`
}
```

{{% alert title="Tip" color="info" %}}
If you use a FHIR model library for Go (such as those generated from FHIR StructureDefinitions), you only need to add or verify the `GetResourceType()` method on each resource type.
{{% /alert %}}

---

## EvaluateResource

Evaluates a FHIRPath expression against a Go struct that implements `Resource`. The resource is marshaled to JSON with `json.Marshal`, then evaluated.

```go
func EvaluateResource(resource Resource, expr string) (Collection, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `Resource` | A Go struct implementing the `Resource` interface |
| `expr` | `string` | A FHIRPath expression to evaluate |

**Returns:**

| Type | Description |
|------|-------------|
| `Collection` | The evaluation result |
| `error` | Non-nil on marshaling, compilation, or evaluation errors |

**Example:**

```go
patient := &Patient{
    ResourceType: "Patient",
    ID:           "123",
    Active:       true,
    Name: []HumanName{
        {Family: "Smith", Given: []string{"John"}, Use: "official"},
    },
    BirthDate: "1990-05-15",
}

result, err := fhirpath.EvaluateResource(patient, "Patient.name.family")
if err != nil {
    log.Fatal(err)
}
fmt.Println(result) // [Smith]
```

{{% alert title="Performance Note" color="warning" %}}
`EvaluateResource` calls `json.Marshal` on every invocation. If you evaluate multiple expressions against the same resource, use `ResourceJSON` to marshal once and evaluate many times.
{{% /alert %}}

---

## EvaluateResourceCached

Like `EvaluateResource`, but uses the `DefaultCache` for expression compilation. The resource is still marshaled to JSON on every call.

```go
func EvaluateResourceCached(resource Resource, expr string) (Collection, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `Resource` | A Go struct implementing the `Resource` interface |
| `expr` | `string` | A FHIRPath expression to evaluate |

**Returns:**

| Type | Description |
|------|-------------|
| `Collection` | The evaluation result |
| `error` | Non-nil on marshaling, compilation, or evaluation errors |

**Example:**

```go
// Process a batch of patients with the same expression
for _, patient := range patients {
    result, err := fhirpath.EvaluateResourceCached(patient, "Patient.active")
    if err != nil {
        log.Printf("error for patient %s: %v", patient.ID, err)
        continue
    }
    fmt.Printf("Patient %s active: %s\n", patient.ID, result)
}
```

---

## ResourceJSON

`ResourceJSON` wraps a `Resource` with its pre-serialized JSON bytes. Create one instance and evaluate multiple expressions without repeated marshaling.

```go
type ResourceJSON struct {
    // unexported fields
}
```

### NewResourceJSON

Creates a `ResourceJSON` by marshaling the given resource to JSON.

```go
func NewResourceJSON(resource Resource) (*ResourceJSON, error)
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `resource` | `Resource` | A Go struct implementing the `Resource` interface |

**Returns:**

| Type | Description |
|------|-------------|
| `*ResourceJSON` | The wrapped resource with pre-serialized JSON |
| `error` | Non-nil if JSON marshaling fails |

### MustNewResourceJSON

Like `NewResourceJSON`, but panics on error.

```go
func MustNewResourceJSON(resource Resource) *ResourceJSON
```

**Panics** if JSON marshaling fails.

### ResourceJSON.Evaluate

Evaluates a FHIRPath expression against the pre-serialized JSON.

```go
func (r *ResourceJSON) Evaluate(expr string) (Collection, error)
```

### ResourceJSON.EvaluateCached

Evaluates a FHIRPath expression using the `DefaultCache`.

```go
func (r *ResourceJSON) EvaluateCached(expr string) (Collection, error)
```

### ResourceJSON.JSON

Returns the pre-serialized JSON bytes.

```go
func (r *ResourceJSON) JSON() []byte
```

### ResourceJSON.Resource

Returns the original Go resource struct.

```go
func (r *ResourceJSON) Resource() Resource
```

**Complete Example:**

```go
patient := &Patient{
    ResourceType: "Patient",
    ID:           "example-1",
    Active:       true,
    Name: []HumanName{
        {Family: "Garcia", Given: []string{"Maria", "Elena"}, Use: "official"},
        {Family: "Garcia", Given: []string{"Mari"}, Use: "nickname"},
    },
    BirthDate: "1988-11-03",
}

// Marshal once
rj, err := fhirpath.NewResourceJSON(patient)
if err != nil {
    log.Fatal(err)
}

// Evaluate many expressions against the same serialized resource
family, err := rj.EvaluateCached("Patient.name.where(use = 'official').family")
if err != nil {
    log.Fatal(err)
}
fmt.Println(family) // [Garcia]

givenNames, err := rj.EvaluateCached("Patient.name.where(use = 'official').given")
if err != nil {
    log.Fatal(err)
}
fmt.Println(givenNames) // [Maria, Elena]

birthDate, err := rj.EvaluateCached("Patient.birthDate")
if err != nil {
    log.Fatal(err)
}
fmt.Println(birthDate) // [1988-11-03]

// Access the underlying data if needed
fmt.Println(string(rj.JSON()))           // Full JSON output
fmt.Println(rj.Resource().GetResourceType()) // Patient
```

---

## When to Use Which

| Scenario | Recommended API |
|----------|----------------|
| Single expression, single resource | `EvaluateResource` |
| Same expression, many resources | `EvaluateResourceCached` |
| Many expressions, same resource | `ResourceJSON` + `EvaluateCached` |
| Many expressions, many resources | `ResourceJSON` per resource + `EvaluateCached` |
| Already have JSON bytes | Use `Evaluate` / `EvaluateCached` directly |

## Using with FHIR Model Libraries

If you already have Go structs from a FHIR model library, you just need to ensure they implement `GetResourceType()`:

```go
// Adapter for an external FHIR model that has ResourceType as a field
type FHIRPatientAdapter struct {
    *externalfhir.Patient
}

func (a *FHIRPatientAdapter) GetResourceType() string {
    return "Patient"
}

// Use with the fhirpath library
adapter := &FHIRPatientAdapter{Patient: externalPatient}
result, err := fhirpath.EvaluateResource(adapter, "Patient.name.family")
```
