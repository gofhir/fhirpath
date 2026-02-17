---
title: "Compilación y Expression"
linkTitle: "Compile"
weight: 2
description: >
  Pre-compilar expresiones FHIRPath para una evaluación repetida eficiente.
---

Las funciones de compilación analizan una expresión FHIRPath una sola vez y retornan un objeto `Expression` que puede ser evaluado múltiples veces contra diferentes recursos. Este es el patrón "compilar una vez, evaluar muchas" y ofrece el mejor rendimiento para rutas críticas.

## Compile

Analiza una cadena de expresión FHIRPath y retorna una `Expression` compilada. Retorna un error si la expresión es sintácticamente inválida.

```go
func Compile(expr string) (*Expression, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `expr` | `string` | Una expresión FHIRPath a compilar |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `*Expression` | Un objeto de expresión compilado y reutilizable |
| `error` | No nulo si la expresión tiene errores de sintaxis |

**Ejemplo:**

```go
expr, err := fhirpath.Compile("Patient.name.where(use = 'official').family")
if err != nil {
    log.Fatalf("invalid expression: %v", err)
}

// Use expr.Evaluate() against many resources
for _, patient := range patients {
    result, err := expr.Evaluate(patient)
    if err != nil {
        log.Printf("evaluation error: %v", err)
        continue
    }
    fmt.Println(result)
}
```

---

## MustCompile

Similar a `Compile`, pero genera un panic en caso de error. Ideal para variables a nivel de paquete o inicialización donde una expresión incorrecta es un error de programación.

```go
func MustCompile(expr string) *Expression
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `expr` | `string` | Una expresión FHIRPath a compilar |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `*Expression` | Un objeto de expresión compilado y reutilizable |

**Genera panic** si la expresión es sintácticamente inválida.

**Ejemplo:**

```go
// Package-level compiled expressions -- compiled once at startup.
var (
    exprFamilyName = fhirpath.MustCompile("Patient.name.family")
    exprBirthDate  = fhirpath.MustCompile("Patient.birthDate")
    exprActive     = fhirpath.MustCompile("Patient.active")
)

func getPatientInfo(resource []byte) (string, error) {
    result, err := exprFamilyName.Evaluate(resource)
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

## Tipo Expression

`Expression` representa una expresión FHIRPath compilada. Contiene el AST (árbol de sintaxis abstracta) analizado y puede ser evaluada contra cualquier recurso FHIR.

```go
type Expression struct {
    // unexported fields
}
```

### Expression.Evaluate

Ejecuta la expresión compilada contra un recurso FHIR en formato JSON.

```go
func (e *Expression) Evaluate(resource []byte) (Collection, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `resource` | `[]byte` | Bytes JSON crudos de un recurso FHIR |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `Collection` | El resultado de la evaluación |
| `error` | No nulo si la evaluación falla |

**Ejemplo:**

```go
expr := fhirpath.MustCompile("Patient.telecom.where(system = 'phone').value")

patient := []byte(`{
    "resourceType": "Patient",
    "telecom": [
        {"system": "phone", "value": "555-0100"},
        {"system": "email", "value": "john@example.com"}
    ]
}`)

result, err := expr.Evaluate(patient)
if err != nil {
    log.Fatal(err)
}
fmt.Println(result) // [555-0100]
```

---

### Expression.EvaluateWithContext

Ejecuta la expresión con un contexto de evaluación personalizado. Este es un método de nivel inferior que otorga control total sobre el entorno de evaluación.

```go
func (e *Expression) EvaluateWithContext(ctx *eval.Context) (Collection, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `ctx` | `*eval.Context` | Un contexto de evaluación creado por `eval.NewContext` |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `Collection` | El resultado de la evaluación |
| `error` | No nulo si la evaluación falla |

Este método está destinado para casos de uso avanzados donde se necesita acceso directo al contexto de evaluación interno (por ejemplo, establecer variables o límites a un nivel inferior). Para la mayoría de los casos, es preferible utilizar `EvaluateWithOptions`.

---

### Expression.EvaluateWithOptions

Ejecuta la expresión contra un recurso JSON con opciones configurables. Las opciones se aplican utilizando el patrón de opciones funcionales.

```go
func (e *Expression) EvaluateWithOptions(resource []byte, opts ...EvalOption) (Collection, error)
```

**Parámetros:**

| Nombre | Tipo | Descripción |
|--------|------|-------------|
| `resource` | `[]byte` | Bytes JSON crudos de un recurso FHIR |
| `opts` | `...EvalOption` | Cero o más opciones funcionales |

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `Collection` | El resultado de la evaluación |
| `error` | No nulo si la evaluación falla |

Cuando no se proporcionan opciones, se utilizan los valores de `DefaultOptions()` (timeout de 5 segundos, profundidad máxima 100, tamaño máximo de colección 10000).

**Ejemplo:**

```go
expr := fhirpath.MustCompile("Patient.name.family")

result, err := expr.EvaluateWithOptions(patient,
    fhirpath.WithTimeout(2*time.Second),
    fhirpath.WithMaxDepth(50),
)
if err != nil {
    log.Fatal(err)
}
fmt.Println(result)
```

Consulte [Opciones de Evaluación](../options/) para la lista completa de opciones disponibles.

---

### Expression.String

Retorna la cadena de expresión FHIRPath original que fue compilada.

```go
func (e *Expression) String() string
```

**Retorna:**

| Tipo | Descripción |
|------|-------------|
| `string` | El texto fuente de la expresión original |

**Ejemplo:**

```go
expr := fhirpath.MustCompile("Patient.name.family")
fmt.Println(expr.String()) // Patient.name.family
```

---

## Compilar Una Vez, Evaluar Muchas

El patrón recomendado para código de producción es compilar las expresiones en el momento de inicialización del paquete y reutilizarlas durante toda la vida de la aplicación:

```go
package patient

import "github.com/gofhir/fhirpath"

// Compiled once when the package loads.
var (
    nameExpr   = fhirpath.MustCompile("Patient.name.where(use = 'official').family")
    phoneExpr  = fhirpath.MustCompile("Patient.telecom.where(system = 'phone').value")
    activeExpr = fhirpath.MustCompile("Patient.active")
)

// GetOfficialName evaluates the pre-compiled expression against any Patient resource.
func GetOfficialName(patient []byte) (string, error) {
    result, err := nameExpr.Evaluate(patient)
    if err != nil {
        return "", err
    }
    if first, ok := result.First(); ok {
        return first.String(), nil
    }
    return "", nil
}

// GetPhoneNumbers returns all phone numbers for a Patient.
func GetPhoneNumbers(patient []byte) ([]string, error) {
    result, err := phoneExpr.Evaluate(patient)
    if err != nil {
        return nil, err
    }
    phones := make([]string, 0, result.Count())
    for _, v := range result {
        phones = append(phones, v.String())
    }
    return phones, nil
}
```

Esto evita la sobrecarga de analizar y compilar la expresión en cada llamada, lo cual es especialmente importante en handlers de solicitudes, pipelines de datos y en cualquier lugar donde las expresiones se evalúen con alta frecuencia.
