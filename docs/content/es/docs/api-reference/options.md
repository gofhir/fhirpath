---
title: "Opciones de Evaluación"
linkTitle: "Options"
weight: 6
description: >
  Configurar el comportamiento de evaluación con timeouts, límites de profundidad, variables y resolución de referencias.
---

La API de opciones permite personalizar cómo se evalúan las expresiones FHIRPath. Las opciones se aplican utilizando el patrón de opciones funcionales de Go, brindando control granular sobre timeouts, límites de recursión, variables externas y resolución de referencias.

## EvalOptions

El struct `EvalOptions` contiene toda la configuración para una ejecución de evaluación. Raramente se construye directamente; en su lugar, utilice las funciones de opciones funcionales para construir opciones de forma incremental.

```go
type EvalOptions struct {
    // Ctx is the context for cancellation and timeout.
    Ctx context.Context

    // Timeout for evaluation (0 means no timeout).
    Timeout time.Duration

    // MaxDepth limits recursion depth for descendants() (0 means default of 100).
    MaxDepth int

    // MaxCollectionSize limits output collection size (0 means no limit).
    MaxCollectionSize int

    // Variables are external variables accessible via %name in expressions.
    Variables map[string]types.Collection

    // Resolver handles reference resolution for the resolve() function.
    Resolver ReferenceResolver
}
```

---

## DefaultOptions

Retorna un nuevo `EvalOptions` preconfigurado con valores por defecto sensatos para producción.

```go
func DefaultOptions() *EvalOptions
```

**Valores por defecto:**

| Campo | Valor por Defecto |
|-------|-------------------|
| `Ctx` | `context.Background()` |
| `Timeout` | `5 * time.Second` |
| `MaxDepth` | `100` |
| `MaxCollectionSize` | `10000` |
| `Variables` | Mapa vacío |
| `Resolver` | `nil` |

Cuando se llama a `Expression.EvaluateWithOptions` sin ninguna opción, se utilizan estos valores por defecto.

---

## Opciones Funcionales

Cada opción funcional es una función de tipo `EvalOption` que modifica `EvalOptions`:

```go
type EvalOption func(*EvalOptions)
```

### WithContext

Establece el `context.Context` para soporte de cancelación. Si el contexto se cancela durante la evaluación, la evaluación retorna inmediatamente con el error del contexto.

```go
func WithContext(ctx context.Context) EvalOption
```

**Ejemplo:**

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithContext(ctx),
)
```

---

### WithTimeout

Establece el tiempo máximo permitido para una única evaluación. Si la evaluación toma más tiempo que la duración especificada, se cancela y se retorna un error. Un valor de cero deshabilita el timeout.

```go
func WithTimeout(d time.Duration) EvalOption
```

**Ejemplo:**

```go
// Allow at most 2 seconds for evaluation
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithTimeout(2*time.Second),
)
```

{{% alert title="Nota" color="info" %}}
Cuando se proporcionan tanto `WithContext` como `WithTimeout`, la fecha límite efectiva es la más temprana de las dos. `WithTimeout` internamente crea un contexto hijo con el timeout dado.
{{% /alert %}}

---

### WithMaxDepth

Establece la profundidad máxima de recursión para funciones como `descendants()` que recorren el árbol del recurso. Esto previene desbordamientos de pila en estructuras profundamente anidadas o circulares. El valor por defecto es 100.

```go
func WithMaxDepth(depth int) EvalOption
```

**Ejemplo:**

```go
// Limit recursion to 50 levels
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithMaxDepth(50),
)
```

---

### WithMaxCollectionSize

Establece el número máximo de valores permitidos en una colección de resultados. Esto previene el uso excesivo de memoria cuando las expresiones producen conjuntos de resultados muy grandes. El valor por defecto es 10000.

```go
func WithMaxCollectionSize(size int) EvalOption
```

**Ejemplo:**

```go
// Limit results to 1000 values
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithMaxCollectionSize(1000),
)
```

---

### WithVariable

Define una variable externa que puede ser referenciada en la expresión FHIRPath usando la sintaxis `%nombre`. Se pueden encadenar múltiples llamadas a `WithVariable`.

```go
func WithVariable(name string, value types.Collection) EvalOption
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `name` | `string` | Nombre de la variable (referenciada como `%name` en las expresiones) |
| `value` | `types.Collection` | El valor de la variable como una Collection |

**Ejemplo:**

```go
import "github.com/gofhir/fhirpath/types"

// Define a variable %maxAge that can be used in the expression
expr := fhirpath.MustCompile("Patient.birthDate < today() - %maxAge")

result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithVariable("maxAge", types.Collection{types.NewString("65 years")}),
)
```

