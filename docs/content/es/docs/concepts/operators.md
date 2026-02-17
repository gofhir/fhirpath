---
title: "Operadores"
linkTitle: "Operadores"
description: "Referencia completa de todos los operadores FHIRPath: aritméticos, de comparación, igualdad, equivalencia, Boolean (con tablas de verdad de tres valores), colección, tipo y operadores de cadena, más reglas de precedencia."
weight: 3
---

FHIRPath define un amplio conjunto de operadores para aritmética, comparación, lógica y manipulación de colecciones. Esta página documenta cada operador soportado por la biblioteca FHIRPath para Go, junto con su comportamiento bajo lógica de tres valores (propagación vacía).

## Operadores Aritméticos

Los operadores aritméticos trabajan con valores `Integer`, `Decimal` y (donde se indica) `Quantity`, `String`, `Date` y `DateTime`.

| Operador | Nombre | Tipos Izquierda | Tipos Derecha | Tipo Resultado |
|----------|------|------------|-------------|-------------|
| `+` | Suma | Integer, Decimal, String, Date, DateTime, Quantity | Integer, Decimal, String, Quantity | Varía (ver abajo) |
| `-` | Resta | Integer, Decimal, Date, DateTime, Quantity | Integer, Decimal, Quantity | Varía |
| `*` | Multiplicación | Integer, Decimal | Integer, Decimal | Integer o Decimal |
| `/` | División | Integer, Decimal | Integer, Decimal | Decimal (siempre) |
| `div` | División entera | Integer | Integer | Integer |
| `mod` | Módulo | Integer | Integer | Integer |

**Promoción de tipo:** Cuando un operando es `Integer` y el otro es `Decimal`, el `Integer` se promueve a `Decimal` automáticamente.

**La división siempre retorna Decimal:** Incluso `6 / 3` retorna el Decimal `2.0`, no Integer `2`. Esto coincide con la especificación FHIRPath. Utilice `div` para la división entera.

