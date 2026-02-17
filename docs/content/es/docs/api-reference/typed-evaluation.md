---
title: "Evaluación Tipada"
linkTitle: "Evaluación Tipada"
weight: 3
description: >
  Funciones de conveniencia que retornan tipos nativos de Go en lugar de Collections.
---

Las funciones de evaluación tipada envuelven `EvaluateCached` y convierten el resultado a un tipo específico de Go. Simplifican patrones comunes como verificar existencia, contar resultados o extraer un único valor de cadena o booleano.

Todas estas funciones utilizan el `DefaultCache` internamente, por lo que las llamadas repetidas con la misma expresión se benefician del caché automático.

## EvaluateToBoolean

Evalúa una expresión FHIRPath y retorna el resultado como un `bool` de Go. Retorna `false` si el resultado está vacío. Retorna un error si el resultado contiene más de un valor o si el valor único no es un Boolean.

```go
func EvaluateToBoolean(resource []byte, expr string) (bool, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `resource` | `[]byte` | Bytes JSON crudos de un recurso FHIR |
| `expr` | `string` | Una expresión FHIRPath que debería producir un único Boolean |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `bool` | El resultado booleano, o `false` si el resultado está vacío |
| `error` | No nulo en errores de compilación/evaluación, múltiples resultados o resultado no booleano |

**Ejemplo:**

```go
patient := []byte(`{
    "resourceType": "Patient",
    "active": true,
    "name": [{"family": "Smith"}]
}`)

// Check a boolean field
active, err := fhirpath.EvaluateToBoolean(patient, "Patient.active")
if err != nil {
    log.Fatal(err)
}
fmt.Println(active) // true

