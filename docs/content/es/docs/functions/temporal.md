---
title: "Funciones Temporales"
linkTitle: "Funciones Temporales"
weight: 9
description: >
  Funciones para trabajar con fechas, horas y extraer componentes temporales en expresiones FHIRPath.
---

Las funciones temporales proporcionan acceso a la fecha y hora actuales, y permiten extraer componentes individuales (ano, mes, dia, etc.) de valores `Date`, `DateTime` y `Time`. Estas son esenciales para el filtrado basado en fechas y calculos sobre recursos FHIR®.

---

## now

Devuelve la fecha y hora actuales como un valor `DateTime`.

**Firma:**

```text
now() : DateTime
```

**Tipo de Retorno:** `DateTime`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "now()")
// e.g., @2024-06-15T14:30:00.000-05:00

result, _ := fhirpath.Evaluate(patient, "Patient.birthDate < now()")
// true (birth date is in the past)

result, _ := fhirpath.Evaluate(resource, "now().year()")
// Current year as an integer (e.g., 2024)
```

**Casos Limite / Notas:**

- Devuelve la hora del sistema en el momento de la evaluacion.
- El `DateTime` devuelto incluye informacion de zona horaria del huso horario local del sistema.
- Cada llamada a `now()` dentro de una sola evaluacion de expresion puede devolver valores ligeramente diferentes si transcurre tiempo significativo. Para consistencia dentro de una sola evaluacion, la biblioteca evalua `now()` en el momento de ejecucion.
- El valor se formatea como `2006-01-02T15:04:05.000-07:00`.

---

## today

Devuelve la fecha actual como un valor `Date` (sin componente de hora).

**Firma:**

```text
today() : Date
```

**Tipo de Retorno:** `Date`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "today()")
// e.g., @2024-06-15

result, _ := fhirpath.Evaluate(patient, "Patient.birthDate <= today()")
// true (birth date is today or in the past)

result, _ := fhirpath.Evaluate(resource, "today().month()")
// Current month as an integer (e.g., 6)
```

**Casos Limite / Notas:**

- Devuelve la fecha del sistema basada en la zona horaria local.
- No incluye informacion de hora ni zona horaria.
- El valor se formatea como `2006-01-02`.

---

## timeOfDay

Devuelve la hora actual como un valor `Time` (sin componente de fecha).

**Firma:**

```text
timeOfDay() : Time
```

**Tipo de Retorno:** `Time`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "timeOfDay()")
// e.g., @T14:30:00.000

result, _ := fhirpath.Evaluate(resource, "timeOfDay().hour()")
// Current hour as an integer (e.g., 14)

result, _ := fhirpath.Evaluate(resource, "timeOfDay().minute()")
// Current minute as an integer (e.g., 30)
```

**Casos Limite / Notas:**

- Devuelve la hora del sistema basada en el reloj local.
- No incluye informacion de fecha ni zona horaria.
- El valor se formatea como `15:04:05.000`.

---

## Extractores de componentes

Las diez funciones siguientes leen un componente de un valor temporal.

Llevan el `Of` porque `year`, `month`, `day`, `hour`, `minute` y `second` son
unidades de calendario en la gramática: una llamada escrita
`Patient.birthDate.month()` no parsea, ya que el identificador tras el punto se
lee como la unidad. Las grafías antiguas siguen registradas y responden lo
mismo, pero alcanzarlas exige comillas invertidas —
``Patient.birthDate.`month`()`` — así que los nombres de abajo son los que
conviene usar.

Las diez comparten tres reglas: una entrada vacía devuelve una colección vacía,
una entrada con más de un elemento señala un error, y un valor que no lleva el
componente pedido devuelve una colección vacía en vez de un cero.

## yearOf

Extrae el componente de año de un valor `Date` o `DateTime`.

**Firma:**

```text
yearOf() : Integer
```

**Tipo de Retorno:** `Integer`

**Tipos Aplicables:** `Date`, `DateTime`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "Patient.birthDate.yearOf()")
// p. ej., 1990

result, _ := fhirpath.Evaluate(resource, "@2024-06-15.yearOf()")
// 2024

result, _ := fhirpath.Evaluate(resource, "now().yearOf()")
// El año actual
```

