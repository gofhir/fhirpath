---
title: "Caché de Expresiones"
linkTitle: "Caché de Expresiones"
weight: 1
description: >
  Use la caché de expresiones LRU incorporada para evitar el análisis redundante y mejorar
  drásticamente el rendimiento en cargas de trabajo de producción.
---

## Por Qué Cachear Expresiones

Cada expresión FHIRPath debe ser **analizada** en un AST antes de poder evaluarse.
El análisis involucra análisis léxico y coincidencia gramatical, lo cual es órdenes de magnitud
más costoso que la subsecuente evaluación por recorrido del árbol.

```text
Compile("Patient.name.family")   ~250 us   (parse + build AST)
expr.Evaluate(resource)          ~5 us     (walk the cached AST)
```

En un servidor FHIR típico evaluará el mismo puñado de expresiones (restricciones de
validación, parámetros de búsqueda, reglas de extracción) millones de veces contra diferentes
recursos. Cachear los objetos `*Expression` compilados elimina el costo de análisis para
cada llamada después de la primera.

## La Caché por Defecto

La biblioteca incluye una caché global lista para usar:

```go
// DefaultCache is a global expression cache with a 1 000-entry LRU limit.
var DefaultCache = NewExpressionCache(1000)
```

Puede usarla a través de las funciones de conveniencia:

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func main() {
    patient := []byte(`{
        "resourceType": "Patient",
        "name": [{"family": "Doe", "given": ["John"]}]
    }`)

    // EvaluateCached compiles (with caching) and evaluates in one call.
    result, err := fhirpath.EvaluateCached(patient, "Patient.name.family")
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // [Doe]

    // Or retrieve the compiled expression directly:
    expr, err := fhirpath.GetCached("Patient.name.given")
    if err != nil {
        panic(err)
    }
    fmt.Println(expr.Evaluate(patient)) // [John] <nil>
}
```

El `DefaultCache` es seguro para uso concurrente. En la primera llamada para una cadena de
expresión dada, la caché la compila y almacena el resultado; las llamadas subsecuentes
devuelven el `*Expression` cacheado sin análisis.

### MustGetCached

Cuando sabe que la expresión es sintácticamente válida (por ejemplo, un literal
codificado directamente), puede omitir el manejo de errores:

```go
expr := fhirpath.MustGetCached("Patient.name.family")
```

`MustGetCached` entra en pánico si la expresión no puede compilarse. Úselo solo para
expresiones cuya sintaxis está garantizada en tiempo de desarrollo.

## Cachés Personalizadas

Si necesita espacios de nombres de caché independientes o límites de tamaño diferentes, cree su propio
`ExpressionCache`:

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func main() {
    // A small cache for hot-path validation rules.
    validationCache := fhirpath.NewExpressionCache(100)

    // A larger cache for ad-hoc search parameter extraction.
    searchCache := fhirpath.NewExpressionCache(5000)

    patient := []byte(`{
        "resourceType": "Patient",
        "active": true
    }`)

    // Each cache tracks its own entries and statistics.
    expr, _ := validationCache.Get("Patient.active")
    result, _ := expr.Evaluate(patient)
    fmt.Println(result) // [true]

    expr2, _ := searchCache.Get("Patient.active")
    result2, _ := expr2.Evaluate(patient)
    fmt.Println(result2) // [true]

    fmt.Println(validationCache.Size()) // 1
    fmt.Println(searchCache.Size())     // 1
}
```

### Caché Sin Límite

Pase `0` (o cualquier valor no positivo) como límite para crear una caché que nunca
desaloje entradas:

```go
// This cache will grow without bound -- only use when you know
// the set of possible expressions is finite and small.
cache := fhirpath.NewExpressionCache(0)
```

## Estadísticas de la Caché

La caché rastrea aciertos y fallos para que pueda monitorear su efectividad:

```go
package main

import (
    "fmt"
    "log"
    "github.com/gofhir/fhirpath"
)

func main() {
    cache := fhirpath.NewExpressionCache(500)

    expressions := []string{
        "Patient.name.family",
        "Patient.birthDate",
        "Patient.name.family", // duplicate -- will be a hit
        "Patient.active",
        "Patient.birthDate",   // duplicate -- will be a hit
    }

    for _, expr := range expressions {
        _, err := cache.Get(expr)
        if err != nil {
            log.Fatal(err)
        }
    }

    // Retrieve aggregate statistics.
    stats := cache.Stats()
    fmt.Printf("Size:   %d\n", stats.Size)   // 3
    fmt.Printf("Limit:  %d\n", stats.Limit)  // 500
    fmt.Printf("Hits:   %d\n", stats.Hits)    // 2
    fmt.Printf("Misses: %d\n", stats.Misses)  // 3

    // Or get the hit rate directly as a percentage (0-100).
    fmt.Printf("Hit rate: %.1f%%\n", cache.HitRate()) // 40.0%
}
```

