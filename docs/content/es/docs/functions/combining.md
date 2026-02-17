---
title: "Funciones de Combinacion"
linkTitle: "Funciones de Combinacion"
weight: 6
description: >
  Funciones para fusionar dos colecciones en expresiones FHIRPath.
---

Las funciones de combinacion permiten fusionar dos colecciones. La diferencia clave entre las dos funciones disponibles es como manejan los duplicados: `union` produce un conjunto (sin duplicados), mientras que `combine` preserva todos los elementos incluyendo duplicados.

---

## union

Devuelve la union de conjuntos de la coleccion de entrada y otra coleccion. Los elementos duplicados se eliminan del resultado.

**Firma:**

```text
union(other : Collection) : Collection
```

**Parametros:**

| Nombre    | Tipo           | Descripcion                          |
|-----------|----------------|--------------------------------------|
| `other`   | `Collection`   | La coleccion con la cual fusionar    |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).union(3 | 4 | 5)")
// { 1, 2, 3, 4, 5 } (duplicates removed)

result, _ := fhirpath.Evaluate(resource, "('a' | 'b').union('c' | 'd')")
// { 'a', 'b', 'c', 'd' }

result, _ := fhirpath.Evaluate(resource, "(1 | 2).union(1 | 2)")
// { 1, 2 } (identical sets)
```

**Casos Limite / Notas:**

- El resultado es un conjunto sin elementos duplicados.
- La igualdad de elementos se determina por las reglas de igualdad de FHIRPath.
- El operador `|` en FHIRPath es equivalente a llamar a `union`. Por ejemplo, `a | b` es lo mismo que `a.union(b)`.
- Si cualquiera de las colecciones esta vacia, el resultado es la otra coleccion (con duplicados eliminados).
- El orden de los elementos en el resultado depende de la implementacion.
- Utiliza el metodo `Collection.Union` internamente, que maneja la deduplicacion.

---

## combine

Devuelve la concatenacion de la coleccion de entrada y otra coleccion. A diferencia de `union`, los duplicados se preservan.

**Firma:**

```text
combine(other : Collection) : Collection
```

**Parametros:**

| Nombre    | Tipo           | Descripcion                             |
|-----------|----------------|-----------------------------------------|
| `other`   | `Collection`   | La coleccion con la cual concatenar     |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).combine(3 | 4 | 5)")
// { 1, 2, 3, 3, 4, 5 } (duplicates preserved)

result, _ := fhirpath.Evaluate(resource, "('a' | 'b').combine('b' | 'c')")
// { 'a', 'b', 'b', 'c' }

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().given.combine(Patient.name.last().given)")
// Combines given names from first and last name entries
```

**Casos Limite / Notas:**

- A diferencia de `union`, `combine` **no** elimina duplicados. Es una concatenacion simple.
- El orden de los elementos se preserva: todos los elementos de la entrada vienen primero, seguidos por todos los elementos de la otra coleccion.
- Si cualquiera de las colecciones esta vacia, el resultado es la otra coleccion.
- Use `combine` cuando necesite preservar duplicados (por ejemplo, para contar o agregar). Use `union` cuando necesite semantica de conjuntos.

---

## Comparacion: union vs. combine

| Caracteristica    | `union`                              | `combine`                        |
|-------------------|--------------------------------------|----------------------------------|
| Duplicados        | Eliminados                           | Preservados                      |
| Semantica de conjuntos | Si                              | No                               |
| Operador equivalente | `\|`                              | Ninguno                          |
| Caso de uso       | Operaciones de conjuntos, deduplicacion | Concatenacion, agregacion     |

**Ejemplo que ilustra la diferencia:**

```go
// Given collections: {1, 2, 3} and {2, 3, 4}

// union removes duplicates
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).union(2 | 3 | 4)")
// { 1, 2, 3, 4 }

// combine preserves duplicates
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3).combine(2 | 3 | 4)")
// { 1, 2, 3, 2, 3, 4 }
```
