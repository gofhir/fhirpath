---
title: "Colecciones"
linkTitle: "Colecciones"
description: "Cómo FHIRPath representa los resultados como colecciones ordenadas, las reglas de propagación vacía y lógica de tres valores, evaluación singleton y el conjunto completo de operaciones de colección."
weight: 2
---

En FHIRPath, **toda expresión se evalúa a una colección**. Una colección es una lista ordenada de cero o más valores. No existen valores escalares independientes -- incluso un solo Boolean `true` se representa como una colección de un elemento que contiene ese Boolean.

En la biblioteca FHIRPath para Go, una colección se define como:

```go
type Collection []Value
```

Este es el tipo de retorno fundamental para todas las expresiones FHIRPath.

## Colecciones Vacías

Una colección vacía (`{}` en la notación FHIRPath, `Collection{}` o `nil` en Go) representa la ausencia de un valor. Navegar a una ruta que no existe en un recurso siempre produce una colección vacía en lugar de un error.

```go
result, _ := fhirpath.Evaluate(patientJSON, "Patient.deceased")
if result.Empty() {
    fmt.Println("No deceased value present")
}
```

## Propagación Vacía (Lógica de Tres Valores)

FHIRPath utiliza **lógica de tres valores** donde los tres estados son `true`, `false` y **vacío** (desconocido). Cuando un operando de la mayoría de los operadores o funciones es una colección vacía, el resultado se propaga como vacío en lugar de producir un error.

Por ejemplo:

```text
{} = 5        --> {}      (empty, not false)
{} and true   --> {}      (empty, not false)
{} + 3        --> {}      (empty, not an error)
```

Esto es diferente de muchos lenguajes de programación donde un valor nulo o ausente causaría un error. FHIRPath está diseñado para datos de salud donde los valores faltantes son comunes y esperados.

Los operadores Boolean (`and`, `or`, `implies`) tienen reglas especiales de propagación. Por ejemplo, `false and {}` se evalúa como `false` (no vacío), porque independientemente del valor desconocido, el resultado debe ser `false`. Consulte la página de [Operadores]({{< relref "operators" >}}) para ver las tablas de verdad completas de tres valores.

## Evaluación Singleton

Muchos operadores (como `=`, `<`, `+`) esperan colecciones **singleton** (colecciones con exactamente un elemento). Cuando estos operadores reciben una colección, FHIRPath aplica **evaluación singleton**:

- Si la colección tiene exactamente **un** elemento, ese elemento se usa como operando.
- Si la colección está **vacía**, el resultado es vacío (por propagación vacía).
- Si la colección tiene **más de un** elemento, el comportamiento depende del operador -- la mayoría retorna vacío o genera un error.

```go
// Single-element collection: works as expected
result, _ := fhirpath.Evaluate(patientJSON, "Patient.birthDate = @1990-05-15")
// result: [true]

// Multi-element collection on one side: empty result
result, _ = fhirpath.Evaluate(patientJSON, "Patient.name.given = 'John'")
// If Patient has multiple given names, this may return empty
```

## Métodos de Colección

El tipo `Collection` proporciona un amplio conjunto de métodos para trabajar con resultados en código Go.

### Acceso Básico

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Empty()` | `func (c Collection) Empty() bool` | Retorna `true` si la colección no tiene elementos. |
| `Count()` | `func (c Collection) Count() int` | Retorna el número de elementos. |
| `First()` | `func (c Collection) First() (Value, bool)` | Retorna el primer elemento y `true`, o `nil` y `false` si está vacía. |
| `Last()` | `func (c Collection) Last() (Value, bool)` | Retorna el último elemento y `true`, o `nil` y `false` si está vacía. |
| `Single()` | `func (c Collection) Single() (Value, error)` | Retorna el único elemento. Genera error si está vacía o tiene más de un elemento. |

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patientJSON, "Patient.name.given")

if result.Empty() {
    fmt.Println("No given names found")
}

fmt.Println("Count:", result.Count())

if first, ok := result.First(); ok {
    fmt.Println("First:", first)
}

if last, ok := result.Last(); ok {
    fmt.Println("Last:", last)
}
```

### Subconjuntos

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Tail()` | `func (c Collection) Tail() Collection` | Retorna todos los elementos excepto el primero. |
| `Skip(n)` | `func (c Collection) Skip(n int) Collection` | Retorna una colección con los primeros `n` elementos eliminados. |
| `Take(n)` | `func (c Collection) Take(n int) Collection` | Retorna una colección con solo los primeros `n` elementos. |

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patientJSON, "Patient.name")

// Get everything after the first name
rest := result.Tail()

// Pagination-style operations
page := result.Skip(10).Take(5) // elements 11-15
```

