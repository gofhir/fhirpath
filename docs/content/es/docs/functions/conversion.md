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

- La condicion sigue la evaluacion singleton: vacia es `false`, un unico elemento que no es Boolean es `true`, y mas de un elemento es un error (`ErrSingletonExpected`); asi, `iif(Patient.name.given = 'x', ...)` funciona, pero `iif(Patient.name.given, ...)` falla en un paciente con varios nombres de pila. La evaluacion es tolerante con el tipo de un unico elemento; `Expression.Analyze`, con un modelo, reporta algunos criterios que por como estan escritos no son Boolean, como espera la suite oficial, por ejemplo `iif('texto', ...)` e `iif(1 | 2 | 3, ...)`. Lee el criterio tal como esta escrito, asi que tambien reporta `iif(true | true, ...)`, que se evalua a un solo `true`; lo que no reporta queda para la evaluacion, que termina con el error de arriba si el criterio tiene varios elementos.
- Una entrada de mas de un elemento tambien es un error: `iif` se llama sin contexto o con un solo elemento.
- Si el `falseResult` no se proporciona y la condicion es `false`, devuelve una coleccion vacia.
- Solo se evalua la rama que elige la condicion, asi que un error en la otra nunca se produce.

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

- Devuelve una coleccion vacia para entrada vacia, como dice la especificacion ("If the input collection is empty, the result is empty").
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

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'42'.toInteger()")
// 42

result, _ := fhirpath.Evaluate(resource, "true.toInteger()")
// 1

result, _ := fhirpath.Evaluate(resource, "(3.7).toInteger()")
// { } (vacio - un Decimal no se convierte a Integer)
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada no puede convertirse (por ejemplo, `'abc'.toInteger()`).
- Devuelve una coleccion vacia si la entrada esta vacia.
- Un `Decimal` no se convierte, entero o no: la especificacion lista Integer, String y Boolean. `toLong()` (FHIRPath 3.0.0) lee lo mismo que `toInteger()`; el Integer de este motor ya es de 64 bits.

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

- Devuelve una coleccion vacia para entrada vacia, como dice la especificacion ("If the input collection is empty, the result is empty").
- Devuelve `false` para valores `Decimal`, que `toInteger()` no convierte. `convertsToLong()` responde lo mismo.
- Devuelve `false` para cadenas que no son enteros validos.

---

## toLong

Convierte la entrada a un entero, como lo hace `toInteger()` (FHIRPath 3.0.0).

**Firma:**

```text
toLong() : Integer
```

**Casos Limite / Notas:**

- El `Long` de 3.0.0 es un entero de 64 bits, que el `Integer` de este motor ya es, asi que `toLong()` lee lo mismo que `toInteger()` y devuelve un `Integer`: `'5000000000'.toLong()` es `5000000000`. No hay tipo `Long`, asi que `is System.Long` es falso.

---

## convertsToLong

Devuelve `true` si la entrada puede convertirse con `toLong()`, como responde `convertsToInteger()` (FHIRPath 3.0.0).

**Firma:**

```text
convertsToLong() : Boolean
```

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
| `String`        | Se analiza cuando tiene la forma `(\+|-)?\d+(\.\d+)?` |

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

- Devuelve una coleccion vacia para entrada vacia, como dice la especificacion ("If the input collection is empty, the result is empty").
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
- Una Quantity FHIR se escribe como la cantidad que representa: `185 '[lb_av]'`. Una duracion de calendario se escribe con su palabra sin comillas, `1 week`, como lo define FHIRPath 3.0.0.

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

- Devuelve una coleccion vacia para entrada vacia, como dice la especificacion ("If the input collection is empty, the result is empty").
- Devuelve `true` para todos los tipos primitivos (`String`, `Boolean`, `Integer`, `Decimal`).
- Devuelve `false` para tipos complejos (objetos), salvo una Quantity FHIR, que se convierte en el texto de la cantidad que representa.

---

## toDate

Convierte la entrada a un valor Date.

**Firma:**

```text
toDate([format : String]) : Date
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
- Para entrada `DateTime`, devuelve la fecha que indica, hasta el dia como maximo: `@2012T.toDate()` es `@2012`.
- Una cadena que no es un dia del calendario, como `'2021-02-30'`, da una coleccion vacia.
- Con un formato (FHIRPath 3.0.0, uso de prueba), una cadena se lee segun esa plantilla: `'15-01-2024'.toDate('dd-MM-yyyy')` es `@2024-01-15`. Los codigos que se leen son `yyyy`, `yy`, `MM`, `M`, `dd`, `d`, `HH`, `H`, `hh`, `h`, `mm`, `m`, `ss`, `s`, `S` (uno por digito fraccionario), `a` (AM/PM) y `Z` (un offset como `+0200` o `Z`); cualquier otro caracter debe coincidir consigo mismo. Los nombres de mes localizados `MMM` y `MMMM` y una zona por nombre `z` no se leen, y una plantilla que los use da una coleccion vacia. Lo mismo vale para `toDateTime()`, `convertsToDate()` y `convertsToDateTime()`. El formato se ignora cuando la entrada no es una cadena.

---

## convertsToDate

Devuelve `true` si la entrada puede convertirse a Date usando `toDate()`.

**Firma:**

```text
convertsToDate([format : String]) : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'2024-01-15'.convertsToDate()")
// true

result, _ := fhirpath.Evaluate(resource, "'not-a-date'.convertsToDate()")
// false

result, _ := fhirpath.Evaluate(resource, "(42).convertsToDate()")
// false
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia para entrada vacia, como dice la especificacion ("If the input collection is empty, the result is empty").
- Responde lo mismo que `toDate()`: `true` para un `Date`, un `DateTime` o una cadena que nombra un dia del calendario; `false` para `'2021-02-30'`.

---

## toDateTime

Convierte la entrada a un valor DateTime.

**Firma:**

```text
toDateTime([format : String]) : DateTime
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
- Acepta un `DateTime`, un `Date` o una cadena; con un formato, la cadena se lee segun esa plantilla, como describe `toDate()`.

---

## convertsToDateTime

Devuelve `true` si la entrada puede convertirse a DateTime usando `toDateTime()`.

**Firma:**

```text
convertsToDateTime([format : String]) : Boolean
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

- Devuelve una coleccion vacia para entrada vacia, como dice la especificacion ("If the input collection is empty, the result is empty").
- Responde lo mismo que la funcion de conversion: `true` para una cadena que se convierte, `false` para una que no.

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
- Acepta un `Time` o una cadena. Una cadena se escribe sin la `T` con que abre un literal: `'T14:34:28'.toTime()` es vacio.

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

- Devuelve una coleccion vacia para entrada vacia, como dice la especificacion ("If the input collection is empty, the result is empty").
- Responde lo mismo que la funcion de conversion: `true` para una cadena que se convierte, `false` para una que no.

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
- Un argumento de unidad vacio, `toQuantity({})`, da una coleccion vacia; una unidad que no es cadena, o de mas de un elemento, termina la evaluacion con un error.

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

- Devuelve una coleccion vacia para entrada vacia, como dice la especificacion ("If the input collection is empty, the result is empty").
- `Integer` y `Decimal` siempre se convierten a Quantity.
- Cuando se especifica una unidad objetivo, verifica la compatibilidad de unidades UCUM entre las unidades de origen y destino usando normalizacion.