### Uso de Estadísticas para Monitoreo

En un sistema de producción podría exponer las estadísticas de la caché como métricas de Prometheus o
líneas de registro periódicas:

```go
func reportCacheMetrics(cache *fhirpath.ExpressionCache) {
    stats := cache.Stats()
    log.Printf(
        "fhirpath_cache size=%d limit=%d hits=%d misses=%d hit_rate=%.1f%%",
        stats.Size, stats.Limit, stats.Hits, stats.Misses, cache.HitRate(),
    )
}
```

Si la tasa de aciertos es consistentemente baja, el límite de su caché puede ser demasiado pequeño y el LRU
está desalojando entradas que aún se necesitan. Considere aumentar el límite.

## Precalentamiento de la Caché

Para aplicaciones sensibles a la latencia puede **pre-compilar** sus expresiones conocidas al
inicio para que la primera solicitud real no pague el costo de análisis:

```go
package main

import (
    "log"
    "github.com/gofhir/fhirpath"
)

// expressions lists every FHIRPath expression the application uses.
var expressions = []string{
    "Patient.name.family",
    "Patient.name.given",
    "Patient.birthDate",
    "Patient.identifier.where(system = 'http://hl7.org/fhir/sid/us-ssn').value",
    "Patient.telecom.where(system = 'phone').value",
    "Patient.address.where(use = 'home')",
    "Observation.code.coding.where(system = 'http://loinc.org').code",
    "Observation.value.ofType(Quantity).value",
}

func warmCache(cache *fhirpath.ExpressionCache) {
    for _, expr := range expressions {
        if _, err := cache.Get(expr); err != nil {
            log.Fatalf("invalid expression during cache warm-up: %s -- %v", expr, err)
        }
    }
    log.Printf("Cache warmed with %d expressions", cache.Size())
}

func main() {
    cache := fhirpath.NewExpressionCache(1000)
    warmCache(cache)

    // The cache now contains compiled ASTs for all known expressions.
    // Subsequent Get() calls for these expressions will be instant cache hits.
}
```

El precalentamiento también sirve como paso de **validación temprana**: si alguna expresión tiene un error
de sintaxis, la aplicación falla inmediatamente al inicio en lugar de en tiempo de ejecución cuando
llega una solicitud.

## Consideraciones de Memoria

Cada `*Expression` cacheado mantiene un árbol de análisis (AST) en memoria. El tamaño depende de
la complejidad de la expresión, pero una expresión típica consume unos pocos kilobytes.

| Límite de Caché | Memoria Aproximada |
|-----------------|-------------------|
| 100             | ~0.5 MB           |
| 1 000           | ~5 MB             |
| 10 000          | ~50 MB            |

Estas son estimaciones aproximadas. El uso real depende de la complejidad de la expresión.

**Directrices para elegir un límite de caché:**

1. **Comience con el valor por defecto (1 000).** Esto es suficiente para la mayoría de aplicaciones
   que evalúan un conjunto fijo de expresiones.
2. **Aumente el límite** si su tasa de aciertos está por debajo del 90% y tiene memoria disponible.
3. **Use cachés separadas** cuando diferentes subsistemas tienen conjuntos de expresiones muy diferentes
   (por ejemplo, reglas de validación vs. extracción de parámetros de búsqueda). Esto
   evita que un subsistema desaloje entradas que otro necesita.
4. **Llame a `Clear()`** si necesita liberar memoria o reiniciar estadísticas:

   ```go
   cache.Clear() // Removes all entries and resets hit/miss counters.
   ```

## Resumen

| Función / Método               | Descripción                                       |
|-------------------------------|---------------------------------------------------|
| `DefaultCache`                | `ExpressionCache` global con un límite de 1 000 entradas |
| `NewExpressionCache(limit)`   | Crear una caché personalizada con el límite LRU dado |
| `cache.Get(expr)`             | Recuperar o compilar una expresión                 |
| `cache.MustGet(expr)`         | Como `Get` pero entra en pánico si hay error       |
| `cache.Clear()`               | Eliminar todas las entradas y reiniciar contadores |
| `cache.Size()`                | Número de entradas actualmente en caché            |
| `cache.Stats()`               | Devuelve `CacheStats{Size, Limit, Hits, Misses}`  |
| `cache.HitRate()`             | Tasa de aciertos como porcentaje float64 (0--100)  |
| `GetCached(expr)`             | Atajo para `DefaultCache.Get(expr)`                |
| `MustGetCached(expr)`         | Atajo para `DefaultCache.MustGet(expr)`            |
| `EvaluateCached(resource, expr)` | Compilar con caché + evaluar en una sola llamada |
