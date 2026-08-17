---
title: "Temporal Functions"
linkTitle: "Temporal Functions"
weight: 9
description: >
  Functions for working with dates, times, and extracting temporal components in FHIRPath expressions.
---

Temporal functions provide access to the current date and time, and allow extracting individual components (year, month, day, etc.) from `Date`, `DateTime`, and `Time` values. These are essential for date-based filtering and calculations on FHIR® resources.

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

## Component extractors

The ten functions below read one component out of a temporal value.

They carry their `Of` because `year`, `month`, `day`, `hour`, `minute` and
`second` are calendar units in the grammar: a call written
`Patient.birthDate.month()` does not parse, since the identifier after the dot
is read as the unit. The older spellings are still registered and answer the
same thing, but reaching them takes backticks —
``Patient.birthDate.`month`()`` — so the names below are the ones to use.

Each of them shares three rules: an empty input gives an empty collection, an
input of more than one item signals an error, and a value that does not carry
the component asked for gives an empty collection rather than a zero.

## yearOf

Extracts the year component from a `Date` or `DateTime` value.

**Signature:**

```text
yearOf() : Integer
```

**Return Type:** `Integer`

**Applicable Types:** `Date`, `DateTime`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "Patient.birthDate.yearOf()")
// e.g., 1990

result, _ := fhirpath.Evaluate(resource, "@2024-06-15.yearOf()")
// 2024

result, _ := fhirpath.Evaluate(resource, "now().yearOf()")
// Current year
```

**Edge Cases / Notes:**

- Returns an empty collection if the input is empty.
- Signals an error if the input holds more than one item.
- The year is always present in a valid date, so this is the one component that a `Date` or `DateTime` never lacks.

---

## monthOf

Extracts the month component from a `Date` or `DateTime` value.

**Signature:**

```text
monthOf() : Integer` (1-12)
```

**Return Type:** `Integer` (1-12)

**Applicable Types:** `Date`, `DateTime`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "@2024-06-15.monthOf()")
// 6

result, _ := fhirpath.Evaluate(resource, "@2024.monthOf()")
// { } -- the value carries no month

result, _ := fhirpath.Evaluate(resource, "Patient.birthDate.monthOf()")
// e.g., 12
```

**Edge Cases / Notes:**

- Returns an empty collection if the input is empty.
- Signals an error if the input holds more than one item.
- A value written without a month gives an empty collection, not zero: `@2024` says nothing about the month.

---

## dayOf

Extracts the day component from a `Date` or `DateTime` value.

**Signature:**

```text
dayOf() : Integer` (1-31)
```

**Return Type:** `Integer` (1-31)

**Applicable Types:** `Date`, `DateTime`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "@2024-06-15.dayOf()")
// 15

result, _ := fhirpath.Evaluate(resource, "@2024-06.dayOf()")
// { } -- the value carries no day
```

**Edge Cases / Notes:**

- Returns an empty collection if the input is empty.
- Signals an error if the input holds more than one item.
- As with the month, a value that does not carry a day gives an empty collection.

---

## hourOf

Extracts the hour component from a `DateTime` or `Time` value.

**Signature:**

```text
hourOf() : Integer` (0-23)
```

**Return Type:** `Integer` (0-23)

**Applicable Types:** `DateTime`, `Time`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "@2024-06-15T14:30:45.hourOf()")
// 14

result, _ := fhirpath.Evaluate(resource, "@T14:30:45.hourOf()")
// 14

result, _ := fhirpath.Evaluate(resource, "@2024-06-15.hourOf()")
// { } -- a date has no time in it
```

**Edge Cases / Notes:**

- Returns an empty collection if the input is empty.
- Signals an error if the input holds more than one item.
- A `Date` has no hour and gives empty. So does a `DateTime` written without a time.
- Midnight answers `0`, which is why an absent hour has to be empty rather than zero.

---

## minuteOf

Extracts the minute component from a `DateTime` or `Time` value.

**Signature:**

```text
minuteOf() : Integer` (0-59)
```

**Return Type:** `Integer` (0-59)

**Applicable Types:** `DateTime`, `Time`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "@2024-06-15T14:30:45.minuteOf()")
// 30

