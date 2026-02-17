---
title: "Opciones de Evaluación"
linkTitle: "Opciones de Evaluación"
weight: 2
description: >
  Controle tiempos de espera, profundidad de recursión, límites de tamaño de colección y variables
  personalizadas a través de la API de opciones funcionales.
---

## Descripción General de EvalOptions

Cuando llama a `Evaluate()` o `EvaluateCached()`, la biblioteca utiliza valores predeterminados sensatos
para cada límite de seguridad. Para un control detallado, compile la expresión primero y
luego llame a `EvaluateWithOptions()` con una o más opciones funcionales.

```go
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithTimeout(2 * time.Second),
    fhirpath.WithMaxDepth(50),
)
```

La estructura subyacente `EvalOptions` contiene estos campos:

| Campo              | Tipo                        | Valor por Defecto       | Descripción                                          |
|--------------------|-----------------------------|-----------------------|------------------------------------------------------|
| `Ctx`              | `context.Context`           | `context.Background()`| Contexto para cancelación y propagación de plazos     |
| `Timeout`          | `time.Duration`             | 5 s                   | Tiempo máximo de reloj de pared para una evaluación   |
| `MaxDepth`         | `int`                       | 100                   | Límite de recursión para `descendants()` y rutas anidadas |
| `MaxCollectionSize`| `int`                       | 10 000                | Número máximo de elementos en cualquier resultado intermedio |
| `Variables`        | `map[string]types.Collection`| mapa vacío            | Variables externas accesibles via `%name`             |
| `Resolver`         | `ReferenceResolver`         | nil                   | Manejador para la función `resolve()`                 |

Todas las opciones se aplican sobre los valores predeterminados devueltos por `DefaultOptions()`, por lo que
solo necesita especificar los valores que desea sobrescribir.

## Protección por Tiempo de Espera

La opción `WithTimeout` envuelve la evaluación en un `context.WithTimeout`. Si la
expresión tarda más que la duración especificada, la evaluación se cancela y
devuelve un error.

Esto es esencial al evaluar **expresiones proporcionadas por el usuario**, porque una
expresión patológica como `Patient.descendants().descendants()` podría de otro modo
ejecutarse durante mucho tiempo.

```go
package main

import (
    "fmt"
    "time"

    "github.com/gofhir/fhirpath"
)

func main() {
    patient := []byte(`{
        "resourceType": "Patient",
        "name": [{"family": "Doe", "given": ["John", "James"]}]
    }`)

    expr := fhirpath.MustCompile("Patient.name.given")

    // Allow at most 500 ms for this evaluation.
    result, err := expr.EvaluateWithOptions(patient,
        fhirpath.WithTimeout(500 * time.Millisecond),
    )
    if err != nil {
        fmt.Println("evaluation timed out:", err)
        return
    }
    fmt.Println(result) // [John, James]
}
```

### Uso de un Contexto Existente

Si su aplicación ya lleva un contexto con alcance de solicitud (por ejemplo, desde un
manejador HTTP), páselo con `WithContext` para que la evaluación respete la
señal de cancelación del llamador:

```go
func handleRequest(ctx context.Context, resource []byte) (fhirpath.Collection, error) {
    expr := fhirpath.MustGetCached("Patient.name.family")
    return expr.EvaluateWithOptions(resource,
        fhirpath.WithContext(ctx),
        fhirpath.WithTimeout(2 * time.Second),
    )
}
```

Cuando se especifican tanto `WithContext` como `WithTimeout`, el tiempo de espera se aplica como
hijo del contexto proporcionado. Si el contexto padre se cancela primero, la
evaluación se detiene inmediatamente.

## Límites de Recursión

La opción `WithMaxDepth` limita cuán profundamente recursará el evaluador al
recorrer estructuras anidadas. Esto protege contra desbordamientos de pila causados por
recursos profundamente anidados o expresiones que usan `descendants()`.

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func main() {
    // A deeply nested Questionnaire with items inside items.
    questionnaire := []byte(`{
        "resourceType": "Questionnaire",
        "item": [{
            "linkId": "1",
            "item": [{
                "linkId": "1.1",
                "item": [{
                    "linkId": "1.1.1"
                }]
            }]
        }]
    }`)

    expr := fhirpath.MustCompile("Questionnaire.descendants().ofType(Questionnaire.item)")

    // Restrict recursion to 50 levels instead of the default 100.
    result, err := expr.EvaluateWithOptions(questionnaire,
        fhirpath.WithMaxDepth(50),
    )
    if err != nil {
        fmt.Println("depth exceeded:", err)
        return
    }
    fmt.Println("items found:", len(result))
}
```

Establezca `MaxDepth` en `0` para usar el valor predeterminado de 100.

## Límites de Tamaño de Colección

La opción `WithMaxCollectionSize` limita el número de elementos en cualquier colección
intermedia. Si una expresión produce más elementos que el límite, la evaluación
devuelve un error en lugar de consumir memoria sin límite.

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func main() {
    // A Bundle with many entries.
    bundle := []byte(`{
        "resourceType": "Bundle",
        "entry": [
            {"resource": {"resourceType": "Patient", "id": "1"}},
            {"resource": {"resourceType": "Patient", "id": "2"}},
            {"resource": {"resourceType": "Patient", "id": "3"}}
        ]
    }`)

    expr := fhirpath.MustCompile("Bundle.entry.resource")

    // Limit intermediate collections to 500 elements.
    result, err := expr.EvaluateWithOptions(bundle,
        fhirpath.WithMaxCollectionSize(500),
    )
    if err != nil {
        fmt.Println("collection too large:", err)
        return
    }
    fmt.Println("resources:", len(result))
}
```

