---
title: "Paquete Types"
linkTitle: "Types"
weight: 7
description: >
  Sistema de tipos FHIRPath: Value, Collection y todos los tipos primitivos.
---

El paquete `github.com/gofhir/fhirpath/types` define el sistema de tipos FHIRPath. Cada valor retornado por una evaluación FHIRPath es un `Value`, y cada resultado de evaluación es una `Collection` (un slice ordenado de `Value`). Esta página documenta todas las interfaces, tipos y sus métodos.

```go
import "github.com/gofhir/fhirpath/types"
```

---

## Interfaces Principales

### Value

La interfaz base para todos los valores FHIRPath. Cada tipo en este paquete implementa `Value`.

```go
type Value interface {
    // Type returns the FHIRPath type name (e.g., "Boolean", "Integer", "String").
    Type() string

    // Equal compares exact equality (the = operator in FHIRPath).
    Equal(other Value) bool

    // Equivalent compares equivalence (the ~ operator in FHIRPath).
    // For strings: case-insensitive, normalized whitespace.
    Equivalent(other Value) bool

    // String returns a human-readable string representation of the value.
    String() string

    // IsEmpty indicates if this value represents an empty value.
    IsEmpty() bool
}
```

### Comparable

Implementada por tipos que soportan ordenamiento (menor que, mayor que). Extiende `Value`.

```go
type Comparable interface {
    Value
    // Compare returns -1 if less than, 0 if equal, 1 if greater than.
    // Returns error if types are incompatible.
    Compare(other Value) (int, error)
}
```

Tipos que implementan `Comparable`: `Integer`, `Decimal`, `String`, `Date`, `DateTime`, `Time`, `Quantity`.

### Numeric

Implementada por tipos numéricos. Proporciona una conversión a `Decimal` para aritmética entre tipos.

```go
type Numeric interface {
    Value
    // ToDecimal converts the numeric value to a Decimal.
    ToDecimal() Decimal
}
```

Tipos que implementan `Numeric`: `Integer`, `Decimal`.

---

## Collection

`Collection` es el tipo de retorno fundamental para todas las expresiones FHIRPath. Es una secuencia ordenada de elementos `Value`.

```go
type Collection []Value
```

### Métodos de Consulta

#### Empty

Retorna `true` si la colección no tiene elementos.

```go
func (c Collection) Empty() bool
```

#### Count

Retorna el número de elementos.

```go
func (c Collection) Count() int
```

#### First

Retorna el primer elemento y `true`, o `nil` y `false` si la colección está vacía.

```go
func (c Collection) First() (Value, bool)
```

#### Last

Retorna el último elemento y `true`, o `nil` y `false` si la colección está vacía.

```go
func (c Collection) Last() (Value, bool)
```

#### Single

Retorna el único elemento si la colección tiene exactamente un elemento. Retorna un error si la colección está vacía o tiene más de un elemento.

```go
func (c Collection) Single() (Value, error)
```

#### Contains

Retorna `true` si la colección contiene un valor igual a `v` (usando `Equal`).

```go
func (c Collection) Contains(v Value) bool
```

### Métodos de Subconjunto

#### Tail

Retorna todos los elementos excepto el primero.

```go
func (c Collection) Tail() Collection
```

#### Skip

Retorna una nueva colección con los primeros `n` elementos eliminados.

```go
func (c Collection) Skip(n int) Collection
```

#### Take

Retorna una nueva colección con solo los primeros `n` elementos.

```go
func (c Collection) Take(n int) Collection
```

### Operaciones de Conjunto

#### Distinct

Retorna una nueva colección con valores duplicados eliminados, preservando el orden de la primera aparición.

```go
func (c Collection) Distinct() Collection
```

#### IsDistinct

Retorna `true` si todos los elementos de la colección son únicos.

```go
func (c Collection) IsDistinct() bool
```

#### Union

Retorna la unión de dos colecciones con duplicados eliminados.

```go
func (c Collection) Union(other Collection) Collection
```

#### Combine

Retorna una nueva colección que concatena ambas colecciones. A diferencia de `Union`, los duplicados se preservan.

```go
func (c Collection) Combine(other Collection) Collection
```

#### Intersect

