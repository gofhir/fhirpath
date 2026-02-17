---
title: "Guía de Rendimiento"
linkTitle: "Guía de Rendimiento"
weight: 5
description: >
  Patrones prácticos para evaluación FHIRPath de alto rendimiento: compilar una vez, caché de
  expresiones, pre-serialización de recursos, filtrado temprano y consejos de conversión de tipos.
---

## Patrón Compilar Una Vez

La optimización individual más impactante es **compilar cada expresión una vez** y
reutilizar el objeto `*Expression` resultante. El análisis es 10-50 veces más costoso que
la evaluación.

### Malo: Compilar en Cada Llamada

```go
// BAD -- parses the expression on every iteration.
for _, resource := range resources {
    result, err := fhirpath.Evaluate(resource, "Patient.name.family")
    if err != nil {
        log.Fatal(err)
    }
    process(result)
}
```

`fhirpath.Evaluate()` llama a `Compile()` internamente cada vez. Para un bucle sobre
10 000 recursos, paga el costo de análisis 10 000 veces.

### Bueno: Compilar Una Vez, Evaluar Muchas

```go
// GOOD -- compile once, evaluate many times.
expr := fhirpath.MustCompile("Patient.name.family")

for _, resource := range resources {
    result, err := expr.Evaluate(resource)
    if err != nil {
        log.Fatal(err)
    }
    process(result)
}
```

El `*Expression` compilado es inmutable y seguro para compartir entre goroutines (ver
[Seguridad en Hilos](../thread-safety/)).

### Mejor: Variables a Nivel de Paquete

Para expresiones conocidas en tiempo de desarrollo, compílelas una vez durante la
inicialización del paquete:

```go
package myvalidator

import "github.com/gofhir/fhirpath"

var (
    exprFamilyName = fhirpath.MustCompile("Patient.name.family")
    exprBirthDate  = fhirpath.MustCompile("Patient.birthDate")
    exprMRN        = fhirpath.MustCompile(
        "Patient.identifier.where(system = 'http://hospital.example.org/mrn').value",
    )
)

func GetFamilyName(patient []byte) (string, error) {
    return fhirpath.EvaluateToString(patient, "Patient.name.family")
}
```

`MustCompile` entra en pánico si la expresión es inválida, lo que expone errores de sintaxis
inmediatamente al inicio en lugar de en tiempo de ejecución.

## Caché de Expresiones

Cuando las expresiones no se conocen en tiempo de compilación (por ejemplo, expresiones de búsqueda
proporcionadas por el usuario o expresiones cargadas desde configuración), use la caché de expresiones:

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func evaluateUserExpression(resource []byte, userExpr string) (fhirpath.Collection, error) {
    // GetCached compiles on the first call and returns the cached
    // *Expression on subsequent calls.
    expr, err := fhirpath.GetCached(userExpr)
    if err != nil {
        return nil, fmt.Errorf("invalid expression: %w", err)
    }
    return expr.Evaluate(resource)
}
```

Consulte [Caché de Expresiones](../caching/) para detalles sobre dimensionamiento de caché, precalentamiento y
monitoreo.

### Cuándo Usar Cada Enfoque

| Escenario                                   | Enfoque                     |
|---------------------------------------------|-----------------------------|
| Expresión codificada, conocida en tiempo de compilación | `MustCompile` como variable de paquete |
| Expresión de configuración, cargada una vez  | `Compile` al inicio          |
| Expresión dinámica, muchos valores distintos  | `GetCached` / `ExpressionCache` |
| Expresión de un solo uso, nunca reutilizada   | `Evaluate` (sin caché)      |

## Pre-serialización de Recursos

Si tiene una estructura Go contra la que necesita evaluar múltiples expresiones,
serialícela a JSON **una vez** usando `ResourceJSON` en lugar de dejar que cada
evaluación llame a `json.Marshal`:

### Malo: Serializar en Cada Evaluación

```go
type MyPatient struct {
    ResourceType string `json:"resourceType"`
    ID           string `json:"id"`
    // ... many fields
}

func (p *MyPatient) GetResourceType() string { return p.ResourceType }

