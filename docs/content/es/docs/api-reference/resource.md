---
title: "Interfaz Resource"
linkTitle: "Resource"
weight: 4
description: >
  Evaluar expresiones FHIRPath contra structs de Go que implementan la interfaz Resource.
---

La API de recursos permite evaluar expresiones FHIRPath directamente contra structs de Go, sin necesidad de serializarlos manualmente a JSON primero. La biblioteca maneja el marshaling JSON internamente. Para evaluaciones repetidas contra el mismo recurso, `ResourceJSON` pre-serializa el struct una vez para un rendimiento óptimo.

## Interfaz Resource

Cualquier struct de Go que implemente la interfaz `Resource` puede ser utilizado con `EvaluateResource`, `EvaluateResourceCached` y `ResourceJSON`.

```go
type Resource interface {
    GetResourceType() string
}
```

El método `GetResourceType()` debe retornar el nombre del tipo de recurso FHIR® (por ejemplo, `"Patient"`, `"Observation"`, `"Bundle"`). Típicamente es el mismo valor que el campo `resourceType` en la representación JSON.

**Implementación de la interfaz:**

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

{{< callout type="info" >}}
**Consejo:** Si utiliza una biblioteca de modelos FHIR® para Go (como las generadas a partir de FHIR® StructureDefinitions), solo necesita agregar o verificar el método `GetResourceType()` en cada tipo de recurso.
{{< /callout >}}

---

## EvaluateResource

Evalúa una expresión FHIRPath contra un struct de Go que implementa `Resource`. El recurso se serializa a JSON con `json.Marshal` y luego se evalúa.

```go
func EvaluateResource(resource Resource, expr string) (Collection, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `resource` | `Resource` | Un struct de Go que implementa la interfaz `Resource` |
| `expr` | `string` | Una expresión FHIRPath a evaluar |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `Collection` | El resultado de la evaluación |
| `error` | No nulo en errores de marshaling, compilación o evaluación |

**Ejemplo:**

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

{{< callout type="warning" >}}
**Nota de Rendimiento:** `EvaluateResource` llama a `json.Marshal` en cada invocación. Si evalúa múltiples expresiones contra el mismo recurso, utilice `ResourceJSON` para serializar una vez y evaluar muchas veces.
{{< /callout >}}

---

## EvaluateResourceCached

Similar a `EvaluateResource`, pero utiliza el `DefaultCache` para la compilación de expresiones. El recurso aún se serializa a JSON en cada llamada.

```go
func EvaluateResourceCached(resource Resource, expr string) (Collection, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `resource` | `Resource` | Un struct de Go que implementa la interfaz `Resource` |
| `expr` | `string` | Una expresión FHIRPath a evaluar |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `Collection` | El resultado de la evaluación |
| `error` | No nulo en errores de marshaling, compilación o evaluación |

**Ejemplo:**

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

`ResourceJSON` envuelve un `Resource` con sus bytes JSON pre-serializados. Cree una instancia y evalúe múltiples expresiones sin marshaling repetido.

```go
type ResourceJSON struct {
    // unexported fields
}
```

### NewResourceJSON

Crea un `ResourceJSON` serializando el recurso dado a JSON.

```go
func NewResourceJSON(resource Resource) (*ResourceJSON, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `resource` | `Resource` | Un struct de Go que implementa la interfaz `Resource` |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `*ResourceJSON` | El recurso envuelto con JSON pre-serializado |
| `error` | No nulo si el marshaling JSON falla |

### MustNewResourceJSON

Similar a `NewResourceJSON`, pero genera un panic en caso de error.

```go
func MustNewResourceJSON(resource Resource) *ResourceJSON
```

**Genera panic** si el marshaling JSON falla.

### ResourceJSON.Evaluate

Evalúa una expresión FHIRPath contra el JSON pre-serializado.

```go
func (r *ResourceJSON) Evaluate(expr string) (Collection, error)
```

### ResourceJSON.EvaluateCached

Evalúa una expresión FHIRPath utilizando el `DefaultCache`.

```go
func (r *ResourceJSON) EvaluateCached(expr string) (Collection, error)
```

### ResourceJSON.JSON

Retorna los bytes JSON pre-serializados.

```go
func (r *ResourceJSON) JSON() []byte
```

### ResourceJSON.Resource

Retorna el struct del recurso Go original.

```go
func (r *ResourceJSON) Resource() Resource
```

**Ejemplo Completo:**

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

## Cuándo Usar Cuál

| Escenario | API Recomendada |
|-----------|-----------------|
| Una sola expresión, un solo recurso | `EvaluateResource` |
| Misma expresión, muchos recursos | `EvaluateResourceCached` |
| Muchas expresiones, mismo recurso | `ResourceJSON` + `EvaluateCached` |
| Muchas expresiones, muchos recursos | `ResourceJSON` por recurso + `EvaluateCached` |
| Ya se tienen bytes JSON | Utilice `Evaluate` / `EvaluateCached` directamente |

## Uso con Bibliotecas de Modelos FHIR®

Si ya se tienen structs de Go de una biblioteca de modelos FHIR®, solo es necesario asegurarse de que implementen `GetResourceType()`:

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