El límite predeterminado es 10 000, lo cual es generoso para la mayoría de cargas de trabajo. Redúzcalo cuando
evalúe expresiones no confiables para prevenir denegación de servicio por agotamiento
de memoria.

## Variables Personalizadas

FHIRPath soporta variables externas referenciadas con el prefijo `%`. La biblioteca
establece automáticamente `%resource` y `%context` al recurso raíz, pero puede
inyectar sus propias variables con `WithVariable`.

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
    "github.com/gofhir/fhirpath/types"
)

func main() {
    patient := []byte(`{
        "resourceType": "Patient",
        "identifier": [
            {"system": "http://hospital.example.org/mrn", "value": "MRN-12345"},
            {"system": "http://hl7.org/fhir/sid/us-ssn",  "value": "123-45-6789"}
        ]
    }`)

    // Find identifiers matching a system provided at runtime.
    expr := fhirpath.MustCompile("Patient.identifier.where(system = %targetSystem).value")

    targetSystem := types.Collection{types.NewString("http://hl7.org/fhir/sid/us-ssn")}

    result, err := expr.EvaluateWithOptions(patient,
        fhirpath.WithVariable("targetSystem", targetSystem),
    )
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // [123-45-6789]
}
```

### Múltiples Variables

Puede pasar tantas opciones `WithVariable` como necesite:

```go
result, err := expr.EvaluateWithOptions(patient,
    fhirpath.WithVariable("minAge", types.Collection{types.NewInteger(18)}),
    fhirpath.WithVariable("system", types.Collection{types.NewString("http://loinc.org")}),
    fhirpath.WithVariable("today", types.Collection{todayDate}),
)
```

### Variables Incorporadas

La biblioteca proporciona automáticamente estas variables para cada evaluación:

| Variable      | Valor                                             |
|---------------|---------------------------------------------------|
| `%resource`   | El recurso raíz que se está evaluando             |
| `%context`    | Igual que `%resource` para evaluación de nivel superior |

Estas son requeridas por las expresiones de restricción FHIR (como `bdl-3` y `bdl-4`)
y no deben sobrescribirse a menos que tenga una razón específica para hacerlo.

## Combinación de Opciones

En código de producción frecuentemente combinará varias opciones. El patrón de opciones
funcionales hace esto limpio y legible:

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/gofhir/fhirpath"
    "github.com/gofhir/fhirpath/types"
)

func evaluateExpression(
    ctx context.Context,
    resource []byte,
    expression string,
    targetSystem string,
) (fhirpath.Collection, error) {
    expr, err := fhirpath.GetCached(expression)
    if err != nil {
        return nil, fmt.Errorf("compile: %w", err)
    }

    return expr.EvaluateWithOptions(resource,
        // Propagate the caller's context for cancellation.
        fhirpath.WithContext(ctx),

        // Hard timeout for this single evaluation.
        fhirpath.WithTimeout(2 * time.Second),

        // Safety limits.
        fhirpath.WithMaxDepth(50),
        fhirpath.WithMaxCollectionSize(5000),

        // Runtime variable.
        fhirpath.WithVariable("targetSystem",
            types.Collection{types.NewString(targetSystem)},
        ),
    )
}

func main() {
    patient := []byte(`{
        "resourceType": "Patient",
        "identifier": [
            {"system": "http://hospital.example.org/mrn", "value": "MRN-001"}
        ]
    }`)

    result, err := evaluateExpression(
        context.Background(),
        patient,
        "Patient.identifier.where(system = %targetSystem).value",
        "http://hospital.example.org/mrn",
    )
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // [MRN-001]
}
```

## Referencia Rápida

| Función                         | Descripción                                               |
|---------------------------------|-----------------------------------------------------------|
| `WithContext(ctx)`              | Establecer el `context.Context` padre                     |
| `WithTimeout(d)`               | Establecer el tiempo de espera de evaluación              |
| `WithMaxDepth(n)`              | Establecer la profundidad máxima de recursión             |
| `WithMaxCollectionSize(n)`     | Establecer el tamaño máximo de colección intermedia       |
| `WithVariable(name, value)`    | Inyectar una variable externa accesible via `%name`       |
| `WithResolver(r)`              | Establecer un `ReferenceResolver` (ver [Resolvedores Personalizados](../custom-resolvers/)) |
| `DefaultOptions()`             | Devuelve un nuevo `EvalOptions` con todos los valores por defecto aplicados |