Retorna los elementos que existen en ambas colecciones.

```go
func (c Collection) Intersect(other Collection) Collection
```

#### Exclude

Retorna los elementos en `c` que no están en `other`.

```go
func (c Collection) Exclude(other Collection) Collection
```

### Agregación Boolean

#### AllTrue

Retorna `true` si cada elemento es un Boolean con valor `true`. Retorna `true` para una colección vacía (verdad vacua).

```go
func (c Collection) AllTrue() bool
```

#### AnyTrue

Retorna `true` si al menos un elemento es un Boolean con valor `true`.

```go
func (c Collection) AnyTrue() bool
```

#### AllFalse

Retorna `true` si cada elemento es un Boolean con valor `false`. Retorna `true` para una colección vacía.

```go
func (c Collection) AllFalse() bool
```

#### AnyFalse

Retorna `true` si al menos un elemento es un Boolean con valor `false`.

```go
func (c Collection) AnyFalse() bool
```

### Conversión

#### ToBoolean

Convierte una colección singleton Boolean a un `bool` de Go. Retorna un error si la colección está vacía, tiene más de un elemento, o el único elemento no es un Boolean.

```go
func (c Collection) ToBoolean() (bool, error)
```

#### String

Retorna una representación en cadena de la colección en la forma `[val1, val2, ...]`.

```go
func (c Collection) String() string
```

### Ejemplo de Collection

```go
import "github.com/gofhir/fhirpath/types"

c := types.Collection{
    types.NewString("alpha"),
    types.NewString("beta"),
    types.NewString("gamma"),
}

fmt.Println(c.Count())       // 3
fmt.Println(c.Empty())       // false

first, ok := c.First()
fmt.Println(first, ok)       // alpha true

tail := c.Tail()
fmt.Println(tail)            // [beta, gamma]

top2 := c.Take(2)
fmt.Println(top2)            // [alpha, beta]

without1 := c.Skip(1)
fmt.Println(without1)        // [beta, gamma]

single, err := c.Take(1).Single()
fmt.Println(single, err)     // alpha <nil>
```

---

## Boolean

Representa un valor booleano FHIRPath (`true` o `false`).

```go
type Boolean struct {
    // unexported fields
}
```

**Implementa:** `Value`

### NewBoolean

```go
func NewBoolean(v bool) Boolean
```

### Métodos Principales

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Bool` | `func (b Boolean) Bool() bool` | Retorna el valor `bool` subyacente |
| `Not` | `func (b Boolean) Not() Boolean` | Retorna la negación lógica |
| `Type` | `func (b Boolean) Type() string` | Retorna `"Boolean"` |
| `String` | `func (b Boolean) String() string` | Retorna `"true"` o `"false"` |

**Ejemplo:**

```go
t := types.NewBoolean(true)
f := t.Not()

fmt.Println(t.Bool())   // true
fmt.Println(f.Bool())   // false
fmt.Println(t.Type())   // Boolean
fmt.Println(t.Equal(f)) // false
```

---

## Integer

Representa un valor entero FHIRPath (respaldado por `int64`).

```go
type Integer struct {
    // unexported fields
}
```

**Implementa:** `Value`, `Comparable`, `Numeric`

### NewInteger

```go
func NewInteger(v int64) Integer
```

### Métodos Principales

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Value` | `func (i Integer) Value() int64` | Retorna el `int64` subyacente |
| `ToDecimal` | `func (i Integer) ToDecimal() Decimal` | Convierte a `Decimal` |
| `Add` | `func (i Integer) Add(other Integer) Integer` | Suma |
| `Subtract` | `func (i Integer) Subtract(other Integer) Integer` | Resta |
| `Multiply` | `func (i Integer) Multiply(other Integer) Integer` | Multiplicación |
| `Divide` | `func (i Integer) Divide(other Integer) (Decimal, error)` | División (retorna `Decimal`) |
| `Div` | `func (i Integer) Div(other Integer) (Integer, error)` | División entera |
| `Mod` | `func (i Integer) Mod(other Integer) (Integer, error)` | Módulo |
| `Negate` | `func (i Integer) Negate() Integer` | Negación |
| `Abs` | `func (i Integer) Abs() Integer` | Valor absoluto |
| `Power` | `func (i Integer) Power(exp Integer) Decimal` | Exponenciación (retorna `Decimal`) |
| `Sqrt` | `func (i Integer) Sqrt() (Decimal, error)` | Raíz cuadrada (retorna `Decimal`) |
| `Compare` | `func (i Integer) Compare(other Value) (int, error)` | Comparación (funciona con `Integer` y `Decimal`) |

