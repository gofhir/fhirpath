---
title: "Sistema de Tipos"
linkTitle: "Sistema de Tipos"
description: "Referencia completa de los ocho tipos primitivos de FHIRPath, sus representaciones en Go, interfaces principales y normalización de cantidades UCUM."
weight: 1
---

FHIRPath define ocho tipos primitivos. La biblioteca FHIRPath para Go mapea cada uno de ellos a un struct concreto de Go en el paquete `github.com/gofhir/fhirpath/types`.

## Tipos Primitivos

| Tipo FHIRPath | Tipo Go | Ejemplos de Literales FHIRPath |
|---------------|-----------------|--------------------------|
| Boolean | `types.Boolean` | `true`, `false` |
| Integer | `types.Integer` | `42`, `-17`, `0` |
| Decimal | `types.Decimal` | `3.14159`, `-0.5` |
| String | `types.String` | `'hello'`, `'FHIRPath'` |
| Date | `types.Date` | `@2024-01-15`, `@2024-01`, `@2024` |
| DateTime | `types.DateTime` | `@2024-01-15T10:30:00Z`, `@2024-01-15T10:30:00+05:00` |
| Time | `types.Time` | `@T14:30:00`, `@T08:00` |
| Quantity | `types.Quantity` | `10 'mg'`, `5.5 'km'`, `1000 'ms'` |

Cada valor FHIRPath en la biblioteca implementa la interfaz `Value`. Las colecciones de valores se representan como `[]Value` (con el alias `Collection`).

## La Interfaz Value

Todos los tipos FHIRPath implementan la interfaz `Value` definida en `types/value.go`:

```go
type Value interface {
    // Type returns the FHIRPath type name (e.g., "Boolean", "Integer").
    Type() string

    // Equal compares exact equality (the = operator).
    Equal(other Value) bool

    // Equivalent compares equivalence (the ~ operator).
    // For strings: case-insensitive, normalizes whitespace.
    Equivalent(other Value) bool

    // String returns a human-readable string representation.
    String() string

    // IsEmpty indicates if this value represents empty.
    IsEmpty() bool
}
```

La distinción entre `Equal` y `Equivalent` es importante. La igualdad (`=`) es una comparación estricta: `'Hello'` no es igual a `'hello'`. La equivalencia (`~`) es una comparación más flexible: para cadenas de texto es insensible a mayúsculas y normaliza los espacios en blanco; para cantidades utiliza la normalización UCUM de modo que `1000 'mg'` es equivalente a `1 'g'`.

## La Interfaz Comparable

Los tipos que soportan ordenamiento implementan `Comparable`:

```go
type Comparable interface {
    Value
    // Compare returns -1, 0, or 1.
    // Returns error if types are incompatible.
    Compare(other Value) (int, error)
}
```

Los siguientes tipos implementan `Comparable`: `Integer`, `Decimal`, `String`, `Date`, `DateTime`, `Time` y `Quantity`.

`Boolean` **no** implementa `Comparable` porque la especificación FHIRPath no define un ordenamiento para los valores Boolean.

## La Interfaz Numeric

Los tipos numéricos (`Integer` y `Decimal`) implementan la interfaz `Numeric`, que permite la aritmética entre tipos:

```go
type Numeric interface {
    Value
    // ToDecimal converts the numeric value to a Decimal.
    ToDecimal() Decimal
}
```

Cuando un operador aritmético recibe un `Integer` y un `Decimal`, el `Integer` se promueve a `Decimal` mediante `ToDecimal()` antes de realizar la operación.

## Detalles de los Tipos

### Boolean

`types.Boolean` envuelve un `bool` de Go.

```go
b := types.NewBoolean(true)
fmt.Println(b.Bool())   // true
fmt.Println(b.Type())   // Boolean
fmt.Println(b.Not())    // false
```

### Integer

`types.Integer` envuelve un `int64` y proporciona métodos aritméticos: `Add`, `Subtract`, `Multiply`, `Divide`, `Div` (división entera), `Mod`, `Negate`, `Abs`, `Power` y `Sqrt`.

```go
i := types.NewInteger(42)
fmt.Println(i.Value())            // 42
fmt.Println(i.Add(types.NewInteger(8)))  // 50
fmt.Println(i.ToDecimal())        // 42
```

### Decimal

`types.Decimal` utiliza `shopspring/decimal` para aritmética de precisión arbitraria. Soporta todos los mismos métodos aritméticos que `Integer` además de `Ceiling`, `Floor`, `Truncate`, `Round`, `Exp`, `Ln` y `Log`.

