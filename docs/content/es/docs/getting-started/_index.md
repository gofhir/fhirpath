---
title: "Primeros Pasos"
linkTitle: "Primeros Pasos"
description: "Instala la biblioteca FHIRPath Go, evalúa tu primera expresión y aprende los patrones principales de la API para compilar, almacenar en caché y extraer datos de recursos FHIR®."
weight: 1
---

Esta guía te llevará paso a paso por la instalación de la biblioteca, la ejecución de tu primera evaluación FHIRPath y la adopción de los patrones que usarás en código de producción.

## Requisitos Previos

- **Go 1.23** o posterior.
- Un módulo Go (`go.mod`) en tu proyecto. Si aún no tienes uno, ejecuta `go mod init <tu-modulo>`.

## Instalación

Agrega la biblioteca a tu proyecto con `go get`:

```bash
go get github.com/gofhir/fhirpath
```

Luego impórtala en tus archivos fuente de Go:

```go
import "github.com/gofhir/fhirpath"
```

## Tu Primera Evaluación

La forma más sencilla de evaluar una expresión FHIRPath es la función de nivel superior `Evaluate`. Acepta bytes JSON sin procesar que representan un recurso FHIR® y una cadena de expresión FHIRPath, y devuelve una `Collection` de resultados.

```go
package main

import (
    "fmt"
    "log"

    "github.com/gofhir/fhirpath"
)

func main() {
    // Define un recurso FHIR Patient como JSON
    patient := []byte(`{
        "resourceType": "Patient",
        "id": "123",
        "name": [{"family": "Doe", "given": ["John"]}],
        "birthDate": "1990-05-15"
    }`)

    // Evalúa una expresión FHIRPath
    result, err := fhirpath.Evaluate(patient, "Patient.name.family")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result) // [Doe]
}
```

`Evaluate` compila y evalúa la expresión en una sola llamada. Devuelve un `types.Collection` (un alias de `[]types.Value`), que contiene el resultado de la evaluación. Cada expresión FHIRPath produce una colección -- incluso un solo valor escalar se envuelve en una colección de un elemento, y una ruta inexistente produce una colección vacía.

## Compilación de Expresiones

Si planeas evaluar la misma expresión contra muchos recursos, compílala una vez con `Compile` o `MustCompile` y luego reutiliza el `*Expression` resultante:

```go
// Compila una vez (devuelve un error si la expresión es inválida)
expr, err := fhirpath.Compile("Patient.name.given")
if err != nil {
    log.Fatal(err)
}

// Evalúa contra múltiples recursos
result1, _ := expr.Evaluate(patient1JSON)
result2, _ := expr.Evaluate(patient2JSON)
```

`MustCompile` es una variante de conveniencia que entra en pánico en lugar de devolver un error. Es útil para variables a nivel de paquete donde la expresión se conoce en tiempo de compilación:

```go
var nameExpr = fhirpath.MustCompile("Patient.name.family")
```

## Caché de Expresiones

Para cargas de trabajo de producción donde las expresiones pueden llegar en tiempo de ejecución (por ejemplo, desde configuración o entrada del usuario), usa `EvaluateCached`. Mantiene un caché LRU global y seguro para hilos de expresiones compiladas, de modo que las evaluaciones repetidas de la misma cadena de expresión no pagan el costo de compilación más de una vez:

```go
result, err := fhirpath.EvaluateCached(patientJSON, "Patient.birthDate")
```

El caché predeterminado almacena hasta 1,000 expresiones. Puedes crear un caché personalizado para un control más fino:

```go
cache := fhirpath.NewExpressionCache(500) // tamaño personalizado

expr, err := cache.Get("Patient.name.family")
if err != nil {
    log.Fatal(err)
}

result, err := expr.Evaluate(patientJSON)
```

Puedes inspeccionar el rendimiento del caché en cualquier momento:

```go
stats := cache.Stats()
fmt.Printf("Tamaño: %d, Aciertos: %d, Fallos: %d, Tasa de aciertos: %.1f%%\n",
    stats.Size, stats.Hits, stats.Misses, cache.HitRate())
```