**Ejemplo:**

```go
a := types.NewInteger(10)
b := types.NewInteger(3)

fmt.Println(a.Add(b).Value())       // 13
fmt.Println(a.Subtract(b).Value())  // 7
fmt.Println(a.Multiply(b).Value())  // 30

div, _ := a.Div(b)
fmt.Println(div.Value())            // 3

mod, _ := a.Mod(b)
fmt.Println(mod.Value())            // 1

result, _ := a.Divide(b)
fmt.Println(result)                 // 3.3333333333333333
```

---

## Decimal

Representa un valor decimal FHIRPath con precisión arbitraria (respaldado por `github.com/shopspring/decimal`).

```go
type Decimal struct {
    // unexported fields
}
```

**Implementa:** `Value`, `Comparable`, `Numeric`

### Constructores

| Función | Firma | Descripción |
|----------|-----------|-------------|
| `NewDecimal` | `func NewDecimal(s string) (Decimal, error)` | Crea desde una cadena como `"3.14"` |
| `NewDecimalFromInt` | `func NewDecimalFromInt(v int64) Decimal` | Crea desde un `int64` |
| `NewDecimalFromFloat` | `func NewDecimalFromFloat(v float64) Decimal` | Crea desde un `float64` |
| `MustDecimal` | `func MustDecimal(s string) Decimal` | Como `NewDecimal`, genera panic en caso de error |

### Métodos Principales

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Value` | `func (d Decimal) Value() decimal.Decimal` | Retorna el `decimal.Decimal` subyacente |
| `ToDecimal` | `func (d Decimal) ToDecimal() Decimal` | Se retorna a sí mismo |
| `Add` | `func (d Decimal) Add(other Decimal) Decimal` | Suma |
| `Subtract` | `func (d Decimal) Subtract(other Decimal) Decimal` | Resta |
| `Multiply` | `func (d Decimal) Multiply(other Decimal) Decimal` | Multiplicación |
| `Divide` | `func (d Decimal) Divide(other Decimal) (Decimal, error)` | División (precisión de 16 dígitos) |
| `Negate` | `func (d Decimal) Negate() Decimal` | Negación |
| `Abs` | `func (d Decimal) Abs() Decimal` | Valor absoluto |
| `Ceiling` | `func (d Decimal) Ceiling() Integer` | Menor entero >= d |
| `Floor` | `func (d Decimal) Floor() Integer` | Mayor entero <= d |
| `Truncate` | `func (d Decimal) Truncate() Integer` | Parte entera |
| `Round` | `func (d Decimal) Round(precision int32) Decimal` | Redondear a precisión |
| `Power` | `func (d Decimal) Power(exp Decimal) Decimal` | Exponenciación |
| `Sqrt` | `func (d Decimal) Sqrt() (Decimal, error)` | Raíz cuadrada |
| `Exp` | `func (d Decimal) Exp() Decimal` | e^d |
| `Ln` | `func (d Decimal) Ln() (Decimal, error)` | Logaritmo natural |
| `Log` | `func (d Decimal) Log(base Decimal) (Decimal, error)` | Logaritmo con base personalizada |
| `IsInteger` | `func (d Decimal) IsInteger() bool` | Verdadero si no tiene parte fraccionaria |
| `ToInteger` | `func (d Decimal) ToInteger() (Integer, bool)` | Convierte a Integer si es número entero |
| `Compare` | `func (d Decimal) Compare(other Value) (int, error)` | Comparación (funciona con `Integer` y `Decimal`) |

**Ejemplo:**

```go
pi, _ := types.NewDecimal("3.14159")
two := types.NewDecimalFromInt(2)

