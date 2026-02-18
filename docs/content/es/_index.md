---
title: "FHIRPath para Go"
description: "Un evaluador completo de expresiones FHIRPath 2.0 para recursos FHIR® en Go, con más de 95 funciones, normalización UCUM, caché LRU y evaluación segura para hilos."
layout: hextra-home
---

<div class="hx-mt-6 hx-mb-6">
{{< hextra/hero-badge >}}
  <span>Código Abierto</span>
  {{< icon name="github" attributes="height=14" >}}
{{< /hextra/hero-badge >}}
</div>

<div class="hx-mt-6 hx-mb-6">
{{< hextra/hero-headline >}}
  FHIRPath para Go
{{< /hextra/hero-headline >}}
</div>

<div class="hx-mb-12">
{{< hextra/hero-subtitle >}}
  Un evaluador de expresiones FHIRPath 2.0 completamente compatible construido para Go&mdash;evalúa, valida y extrae datos de recursos FHIR® con facilidad.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx-mb-6">
{{< hextra/hero-button text="Comenzar" link="docs/getting-started" >}}
</div>

## ¿Por qué FHIRPath Go? {.hx-mt-6}

{{< cards >}}
  {{< card link="docs/getting-started" title="Más de 95 Funciones Integradas" icon="puzzle" subtitle="Biblioteca de funciones completa que cubre existencia, filtrado, subconjuntos, manipulación de cadenas, matemáticas, verificación de tipos, operaciones de fecha/hora, agregación y más." >}}
  {{< card link="docs/concepts" title="Cumplimiento Total de la Especificación" icon="badge-check" subtitle="Implementa la especificación completa de FHIRPath 2.0 incluyendo lógica booleana de tres valores, precisión parcial de fecha/hora, normalización de unidades UCUM y todas las categorías de operadores." >}}
  {{< card link="docs/getting-started" title="Listo para Producción" icon="lightning-bolt" subtitle="Evaluación concurrente segura para hilos, caché LRU de expresiones, tiempos de espera configurables, reutilización de objetos eficiente en memoria y cero dependencias de modelos FHIR® externos." >}}
{{< /cards >}}

## Inicio Rápido {.hx-mt-6}

### Instala la biblioteca

```bash
go get github.com/gofhir/fhirpath
```

### Evalúa una expresión FHIRPath en cinco líneas

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func main() {
    patient := []byte(`{"resourceType":"Patient","name":[{"family":"Doe","given":["John"]}]}`)
    result, err := fhirpath.Evaluate(patient, "Patient.name.family")
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // [Doe]
}
```

{{< hextra/hero-button text="Leer la guía completa" link="docs/getting-started" >}}
