---
title: "Seguridad en Hilos"
linkTitle: "Seguridad en Hilos"
weight: 6
description: >
  Comprenda el modelo de concurrencia: qué objetos son seguros para compartir entre goroutines
  y cuáles deben permanecer por evaluación.
---

## Qué Es Seguro para Hilos

Los siguientes objetos están diseñados para ser **compartidos de forma segura** entre múltiples goroutines:

### Expresión Compilada (`*Expression`)

Un `*Expression` compilado es inmutable después de su creación. Solo contiene el AST analizado
y la cadena de expresión original. Puede llamar a `Evaluate()` o
`EvaluateWithOptions()` desde cualquier número de goroutines simultáneamente:

```go
// Safe: one expression, many goroutines.
expr := fhirpath.MustCompile("Patient.name.family")

var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(resource []byte) {
        defer wg.Done()
        result, err := expr.Evaluate(resource)
        // handle result...
    }(resources[i])
}
wg.Wait()
```

### Caché de Expresiones (`*ExpressionCache`)

El `ExpressionCache` (incluyendo el global `DefaultCache`) usa un `sync.RWMutex`
internamente. Todos los métodos -- `Get()`, `MustGet()`, `Clear()`, `Size()`, `Stats()`
y `HitRate()` -- son seguros para uso concurrente:

```go
// Safe: concurrent cache access from multiple goroutines.
cache := fhirpath.NewExpressionCache(500)

var wg sync.WaitGroup
for _, exprStr := range expressions {
    wg.Add(1)
    go func(e string) {
        defer wg.Done()
        compiled, err := cache.Get(e)
        // use compiled...
    }(exprStr)
}
wg.Wait()
```

### Funciones de Conveniencia

Las funciones de nivel superior `EvaluateCached()`, `GetCached()` y `MustGetCached()` todas
delegan a `DefaultCache` y por lo tanto son seguras para uso concurrente.

## Qué No Se Comparte

### Contexto de Evaluación (`eval.Context`)

Cada llamada a `Evaluate()` o `EvaluateWithOptions()` crea un **nuevo** `eval.Context`
internamente. El contexto contiene estado mutable de evaluación: el valor actual `$this`,
`$index`, variables y límites. **Nunca** debe compartirse entre evaluaciones
concurrentes.

Normalmente no necesita preocuparse por esto porque la API pública crea un contexto
nuevo por llamada. Sin embargo, si crea un `eval.Context` manualmente, no lo reutilice
entre goroutines:

```go
// WRONG -- sharing a context between goroutines.
ctx := eval.NewContext(resource)
go func() { expr1.EvaluateWithContext(ctx) }() // DATA RACE
go func() { expr2.EvaluateWithContext(ctx) }() // DATA RACE
```

```go
// CORRECT -- each goroutine gets its own context.
go func() {
    ctx := eval.NewContext(resource)
    expr1.EvaluateWithContext(ctx)
}()
go func() {
    ctx := eval.NewContext(resource)
    expr2.EvaluateWithContext(ctx)
}()
```

### Slices de Bytes del Recurso

Los datos del recurso `[]byte` pasados a `Evaluate()` son de **solo lectura** durante la evaluación.
Es seguro pasar el mismo slice de bytes a múltiples evaluaciones concurrentes siempre que
ninguna goroutine lo mute durante la evaluación.

## Patrón de Evaluación Concurrente

El patrón concurrente más común es un fan-out donde múltiples recursos se
evalúan en paralelo usando la misma expresión compilada:

```go
package main

import (
    "fmt"
    "sync"

    "github.com/gofhir/fhirpath"
)

func main() {
    // Compile once.
    expr := fhirpath.MustCompile("Patient.name.family")

    resources := [][]byte{
        []byte(`{"resourceType":"Patient","name":[{"family":"Smith"}]}`),
        []byte(`{"resourceType":"Patient","name":[{"family":"Johnson"}]}`),
        []byte(`{"resourceType":"Patient","name":[{"family":"Williams"}]}`),
        []byte(`{"resourceType":"Patient","name":[{"family":"Brown"}]}`),
    }

    results := make([]fhirpath.Collection, len(resources))
    errors := make([]error, len(resources))

    var wg sync.WaitGroup
    for i, res := range resources {
        wg.Add(1)
        go func(idx int, resource []byte) {
            defer wg.Done()
            results[idx], errors[idx] = expr.Evaluate(resource)
        }(i, res)
    }
    wg.Wait()

    for i, result := range results {
        if errors[i] != nil {
            fmt.Printf("resource %d: error: %v\n", i, errors[i])
        } else {
            fmt.Printf("resource %d: %v\n", i, result)
        }
    }
}
```

## Patrones de Producción

### Manejador HTTP

En un servidor HTTP, cada manejador de solicitud se ejecuta en su propia goroutine. Las expresiones
compiladas y la caché de expresiones pueden compartirse entre todos los manejadores:

