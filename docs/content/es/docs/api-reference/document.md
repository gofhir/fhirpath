---
title: "Document"
linkTitle: "Document"
weight: 5
description: >
  Lea un recurso una vez y evalúe muchas expresiones contra él, como hace la validación.
---

`Evaluate` lee el recurso que recibe. Un validador que corre las invariantes de un recurso — `dom-6`, luego `pat-1`, luego las que agregue el perfil — vuelve a leerlo en cada expresión, junto con cada elemento del camino hacia lo que cada una pide.

Un `Document` lo lee una vez y comparte esa lectura, de modo que la segunda expresión de un camino encuentra lo que la primera ya navegó.

```go
doc, err := fhirpath.NewDocument(resource)
if err != nil {
    return err
}

for _, invariant := range invariants {
    result, err := doc.Evaluate(invariant)
    // procesar el resultado...
}
```

## Cuándo Conviene

El ahorro crece con la cantidad de expresiones y con el tamaño del recurso. Sobre un Bundle es la diferencia entre leer el bundle una vez y leerlo por invariante.

Ocho invariantes sobre un Bundle de 50 entradas, en un Apple M4 Pro:

| | tiempo | asignaciones |
|---|---|---|
| `expr.Evaluate(resource)` por expresión | 0.55 ms | 4952 |
| a través de un `Document` | **0.24 ms** | **3478** |

Bajo un modelo R4, las mismas ocho: 0.54 ms contra 0.27 ms.

Una expresión contra un recurso no se beneficia — no hay nada que compartir — y ahí un `Document` cuesta algo más que una evaluación simple. Recurra a él cuando varias expresiones se encuentran con un mismo recurso.

## Costo

Un `Document` conserva lo que lee, así que retiene memoria en proporción a cuánto del recurso se navega, hasta que se libera el documento mismo. Un documento por request, liberado con el request, es la forma a buscar. Sostener uno de un Bundle grande durante toda la vida del proceso, no.

## Concurrencia

Un `Document` **no** es seguro para uso concurrente. Lo que conserva es un mapa común, escrito cada vez que un campo se lee por primera vez, así que dos goroutines evaluando contra un mismo documento son una condición de carrera.

Evalúe contra él desde una goroutine a la vez, o dele a cada goroutine el suyo:

```go
// INCORRECTO -- un documento, muchas goroutines.
doc := fhirpath.MustNewDocument(resource)
for _, invariant := range invariants {
    go func(expr string) { doc.Evaluate(expr) }(invariant) // CONDICIÓN DE CARRERA
}
```

```go
// CORRECTO -- un documento por goroutine.
for _, chunk := range chunks {
    go func(exprs []string) {
        doc := fhirpath.MustNewDocument(resource)
        for _, expr := range exprs {
            doc.Evaluate(expr)
        }
    }(chunk)
}
```

Las expresiones compiladas se siguen pudiendo compartir, como siempre: lo que el documento posee es la lectura del recurso, no la expresión. Vea [Seguridad en Hilos]({{< relref "/docs/advanced/thread-safety" >}}).

## Métodos

### NewDocument

```go
func NewDocument(resource []byte) (*Document, error)
func MustNewDocument(resource []byte) *Document
```

Lee un recurso JSON para evaluarlo repetidamente. Devuelve un error si el JSON no se puede leer; `MustNewDocument` entra en pánico en su lugar.

### Evaluate

```go
func (d *Document) Evaluate(expr string) (Collection, error)
```

Evalúa una expresión, compilándola a través del caché de expresiones por defecto — así la expresión se compila una vez en el proceso, y el recurso se lee una vez en el documento.

### EvaluateCompiled

```go
func (d *Document) EvaluateCompiled(expr *Expression) (Collection, error)
```

Evalúa una expresión compilada de antemano. Es lo que conviene usar cuando las expresiones se conocen al arrancar, que es el caso habitual de las invariantes.

### EvaluateWithOptions

```go
func (d *Document) EvaluateWithOptions(expr *Expression, opts ...EvalOption) (Collection, error)
```

Evalúa con las opciones que toma una evaluación ordinaria — un modelo ante todo, que es lo que configura un validador:

```go
doc := fhirpath.MustNewDocument(resource)
model := fhirpath.WithModel(r4.FHIRPathModel())

for _, expr := range invariants {
    result, err := doc.EvaluateWithOptions(expr, model)
    // procesar el resultado...
}
```

Las opciones que describen la evaluación — variables, límites, tiempo máximo, resolutor — rigen solo para esa llamada. Vea [Opciones]({{< relref "/docs/api-reference/options" >}}).

### Context

```go
func (d *Document) Context() *eval.Context
```

Devuelve un contexto de evaluación nuevo sobre el contenido del documento, para quien configure uno directamente. Un contexto lleva las variables que una evaluación define, así que las expresiones que llaman a `defineVariable()` necesitan uno cada una; la lectura del recurso se comparte de todos modos.
