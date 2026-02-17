---
title: "Patrones del Mundo Real"
linkTitle: "Patrones del Mundo Real"
weight: 6
description: >
  Patrones de producción para middleware HTTP, pipelines de procesamiento por lotes, endpoints de validación de expresiones y manejo robusto de errores.
---

Esta página muestra cómo la biblioteca FHIRPath de Go se integra en servicios Go de producción. Cada patrón es autocontenido y demuestra un escenario de integración común.

## Middleware HTTP para Evaluación de Parámetros de Búsqueda FHIR

Un servidor FHIR frecuentemente necesita evaluar parámetros de búsqueda contra recursos almacenados. El siguiente middleware evalúa una expresión FHIRPath proporcionada en un parámetro de consulta y retorna el resultado como JSON.

```go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gofhir/fhirpath"
)

// FHIRPathHandler evaluates a FHIRPath expression against a posted FHIR resource.
//
// Usage:
//   POST /fhirpath?expr=Patient.name.family
//   Body: { "resourceType": "Patient", ... }
//
// Response:
//   { "result": ["Smith"], "count": 1 }
func FHIRPathHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	expr := r.URL.Query().Get("expr")
	if expr == "" {
		http.Error(w, `missing "expr" query parameter`, http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Validate that the body is valid JSON
	if !json.Valid(body) {
		http.Error(w, "body is not valid JSON", http.StatusBadRequest)
		return
	}

	// Use EvaluateCached for automatic expression caching
	result, err := fhirpath.EvaluateCached(body, expr)
	if err != nil {
		http.Error(w, fmt.Sprintf("evaluation error: %v", err), http.StatusUnprocessableEntity)
		return
	}

	// Convert the result to a JSON-serializable form
	items := make([]interface{}, len(result))
	for i, v := range result {
		items[i] = v.String()
	}

	response := map[string]interface{}{
		"result": items,
		"count":  len(items),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	http.HandleFunc("/fhirpath", FHIRPathHandler)
	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Prueba del Middleware

```bash
curl -X POST "http://localhost:8080/fhirpath?expr=Patient.name.family" \
  -H "Content-Type: application/json" \
  -d '{"resourceType":"Patient","name":[{"family":"Smith","given":["John"]}]}'
```

Respuesta esperada:

```json
{"count":1,"result":["Smith"]}
```

## Pipeline de Procesamiento por Lotes

Cuando necesita evaluar expresiones contra una gran cantidad de recursos (por ejemplo, extrayendo índices de búsqueda), precompile la expresión una vez y reutilícela:

```go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/gofhir/fhirpath"
)

// ExtractionRule defines a field to extract from a resource.
type ExtractionRule struct {
	Name       string
	Expression *fhirpath.Expression
}

// ExtractionResult holds the extracted values for one resource.
type ExtractionResult struct {
	ResourceID string
	Fields     map[string][]string
	Errors     []string
}

// BatchExtractor precompiles expressions and runs them against many resources.
type BatchExtractor struct {
	rules []ExtractionRule
}

// NewBatchExtractor creates an extractor from a map of name -> FHIRPath expression strings.
func NewBatchExtractor(expressions map[string]string) (*BatchExtractor, error) {
	rules := make([]ExtractionRule, 0, len(expressions))
	for name, expr := range expressions {
		compiled, err := fhirpath.Compile(expr)
		if err != nil {
			return nil, fmt.Errorf("failed to compile %q: %w", name, err)
		}
		rules = append(rules, ExtractionRule{Name: name, Expression: compiled})
	}
	return &BatchExtractor{rules: rules}, nil
}

// Extract evaluates all rules against a single resource.
func (b *BatchExtractor) Extract(resource []byte) ExtractionResult {
	result := ExtractionResult{
		Fields: make(map[string][]string),
	}

	// Try to get the resource ID
	id, _ := fhirpath.EvaluateToString(resource, "id")
	result.ResourceID = id

	for _, rule := range b.rules {
		col, err := rule.Expression.Evaluate(resource)
		if err != nil {
			result.Errors = append(result.Errors,
				fmt.Sprintf("%s: %v", rule.Name, err))
			continue
		}
		values := make([]string, len(col))
		for i, v := range col {
			values[i] = v.String()
		}
		result.Fields[rule.Name] = values
	}

	return result
}

// ExtractBatch processes multiple resources concurrently.
func (b *BatchExtractor) ExtractBatch(resources [][]byte, workers int) []ExtractionResult {
	results := make([]ExtractionResult, len(resources))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)

	for i, resource := range resources {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore slot

		go func(idx int, res []byte) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore slot

			results[idx] = b.Extract(res)
		}(i, resource)
	}

	wg.Wait()
	return results
}

