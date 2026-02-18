---
title: "Funciones de Verificacion de Tipos"
linkTitle: "Funciones de Verificacion de Tipos"
weight: 8
description: >
  Funciones para inspeccionar y convertir tipos de elementos en expresiones FHIRPath.
---

Las funciones de verificacion de tipos permiten probar el tipo de un elemento y convertir elementos a tipos especificos. Estas son esenciales cuando se trabaja con elementos FHIR® polimorficos (como `value[x]`) donde el tipo real puede variar.

---

## is

Devuelve `true` si el elemento de entrada es del tipo especificado.

**Firma:**

```text
is(type : TypeSpecifier) : Boolean
```

**Parametros:**

| Nombre   | Tipo              | Descripcion                                                                               |
|----------|-------------------|-------------------------------------------------------------------------------------------|
| `type`   | `TypeSpecifier`   | El nombre del tipo FHIR® contra el cual verificar (por ejemplo, `Quantity`, `String`, `Patient`) |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(observation, "Observation.value.is(Quantity)")
// true if value is a Quantity

result, _ := fhirpath.Evaluate(observation, "Observation.value.is(CodeableConcept)")
// true if value is a CodeableConcept

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().is(HumanName)")
// true
```

**Casos Limite / Notas:**

- Requiere una entrada singleton (exactamente un elemento). Si la entrada contiene mas de un elemento, se genera un error.
- Devuelve una coleccion vacia si la entrada esta vacia.
- El nombre del tipo es resuelto por el evaluador directamente desde el AST de la expresion, por lo que nombres de tipo como `Patient` o `Quantity` se usan sin comillas.
- La coincidencia de tipos utiliza la funcion `eval.TypeMatches`, que soporta tanto nombres de tipo simples como nombres de tipo FHIR® completamente calificados.
- La forma de funcion `value.is(Quantity)` es equivalente a la forma de operador `value is Quantity`.

---

## as

Convierte la entrada al tipo especificado. Devuelve la entrada si coincide con el tipo, en caso contrario devuelve una coleccion vacia.

**Firma:**

```text
as(type : TypeSpecifier) : Collection
```

**Parametros:**

| Nombre   | Tipo              | Descripcion                                                                               |
|----------|-------------------|-------------------------------------------------------------------------------------------|
| `type`   | `TypeSpecifier`   | El nombre del tipo FHIR® al cual convertir (por ejemplo, `Quantity`, `String`, `Patient`)   |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(observation, "Observation.value.as(Quantity)")
// Returns the value as a Quantity, or empty if not a Quantity

result, _ := fhirpath.Evaluate(observation, "Observation.value.as(Quantity).value")
// Accesses the numeric value if value[x] is a Quantity

result, _ := fhirpath.Evaluate(resource, "Bundle.entry.resource.as(Patient)")
// Returns only entries that are Patient resources
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia.
- Devuelve una coleccion vacia si ninguno de los elementos coincide con el tipo especificado.
- A diferencia de `is()`, la funcion `as()` funciona con colecciones de multiples elementos -- filtra y devuelve solo los elementos coincidentes.
- La forma de funcion `value.as(Quantity)` es equivalente a la forma de operador `value as Quantity`.
- El nombre del tipo tipicamente es manejado de manera especial por el evaluador, extrayendolo directamente del AST.

---

## ofType

Filtra la coleccion de entrada, devolviendo solo los elementos que son del tipo especificado. Esta funcion es identica en comportamiento a `as()` pero es la forma preferida cuando se filtran colecciones.

**Firma:**

```text
ofType(type : TypeSpecifier) : Collection
```

**Parametros:**

| Nombre   | Tipo              | Descripcion                                                                                 |
|----------|-------------------|---------------------------------------------------------------------------------------------|
| `type`   | `TypeSpecifier`   | El nombre del tipo FHIR® para filtrar (por ejemplo, `Quantity`, `String`, `HumanName`)        |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(observation, "Observation.value.ofType(Quantity)")
// Returns value only if it is a Quantity

result, _ := fhirpath.Evaluate(resource, "Bundle.entry.resource.ofType(Patient)")
// Returns only Patient resources from a Bundle

result, _ := fhirpath.Evaluate(resource, "Bundle.entry.resource.ofType(Observation).status")
// Gets status from all Observation resources in a Bundle
```

**Casos Limite / Notas:**

- Esta funcion tambien esta documentada en [Funciones de Filtrado]({{< relref "filtering" >}}) ya que filtra colecciones por tipo.
- La coincidencia de tipos compara el nombre del tipo en tiempo de ejecucion del elemento contra el nombre de tipo especificado.
- Devuelve una coleccion vacia si ningun elemento coincide con el tipo.
- A diferencia de `is()`, `ofType()` funciona con colecciones de multiples elementos y nunca genera errores en entradas no singleton.

---

## Comparacion: is vs. as vs. ofType

| Funcion       | Entrada      | Devuelve       | Caso de Uso                                                   |
|---------------|--------------|----------------|---------------------------------------------------------------|
| `is(T)`       | Singleton    | `Boolean`      | Verificar si un valor unico es de un tipo especifico          |
| `as(T)`       | Collection   | `Collection`   | Convertir / filtrar una coleccion a un tipo                   |
| `ofType(T)`   | Collection   | `Collection`   | Filtrar una coleccion a elementos de un tipo                  |

**Ejemplo que ilustra las diferencias:**

```go
// is() -- returns a boolean
result, _ := fhirpath.Evaluate(observation, "Observation.value.is(Quantity)")
// true or false

// as() -- returns the value if it matches, empty otherwise
result, _ := fhirpath.Evaluate(observation, "Observation.value.as(Quantity)")
// The Quantity object, or empty

// ofType() -- filters multiple elements by type
result, _ := fhirpath.Evaluate(resource, "Bundle.entry.resource.ofType(Patient)")
// All Patient resources from the Bundle
```

En la practica, `as()` y `ofType()` se comportan de manera identica en esta implementacion -- ambas filtran elementos por tipo. La especificacion FHIRPath recomienda usar `ofType()` cuando se filtran colecciones y `as()` cuando se convierte un valor unico.
