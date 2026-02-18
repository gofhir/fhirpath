---
title: "Funciones de Subconjunto"
linkTitle: "Funciones de Subconjunto"
weight: 5
description: >
  Funciones para extraer subconjuntos de elementos de colecciones en expresiones FHIRPath.
---

Las funciones de subconjunto permiten seleccionar elementos especificos o rangos de elementos de una coleccion. Son esenciales para navegar datos FHIR® ordenados como entradas de nombre, contactos de telecomunicacion o recursos de lista.

---

## first

Devuelve una coleccion que contiene solo el primer elemento de la coleccion de entrada.

**Firma:**

```text
first() : Collection
```

**Tipo de Retorno:** `Collection` (que contiene como maximo un elemento)

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.first()")
// Returns the first name entry

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family")
// Returns the family name from the first name entry

result, _ := fhirpath.Evaluate(resource, "{}.first()")
// { } (empty collection)
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia.
- Equivalente a `take(1)`.
- El resultado sigue siendo una coleccion (con cero o un elemento), no un escalar.

---

## last

Devuelve una coleccion que contiene solo el ultimo elemento de la coleccion de entrada.

**Firma:**

```text
last() : Collection
```

**Tipo de Retorno:** `Collection` (que contiene como maximo un elemento)

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.last()")
// Returns the last name entry

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.last().value")
// Returns the value from the last telecom entry

result, _ := fhirpath.Evaluate(resource, "{}.last()")
// { } (empty collection)
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia.
- El resultado sigue siendo una coleccion (con cero o un elemento), no un escalar.

---

## tail

Devuelve todos los elementos excepto el primero. Equivalente a `skip(1)`.

**Firma:**

```text
tail() : Collection
```

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.tail()")
// Returns all name entries except the first

result, _ := fhirpath.Evaluate(patient, "Patient.name.tail().count()")
// Number of names minus 1

result, _ := fhirpath.Evaluate(resource, "{}.tail()")
// { } (empty collection)
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada tiene cero o un elemento.
- Equivalente a `skip(1)`.

---

## take

Devuelve los primeros `n` elementos de la coleccion de entrada.

**Firma:**

```text
take(num : Integer) : Collection
```

**Parametros:**

| Nombre  | Tipo      | Descripcion                                                 |
|---------|-----------|-------------------------------------------------------------|
| `num`   | `Integer` | El numero de elementos a tomar desde el inicio              |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.take(2)")
// Returns the first 2 name entries

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.take(1)")
// Equivalent to first()

result, _ := fhirpath.Evaluate(patient, "Patient.name.take(100)")
// Returns all names (if fewer than 100 exist)
```

**Casos Limite / Notas:**

- Si `n` es mayor que el tamano de la coleccion, se devuelven todos los elementos.
- Si `n` es cero o negativo, devuelve una coleccion vacia.
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## skip

Devuelve todos los elementos excepto los primeros `n` elementos.

**Firma:**

```text
skip(num : Integer) : Collection
```

**Parametros:**

| Nombre  | Tipo      | Descripcion                                                   |
|---------|-----------|---------------------------------------------------------------|
| `num`   | `Integer` | El numero de elementos a omitir desde el inicio               |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.skip(1)")
// Equivalent to tail() - skips the first name

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.skip(2)")
// Returns all telecom entries after the second

result, _ := fhirpath.Evaluate(patient, "Patient.name.skip(0)")
// Returns all names (skip nothing)
```

**Casos Limite / Notas:**

- Si `n` es mayor o igual al tamano de la coleccion, devuelve una coleccion vacia.
- Si `n` es cero o negativo, se devuelven todos los elementos.
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## single

Devuelve el unico elemento de la coleccion de entrada. Si la coleccion no contiene exactamente un elemento, se genera un error.

**Firma:**

```text
single() : Collection
```

**Tipo de Retorno:** `Collection` (que contiene exactamente un elemento)

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.single()")
// Returns the birth date (exactly one)

result, _ := fhirpath.Evaluate(patient, "Patient.active.single()")
// Returns the active flag (exactly one)

result, err := fhirpath.Evaluate(patient, "Patient.name.single()")
// Error if patient has more than one name entry
```

**Casos Limite / Notas:**

- Devuelve un error de tipo `ErrSingletonExpected` si la coleccion contiene cero o mas de un elemento.
- Use esta funcion cuando espera exactamente un resultado y desea imponer esa restriccion.
- Es mas estricta que `first()`, que silenciosamente devuelve vacio o el primer elemento.

---

## intersect

Devuelve la interseccion de conjuntos de la coleccion de entrada y otra coleccion -- elementos que aparecen en ambas.

**Firma:**

```text
intersect(other : Collection) : Collection
```

**Parametros:**

| Nombre    | Tipo           | Descripcion                                   |
|-----------|----------------|-----------------------------------------------|
| `other`   | `Collection`   | La coleccion con la cual intersectar          |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).intersect(2 | 3 | 4)")
// { 2, 3 }

result, _ := fhirpath.Evaluate(resource, "(1 | 2).intersect(3 | 4)")
// { } (no common elements)

result, _ := fhirpath.Evaluate(resource, "('a' | 'b' | 'c').intersect('b' | 'd')")
// { 'b' }
```

**Casos Limite / Notas:**

- El resultado no contiene duplicados (semantica de conjuntos).
- La igualdad de elementos se determina por las reglas de igualdad de FHIRPath.
- Devuelve una coleccion vacia si cualquiera de las entradas esta vacia o no hay elementos comunes.

---

## exclude

Devuelve los elementos de la coleccion de entrada que **no** estan en la otra coleccion.

**Firma:**

```text
exclude(other : Collection) : Collection
```

**Parametros:**

| Nombre    | Tipo           | Descripcion                                    |
|-----------|----------------|------------------------------------------------|
| `other`   | `Collection`   | La coleccion de elementos a excluir            |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).exclude(2)")
// { 1, 3 }

result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).exclude(4 | 5)")
// { 1, 2, 3 } (nothing to exclude)

result, _ := fhirpath.Evaluate(resource, "('a' | 'b' | 'c').exclude('a' | 'c')")
// { 'b' }
```

**Casos Limite / Notas:**

- Esta es la operacion de diferencia de conjuntos: `entrada - otra`.
- La igualdad de elementos se determina por las reglas de igualdad de FHIRPath.
- Si la otra coleccion esta vacia, la entrada se devuelve sin cambios.
- Devuelve una coleccion vacia si la entrada esta vacia.