func main() {
	// Define extraction rules for Patient resources
	extractor, err := NewBatchExtractor(map[string]string{
		"family":    "Patient.name.where(use = 'official').family",
		"given":     "Patient.name.where(use = 'official').given",
		"birthDate": "Patient.birthDate",
		"city":      "Patient.address.where(use = 'home').city",
		"phone":     "Patient.telecom.where(system = 'phone').value",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Simulate a batch of patients
	patients := [][]byte{
		[]byte(`{
			"resourceType": "Patient", "id": "p1",
			"name": [{"use": "official", "family": "Smith", "given": ["Alice"]}],
			"birthDate": "1990-05-15",
			"address": [{"use": "home", "city": "Portland"}],
			"telecom": [{"system": "phone", "value": "555-0001"}]
		}`),
		[]byte(`{
			"resourceType": "Patient", "id": "p2",
			"name": [{"use": "official", "family": "Johnson", "given": ["Bob", "James"]}],
			"birthDate": "1985-11-20",
			"address": [{"use": "home", "city": "Seattle"}],
			"telecom": [{"system": "phone", "value": "555-0002"}, {"system": "email", "value": "bob@example.com"}]
		}`),
		[]byte(`{
			"resourceType": "Patient", "id": "p3",
			"name": [{"use": "official", "family": "Williams", "given": ["Carol"]}],
			"birthDate": "1978-03-08",
			"address": [{"use": "work", "city": "Denver"}],
			"telecom": [{"system": "email", "value": "carol@example.com"}]
		}`),
	}

	// Process with 4 concurrent workers
	results := extractor.ExtractBatch(patients, 4)

	// Output results
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	for _, r := range results {
		fmt.Printf("--- Patient %s ---\n", r.ResourceID)
		enc.Encode(r.Fields)
		if len(r.Errors) > 0 {
			fmt.Println("  Errors:", r.Errors)
		}
	}
}
```

## Endpoint de Validación de Expresiones

Antes de almacenar una expresión FHIRPath proporcionada por el usuario (por ejemplo, en una base de datos de configuración), debe validar que se compile correctamente. Este endpoint hace exactamente eso:

```go
package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gofhir/fhirpath"
)

type ValidateRequest struct {
	Expression string `json:"expression"`
}

type ValidateResponse struct {
	Valid   bool   `json:"valid"`
	Error   string `json:"error,omitempty"`
}

func ValidateExpressionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	if req.Expression == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ValidateResponse{
			Valid: false,
			Error: "expression is required",
		})
		return
	}

	// Try to compile the expression
	_, err := fhirpath.Compile(req.Expression)

	resp := ValidateResponse{Valid: err == nil}
	if err != nil {
		resp.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func main() {
	http.HandleFunc("/validate-expression", ValidateExpressionHandler)
	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Prueba del Endpoint de Validación

Expresión válida:

```bash
curl -X POST http://localhost:8080/validate-expression \
  -H "Content-Type: application/json" \
  -d '{"expression": "Patient.name.where(use = '\''official'\'').family"}'
```

```json
{"valid": true}
```

Expresión inválida:

```bash
curl -X POST http://localhost:8080/validate-expression \
  -H "Content-Type: application/json" \
  -d '{"expression": "Patient.name.where(!!!"}'
```

```json
{"valid": false, "error": "...parse error details..."}
```

## Patrones de Manejo de Errores en Producción

El manejo robusto de errores es esencial para código de producción. Estos son los patrones que debe seguir.

### Distinción entre Errores de Compilación y Evaluación

```go
package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/gofhir/fhirpath"
)

func safeEvaluate(resource []byte, expr string) {
	// Step 1: Compile the expression (catches syntax errors)
	compiled, err := fhirpath.Compile(expr)
	if err != nil {
		log.Printf("COMPILE ERROR for %q: %v", expr, err)
		return
	}

	// Step 2: Evaluate against the resource (catches runtime errors)
	result, err := compiled.Evaluate(resource)
	if err != nil {
		log.Printf("EVAL ERROR for %q: %v", expr, err)
		return
	}

	fmt.Printf("Expression: %s\n", expr)
	fmt.Printf("Result:     %v (count: %d)\n\n", result, len(result))
}

func main() {
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "pat-err",
		"name": [{"family": "TestPatient"}]
	}`)

	// Valid expression
	safeEvaluate(patient, "Patient.name.family")

	// Syntax error
	safeEvaluate(patient, "Patient.name.!!!")

	// Valid expression but path does not exist -- returns empty, not an error
	safeEvaluate(patient, "Patient.maritalStatus.coding.code")
}
```

### Envolviendo Errores con Contexto

```go
func extractField(resource []byte, resourceID, fieldName, expr string) (string, error) {
	result, err := fhirpath.EvaluateCached(resource, expr)
	if err != nil {
		return "", fmt.Errorf(
			"failed to extract %s from resource %s: %w",
			fieldName, resourceID, err,
		)
	}

	if result.Empty() {
		return "", nil // field is absent -- not an error in FHIR
	}

	return result[0].String(), nil
}
```

### Degradación Elegante

En muchos escenarios de producción, se desea extraer la mayor cantidad de datos posible, registrando las fallas en lugar de abortar:

```go
package main

