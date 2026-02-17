---
title: "Caché de Expresiones"
linkTitle: "Cache"
weight: 5
description: >
  Caché LRU seguro para hilos de expresiones FHIRPath compiladas con monitoreo.
---

Compilar una expresión FHIRPath implica análisis léxico y sintáctico. Cuando las mismas expresiones se evalúan repetidamente (por ejemplo, en un handler HTTP o pipeline de datos), almacenar en caché la forma compilada evita trabajo redundante. El `ExpressionCache` proporciona un caché seguro para hilos, con desalojo LRU y estadísticas integradas.

## ExpressionCache

```go
type ExpressionCache struct {
    // unexported fields
}
```

`ExpressionCache` almacena objetos `*Expression` compilados indexados por su cadena fuente. Utiliza desalojo LRU (Least Recently Used) cuando el caché alcanza su límite de tamaño. Todos los métodos son seguros para uso concurrente.

### NewExpressionCache

Crea un nuevo caché con el número máximo dado de entradas. Si `limit` es 0 o negativo, el caché crece sin límite (no recomendado para producción).

```go
func NewExpressionCache(limit int) *ExpressionCache
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `limit` | `int` | Número máximo de expresiones en caché. Use 0 para sin límite. |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `*ExpressionCache` | Un nuevo caché vacío |

**Ejemplo:**

```go
// Create a cache that holds up to 500 compiled expressions
cache := fhirpath.NewExpressionCache(500)
```

---

### ExpressionCache.Get

Recupera una expresión compilada del caché. Si la expresión no está en caché, se compila, almacena y retorna. En caso de acierto de caché, la entrada se promueve al frente de la lista LRU.

```go
func (c *ExpressionCache) Get(expr string) (*Expression, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `expr` | `string` | Una cadena de expresión FHIRPath |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `*Expression` | La expresión compilada (del caché o recién compilada) |
| `error` | No nulo si la expresión es sintácticamente inválida |

**Ejemplo:**

```go
cache := fhirpath.NewExpressionCache(100)

// First call: compiles and caches
expr, err := cache.Get("Patient.name.family")
if err != nil {
    log.Fatal(err)
}

// Second call: returns from cache (no compilation)
expr2, err := cache.Get("Patient.name.family")
if err != nil {
    log.Fatal(err)
}
// expr and expr2 point to the same *Expression
```

---

### ExpressionCache.MustGet

Similar a `Get`, pero genera un panic en caso de error.

```go
func (c *ExpressionCache) MustGet(expr string) *Expression
```

**Genera panic** si la expresión es sintácticamente inválida.

---

### ExpressionCache.Clear

Elimina todas las entradas del caché y restablece los contadores de aciertos/fallos a cero.

```go
func (c *ExpressionCache) Clear()
```

**Ejemplo:**

```go
cache.Clear()
fmt.Println(cache.Size()) // 0
```

---

### ExpressionCache.Size

Retorna el número actual de expresiones en caché.

```go
func (c *ExpressionCache) Size() int
```

---

### ExpressionCache.Stats

Retorna una instantánea de las estadísticas de rendimiento del caché.

```go
func (c *ExpressionCache) Stats() CacheStats
```

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `CacheStats` | Un struct que contiene tamaño, límite, conteo de aciertos y conteo de fallos |

---

### ExpressionCache.HitRate

Retorna la tasa de aciertos del caché como un porcentaje entre 0 y 100. Retorna 0 si no se han realizado consultas.

```go
func (c *ExpressionCache) HitRate() float64
```

**Ejemplo:**

```go
fmt.Printf("Cache hit rate: %.1f%%\n", cache.HitRate())
```

---

## CacheStats

Una instantánea de las métricas de rendimiento del caché.

```go
type CacheStats struct {
    Size   int   // Current number of cached entries
    Limit  int   // Maximum cache capacity
    Hits   int64 // Total number of cache hits
    Misses int64 // Total number of cache misses
}
```

**Ejemplo:**

```go
stats := cache.Stats()
fmt.Printf("Cache: %d/%d entries, %d hits, %d misses\n",
    stats.Size, stats.Limit, stats.Hits, stats.Misses)
```

---

## DefaultCache

El paquete proporciona un caché global preconfigurado con un límite de 1000 entradas. Funciones como `EvaluateCached`, `GetCached` y `MustGetCached` utilizan este caché.

```go
var DefaultCache = NewExpressionCache(1000)
```

---

## GetCached

Recupera o compila una expresión utilizando `DefaultCache`. Es un envoltorio de conveniencia alrededor de `DefaultCache.Get(expr)`.

```go
func GetCached(expr string) (*Expression, error)
```

**Ejemplo:**

```go
expr, err := fhirpath.GetCached("Patient.name.family")
if err != nil {
    log.Fatal(err)
}
result, err := expr.Evaluate(patientJSON)
```

---

## MustGetCached

Similar a `GetCached`, pero genera un panic en caso de error. Envuelve `DefaultCache.MustGet(expr)`.

```go
func MustGetCached(expr string) *Expression
```

---

## Monitoreo y Observabilidad

### Exponer Métricas del Caché

Se pueden registrar o exportar periódicamente las estadísticas del caché al sistema de monitoreo:

```go
func logCacheStats() {
    stats := fhirpath.DefaultCache.Stats()
    log.Printf("FHIRPath cache: size=%d/%d hits=%d misses=%d hitRate=%.1f%%",
        stats.Size, stats.Limit, stats.Hits, stats.Misses,
        fhirpath.DefaultCache.HitRate())
}
```

### Ejemplo de Métricas Prometheus

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/gofhir/fhirpath"
)

