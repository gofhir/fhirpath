---
title: "Modelos FHIR por Versi\u00f3n"
linkTitle: "Modelos FHIR"
weight: 4
description: >
  Usa la interfaz Model para proporcionar metadatos de tipos espec\u00edficos de cada versi\u00f3n
  FHIR, logrando resoluci\u00f3n polim\u00f3rfica precisa, verificaci\u00f3n de jerarqu\u00eda de tipos e
  inferencia basada en rutas.
---

## La Interfaz Model

El motor FHIRPath puede operar en dos modos:

- **Sin modelo** (por defecto): usa heur\u00edsticas internas para la resoluci\u00f3n de tipos. Esto
  funciona para la mayor\u00eda de casos comunes, pero puede producir resultados incorrectos en
  consultas avanzadas de jerarqu\u00eda de tipos (ej., `Age is Quantity` retorna `false`).
- **Con modelo**: usa metadatos precisos y espec\u00edficos de la versi\u00f3n. El modelo es
  **autoritativo** --- sus respuestas prevalecen sobre las heur\u00edsticas internas.

La interfaz `Model` proporciona siete m\u00e9todos:

```go
type Model interface {
    // ChoiceTypes retorna los tipos permitidos para un elemento polim\u00f3rfico.
    // Ejemplo: ChoiceTypes("Observation.value") retorna ["Quantity", "string", "boolean", ...]
    ChoiceTypes(path string) []string

    // TypeOf retorna el tipo FHIR de un elemento.
    // Ejemplo: TypeOf("Patient.name") retorna "HumanName"
    TypeOf(path string) string

    // ReferenceTargets retorna los tipos de recurso objetivo permitidos para un elemento Reference.
    // Ejemplo: ReferenceTargets("Observation.subject") retorna ["Patient", "Group", ...]
    ReferenceTargets(path string) []string

    // ParentType retorna el tipo padre en la jerarqu\u00eda de tipos FHIR.
    // Ejemplo: ParentType("Patient") retorna "DomainResource"
    ParentType(typeName string) string

    // IsSubtype retorna true si child es un subtipo de parent (transitivo).
    // Ejemplo: IsSubtype("Patient", "Resource") retorna true
    IsSubtype(child, parent string) bool

    // ResolvePath resuelve referencias de contenido a su ruta can\u00f3nica.
    // Ejemplo: ResolvePath("Questionnaire.item.item") retorna "Questionnaire.item"
    ResolvePath(path string) string

    // IsResource retorna true si el tipo es un tipo de recurso FHIR conocido.
    // Ejemplo: IsResource("Patient") retorna true, IsResource("HumanName") retorna false
    IsResource(typeName string) bool
}
```

La interfaz usa **tipado estructural de Go** (duck typing): cualquier tipo que implemente
estos siete m\u00e9todos satisface `Model`. No se requiere dependencia de import entre el
paquete del modelo y el motor FHIRPath.

## \u00bfPor Qu\u00e9 Usar un Modelo?

| Caracter\u00edstica | Sin Modelo | Con Modelo |
|----------------|-----------|------------|
| Resoluci\u00f3n polim\u00f3rfica (`value[x]`) | Prueba 39 sufijos hardcodeados | Usa `ChoiceTypes()` para resoluci\u00f3n precisa |
| Jerarqu\u00eda de tipos (`is`, `as`, `ofType`) | Heur\u00edstica: PascalCase = recurso | Usa `IsSubtype()` con cadena completa de tipos |
| `Age is Quantity` | `false` | `true` (v\u00eda cadena de `ParentType`) |
| `HumanName is Resource` | `true` (heur\u00edstica incorrecta) | `false` (correcto) |
| Referencias de contenido | Sin resoluci\u00f3n | Usa `ResolvePath()` |

## Usando gofhir/models

El paquete [`gofhir/models`](https://github.com/gofhir/models) proporciona modelos
pre-generados para FHIR R4, R4B y R5:

```go
package main

import (
    "fmt"

    "github.com/gofhir/fhirpath"
    "github.com/gofhir/models/r4"
)

func main() {
    observation := []byte(`{
        "resourceType": "Observation",
        "status": "final",
        "code": {"coding": [{"system": "http://loinc.org", "code": "29463-7"}]},
        "valueQuantity": {"value": 72, "unit": "kg"}
    }`)

    expr := fhirpath.MustCompile("Observation.value")

    // Con modelo R4 --- resoluci\u00f3n polim\u00f3rfica precisa
    result, err := expr.EvaluateWithOptions(observation,
        fhirpath.WithModel(r4.FHIRPathModel()),
    )
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // El valor Quantity
}
```

Para otras versiones de FHIR, usa el paquete correspondiente:

```go
import "github.com/gofhir/models/r5"

result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithModel(r5.FHIRPathModel()),
)
```

## Implementaci\u00f3n de Modelo Personalizado

Puedes implementar un modelo personalizado para pruebas o perfiles FHIR:

```go
type myModel struct{}

func (m *myModel) ChoiceTypes(path string) []string {
    if path == "Observation.value" {
        return []string{"Quantity", "string", "boolean"}
    }
    return nil
}

func (m *myModel) TypeOf(path string) string             { return "" }
func (m *myModel) ReferenceTargets(path string) []string  { return nil }
func (m *myModel) ParentType(typeName string) string      { return "" }
func (m *myModel) IsSubtype(child, parent string) bool    { return child == parent }
func (m *myModel) ResolvePath(path string) string         { return path }
func (m *myModel) IsResource(typeName string) bool        { return false }
```

{{< callout type="info" >}}
Los m\u00e9todos que retornan valores cero (string vac\u00edo, slice nil, false) se\u00f1alan "no hay
informaci\u00f3n disponible". El motor usa sus heur\u00edsticas internas como fallback para esas
consultas espec\u00edficas. Sin embargo, para consultas de jerarqu\u00eda de tipos (`IsSubtype`),
el modelo es **autoritativo** cuando est\u00e1 presente --- el fallback heur\u00edstico se omite
por completo.
{{< /callout >}}

## Sin Modelo

Cuando no se proporciona un modelo, el motor usa heur\u00edsticas internas:

- **Resoluci\u00f3n polim\u00f3rfica**: prueba los 39 sufijos de tipos conocidos (ej., `valueQuantity`,
  `valueString`, `valueBoolean`, etc.)
- **Jerarqu\u00eda de tipos**: asume que los nombres PascalCase que no son primitivos son tipos
  de recurso
- **Resource/DomainResource**: usa una lista hardcodeada de tipos que no heredan de
  DomainResource (Bundle, Binary, Parameters)

Este modo es completamente retrocompatible y funciona correctamente para la mayor\u00eda de
expresiones FHIRPath. Usa un modelo cuando necesites verificaci\u00f3n precisa de jerarqu\u00eda de
tipos o trabajes con elementos polim\u00f3rficos en m\u00faltiples versiones FHIR.

## Resumen

| Concepto | Descripci\u00f3n |
|----------|-------------|
| Interfaz `Model` | 7 m\u00e9todos para metadatos espec\u00edficos de versi\u00f3n FHIR |
| `WithModel(m)` | Opci\u00f3n funcional para adjuntar un modelo a una evaluaci\u00f3n |
| `gofhir/models` | Modelos pre-generados para R4, R4B y R5 |
| Duck typing | Sin dependencia de import entre modelo y motor |
| Autoritativo | El modelo prevalece sobre heur\u00edsticas para jerarqu\u00eda de tipos |
| Retrocompatible | Sin modelo = mismo comportamiento que antes |
