---
title: "Servicios de Terminología"
linkTitle: "Servicios de Terminología"
weight: 4
description: >
  Conecte las funciones memberOf() y conformsTo() a servidores de terminología externos
  y validadores de perfiles implementando las interfaces TerminologyService y ProfileValidator.
---

## Descripción General

Dos funciones FHIRPath requieren integración con servicios externos para producir resultados
significativos:

- **`memberOf(valueSetUrl)`** -- verifica si un código, Coding o CodeableConcept
  pertenece a un ValueSet dado.
- **`conformsTo(profileUrl)`** -- verifica si un recurso conforma a un
  StructureDefinition (perfil) dado.

Sin un servicio respaldo, ambas funciones devuelven una **colección vacía** (que significa
"desconocido"), lo cual es seguro pero no útil para validación. Esta página muestra cómo
implementar las dos interfaces que alimentan estas funciones.

## La Interfaz TerminologyService

La interfaz `TerminologyService` está definida en el paquete `eval`:

```go
package eval

// TerminologyService handles terminology operations like ValueSet membership.
type TerminologyService interface {
    // MemberOf checks if a code/Coding/CodeableConcept is in the specified ValueSet.
    // Returns true if the code is in the ValueSet, false otherwise.
    // Returns error if the ValueSet cannot be resolved or validation fails.
    MemberOf(ctx context.Context, code interface{}, valueSetURL string) (bool, error)
}
```

El parámetro `code` es un `map[string]interface{}` con las siguientes formas posibles
dependiendo del tipo de entrada:

| Tipo de Entrada   | Claves del Mapa                                 |
|-------------------|-------------------------------------------------|
| Cadena de código simple | `{"code": "active"}`                       |
| Coding            | `{"system": "...", "code": "...", "version": "...", "display": "..."}` |
| CodeableConcept   | `{"coding": [{"system": "...", "code": "..."}], "text": "..."}` |

Su implementación debe inspeccionar el mapa y llamar a la operación de terminología
apropiada (típicamente una operación FHIR `$validate-code` o `$expand`).

## La Interfaz ProfileValidator

La interfaz `ProfileValidator` también está definida en el paquete `eval`:

```go
package eval

// ProfileValidator handles profile conformance validation.
type ProfileValidator interface {
    // ConformsTo checks if a resource conforms to the specified profile.
    // Returns true if the resource conforms, false otherwise.
    ConformsTo(ctx context.Context, resource []byte, profileURL string) (bool, error)
}
```

El parámetro `resource` es el JSON sin procesar del recurso que se está validando. El
`profileURL` es la URL canónica del StructureDefinition contra el cual validar.

## Uso de memberOf()

La función `memberOf()` se llama sobre un elemento de código, Coding o CodeableConcept:

```fhirpath
// Check if a patient's marital status is in a specific ValueSet.
Patient.maritalStatus.coding.memberOf('http://hl7.org/fhir/ValueSet/marital-status')

// Check a simple code value.
Observation.status.memberOf('http://hl7.org/fhir/ValueSet/observation-status')
```

Al evaluarse:

1. La biblioteca extrae la información del código del elemento de entrada.
2. Llama a `TerminologyService.MemberOf()` con los datos del código extraídos y la
   URL del ValueSet.
3. La función devuelve `true` si el código es miembro, `false` si no.
4. Si no hay `TerminologyService` configurado, la función devuelve una colección vacía.

## Uso de conformsTo()

La función `conformsTo()` se llama sobre un recurso:

```fhirpath
// Check if a resource conforms to the US Core Patient profile.
conformsTo('http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient')
```

Al evaluarse:

1. La biblioteca extrae el JSON sin procesar del recurso.
2. Llama a `ProfileValidator.ConformsTo()` con el JSON y la URL del perfil.
3. La función devuelve `true` si el recurso conforma, `false` si no.
4. Si no hay `ProfileValidator` configurado, la función devuelve una colección vacía.

## Ejemplo de Implementación

A continuación se muestra un ejemplo completo que se conecta a un servidor de terminología FHIR para
validación con `memberOf()` e implementa un validador de perfiles simple.

### Implementación del Servicio de Terminología

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"

    "github.com/gofhir/fhirpath/eval"
)

// FHIRTerminologyService validates codes against a FHIR terminology server.
type FHIRTerminologyService struct {
    BaseURL    string
    HTTPClient *http.Client
}

// Ensure interface compliance at compile time.
var _ eval.TerminologyService = (*FHIRTerminologyService)(nil)

