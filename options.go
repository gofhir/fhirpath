package fhirpath

import (
	"context"
	"time"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// EvalOptions configures expression evaluation.
type EvalOptions struct {
	// Context for cancellation and timeout
	Ctx context.Context

	// Timeout for evaluation (0 means no timeout)
	Timeout time.Duration

	// MaxDepth limits recursion depth for descendants() (0 means default of 100)
	MaxDepth int

	// MaxCollectionSize limits output collection size (0 means no limit)
	MaxCollectionSize int

	// Variables are external variables accessible via %name
	Variables map[string]types.Collection

	// Resolver handles reference resolution for resolve() function
	Resolver ReferenceResolver

	// Model provides FHIR version-specific type metadata.
	// When nil, the engine uses built-in heuristics.
	Model Model
}

// DefaultOptions returns default evaluation options suitable for production.
func DefaultOptions() *EvalOptions {
	return &EvalOptions{
		Ctx:               context.Background(),
		Timeout:           5 * time.Second,
		MaxDepth:          100,
		MaxCollectionSize: 10000,
		Variables:         make(map[string]types.Collection),
	}
}

// EvalOption is a functional option for configuring evaluation.
type EvalOption func(*EvalOptions)

// WithContext sets the context for cancellation.
func WithContext(ctx context.Context) EvalOption {
	return func(o *EvalOptions) {
		o.Ctx = ctx
	}
}

// WithTimeout sets the evaluation timeout.
func WithTimeout(d time.Duration) EvalOption {
	return func(o *EvalOptions) {
		o.Timeout = d
	}
}

// WithMaxDepth sets the maximum recursion depth.
func WithMaxDepth(depth int) EvalOption {
	return func(o *EvalOptions) {
		o.MaxDepth = depth
	}
}

// WithMaxCollectionSize sets the maximum output collection size.
func WithMaxCollectionSize(size int) EvalOption {
	return func(o *EvalOptions) {
		o.MaxCollectionSize = size
	}
}

// WithVariable sets an external variable.
func WithVariable(name string, value types.Collection) EvalOption {
	return func(o *EvalOptions) {
		if o.Variables == nil {
			o.Variables = make(map[string]types.Collection)
		}
		o.Variables[name] = value
	}
}

// WithResolver sets the reference resolver.
func WithResolver(r ReferenceResolver) EvalOption {
	return func(o *EvalOptions) {
		o.Resolver = r
	}
}

// ReferenceResolver resolves FHIR references for the resolve() function.
type ReferenceResolver interface {
	// Resolve takes a reference string (e.g., "Patient/123") and returns the resource.
	Resolve(ctx context.Context, reference string) ([]byte, error)
}

// EvaluateWithOptions evaluates an expression with custom options.
func (e *Expression) EvaluateWithOptions(resource []byte, opts ...EvalOption) (types.Collection, error) {
	evalCtx, done := configureContext(eval.NewContext(resource), opts...)
	defer done()

	return e.EvaluateWithContext(evalCtx)
}

// configureContext applies the options to an evaluation context, returning it
// with the call that releases what a timeout holds.
//
// The context is taken rather than made here, so that the same options can be
// applied to an evaluation over a resource read for the occasion and to one
// over a Document, which carries a reading of its own.
func configureContext(evalCtx *eval.Context, opts ...EvalOption) (configured *eval.Context, done func()) {
	options := DefaultOptions()
	for _, opt := range opts {
		opt(options)
	}

	// Create context with timeout if specified
	ctx := options.Ctx
	done = func() {}
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		done = cancel
	}

	// Set variables
	for name, value := range options.Variables {
		evalCtx.SetVariable(name, value)
	}

	// Set limits in context
	evalCtx.SetLimit("maxDepth", options.MaxDepth)
	evalCtx.SetLimit("maxCollectionSize", options.MaxCollectionSize)
	evalCtx.SetContext(ctx)

	// Set resolver if provided
	if options.Resolver != nil {
		evalCtx.SetResolver(newResolverAdapter(options.Resolver))
	}

	// Set model if provided
	if options.Model != nil {
		evalCtx.SetModel(newModelAdapter(options.Model))
	}

	return evalCtx, done
}

// resolverAdapter adapts ReferenceResolver to eval.Resolver
type resolverAdapter struct {
	resolver ReferenceResolver
}

func newResolverAdapter(r ReferenceResolver) *resolverAdapter {
	return &resolverAdapter{resolver: r}
}

func (a *resolverAdapter) Resolve(ctx context.Context, reference string) ([]byte, error) {
	return a.resolver.Resolve(ctx, reference)
}

// modelAdapter adapts fhirpath.Model to eval.Model
type modelAdapter struct {
	model Model
}

func newModelAdapter(m Model) *modelAdapter {
	return &modelAdapter{model: m}
}

func (a *modelAdapter) ChoiceTypes(path string) []string      { return a.model.ChoiceTypes(path) }
func (a *modelAdapter) TypeOf(path string) string             { return a.model.TypeOf(path) }
func (a *modelAdapter) ReferenceTargets(path string) []string { return a.model.ReferenceTargets(path) }
func (a *modelAdapter) ParentType(typeName string) string     { return a.model.ParentType(typeName) }
func (a *modelAdapter) IsSubtype(child, parent string) bool   { return a.model.IsSubtype(child, parent) }
func (a *modelAdapter) ResolvePath(path string) string        { return a.model.ResolvePath(path) }
func (a *modelAdapter) IsResource(typeName string) bool       { return a.model.IsResource(typeName) }

// FHIRVersion forwards the version when the wrapped model declares one, and
// returns "" otherwise, which the evaluator reads as pre-R5.
func (a *modelAdapter) FHIRVersion() string {
	if versioned, ok := a.model.(VersionedModel); ok {
		return versioned.FHIRVersion()
	}
	return ""
}

// LookupType forwards a type-name lookup, reporting separately whether the
// wrapped model could answer at all.
//
// The two results have to stay apart: a model that cannot enumerate its types
// is not a model in which every type is missing, and collapsing the two would
// turn every type specifier into an error.
func (a *modelAdapter) LookupType(typeName string) (known, supported bool) {
	registry, ok := a.model.(TypeRegistry)
	if !ok {
		return false, false
	}
	return registry.HasType(typeName), true
}
