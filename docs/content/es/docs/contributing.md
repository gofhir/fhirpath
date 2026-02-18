---
title: "Contribuir"
linkTitle: "Contribuir"
weight: 99
description: >
  Cómo configurar el entorno de desarrollo, ejecutar pruebas y benchmarks, agregar nuevas funciones y enviar cambios a la biblioteca FHIRPath de Go.
---

Gracias por su interés en contribuir a FHIRPath Go! Esta guía cubre todo lo que necesita para comenzar, desde la configuración de su entorno local hasta el envío de un pull request.

## Configuración del Entorno de Desarrollo

### Prerequisitos

- **Go 1.23 o posterior** -- la versión mínima especificada en `go.mod`
- **Git**
- (Opcional) **golangci-lint** para ejecutar el linter localmente

### Clonar el Repositorio

```bash
git clone https://github.com/gofhir/fhirpath.git
cd fhirpath
```

### Instalar Dependencias

```bash
go mod download
```

### Verificar que Todo Funciona

```bash
go test -v -race ./...
```

Si todas las pruebas pasan, está listo para comenzar a desarrollar.

## Estructura del Proyecto

El repositorio está organizado en los siguientes paquetes:

```text
fhirpath/
  fhirpath.go          # Top-level API: Evaluate, MustEvaluate
  compiler.go          # Expression compilation (Compile, MustCompile)
  expression.go        # Expression type and Evaluate/EvaluateWithContext
  resource.go          # Resource interface, typed helpers (EvaluateToBoolean, etc.)
  cache.go             # ExpressionCache with LRU eviction
  options.go           # EvalOptions, functional options, ReferenceResolver
  eval/
    evaluator.go       # Core evaluation engine (tree walker)
    operators.go       # Operator implementations (+, -, =, >, and, or, etc.)
    errors.go          # Evaluation error types
  funcs/
    registry.go        # Function registry (Register, GetRegistry)
    existence.go       # exists(), empty(), count(), distinct(), all(), etc.
    filtering.go       # where(), select(), repeat(), ofType()
    subsetting.go      # first(), last(), tail(), skip(), take()
    strings.go         # startsWith(), endsWith(), contains(), replace(), etc.
    math.go            # abs(), ceiling(), floor(), ln(), log(), power(), etc.
    typechecking.go    # is(), as(), ofType() type-checking functions
    conversion.go      # toBoolean(), toInteger(), toDecimal(), toString(), etc.
    temporal.go        # now(), today(), dateTime arithmetic
    aggregate.go       # aggregate() function
    utility.go         # trace(), iif(), and other utility functions
    regex.go           # matches(), replaceMatches()
    fhir.go            # FHIR-specific: extension(), resolve(), memberOf(), etc.
  types/
    value.go           # Value interface
    collection.go      # Collection type and methods
    boolean.go         # Boolean type
    integer.go         # Integer type
    decimal.go         # Decimal type (uses shopspring/decimal)
    string.go          # String type
    date.go            # Date type with partial precision
    datetime.go        # DateTime type with partial precision
    time.go            # Time type
    quantity.go        # Quantity type with UCUM support
    object.go          # ObjectValue for JSON objects
    pool.go            # Object pooling for memory efficiency
    errors.go          # Type-system error types
  parser/
    grammar/           # ANTLR-generated lexer, parser, and visitor
  internal/
    ucum/
      ucum.go          # UCUM unit normalization and conversion
```

### Decisiones de Diseño Clave

- **Sin dependencia de modelos FHIR®.** La biblioteca trabaja directamente con bytes JSON crudos vía `github.com/buger/jsonparser`. Esto mantiene el árbol de dependencias pequeño y permite a los usuarios usar cualquier biblioteca de modelos FHIR® (o ninguna).
- **Parser generado por ANTLR.** La gramática FHIRPath se analiza con `antlr4-go`. Los archivos de gramática están en `parser/grammar/`. No edite los archivos Go generados directamente; regenere los desde el archivo `.g4` de gramática si es necesario.
- **Decimales de precisión arbitraria.** Los valores decimales usan `github.com/shopspring/decimal` para evitar sorpresas de punto flotante.

## Ejecución de Pruebas

### Suite Completa de Pruebas

```bash
go test -v -race ./...
```

La bandera `-race` habilita el detector de carreras de Go, lo cual es importante porque la biblioteca está diseñada para uso concurrente.

### Un Solo Paquete

```bash
go test -v -race ./funcs/
go test -v -race ./eval/
go test -v -race ./types/
```

### Una Sola Prueba

```bash
go test -v -race -run TestEvaluateToBoolean ./...
```

## Ejecución de Benchmarks

Los benchmarks de rendimiento están junto a las pruebas en archivos `*_bench_test.go`.

```bash
go test -bench=. -benchmem ./...
```

Para ejecutar benchmarks solo del paquete principal:

```bash
go test -bench=. -benchmem -benchtime=5s .
```

Compare antes y después de su cambio:

```bash
# Before
go test -bench=. -benchmem -count=6 . > old.txt

# Make your changes, then:
go test -bench=. -benchmem -count=6 . > new.txt

# Compare (requires golang.org/x/perf/cmd/benchstat)
benchstat old.txt new.txt
```

## Linting

