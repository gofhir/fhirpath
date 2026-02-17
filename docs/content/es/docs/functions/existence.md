---
title: "Funciones de Existencia"
linkTitle: "Funciones de Existencia"
weight: 3
description: >
  Funciones para verificar la existencia y propiedades de elementos dentro de colecciones.
---

Las funciones de existencia permiten verificar si las colecciones contienen elementos, si esos elementos cumplen ciertos criterios, y obtener valores distintos. Estas son fundamentales para las expresiones FHIRPath y se utilizan extensamente en la validacion y extraccion de datos FHIR.

---

## empty

Devuelve `true` si la coleccion de entrada esta vacia, `false` en caso contrario.

**Firma:**

```text
empty() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.empty()")
// false (patient has at least one name)

result, _ := fhirpath.Evaluate(patient, "Patient.contact.empty()")
// true (if patient has no contacts)

result, _ := fhirpath.Evaluate(resource, "{}.empty()")
// true
```

**Casos Limite / Notas:**

- Siempre devuelve `true` o `false`, nunca una coleccion vacia.
- Esta es la unica funcion de existencia que garantiza devolver un booleano incluso para una entrada vacia.

---

## exists

Devuelve `true` si la coleccion de entrada contiene algun elemento. Con una expresion de criterio opcional, devuelve `true` si algun elemento satisface el criterio.

**Firma:**

```text
exists([criteria : Expression]) : Boolean
```

**Parametros:**

| Nombre       | Tipo         | Descripcion                                                             |
|--------------|--------------|-------------------------------------------------------------------------|
| `criteria`   | `Expression` | (Opcional) Una expresion de filtro evaluada para cada elemento           |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.exists()")
// true (patient has at least one name)

result, _ := fhirpath.Evaluate(patient, "Patient.name.exists(use = 'official')")
// true if any name has use = 'official'

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.exists(system = 'phone')")
// true if patient has a phone telecom entry
```

**Casos Limite / Notas:**

- Sin criterio, `exists()` es el inverso de `empty()`.
- Con criterio, es equivalente a `where(criterio).exists()`.
- Devuelve `false` para una coleccion de entrada vacia.
- La expresion de criterio se evalua con `$this` asignado a cada elemento.

---

## all

Devuelve `true` si **todos** los elementos de la coleccion satisfacen la expresion de criterio dada. Devuelve `true` para una coleccion vacia (verdad vacua).

**Firma:**

```text
all(criteria : Expression) : Boolean
```

**Parametros:**

| Nombre       | Tipo         | Descripcion                                                                |
|--------------|--------------|----------------------------------------------------------------------------|
| `criteria`   | `Expression` | Una expresion de filtro que debe ser verdadera para cada elemento           |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.all(use.exists())")
// true if every name entry has a 'use' field

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.all(system = 'phone')")
// true only if every telecom entry is a phone

result, _ := fhirpath.Evaluate(patient, "Patient.contact.all(name.exists())")
// true if patient has no contacts (vacuous truth)
```

**Casos Limite / Notas:**

- Una coleccion vacia devuelve `true` (verdad vacua segun la especificacion FHIRPath).
- La expresion de criterio se evalua con `$this` asignado a cada elemento.
- Esta funcion se usa comunmente en definiciones de invariantes FHIR.

---

## allTrue

Devuelve `true` si todos los elementos de la coleccion son booleanos `true`.

**Firma:**

```text
allTrue() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "Patient.active.allTrue()")
// true if active is true

result, _ := fhirpath.Evaluate(resource, "(true | true | true).allTrue()")
// true

result, _ := fhirpath.Evaluate(resource, "(true | false | true).allTrue()")
// false
```

**Casos Limite / Notas:**

- Una coleccion vacia devuelve `true` (verdad vacua).
- Los elementos no booleanos hacen que la funcion devuelva `false`.
- Se usa comunmente despues de mapear una coleccion a valores booleanos.

---

## anyTrue

Devuelve `true` si **algun** elemento de la coleccion es booleano `true`.

**Firma:**

```text
anyTrue() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(true | false | false).anyTrue()")
// true

result, _ := fhirpath.Evaluate(resource, "(false | false).anyTrue()")
// false

result, _ := fhirpath.Evaluate(resource, "{}.anyTrue()")
// false
```

**Casos Limite / Notas:**

- Una coleccion vacia devuelve `false`.
- Devuelve `true` tan pronto como se encuentra un booleano `true`.

---

## allFalse

Devuelve `true` si todos los elementos de la coleccion son booleanos `false`.

