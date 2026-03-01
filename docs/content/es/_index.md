---
title: "FHIRPath para Go"
description: "Un evaluador completo de expresiones FHIRPath 2.0 para recursos FHIR® en Go, con más de 95 funciones, normalización UCUM, caché LRU y evaluación segura para hilos."
layout: hextra-home
---

<div class="hx:text-center hx:mt-24 hx:mb-6">
{{< hextra/hero-badge >}}
  <span>Código Abierto</span>
  {{< icon name="github" attributes="height=14" >}}
{{< /hextra/hero-badge >}}
</div>

<div class="hx:text-center hx:mt-8 hx:mb-6">
{{< hextra/hero-headline >}}
  FHIRPath para Go
{{< /hextra/hero-headline >}}
</div>

<div class="hx:text-center hx:mt-6 hx:mb-20">
{{< hextra/hero-subtitle >}}
  Un evaluador de expresiones FHIRPath 2.0 completamente compatible construido para Go.&nbsp;<br class="sm:hx:block hx:hidden" />Evalúa, valida y extrae datos de recursos FHIR® con facilidad.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:text-center hx:mb-32">
{{< hextra/hero-button text="Comenzar" link="docs/getting-started" >}}
{{< hextra/hero-button text="Ver en GitHub" link="https://github.com/gofhir/fhirpath" style="background: transparent; border: 1px solid #e5e7eb; color: inherit;" >}}
</div>

<div class="hx:mt-32"></div>

## Características

<div class="hx:mt-8"></div>

{{< cards >}}
  {{< card link="docs/functions" title="Más de 95 Funciones" icon="puzzle" subtitle="Biblioteca completa: existencia, filtrado, subconjuntos, cadenas, matemáticas, verificación de tipos, fecha/hora, agregación y más." >}}
  {{< card link="docs/concepts" title="Cumplimiento Total" icon="badge-check" subtitle="FHIRPath 2.0 con lógica booleana de tres valores, precisión parcial de fecha/hora, normalización UCUM y todas las categorías de operadores." >}}
  {{< card link="docs/advanced/performance" title="Listo para Producción" icon="lightning-bolt" subtitle="Evaluación segura para hilos, caché LRU, tiempos de espera configurables, reutilización eficiente de memoria y cero dependencias de modelos FHIR®." >}}
{{< /cards >}}

<div class="hx:mt-32"></div>

## Inicio Rápido

<div class="hx:mt-8"></div>

{{< callout type="info" >}}
Requiere **Go 1.21** o superior.
{{< /callout >}}

<div class="hx:mt-8"></div>

**Instala la biblioteca:**

```bash
go get github.com/gofhir/fhirpath
```

<div class="hx:mt-6"></div>

**Evalúa una expresión FHIRPath:**

```go
package main

import (
    "fmt"
    "github.com/gofhir/fhirpath"
)

func main() {
    patient := []byte(`{
        "resourceType": "Patient",
        "name": [{"family": "Doe", "given": ["John"]}]
    }`)

    result, err := fhirpath.Evaluate(patient, "Patient.name.family")
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // [Doe]
}
```

<div class="hx:text-center hx:mt-16 hx:mb-32">
{{< hextra/hero-button text="Leer la guía completa →" link="docs/getting-started" >}}
</div>