## Funciones de Conveniencia

La biblioteca proporciona varias funciones de conveniencia tipadas que evalúan una expresión y extraen el resultado en una sola llamada. Todas ellas usan el caché de expresiones internamente.

### EvaluateToBoolean

Devuelve un `bool` de Go. Útil para expresiones FHIRPath que producen un único valor Boolean, como restricciones de validación:

```go
active, err := fhirpath.EvaluateToBoolean(patientJSON, "Patient.active")
if err != nil {
    log.Fatal(err)
}
fmt.Println(active) // true o false
```

### EvaluateToString

Devuelve un único `string` de Go:

```go
family, err := fhirpath.EvaluateToString(patientJSON, "Patient.name.first().family")
if err != nil {
    log.Fatal(err)
}
fmt.Println(family) // Doe
```

### EvaluateToStrings

Devuelve un `[]string` que contiene la representación en cadena de cada valor en la colección de resultados:

```go
givenNames, err := fhirpath.EvaluateToStrings(patientJSON, "Patient.name.given")
if err != nil {
    log.Fatal(err)
}
fmt.Println(givenNames) // [John]
```

### Exists

Devuelve `true` si la expresión produce una colección no vacía:

```go
hasPhone, err := fhirpath.Exists(patientJSON, "Patient.telecom.where(system='phone')")
if err != nil {
    log.Fatal(err)
}
fmt.Println(hasPhone) // true o false
```

### Count

Devuelve el número de elementos en la colección de resultados:

```go
nameCount, err := fhirpath.Count(patientJSON, "Patient.name")
if err != nil {
    log.Fatal(err)
}
fmt.Println(nameCount) // 1
```

## Manejo de Errores

La biblioteca reporta dos categorías de errores:

1. **Errores de compilación** -- devueltos por `Compile` (o lanzados por `MustCompile` como pánico) cuando la cadena de expresión contiene sintaxis FHIRPath inválida.

2. **Errores de evaluación** -- devueltos por `Evaluate` y funciones relacionadas cuando ocurre un error en tiempo de ejecución (por ejemplo, al comparar tipos incompatibles).

Un patrón típico de manejo de errores se ve así:

```go
result, err := fhirpath.Evaluate(resource, expr)
if err != nil {
    // Maneja o registra el error
    return fmt.Errorf("la evaluación de fhirpath falló: %w", err)
}

if result.Empty() {
    // La ruta se resolvió pero no produjo valores
    fmt.Println("No se encontraron resultados")
} else {
    fmt.Println("Resultado:", result)
}
```

Las colecciones vacías no son errores. En FHIRPath, navegar a una ruta que no existe simplemente devuelve una colección vacía (`{}`). Siempre verifica `result.Empty()` antes de extraer valores.

## Evaluación con Opciones

Para casos de uso avanzados, puedes pasar opciones funcionales para controlar tiempos de espera, límites de recursión y variables personalizadas:

```go
expr := fhirpath.MustCompile("Patient.name.family")

result, err := expr.EvaluateWithOptions(patientJSON,
    fhirpath.WithTimeout(2*time.Second),
    fhirpath.WithMaxDepth(50),
    fhirpath.WithVariable("name", types.Collection{types.NewString("test")}),
)
```

Consulta [Variables de Entorno]({{< relref "../concepts/environment-variables" >}}) para más detalles sobre variables personalizadas.

## Próximos Pasos

- **[Sistema de Tipos]({{< relref "../concepts/type-system" >}})** -- aprende sobre los ocho tipos FHIRPath y cómo se mapean a tipos de Go.
- **[Colecciones]({{< relref "../concepts/collections" >}})** -- comprende la propagación de vacío, la evaluación singleton y las operaciones de colección.
- **[Operadores]({{< relref "../concepts/operators" >}})** -- referencia para operadores aritméticos, de comparación, booleanos y de colección.
- **[Variables de Entorno]({{< relref "../concepts/environment-variables" >}})** -- usa variables integradas y personalizadas en tus expresiones.
