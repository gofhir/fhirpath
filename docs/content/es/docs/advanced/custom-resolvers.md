---
title: "Resolvedores de Referencias Personalizados"
linkTitle: "Resolvedores de Referencias Personalizados"
weight: 3
description: >
  Implemente la interfaz ReferenceResolver para permitir que la función resolve() obtenga
  recursos FHIR referenciados desde endpoints HTTP, bundles en memoria o cualquier fuente de datos.
---

## La Interfaz ReferenceResolver

Los recursos FHIR frecuentemente referencian otros recursos. La función FHIRPath `resolve()`
sigue esas referencias y devuelve el recurso referenciado como parte del resultado de la evaluación.
Para que `resolve()` funcione, necesita proporcionar un `ReferenceResolver` que sepa
cómo obtener recursos dada una cadena de referencia.

La interfaz es intencionalmente mínima:

```go
// ReferenceResolver resolves FHIR references for the resolve() function.
type ReferenceResolver interface {
    // Resolve takes a reference string (e.g., "Patient/123") and returns
    // the resource as raw JSON bytes.
    Resolve(ctx context.Context, reference string) ([]byte, error)
}
```

Puntos clave:

- El parámetro `reference` es la cadena sin procesar extraída de un campo FHIR `Reference.reference`.
  Puede ser una referencia relativa (`"Patient/123"`), una URL absoluta
  (`"http://example.org/fhir/Patient/123"`), o un fragmento (`"#contained-1"`).
- El resolver debe devolver el recurso como **bytes JSON** (`[]byte`).
- El parámetro `ctx` lleva el tiempo de espera de evaluación y la señal de cancelación.
  Respételo en cualquier operación de E/S.
- Si la referencia no puede resolverse, devuelva un error. La función `resolve()`
  omitirá silenciosamente las referencias irresolubles y continuará con el siguiente elemento.

## Resolver HTTP Simple

El caso de uso más común es resolver referencias contra un servidor FHIR remoto:

```go
package main

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"

    "github.com/gofhir/fhirpath"
)

// HTTPResolver resolves FHIR references by making HTTP GET requests.
type HTTPResolver struct {
    BaseURL    string       // e.g., "http://hapi.fhir.org/baseR4"
    HTTPClient *http.Client
}

func (r *HTTPResolver) Resolve(ctx context.Context, reference string) ([]byte, error) {
    // Build the full URL.
    var url string
    if strings.HasPrefix(reference, "http://") || strings.HasPrefix(reference, "https://") {
        url = reference
    } else {
        url = strings.TrimRight(r.BaseURL, "/") + "/" + reference
    }

    // Create request with context for timeout propagation.
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := r.HTTPClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("HTTP GET %s: %w", url, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("HTTP GET %s returned %d", url, resp.StatusCode)
    }

    return io.ReadAll(resp.Body)
}

func main() {
    resolver := &HTTPResolver{
        BaseURL:    "http://hapi.fhir.org/baseR4",
        HTTPClient: &http.Client{Timeout: 10 * time.Second},
    }

    // An Observation that references a Patient.
    observation := []byte(`{
        "resourceType": "Observation",
        "subject": {
            "reference": "Patient/example"
        },
        "code": {
            "coding": [{"system": "http://loinc.org", "code": "29463-7"}]
        }
    }`)

    expr := fhirpath.MustCompile("Observation.subject.resolve().name.family")

    result, err := expr.EvaluateWithOptions(observation,
        fhirpath.WithResolver(resolver),
        fhirpath.WithTimeout(5 * time.Second),
    )
    if err != nil {
        fmt.Println("evaluation error:", err)
        return
    }
    fmt.Println(result) // The patient's family name, if the reference resolves.
}
```

## Resolver de Bundle en Memoria

Cuando trabaja con Bundles FHIR, las referencias frecuentemente son internas al bundle. Un
resolver en memoria evita cualquier llamada de red:

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/gofhir/fhirpath"
)

// BundleResolver resolves references within a pre-parsed FHIR Bundle.
type BundleResolver struct {
    // resources maps "ResourceType/id" to raw JSON bytes.
    resources map[string][]byte
}

// NewBundleResolver builds an index from a raw FHIR Bundle.
func NewBundleResolver(bundleJSON []byte) (*BundleResolver, error) {
    var bundle struct {
        Entry []struct {
            FullURL  string          `json:"fullUrl"`
            Resource json.RawMessage `json:"resource"`
        } `json:"entry"`
    }
    if err := json.Unmarshal(bundleJSON, &bundle); err != nil {
        return nil, fmt.Errorf("unmarshal bundle: %w", err)
    }

    resources := make(map[string][]byte, len(bundle.Entry))
    for _, entry := range bundle.Entry {
        // Index by fullUrl.
        if entry.FullURL != "" {
            resources[entry.FullURL] = entry.Resource
        }

        // Also index by "ResourceType/id" for relative references.
        var meta struct {
            ResourceType string `json:"resourceType"`
            ID           string `json:"id"`
        }
        if err := json.Unmarshal(entry.Resource, &meta); err == nil && meta.ID != "" {
            key := meta.ResourceType + "/" + meta.ID
            resources[key] = entry.Resource
        }
    }

    return &BundleResolver{resources: resources}, nil
}