fmt.Println(pi.Add(two))           // 5.14159
fmt.Println(pi.Multiply(two))      // 6.28318
fmt.Println(pi.Round(2))           // 3.14
fmt.Println(pi.Ceiling())          // 4
fmt.Println(pi.Floor())            // 3
fmt.Println(pi.IsInteger())        // false

// MustDecimal for constants
half := types.MustDecimal("0.5")
fmt.Println(half)                  // 0.5
```

---

## String

Representa un valor de cadena FHIRPath.

```go
type String struct {
    // unexported fields
}
```

**Implementa:** `Value`, `Comparable`

### NewString

```go
func NewString(v string) String
```

### Métodos Principales

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Value` | `func (s String) Value() string` | Retorna la cadena de Go subyacente |
| `Length` | `func (s String) Length() int` | Número de caracteres (conteo de runas) |
| `Contains` | `func (s String) Contains(substr string) bool` | Verificación de subcadena |
| `StartsWith` | `func (s String) StartsWith(prefix string) bool` | Verificación de prefijo |
| `EndsWith` | `func (s String) EndsWith(suffix string) bool` | Verificación de sufijo |
| `Upper` | `func (s String) Upper() String` | Mayúsculas |
| `Lower` | `func (s String) Lower() String` | Minúsculas |
| `IndexOf` | `func (s String) IndexOf(substr string) int` | Índice de primera aparición (-1 si no se encuentra) |
| `Substring` | `func (s String) Substring(start, length int) String` | Extraer subcadena |
| `Replace` | `func (s String) Replace(old, replacement string) String` | Reemplazar todas las apariciones |
| `ToChars` | `func (s String) ToChars() Collection` | Dividir en cadenas de un solo carácter |
| `Compare` | `func (s String) Compare(other Value) (int, error)` | Comparación lexicográfica |

**Comportamiento de equivalencia:** `Equivalent()` para cadenas es insensible a mayúsculas y normaliza los espacios en blanco (elimina espacios al inicio/final, colapsa espacios internos a espacios simples).

**Ejemplo:**

```go
s := types.NewString("Hello, World!")

fmt.Println(s.Length())                  // 13
fmt.Println(s.Contains("World"))         // true
fmt.Println(s.StartsWith("Hello"))       // true
fmt.Println(s.Upper())                   // HELLO, WORLD!
fmt.Println(s.Lower())                   // hello, world!
fmt.Println(s.IndexOf("World"))          // 7
fmt.Println(s.Substring(0, 5))           // Hello
fmt.Println(s.Replace("World", "Go"))    // Hello, Go!

// Equivalence is case-insensitive
a := types.NewString("  hello  world  ")
b := types.NewString("Hello World")
fmt.Println(a.Equivalent(b))             // true
fmt.Println(a.Equal(b))                  // false
```

---

## Date

Representa un valor de fecha FHIRPath con precisión variable (año, año-mes o año-mes-día).

```go
type Date struct {
    // unexported fields
}
```

**Implementa:** `Value`, `Comparable`

### Constructores

| Función | Firma | Descripción |
|----------|-----------|-------------|
| `NewDate` | `func NewDate(s string) (Date, error)` | Analiza `"2024"`, `"2024-03"` o `"2024-03-15"` |
| `NewDateFromTime` | `func NewDateFromTime(t time.Time) Date` | Crea desde `time.Time` con precisión de día |

### Constantes de Precisión

```go
type DatePrecision int

const (
    YearPrecision  DatePrecision = iota // e.g., "2024"
    MonthPrecision                       // e.g., "2024-03"
    DayPrecision                         // e.g., "2024-03-15"
)
```

