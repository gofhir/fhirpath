---
title: "Funciones de Conversion"
linkTitle: "Funciones de Conversion"
weight: 7
description: >
  Funciones para convertir entre tipos FHIRPath y para evaluacion condicional.
---

Las funciones de conversion permiten convertir valores entre tipos FHIRPath (Boolean, Integer, Decimal, String, Date, DateTime, Time, Quantity) y verificar si dichas conversiones son posibles. La funcion `iif` proporciona evaluacion condicional.

Cada funcion `to*` realiza la conversion real (devolviendo vacio si la conversion falla), mientras que su correspondiente funcion `convertsTo*` devuelve un booleano indicando si la conversion tendria exito.

---

## iif

Funcion condicional que devuelve uno de dos valores dependiendo de una condicion booleana. Este es el equivalente en FHIRPath de un operador ternario.

**Firma:**

```text
iif(condition : Boolean, trueResult : Expression [, falseResult : Expression]) : Collection
```

**Parametros:**

| Nombre         | Tipo           | Descripcion                                                                                    |
|----------------|----------------|------------------------------------------------------------------------------------------------|
| `condition`    | `Boolean`      | La condicion a evaluar                                                                         |
| `trueResult`   | `Expression`   | El valor a devolver si la condicion es `true`                                                  |
| `falseResult`  | `Expression`   | (Opcional) El valor a devolver si la condicion es `false`. Por defecto es coleccion vacia       |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "iif(Patient.active, 'Active', 'Inactive')")
// Returns 'Active' if patient is active, 'Inactive' otherwise

result, _ := fhirpath.Evaluate(patient, "iif(Patient.birthDate.exists(), Patient.birthDate, 'Unknown')")
// Returns birth date if it exists, otherwise 'Unknown'

result, _ := fhirpath.Evaluate(patient, "iif(Patient.gender = 'male', 'M', 'F')")
// Returns 'M' or 'F' based on gender
```

**Casos Limite / Notas:**

- Si la condicion esta vacia o no es booleana, se trata como `false`.
- Si el `falseResult` no se proporciona y la condicion es `false`, devuelve una coleccion vacia.
- Ambas ramas se evaluan como expresiones y se pasan como colecciones.

---

## toBoolean

Convierte la entrada a un valor Boolean.

**Firma:**

```text
toBoolean() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Reglas de Conversion:**

| Tipo de Entrada | Conversion |
| --------------- | ---------- |
| `Boolean` | Se devuelve tal cual |
| `String` | `'true'`, `'t'`, `'yes'`, `'y'`, `'1'`, `'1.0'` se convierten en `true`; `'false'`, `'f'`, `'no'`, `'n'`, `'0'`, `'0.0'` se convierten en `false` (sin distincion de mayusculas) |
| `Integer` | `1` se convierte en `true`, `0` se convierte en `false` |
| `Decimal` | `1.0` se convierte en `true`, `0.0` se convierte en `false` |

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'true'.toBoolean()")
// true

result, _ := fhirpath.Evaluate(resource, "(1).toBoolean()")
// true

result, _ := fhirpath.Evaluate(resource, "'yes'.toBoolean()")
// true
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada no puede convertirse (por ejemplo, `'maybe'.toBoolean()`).
- Devuelve una coleccion vacia si la entrada esta vacia.
- La comparacion de cadenas no distingue entre mayusculas y minusculas.

---

## convertsToBoolean

Devuelve `true` si la entrada puede convertirse a Boolean usando `toBoolean()`.

**Firma:**

```text
convertsToBoolean() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'true'.convertsToBoolean()")
// true

result, _ := fhirpath.Evaluate(resource, "'maybe'.convertsToBoolean()")
// false

result, _ := fhirpath.Evaluate(resource, "(1).convertsToBoolean()")
// true
```

**Casos Limite / Notas:**

- Devuelve `false` para entrada vacia.
- Devuelve `false` para valores `Integer` distintos de `0` y `1`.

---

## toInteger

Convierte la entrada a un valor Integer.

**Firma:**

```text
toInteger() : Integer
```

**Tipo de Retorno:** `Integer`

**Reglas de Conversion:**

| Tipo de Entrada | Conversion                                        |
|-----------------|---------------------------------------------------|
| `Integer`       | Se devuelve tal cual                              |
| `Boolean`       | `true` se convierte en `1`, `false` se convierte en `0` |
| `String`        | Se analiza como un entero con signo de 64 bits    |
| `Decimal`       | Devuelve la parte entera (truncamiento)           |

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'42'.toInteger()")
// 42

result, _ := fhirpath.Evaluate(resource, "true.toInteger()")
// 1

result, _ := fhirpath.Evaluate(resource, "(3.7).toInteger()")
// 3
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada no puede convertirse (por ejemplo, `'abc'.toInteger()`).
- Devuelve una coleccion vacia si la entrada esta vacia.
- Para entrada `Decimal`, la parte fraccionaria se descarta (truncamiento hacia cero).

---

## convertsToInteger