func (r *BundleResolver) Resolve(_ context.Context, reference string) ([]byte, error) {
    // Try exact match first (handles both fullUrl and relative references).
    if data, ok := r.resources[reference]; ok {
        return data, nil
    }

    // Try matching the tail of fullUrl entries.
    for key, data := range r.resources {
        if strings.HasSuffix(key, "/"+reference) {
            return data, nil
        }
    }

    return nil, fmt.Errorf("reference not found in bundle: %s", reference)
}

func main() {
    bundle := []byte(`{
        "resourceType": "Bundle",
        "type": "transaction",
        "entry": [
            {
                "fullUrl": "urn:uuid:patient-1",
                "resource": {
                    "resourceType": "Patient",
                    "id": "patient-1",
                    "name": [{"family": "Smith", "given": ["Jane"]}]
                }
            },
            {
                "fullUrl": "urn:uuid:obs-1",
                "resource": {
                    "resourceType": "Observation",
                    "id": "obs-1",
                    "subject": {"reference": "Patient/patient-1"},
                    "code": {
                        "coding": [{"system": "http://loinc.org", "code": "29463-7"}]
                    }
                }
            }
        ]
    }`)

    resolver, err := NewBundleResolver(bundle)
    if err != nil {
        panic(err)
    }

    // Evaluate on a single entry's resource.
    observation := []byte(`{
        "resourceType": "Observation",
        "subject": {"reference": "Patient/patient-1"},
        "code": {
            "coding": [{"system": "http://loinc.org", "code": "29463-7"}]
        }
    }`)

    expr := fhirpath.MustCompile("Observation.subject.resolve().name.family")

    result, err := expr.EvaluateWithOptions(observation,
        fhirpath.WithResolver(resolver),
    )
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // [Smith]
}
```

## Manejo de Errores

La función `resolve()` maneja los errores del resolver de forma elegante:

1. Si no hay resolver configurado, `resolve()` devuelve una **colección vacía**.
2. Si el resolver devuelve un error para una referencia específica, esa referencia es
   **omitida silenciosamente** y se intenta con el siguiente elemento de la colección.
3. Si el JSON devuelto no puede analizarse, el elemento se omite.

Este diseño sigue la especificación FHIRPath, que establece que `resolve()`
no debe hacer fallar la expresión completa cuando una referencia no puede seguirse.

```go
// A resolver that rejects certain references.
type SelectiveResolver struct {
    inner fhirpath.ReferenceResolver
}

func (r *SelectiveResolver) Resolve(ctx context.Context, ref string) ([]byte, error) {
    // Only resolve Patient references.
    if !strings.HasPrefix(ref, "Patient/") {
        return nil, fmt.Errorf("unsupported reference type: %s", ref)
    }
    return r.inner.Resolve(ctx, ref)
}
```

En el ejemplo anterior, las referencias que no son de Patient serán silenciosamente excluidas del
resultado. La expresión continúa evaluándose sin error.

### Registro de Fallos de Resolución

Si desea visibilidad sobre los fallos de resolución, agregue registro dentro de su resolver:

```go
func (r *HTTPResolver) Resolve(ctx context.Context, reference string) ([]byte, error) {
    data, err := r.doResolve(ctx, reference)
    if err != nil {
        log.Printf("WARN: failed to resolve reference %q: %v", reference, err)
        return nil, err
    }
    return data, nil
}
```

## Conexión

Hay dos formas de adjuntar un resolver a una evaluación:

### Opción 1: Opción Funcional (Recomendada)

Use `WithResolver` al llamar a `EvaluateWithOptions`:

```go
expr := fhirpath.MustCompile("Observation.subject.resolve().name.family")

result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithResolver(myResolver),
    fhirpath.WithTimeout(3 * time.Second),
)
```

Este es el enfoque recomendado porque mantiene el resolver con alcance a una sola
evaluación y se compone limpiamente con otras opciones.

### Opción 2: Configuración Directa del Contexto

Para mayor control, cree un `eval.Context` manualmente y establezca el resolver directamente:

```go
import "github.com/gofhir/fhirpath/eval"

ctx := eval.NewContext(resource)
ctx.SetResolver(myResolverAdapter)
ctx.SetContext(requestCtx)
ctx.SetLimit("maxDepth", 100)
ctx.SetLimit("maxCollectionSize", 10000)

result, err := expr.EvaluateWithContext(ctx)
```

Tenga en cuenta que al usar `eval.Context` directamente, debe usar un adaptador que
implemente la interfaz `eval.Resolver` (que tiene la misma firma que
`fhirpath.ReferenceResolver`). La opción `WithResolver` maneja esta adaptación
automáticamente.

## Resumen

| Concepto                 | Descripción                                                   |
|--------------------------|---------------------------------------------------------------|
| `ReferenceResolver`      | Interfaz con un único método `Resolve(ctx, reference) ([]byte, error)` |
| `WithResolver(r)`        | Opción funcional para adjuntar un resolver a una evaluación   |
| Resolver HTTP            | Resuelve referencias obteniendo datos de una API REST FHIR   |
| Resolver de Bundle       | Resuelve referencias dentro de un Bundle FHIR pre-indexado    |
| Comportamiento de error  | Las referencias irresolubles se omiten silenciosamente        |
| Sin resolver configurado | `resolve()` devuelve una colección vacía                      |
