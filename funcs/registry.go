// Package funcs provides FHIRPath function implementations.
package funcs

import (
	"sync"

	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// FuncDef is an alias for eval.FuncDef.
type FuncDef = eval.FuncDef

// Registry holds registered functions.
type Registry struct {
	funcs map[string]eval.FuncDef
	mu    sync.RWMutex
}

// globalRegistry is the default function registry.
var globalRegistry = NewRegistry()

// NewRegistry creates a new function registry.
func NewRegistry() *Registry {
	r := &Registry{
		funcs: make(map[string]eval.FuncDef),
	}
	return r
}

// Register adds a function to the registry.
func (r *Registry) Register(def eval.FuncDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.funcs[def.Name] = def
}

// Get retrieves a function by name.
func (r *Registry) Get(name string) (eval.FuncDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	fn, ok := r.funcs[name]
	return fn, ok
}

// Has checks if a function exists.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.funcs[name]
	return ok
}

// List returns all registered function names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.funcs))
	for name := range r.funcs {
		names = append(names, name)
	}
	return names
}

// Global registry functions

// Register adds a function to the global registry.
func Register(def eval.FuncDef) {
	globalRegistry.Register(def)
}

// Get retrieves a function from the global registry.
func Get(name string) (eval.FuncDef, bool) {
	return globalRegistry.Get(name)
}

// Has checks if a function exists in the global registry.
func Has(name string) bool {
	return globalRegistry.Has(name)
}

// List returns all function names from the global registry.
func List() []string {
	return globalRegistry.List()
}

// GetRegistry returns the global registry.
func GetRegistry() *Registry {
	return globalRegistry
}

// singleInput ends the evaluation when a function that operates on one value
// is given more, as the specification requires of each: "If the input
// collection contains multiple items, the evaluation of the expression will end
// and signal an error to the calling environment." It says so of the to- and
// convertsTo- functions, the math functions and the boundaries; encode, decode,
// escape, unescape and comparable are defined on "a singleton".
//
// Without it a function reads the first item, or answers empty, for a value the
// expression never singled out: (1 | 2).toString() gave '1', and
// (1 | -2).abs() gave 1.
func singleInput(name string, fn eval.FuncImpl) eval.FuncImpl {
	return func(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
		if len(input) > 1 {
			return nil, eval.NewEvalError(eval.ErrSingletonExpected,
				"%s() requires a singleton input, got %d items", name, len(input))
		}
		return fn(ctx, input, args)
	}
}