```go
d, _ := types.NewDecimal("3.14159")
fmt.Println(d.Value())      // 3.14159
fmt.Println(d.Round(2))     // 3.14
fmt.Println(d.Ceiling())    // 4
fmt.Println(d.Floor())      // 3
```

La división siempre retorna un `Decimal` (incluso para `Integer / Integer`), de acuerdo con la especificación FHIRPath.

### String

`types.String` envuelve un `string` de Go y proporciona `Length`, `Contains`, `StartsWith`, `EndsWith`, `Upper`, `Lower`, `IndexOf`, `Substring`, `Replace` y `ToChars`.

```go
s := types.NewString("FHIRPath")
fmt.Println(s.Length())           // 8
fmt.Println(s.Lower())           // fhirpath
fmt.Println(s.Contains("Path"))  // true
```

La equivalencia para cadenas de texto es insensible a mayúsculas y normaliza los espacios en blanco:

```go
a := types.NewString("Hello  World")
b := types.NewString("hello world")
fmt.Println(a.Equal(b))      // false
fmt.Println(a.Equivalent(b)) // true
```

### Date

`types.Date` soporta precisión parcial: solo año (`@2024`), año-mes (`@2024-01`) o fecha completa (`@2024-01-15`).

```go
d, _ := types.NewDate("2024-01-15")
fmt.Println(d.Year())   // 2024
fmt.Println(d.Month())  // 1
fmt.Println(d.Day())    // 15
```

Comparar fechas con diferentes precisiones puede ser **ambiguo**. Por ejemplo, `@2024` vs `@2024-06-15` no es claramente menor ni mayor, por lo que `Compare` retorna un error para señalar la ambigüedad (coincidiendo con la semántica de propagación vacía de FHIRPath para valores incomparables).

La aritmética de fechas se soporta a través de `AddDuration` y `SubtractDuration` con unidades de cantidad temporales (`year`, `month`, `week`, `day`).

### DateTime

`types.DateTime` extiende `Date` con componentes de tiempo (hora, minuto, segundo, milisegundo) y un desplazamiento de zona horaria opcional. Soporta siete niveles de precisión, desde solo año hasta milisegundo.

```go
dt, _ := types.NewDateTime("2024-01-15T10:30:00Z")
fmt.Println(dt.Year())   // 2024
fmt.Println(dt.Hour())   // 10
fmt.Println(dt.Minute()) // 30
```

La aritmética de DateTime soporta todas las unidades temporales incluyendo `hour`, `minute`, `second` y `millisecond`.

### Time

`types.Time` representa una hora del día sin componente de fecha. Soporta precisión desde hora hasta milisegundo.

```go
t, _ := types.NewTime("14:30:00")
fmt.Println(t.Hour())   // 14
fmt.Println(t.Minute()) // 30
fmt.Println(t.Second()) // 0
```

### Quantity

`types.Quantity` combina un valor `decimal.Decimal` con una cadena de unidad UCUM. Las cantidades soportan aritmética (`Add`, `Subtract`, `Multiply`, `Divide`) y comparación.

```go
q, _ := types.NewQuantity("10 'mg'")
fmt.Println(q.Value()) // 10
fmt.Println(q.Unit())  // mg
```

## Normalización UCUM

Una de las características más poderosas del tipo Quantity es la normalización automática UCUM (Unified Code for Units of Measure). Al comparar o probar la equivalencia de cantidades con unidades diferentes pero compatibles, la biblioteca normaliza ambas cantidades a su forma canónica UCUM antes de comparar.

Esto significa que las siguientes equivalencias se cumplen:

```text
1000 'mg' ~ 1 'g'      // true -- both normalize to grams
100 'cm'  ~ 1 'm'      // true -- both normalize to meters
1000 'ms' ~ 1 's'      // true -- both normalize to seconds
```

La normalización se realiza automáticamente por los métodos `Equal`, `Equivalent` y `Compare` de `Quantity`. También se puede llamar directamente a `Normalize()` para obtener la forma canónica:

```go
q, _ := types.NewQuantity("1000 'mg'")
norm := q.Normalize()
fmt.Printf("Value: %f, Unit: %s\n", norm.Value, norm.Code) // Value: 1.000000, Unit: g
```

Si dos cantidades tienen unidades incompatibles (por ejemplo, `'mg'` y `'m'`), la comparación retorna un error en lugar de un resultado incorrecto.
