---
title: "Funciones de Limites"
linkTitle: "Funciones de Limites"
weight: 13
description: >
  Funciones de limites de FHIRPath 2.0 para determinar los valores mas bajos y mas altos posibles segun la precision.
---

Las funciones de limites devuelven el valor mas bajo o mas alto posible para una entrada dada segun su precision. Estan definidas en la [especificacion FHIRPath 2.0](http://hl7.org/fhirpath/) y operan sobre los tipos `Date`, `DateTime`, `Time`, `Decimal`, `Integer` y `Quantity`.

---

## lowBoundary

Devuelve el valor mas bajo posible que la entrada podria representar, dada su precision.

**Firma:**

```text
lowBoundary([precision : Integer]) : Date | DateTime | Time | Decimal | Integer | Quantity
```

**Parametros:**

| Nombre      | Tipo      | Descripcion                                                                                                    |
|-------------|-----------|----------------------------------------------------------------------------------------------------------------|
| `precision` | `Integer` | (Opcional) Numero de digitos decimales para `Decimal` y `Quantity`. Se infiere de la representacion si se omite |

**Tipo de Retorno:** Mismo tipo que la entrada

**Ejemplos:**

```go
// Date -- completa componentes faltantes con sus valores mas bajos
result, _ := fhirpath.Evaluate(resource, "@2024.lowBoundary()")
// @2024-01-01

result, _ := fhirpath.Evaluate(resource, "@2024-06.lowBoundary()")
// @2024-06-01

// DateTime -- completa hasta precision de milisegundos con +14:00 (zona horaria mas temprana)
result, _ := fhirpath.Evaluate(resource, "@2024-06-15.lowBoundary()")
// Nota: como literal DateTime, devuelve @2024-06-15T00:00:00.000+14:00

// DateTime con zona horaria existente -- preserva la TZ
result, _ := fhirpath.Evaluate(resource, "@2024-06-15T10:00:00+02:00.lowBoundary()")
// @2024-06-15T10:00:00.000+02:00

// Time -- completa componentes faltantes con ceros
result, _ := fhirpath.Evaluate(resource, "@T12.lowBoundary()")
// @T12:00:00.000

// Decimal -- resta la mitad de la unidad de precision
result, _ := fhirpath.Evaluate(resource, "(1.0).lowBoundary()")
// 0.95 (= 1.0 - 0.05, precision inferida 1)

result, _ := fhirpath.Evaluate(resource, "(1.0).lowBoundary(1)")
// 0.95 (= 1.0 - 0.05, precision explicita 1)

// Integer -- devuelve el mismo valor (sin rango basado en precision)
result, _ := fhirpath.Evaluate(resource, "(42).lowBoundary()")
// 42

// Quantity -- resta la mitad de la unidad de precision del valor
result, _ := fhirpath.Evaluate(resource, "(1.0 'mg').lowBoundary(1)")
// 0.95 'mg'
```

**Casos Limite / Notas:**

- Para `Date`: completa el mes faltante con `01` y el dia faltante con `01`.
- Para `DateTime` sin zona horaria: agrega `+14:00` (el desplazamiento UTC mas temprano) segun la especificacion FHIRPath. Si ya existe una zona horaria, se preserva.
- Para `DateTime` que ya tiene precision de milisegundos: devuelve el valor sin cambios.
- Para `Time`: completa minuto, segundo y milisegundo faltantes con `0`.
- Para `Decimal`: si no se proporciona precision explicita, se infiere de la representacion original de la cadena (por ejemplo, `"1.0"` tiene precision implicita 1). Si el decimal no tiene digitos fraccionarios (por ejemplo, `"1"`), devuelve una coleccion vacia.
- Para `Integer`: devuelve el valor de entrada sin cambios.
- Para `Quantity`: funciona como `Decimal` pero preserva la unidad. La precision se infiere del exponente del valor cuando no se proporciona explicitamente.
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## highBoundary

Devuelve el valor mas alto posible que la entrada podria representar, dada su precision.

**Firma:**

```text
highBoundary([precision : Integer]) : Date | DateTime | Time | Decimal | Integer | Quantity
```

**Parametros:**

| Nombre      | Tipo      | Descripcion                                                                                                    |
|-------------|-----------|----------------------------------------------------------------------------------------------------------------|
| `precision` | `Integer` | (Opcional) Numero de digitos decimales para `Decimal` y `Quantity`. Se infiere de la representacion si se omite |

**Tipo de Retorno:** Mismo tipo que la entrada

**Ejemplos:**

```go
// Date -- completa componentes faltantes con sus valores mas altos
result, _ := fhirpath.Evaluate(resource, "@2024.highBoundary()")
// @2024-12-31

result, _ := fhirpath.Evaluate(resource, "@2024-02.highBoundary()")
// @2024-02-29 (ano bisiesto)

result, _ := fhirpath.Evaluate(resource, "@2023-02.highBoundary()")
// @2023-02-28 (ano no bisiesto)

// DateTime -- completa hasta precision de milisegundos con -12:00 (zona horaria mas tardia)
result, _ := fhirpath.Evaluate(resource, "@2024-06-15.highBoundary()")
// Nota: como literal DateTime, devuelve @2024-06-15T23:59:59.999-12:00

// DateTime con zona horaria existente -- preserva la TZ
result, _ := fhirpath.Evaluate(resource, "@2024-06-15T10:00:00+02:00.highBoundary()")
// @2024-06-15T10:00:00.999+02:00

// Time -- completa componentes faltantes con valores maximos
result, _ := fhirpath.Evaluate(resource, "@T12.highBoundary()")
// @T12:59:59.999

// Decimal -- suma la mitad de la unidad de precision
result, _ := fhirpath.Evaluate(resource, "(1.0).highBoundary()")
// 1.05 (= 1.0 + 0.05, precision inferida 1)

result, _ := fhirpath.Evaluate(resource, "(1.0).highBoundary(1)")
// 1.05 (= 1.0 + 0.05, precision explicita 1)

// Integer -- devuelve el mismo valor
result, _ := fhirpath.Evaluate(resource, "(42).highBoundary()")
// 42

// Quantity -- suma la mitad de la unidad de precision al valor
result, _ := fhirpath.Evaluate(resource, "(1.0 'mg').highBoundary(1)")
// 1.05 'mg'
```

**Casos Limite / Notas:**

- Para `Date`: completa el mes faltante con `12` y el dia faltante con el ultimo dia del mes resuelto (tiene en cuenta anos bisiestos).
- Para `DateTime` sin zona horaria: agrega `-12:00` (el desplazamiento UTC mas tardio) segun la especificacion FHIRPath. Si ya existe una zona horaria, se preserva.
- Para `DateTime` que ya tiene precision de milisegundos: devuelve el valor sin cambios.
- Para `Time`: completa minuto y segundo faltantes con `59`, milisegundo faltante con `999`.
- Para `Decimal`: si no se proporciona precision explicita, se infiere de la representacion original de la cadena. Si el decimal no tiene digitos fraccionarios, devuelve una coleccion vacia.
- Para `Integer`: devuelve el valor de entrada sin cambios.
- Para `Quantity`: funciona como `Decimal` pero preserva la unidad.
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## Inferencia de Precision

Cuando `lowBoundary()` o `highBoundary()` se invocan sobre valores `Decimal` o `Quantity` sin un argumento `precision` explicito, la precision se infiere automaticamente:

- **Decimal**: La precision se determina a partir de la representacion original de la cadena. Por ejemplo, `"1.0"` tiene 1 lugar decimal, `"3.14"` tiene 2, y `"42"` tiene 0.
- **Quantity**: La precision se infiere del exponente decimal del valor numerico.

Si la precision inferida es `0` (sin digitos fraccionarios), las funciones devuelven una coleccion vacia -- de acuerdo con el comportamiento de la especificacion FHIRPath de que los valores tipo entero sin precision fraccionaria no tienen un rango de limite significativo.

```go
// Precision inferida de la representacion de la cadena
result, _ := fhirpath.Evaluate(resource, "(1.0).lowBoundary()")
// 0.95 (precision 1 inferida de "1.0")

result, _ := fhirpath.Evaluate(resource, "(3.14).lowBoundary()")
// 3.135 (precision 2 inferida de "3.14")

// Decimal tipo entero devuelve vacio
result, _ := fhirpath.Evaluate(resource, "(1).lowBoundary()")
// {} (vacio -- sin precision fraccionaria)
```

---

## Comportamiento de Zona Horaria para DateTime

La especificacion FHIRPath define reglas especificas de desplazamiento de zona horaria para las funciones de limites sobre valores `DateTime`:

| Funcion         | Sin TZ presente | TZ presente      |
|-----------------|-----------------|------------------|
| `lowBoundary`   | Agrega `+14:00` | Preserva TZ      |
| `highBoundary`  | Agrega `-12:00` | Preserva TZ      |

La razon es que `+14:00` representa el punto mas temprano posible en el tiempo (el mas adelantado respecto a UTC), mientras que `-12:00` representa el punto mas tardio posible en el tiempo (el mas atrasado respecto a UTC). Esto asegura que el rango de limites cubra todos los instantes posibles que el DateTime podria representar.
