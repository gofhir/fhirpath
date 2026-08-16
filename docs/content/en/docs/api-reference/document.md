---
title: "Document"
linkTitle: "Document"
weight: 5
description: >
  Read a resource once and evaluate many expressions against it, as validation does.
---

`Evaluate` reads the resource it is given. A validator running the invariants of one resource — `dom-6`, then `pat-1`, then whatever the profile adds — reads it again for every expression, along with every element on the way to what each one asks for.

A `Document` reads it once and shares that reading, so the second expression down a path finds what the first one already navigated.

```go
doc, err := fhirpath.NewDocument(resource)
if err != nil {
    return err
}

for _, invariant := range invariants {
    result, err := doc.Evaluate(invariant)
    // handle result...
}
```

## When It Pays

The saving grows with the number of expressions and with the size of the resource. Over a Bundle it is the difference between reading the bundle once and reading it per invariant.

Eight invariants over a Bundle of 50 entries, on an Apple M4 Pro:

| | time | allocations |
|---|---|---|
| `expr.Evaluate(resource)` per expression | 0.55 ms | 4952 |
| through a `Document` | **0.24 ms** | **3478** |

Under an R4 model, the same eight: 0.54 ms against 0.27 ms.

One expression against one resource does not benefit — there is nothing to share — and a `Document` costs a little more than a plain evaluation there. Reach for it when several expressions meet one resource.

## Cost

A `Document` keeps what it reads, so it holds memory in proportion to how much of the resource is navigated, until the document itself is released. A document per request, released with the request, is the shape to aim for. Holding one for a large Bundle across the lifetime of a process is not.

## Concurrency

A `Document` is **not** safe for concurrent use. What it keeps is a plain map, written whenever a field is read for the first time, so two goroutines evaluating against one document is a data race.

Evaluate against it from one goroutine at a time, or give each goroutine its own:

```go
// WRONG -- one document, many goroutines.
doc := fhirpath.MustNewDocument(resource)
for _, invariant := range invariants {
    go func(expr string) { doc.Evaluate(expr) }(invariant) // DATA RACE
}
```

```go
// CORRECT -- a document per goroutine.
for _, chunk := range chunks {
    go func(exprs []string) {
        doc := fhirpath.MustNewDocument(resource)
        for _, expr := range exprs {
            doc.Evaluate(expr)
        }
    }(chunk)
}
```

Compiled expressions remain shareable, as they always were: it is the reading of the resource that a document owns, not the expression. See [Thread Safety]({{< relref "/docs/advanced/thread-safety" >}}).

## Methods

### NewDocument

```go
func NewDocument(resource []byte) (*Document, error)
func MustNewDocument(resource []byte) *Document
```

Reads a JSON resource for repeated evaluation. Returns an error if the JSON cannot be read; `MustNewDocument` panics instead.

### Evaluate

```go
func (d *Document) Evaluate(expr string) (Collection, error)
```

Evaluates an expression, compiling it through the default expression cache — so the expression is compiled once across the process, and the resource is read once across the document.

### EvaluateCompiled

```go
func (d *Document) EvaluateCompiled(expr *Expression) (Collection, error)
```

Evaluates an expression compiled ahead of time. This is what to use when the expressions are known at startup, which is the usual case for invariants.

### EvaluateWithOptions

```go
func (d *Document) EvaluateWithOptions(expr *Expression, opts ...EvalOption) (Collection, error)
```

Evaluates with the options an ordinary evaluation takes — a model above all, which is what a validator configures:

```go
doc := fhirpath.MustNewDocument(resource)
model := fhirpath.WithModel(r4.FHIRPathModel())

for _, expr := range invariants {
    result, err := doc.EvaluateWithOptions(expr, model)
    // handle result...
}
```

Options that describe the evaluation — variables, limits, timeout, resolver — apply to that call alone. See [Options]({{< relref "/docs/api-reference/options" >}}).

### Context

```go
func (d *Document) Context() *eval.Context
```

Returns a fresh evaluation context over the document's contents, for callers configuring one directly. A context carries the variables an evaluation defines, so expressions calling `defineVariable()` need one each; the reading of the resource is shared either way.