var (
    cacheSize = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
        Name: "fhirpath_cache_size",
        Help: "Number of cached FHIRPath expressions",
    }, func() float64 {
        return float64(fhirpath.DefaultCache.Size())
    })

    cacheHitRate = prometheus.NewGaugeFunc(prometheus.GaugeOpts{
        Name: "fhirpath_cache_hit_rate",
        Help: "FHIRPath expression cache hit rate (0-100)",
    }, func() float64 {
        return fhirpath.DefaultCache.HitRate()
    })
)

func init() {
    prometheus.MustRegister(cacheSize, cacheHitRate)
}
```

### Endpoint de Estado de Salud

```go
func cacheHealthHandler(w http.ResponseWriter, r *http.Request) {
    stats := fhirpath.DefaultCache.Stats()
    hitRate := fhirpath.DefaultCache.HitRate()

    response := map[string]interface{}{
        "size":     stats.Size,
        "limit":    stats.Limit,
        "hits":     stats.Hits,
        "misses":   stats.Misses,
        "hit_rate": hitRate,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
```

---

## Uso de un Caché Personalizado

Para aplicaciones que necesitan múltiples cachés aislados o diferentes límites de tamaño:

```go
// Separate caches for different workloads
var (
    validationCache = fhirpath.NewExpressionCache(200)
    extractionCache = fhirpath.NewExpressionCache(500)
)

func validateResource(resource []byte) error {
    expr, err := validationCache.Get("Patient.name.exists()")
    if err != nil {
        return err
    }
    result, err := expr.Evaluate(resource)
    if err != nil {
        return err
    }
    if result.Empty() {
        return fmt.Errorf("patient must have a name")
    }
    return nil
}

func extractFamilyName(resource []byte) (string, error) {
    expr, err := extractionCache.Get("Patient.name.first().family")
    if err != nil {
        return "", err
    }
    result, err := expr.Evaluate(resource)
    if err != nil {
        return "", err
    }
    if first, ok := result.First(); ok {
        return first.String(), nil
    }
    return "", nil
}
```

---

## Comportamiento de Desalojo LRU

Cuando el caché está lleno y se compila una nueva expresión:

1. La entrada **menos recientemente usada** (la que ha pasado más tiempo sin una llamada `Get`) se desaloja.
2. La nueva entrada se coloca al frente de la lista LRU.
3. Cada llamada a `Get` (acierto o fallo) actualiza la posición de la entrada en la lista LRU.

Esto significa que las expresiones usadas frecuentemente permanecen en el caché mientras que las raramente usadas se desalojan primero. Si la aplicación tiene un conjunto estable de expresiones más pequeño que el límite del caché, el caché eventualmente alcanzará una tasa de aciertos del 100%.
