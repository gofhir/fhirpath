---
title: "Temas Avanzados"
linkTitle: "Temas Avanzados"
weight: 5
description: >
  Profundización en caché de expresiones, opciones de evaluación, resolución de referencias, servicios de terminología, optimización del rendimiento y seguridad en hilos.
---

Esta sección cubre las características avanzadas de la biblioteca FHIRPath Go que le ayudan a construir
aplicaciones listas para producción. Cada tema se basa en la API principal presentada en la guía de
[Primeros Pasos](/docs/getting-started/).

## Lo Que Encontrará Aquí

- **[Caché de Expresiones](caching/)** -- Evite el análisis redundante con la caché
  de expresiones LRU incorporada. Aprenda a usar el `DefaultCache` global, crear cachés
  personalizadas, monitorear tasas de aciertos y precalentar cachés al inicio.

- **[Opciones de Evaluación](options-and-context/)** -- Controle el comportamiento de la evaluación con
  tiempos de espera, límites de recursión, límites de tamaño de colección y variables
  personalizadas a través de la API de opciones funcionales.

- **[Resolvedores de Referencias Personalizados](custom-resolvers/)** -- Implemente la interfaz
  `ReferenceResolver` para permitir que la función `resolve()` obtenga recursos FHIR
  referenciados desde endpoints HTTP, bundles en memoria o cualquier otra fuente de datos.

- **[Servicios de Terminología](terminology-services/)** -- Conecte las funciones `memberOf()` y
  `conformsTo()` a servidores de terminología externos y validadores de perfiles
  implementando las interfaces `TerminologyService` y `ProfileValidator`.

- **[Guía de Rendimiento](performance/)** -- Patrones prácticos para evaluación de alto
  rendimiento: compilar una vez, caché de expresiones, pre-serialización de recursos, filtrado
  temprano y evitar conversiones de tipos innecesarias.

- **[Seguridad en Hilos](thread-safety/)** -- Comprenda el modelo de concurrencia: qué
  objetos son seguros para compartir entre goroutines y cuáles deben permanecer por evaluación.
  Incluye ejemplos de manejadores HTTP y pools de workers.
