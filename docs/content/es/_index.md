---
title: "FHIRPath para Go"
description: "Un evaluador completo de expresiones FHIRPath 2.0 para recursos FHIR en Go, con más de 95 funciones, normalización UCUM, caché LRU y evaluación segura para hilos."
---

{{< blocks/cover title="FHIRPath para Go" image_anchor="top" height="full" >}}
<a class="btn btn-lg btn-primary me-3 mb-4" href="{{< relref "/docs" >}}">
Documentación <i class="fas fa-arrow-alt-circle-right ms-2"></i>
</a>
<a class="btn btn-lg btn-secondary me-3 mb-4" href="https://github.com/gofhir/fhirpath">
GitHub <i class="fab fa-github ms-2"></i>
</a>
<p class="lead mt-5">Un evaluador de expresiones FHIRPath 2.0 completamente compatible construido para Go &mdash; evalúa, valida y extrae datos de recursos FHIR con facilidad.</p>
{{< blocks/link-down color="info" >}}
{{< /blocks/cover >}}


{{% blocks/lead color="primary" %}}
**FHIRPath Go** es una biblioteca de código abierto lista para producción que implementa la
[especificación FHIRPath 2.0](http://hl7.org/fhirpath/) para evaluar expresiones
sobre recursos FHIR en aplicaciones Go. Incluye más de 95 funciones integradas,
normalización automática de cantidades UCUM, un caché LRU de expresiones y un motor
de evaluación completamente seguro para hilos.
{{% /blocks/lead %}}


{{% blocks/section color="dark" type="row" %}}

{{% blocks/feature icon="fa-solid fa-puzzle-piece" title="Más de 95 Funciones Integradas" url="/docs/getting-started/" %}}
Biblioteca de funciones completa que cubre existencia, filtrado, subconjuntos,
manipulación de cadenas, matemáticas, verificación de tipos, operaciones de fecha/hora,
agregación y más. Cada función de la especificación FHIRPath está implementada.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-certificate" title="Cumplimiento Total de la Especificación" url="/docs/concepts/" %}}
Implementa la especificación completa de FHIRPath 2.0 incluyendo lógica booleana
de tres valores, precisión parcial de fecha/hora, normalización de unidades UCUM y
todas las categorías de operadores. Validado contra la suite de pruebas oficial de FHIRPath.
{{% /blocks/feature %}}

{{% blocks/feature icon="fa-solid fa-bolt" title="Listo para Producción" url="/docs/getting-started/" %}}
Evaluación concurrente segura para hilos, caché LRU de expresiones, tiempos de espera
y límites de profundidad configurables, reutilización de objetos eficiente en memoria y
cero dependencias de modelos FHIR externos. Listo para servicios de alto rendimiento.
{{% /blocks/feature %}}

{{% /blocks/section %}}


{{< blocks/section color="light" >}}
<div class="col-lg-8 mx-auto">
  <h2 class="text-center mb-4">Inicio Rápido</h2>

  <h5><i class="fa-solid fa-download me-2 text-primary"></i>Instala la biblioteca</h5>

{{< highlight bash >}}
go get github.com/gofhir/fhirpath
{{< /highlight >}}

  <h5 class="mt-4"><i class="fa-solid fa-code me-2 text-primary"></i>Evalúa una expresión FHIRPath en cinco líneas</h5>

{{< highlight go >}}
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
{{< /highlight >}}

  <div class="text-center mt-4">
    <a class="btn btn-lg btn-primary" href="{{< relref "/docs/getting-started" >}}">
      Leer la guía completa <i class="fas fa-arrow-right ms-2"></i>
    </a>
  </div>
</div>
{{< /blocks/section >}}
