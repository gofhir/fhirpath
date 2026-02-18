---
title: "Funciones de Filtrado"
linkTitle: "Funciones de Filtrado"
weight: 4
description: >
  Funciones para filtrar, proyectar y navegar recursivamente colecciones en expresiones FHIRPath.
---

Las funciones de filtrado permiten reducir colecciones basandose en criterios, proyectar elementos para extraer propiedades especificas y navegar recursivamente a traves de estructuras de recursos. Estas se encuentran entre las funciones mas comunmente utilizadas en FHIRPath.

---

## where

Filtra la coleccion de entrada, devolviendo solo los elementos donde la expresion de criterio se evalua como `true`.

**Firma:**

```text
where(criteria : Expression) : Collection
```

**Parametros:**

| Nombre       | Tipo         | Descripcion                                                                                                            |
|--------------|--------------|------------------------------------------------------------------------------------------------------------------------|
| `criteria`   | `Expression` | Una expresion booleana evaluada para cada elemento. Dentro de la expresion, `$this` se refiere al elemento actual       |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.where(use = 'official')")
// Returns only name entries where use is 'official'

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.where(system = 'phone' and use = 'home')")
// Returns telecom entries that are home phone numbers

result, _ := fhirpath.Evaluate(patient, "Patient.name.where(given.exists())")
// Returns name entries that have at least one given name
```

**Casos Limite / Notas:**

- La expresion de criterio se evalua con `$this` asignado a cada elemento de la coleccion de entrada.
- Si el criterio se evalua como una coleccion vacia o `false` para un elemento, ese elemento se excluye.
- Una coleccion de entrada vacia devuelve una coleccion vacia.
- A diferencia de `select`, `where` preserva los elementos originales -- no los transforma.

---

## select

Proyecta cada elemento de la coleccion de entrada a traves de una expresion, devolviendo los resultados aplanados.

**Firma:**

```text
select(projection : Expression) : Collection
```

**Parametros:**

| Nombre         | Tipo         | Descripcion                                                                                                      |
|----------------|--------------|------------------------------------------------------------------------------------------------------------------|
| `projection`   | `Expression` | Una expresion evaluada para cada elemento. Dentro de la expresion, `$this` se refiere al elemento actual          |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.telecom.select(value)")
// Returns just the values from each telecom entry

result, _ := fhirpath.Evaluate(patient, "Patient.name.select(given)")
// Returns all given names (flattened from all name entries)

result, _ := fhirpath.Evaluate(patient, "Patient.name.select(family + ', ' + given.first())")
// Returns formatted names like "Smith, John"
```

**Casos Limite / Notas:**

- Los resultados de todos los elementos se **aplanan** en una sola coleccion. Si cada elemento produce una coleccion, todos los items se fusionan en una sola.
- Una coleccion de entrada vacia devuelve una coleccion vacia.
- `select` transforma elementos, mientras que `where` los filtra. Use `select` para extraer o calcular valores.
- La expresion de proyeccion se evalua con `$this` asignado a cada elemento.

---

## repeat

Aplica repetidamente una expresion a la coleccion de entrada y sus resultados, recopilando todos los resultados hasta que no se producen nuevos elementos. Esto permite la navegacion recursiva a traves de datos jerarquicos.

**Firma:**

```text
repeat(expression : Expression) : Collection
```

**Parametros:**

| Nombre         | Tipo         | Descripcion                                          |
|----------------|--------------|------------------------------------------------------|
| `expression`   | `Expression` | Una expresion que se aplica recursivamente            |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "QuestionnaireResponse.item.repeat(item)")
// Recursively collects all nested items at every level

result, _ := fhirpath.Evaluate(resource, "Observation.component.repeat(component)")
// Recursively navigates nested components

result, _ := fhirpath.Evaluate(resource, "ValueSet.expansion.contains.repeat(contains)")
// Navigates the full hierarchy of a ValueSet expansion
```

**Casos Limite / Notas:**

- La expresion se aplica a la entrada, luego a los resultados, y asi sucesivamente hasta que no se producen nuevos elementos.
- La deteccion de duplicados previene bucles infinitos en estructuras ciclicas.
- Una coleccion de entrada vacia devuelve una coleccion vacia.
- Los resultados incluyen todos los resultados intermedios, no solo el nivel final.
- Esta funcion requiere un manejo especial en el evaluador para una evaluacion recursiva adecuada.

---

## ofType

Filtra la coleccion de entrada, devolviendo solo los elementos que son del tipo especificado.

**Firma:**

```text
ofType(type : TypeSpecifier) : Collection
```

**Parametros:**

| Nombre   | Tipo              | Descripcion                                                                          |
|----------|-------------------|--------------------------------------------------------------------------------------|
| `type`   | `TypeSpecifier`   | El nombre del tipo FHIR® para filtrar (por ejemplo, `Quantity`, `String`, `HumanName`) |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(observation, "Observation.value.ofType(Quantity)")
// Returns value only if it is a Quantity type

result, _ := fhirpath.Evaluate(observation, "Observation.value.ofType(CodeableConcept)")
// Returns value only if it is a CodeableConcept type

result, _ := fhirpath.Evaluate(resource, "Bundle.entry.resource.ofType(Patient)")
// Returns only Patient resources from a Bundle
```

**Casos Limite / Notas:**

- Esta funcion es particularmente util para elementos FHIR® polimorficos (por ejemplo, `value[x]`).
- La coincidencia de tipos compara el tipo en tiempo de ejecucion del elemento contra el nombre de tipo especificado.
- Una coleccion de entrada vacia devuelve una coleccion vacia.
- Esta funcion tambien esta documentada en [Funciones de Verificacion de Tipos]({{< relref "type-checking" >}}) ya que cumple un doble proposito.
- A diferencia de `as()`, `ofType()` funciona con colecciones de multiples elementos y nunca genera errores.
