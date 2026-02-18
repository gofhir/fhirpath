---
title: "Referencia de Funciones"
linkTitle: "Referencia de Funciones"
weight: 4
description: >
  Referencia completa de todas las funciones FHIRPath soportadas por la biblioteca Go FHIRPath.
---

La especificacion FHIRPath define un amplio conjunto de funciones para navegar, filtrar y transformar datos FHIR®. Esta biblioteca implementa el catalogo completo de funciones de la [especificacion FHIRPath 2.0](http://hl7.org/fhirpath/), junto con extensiones especificas de FHIR®.

Las funciones se invocan utilizando notacion de punto sobre una coleccion:

```
Patient.name.where(use = 'official').first().given
```

Todas las funciones operan sobre colecciones y devuelven colecciones, siguiendo el modelo consistente basado en colecciones de FHIRPath. Cuando una funcion se invoca sobre una coleccion vacia, tipicamente devuelve una coleccion vacia (propagando el vacio).

## Categorias de Funciones

| Categoria | Funciones | Descripcion |
|----------|-----------|-------------|
| [Funciones de Cadena]({{< relref "strings" >}}) | 16 | Manipulacion de texto: `startsWith`, `contains`, `replace`, `matches`, `substring`, `lower`, `upper`, `split`, `join`, y mas |
| [Funciones Matematicas]({{< relref "math" >}}) | 10 | Operaciones numericas: `abs`, `ceiling`, `floor`, `round`, `sqrt`, `power`, `ln`, `log`, `exp`, `truncate` |
| [Funciones de Existencia]({{< relref "existence" >}}) | 12 | Pruebas de colecciones: `empty`, `exists`, `all`, `count`, `distinct`, `allTrue`, `anyTrue`, `subsetOf`, `supersetOf`, y mas |
| [Funciones de Filtrado]({{< relref "filtering" >}}) | 4 | Filtrado de colecciones: `where`, `select`, `repeat`, `ofType` |
| [Funciones de Subconjunto]({{< relref "subsetting" >}}) | 8 | Segmentacion de colecciones: `first`, `last`, `tail`, `take`, `skip`, `single`, `intersect`, `exclude` |
| [Funciones de Combinacion]({{< relref "combining" >}}) | 2 | Fusion de colecciones: `union`, `combine` |
| [Funciones de Conversion]({{< relref "conversion" >}}) | 17 | Conversion de tipos: `iif`, `toBoolean`, `toInteger`, `toDecimal`, `toString`, `toDate`, `toDateTime`, `toTime`, `toQuantity`, y variantes `convertsTo*` |
| [Funciones de Verificacion de Tipos]({{< relref "type-checking" >}}) | 3 | Inspeccion de tipos: `is`, `as`, `ofType` |
| [Funciones Temporales]({{< relref "temporal" >}}) | 10 | Operaciones de fecha/hora: `now`, `today`, `timeOfDay`, `year`, `month`, `day`, `hour`, `minute`, `second`, `millisecond` |
| [Funciones de Utilidad]({{< relref "utility" >}}) | 3 | Depuracion y navegacion: `trace`, `children`, `descendants` |
| [Funciones Especificas de FHIR®]({{< relref "fhir-specific" >}}) | 8 | Extensiones FHIR®: `extension`, `hasExtension`, `resolve`, `memberOf`, `conformsTo`, `hasValue`, `getValue`, `getReferenceKey` |
| [Funciones de Agregacion]({{< relref "aggregate" >}}) | 5 | Operaciones de reduccion: `aggregate`, `sum`, `avg`, `min`, `max` |

## Patrones Comunes

### Propagacion de Coleccion Vacia

La mayoria de las funciones devuelven una coleccion vacia cuando se invocan sobre una entrada vacia:

```go
result, _ := fhirpath.Evaluate(resource, "Patient.deceased.startsWith('abc')")
// Si Patient.deceased esta ausente, result esta vacio -- no es un error
```

### Evaluacion Singleton

Las funciones que esperan un valor unico (como las funciones de cadena) operan sobre el primer elemento de la coleccion de entrada. Si la coleccion contiene mas de un elemento, algunas funciones pueden devolver un error u operar solo sobre el primer elemento.

### Seguridad de Tipos

La biblioteca realiza verificacion de tipos en tiempo de ejecucion. Si una funcion recibe una entrada de un tipo inesperado, devuelve una coleccion vacia en lugar de generar un error, de manera consistente con la especificacion FHIRPath.
