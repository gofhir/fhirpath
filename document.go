package fhirpath

import (
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/funcs"
	"github.com/gofhir/fhirpath/types"
)

// A Document is a resource read once and evaluated many times.
//
// Evaluate reads the resource it is given, and a validator evaluating the
// invariants of one resource — dom-6, then pat-1, then the profile's own —
// reads it again per expression, along with every element on the way to what
// each one asks for. A Document does that reading once and shares it, so the
// second expression down a path finds what the first one already navigated.
//
// The saving grows with the number of expressions and with the size of the
// resource; over a Bundle it is the difference between reading the bundle once
// and reading it per invariant.
//
// A Document holds what it reads, so it costs memory in proportion to how much
// of the resource is navigated, and it is not safe for concurrent use: evaluate
// against it from one goroutine at a time, or give each goroutine its own.
type Document struct {
	root types.Collection
}

// NewDocument reads a JSON resource for repeated evaluation.
func NewDocument(resource []byte) (*Document, error) {
	root, err := types.JSONToCollection(resource)
	if err != nil {
		return nil, err
	}

	// What the document reads is worth keeping, since the point of it is the
	// next expression over the same resource.
	for _, value := range root {
		if obj, ok := value.(*types.ObjectValue); ok {
			obj.EnableCaching()
		}
	}

	return &Document{root: root}, nil
}

// MustNewDocument is like NewDocument but panics on error.
func MustNewDocument(resource []byte) *Document {
	doc, err := NewDocument(resource)
	if err != nil {
		panic(err)
	}
	return doc
}

// Evaluate evaluates an expression against the document, compiling it through
// the default expression cache.
func (d *Document) Evaluate(expr string) (Collection, error) {
	compiled, err := DefaultCache.Get(expr)
	if err != nil {
		return nil, err
	}
	return d.EvaluateCompiled(compiled)
}

// EvaluateCompiled evaluates an already compiled expression against the
// document.
func (d *Document) EvaluateCompiled(expr *Expression) (Collection, error) {
	return d.EvaluateWithContext(expr, d.Context())
}

// EvaluateWithContext evaluates a compiled expression in a context the caller
// has set up — with variables, a model, or a resolver of its own. The context
// must be one this document produced, since it is what carries the reading of
// the resource.
func (d *Document) EvaluateWithContext(expr *Expression, ctx *eval.Context) (Collection, error) {
	evaluator := eval.NewEvaluator(ctx, funcs.GetRegistry())
	return evaluator.EvaluateNode(expr.node)
}

// Context returns a fresh evaluation context over the document's contents, for
// callers that need to configure one before evaluating.
//
// A context carries the variables an evaluation defines, so expressions that
// call defineVariable() need one each; the reading of the resource is shared
// either way.
func (d *Document) Context() *eval.Context {
	return eval.NewContextForRoot(d.root)
}