// Boolean expressions also work
hasName, err := fhirpath.EvaluateToBoolean(patient, "Patient.name.exists()")
if err != nil {
    log.Fatal(err)
}
fmt.Println(hasName) // true
```

---

## EvaluateToString

Evalúa una expresión FHIRPath y retorna el resultado como un `string` de Go. Retorna una cadena vacía si el resultado está vacío. Si el resultado único es un `types.String`, se retorna su valor crudo; de lo contrario, se utiliza la representación `String()` del valor. Retorna un error si el resultado contiene más de un valor.

```go
func EvaluateToString(resource []byte, expr string) (string, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `resource` | `[]byte` | Bytes JSON crudos de un recurso FHIR |
| `expr` | `string` | Una expresión FHIRPath que debería producir un único valor |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `string` | El resultado como cadena, o `""` si el resultado está vacío |
| `error` | No nulo en errores de compilación/evaluación, o si el resultado tiene más de un valor |

**Ejemplo:**

```go
patient := []byte(`{
    "resourceType": "Patient",
    "name": [{"family": "Johnson", "given": ["Alice"]}],
    "birthDate": "1985-03-22"
}`)

family, err := fhirpath.EvaluateToString(patient, "Patient.name.first().family")
if err != nil {
    log.Fatal(err)
}
fmt.Println(family) // Johnson

birthDate, err := fhirpath.EvaluateToString(patient, "Patient.birthDate")
if err != nil {
    log.Fatal(err)
}
fmt.Println(birthDate) // 1985-03-22
```

---

## EvaluateToStrings

Evalúa una expresión FHIRPath y retorna todos los resultados como un `[]string`. Cada elemento se convierte a su representación en cadena. A diferencia de `EvaluateToString`, esta función maneja colecciones de cualquier tamaño.

```go
func EvaluateToStrings(resource []byte, expr string) ([]string, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `resource` | `[]byte` | Bytes JSON crudos de un recurso FHIR |
| `expr` | `string` | Una expresión FHIRPath |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `[]string` | Todos los valores del resultado como cadenas |
| `error` | No nulo en errores de compilación o evaluación |

**Ejemplo:**

```go
patient := []byte(`{
    "resourceType": "Patient",
    "name": [
        {"family": "Williams", "given": ["Robert", "James"]},
        {"family": "Bill", "given": ["Bob"]}
    ]
}`)

// Get all given names across all name entries
names, err := fhirpath.EvaluateToStrings(patient, "Patient.name.given")
if err != nil {
    log.Fatal(err)
}
fmt.Println(names) // [Robert James Bob]
```

---

## Exists

Evalúa una expresión FHIRPath y retorna `true` si la colección de resultados no está vacía. Esto es equivalente a llamar `Evaluate` y verificar `!result.Empty()`, pero más conciso.

```go
func Exists(resource []byte, expr string) (bool, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `resource` | `[]byte` | Bytes JSON crudos de un recurso FHIR |
| `expr` | `string` | Una expresión FHIRPath |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `bool` | `true` si existe al menos un resultado |
| `error` | No nulo en errores de compilación o evaluación |

**Ejemplo:**

```go
patient := []byte(`{
    "resourceType": "Patient",
    "telecom": [
        {"system": "phone", "value": "555-0100"}
    ]
}`)

hasPhone, err := fhirpath.Exists(patient, "Patient.telecom.where(system = 'phone')")
if err != nil {
    log.Fatal(err)
}
fmt.Println(hasPhone) // true

hasEmail, err := fhirpath.Exists(patient, "Patient.telecom.where(system = 'email')")
if err != nil {
    log.Fatal(err)
}
fmt.Println(hasEmail) // false
```

---

## Count

Evalúa una expresión FHIRPath y retorna el número de valores en la colección de resultados.

```go
func Count(resource []byte, expr string) (int, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `resource` | `[]byte` | Bytes JSON crudos de un recurso FHIR |
| `expr` | `string` | Una expresión FHIRPath |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `int` | El número de valores del resultado |
| `error` | No nulo en errores de compilación o evaluación |

**Ejemplo:**

```go
patient := []byte(`{
    "resourceType": "Patient",
    "name": [
        {"family": "Smith", "given": ["John", "Jacob"]},
        {"family": "Doe"}
    ],
    "address": [
        {"city": "Springfield"},
        {"city": "Shelbyville"},
        {"city": "Capital City"}
    ]
}`)

nameCount, err := fhirpath.Count(patient, "Patient.name")
if err != nil {
    log.Fatal(err)
}
fmt.Println(nameCount) // 2

addressCount, err := fhirpath.Count(patient, "Patient.address")
if err != nil {
    log.Fatal(err)
}
fmt.Println(addressCount) // 3
```

---

## Resumen

| Función | Tipo de Retorno | Resultado Vacío | Múltiples Resultados | Caché |
|---------|-----------------|-----------------|----------------------|-------|
| `EvaluateToBoolean` | `bool` | `false` | Error | Sí |
| `EvaluateToString` | `string` | `""` | Error | Sí |
| `EvaluateToStrings` | `[]string` | `[]string{}` | Todos convertidos | Sí |
| `Exists` | `bool` | `false` | `true` | Sí |
| `Count` | `int` | `0` | Retorna la cuenta | Sí |

Todas las funciones utilizan `EvaluateCached` internamente, por lo que la primera llamada para una expresión dada incurre en el costo de compilación, y todas las llamadas posteriores se sirven desde el `DefaultCache`.

## Patrones Prácticos

### Validación con EvaluateToBoolean

```go
func validatePatient(resource []byte) error {
    // Check required fields
    hasName, err := fhirpath.EvaluateToBoolean(resource, "Patient.name.exists()")
    if err != nil {
        return fmt.Errorf("validation error: %w", err)
    }
    if !hasName {
        return fmt.Errorf("Patient must have at least one name")
    }
    return nil
}
```

### Extracción de Listas con EvaluateToStrings

```go
func getAllIdentifiers(resource []byte) ([]string, error) {
    return fhirpath.EvaluateToStrings(resource, "Patient.identifier.value")
}
```

### Lógica Condicional con Exists

```go
func isDeceased(resource []byte) (bool, error) {
    return fhirpath.Exists(resource, "Patient.deceased.where($this = true)")
}
```
