---
title: "Documentación"
linkTitle: "Documentación"
description: "Documentación completa de la biblioteca FHIRPath Go -- un evaluador de expresiones FHIRPath 2.0 para recursos FHIR."
weight: 1
---

Bienvenido a la documentación de **FHIRPath Go**. Esta biblioteca proporciona una implementación completa y lista para producción de la [especificación FHIRPath 2.0](http://hl7.org/fhirpath/) para evaluar expresiones sobre recursos FHIR en Go.

## Por Dónde Empezar

<div class="row">
<div class="col-md-6 mb-4">

### [Primeros Pasos]({{< relref "getting-started" >}})
Instala la biblioteca, escribe tu primera evaluación, aprende sobre la compilación y el almacenamiento en caché de expresiones, y explora las funciones de conveniencia.

</div>
<div class="col-md-6 mb-4">

### [Conceptos Fundamentales]({{< relref "concepts" >}})
Comprende el sistema de tipos de FHIRPath, las colecciones y la propagación de vacío, los operadores y la lógica de tres valores, y las variables de entorno.

</div>
</div>

## Características Principales

- **Más de 95 funciones integradas** que cubren existencia, filtrado, subconjuntos, manipulación de cadenas, matemáticas, verificación de tipos, operaciones de fecha/hora, agregación y más.
- **Cumplimiento total de FHIRPath 2.0** incluyendo lógica booleana de tres valores, precisión parcial de fecha/hora y normalización de cantidades UCUM.
- **Listo para producción** con evaluación segura para hilos, caché LRU de expresiones, tiempos de espera configurables y reutilización eficiente de memoria.
- **Sin dependencia de modelos FHIR** -- trabaja directamente con bytes JSON sin procesar, por lo que puedes usar cualquier biblioteca de modelos FHIR o ninguna.

## Resumen de Paquetes

| Paquete | Descripción |
|---------|-------------|
| `github.com/gofhir/fhirpath` | API de nivel superior: `Evaluate`, `Compile`, `EvaluateCached`, funciones auxiliares de conveniencia |
| `github.com/gofhir/fhirpath/types` | Sistema de tipos FHIRPath: `Value`, `Collection`, `Boolean`, `Integer`, `Decimal`, `String`, `Date`, `DateTime`, `Time`, `Quantity` |
| `github.com/gofhir/fhirpath/eval` | Motor de evaluación interno e implementaciones de operadores |
| `github.com/gofhir/fhirpath/funcs` | Registro de funciones integradas (existencia, filtrado, cadenas, matemáticas, etc.) |
