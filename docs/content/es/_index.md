---
title: "FHIRPath para Go"
description: "Un evaluador completo de expresiones FHIRPath 2.0 para recursos FHIR® en Go, con más de 95 funciones, normalización UCUM, caché LRU y evaluación segura para hilos."
layout: hextra-home
---

<div class="hx:text-center hx:mt-24 hx:mb-6">
{{< hextra/hero-badge >}}
  <span>Open Source</span>
  {{< icon name="github" attributes="height=14" >}}
{{< /hextra/hero-badge >}}
</div>

<div class="hx:mt-6 hx:mb-6">
{{< hextra/hero-headline >}}
  FHIRPath para Go
{{< /hextra/hero-headline >}}
</div>

<div class="hx:mb-12">
{{< hextra/hero-subtitle >}}
  Un evaluador de expresiones FHIRPath 2.0 completamente compatible construido para Go.&nbsp;<br class="sm:hx:block hx:hidden" />Evalúa, valida y extrae datos de recursos FHIR® con facilidad.
{{< /hextra/hero-subtitle >}}
</div>

<div class="hx:mb-6">
{{< hextra/hero-button text="Comenzar" link="docs/getting-started" >}}
{{< hextra/hero-button text="Ver en GitHub" link="https://github.com/gofhir/fhirpath" style="alt" >}}
</div>

<div class="hx:mt-6"></div>

{{< hextra/feature-grid >}}
  {{< hextra/feature-card
    title="Más de 95 Funciones"
    icon="puzzle"
    subtitle="Biblioteca completa: existencia, filtrado, subconjuntos, cadenas, matemáticas, verificación de tipos, fecha/hora, agregación y más."
  >}}
  {{< hextra/feature-card
    title="Cumplimiento Total"
    icon="badge-check"
    subtitle="FHIRPath 2.0 con lógica booleana de tres valores, precisión parcial de fecha/hora, normalización UCUM y todas las categorías de operadores."
  >}}
  {{< hextra/feature-card
    title="Listo para Producción"
    icon="lightning-bolt"
    subtitle="Evaluación segura para hilos, caché LRU, tiempos de espera configurables, reutilización eficiente de memoria y cero dependencias de modelos FHIR®."
  >}}
{{< /hextra/feature-grid >}}

## Inicio Rápido

{{< callout type="info" >}}
  Requiere **Go 1.21** o superior.
{{< /callout >}}

Instala la biblioteca:

```shell
go get github.com/gofhir/fhirpath
```

Evalúa una expresión FHIRPath:

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
    fmt.Println(result)
}
```

Salida:

```json
["Doe"]
```

{{< hextra/hero-button text="Leer la guía completa" link="docs/getting-started" >}}