// BAD -- marshals the struct to JSON on every call.
func validatePatient(p *MyPatient) error {
    _, err := fhirpath.EvaluateResource(p, "Patient.name.exists()")
    if err != nil {
        return err
    }
    _, err = fhirpath.EvaluateResource(p, "Patient.birthDate.exists()")
    return err
}
```

### Bueno: Serializar Una Vez con ResourceJSON

```go
// GOOD -- serialize once, evaluate many times.
func validatePatient(p *MyPatient) error {
    rj, err := fhirpath.NewResourceJSON(p)
    if err != nil {
        return fmt.Errorf("serialize: %w", err)
    }

    // Each call reuses the pre-serialized JSON bytes.
    _, err = rj.EvaluateCached("Patient.name.exists()")
    if err != nil {
        return err
    }
    _, err = rj.EvaluateCached("Patient.birthDate.exists()")
    return err
}
```

Para un rendimiento aún mejor, mantenga los bytes JSON `[]byte` cuando ya los tenga
(por ejemplo, del cuerpo de una solicitud HTTP) y evalúe directamente contra ellos:

```go
func handleCreatePatient(body []byte) error {
    // body is already JSON -- no marshalling needed.
    result, err := fhirpath.EvaluateCached(body, "Patient.name.exists()")
    if err != nil {
        return err
    }
    // ...
}
```

## Filtrar Temprano

Cuando una expresión opera sobre una colección grande, use `where()` para reducir su tamaño
lo antes posible. Esto minimiza el número de elementos que las funciones posteriores
deben procesar.

### Malo: Procesar Todo, Filtrar Tarde

```go
// BAD -- descendants() expands the entire resource tree, then filters.
expr := fhirpath.MustCompile(
    "Bundle.entry.resource.descendants().ofType(Coding).where(system = 'http://loinc.org')",
)
```

### Bueno: Filtrar en Cada Nivel

```go
// GOOD -- filter entries first, then navigate to the specific element.
expr := fhirpath.MustCompile(
    "Bundle.entry.resource.ofType(Observation).code.coding.where(system = 'http://loinc.org')",
)
```

La segunda expresión evita llamar a `descendants()` completamente. En su lugar, se reduce
primero a recursos Observation, luego navega directamente al elemento de código.

### Límites de Tamaño de Colección

Como red de seguridad, establezca `WithMaxCollectionSize` al evaluar expresiones no confiables
para prevenir que consultas patológicas consuman memoria sin límite:

```go
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithMaxCollectionSize(5000),
)
```

## Evitar Conversiones Innecesarias

La biblioteca trabaja con `types.Collection` (un slice de `types.Value`). Evite
ir y venir a través de tipos nativos de Go cuando pueda trabajar con los valores FHIRPath
directamente.

### Malo: Convertir a String Solo para Comparar

```go
// BAD -- unnecessary string conversion.
result, _ := expr.Evaluate(patient)
for _, v := range result {
    str := v.String()
    if str == "active" {
        // ...
    }
}
```

### Bueno: Usar Comparación Consciente del Tipo

```go
// GOOD -- compare at the FHIRPath type level.
result, _ := expr.Evaluate(patient)
for _, v := range result {
    if s, ok := v.(types.String); ok && s.Value() == "active" {
        // ...
    }
}
```

### Usar Funciones de Conveniencia

Para patrones de extracción comunes, use las funciones de conveniencia incorporadas que manejan
la conversión de tipos correctamente:

```go
// Extract a single boolean.
active, err := fhirpath.EvaluateToBoolean(patient, "Patient.active")

// Extract a single string.
family, err := fhirpath.EvaluateToString(patient, "Patient.name.first().family")

// Extract multiple strings.
givens, err := fhirpath.EvaluateToStrings(patient, "Patient.name.first().given")

// Check existence.
hasName, err := fhirpath.Exists(patient, "Patient.name")

// Count results.
nameCount, err := fhirpath.Count(patient, "Patient.name")
```

## Resumen de Mejores Prácticas

1. **Compile expresiones una vez.** Use `MustCompile` para expresiones codificadas o
   `GetCached` para dinámicas. Nunca llame a `Evaluate()` en un bucle intensivo.

2. **Use la caché de expresiones** para expresiones dinámicas. Dimensiónela apropiadamente y
   monitoree la tasa de aciertos.

3. **Pre-serialice recursos** al evaluar múltiples expresiones contra la misma
   estructura Go. Use `ResourceJSON` o mantenga los bytes crudos `[]byte`.

4. **Filtre temprano.** Use `where()` y `ofType()` para reducir colecciones antes de
   aplicar operaciones costosas como `descendants()`.

5. **Establezca límites de seguridad.** Use `WithTimeout`, `WithMaxDepth` y
   `WithMaxCollectionSize` al evaluar expresiones no confiables.

6. **Evite conversiones de tipos innecesarias.** Trabaje con `types.Value` directamente y use
   las funciones de conveniencia (`EvaluateToString`, `EvaluateToBoolean`, etc.) cuando
   necesite tipos nativos de Go.

7. **Precaliente la caché al inicio** para aplicaciones sensibles a la latencia. Esto también
   valida la sintaxis de las expresiones tempranamente.

8. **Perfile antes de optimizar.** Use las herramientas incorporadas de benchmarking y profiling de Go
   (`go test -bench`, `pprof`) para identificar cuellos de botella reales antes de aplicar
   optimizaciones.
