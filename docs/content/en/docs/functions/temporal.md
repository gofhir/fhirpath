---
title: "Temporal Functions"
linkTitle: "Temporal Functions"
weight: 9
description: >
  Functions for working with dates, times, and extracting temporal components in FHIRPath expressions.
---

Temporal functions provide access to the current date and time, and allow extracting individual components (year, month, day, etc.) from `Date`, `DateTime`, and `Time` values. These are essential for date-based filtering and calculations on FHIR resources.

---

## now

Returns the current date and time as a `DateTime` value.

**Signature:**

```text
now() : DateTime
```

**Return Type:** `DateTime`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "now()")
// e.g., @2024-06-15T14:30:00.000-05:00

result, _ := fhirpath.Evaluate(patient, "Patient.birthDate < now()")
// true (birth date is in the past)

result, _ := fhirpath.Evaluate(resource, "now().year()")
// Current year as an integer (e.g., 2024)
```

**Edge Cases / Notes:**

- Returns the system time at the moment of evaluation.
- The returned `DateTime` includes timezone offset information from the system's local timezone.
- Each call to `now()` within a single expression evaluation may return slightly different values if significant time passes. For consistency within a single evaluation, the library evaluates `now()` at execution time.
- The value is formatted as `2006-01-02T15:04:05.000-07:00`.

---

## today

Returns the current date as a `Date` value (without time component).

**Signature:**

```text
today() : Date
```

**Return Type:** `Date`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "today()")
// e.g., @2024-06-15

result, _ := fhirpath.Evaluate(patient, "Patient.birthDate <= today()")
// true (birth date is today or in the past)

result, _ := fhirpath.Evaluate(resource, "today().month()")
// Current month as an integer (e.g., 6)
```

**Edge Cases / Notes:**

- Returns the system date based on the local timezone.
- Does not include any time or timezone information.
- The value is formatted as `2006-01-02`.

---

## timeOfDay

Returns the current time as a `Time` value (without date component).

**Signature:**

```text
timeOfDay() : Time
```

**Return Type:** `Time`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "timeOfDay()")
// e.g., @T14:30:00.000

result, _ := fhirpath.Evaluate(resource, "timeOfDay().hour()")
// Current hour as an integer (e.g., 14)

result, _ := fhirpath.Evaluate(resource, "timeOfDay().minute()")
// Current minute as an integer (e.g., 30)
```

**Edge Cases / Notes:**

- Returns the system time based on the local clock.
- Does not include any date or timezone information.
- The value is formatted as `15:04:05.000`.

---

## year

Extracts the year component from a `Date` or `DateTime` value.

**Signature:**

```text
year() : Integer
```

**Return Type:** `Integer`

**Applicable Types:** `Date`, `DateTime`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.year()")
// e.g., 1990

result, _ := fhirpath.Evaluate(resource, "@2024-06-15.year()")
// 2024

result, _ := fhirpath.Evaluate(resource, "now().year()")
// Current year
```

**Edge Cases / Notes:**

- Returns empty collection if the input is empty or not a `Date`/`DateTime`.
- The year is always available for valid dates.

---

## month

Extracts the month component from a `Date` or `DateTime` value.

**Signature:**

```text
month() : Integer
```

**Return Type:** `Integer` (1-12)

**Applicable Types:** `Date`, `DateTime`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.month()")
// e.g., 3 (March)

result, _ := fhirpath.Evaluate(resource, "@2024-06-15.month()")
// 6

result, _ := fhirpath.Evaluate(resource, "today().month()")
// Current month
```

**Edge Cases / Notes:**

- Returns empty collection if the input is empty or not a `Date`/`DateTime`.
- Returns empty collection if the date has year-only precision (month component is `0`).
- Months are 1-based: January = 1, December = 12.

---

## day

Extracts the day-of-month component from a `Date` or `DateTime` value.

**Signature:**

```text
day() : Integer
```

**Return Type:** `Integer` (1-31)

**Applicable Types:** `Date`, `DateTime`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.day()")
// e.g., 25

result, _ := fhirpath.Evaluate(resource, "@2024-06-15.day()")
// 15

result, _ := fhirpath.Evaluate(resource, "today().day()")
// Current day of month
```

**Edge Cases / Notes:**

- Returns empty collection if the input is empty or not a `Date`/`DateTime`.
- Returns empty collection if the date has year-month precision only (day component is `0`).

---

## hour

Extracts the hour component from a `DateTime` or `Time` value.

**Signature:**

```text
hour() : Integer
```

**Return Type:** `Integer` (0-23)

**Applicable Types:** `DateTime`, `Time`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "now().hour()")
// Current hour (e.g., 14)

result, _ := fhirpath.Evaluate(resource, "@T14:30:00.hour()")
// 14

result, _ := fhirpath.Evaluate(resource, "timeOfDay().hour()")
// Current hour
```

**Edge Cases / Notes:**

- Returns empty collection if the input is empty or not a `DateTime`/`Time`.
- Not applicable to `Date` values (which have no time component) -- returns empty.
- Hours are in 24-hour format: 0-23.

---

## minute

Extracts the minute component from a `DateTime` or `Time` value.

**Signature:**

```text
minute() : Integer
```

**Return Type:** `Integer` (0-59)

**Applicable Types:** `DateTime`, `Time`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "now().minute()")
// Current minute (e.g., 30)

result, _ := fhirpath.Evaluate(resource, "@T14:30:00.minute()")
// 30

result, _ := fhirpath.Evaluate(resource, "timeOfDay().minute()")
// Current minute
```

**Edge Cases / Notes:**

- Returns empty collection if the input is empty or not a `DateTime`/`Time`.
- Not applicable to `Date` values -- returns empty.

---

## second

Extracts the second component from a `DateTime` or `Time` value.

**Signature:**

```text
second() : Integer
```

**Return Type:** `Integer` (0-59)

**Applicable Types:** `DateTime`, `Time`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "now().second()")
// Current second (e.g., 45)

result, _ := fhirpath.Evaluate(resource, "@T14:30:45.second()")
// 45

result, _ := fhirpath.Evaluate(resource, "timeOfDay().second()")
// Current second
```

**Edge Cases / Notes:**

- Returns empty collection if the input is empty or not a `DateTime`/`Time`.
- Not applicable to `Date` values -- returns empty.

---

## millisecond

Extracts the millisecond component from a `DateTime` or `Time` value.

**Signature:**

```text
millisecond() : Integer
```

**Return Type:** `Integer` (0-999)

**Applicable Types:** `DateTime`, `Time`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "now().millisecond()")
// Current millisecond (e.g., 123)

result, _ := fhirpath.Evaluate(resource, "@T14:30:45.123.millisecond()")
// 123

result, _ := fhirpath.Evaluate(resource, "timeOfDay().millisecond()")
// Current millisecond
```

**Edge Cases / Notes:**

- Returns empty collection if the input is empty or not a `DateTime`/`Time`.
- Not applicable to `Date` values -- returns empty.
- Precision depends on the underlying time representation. Some FHIR date-time values may not have millisecond precision.