El proyecto usa [golangci-lint](https://golangci-lint.run/) para análisis estático:

```bash
golangci-lint run
```

Instálelo con:

```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

La configuración del linter está en `.golangci.yml` en la raíz del repositorio. Asegúrese de que `golangci-lint run` pase limpiamente antes de enviar un pull request.

## Agregar una Nueva Función

El registro de funciones en `funcs/` facilita agregar nuevas funciones FHIRPath. Siga estos pasos:

### 1. Elegir el Archivo Correcto

Coloque su función en el archivo que coincida con su categoría:

| Categoría | Archivo |
|-----------|---------|
| Existencia / conteo | `funcs/existence.go` |
| Filtrado / proyección | `funcs/filtering.go` |
| Subconjuntos (first, last, etc.) | `funcs/subsetting.go` |
| Manipulación de cadenas | `funcs/strings.go` |
| Matemáticas | `funcs/math.go` |
| Verificación / conversión de tipos | `funcs/typechecking.go` o `funcs/conversion.go` |
| Fecha / hora | `funcs/temporal.go` |
| Agregación | `funcs/aggregate.go` |
| Específicas de FHIR® | `funcs/fhir.go` |
| Expresiones regulares | `funcs/regex.go` |
| Utilidades | `funcs/utility.go` |

### 2. Implementar la Función

Toda función tiene la firma:

```go
func fnMyFunction(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error)
```

- `ctx` -- el contexto de evaluación (acceso al recurso, variables, límites)
- `input` -- la colección sobre la que se invoca la función (lado izquierdo del punto)
- `args` -- los argumentos pasados a la función

Esqueleto de ejemplo:

```go
func fnMyFunction(ctx *eval.Context, input types.Collection, args []interface{}) (types.Collection, error) {
    if input.Empty() {
        return types.Collection{}, nil
    }

    // Implement your logic here

    return result, nil
}
```

### 3. Registrar la Función

En el bloque `init()` del mismo archivo, registre su función en el registro:

```go
func init() {
    // ... existing registrations ...

    Register(FuncDef{
        Name:    "myFunction",      // the name used in FHIRPath expressions
        MinArgs: 0,                 // minimum number of arguments
        MaxArgs: 1,                 // maximum number of arguments
        Fn:      fnMyFunction,
    })
}
```

### 4. Escribir Pruebas

Agregue pruebas en el archivo `*_test.go` correspondiente. Pruebe como mínimo:

- Caso normal con entrada esperada
- Entrada vacía (debe retornar colección vacía)
- Casos límite (valores nulos, tipos incorrectos, valores frontera)
- Casos de error (número incorrecto de argumentos, incompatibilidades de tipo)

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name     string
        resource string
        expr     string
        want     string
    }{
        {
            name:     "basic case",
            resource: `{"resourceType": "Patient", "id": "1"}`,
            expr:     "Patient.id.myFunction()",
            want:     "expected-value",
        },
        // ... more test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := fhirpath.Evaluate([]byte(tt.resource), tt.expr)
            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }
            // Assert result matches tt.want
        })
    }
}
```

### 5. Documentar la Función

Si la función es parte de la especificación FHIRPath, haga referencia a la sección de la especificación. Si es una función personalizada específica de esta biblioteca, documéntela claramente en el comentario de documentación de la función.

## Estilo de Código

- Siga las convenciones ya presentes en el código base.
- Ejecute `gofmt` (o `goimports`) antes de hacer commit.
- Asegúrese de que `golangci-lint run` pase sin advertencias.
- Mantenga las funciones enfocadas. Si una función crece más de ~50 líneas, considere extraer funciones auxiliares.
- Escriba comentarios de documentación Go claros en todos los símbolos exportados.
- Prefiera retornar `(types.Collection, error)` en lugar de hacer panic.
- Maneje las colecciones vacías explícitamente -- retornar una colección vacía es generalmente el comportamiento correcto según la especificación FHIRPath.

## Envío de Cambios

### 1. Hacer Fork del Repositorio

Haga fork de `github.com/gofhir/fhirpath` a su propia cuenta de GitHub.

### 2. Crear una Rama de Funcionalidad

```bash
git checkout -b feature/my-new-function
```

Use un nombre de rama descriptivo:

- `feature/add-encode-function` para nuevas funcionalidades
- `fix/where-empty-collection` para correcciones de errores
- `docs/update-readme` para cambios en la documentación

### 3. Realizar Sus Cambios

Haga commits en unidades pequeñas y lógicas. Cada commit debe compilar y pasar las pruebas.

### 4. Ejecutar la Suite Completa

```bash
go test -v -race ./...
golangci-lint run
```

### 5. Hacer Push y Abrir un Pull Request

```bash
git push origin feature/my-new-function
```

Abra un pull request contra `main`. En la descripción del PR:

- Describa **qué** cambió y **por qué**.
- Haga referencia a issues relacionados (por ejemplo, `Fixes #42`).
- Si agregó una nueva función, incluya un ejemplo de uso.
- Si cambió código sensible al rendimiento, incluya resultados de benchmarks.

### 6. Responder a la Revisión

Un mantenedor revisará su PR. Sea receptivo a los comentarios y haga push de commits de seguimiento a la misma rama.

## Licencia

FHIRPath Go se publica bajo la [Licencia MIT](https://github.com/gofhir/fhirpath/blob/main/LICENSE). Al contribuir, acepta que sus contribuciones se licenciarán bajo los mismos términos.
