# Construyendo una Librería FHIRPath en Go: Del Concepto a la Implementación

![Image description](https://dev-to-uploads.s3.amazonaws.com/uploads/articles/k71z5vbqwfwvxvek1zm8.png)

*Cómo diseñé e implementé un motor de evaluación FHIRPath completo en Golang, desde el análisis léxico hasta la ejecución de expresiones*

---

## Introducción

En el ecosistema de la salud digital, la interoperabilidad no es un lujo, es una necesidad. **FHIR®** (Fast Healthcare Interoperability Resources) se ha convertido en el estándar de facto para el intercambio de datos clínicos, y **FHIRPath** es el lenguaje de consulta que nos permite navegar y extraer información de estos recursos de manera precisa y eficiente.

Este artículo documenta mi experiencia construyendo [gofhir](https://github.com/robertoAraneda/gofhir), una implementación completa de FHIRPath en Go. Compartiré las decisiones arquitectónicas, los patrones de diseño utilizados y los desafíos técnicos que enfrenté durante el desarrollo.

---

## ¿Qué es FHIR®?

**[FHIR®](https://hl7.org/fhir/)** (pronunciado "fire") es un estándar desarrollado por HL7 International para el intercambio electrónico de información de salud. A diferencia de sus predecesores (HL7 v2 y v3), FHIR® fue diseñado desde cero con tecnologías web modernas en mente.

### Características Principales de FHIR®

FHIR® se basa en el concepto de **Resources** (Recursos), que representan entidades clínicas y administrativas como Patient, Observation, Medication, Encounter, entre muchas otras. Cada recurso tiene una estructura bien definida que puede ser representada en JSON o XML.

```json
{
  "resourceType": "Patient",
  "id": "example",
  "name": [
    {
      "use": "official",
      "family": "Garcia",
      "given": ["Maria", "Jose"]
    }
  ],
  "birthDate": "1990-05-15",
  "gender": "female",
  "address": [
    {
      "city": "Santiago",
      "country": "Chile"
    }
  ]
}
```

La arquitectura RESTful de FHIR® permite operaciones estándar HTTP (GET, POST, PUT, DELETE) sobre estos recursos, facilitando la integración entre sistemas de salud heterogéneos.

---

## ¿Qué es FHIRPath?

**[FHIRPath](http://hl7.org/fhirpath/N1/)** es un lenguaje de navegación y extracción de datos diseñado específicamente para trabajar con recursos FHIR®. Piensa en él como XPath para XML o JSONPath para JSON, pero optimizado para el modelo de datos de FHIR®.

### Ejemplos de Expresiones FHIRPath

```
// Obtener el apellido de un paciente
Patient.name.family

// Filtrar nombres con uso "official"
Patient.name.where(use = 'official').given

// Verificar si el paciente es mayor de edad
Patient.birthDate <= today() - 18 years

// Obtener el primer elemento de una colección
Patient.address.first().city

// Combinar condiciones
Observation.value.where($this is Quantity and value > 100)
```

El poder de FHIRPath radica en su capacidad para expresar consultas complejas de manera concisa y legible.

---

## Arquitectura de la Librería

### Visión General

La arquitectura de `gofhir/fhirpath` sigue un diseño clásico de compilador/intérprete:

![Image description](https://dev-to-uploads.s3.amazonaws.com/uploads/articles/b7weqs0pn5jt7gjyj0rh.png)

### Principios de Diseño

#### 1. Separación de Responsabilidades

Cada componente tiene una única responsabilidad bien definida:

- **ANTLR Parser**: Análisis léxico, sintáctico y construcción del parse tree
- **Evaluator**: Visitor pattern que recorre el parse tree
- **Funcs**: Implementaciones de funciones FHIRPath
- **Types**: Sistema de tipos que representa valores FHIRPath

#### 2. Inmutabilidad

Las expresiones compiladas son inmutables, permitiendo su reutilización segura en entornos concurrentes:

```go
// Una expresión compilada puede ser reutilizada múltiples veces
expr, err := fhirpath.Compile("Patient.name.given")
if err != nil {
    log.Fatal(err)
}

// Seguro para uso concurrente
for _, patientJSON := range patientsJSON {
    result, _ := expr.Evaluate(patientJSON)
    // procesar resultado
}
```

---

## Implementación del Lexer y Parser con ANTLR

### ¿Por qué ANTLR?

ANTLR (ANother Tool for Language Recognition) es un generador de parsers que produce código en múltiples lenguajes, incluyendo Go. Las razones para elegir ANTLR fueron:

1. **Gramática declarativa**: Define la sintaxis de FHIRPath de forma clara y mantenible
2. **Generación automática**: Produce lexer y parser optimizados
3. **Manejo de errores robusto**: Proporciona mensajes de error precisos
4. **Soporte oficial de Go**: Target estable y bien mantenido

### Definición de la Gramática

La gramática FHIRPath se define en un archivo `.g4`:

```antlr
// FHIRPath.g4
grammar FHIRPath;

// Reglas del Parser
expression
    : term                                          # termExpression
    | expression '.' invocation                     # invocationExpression
    | expression '[' expression ']'                 # indexerExpression
    | ('+' | '-') expression                        # polarityExpression
    | expression ('*' | '/' | 'div' | 'mod') expression  # multiplicativeExpression
    | expression ('+' | '-' | '&') expression       # additiveExpression
    | expression ('|') expression                   # unionExpression
    | expression ('<=' | '<' | '>' | '>=') expression    # inequalityExpression
    | expression ('=' | '~' | '!=' | '!~') expression    # equalityExpression
    | expression ('in' | 'contains') expression     # membershipExpression
    | expression 'and' expression                   # andExpression
    | expression ('or' | 'xor') expression          # orExpression
    | expression 'implies' expression               # impliesExpression
    ;

term
    : invocation                                    # invocationTerm
    | literal                                       # literalTerm
    | externalConstant                              # externalConstantTerm
    | '(' expression ')'                            # parenthesizedTerm
    ;

// Reglas del Lexer
IDENTIFIER
    : ([A-Za-z] | '_') ([A-Za-z0-9] | '_')*
    ;

STRING
    : '\'' (ESC | ~['\\])* '\''
    ;

NUMBER
    : [0-9]+ ('.' [0-9]+)?
    ;
```

### Generación del Código Go

```bash
# Generar lexer y parser en Go
antlr4 -Dlanguage=Go -package grammar -visitor FHIRPath.g4
```

---

## Sistema de Tipos

FHIRPath define un conjunto de tipos primitivos del sistema. En la implementación, definimos una interfaz base `Value`:

```go
// types/value.go
package types

// Value es la interfaz base para todos los valores FHIRPath
type Value interface {
    // Type retorna el nombre del tipo FHIRPath
    Type() string

    // Equal compara igualdad exacta (operador =)
    Equal(other Value) bool

    // Equivalent compara equivalencia (operador ~)
    // Para strings: case-insensitive, ignora espacios
    Equivalent(other Value) bool

    // String retorna representación como string
    String() string

    // IsEmpty indica si el valor representa vacío
    IsEmpty() bool
}

// Comparable es implementado por tipos que soportan ordenamiento
type Comparable interface {
    Value
    // Compare retorna -1 si menor, 0 si igual, 1 si mayor
    Compare(other Value) (int, error)
}

// Numeric es implementado por tipos numéricos (Integer, Decimal)
type Numeric interface {
    Value
    // ToDecimal convierte el valor a Decimal
    ToDecimal() Decimal
}
```

### Tipos Implementados

| Tipo | Descripción | Implementación |
|------|-------------|----------------|
| `Boolean` | Valores true/false | `types.Boolean` |
| `Integer` | Enteros de 64 bits | `types.Integer` (int64) |
| `Decimal` | Precisión arbitraria | `types.Decimal` (shopspring/decimal) |
| `String` | Cadenas UTF-8 | `types.String` |
| `Date` | Fecha (YYYY-MM-DD) | `types.Date` |
| `DateTime` | Fecha y hora ISO 8601 | `types.DateTime` |
| `Time` | Hora (HH:MM:SS) | `types.Time` |
| `Quantity` | Valor + unidad UCUM | `types.Quantity` |

### Implementación de Decimal con Precisión Arbitraria

Para manejar correctamente valores decimales sin pérdida de precisión, utilizamos la librería `shopspring/decimal`:

```go
// types/decimal.go
package types

import (
    "github.com/shopspring/decimal"
)

// Decimal representa un valor decimal FHIRPath con precisión arbitraria
type Decimal struct {
    value decimal.Decimal
}

// NewDecimal crea un Decimal desde un string
func NewDecimal(s string) (Decimal, error) {
    d, err := decimal.NewFromString(s)
    if err != nil {
        return Decimal{}, fmt.Errorf("invalid decimal: %s", s)
    }
    return Decimal{value: d}, nil
}

func (d Decimal) Type() string {
    return "Decimal"
}

func (d Decimal) Equal(other Value) bool {
    switch o := other.(type) {
    case Decimal:
        return d.value.Equal(o.value)
    case Integer:
        return d.value.Equal(decimal.NewFromInt(o.value))
    }
    return false
}

// Operaciones aritméticas
func (d Decimal) Add(other Decimal) Decimal {
    return Decimal{value: d.value.Add(other.value)}
}

func (d Decimal) Divide(other Decimal) (Decimal, error) {
    if other.value.IsZero() {
        return Decimal{}, fmt.Errorf("division by zero")
    }
    return Decimal{value: d.value.DivRound(other.value, 16)}, nil
}
```

---

## Collections: El Corazón de FHIRPath

En FHIRPath, **todas las expresiones operan sobre y retornan colecciones**. Esta es una característica fundamental que simplifica el manejo de valores opcionales y múltiples.

```go
// types/collection.go
package types

// Collection es una secuencia ordenada de valores FHIRPath
type Collection []Value
```

### Métodos Principales de Collection

| Método | Descripción |
|--------|-------------|
| `Empty() bool` | Retorna true si la colección está vacía |
| `Count() int` | Retorna el número de elementos |
| `First() (Value, bool)` | Retorna el primer elemento |
| `Last() (Value, bool)` | Retorna el último elemento |
| `Single() (Value, error)` | Retorna el único elemento (error si != 1) |
| `Contains(v Value) bool` | Verifica si contiene el valor |
| `Distinct() Collection` | Elimina duplicados |
| `Where(predicate) Collection` | Filtra elementos por criterio |

> La implementación incluye 21 métodos en total. Ver [collection.go](https://github.com/robertoAraneda/gofhir/blob/main/pkg/fhirpath/types/collection.go) para la lista completa.

```go
// Ejemplos de implementación:

func (c Collection) Empty() bool {
    return len(c) == 0
}

func (c Collection) First() (Value, bool) {
    if len(c) == 0 {
        return nil, false
    }
    return c[0], true
}

func (c Collection) Contains(v Value) bool {
    for _, item := range c {
        if item.Equal(v) {
            return true
        }
    }
    return false
}

func (c Collection) Distinct() Collection {
    result := make(Collection, 0, len(c))
    for _, item := range c {
        if !result.Contains(item) {
            result = append(result, item)
        }
    }
    return result
}
```

---

## El Evaluador

El evaluador implementa el patrón Visitor sobre el parse tree generado por ANTLR.

### Contexto de Evaluación

```go
// eval/evaluator.go
package eval

import (
    "context"
    "github.com/gofhir/fhirpath/types"
)

// Context mantiene el estado durante la evaluación
type Context struct {
    root      types.Collection          // Recurso raíz
    this      types.Collection          // Contexto actual ($this)
    index     int                       // Índice actual ($index)
    total     types.Value               // Acumulador ($total)
    variables map[string]types.Collection // Variables externas (%name)
    limits    map[string]int            // Límites de evaluación
    goCtx     context.Context           // Context de Go para cancelación
    resolver  Resolver                  // Resolver de referencias FHIR®
}

// NewContext crea un nuevo contexto desde JSON
func NewContext(resource []byte) *Context {
    root, _ := types.JSONToCollection(resource)

    variables := make(map[string]types.Collection)
    variables["resource"] = root  // %resource para constraints FHIR®
    variables["context"] = root   // %context

    return &Context{
        root:      root,
        this:      root,
        variables: variables,
        limits:    make(map[string]int),
        goCtx:     context.Background(),
    }
}

// WithThis retorna un nuevo contexto con el $this dado
func (c *Context) WithThis(this types.Collection) *Context {
    newCtx := *c
    newCtx.this = this
    return &newCtx
}

// SetVariable define una variable externa
func (c *Context) SetVariable(name string, value types.Collection) {
    c.variables[name] = value
}
```

### Implementación del Visitor

```go
// eval/evaluator.go
package eval

import (
    "github.com/antlr4-go/antlr/v4"
    "github.com/gofhir/fhirpath/parser/grammar"
    "github.com/gofhir/fhirpath/types"
)

// Evaluator evalúa expresiones FHIRPath usando el patrón visitor
type Evaluator struct {
    grammar.BasefhirpathVisitor
    ctx   *Context
    funcs FuncRegistry
}

// Evaluate evalúa un parse tree y retorna el resultado
func (e *Evaluator) Evaluate(tree antlr.ParseTree) (types.Collection, error) {
    result := e.Visit(tree)
    if err, ok := result.(error); ok {
        return nil, err
    }
    if col, ok := result.(types.Collection); ok {
        return col, nil
    }
    return types.Collection{}, nil
}

// VisitInvocationExpression visita expr.invocation
func (e *Evaluator) VisitInvocationExpression(ctx *grammar.InvocationExpressionContext) interface{} {
    // Evaluar la expresión base
    base := e.Visit(ctx.Expression())
    if err, ok := base.(error); ok {
        return err
    }
    baseCol := base.(types.Collection)

    // Guardar $this actual y establecer nuevo
    oldThis := e.ctx.this
    e.ctx.this = baseCol
    defer func() { e.ctx.this = oldThis }()

    // Evaluar la invocación
    return e.Visit(ctx.Invocation())
}

// VisitEqualityExpression visita expresiones de igualdad
func (e *Evaluator) VisitEqualityExpression(ctx *grammar.EqualityExpressionContext) interface{} {
    left := e.Visit(ctx.Expression(0))
    if err, ok := left.(error); ok {
        return err
    }
    leftCol := left.(types.Collection)

    right := e.Visit(ctx.Expression(1))
    if err, ok := right.(error); ok {
        return err
    }
    rightCol := right.(types.Collection)

    op := ctx.GetChild(1).(antlr.TerminalNode).GetText()

    switch op {
    case "=":
        return Equal(leftCol, rightCol)
    case "!=":
        return NotEqual(leftCol, rightCol)
    case "~":
        return Equivalent(leftCol, rightCol)
    case "!~":
        return NotEquivalent(leftCol, rightCol)
    }

    return types.Collection{}
}
```

---

## Funciones Built-in

La librería implementa **99 funciones** organizadas en 10 categorías. A continuación se presentan las más relevantes para interoperabilidad FHIR®:

### Resumen de Funciones por Categoría

| Categoría | Funciones | Uso Principal |
|-----------|-----------|---------------|
| **Existencia** | `empty`, `exists`, `count`, `all`, `distinct` | Validación de datos obligatorios |
| **Filtrado** | `where`, `select`, `ofType`, `repeat` | Extracción de datos específicos |
| **Subsetting** | `first`, `last`, `single`, `skip`, `take` | Navegación de colecciones |
| **Strings** | `contains`, `startsWith`, `matches`, `lower` | Búsqueda y normalización de texto |
| **Matemáticas** | `round`, `abs`, `sum`, `min`, `max`, `avg` | Cálculos clínicos |
| **Temporales** | `now`, `today`, `year`, `month`, `day` | Validación de fechas |
| **Conversión** | `toInteger`, `toDecimal`, `toString`, `toDate` | Transformación de tipos |
| **Navegación** | `children`, `descendants` | Exploración de recursos |
| **FHIR®** | `resolve`, `extension`, `hasExtension` | Interoperabilidad FHIR® |
| **Utility** | `trace`, `iif`, `not` | Debugging y lógica condicional |

### Funciones Clave para Interoperabilidad

#### where() - Filtrado con Criterios

```fhirpath
// Filtrar nombres por uso
Patient.name.where(use = 'official')

// Filtrar observaciones por código
Observation.where(code.coding.code = '8867-4')

// Filtrar con múltiples condiciones
Patient.telecom.where(system = 'phone' and use = 'mobile')
```

#### exists() - Validación de Datos

```fhirpath
// Verificar campos obligatorios
Patient.identifier.exists()

// Validar con criterio
Patient.name.exists(use = 'official')

// Verificar referencias
Observation.subject.exists()
```

#### extension() - Acceso a Extensiones FHIR®

```fhirpath
// Obtener extensión por URL
Patient.extension('http://hl7.org/fhir/StructureDefinition/patient-nationality')

// Verificar existencia
Patient.hasExtension('http://example.org/fhir/StructureDefinition/custom')

// Obtener valor directamente
Patient.getExtensionValue('http://example.org/extension-url')
```

#### resolve() - Resolución de Referencias

```fhirpath
// Resolver referencia a paciente
Observation.subject.resolve()

// Encadenar navegación después de resolver
Observation.subject.resolve().name.family
```

#### Funciones Temporales

```fhirpath
// Comparar fechas
Patient.birthDate <= today() - 18 years

// Extraer componentes
Patient.birthDate.year()
Observation.effectiveDateTime.month()

// Timestamp actual
now()  // DateTime completo
today() // Solo fecha
```

#### Funciones de Navegación de Árboles

```fhirpath
// Obtener todos los hijos directos
Patient.children()

// Buscar en todo el árbol (recursivo)
Bundle.entry.resource.descendants().where($this is Coding)
```

### Implementación de where()

El filtrado con `where()` establece contexto por elemento usando `$this` y `$index`:

```go
func (e *Evaluator) evaluateWhere(input types.Collection, criteria grammar.IExpressionContext) interface{} {
    result := types.Collection{}

    for i, item := range input {
        e.ctx.this = types.Collection{item}  // $this = elemento actual
        e.ctx.index = i                       // $index = posición

        criteriaResult := e.Visit(criteria)

        if col, ok := criteriaResult.(types.Collection); ok && !col.Empty() {
            if b, ok := col[0].(types.Boolean); ok && b.Bool() {
                result = append(result, item)
            }
        }
    }
    return result
}
```

### Función trace() para Debugging

```go
// Debugging de expresiones FHIRPath
result := fhirpath.MustEvaluate(patientJSON,
    "Patient.name.trace('nombres').where(use = 'official').trace('filtrado')")

// Output en stderr:
// [trace] nombres: { Garcia }
// [trace] filtrado: { Garcia }
```

La función `trace()` soporta logging estructurado configurable vía `SetTraceLogger()` para integración con sistemas de observabilidad.

---

## API Pública

### Interfaz Simple y Expresiva

```go
// fhirpath.go
package fhirpath

import "github.com/gofhir/fhirpath/types"

// Evaluate parsea y evalúa una expresión FHIRPath en un solo paso
// El recurso debe ser JSON válido como []byte
func Evaluate(resource []byte, expr string) (types.Collection, error) {
    compiled, err := Compile(expr)
    if err != nil {
        return nil, err
    }
    return compiled.Evaluate(resource)
}

// MustEvaluate es como Evaluate pero hace pánic en caso de error
func MustEvaluate(resource []byte, expr string) types.Collection {
    result, err := Evaluate(resource, expr)
    if err != nil {
        panic(err)
    }
    return result
}

// Compile parsea una expresión FHIRPath y retorna una Expression compilada
// La expresión compilada puede ser evaluada múltiples veces
func Compile(expr string) (*Expression, error) {
    return compile(expr)
}

// MustCompile es como Compile pero hace pánic en caso de error
func MustCompile(expr string) *Expression {
    compiled, err := Compile(expr)
    if err != nil {
        panic(err)
    }
    return compiled
}
```

### Expression

```go
// expression.go
package fhirpath

// Expression representa una expresión FHIRPath compilada
type Expression struct {
    source string
    tree   *grammar.EntireExpressionContext
}

// Evaluate ejecuta la expresión sobre un recurso JSON
func (e *Expression) Evaluate(resource []byte) (types.Collection, error) {
    ctx := eval.NewContext(resource)
    return e.EvaluateWithContext(ctx)
}

// EvaluateWithContext ejecuta la expresión con un contexto personalizado
func (e *Expression) EvaluateWithContext(ctx *eval.Context) (types.Collection, error) {
    evaluator := eval.NewEvaluator(ctx, funcs.GetRegistry())
    return evaluator.Evaluate(e.tree)
}

// EvaluateWithOptions evalúa con opciones funcionales personalizadas
func (e *Expression) EvaluateWithOptions(resource []byte, opts ...EvalOption) (types.Collection, error) {
    options := DefaultOptions()
    for _, opt := range opts {
        opt(options)
    }
    // Configura timeout, variables, límites, y resolver
    // Ver sección "Opciones de Evaluación" para detalles
}

// String retorna la expresión original
func (e *Expression) String() string {
    return e.source
}
```

### Opciones de Evaluación (Functional Options Pattern)

La librería utiliza el patrón de opciones funcionales para configurar la evaluación:

```go
// options.go
package fhirpath

// EvalOptions configura la evaluación de expresiones
type EvalOptions struct {
    Ctx               context.Context            // Context para cancelación y timeout
    Timeout           time.Duration              // Timeout para evaluación (default: 5s)
    MaxDepth          int                        // Profundidad máxima de recursión (default: 100)
    MaxCollectionSize int                        // Tamaño máximo de colección (default: 10000)
    Variables         map[string]types.Collection // Variables externas (%name)
    Resolver          ReferenceResolver          // Resolver para resolve()
}

// DefaultOptions retorna opciones por defecto para producción
func DefaultOptions() *EvalOptions {
    return &EvalOptions{
        Ctx:               context.Background(),
        Timeout:           5 * time.Second,
        MaxDepth:          100,
        MaxCollectionSize: 10000,
        Variables:         make(map[string]types.Collection),
    }
}

// Opciones funcionales disponibles
func WithContext(ctx context.Context) EvalOption      // Context para cancelación
func WithTimeout(d time.Duration) EvalOption          // Timeout de evaluación
func WithMaxDepth(depth int) EvalOption               // Límite de recursión
func WithMaxCollectionSize(size int) EvalOption       // Límite de colección
func WithVariable(name string, value types.Collection) EvalOption // Variable externa
func WithResolver(r ReferenceResolver) EvalOption     // Resolver de referencias FHIR®

// ReferenceResolver interface para resolver()
type ReferenceResolver interface {
    Resolve(ctx context.Context, reference string) ([]byte, error)
}
```

**Ejemplo de uso con opciones:**

```go
expr := fhirpath.MustCompile("Patient.name.given")

// Evaluar con timeout y variables personalizadas
result, err := expr.EvaluateWithOptions(patientJSON,
    fhirpath.WithTimeout(2*time.Second),
    fhirpath.WithVariable("minAge", types.Collection{types.NewInteger(18)}),
    fhirpath.WithMaxCollectionSize(5000),
)
```

---

## Ejemplo Completo de Uso

```go
package main

import (
    "fmt"
    "log"

    "github.com/gofhir/fhirpath"
)

func main() {
    // Recurso Patient de ejemplo como JSON
    patientJSON := []byte(`{
        "resourceType": "Patient",
        "id": "example",
        "name": [
            {
                "use": "official",
                "family": "Garcia",
                "given": ["Maria", "Jose"]
            },
            {
                "use": "nickname",
                "given": ["Mari"]
            }
        ],
        "birthDate": "1990-05-15",
        "gender": "female",
        "address": [
            {
                "use": "home",
                "city": "Santiago",
                "country": "Chile"
            }
        ],
        "telecom": [
            {
                "system": "phone",
                "value": "+56912345678",
                "use": "mobile"
            },
            {
                "system": "email",
                "value": "maria.garcia@example.com"
            }
        ]
    }`)

    // Ejemplos de consultas FHIRPath
    examples := []struct {
        description string
        expression  string
    }{
        {"Obtener el apellido", "Patient.name.family"},
        {"Obtener el nombre oficial", "Patient.name.where(use = 'official').given"},
        {"Obtener el primer nombre dado", "Patient.name.first().given.first()"},
        {"Verificar si es mujer", "Patient.gender = 'female'"},
        {"Obtener el email", "Patient.telecom.where(system = 'email').value"},
        {"Contar direcciones", "Patient.address.count()"},
        {"Verificar teléfono móvil", "Patient.telecom.exists(system = 'phone' and use = 'mobile')"},
        {"Ciudad en mayúsculas", "Patient.address.city.upper()"},
    }

    for _, ex := range examples {
        // Compilar expresión (puede ser reutilizada)
        expr, err := fhirpath.Compile(ex.expression)
        if err != nil {
            log.Printf("Error compilando '%s': %v", ex.expression, err)
            continue
        }

        // Evaluar sobre el recurso JSON
        result, err := expr.Evaluate(patientJSON)
        if err != nil {
            log.Printf("Error evaluando '%s': %v", ex.expression, err)
            continue
        }

        fmt.Printf("\n%s\n", ex.description)
        fmt.Printf("  Expresión: %s\n", ex.expression)
        fmt.Printf("  Resultado: %v\n", result)
    }
}
```

**Salida:**

```
Obtener el apellido
  Expresión: Patient.name.family
  Resultado: [Garcia]

Obtener el nombre oficial
  Expresión: Patient.name.where(use = 'official').given
  Resultado: [Maria, Jose]

Obtener el primer nombre dado
  Expresión: Patient.name.first().given.first()
  Resultado: [Maria]

Verificar si es mujer
  Expresión: Patient.gender = 'female'
  Resultado: [true]

Obtener el email
  Expresión: Patient.telecom.where(system = 'email').value
  Resultado: [maria.garcia@example.com]

Contar direcciones
  Expresión: Patient.address.count()
  Resultado: [1]

Verificar teléfono móvil
  Expresión: Patient.telecom.exists(system = 'phone' and use = 'mobile')
  Resultado: [true]

Ciudad en mayúsculas
  Expresión: Patient.address.city.upper()
  Resultado: [SANTIAGO]
```

---

## Operadores Soportados

La implementación soporta **todos los operadores** definidos en la especificación FHIRPath:

| Categoría | Operadores | Notas |
|-----------|------------|-------|
| **Aritméticos** | `+`, `-`, `*`, `/`, `div`, `mod` | División `/` retorna Decimal |
| **Concatenación** | `+`, `&` | `&` trata empty como string vacío |
| **Comparación** | `=`, `!=`, `<`, `<=`, `>`, `>=` | Compara valores ordenables |
| **Equivalencia** | `~`, `!~` | Case-insensitive para strings |
| **Lógicos** | `and`, `or`, `xor`, `implies`, `not()` | Three-valued logic (true/false/empty) |
| **Colección** | &#124; (union), `in`, `contains` | Unión y membresía |
| **Tipo** | `is`, `as` | Verificación y casting |

> El desglose completo de operadores y funciones está disponible en el [README del repositorio](https://github.com/robertoAraneda/gofhir).

---

## Características Avanzadas

### Protección contra ReDoS

Las funciones `matches()` y `replaceMatches()` incluyen protección contra ataques de denegación de servicio por expresiones regulares:

```go
// funcs/regex.go
type RegexCache struct {
    cache    *lru.Cache  // Cache LRU con 500 patrones
    maxLen   int         // Máximo 1000 caracteres por patrón
    timeout  time.Duration
}

// Valida patrones peligrosos (cuantificadores anidados, grupos profundos)
func (c *RegexCache) validatePattern(pattern string) error {
    // Detecta patrones como (a+)+ que causan backtracking exponencial
}

// Timeout de 100ms para strings largos
func (c *RegexCache) MatchWithTimeout(ctx context.Context, pattern, str string) (bool, error)
```

### Soporte UCUM para Quantities

Las cantidades incluyen normalización de unidades UCUM para comparaciones correctas:

```go
// types/quantity.go
type Quantity struct {
    value decimal.Decimal
    unit  string
}

// Normalize convierte a unidades canónicas para comparación
func (q Quantity) Normalize() NormalizedQuantity {
    return ucum.Normalize(q.value, q.unit)
}
```

### Límites de Evaluación

```go
ctx := eval.NewContext(resource)
ctx.SetLimit("maxDepth", 100)           // Profundidad máxima de recursión
ctx.SetLimit("maxCollectionSize", 10000) // Tamaño máximo de colección
ctx.SetContext(timeoutCtx)              // Context de Go para cancelación
```

---

## Lecciones Aprendidas

### 1. El Poder de las Colecciones

Inicialmente consideré manejar valores singulares y colecciones de forma diferente. Sin embargo, el enfoque de FHIRPath donde **todo es una colección** simplifica enormemente la implementación y elimina muchos casos especiales.

### 2. ANTLR vs Parser Manual

ANTLR ahorra meses de trabajo. La gramática oficial de FHIRPath está disponible, y generar un parser robusto es cuestión de minutos. El mantenimiento también es más sencillo cuando la gramática está separada del código.

### 3. Precisión Decimal

Usar `shopspring/decimal` en lugar de `float64` fue crucial para manejar correctamente valores financieros y médicos donde la precisión importa.

### 4. Inmutabilidad para Concurrencia

Mantener las expresiones compiladas inmutables permite compartirlas entre goroutines sin sincronización adicional. Este diseño es crucial en aplicaciones de servidor.

---

## Conclusión

Construir una implementación de FHIRPath en Go fue un proyecto desafiante pero extremadamente educativo. La combinación de ANTLR para el parsing, el sistema de tipos de Go para la seguridad, y un diseño orientado a colecciones resultó en una librería robusta y fácil de usar.

El proyecto está disponible en [GitHub](https://github.com/robertoAraneda/gofhir) y acepta contribuciones. Si trabajas con FHIR® y Go, espero que esta librería te sea útil.

---

## Referencias

- [HL7 FHIR® Specification](https://hl7.org/fhir/)
- [FHIRPath Specification](https://hl7.org/fhirpath/)
- [ANTLR 4 Go Target](https://github.com/antlr/antlr4/blob/master/doc/go-target.md)
- [gofhir Repository](https://github.com/robertoAraneda/gofhir)
- [shopspring/decimal](https://github.com/shopspring/decimal)

---

*¿Tienes preguntas o sugerencias? Déjalas en los comentarios o abre un issue en el repositorio.*

---

**Tags:** #go #golang #fhir #healthcare #fhirpath #parser #antlr #opensource