**Concatenación de cadenas:** El operador `+` concatena dos cadenas de texto: `'Hello' + ' World'` produce `'Hello World'`. Si alguno de los operandos está vacío, el resultado es vacío. Para concatenación segura frente a nulos, utilice el operador `&` en su lugar (ver [Operadores de Cadena](#operadores-de-cadena)).

**Aritmética de Date/DateTime:** Se puede sumar o restar una `Quantity` con una unidad temporal a/de un `Date` o `DateTime`:

```text
@2024-01-15 + 30 days        --> @2024-02-14
@2024-01-15T10:00:00Z - 2 hours  --> @2024-01-15T08:00:00Z
```

**Aritmética de Quantity:** Las cantidades con la misma unidad pueden sumarse o restarse:

```text
10 'mg' + 5 'mg'  --> 15 'mg'
10 'mg' - 3 'mg'  --> 7 'mg'
```

**Propagación vacía:** Si alguno de los operandos está vacío, los operadores aritméticos retornan vacío.

### Ejemplos

```text
2 + 3           --> 5          (Integer)
2.0 + 3         --> 5.0        (Decimal, due to promotion)
10 / 3          --> 3.3333...  (Decimal)
10 div 3        --> 3          (Integer)
10 mod 3        --> 1          (Integer)
'Hello' + ' '   --> 'Hello '   (String)
```

## Operadores de Comparación

Los operadores de comparación trabajan con dos valores del mismo tipo `Comparable` (Integer, Decimal, String, Date, DateTime, Time, Quantity). Retornan una colección singleton Boolean.

| Operador | Nombre | Descripción |
|----------|------|-------------|
| `<` | Menor que | Verdadero si el izquierdo es estrictamente menor que el derecho |
| `>` | Mayor que | Verdadero si el izquierdo es estrictamente mayor que el derecho |
| `<=` | Menor o igual | Verdadero si el izquierdo es menor o igual al derecho |
| `>=` | Mayor o igual | Verdadero si el izquierdo es mayor o igual al derecho |

**Propagación vacía:** Si alguno de los operandos está vacío, los operadores de comparación retornan vacío.

**Comparación entre tipos:** Integer y Decimal pueden compararse directamente (el Integer se promueve). Comparar tipos incompatibles (por ejemplo, String vs Integer) retorna un error.

**Precisión parcial:** Comparar fechas u horas con diferentes precisiones puede ser **ambiguo**. Por ejemplo, `@2024 < @2024-06-15` no puede determinarse porque `@2024` podría representar cualquier día en 2024. En este caso, la comparación retorna vacío (señalando ambigüedad) en lugar de un resultado incorrecto.

### Ejemplos

```text
3 < 5              --> true
'apple' < 'banana' --> true   (lexicographic)
@2024-01 > @2023-12 --> true
10 'kg' > 5 'kg'   --> true
{} < 5             --> {}     (empty propagation)
```

## Igualdad y Equivalencia

FHIRPath distingue entre **igualdad** y **equivalencia**.

### Igualdad (`=`, `!=`)

| Operador | Nombre | Descripción |
|----------|------|-------------|
| `=` | Igual | Comparación estricta de valor |
| `!=` | No igual | Negación de `=` |

**Propagación vacía:** Si alguno de los operandos está vacío, `=` retorna **vacío** (no `false`). Esta es una diferencia crítica con la mayoría de los lenguajes de programación.

```text
5 = 5         --> true
5 = 6         --> false
{} = 5        --> {}      (empty, NOT false)
{} = {}       --> {}      (empty)
5 != 6        --> true
5 != {}       --> {}      (empty)
```

**Evaluación singleton:** Ambos operandos deben ser colecciones singleton. Si alguno tiene más de un elemento, el resultado es vacío.

### Equivalencia (`~`, `!~`)

| Operador | Nombre | Descripción |
|----------|------|-------------|
| `~` | Equivalente | Comparación flexible de valor |
| `!~` | No equivalente | Negación de `~` |

La equivalencia difiere de la igualdad en varios aspectos importantes:

1. **Manejo de vacío:** Dos colecciones vacías son **equivalentes** (`{} ~ {}` retorna `true`). Una colección vacía y una no vacía no son equivalentes (`{} ~ 5` retorna `false`). La equivalencia nunca retorna vacío.
2. **Comparación de cadenas:** Insensible a mayúsculas con espacios en blanco normalizados. `'Hello World' ~ 'hello  world'` es `true`.
3. **Comparación de cantidades:** Utiliza normalización UCUM. `1000 'mg' ~ 1 'g'` es `true`.

```text
5 ~ 5               --> true
{} ~ {}             --> true   (unlike = which returns {})
{} ~ 5              --> false  (unlike = which returns {})
'Hello' ~ 'hello'   --> true   (case-insensitive)
1000 'mg' ~ 1 'g'   --> true   (UCUM normalization)
```

### Resumen de Igualdad vs Equivalencia

| Escenario | `=` (Igualdad) | `~` (Equivalencia) |
|----------|:--------------:|:------------------:|
| `5 = 5` / `5 ~ 5` | `true` | `true` |
| `5 = 6` / `5 ~ 6` | `false` | `false` |
| `{} = {}` / `{} ~ {}` | `{}` (vacío) | `true` |
| `{} = 5` / `{} ~ 5` | `{}` (vacío) | `false` |
| `'Hi' = 'hi'` / `'Hi' ~ 'hi'` | `false` | `true` |
| `1000 'mg' = 1 'g'` / `1000 'mg' ~ 1 'g'` | `true` | `true` |

## Operadores Boolean

Los operadores Boolean implementan **lógica de tres valores** donde los tres estados son `true`, `false` y `{}` (vacío/desconocido). Esto es requerido por la especificación FHIRPath para manejar correctamente los datos faltantes en recursos de salud.

### and

Retorna `true` solo si ambos operandos son `true`.

| `and` | **true** | **false** | **{}** |
|-------|:--------:|:---------:|:------:|
| **true** | `true` | `false` | `{}` |
| **false** | `false` | `false` | `false` |
| **{}** | `{}` | `false` | `{}` |

Punto clave: `false and {}` es `false` (no vacío), porque sin importar cuál sea el valor desconocido, el resultado debe ser `false`.

### or

Retorna `true` si al menos un operando es `true`.

| `or` | **true** | **false** | **{}** |
|------|:--------:|:---------:|:------:|
| **true** | `true` | `true` | `true` |
| **false** | `true` | `false` | `{}` |
| **{}** | `true` | `{}` | `{}` |

Punto clave: `true or {}` es `true` (no vacío), porque sin importar cuál sea el valor desconocido, el resultado debe ser `true`.

### xor

Retorna `true` si exactamente un operando es `true`.

| `xor` | **true** | **false** | **{}** |
|-------|:--------:|:---------:|:------:|
| **true** | `false` | `true` | `{}` |
| **false** | `true` | `false` | `{}` |
| **{}** | `{}` | `{}` | `{}` |

### implies

Implicación lógica: `A implies B` es equivalente a `(not A) or B`.

| `implies` | **true** | **false** | **{}** |
|-----------|:--------:|:---------:|:------:|
| **true** | `true` | `false` | `{}` |
| **false** | `true` | `true` | `true` |
| **{}** | `true` | `{}` | `{}` |

Punto clave: `false implies X` siempre es `true`, independientemente de `X`. Esta es la tabla de verdad estándar para la implicación material.

### not

Negación unaria. Retorna la negación lógica de un singleton Boolean.

| Entrada | Resultado de `not` |
|-------|:------------:|
| `true` | `false` |
| `false` | `true` |
| `{}` | `{}` |

Si la entrada no es un singleton Boolean, el resultado es vacío.

### Ejemplos

```text
true and false       --> false
true and {}          --> {}
false and {}         --> false   (short-circuit)
true or {}           --> true    (short-circuit)
true xor false       --> true
false implies false   --> true
(not true)           --> false
```

## Operadores de Colección

| Operador | Nombre | Descripción |
|----------|------|-------------|
| `\|` | Unión | Retorna la unión de dos colecciones con duplicados eliminados |
| `in` | Pertenencia | Retorna `true` si el singleton izquierdo está en la colección derecha |
| `contains` | Contiene | Retorna `true` si el singleton derecho está en la colección izquierda |

### Unión (`|`)

Fusiona dos colecciones y elimina valores duplicados. Esta es la forma de operador de `Collection.Union()`.

```text
(1 | 2 | 3) | (2 | 3 | 4)  --> (1 | 2 | 3 | 4)
```

### in

Verifica si un solo valor (izquierda) existe en una colección (derecha). El operando izquierdo debe ser un singleton.

```text
2 in (1 | 2 | 3)     --> true
5 in (1 | 2 | 3)     --> false
{} in (1 | 2 | 3)    --> {}     (empty propagation)
```

### contains

Lo inverso de `in`. Verifica si una colección (izquierda) contiene un solo valor (derecha). El operando derecho debe ser un singleton.

```text
(1 | 2 | 3) contains 2    --> true
(1 | 2 | 3) contains 5    --> false
(1 | 2 | 3) contains {}   --> {}   (empty propagation)
```

## Operadores de Tipo

| Operador | Nombre | Descripción |
|----------|------|-------------|
| `is` | Prueba de tipo | Retorna `true` si el valor es del tipo dado |
| `as` | Conversión de tipo | Retorna el valor si es del tipo dado, de lo contrario vacío |

### is

Prueba si un valor es de un tipo específico:

```text
5 is Integer         --> true
5 is String          --> false
'hello' is String    --> true
@2024-01 is Date     --> true
```

### as

Convierte un valor a un tipo específico. Si el valor no es de ese tipo, retorna vacío:

```text
5 as Integer         --> 5
5 as String          --> {}
'hello' as String    --> 'hello'
```

El operador `as` es útil en cláusulas `where` para filtrar y convertir simultáneamente.

## Operadores de Cadena

| Operador | Nombre | Descripción |
|----------|------|-------------|
| `&` | Concatenación | Concatenación de cadenas segura frente a nulos |
| `+` | Suma | Concatenación de cadenas (con propagación vacía) |

El operador `&` difiere de `+` en su manejo de colecciones vacías. El operador `+` propaga el vacío (si cualquier lado está vacío, el resultado es vacío), mientras que `&` trata el vacío como una cadena vacía:

```text
'Hello' + {}      --> {}            (empty propagation)
'Hello' & {}      --> 'Hello'       (empty treated as '')
{} & {}           --> ''            (both treated as '')
'Hello' & ' ' & 'World'  --> 'Hello World'
```

Esto hace que `&` sea el operador preferido para construir cadenas de visualización donde algunas partes pueden estar ausentes.

## Precedencia de Operadores

Los operadores se listan desde la precedencia **más alta** a la **más baja**:

| Precedencia | Operadores | Asociatividad |
|:----------:|-----------|:-------------:|
| 1 | `.` (navegación de ruta) | Izquierda |
| 2 | `[]` (indexador) | Izquierda |
| 3 | Unario `+`, `-` | Derecha |
| 4 | `*`, `/`, `div`, `mod` | Izquierda |
| 5 | `+`, `-` | Izquierda |
| 6 | `&` (concatenación de cadenas) | Izquierda |
| 7 | `is`, `as` | Izquierda |
| 8 | `\|` (unión) | Izquierda |
| 9 | `<`, `>`, `<=`, `>=` | Izquierda |
| 10 | `=`, `!=`, `~`, `!~` | Izquierda |
| 11 | `in`, `contains` | Izquierda |
| 12 | `and` | Izquierda |
| 13 | `xor` | Izquierda |
| 14 | `or` | Izquierda |
| 15 | `implies` | Izquierda |

Utilice paréntesis para sobreescribir la precedencia predeterminada cuando sea necesario:

```text
2 + 3 * 4          --> 14     (multiplication first)
(2 + 3) * 4        --> 20     (addition first)
true or false and true  --> true  (and binds tighter than or)
```