result, _ := fhirpath.Evaluate(resource, "@T14.minuteOf()")
// { } -- the value stops at the hour
```

**Edge Cases / Notes:**

- Returns an empty collection if the input is empty.
- Signals an error if the input holds more than one item.
- A value whose precision stops short of the minute gives an empty collection.

---

## secondOf

Extracts the second component from a `DateTime` or `Time` value.

**Signature:**

```text
secondOf() : Integer` (0-59)
```

**Return Type:** `Integer` (0-59)

**Applicable Types:** `DateTime`, `Time`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "@2024-06-15T14:30:45.secondOf()")
// 45

result, _ := fhirpath.Evaluate(resource, "@T14:30.secondOf()")
// { } -- the value stops at the minute
```

**Edge Cases / Notes:**

- Returns an empty collection if the input is empty.
- Signals an error if the input holds more than one item.
- A value whose precision stops short of the second gives an empty collection.

---

## millisecondOf

Extracts the millisecond component from a `DateTime` or `Time` value.

**Signature:**

```text
millisecondOf() : Integer` (0-999)
```

**Return Type:** `Integer` (0-999)

**Applicable Types:** `DateTime`, `Time`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "@T14:30:45.123.millisecondOf()")
// 123

result, _ := fhirpath.Evaluate(resource, "@T14:30:45.millisecondOf()")
// { } -- the value stops at the second
```

**Edge Cases / Notes:**

- Returns an empty collection if the input is empty.
- Signals an error if the input holds more than one item.
- A value whose precision stops short of the millisecond gives an empty collection.

---

## timezoneOffsetOf

Extracts the timezone offset from a `DateTime`, as hours from UTC.

**Signature:**

```text
timezoneOffsetOf() : Decimal
```

**Return Type:** `Decimal`

**Applicable Types:** `DateTime`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "@2012-01-01T12:30:00.000-07:00.timezoneOffsetOf()")
// -7.0

result, _ := fhirpath.Evaluate(resource, "@2012-01-01T12:30:00.000+08:45.timezoneOffsetOf()")
// 8.75 -- Eucla, Western Australia

result, _ := fhirpath.Evaluate(resource, "@2012-01-01T12:30:00.timezoneOffsetOf()")
// { } -- no offset was written
```

**Edge Cases / Notes:**

- Returns an empty collection if the input is empty.
- Signals an error if the input holds more than one item.
- Fractional hours are decimal: a quarter of an hour is `0.25`.
- An offset whose minutes do not divide the hour exactly -- twenty minutes is a third of one -- gives a repeating decimal carried to the engine's division precision.
- A `Date` or a `DateTime` written without an offset gives an empty collection.

---

## dateOf

Extracts the date part of a `Date` or `DateTime`.

**Signature:**

```text
dateOf() : Date
```

**Return Type:** `Date`

**Applicable Types:** `Date`, `DateTime`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "@2012-01-01T12:30:00.000-07:00.dateOf()")
// @2012-01-01

result, _ := fhirpath.Evaluate(resource, "@2012.dateOf()")
// @2012 -- the precision of the input is kept

result, _ := fhirpath.Evaluate(resource, "@2012-05T12:30:00.dateOf()")
// @2012-05
```

**Edge Cases / Notes:**

- Returns an empty collection if the input is empty.
- Signals an error if the input holds more than one item.
- The result keeps the precision the input was written with, rather than filling in a month and day the value does not state.

---

## timeOf

Extracts the time part of a `DateTime`.

**Signature:**

```text
timeOf() : Time
```

**Return Type:** `Time`

**Applicable Types:** `DateTime`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "@2012-01-01T12:30:00.000-07:00.timeOf()")
// @T12:30:00.000

result, _ := fhirpath.Evaluate(resource, "@2012-01-01.timeOf()")
// { } -- the value carries no time
```

**Edge Cases / Notes:**

- Returns an empty collection if the input is empty.
- Signals an error if the input holds more than one item.
- The offset is not part of the result: the example above gives `@T12:30:00.000`, not `@T12:30:00.000-07:00`.
- Unlike `dateOf`, this one does not accept a `Time` -- a `Time` is already what it would return.
