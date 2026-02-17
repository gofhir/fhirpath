---
title: "Funciones de Evaluacion"
linkTitle: "Evaluate"
weight: 1
description: >
  Evaluacion directa de expresiones FHIRPath contra recursos FHIR en formato JSON.
---

Las funciones de evaluacion son la forma mas sencilla de ejecutar una expresion FHIRPath. Aceptan bytes JSON crudos y una cadena de expresion, y retornan una `Collection` de resultados. Elija la variante que mejor se adapte a sus necesidades.

## Evaluate

Analiza y evalua una expresion FHIRPath contra un recurso JSON en una sola llamada. La expresion se compila en cada llamada, por lo que es mas adecuada para evaluaciones puntuales.

```go
func Evaluate(resource []byte, expr string) (Collection, error)
```

**Parametros:**

| Nombre | Tipo | Descripcion |
|--------|------|-------------|
| `resource` | `[]byte` | Bytes JSON crudos de un recurso FHIR |
| `expr` | `string` | Una expresion FHIRPath a evaluar |

**Retorna:**

| Tipo | Descripcion |
|------|-------------|
| `Collection` | Una secuencia ordenada de valores FHIRPath (alias de `types.Collection`) |
| `error` | No nulo si la expresion es invalida o la evaluacion falla |

**Ejemplo:**

```go
package main

import (
    "fmt"
    "log"

    "github.com/gofhir/fhirpath"
)

func main() {
    patient := []byte(`{
        "resourceType": "Patient",
        "name": [{"family": "Smith", "given": ["John", "Jacob"]}],
        "birthDate": "1990-01-15"
    }`)

    // Extraer el apellido
    result, err := fhirpath.Evaluate(patient, "Patient.name.family")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result) // [Smith]

    // Usar una expresion mas compleja
    result, err = fhirpath.Evaluate(patient, "Patient.name.given.count()")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result) // [2]
}
```

{{% alert title="Nota de Rendimiento" color="warning" %}}
`Evaluate` compila la expresion en cada llamada. Si evalua la misma expresion repetidamente, use `EvaluateCached` o precompile con `Compile` en su lugar.
{{% /alert %}}

---

## MustEvaluate

Similar a `Evaluate`, pero genera un panic en lugar de retornar un error. Uselo en pruebas o codigo de inicializacion donde un fallo es irrecuperable.

```go
func MustEvaluate(resource []byte, expr string) Collection
```

**Parametros:**

| Nombre | Tipo | Descripcion |
|--------|------|-------------|
| `resource` | `[]byte` | Bytes JSON crudos de un recurso FHIR |
| `expr` | `string` | Una expresion FHIRPath a evaluar |

**Retorna:**

| Tipo | Descripcion |
|------|-------------|
| `Collection` | Una secuencia ordenada de valores FHIRPath |

**Genera panic** si la expresion es invalida o la evaluacion falla.

**Ejemplo:**

```go
// En una prueba
func TestPatientName(t *testing.T) {
    patient := []byte(`{"resourceType": "Patient", "name": [{"family": "Doe"}]}`)
    result := fhirpath.MustEvaluate(patient, "Patient.name.family")

    if result.Count() != 1 {
        t.Errorf("expected 1 name, got %d", result.Count())
    }
}
```

---

## EvaluateCached

Compila (con cache automatico) y evalua una expresion FHIRPath. Las llamadas posteriores con la misma cadena de expresion omiten la compilacion por completo y reutilizan el arbol de analisis en cache. Esta es la **funcion recomendada para uso en produccion**.

```go
func EvaluateCached(resource []byte, expr string) (Collection, error)
```

**Parametros:**

| Nombre | Tipo | Descripcion |
|--------|------|-------------|
| `resource` | `[]byte` | Bytes JSON crudos de un recurso FHIR |
| `expr` | `string` | Una expresion FHIRPath a evaluar |

**Retorna:**

| Tipo | Descripcion |
|------|-------------|
| `Collection` | Una secuencia ordenada de valores FHIRPath |
| `error` | No nulo si la expresion es invalida o la evaluacion falla |

`EvaluateCached` utiliza el `DefaultCache` a nivel de paquete (un cache LRU con un limite de 1000 entradas). Para configuraciones de cache personalizadas, cree su propio `ExpressionCache`.

**Ejemplo:**

```go
func extractNames(patients [][]byte) ([]string, error) {
    var names []string
    for _, p := range patients {
        // La expresion se compila solo en la primera llamada;
        // las iteraciones posteriores reutilizan la compilacion en cache.
        result, err := fhirpath.EvaluateCached(p, "Patient.name.family")
        if err != nil {
            return nil, err
        }
        if first, ok := result.First(); ok {
            names = append(names, first.String())
        }
    }
    return names, nil
}
```

---

## Cuando Usar Cada Funcion

| Funcion | Compilacion | Panic | Ideal Para |
|---------|-------------|-------|------------|
| `Evaluate` | Cada llamada | No | Evaluaciones puntuales, scripts, trabajo exploratorio |
| `MustEvaluate` | Cada llamada | Si | Pruebas, codigo de inicializacion, expresiones garantizadas como validas |
| `EvaluateCached` | Una vez (en cache) | No | Cargas de trabajo en produccion, bucles, handlers HTTP |

Para aun mas control, vea [Compilacion y Expression](../compile/) para precompilar expresiones, o [Cache de Expresiones](../cache/) para gestionar el tamano del cache y monitorear las tasas de acierto.

## Manejo de Errores

Todas las funciones que no son `Must` retornan un `error` como segundo valor. Los errores se dividen en dos categorias:

1. **Errores de compilacion** -- La cadena de expresion es sintacticamente invalida.
2. **Errores de evaluacion** -- La expresion es valida pero falla en tiempo de ejecucion (por ejemplo, incompatibilidad de tipos, division por cero).

```go
result, err := fhirpath.Evaluate(resource, "Patient.name.family")
if err != nil {
    // Manejar error: verifique err.Error() para mas detalles
    log.Printf("La evaluacion de FHIRPath fallo: %v", err)
    return
}
// Usar result de forma segura
```