### Operaciones de Conjunto

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Union(other)` | `func (c Collection) Union(other Collection) Collection` | Retorna la unión de ambas colecciones con duplicados eliminados. |
| `Combine(other)` | `func (c Collection) Combine(other Collection) Collection` | Concatena ambas colecciones, preservando duplicados. |
| `Intersect(other)` | `func (c Collection) Intersect(other Collection) Collection` | Retorna los elementos presentes en ambas colecciones. |
| `Exclude(other)` | `func (c Collection) Exclude(other Collection) Collection` | Retorna los elementos en `c` que no están en `other`. |
| `Distinct()` | `func (c Collection) Distinct() Collection` | Retorna una nueva colección con duplicados eliminados, preservando el orden de la primera aparición. |

**Ejemplos:**

```go
a, _ := fhirpath.Evaluate(patientJSON, "Patient.name.given")
b, _ := fhirpath.Evaluate(patientJSON, "Patient.contact.name.given")

// All unique given names from patient and contacts
all := a.Union(b)

// All given names including duplicates
combined := a.Combine(b)

// Given names that appear in both
shared := a.Intersect(b)

// Given names only on the patient (not contacts)
patientOnly := a.Exclude(b)

// Remove duplicates from a single collection
unique := a.Distinct()
```

La distinción entre `Union` y `Combine` es importante:
- **Union** (`|` en FHIRPath) fusiona dos colecciones y elimina duplicados.
- **Combine** concatena dos colecciones y preserva duplicados.

### Agregación Boolean

Estos métodos evalúan colecciones de valores Boolean:

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `AllTrue()` | `func (c Collection) AllTrue() bool` | Retorna `true` si cada elemento es Boolean `true`. |
| `AnyTrue()` | `func (c Collection) AnyTrue() bool` | Retorna `true` si al menos un elemento es Boolean `true`. |
| `AllFalse()` | `func (c Collection) AllFalse() bool` | Retorna `true` si cada elemento es Boolean `false`. |
| `AnyFalse()` | `func (c Collection) AnyFalse() bool` | Retorna `true` si al menos un elemento es Boolean `false`. |
| `ToBoolean()` | `func (c Collection) ToBoolean() (bool, error)` | Convierte una colección singleton Boolean a un `bool` de Go. Genera error si está vacía, tiene múltiples valores o no es Boolean. |

**Ejemplos:**

```go
// Check if all validation results are true
results, _ := fhirpath.Evaluate(bundleJSON, "Bundle.entry.resource.active")
if results.AllTrue() {
    fmt.Println("All resources are active")
}

// Extract a single boolean
isActive, _ := fhirpath.Evaluate(patientJSON, "Patient.active")
if active, err := isActive.ToBoolean(); err == nil {
    fmt.Println("Active:", active)
}
```

### Pertenencia

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Contains(v)` | `func (c Collection) Contains(v Value) bool` | Retorna `true` si la colección contiene un valor igual a `v`. |
| `IsDistinct()` | `func (c Collection) IsDistinct() bool` | Retorna `true` si todos los elementos de la colección son únicos. |

**Ejemplo:**

```go
names, _ := fhirpath.Evaluate(patientJSON, "Patient.name.given")
if names.Contains(types.NewString("John")) {
    fmt.Println("Patient has given name John")
}
```

## Trabajo con Colecciones en Go

Un patrón común es iterar sobre una colección y hacer aserción de tipo de cada valor:

```go
result, _ := fhirpath.Evaluate(patientJSON, "Patient.name.given")
for _, val := range result {
    if s, ok := val.(types.String); ok {
        fmt.Println("Given name:", s.Value())
    }
}
```

Para resultados de un solo valor, utilice `First()` o `Single()`:

```go
result, _ := fhirpath.Evaluate(patientJSON, "Patient.birthDate")
if val, ok := result.First(); ok {
    if d, ok := val.(types.Date); ok {
        fmt.Println("Birth year:", d.Year())
    }
}
```

O utilice las funciones de conveniencia para los patrones más comunes:

```go
// These handle collection unwrapping for you
family, _ := fhirpath.EvaluateToString(patientJSON, "Patient.name.first().family")
active, _ := fhirpath.EvaluateToBoolean(patientJSON, "Patient.active")
names, _  := fhirpath.EvaluateToStrings(patientJSON, "Patient.name.given")
exists, _ := fhirpath.Exists(patientJSON, "Patient.telecom")
count, _  := fhirpath.Count(patientJSON, "Patient.name")
```
