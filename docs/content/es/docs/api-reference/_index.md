---
title: "Referencia de la API"
linkTitle: "Referencia de la API"
weight: 3
description: >
  Referencia completa de la API publica de la biblioteca FHIRPath para Go.
---

El paquete `github.com/gofhir/fhirpath` proporciona un evaluador completo de expresiones FHIRPath 2.0 para recursos FHIR® en Go. Esta seccion documenta cada funcion, tipo e interfaz publica disponible en la biblioteca.

## Descripcion General del Paquete

La biblioteca esta organizada en dos paquetes:

| Paquete | Ruta de Importacion | Descripcion |
|---------|---------------------|-------------|
| **fhirpath** | `github.com/gofhir/fhirpath` | Motor de evaluacion principal, compilacion, cache y opciones |
| **types** | `github.com/gofhir/fhirpath/types` | Sistema de tipos FHIRPath: Value, Collection y todos los tipos primitivos |

## Navegacion Rapida

### Evaluacion Principal

- **[Funciones de Evaluacion](evaluate/)** -- `Evaluate`, `MustEvaluate` y `EvaluateCached` para la evaluacion directa de expresiones contra recursos JSON.
- **[Compilacion y Expression](compile/)** -- `Compile`, `MustCompile` y el tipo `Expression` para precompilar expresiones y evaluarlas multiples veces.
- **[Evaluacion Tipada](typed-evaluation/)** -- Funciones de conveniencia que retornan tipos nativos de Go: `EvaluateToBoolean`, `EvaluateToString`, `EvaluateToStrings`, `Exists` y `Count`.

### Manejo de Recursos

- **[Interfaz Resource](resource/)** -- La interfaz `Resource`, `EvaluateResource`, `EvaluateResourceCached` y `ResourceJSON` para evaluar structs de Go directamente.

### Rendimiento y Configuracion

- **[Cache de Expresiones](cache/)** -- `ExpressionCache` con desalojo LRU, `DefaultCache`, estadisticas de cache y monitoreo.
- **[Opciones de Evaluacion](options/)** -- `EvalOptions`, opciones funcionales (`WithTimeout`, `WithContext`, `WithVariable`, etc.) y la interfaz `ReferenceResolver`.

### Sistema de Tipos

- **[Paquete Types](types/)** -- La interfaz `Value`, el tipo `Collection` con todos sus metodos, y todos los tipos primitivos de FHIRPath (`Boolean`, `Integer`, `Decimal`, `String`, `Date`, `DateTime`, `Time`, `Quantity`, `ObjectValue`).

## Elegir la Funcion Correcta

Utilice el siguiente arbol de decision para elegir el mejor punto de entrada para su caso de uso:

```text
Tiene un struct de Go que implementa Resource?
  SI --> Evalua muchas expresiones sobre el?
            SI --> Use ResourceJSON (serializar una vez, evaluar muchas)
            NO  --> Use EvaluateResource / EvaluateResourceCached
  NO  --> (Tiene bytes JSON crudos)
          Reutiliza la misma expresion muchas veces?
            SI --> Use Compile + Expression.Evaluate
                    (o ExpressionCache para cache automatico)
            NO  --> Necesita un tipo de Go especifico de retorno?
                      SI --> Use EvaluateToBoolean / EvaluateToString / Exists / Count
                      NO  --> Use Evaluate o EvaluateCached
```