---

### WithResolver

Proporciona una implementación de `ReferenceResolver` para la función `resolve()` de FHIRPath. Cuando una expresión llama a `resolve()` sobre una referencia FHIR, el resolver obtiene el recurso referenciado.

```go
func WithResolver(r ReferenceResolver) EvalOption
```

**Ejemplo:**

```go
result, err := expr.EvaluateWithOptions(resource,
    fhirpath.WithResolver(myResolver),
)
```

Consulte [Interfaz ReferenceResolver](#interfaz-referenceresolver) más abajo para más detalles.

---

## Interfaz ReferenceResolver

La interfaz `ReferenceResolver` es implementada por la aplicación para proporcionar resolución de referencias para la función `resolve()` de FHIRPath. Cuando una expresión evalúa `Reference.resolve()`, la biblioteca llama al resolver con la cadena de referencia.

```go
type ReferenceResolver interface {
    Resolve(ctx context.Context, reference string) ([]byte, error)
}
```

**Parámetros pasados a Resolve:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `ctx` | `context.Context` | El contexto de evaluación (respeta cancelación/timeout) |
| `reference` | `string` | La cadena de referencia FHIR (por ejemplo, `"Patient/123"`, `"http://example.com/fhir/Patient/123"`) |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `[]byte` | Bytes JSON crudos del recurso referenciado |
| `error` | No nulo si la referencia no puede ser resuelta |

### Ejemplo de Implementación: Resolver HTTP

```go
type HTTPResolver struct {
    BaseURL    string
    HTTPClient *http.Client
}

func (r *HTTPResolver) Resolve(ctx context.Context, reference string) ([]byte, error) {
    url := r.BaseURL + "/" + reference

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("creating request: %w", err)
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := r.HTTPClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("fetching %s: %w", reference, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, reference)
    }

    return io.ReadAll(resp.Body)
}
```

### Ejemplo de Implementación: Resolver de Bundle en Memoria

```go
type BundleResolver struct {
    resources map[string][]byte
}

func NewBundleResolver(bundle []byte) (*BundleResolver, error) {
    resolver := &BundleResolver{
        resources: make(map[string][]byte),
    }

    // Parse bundle and index resources by their reference key
    // (e.g., "Patient/123")
    var b struct {
        Entry []struct {
            Resource json.RawMessage `json:"resource"`
        } `json:"entry"`
    }
    if err := json.Unmarshal(bundle, &b); err != nil {
        return nil, err
    }

    for _, entry := range b.Entry {
        var meta struct {
            ResourceType string `json:"resourceType"`
            ID           string `json:"id"`
        }
        if err := json.Unmarshal(entry.Resource, &meta); err == nil {
            key := meta.ResourceType + "/" + meta.ID
            resolver.resources[key] = entry.Resource
        }
    }

    return resolver, nil
}

func (r *BundleResolver) Resolve(ctx context.Context, reference string) ([]byte, error) {
    if data, ok := r.resources[reference]; ok {
        return data, nil
    }
    return nil, fmt.Errorf("resource not found: %s", reference)
}
```

---

## Combinación de Múltiples Opciones

Las opciones son variádicas, por lo que se pueden pasar tantas como se necesite. Se aplican en orden, con las opciones posteriores sobreescribiendo las anteriores para el mismo campo.

```go
expr := fhirpath.MustCompile(
    "Observation.subject.resolve().name.family",
)

resolver := &HTTPResolver{
    BaseURL:    "https://fhir.example.com",
    HTTPClient: &http.Client{Timeout: 10 * time.Second},
}

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := expr.EvaluateWithOptions(observationJSON,
    fhirpath.WithContext(ctx),
    fhirpath.WithTimeout(10*time.Second),
    fhirpath.WithMaxDepth(50),
    fhirpath.WithMaxCollectionSize(500),
    fhirpath.WithVariable("today", types.Collection{types.NewString("2025-01-15")}),
    fhirpath.WithResolver(resolver),
)
if err != nil {
    log.Fatal(err)
}
fmt.Println(result)
```

---

## Resumen de Opciones

| Opción | Valor por Defecto | Descripción |
|--------|-------------------|-------------|
| `WithContext` | `context.Background()` | Establece el contexto de cancelación |
| `WithTimeout` | `5s` | Tiempo máximo de evaluación |
| `WithMaxDepth` | `100` | Profundidad máxima de recursión para `descendants()` |
| `WithMaxCollectionSize` | `10000` | Tamaño máximo de la colección de resultados |
| `WithVariable` | Ninguno | Define variables externas accesibles vía `%name` |
| `WithResolver` | `nil` | Proporciona resolución de referencias para `resolve()` |