Devuelve `true` si la entrada puede convertirse a Integer usando `toInteger()`.

**Firma:**

```text
convertsToInteger() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'42'.convertsToInteger()")
// true

result, _ := fhirpath.Evaluate(resource, "'3.14'.convertsToInteger()")
// false

result, _ := fhirpath.Evaluate(resource, "true.convertsToInteger()")
// true
```

**Casos Limite / Notas:**

- Devuelve `false` para entrada vacia.
- Devuelve `true` para valores `Decimal` (siempre pueden truncarse).
- Devuelve `false` para cadenas que no son enteros validos.

---

## toDecimal

Convierte la entrada a un valor Decimal.

**Firma:**

```text
toDecimal() : Decimal
```

**Tipo de Retorno:** `Decimal`

**Reglas de Conversion:**

| Tipo de Entrada | Conversion                                          |
|-----------------|-----------------------------------------------------|
| `Decimal`       | Se devuelve tal cual                                |
| `Integer`       | Se convierte a Decimal                              |
| `Boolean`       | `true` se convierte en `1.0`, `false` se convierte en `0.0` |
| `String`        | Se analiza como un numero decimal                   |

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'3.14'.toDecimal()")
// 3.14

result, _ := fhirpath.Evaluate(resource, "(42).toDecimal()")
// 42.0

result, _ := fhirpath.Evaluate(resource, "true.toDecimal()")
// 1.0
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada no puede convertirse.
- Devuelve una coleccion vacia si la entrada esta vacia.
- Utiliza la biblioteca `shopspring/decimal` para aritmetica decimal precisa.

---

## convertsToDecimal

Devuelve `true` si la entrada puede convertirse a Decimal usando `toDecimal()`.

**Firma:**

```text
convertsToDecimal() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'3.14'.convertsToDecimal()")
// true

result, _ := fhirpath.Evaluate(resource, "'not-a-number'.convertsToDecimal()")
// false

result, _ := fhirpath.Evaluate(resource, "(42).convertsToDecimal()")
// true
```

**Casos Limite / Notas:**

- Devuelve `false` para entrada vacia.
- `Integer`, `Decimal` y `Boolean` siempre se convierten a Decimal.

---

## toString

Convierte la entrada a una representacion String.

**Firma:**

```text
toString() : String
```

**Tipo de Retorno:** `String`

**Reglas de Conversion:**

| Tipo de Entrada | Conversion                                               |
|-----------------|----------------------------------------------------------|
| `String`        | Se devuelve tal cual                                     |
| `Boolean`       | `'true'` o `'false'`                                    |
| `Integer`       | Representacion de cadena decimal (por ejemplo, `'42'`)   |
| `Decimal`       | Representacion de cadena decimal (por ejemplo, `'3.14'`) |

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(42).toString()")
// '42'

result, _ := fhirpath.Evaluate(resource, "true.toString()")
// 'true'

result, _ := fhirpath.Evaluate(resource, "(3.14).toString()")
// '3.14'
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia.
- Todos los tipos primitivos pueden convertirse a cadena usando su representacion `.String()`.

---

## convertsToString

Devuelve `true` si la entrada puede convertirse a String usando `toString()`.

**Firma:**

```text
convertsToString() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(42).convertsToString()")
// true

result, _ := fhirpath.Evaluate(resource, "true.convertsToString()")
// true

result, _ := fhirpath.Evaluate(resource, "'hello'.convertsToString()")
// true
```

**Casos Limite / Notas:**

- Devuelve `false` para entrada vacia.
- Devuelve `true` para todos los tipos primitivos (`String`, `Boolean`, `Integer`, `Decimal`).
- Devuelve `false` para tipos complejos (objetos).

---

## toDate

Convierte la entrada a un valor Date.

**Firma:**

```text
toDate() : Date
```

**Tipo de Retorno:** `Date`

**Reglas de Conversion:**

| Tipo de Entrada | Conversion                                       |
|-----------------|--------------------------------------------------|
| `Date`          | Se devuelve tal cual                             |
| `DateTime`      | Extrae la porcion de fecha                       |
| `String`        | Se analiza como fecha (por ejemplo, `'2024-01-15'`) |

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'2024-01-15'.toDate()")
// @2024-01-15

result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.toDate()")
// Returns the birth date as a Date type

result, _ := fhirpath.Evaluate(resource, "'not-a-date'.toDate()")
// { } (empty - invalid date string)
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada no puede analizarse como fecha.
- Devuelve una coleccion vacia si la entrada esta vacia.
- Para entrada `DateTime`, extrae los primeros 10 caracteres (la porcion de fecha).

---

## convertsToDate

Devuelve `true` si la entrada puede convertirse a Date usando `toDate()`.

**Firma:**

```text
convertsToDate() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'2024-01-15'.convertsToDate()")
// true

result, _ := fhirpath.Evaluate(resource, "'not-a-date'.convertsToDate()")
// true (basic check -- returns true for any string)

result, _ := fhirpath.Evaluate(resource, "(42).convertsToDate()")
// false
```