**Firma:**

```text
allFalse() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(false | false | false).allFalse()")
// true

result, _ := fhirpath.Evaluate(resource, "(false | true | false).allFalse()")
// false

result, _ := fhirpath.Evaluate(resource, "{}.allFalse()")
// true (vacuous truth)
```

**Casos Limite / Notas:**

- Una coleccion vacia devuelve `true` (verdad vacua).
- Los elementos no booleanos hacen que la funcion devuelva `false`.

---

## anyFalse

Devuelve `true` si **algun** elemento de la coleccion es booleano `false`.

**Firma:**

```text
anyFalse() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(true | false | true).anyFalse()")
// true

result, _ := fhirpath.Evaluate(resource, "(true | true).anyFalse()")
// false

result, _ := fhirpath.Evaluate(resource, "{}.anyFalse()")
// false
```

**Casos Limite / Notas:**

- Una coleccion vacia devuelve `false`.
- Devuelve `true` tan pronto como se encuentra un booleano `false`.

---

## count

Devuelve el numero de elementos en la coleccion de entrada.

**Firma:**

```text
count() : Integer
```

**Tipo de Retorno:** `Integer`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.count()")
// Number of name entries (e.g., 2)

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.count()")
// Number of telecom entries

result, _ := fhirpath.Evaluate(resource, "{}.count()")
// 0
```

**Casos Limite / Notas:**

- Siempre devuelve un entero no negativo, nunca una coleccion vacia.
- Una coleccion vacia devuelve `0`.

---

## distinct

Devuelve una coleccion que contiene solo los elementos distintos (unicos) de la entrada.

**Firma:**

```text
distinct() : Collection
```

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 2 | 3 | 3 | 3).distinct()")
// { 1, 2, 3 }

result, _ := fhirpath.Evaluate(resource, "('a' | 'b' | 'a').distinct()")
// { 'a', 'b' }

result, _ := fhirpath.Evaluate(resource, "{}.distinct()")
// { } (empty)
```

**Casos Limite / Notas:**

- El orden de los elementos en el resultado depende de la implementacion.
- La igualdad de elementos se determina por las reglas de igualdad de FHIRPath.
- Una coleccion vacia devuelve una coleccion vacia.

---

## isDistinct

Devuelve `true` si todos los elementos de la coleccion son distintos (sin duplicados).

**Firma:**

```text
isDistinct() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).isDistinct()")
// true

result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 2).isDistinct()")
// false

result, _ := fhirpath.Evaluate(resource, "{}.isDistinct()")
// true (empty collection is trivially distinct)
```

**Casos Limite / Notas:**

- Equivalente a `count() = distinct().count()`.
- Una coleccion vacia devuelve `true`.

---

## subsetOf

Devuelve `true` si todos los elementos de la coleccion de entrada tambien estan presentes en la otra coleccion.

**Firma:**

```text
subsetOf(other : Collection) : Boolean
```

**Parametros:**

| Nombre    | Tipo           | Descripcion                                    |
|-----------|----------------|------------------------------------------------|
| `other`   | `Collection`   | La coleccion contra la cual verificar           |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2).subsetOf(1 | 2 | 3)")
// true

result, _ := fhirpath.Evaluate(resource, "(1 | 4).subsetOf(1 | 2 | 3)")
// false

result, _ := fhirpath.Evaluate(resource, "{}.subsetOf(1 | 2)")
// true (empty set is a subset of any set)
```

**Casos Limite / Notas:**

- Una coleccion de entrada vacia siempre es subconjunto de cualquier coleccion.
- La comparacion de elementos sigue las reglas de igualdad de FHIRPath.

---

## supersetOf

Devuelve `true` si todos los elementos de la otra coleccion tambien estan presentes en la coleccion de entrada.

**Firma:**

```text
supersetOf(other : Collection) : Boolean
```

**Parametros:**

| Nombre    | Tipo           | Descripcion                                    |
|-----------|----------------|------------------------------------------------|
| `other`   | `Collection`   | La coleccion contra la cual verificar           |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).supersetOf(1 | 2)")
// true

result, _ := fhirpath.Evaluate(resource, "(1 | 2).supersetOf(1 | 2 | 3)")
// false

result, _ := fhirpath.Evaluate(resource, "(1 | 2).supersetOf({})")
// true (any set is a superset of the empty set)
```

**Casos Limite / Notas:**

- `a.supersetOf(b)` es equivalente a `b.subsetOf(a)`.
- Cualquier coleccion es superconjunto de la coleccion vacia.