### Métodos Principales

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Year` | `func (d Date) Year() int` | Componente de año |
| `Month` | `func (d Date) Month() int` | Componente de mes (0 si no está especificado) |
| `Day` | `func (d Date) Day() int` | Componente de día (0 si no está especificado) |
| `Precision` | `func (d Date) Precision() DatePrecision` | El nivel de precisión |
| `ToTime` | `func (d Date) ToTime() time.Time` | Convierte a `time.Time` (valores por defecto para componentes faltantes) |
| `AddDuration` | `func (d Date) AddDuration(value int, unit string) Date` | Suma una duración temporal |
| `SubtractDuration` | `func (d Date) SubtractDuration(value int, unit string) Date` | Resta una duración temporal |
| `Compare` | `func (d Date) Compare(other Value) (int, error)` | Comparación (puede retornar error para precisión ambigua) |

Unidades de duración soportadas para `AddDuration`/`SubtractDuration`: `"year"`, `"years"`, `"month"`, `"months"`, `"week"`, `"weeks"`, `"day"`, `"days"`.

**Ejemplo:**

```go
d, _ := types.NewDate("2024-03-15")
fmt.Println(d.Year())       // 2024
fmt.Println(d.Month())      // 3
fmt.Println(d.Day())        // 15
fmt.Println(d.Precision())  // DayPrecision

// Partial date
partial, _ := types.NewDate("2024-03")
fmt.Println(partial)          // 2024-03
fmt.Println(partial.Day())   // 0 (not specified)

// Date arithmetic
next := d.AddDuration(1, "month")
fmt.Println(next)            // 2024-04-15
```

---

## DateTime

Representa un valor de fecha y hora FHIRPath con precisión variable desde año hasta milisegundo, con zona horaria opcional.

```go
type DateTime struct {
    // unexported fields
}
```

**Implementa:** `Value`, `Comparable`

### Constructores

| Función | Firma | Descripción |
|----------|-----------|-------------|
| `NewDateTime` | `func NewDateTime(s string) (DateTime, error)` | Analiza cadenas datetime ISO 8601 |
| `NewDateTimeFromTime` | `func NewDateTimeFromTime(t time.Time) DateTime` | Crea desde `time.Time` con precisión de milisegundo |

Los formatos aceptados incluyen: `"2024"`, `"2024-03"`, `"2024-03-15"`, `"2024-03-15T10:30"`, `"2024-03-15T10:30:00"`, `"2024-03-15T10:30:00.000"`, `"2024-03-15T10:30:00Z"`, `"2024-03-15T10:30:00+05:00"`.

### Constantes de Precisión

```go
type DateTimePrecision int

const (
    DTYearPrecision   DateTimePrecision = iota
    DTMonthPrecision
    DTDayPrecision
    DTHourPrecision
    DTMinutePrecision
    DTSecondPrecision
    DTMillisPrecision
)
```

### Métodos Principales

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Year` | `func (dt DateTime) Year() int` | Componente de año |
| `Month` | `func (dt DateTime) Month() int` | Componente de mes |
| `Day` | `func (dt DateTime) Day() int` | Componente de día |
| `Hour` | `func (dt DateTime) Hour() int` | Componente de hora |
| `Minute` | `func (dt DateTime) Minute() int` | Componente de minuto |
| `Second` | `func (dt DateTime) Second() int` | Componente de segundo |
| `Millisecond` | `func (dt DateTime) Millisecond() int` | Componente de milisegundo |
| `ToTime` | `func (dt DateTime) ToTime() time.Time` | Convierte a `time.Time` |
| `AddDuration` | `func (dt DateTime) AddDuration(value int, unit string) DateTime` | Suma una duración temporal |
| `SubtractDuration` | `func (dt DateTime) SubtractDuration(value int, unit string) DateTime` | Resta una duración temporal |
| `Compare` | `func (dt DateTime) Compare(other Value) (int, error)` | Comparación (puede retornar error para precisión ambigua) |

Unidades de duración soportadas: `"year"`, `"years"`, `"month"`, `"months"`, `"week"`, `"weeks"`, `"day"`, `"days"`, `"hour"`, `"hours"`, `"minute"`, `"minutes"`, `"second"`, `"seconds"`, `"millisecond"`, `"milliseconds"`, `"ms"`.

**Ejemplo:**

```go
dt, _ := types.NewDateTime("2024-03-15T14:30:00Z")
fmt.Println(dt.Year())        // 2024
fmt.Println(dt.Hour())        // 14
fmt.Println(dt.Minute())      // 30
fmt.Println(dt)               // 2024-03-15T14:30:00Z

// DateTime arithmetic
later := dt.AddDuration(2, "hours")
fmt.Println(later)             // 2024-03-15T16:30:00Z

// From time.Time
now := types.NewDateTimeFromTime(time.Now())
fmt.Println(now.Type())        // DateTime
```

