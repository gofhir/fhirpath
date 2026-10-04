package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/funcs"
	"github.com/gofhir/fhirpath/types"
)

// corpusSample is a fixed tenth of the R4 examples with the corpus expressions
// for each, compiled: what a validator evaluates over what it is given, small
// enough for an iteration to take well under a second.
type corpusSample struct {
	examples []sampleExample
	model    fhirpath.Model
	evals    int
}

type sampleExample struct {
	data  []byte
	exprs []*fhirpath.Expression
}

var (
	sampleOnce sync.Once
	sample     *corpusSample
	sampleErr  error
)

func loadSample() (*corpusSample, error) {
	sampleOnce.Do(func() {
		version := versions["r4"]
		cache := filepath.Join("..", "build", "corpusdiff", "cache")
		coreDir, err := fetch(cache, version.core)
		if err != nil {
			sampleErr = err
			return
		}
		examplesDir, err := fetch(cache, version.examples)
		if err != nil {
			sampleErr = err
			return
		}
		corpus, err := buildCorpus(coreDir, examplesDir)
		if err != nil {
			sampleErr = err
			return
		}

		files, err := filepath.Glob(filepath.Join(examplesDir, "*.json"))
		if err != nil {
			sampleErr = err
			return
		}
		sort.Strings(files)

		s := &corpusSample{model: version.model()}
		compiled := map[string]*fhirpath.Expression{}
		for i, file := range files {
			if i%10 != 0 {
				continue
			}
			data, err := os.ReadFile(file)
			if err != nil || len(data) > maxResource {
				continue
			}
			var head struct {
				ResourceType string `json:"resourceType"`
			}
			if json.Unmarshal(data, &head) != nil || head.ResourceType == "" {
				continue
			}

			ex := sampleExample{data: data}
			for _, text := range corpus[head.ResourceType] {
				expr, seen := compiled[text]
				if !seen {
					expr, _ = fhirpath.Compile(text)
					compiled[text] = expr
				}
				if expr != nil {
					ex.exprs = append(ex.exprs, expr)
				}
			}
			s.examples = append(s.examples, ex)
			s.evals += len(ex.exprs)
		}
		sample = s
	})
	return sample, sampleErr
}

// BenchmarkCorpus evaluates the corpus sample the three ways a caller can:
//
//   - document: one Document per resource, with the model — reading shared
//     across expressions.
//   - oneshot: Expression.Evaluate on the raw bytes, without a model — the
//     resource read again by every expression.
//   - context: a fresh eval.Context per evaluation, with the resource shared,
//     cached, as %resource — how gofhir/validator evaluates constraints.
//
// It reports time and allocations per evaluation. Fetches the R4 packages on
// first use, as make corpusdiff does.
func BenchmarkCorpus(b *testing.B) {
	s, err := loadSample()
	if err != nil {
		b.Skip("corpus unavailable:", err)
	}
	funcs.SetTraceLogger(funcs.NullTraceLogger{})

	run := func(b *testing.B, evaluate func(ex sampleExample)) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, ex := range s.examples {
				evaluate(ex)
			}
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*s.evals), "ns/eval")
	}

	b.Run("document", func(b *testing.B) {
		run(b, func(ex sampleExample) {
			doc, err := fhirpath.NewDocument(ex.data)
			if err != nil {
				return
			}
			for _, expr := range ex.exprs {
				_, _ = doc.EvaluateWithOptions(expr, fhirpath.WithModel(s.model))
			}
		})
	})

	b.Run("oneshot", func(b *testing.B) {
		run(b, func(ex sampleExample) {
			for _, expr := range ex.exprs {
				_, _ = expr.Evaluate(ex.data)
			}
		})
	})

	b.Run("context", func(b *testing.B) {
		run(b, func(ex sampleExample) {
			resource, err := types.JSONToCollection(ex.data)
			if err != nil {
				return
			}
			for _, value := range resource {
				if obj, ok := value.(*types.ObjectValue); ok {
					obj.EnableCaching()
				}
			}
			for _, expr := range ex.exprs {
				ctx := eval.NewContext(ex.data)
				ctx.SetVariable("resource", resource)
				ctx.SetVariable("rootResource", resource)
				_, _ = expr.EvaluateWithContext(ctx)
			}
		})
	})
}