**Casos Límite / Notas:**

- Devuelve una colección vacía si la entrada está vacía.
- Señala un error si la entrada tiene más de un elemento.
- El año siempre está presente en una fecha válida, así que es el único componente que un `Date` o `DateTime` nunca deja de tener.

---

## monthOf

Extrae el componente de mes de un valor `Date` o `DateTime`.

**Firma:**

```text
monthOf() : Integer` (1-12)
```

**Tipo de Retorno:** `Integer` (1-12)

**Tipos Aplicables:** `Date`, `DateTime`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "@2024-06-15.monthOf()")
// 6

result, _ := fhirpath.Evaluate(resource, "@2024.monthOf()")
// { } -- el valor no lleva mes

result, _ := fhirpath.Evaluate(resource, "Patient.birthDate.monthOf()")
// p. ej., 12
```

**Casos Límite / Notas:**

- Devuelve una colección vacía si la entrada está vacía.
- Señala un error si la entrada tiene más de un elemento.
- Un valor escrito sin mes devuelve una colección vacía, no cero: `@2024` no dice nada sobre el mes.

---

## dayOf

Extrae el componente de día de un valor `Date` o `DateTime`.

**Firma:**

```text
dayOf() : Integer` (1-31)
```

**Tipo de Retorno:** `Integer` (1-31)

**Tipos Aplicables:** `Date`, `DateTime`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "@2024-06-15.dayOf()")
// 15

result, _ := fhirpath.Evaluate(resource, "@2024-06.dayOf()")
// { } -- el valor no lleva día
```

**Casos Límite / Notas:**

- Devuelve una colección vacía si la entrada está vacía.
- Señala un error si la entrada tiene más de un elemento.
- Como con el mes, un valor que no lleva día devuelve una colección vacía.

---

## hourOf

Extrae el componente de hora de un valor `DateTime` o `Time`.

**Firma:**

```text
hourOf() : Integer` (0-23)
```

**Tipo de Retorno:** `Integer` (0-23)

**Tipos Aplicables:** `DateTime`, `Time`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "@2024-06-15T14:30:45.hourOf()")
// 14

result, _ := fhirpath.Evaluate(resource, "@T14:30:45.hourOf()")
// 14

result, _ := fhirpath.Evaluate(resource, "@2024-06-15.hourOf()")
// { } -- una fecha no lleva hora
```

**Casos Límite / Notas:**

- Devuelve una colección vacía si la entrada está vacía.
- Señala un error si la entrada tiene más de un elemento.
- Un `Date` no tiene hora y devuelve vacío. Lo mismo un `DateTime` escrito sin hora.
- La medianoche responde `0`, y por eso una hora ausente tiene que ser vacío y no cero.

---

## minuteOf

Extrae el componente de minuto de un valor `DateTime` o `Time`.

**Firma:**

```text
minuteOf() : Integer` (0-59)
```

**Tipo de Retorno:** `Integer` (0-59)

**Tipos Aplicables:** `DateTime`, `Time`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "@2024-06-15T14:30:45.minuteOf()")
// 30

result, _ := fhirpath.Evaluate(resource, "@T14.minuteOf()")
// { } -- el valor se detiene en la hora
```

**Casos Límite / Notas:**

- Devuelve una colección vacía si la entrada está vacía.
- Señala un error si la entrada tiene más de un elemento.
- Un valor cuya precisión no llega al minuto devuelve una colección vacía.

---

## secondOf

Extrae el componente de segundo de un valor `DateTime` o `Time`.

**Firma:**

```text
secondOf() : Integer` (0-59)
```

**Tipo de Retorno:** `Integer` (0-59)

**Tipos Aplicables:** `DateTime`, `Time`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "@2024-06-15T14:30:45.secondOf()")
// 45

result, _ := fhirpath.Evaluate(resource, "@T14:30.secondOf()")
// { } -- el valor se detiene en el minuto
```