---

## Time

Representa un valor de hora FHIRPath con precisión variable desde hora hasta milisegundo.

```go
type Time struct {
    // unexported fields
}
```

**Implementa:** `Value`, `Comparable`

### Constructores

| Función | Firma | Descripción |
|----------|-----------|-------------|
| `NewTime` | `func NewTime(s string) (Time, error)` | Analiza cadenas de hora como `"14:30"`, `"14:30:00"`, `"T14:30:00.000"` |
| `NewTimeFromGoTime` | `func NewTimeFromGoTime(t time.Time) Time` | Crea desde `time.Time` con precisión de milisegundo |

### Constantes de Precisión

```go
type TimePrecision int

const (
    HourPrecision   TimePrecision = iota
    MinutePrecision
    SecondPrecision
    MillisPrecision
)
```

### Métodos Principales

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Hour` | `func (t Time) Hour() int` | Componente de hora |
| `Minute` | `func (t Time) Minute() int` | Componente de minuto |
| `Second` | `func (t Time) Second() int` | Componente de segundo |
| `Millisecond` | `func (t Time) Millisecond() int` | Componente de milisegundo |
| `Compare` | `func (t Time) Compare(other Value) (int, error)` | Comparación (puede retornar error para precisión ambigua) |

**Ejemplo:**

```go
t, _ := types.NewTime("14:30:00")
fmt.Println(t.Hour())        // 14
fmt.Println(t.Minute())      // 30
fmt.Println(t.Second())      // 0
fmt.Println(t)               // 14:30:00

// With milliseconds
precise, _ := types.NewTime("T08:15:30.500")
fmt.Println(precise.Millisecond()) // 500
```

---

## Quantity

Representa una cantidad FHIRPath -- un valor numérico combinado con una cadena de unidad. Soporta normalización de unidades UCUM para comparar cantidades con unidades diferentes pero compatibles.

```go
type Quantity struct {
    // unexported fields
}
```

**Implementa:** `Value`, `Comparable`

### Constructores

| Función | Firma | Descripción |
|----------|-----------|-------------|
| `NewQuantity` | `func NewQuantity(s string) (Quantity, error)` | Analiza cadenas como `"5.5 'mg'"`, `"100 kg"` |
| `NewQuantityFromDecimal` | `func NewQuantityFromDecimal(value decimal.Decimal, unit string) Quantity` | Crea desde un `decimal.Decimal` y cadena de unidad |

### Métodos Principales

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Value` | `func (q Quantity) Value() decimal.Decimal` | Retorna el valor numérico |
| `Unit` | `func (q Quantity) Unit() string` | Retorna la cadena de unidad |
| `Add` | `func (q Quantity) Add(other Quantity) (Quantity, error)` | Suma (misma unidad requerida) |
| `Subtract` | `func (q Quantity) Subtract(other Quantity) (Quantity, error)` | Resta (misma unidad requerida) |
| `Multiply` | `func (q Quantity) Multiply(factor decimal.Decimal) Quantity` | Multiplicar por un número |
| `Divide` | `func (q Quantity) Divide(divisor decimal.Decimal) (Quantity, error)` | Dividir por un número |
| `Normalize` | `func (q Quantity) Normalize() ucum.NormalizedQuantity` | Normalización UCUM |
| `Compare` | `func (q Quantity) Compare(other Value) (int, error)` | Comparación (soporta unidades compatibles vía UCUM) |

**Comportamiento de equivalencia:** `Equivalent()` para cantidades utiliza normalización UCUM, por lo que `10 'cm'` y `0.1 'm'` se consideran equivalentes.

**Ejemplo:**

```go
q, _ := types.NewQuantity("75.5 'kg'")
fmt.Println(q.Value()) // 75.5
fmt.Println(q.Unit())  // kg
fmt.Println(q)         // 75.5 kg

// Arithmetic
q2, _ := types.NewQuantity("2.5 'kg'")
sum, _ := q.Add(q2)
fmt.Println(sum)       // 78 kg
```

---

## ObjectValue

