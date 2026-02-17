---
title: "Funciones de Utilidad"
linkTitle: "Funciones de Utilidad"
weight: 10
description: >
  Funciones para depuracion, registro y navegacion del arbol de elementos en expresiones FHIRPath.
---

Las funciones de utilidad ayudan con la depuracion de expresiones FHIRPath y la navegacion de la estructura jerarquica de los recursos FHIR. La funcion `trace` proporciona observabilidad durante la evaluacion de expresiones, mientras que `children` y `descendants` permiten el recorrido del arbol.

---

## trace

Registra la coleccion de entrada con fines de depuracion y la devuelve sin cambios. Esta funcion es un paso transparente que agrega observabilidad sin afectar el resultado de la evaluacion.

**Firma:**

```text
trace(name : String [, projection : Expression]) : Collection
```

**Parametros:**

| Nombre         | Tipo         | Descripcion                                                                        |
|----------------|--------------|------------------------------------------------------------------------------------|
| `name`         | `String`     | Una etiqueta para identificar este punto de rastreo en la salida del registro      |
| `projection`   | `Expression` | (Opcional) Una expresion adicional a evaluar y registrar junto con la entrada       |

**Tipo de Retorno:** `Collection` (la coleccion de entrada, sin cambios)

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.trace('names').where(use = 'official')")
// Logs: [trace] names: { ... }
// Returns: the filtered names (trace does not alter the pipeline)

result, _ := fhirpath.Evaluate(patient, "Patient.telecom.trace('telecom').select(value)")
// Logs the telecom entries before selecting values

result, _ := fhirpath.Evaluate(patient, "Patient.name.trace('names', given)")
// Logs both the name entries and their 'given' projection
```

**Casos Limite / Notas:**

- La funcion **no** modifica el resultado. Es puramente un efecto secundario para registro.
- Por defecto, la salida de trace se escribe en `stderr` en formato de texto plano.
- El registrador de trace puede personalizarse usando `funcs.SetTraceLogger()`:
  - `funcs.NewDefaultTraceLogger(writer, false)` para salida en texto plano.
  - `funcs.NewDefaultTraceLogger(writer, true)` para salida estructurada en JSON.
  - `funcs.NullTraceLogger{}` para deshabilitar la salida de trace por completo (recomendado para produccion).
- Cada entrada de trace incluye una marca de tiempo, la etiqueta del nombre, la coleccion de entrada, el conteo y una proyeccion opcional.

### Configuracion del Registrador de Trace

```go
import "github.com/gofhir/fhirpath/funcs"

// JSON-structured logging to stdout
funcs.SetTraceLogger(funcs.NewDefaultTraceLogger(os.Stdout, true))

// Disable trace output in production
funcs.SetTraceLogger(funcs.NullTraceLogger{})

// Custom logger implementing the funcs.TraceLogger interface
funcs.SetTraceLogger(myCustomLogger)
```

---

## children

Devuelve todos los elementos hijos directos de cada elemento en la coleccion de entrada.

**Firma:**

```text
children() : Collection
```

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.children()")
// Returns all direct child elements: name, telecom, birthDate, gender, etc.

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().children()")
// Returns children of the first name: use, family, given, etc.

result, _ := fhirpath.Evaluate(patient, "Patient.children().count()")
// Number of direct child elements
```

**Casos Limite / Notas:**

- Solo funciona con tipos complejos (`ObjectValue`). Los valores primitivos (cadenas, enteros, etc.) no tienen hijos y no producen salida.
- Devuelve los valores de todos los campos del objeto, independientemente del nombre del campo.
- Una coleccion de entrada vacia devuelve una coleccion vacia.
- El orden de los hijos depende de la estructura del objeto subyacente.

---

## descendants

Devuelve todos los elementos descendientes de cada elemento en la coleccion de entrada, recursivamente. Esto incluye hijos, nietos, y asi sucesivamente en cada nivel de anidacion.

**Firma:**

```text
descendants() : Collection
```

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.descendants()")
// Returns ALL nested elements at every level of the Patient resource

result, _ := fhirpath.Evaluate(patient, "Patient.descendants().ofType(HumanName)")
// Finds all HumanName elements anywhere in the resource

result, _ := fhirpath.Evaluate(patient, "Patient.name.descendants()")
// Returns all nested elements within name entries
```

**Casos Limite / Notas:**

- Esta funcion recorre recursivamente todo el arbol de elementos bajo la entrada.
- La deteccion de ciclos esta incorporada: los elementos que ya han sido visitados se omiten para prevenir bucles infinitos.
- Solo los tipos complejos (`ObjectValue`) se recorren. Los valores primitivos se incluyen en el resultado pero no producen mas descendientes.
- Una coleccion de entrada vacia devuelve una coleccion vacia.
- Para recursos grandes, `descendants()` puede devolver una coleccion muy grande. Considere usar expresiones de ruta mas especificas cuando sea posible.
- El resultado incluye nodos intermedios, no solo nodos hoja. Se devuelven tanto descendientes complejos como primitivos.

---

## Patrones de Uso Practico

### Encontrar Todos los Elementos de un Tipo Especifico

```go
// Find all CodeableConcept elements anywhere in a resource
result, _ := fhirpath.Evaluate(resource, "Resource.descendants().ofType(CodeableConcept)")
```

### Depurar Expresiones Complejas

```go
// Trace intermediate values in a chain
result, _ := fhirpath.Evaluate(patient,
    "Patient.name.trace('all-names').where(use = 'official').trace('official-names').first().given")
```

### Explorar la Estructura de un Recurso

```go
// Count all nested elements
result, _ := fhirpath.Evaluate(patient, "Patient.descendants().count()")

// Get all direct child element names
result, _ := fhirpath.Evaluate(patient, "Patient.children()")
```