**Casos Límite / Notas:**

- Devuelve una colección vacía si la entrada está vacía.
- Señala un error si la entrada tiene más de un elemento.
- Un valor cuya precisión no llega al segundo devuelve una colección vacía.

---

## millisecondOf

Extrae el componente de milisegundo de un valor `DateTime` o `Time`.

**Firma:**

```text
millisecondOf() : Integer` (0-999)
```

**Tipo de Retorno:** `Integer` (0-999)

**Tipos Aplicables:** `DateTime`, `Time`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "@T14:30:45.123.millisecondOf()")
// 123

result, _ := fhirpath.Evaluate(resource, "@T14:30:45.millisecondOf()")
// { } -- el valor se detiene en el segundo
```

**Casos Límite / Notas:**

- Devuelve una colección vacía si la entrada está vacía.
- Señala un error si la entrada tiene más de un elemento.
- Un valor cuya precisión no llega al milisegundo devuelve una colección vacía.

---

## timezoneOffsetOf

Extrae el desplazamiento de zona horaria de un `DateTime`, en horas desde UTC.

**Firma:**

```text
timezoneOffsetOf() : Decimal
```

**Tipo de Retorno:** `Decimal`

**Tipos Aplicables:** `DateTime`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "@2012-01-01T12:30:00.000-07:00.timezoneOffsetOf()")
// -7.0

result, _ := fhirpath.Evaluate(resource, "@2012-01-01T12:30:00.000+08:45.timezoneOffsetOf()")
// 8.75 -- Eucla, Australia Occidental

result, _ := fhirpath.Evaluate(resource, "@2012-01-01T12:30:00.timezoneOffsetOf()")
// { } -- no se escribió desplazamiento
```

**Casos Límite / Notas:**

- Devuelve una colección vacía si la entrada está vacía.
- Señala un error si la entrada tiene más de un elemento.
- Las fracciones de hora son decimales: un cuarto de hora es `0.25`.
- Un desplazamiento cuyos minutos no dividen la hora de forma exacta -- veinte minutos son un tercio de ella -- da un decimal periódico llevado a la precisión de división del motor.
- Un `Date`, o un `DateTime` escrito sin desplazamiento, devuelve una colección vacía.

---

## dateOf

Extrae la parte de fecha de un `Date` o `DateTime`.

**Firma:**

```text
dateOf() : Date
```

**Tipo de Retorno:** `Date`

**Tipos Aplicables:** `Date`, `DateTime`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "@2012-01-01T12:30:00.000-07:00.dateOf()")
// @2012-01-01

result, _ := fhirpath.Evaluate(resource, "@2012.dateOf()")
// @2012 -- se conserva la precisión de la entrada

result, _ := fhirpath.Evaluate(resource, "@2012-05T12:30:00.dateOf()")
// @2012-05
```

**Casos Límite / Notas:**

- Devuelve una colección vacía si la entrada está vacía.
- Señala un error si la entrada tiene más de un elemento.
- El resultado conserva la precisión con que se escribió la entrada, en vez de rellenar un mes y un día que el valor no declara.

---

## timeOf

Extrae la parte de hora de un `DateTime`.

**Firma:**

```text
timeOf() : Time
```

**Tipo de Retorno:** `Time`

**Tipos Aplicables:** `DateTime`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "@2012-01-01T12:30:00.000-07:00.timeOf()")
// @T12:30:00.000

result, _ := fhirpath.Evaluate(resource, "@2012-01-01.timeOf()")
// { } -- el valor no lleva hora
```

**Casos Límite / Notas:**

- Devuelve una colección vacía si la entrada está vacía.
- Señala un error si la entrada tiene más de un elemento.
- El desplazamiento no forma parte del resultado: el ejemplo de arriba da `@T12:30:00.000`, no `@T12:30:00.000-07:00`.
- A diferencia de `dateOf`, esta no acepta un `Time` -- un `Time` ya es lo que devolvería.
