---
title: "Funciones Temporales"
linkTitle: "Funciones Temporales"
weight: 9
description: >
  Funciones para trabajar con fechas, horas y extraer componentes temporales en expresiones FHIRPath.
---

Las funciones temporales proporcionan acceso a la fecha y hora actuales, y permiten extraer componentes individuales (ano, mes, dia, etc.) de valores `Date`, `DateTime` y `Time`. Estas son esenciales para el filtrado basado en fechas y calculos sobre recursos FHIR.

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

## year

Extrae el componente de ano de un valor `Date` o `DateTime`.

**Firma:**

```text
year() : Integer
```

**Tipo de Retorno:** `Integer`

**Tipos Aplicables:** `Date`, `DateTime`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.year()")
// e.g., 1990

result, _ := fhirpath.Evaluate(resource, "@2024-06-15.year()")
// 2024

result, _ := fhirpath.Evaluate(resource, "now().year()")
// Current year
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia o no es `Date`/`DateTime`.
- El ano siempre esta disponible para fechas validas.

---

## month

Extrae el componente de mes de un valor `Date` o `DateTime`.

**Firma:**

```text
month() : Integer
```

**Tipo de Retorno:** `Integer` (1-12)

**Tipos Aplicables:** `Date`, `DateTime`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.month()")
// e.g., 3 (March)

result, _ := fhirpath.Evaluate(resource, "@2024-06-15.month()")
// 6

result, _ := fhirpath.Evaluate(resource, "today().month()")
// Current month
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia o no es `Date`/`DateTime`.
- Devuelve una coleccion vacia si la fecha tiene precision de solo ano (el componente de mes es `0`).
- Los meses son base 1: Enero = 1, Diciembre = 12.

---

## day

Extrae el componente de dia del mes de un valor `Date` o `DateTime`.

**Firma:**

```text
day() : Integer
```

**Tipo de Retorno:** `Integer` (1-31)

**Tipos Aplicables:** `Date`, `DateTime`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.day()")
// e.g., 25

result, _ := fhirpath.Evaluate(resource, "@2024-06-15.day()")
// 15

result, _ := fhirpath.Evaluate(resource, "today().day()")
// Current day of month
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia o no es `Date`/`DateTime`.
- Devuelve una coleccion vacia si la fecha tiene precision de solo ano-mes (el componente de dia es `0`).

---

## hour

Extrae el componente de hora de un valor `DateTime` o `Time`.

**Firma:**

```text
hour() : Integer
```

**Tipo de Retorno:** `Integer` (0-23)

**Tipos Aplicables:** `DateTime`, `Time`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "now().hour()")
// Current hour (e.g., 14)

result, _ := fhirpath.Evaluate(resource, "@T14:30:00.hour()")
// 14

result, _ := fhirpath.Evaluate(resource, "timeOfDay().hour()")
// Current hour
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia o no es `DateTime`/`Time`.
- No aplicable a valores `Date` (que no tienen componente de hora) -- devuelve vacio.
- Las horas estan en formato de 24 horas: 0-23.

---

## minute

Extrae el componente de minuto de un valor `DateTime` o `Time`.

**Firma:**

```text
minute() : Integer
```

**Tipo de Retorno:** `Integer` (0-59)

**Tipos Aplicables:** `DateTime`, `Time`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "now().minute()")
// Current minute (e.g., 30)

result, _ := fhirpath.Evaluate(resource, "@T14:30:00.minute()")
// 30

result, _ := fhirpath.Evaluate(resource, "timeOfDay().minute()")
// Current minute
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia o no es `DateTime`/`Time`.
- No aplicable a valores `Date` -- devuelve vacio.

---

## second

Extrae el componente de segundo de un valor `DateTime` o `Time`.

**Firma:**

```text
second() : Integer
```

**Tipo de Retorno:** `Integer` (0-59)

**Tipos Aplicables:** `DateTime`, `Time`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "now().second()")
// Current second (e.g., 45)

result, _ := fhirpath.Evaluate(resource, "@T14:30:45.second()")
// 45

result, _ := fhirpath.Evaluate(resource, "timeOfDay().second()")
// Current second
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia o no es `DateTime`/`Time`.
- No aplicable a valores `Date` -- devuelve vacio.

---

## millisecond

Extrae el componente de milisegundo de un valor `DateTime` o `Time`.

**Firma:**

```text
millisecond() : Integer
```

**Tipo de Retorno:** `Integer` (0-999)

**Tipos Aplicables:** `DateTime`, `Time`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "now().millisecond()")
// Current millisecond (e.g., 123)

result, _ := fhirpath.Evaluate(resource, "@T14:30:45.123.millisecond()")
// 123

result, _ := fhirpath.Evaluate(resource, "timeOfDay().millisecond()")
// Current millisecond
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia o no es `DateTime`/`Time`.
- No aplicable a valores `Date` -- devuelve vacio.
- La precision depende de la representacion temporal subyacente. Algunos valores de fecha-hora FHIR pueden no tener precision de milisegundos.