```go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "time"

    "github.com/gofhir/fhirpath"
)

// expressionCache is shared across all request handlers.
var expressionCache = fhirpath.NewExpressionCache(1000)

// Pre-compiled expressions for known operations.
var (
    exprFamilyName = fhirpath.MustCompile("Patient.name.family")
    exprBirthDate  = fhirpath.MustCompile("Patient.birthDate")
    exprActive     = fhirpath.MustCompile("Patient.active")
)

func handleExtract(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "failed to read body", http.StatusBadRequest)
        return
    }

    // Each evaluation creates its own internal eval.Context --
    // no shared mutable state between requests.
    family, err := exprFamilyName.EvaluateWithOptions(body,
        fhirpath.WithContext(r.Context()),
        fhirpath.WithTimeout(2*time.Second),
    )
    if err != nil {
        http.Error(w, fmt.Sprintf("evaluation error: %v", err),
            http.StatusInternalServerError)
        return
    }

    resp := map[string]interface{}{
        "familyName": family.String(),
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func handleDynamic(w http.ResponseWriter, r *http.Request) {
    // For user-supplied expressions, use the cache.
    exprStr := r.URL.Query().Get("expression")
    if exprStr == "" {
        http.Error(w, "missing expression parameter", http.StatusBadRequest)
        return
    }

    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "failed to read body", http.StatusBadRequest)
        return
    }

    // GetCached is safe for concurrent use.
    compiled, err := expressionCache.Get(exprStr)
    if err != nil {
        http.Error(w, fmt.Sprintf("invalid expression: %v", err),
            http.StatusBadRequest)
        return
    }

    result, err := compiled.EvaluateWithOptions(body,
        fhirpath.WithContext(r.Context()),
        fhirpath.WithTimeout(2*time.Second),
        fhirpath.WithMaxCollectionSize(1000),
    )
    if err != nil {
        http.Error(w, fmt.Sprintf("evaluation error: %v", err),
            http.StatusInternalServerError)
        return
    }

    resp := map[string]interface{}{
        "result": result.String(),
    }
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func main() {
    http.HandleFunc("/extract", handleExtract)
    http.HandleFunc("/evaluate", handleDynamic)
    log.Println("listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Pool de Workers

Para procesamiento por lotes (por ejemplo, validar todos los recursos en una base de datos), use un
pool de workers para limitar la concurrencia:

```go
package main

import (
    "fmt"
    "sync"
    "time"

    "github.com/gofhir/fhirpath"
)

// ValidationResult holds the outcome for one resource.
type ValidationResult struct {
    Index int
    Valid bool
    Error error
}

func validateBatch(
    resources [][]byte,
    expression *fhirpath.Expression,
    workers int,
) []ValidationResult {
    jobs := make(chan int, len(resources))
    results := make([]ValidationResult, len(resources))

    var wg sync.WaitGroup
    for w := 0; w < workers; w++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for idx := range jobs {
                result, err := expression.EvaluateWithOptions(resources[idx],
                    fhirpath.WithTimeout(2*time.Second),
                    fhirpath.WithMaxCollectionSize(5000),
                )
                if err != nil {
                    results[idx] = ValidationResult{Index: idx, Error: err}
                    continue
                }

                // Check if the expression returned true (valid).
                valid := false
                if !result.Empty() {
                    valid = result.String() == "[true]"
                }
                results[idx] = ValidationResult{Index: idx, Valid: valid}
            }
        }()
    }

    // Enqueue all jobs.
    for i := range resources {
        jobs <- i
    }
    close(jobs)

    wg.Wait()
    return results
}

func main() {
    // Compile the validation expression once.
    expr := fhirpath.MustCompile("Patient.name.exists() and Patient.birthDate.exists()")

    resources := [][]byte{
        []byte(`{"resourceType":"Patient","name":[{"family":"Doe"}],"birthDate":"1990-01-15"}`),
        []byte(`{"resourceType":"Patient","name":[{"family":"Smith"}]}`),
        []byte(`{"resourceType":"Patient","birthDate":"1985-03-22"}`),
        []byte(`{"resourceType":"Patient","name":[{"family":"Lee"}],"birthDate":"2000-07-04"}`),
    }

    // Process with 4 worker goroutines.
    results := validateBatch(resources, expr, 4)

    for _, r := range results {
        if r.Error != nil {
            fmt.Printf("resource %d: error: %v\n", r.Index, r.Error)
        } else {
            fmt.Printf("resource %d: valid=%v\n", r.Index, r.Valid)
        }
    }
    // Output:
    // resource 0: valid=true
    // resource 1: valid=false
    // resource 2: valid=false
    // resource 3: valid=true
}
```

## Resumen

| Objeto                        | Seguro para Hilos | Notas                                                 |
|-------------------------------|-------------------|-------------------------------------------------------|
| `*Expression`                 | Si                | Inmutable después de creación; comparta libremente     |
| `*ExpressionCache`            | Si                | Usa `sync.RWMutex` internamente                       |
| `DefaultCache`                | Si                | `*ExpressionCache` global                              |
| `EvaluateCached()`, `GetCached()` | Si           | Delegan a `DefaultCache`                               |
| `eval.Context`                | **No**            | Creado por evaluación; nunca compartir entre goroutines |
| Recurso `[]byte`              | Solo lectura      | Seguro si ninguna goroutine lo muta durante evaluación |
| `ReferenceResolver`           | Depende           | Su implementación debe ser segura para uso concurrente |
| `TerminologyService`          | Depende           | Su implementación debe ser segura para uso concurrente |
| `ProfileValidator`            | Depende           | Su implementación debe ser segura para uso concurrente |