func (ts *FHIRTerminologyService) MemberOf(
    ctx context.Context,
    code interface{},
    valueSetURL string,
) (bool, error) {
    codeMap, ok := code.(map[string]interface{})
    if !ok {
        return false, fmt.Errorf("unexpected code type: %T", code)
    }

    // Build the $validate-code request parameters.
    params := url.Values{}
    params.Set("url", valueSetURL)

    if system, ok := codeMap["system"].(string); ok {
        params.Set("system", system)
    }
    if codeVal, ok := codeMap["code"].(string); ok {
        params.Set("code", codeVal)
    }
    if version, ok := codeMap["version"].(string); ok {
        params.Set("version", version)
    }

    reqURL := fmt.Sprintf("%s/ValueSet/$validate-code?%s", ts.BaseURL, params.Encode())
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
    if err != nil {
        return false, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := ts.HTTPClient.Do(req)
    if err != nil {
        return false, fmt.Errorf("terminology request: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return false, fmt.Errorf("read response: %w", err)
    }

    // Parse the Parameters response.
    var result struct {
        Parameter []struct {
            Name         string `json:"name"`
            ValueBoolean *bool  `json:"valueBoolean,omitempty"`
        } `json:"parameter"`
    }
    if err := json.Unmarshal(body, &result); err != nil {
        return false, fmt.Errorf("parse response: %w", err)
    }

    for _, param := range result.Parameter {
        if param.Name == "result" && param.ValueBoolean != nil {
            return *param.ValueBoolean, nil
        }
    }

    return false, fmt.Errorf("no result parameter in response")
}
```

### Implementación del Validador de Perfiles

```go
// SimpleProfileValidator checks resource conformance using a FHIR server's
// $validate operation.
type SimpleProfileValidator struct {
    BaseURL    string
    HTTPClient *http.Client
}

// Ensure interface compliance at compile time.
var _ eval.ProfileValidator = (*SimpleProfileValidator)(nil)

func (pv *SimpleProfileValidator) ConformsTo(
    ctx context.Context,
    resource []byte,
    profileURL string,
) (bool, error) {
    // Determine resource type from the JSON.
    var meta struct {
        ResourceType string `json:"resourceType"`
    }
    if err := json.Unmarshal(resource, &meta); err != nil {
        return false, fmt.Errorf("parse resource: %w", err)
    }

    reqURL := fmt.Sprintf("%s/%s/$validate?profile=%s",
        pv.BaseURL, meta.ResourceType, url.QueryEscape(profileURL))

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
        bytes.NewReader(resource))
    if err != nil {
        return false, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/fhir+json")
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := pv.HTTPClient.Do(req)
    if err != nil {
        return false, fmt.Errorf("validation request: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return false, fmt.Errorf("read response: %w", err)
    }

    // Parse the OperationOutcome.
    var outcome struct {
        Issue []struct {
            Severity string `json:"severity"`
        } `json:"issue"`
    }
    if err := json.Unmarshal(body, &outcome); err != nil {
        return false, fmt.Errorf("parse outcome: %w", err)
    }

    // The resource conforms if there are no error- or fatal-level issues.
    for _, issue := range outcome.Issue {
        if issue.Severity == "error" || issue.Severity == "fatal" {
            return false, nil
        }
    }
    return true, nil
}
```

### Conexión de Todo

Dado que no existen opciones funcionales `WithTerminologyService` o `WithProfileValidator`,
se conectan estos servicios creando un `eval.Context` directamente:

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/gofhir/fhirpath"
    "github.com/gofhir/fhirpath/eval"
)

func main() {
    terminologyService := &FHIRTerminologyService{
        BaseURL:    "http://tx.fhir.org/r4",
        HTTPClient: &http.Client{Timeout: 10 * time.Second},
    }

    profileValidator := &SimpleProfileValidator{
        BaseURL:    "http://hapi.fhir.org/baseR4",
        HTTPClient: &http.Client{Timeout: 10 * time.Second},
    }

    patient := []byte(`{
        "resourceType": "Patient",
        "maritalStatus": {
            "coding": [{
                "system": "http://terminology.hl7.org/CodeSystem/v3-MaritalStatus",
                "code": "M"
            }]
        }
    }`)

    // Compile the expression.
    expr := fhirpath.MustCompile(
        "Patient.maritalStatus.coding.memberOf('http://hl7.org/fhir/ValueSet/marital-status')",
    )

    // Create an eval.Context with the services attached.
    ctx := eval.NewContext(patient)
    ctx.SetTerminologyService(terminologyService)
    ctx.SetProfileValidator(profileValidator)
    ctx.SetContext(context.Background())
    ctx.SetLimit("maxDepth", 100)
    ctx.SetLimit("maxCollectionSize", 10000)

    result, err := expr.EvaluateWithContext(ctx)
    if err != nil {
        fmt.Println("error:", err)
        return
    }
    fmt.Println(result) // [true] if the code is in the ValueSet
}
```

## Comportamiento Cuando los Servicios No Están Configurados

| Escenario                       | memberOf() devuelve   | conformsTo() devuelve  |
|---------------------------------|-----------------------|------------------------|
| Sin servicio configurado        | colección vacía       | colección vacía        |
| El servicio devuelve un error   | colección vacía       | colección vacía        |
| El código es miembro / conforma | `[true]`              | `[true]`               |
| El código no es miembro / no conforma | `[false]`        | `[false]`              |
| La entrada está vacía           | colección vacía       | colección vacía        |

Este comportamiento sigue el tratamiento de la especificación FHIRPath para resultados desconocidos:
cuando el sistema no puede determinar la respuesta, devuelve una colección vacía en lugar
de lanzar un error.

## Resumen

| Interfaz               | Método                                                          | Usada Por      |
|------------------------|-----------------------------------------------------------------|----------------|
| `eval.TerminologyService` | `MemberOf(ctx, code, valueSetURL) (bool, error)`            | `memberOf()`   |
| `eval.ProfileValidator`   | `ConformsTo(ctx, resource, profileURL) (bool, error)`        | `conformsTo()` |

Ambos servicios se adjuntan a un `eval.Context` mediante `SetTerminologyService()` y
`SetProfileValidator()` respectivamente. Cree el contexto, adjunte los servicios y
llame a `expr.EvaluateWithContext(ctx)` para usarlos.