import (
	"fmt"
	"log"

	"github.com/gofhir/fhirpath"
)

// FieldSpec defines a field to extract.
type FieldSpec struct {
	Name       string
	Expression string
	Required   bool
}

// ExtractFields extracts all specified fields, collecting errors for failed ones.
func ExtractFields(resource []byte, specs []FieldSpec) (map[string]string, []error) {
	fields := make(map[string]string)
	var errs []error

	for _, spec := range specs {
		value, err := fhirpath.EvaluateToString(resource, spec.Expression)
		if err != nil {
			errs = append(errs, fmt.Errorf("field %s: %w", spec.Name, err))
			continue
		}

		if value == "" && spec.Required {
			errs = append(errs,
				fmt.Errorf("field %s: required but empty", spec.Name))
			continue
		}

		fields[spec.Name] = value
	}

	return fields, errs
}

func main() {
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "pat-degrade",
		"name": [{"family": "Torres", "given": ["Sofia"]}],
		"gender": "female"
	}`)

	specs := []FieldSpec{
		{Name: "family", Expression: "Patient.name.family", Required: true},
		{Name: "given", Expression: "Patient.name.given.first()", Required: true},
		{Name: "gender", Expression: "Patient.gender", Required: false},
		{Name: "birthDate", Expression: "Patient.birthDate", Required: true},
		{Name: "phone", Expression: "Patient.telecom.where(system = 'phone').value", Required: false},
	}

	fields, errs := ExtractFields(patient, specs)

	fmt.Println("Extracted fields:")
	for k, v := range fields {
		fmt.Printf("  %s = %s\n", k, v)
	}

	if len(errs) > 0 {
		fmt.Println("\nWarnings/Errors:")
		for _, e := range errs {
			log.Printf("  %v", e)
		}
	}
}
```

### Monitoreo del Rendimiento de la Caché

En servicios de larga ejecución, monitoree la caché de expresiones para asegurar buenas tasas de acierto:

```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"encoding/json"

	"github.com/gofhir/fhirpath"
)

// CacheStatsHandler exposes expression cache metrics.
func CacheStatsHandler(w http.ResponseWriter, r *http.Request) {
	stats := fhirpath.DefaultCache.Stats()

	response := map[string]interface{}{
		"size":     stats.Size,
		"limit":    stats.Limit,
		"hits":     stats.Hits,
		"misses":   stats.Misses,
		"hit_rate": fhirpath.DefaultCache.HitRate(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	// Simulate some cache usage
	patient := []byte(`{"resourceType":"Patient","name":[{"family":"Doe"}]}`)

	for i := 0; i < 100; i++ {
		fhirpath.EvaluateCached(patient, "Patient.name.family")
	}
	for i := 0; i < 50; i++ {
		fhirpath.EvaluateCached(patient, "Patient.id")
	}

	stats := fhirpath.DefaultCache.Stats()
	fmt.Printf("Cache size: %d / %d\n", stats.Size, stats.Limit)
	fmt.Printf("Hits: %d, Misses: %d\n", stats.Hits, stats.Misses)
	fmt.Printf("Hit rate: %.1f%%\n", fhirpath.DefaultCache.HitRate())

	// Expose as an HTTP endpoint
	http.HandleFunc("/metrics/cache", CacheStatsHandler)
	log.Println("Cache metrics at :8080/metrics/cache")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## Resumen de Mejores Prácticas

| Práctica | Por Qué |
|----------|---------|
| Usar `EvaluateCached` en manejadores de solicitudes | Evita recompilar la misma expresión en cada solicitud |
| Usar `Compile` + `Expression.Evaluate` en trabajos por lotes | Compilar una vez, evaluar muchas veces para máximo rendimiento |
| Validar expresiones proporcionadas por usuarios antes de almacenarlas | Detecta errores de sintaxis temprano, antes de que causen fallas en tiempo de ejecución |
| Registrar errores de evaluación pero no abortar | Los datos FHIR son inherentemente variables; los campos ausentes son normales |
| Monitorear `DefaultCache.Stats()` | Asegura que la caché tenga el tamaño correcto para su carga de trabajo |
| Usar workers concurrentes con un semáforo | Aprovecha la concurrencia de Go sin sobrecargar la CPU |
| Separar errores de compilación de errores de evaluación | Proporciona diagnósticos más claros a operadores y usuarios |
