---
title: "Variables de Entorno"
linkTitle: "Variables de Entorno"
description: "Cómo usar las variables de entorno incorporadas de FHIRPath (%resource, %context, %ucum) y definir variables personalizadas con WithVariable()."
weight: 4
---

Las variables de entorno de FHIRPath son identificadores especiales con el prefijo `%` que proporcionan acceso a información contextual durante la evaluación de expresiones. Son esenciales para escribir restricciones de invariantes FHIR, referenciar el recurso raíz desde rutas anidadas e inyectar datos externos en las expresiones.

## Variables Incorporadas

La biblioteca FHIRPath para Go establece automáticamente las siguientes variables de entorno cuando comienza una evaluación.

### %resource

`%resource` se refiere al **recurso raíz** que se está evaluando. Se establece automáticamente al recurso de nivel superior pasado a `Evaluate`, `EvaluateCached` o `Expression.Evaluate`.

Esta variable es requerida por muchas restricciones de StructureDefinition de FHIR (invariantes) que necesitan referenciar el recurso raíz desde un contexto anidado. Por ejemplo, el invariante FHIR de Bundle `bdl-3` utiliza `%resource` para referenciar el Bundle desde dentro de una entrada:

```text
// FHIR invariant bdl-3: fullUrl must be unique within a Bundle
%resource.entry.where(fullUrl.exists()).select(fullUrl).isDistinct()
```

En un programa Go:

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

`%context` representa el **nodo original** pasado al motor de evaluación. Para la evaluación de nivel superior (el caso más común), `%context` es lo mismo que `%resource`. La distinción es relevante en escenarios avanzados donde una expresión se evalúa contra un sub-nodo de un recurso.

```text
// For top-level evaluation, these are equivalent:
%resource.id
%context.id
```

Tanto `%resource` como `%context` se establecen automáticamente por el constructor `eval.NewContext` y no requieren configuración manual.

### %ucum

`%ucum` es una constante estándar de FHIRPath que se resuelve a la cadena `'http://unitsofmeasure.org'`. Se utiliza en expresiones que verifican el sistema de codificación de la unidad de una Quantity:

```text
Observation.value.ofType(Quantity).system = %ucum
```

Esto es una abreviatura de:

```text
Observation.value.ofType(Quantity).system = 'http://unitsofmeasure.org'
```

## Variables Personalizadas con WithVariable()

Se pueden inyectar variables de entorno propias en una evaluación utilizando la opción funcional `WithVariable`. Las variables personalizadas se acceden en expresiones FHIRPath mediante la sintaxis `%nombre`, igual que las variables incorporadas.

### Uso Básico

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

### Múltiples Variables

Se pueden pasar múltiples opciones `WithVariable` para establecer varias variables a la vez:

```go
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithVariable("minAge", types.Collection{types.NewInteger(18)}),
    fhirpath.WithVariable("maxAge", types.Collection{types.NewInteger(65)}),
    fhirpath.WithVariable("status", types.Collection{types.NewString("active")}),
)
```

### Tipos de Variables

Los valores de las variables son instancias de `types.Collection`, por lo que se puede pasar cualquier tipo de valor FHIRPath:

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

### Casos de Uso

Las variables personalizadas son particularmente útiles para:

1. **Reglas de validación parametrizadas** -- pasar umbrales, valores esperados o configuración como variables en lugar de codificarlos directamente en las expresiones.

```go
// Validate that a patient's age exceeds a configurable minimum
expr := fhirpath.MustCompile(
    "Patient.birthDate <= today() - %minAge 'years'",
)
result, _ := expr.EvaluateWithOptions(patientJSON,
    fhirpath.WithVariable("minAge", types.Collection{types.NewInteger(18)}),
)
```

2. **Referencias entre recursos** -- pasar datos de un recurso como variable al evaluar otro.

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

3. **Evaluación dinámica de expresiones** -- cuando las expresiones se cargan desde configuración o entrada de usuario y necesitan parámetros en tiempo de ejecución.

## Combinación con Otras Opciones

`WithVariable` se puede combinar con otras opciones de evaluación como `WithTimeout`, `WithMaxDepth` y `WithContext`:

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

Consulte la guía de [Primeros Pasos]({{< relref "../getting-started" >}}) para más información sobre las opciones de evaluación.
