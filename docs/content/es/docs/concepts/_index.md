---
title: "Conceptos Fundamentales"
linkTitle: "Conceptos"
description: "Comprende los conceptos fundamentales de FHIRPath tal como se implementan en la biblioteca FHIRPath Go: el sistema de tipos, las colecciones, los operadores y las variables de entorno."
weight: 2
---

FHIRPath es un lenguaje de navegación y extracción basado en rutas diseñado para su uso con recursos FHIR®. Antes de profundizar en el uso avanzado, es importante comprender los conceptos fundamentales que gobiernan cómo se evalúan las expresiones.

Esta sección cubre:

- **[Sistema de Tipos]({{< relref "type-system" >}})** -- Los ocho tipos primitivos (Boolean, Integer, Decimal, String, Date, DateTime, Time, Quantity), sus representaciones en Go y las interfaces `Value`, `Comparable` y `Numeric`.

- **[Colecciones]({{< relref "collections" >}})** -- Cómo FHIRPath representa todos los resultados como colecciones ordenadas, las reglas para la propagación de vacío (lógica de tres valores), la evaluación singleton y el conjunto completo de operaciones de colección.

- **[Operadores]({{< relref "operators" >}})** -- Operadores aritméticos, de comparación, de igualdad, de equivalencia, booleanos, de colección y de tipo, incluyendo reglas de precedencia y tablas de verdad de tres valores.

- **[Variables de Entorno]({{< relref "environment-variables" >}})** -- Variables integradas (`%resource`, `%context`, `%ucum`) y cómo definir variables personalizadas con `WithVariable()`.