Representa un recurso FHIR® o tipo complejo como un objeto JSON. Este tipo se utiliza internamente para representar datos estructurados dentro del motor de evaluación. El método `Type()` intenta inferir el tipo FHIR® desde la estructura JSON (verificando `resourceType` primero, luego patrones estructurales para tipos complejos comunes).

```go
type ObjectValue struct {
    // unexported fields
}
```

**Implementa:** `Value`

### NewObjectValue

```go
func NewObjectValue(data []byte) *ObjectValue
```

Crea un `ObjectValue` desde bytes JSON crudos que representan un objeto.

### Métodos Principales

| Método | Firma | Descripción |
|--------|-----------|-------------|
| `Type` | `func (o *ObjectValue) Type() string` | Tipo FHIR® inferido u `"Object"` |
| `Data` | `func (o *ObjectValue) Data() []byte` | Bytes JSON crudos |
| `Get` | `func (o *ObjectValue) Get(field string) (Value, bool)` | Obtener un valor de campo (con caché) |
| `GetCollection` | `func (o *ObjectValue) GetCollection(field string) Collection` | Obtener un campo como Collection |
| `Keys` | `func (o *ObjectValue) Keys() []string` | Todos los nombres de campo |
| `Children` | `func (o *ObjectValue) Children() Collection` | Todos los valores hijos |
| `ToQuantity` | `func (o *ObjectValue) ToQuantity() (Quantity, bool)` | Convertir a Quantity si la estructura coincide |

**Inferencia de tipo:** El método `Type()` reconoce tipos de recurso FHIR® (vía el campo `resourceType`) y tipos complejos comunes incluyendo `Quantity`, `Coding`, `CodeableConcept`, `Reference`, `Period`, `Identifier`, `Range`, `Ratio`, `Attachment`, `HumanName`, `Address`, `ContactPoint` y `Annotation`.

**Ejemplo:**

```go
data := []byte(`{"resourceType": "Patient", "id": "123", "active": true}`)
obj := types.NewObjectValue(data)

fmt.Println(obj.Type()) // Patient

if v, ok := obj.Get("id"); ok {
    fmt.Println(v) // 123
}

keys := obj.Keys()
fmt.Println(keys) // [resourceType id active]
```

---

## TypeError

Representa un error de incompatibilidad de tipos que puede ocurrir durante operaciones con valores.

```go
type TypeError struct {
    Expected  string
    Actual    string
    Operation string
}
```

### NewTypeError

```go
func NewTypeError(expected, actual, operation string) *TypeError
```

### Error

```go
func (e *TypeError) Error() string
// Returns: "type error in <operation>: expected <expected>, got <actual>"
```

**Ejemplo:**

```go
err := types.NewTypeError("Integer", "String", "comparison")
fmt.Println(err.Error()) // type error in comparison: expected Integer, got String
```

---

## Funciones de Utilidad

### JSONToCollection

Convierte bytes JSON crudos (que pueden ser un objeto, arreglo o primitivo) a una `Collection`.

```go
func JSONToCollection(data []byte) (Collection, error)
```

**Comportamiento:**

- Objeto JSON: Retorna una colección singleton con un `*ObjectValue`
- Arreglo JSON: Retorna una colección con un elemento por cada ítem del arreglo
- JSON null: Retorna una colección vacía
- Primitivo JSON: Retorna una colección singleton con el tipo correspondiente

---

## Resumen de Tipos

| Tipo | Nombre FHIRPath | Tipo Go Subyacente | Implementa |
|------|---------------|-----------------|------------|
| `Boolean` | Boolean | `bool` | `Value` |
| `Integer` | Integer | `int64` | `Value`, `Comparable`, `Numeric` |
| `Decimal` | Decimal | `decimal.Decimal` | `Value`, `Comparable`, `Numeric` |
| `String` | String | `string` | `Value`, `Comparable` |
| `Date` | Date | year/month/day ints | `Value`, `Comparable` |
| `DateTime` | DateTime | component ints + timezone | `Value`, `Comparable` |
| `Time` | Time | hour/minute/second/millis ints | `Value`, `Comparable` |
| `Quantity` | Quantity | `decimal.Decimal` + unit string | `Value`, `Comparable` |
| `*ObjectValue` | (inferred) | `[]byte` JSON | `Value` |