**Casos Limite / Notas:**

- Devuelve `false` para entrada vacia.
- La implementacion actual realiza una verificacion basica de tipo (devuelve `true` para cualquier cadena). Esto puede mejorarse en versiones futuras con una validacion mas estricta del formato de fecha.

---

## toDateTime

Convierte la entrada a un valor DateTime.

**Firma:**

```text
toDateTime() : DateTime
```

**Tipo de Retorno:** `DateTime`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'2024-01-15T10:30:00Z'.toDateTime()")
// @2024-01-15T10:30:00Z

result, _ := fhirpath.Evaluate(resource, "'2024-01-15'.toDateTime()")
// Converts date string to DateTime

result, _ := fhirpath.Evaluate(resource, "(42).toDateTime()")
// { } (empty - integer cannot convert to DateTime)
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada no puede convertirse.
- Devuelve una coleccion vacia si la entrada esta vacia.
- Actualmente acepta entrada `String` para la conversion.

---

## convertsToDateTime

Devuelve `true` si la entrada puede convertirse a DateTime usando `toDateTime()`.

**Firma:**

```text
convertsToDateTime() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'2024-01-15T10:30:00Z'.convertsToDateTime()")
// true

result, _ := fhirpath.Evaluate(resource, "(42).convertsToDateTime()")
// false
```

**Casos Limite / Notas:**

- Devuelve `false` para entrada vacia.
- Devuelve `true` para entrada `String` (verificacion basica de tipo).

---

## toTime

Convierte la entrada a un valor Time.

**Firma:**

```text
toTime() : Time
```

**Tipo de Retorno:** `Time`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'14:30:00'.toTime()")
// @T14:30:00

result, _ := fhirpath.Evaluate(resource, "'10:00:00.000'.toTime()")
// @T10:00:00.000

result, _ := fhirpath.Evaluate(resource, "(42).toTime()")
// { } (empty - integer cannot convert to Time)
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada no puede convertirse.
- Devuelve una coleccion vacia si la entrada esta vacia.
- Actualmente acepta entrada `String` para la conversion.

---

## convertsToTime

Devuelve `true` si la entrada puede convertirse a Time usando `toTime()`.

**Firma:**

```text
convertsToTime() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'14:30:00'.convertsToTime()")
// true

result, _ := fhirpath.Evaluate(resource, "(42).convertsToTime()")
// false
```

**Casos Limite / Notas:**

- Devuelve `false` para entrada vacia.
- Devuelve `true` para entrada `String` (verificacion basica de tipo).

---

## toQuantity

Convierte la entrada a un valor Quantity, opcionalmente con una unidad especificada.

**Firma:**

```text
toQuantity([unit : String]) : Quantity
```

**Parametros:**

| Nombre   | Tipo       | Descripcion                                              |
|----------|------------|----------------------------------------------------------|
| `unit`   | `String`   | (Opcional) La unidad para la cantidad resultante         |

**Tipo de Retorno:** `Quantity`

**Reglas de Conversion:**

| Tipo de Entrada | Conversion |
| --------------- | ---------- |
| `Quantity` | Se devuelve tal cual |
| `Integer` | Se convierte a Quantity con la unidad dada (o sin unidad) |
| `Decimal` | Se convierte a Quantity con la unidad dada (o sin unidad) |
| `String` | Se analiza como cadena de cantidad (por ejemplo, `'5.5 mg'`, `"10 'kg'"`) |

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(42).toQuantity('mg')")
// 42 'mg'

result, _ := fhirpath.Evaluate(resource, "'5.5 mg'.toQuantity()")
// 5.5 'mg' (parsed from string)

result, _ := fhirpath.Evaluate(resource, "(3.14).toQuantity()")
// 3.14 (unitless quantity)
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada no puede convertirse.
- Devuelve una coleccion vacia si la entrada esta vacia.
- El analisis de cadenas soporta notacion de unidades UCUM.

---

## convertsToQuantity

Devuelve `true` si la entrada puede convertirse a Quantity usando `toQuantity()`. Opcionalmente verifica si la cantidad puede expresarse en la unidad especificada.

**Firma:**

```text
convertsToQuantity([unit : String]) : Boolean
```

**Parametros:**

| Nombre   | Tipo       | Descripcion                                                            |
|----------|------------|------------------------------------------------------------------------|
| `unit`   | `String`   | (Opcional) Unidad objetivo para verificar compatibilidad               |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(42).convertsToQuantity()")
// true

result, _ := fhirpath.Evaluate(resource, "'5.5 mg'.convertsToQuantity()")
// true

result, _ := fhirpath.Evaluate(resource, "'not-a-quantity'.convertsToQuantity()")
// false
```

**Casos Limite / Notas:**

- Devuelve `false` para entrada vacia.
- `Integer` y `Decimal` siempre se convierten a Quantity.
- Cuando se especifica una unidad objetivo, verifica la compatibilidad de unidades UCUM entre las unidades de origen y destino usando normalizacion.
