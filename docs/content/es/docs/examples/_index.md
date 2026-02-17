---
title: "Ejemplos"
linkTitle: "Ejemplos"
weight: 6
description: >
  Ejemplos prácticos y recetas tipo cookbook para tareas comunes de evaluación FHIRPath en Go.
---

Esta sección contiene ejemplos prácticos que demuestran cómo usar la biblioteca FHIRPath Go en escenarios del mundo real. Cada página incluye código Go completo y ejecutable junto con recursos FHIR JSON realistas para que pueda copiar, pegar y adaptarlos a sus propios proyectos.

## Lo Que Encontrará Aquí

| Página | Descripción |
|--------|-------------|
| [Consultas Básicas](basic-queries/) | Extraer datos demográficos, navegar estructuras anidadas, trabajar con arreglos y usar sintaxis de rutas |
| [Filtrado de Datos](filtering-data/) | Usar `where()`, `select()`, `exists()`, `count()` y `empty()` para filtrar y proyectar datos FHIR |
| [Validación FHIR](fhir-validation/) | Evaluar restricciones e invariantes FHIR usando expresiones booleanas |
| [Trabajo con Extensions](working-with-extensions/) | Acceder, verificar y extraer valores de extensions FHIR |
| [Cantidades y Unidades](quantities-and-units/) | Comparar y manipular cantidades UCUM en valores de Observation |
| [Patrones del Mundo Real](real-world-patterns/) | Patrones de producción incluyendo middleware HTTP, pipelines de procesamiento por lotes y manejo de errores |

## Prerrequisitos

Todos los ejemplos asumen que tiene la biblioteca instalada:

```bash
go get github.com/gofhir/fhirpath
```

E importada en sus archivos Go:

```go
import "github.com/gofhir/fhirpath"
```

La mayoría de los ejemplos trabajan con bytes JSON sin procesar (`[]byte`) como entrada, lo que significa que no necesita ninguna biblioteca de modelos FHIR. Puede cargar recursos desde archivos, respuestas HTTP o bases de datos -- cualquier cosa que le proporcione el JSON como bytes.
