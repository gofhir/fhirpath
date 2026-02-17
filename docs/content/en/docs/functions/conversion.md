---
title: "Conversion Functions"
linkTitle: "Conversion Functions"
weight: 7
description: >
  Functions for converting between FHIRPath types and for conditional evaluation.
---

Conversion functions allow you to convert values between FHIRPath types (Boolean, Integer, Decimal, String, Date, DateTime, Time, Quantity) and to test whether such conversions are possible. The `iif` function provides conditional evaluation.

Each `to*` function performs the actual conversion (returning empty if conversion fails), while its corresponding `convertsTo*` function returns a boolean indicating whether the conversion would succeed.

---

## iif

Conditional function that returns one of two values depending on a boolean condition. This is the FHIRPath equivalent of a ternary operator.

**Signature:**

```text
iif(condition : Boolean, trueResult : Expression [, falseResult : Expression]) : Collection
```

**Parameters:**

| Name           | Type           | Description                                                                          |
|----------------|----------------|--------------------------------------------------------------------------------------|
| `condition`    | `Boolean`      | The condition to evaluate                                                            |
| `trueResult`   | `Expression`   | The value to return if condition is `true`                                           |
| `falseResult`  | `Expression`   | (Optional) The value to return if condition is `false`. Defaults to empty collection |

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "iif(Patient.active, 'Active', 'Inactive')")
// Returns 'Active' if patient is active, 'Inactive' otherwise

result, _ := fhirpath.Evaluate(patient, "iif(Patient.birthDate.exists(), Patient.birthDate, 'Unknown')")
// Returns birth date if it exists, otherwise 'Unknown'

result, _ := fhirpath.Evaluate(patient, "iif(Patient.gender = 'male', 'M', 'F')")
// Returns 'M' or 'F' based on gender
```

**Edge Cases / Notes:**

- If the condition is empty or not a boolean, it is treated as `false`.
- If the `falseResult` is not provided and the condition is `false`, returns an empty collection.
- Both branches are evaluated as expressions and passed as collections.

---

## toBoolean

Converts the input to a Boolean value.

**Signature:**

```text
toBoolean() : Boolean
```

**Return Type:** `Boolean`

**Conversion Rules:**

| Input Type | Conversion |
| ---------- | ---------- |
| `Boolean` | Returns as-is |
| `String` | `'true'`, `'t'`, `'yes'`, `'y'`, `'1'`, `'1.0'` become `true`; `'false'`, `'f'`, `'no'`, `'n'`, `'0'`, `'0.0'` become `false` (case-insensitive) |
| `Integer` | `1` becomes `true`, `0` becomes `false` |
| `Decimal` | `1.0` becomes `true`, `0.0` becomes `false` |

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'true'.toBoolean()")
// true

result, _ := fhirpath.Evaluate(resource, "(1).toBoolean()")
// true

result, _ := fhirpath.Evaluate(resource, "'yes'.toBoolean()")
// true
```

**Edge Cases / Notes:**

- Returns empty collection if the input cannot be converted (e.g., `'maybe'.toBoolean()`).
- Returns empty collection if the input is empty.
- String comparison is case-insensitive.

---

## convertsToBoolean

Returns `true` if the input can be converted to a Boolean using `toBoolean()`.

**Signature:**

```text
convertsToBoolean() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'true'.convertsToBoolean()")
// true

result, _ := fhirpath.Evaluate(resource, "'maybe'.convertsToBoolean()")
// false

result, _ := fhirpath.Evaluate(resource, "(1).convertsToBoolean()")
// true
```

**Edge Cases / Notes:**

- Returns `false` for empty input.
- Returns `false` for `Integer` values other than `0` and `1`.

---

## toInteger

Converts the input to an Integer value.

**Signature:**

```text
toInteger() : Integer
```

**Return Type:** `Integer`

**Conversion Rules:**

| Input Type  | Conversion                                        |
|-------------|---------------------------------------------------|
| `Integer`   | Returns as-is                                     |
| `Boolean`   | `true` becomes `1`, `false` becomes `0`           |
| `String`    | Parsed as a 64-bit signed integer                 |
| `Decimal`   | Returns the integer part (truncation)             |

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'42'.toInteger()")
// 42

result, _ := fhirpath.Evaluate(resource, "true.toInteger()")
// 1

result, _ := fhirpath.Evaluate(resource, "(3.7).toInteger()")
// 3
```

**Edge Cases / Notes:**

- Returns empty collection if the input cannot be converted (e.g., `'abc'.toInteger()`).
- Returns empty collection if the input is empty.
- For `Decimal` input, the fractional part is discarded (truncation toward zero).

---

## convertsToInteger

Returns `true` if the input can be converted to an Integer using `toInteger()`.

**Signature:**

```text
convertsToInteger() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'42'.convertsToInteger()")
// true

result, _ := fhirpath.Evaluate(resource, "'3.14'.convertsToInteger()")
// false

result, _ := fhirpath.Evaluate(resource, "true.convertsToInteger()")
// true
```

**Edge Cases / Notes:**

- Returns `false` for empty input.
- Returns `true` for `Decimal` values (they can always be truncated).
- Returns `false` for strings that are not valid integers.

---

## toDecimal

Converts the input to a Decimal value.

**Signature:**

```text
toDecimal() : Decimal
```

**Return Type:** `Decimal`

**Conversion Rules:**

| Input Type  | Conversion                                          |
|-------------|-----------------------------------------------------|
| `Decimal`   | Returns as-is                                       |
| `Integer`   | Converted to Decimal                                |
| `Boolean`   | `true` becomes `1.0`, `false` becomes `0.0`         |
| `String`    | Parsed as a decimal number                          |

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'3.14'.toDecimal()")
// 3.14

result, _ := fhirpath.Evaluate(resource, "(42).toDecimal()")
// 42.0

result, _ := fhirpath.Evaluate(resource, "true.toDecimal()")
// 1.0
```

**Edge Cases / Notes:**

- Returns empty collection if the input cannot be converted.
- Returns empty collection if the input is empty.
- Uses the `shopspring/decimal` library for precise decimal arithmetic.

---

## convertsToDecimal

Returns `true` if the input can be converted to a Decimal using `toDecimal()`.

**Signature:**

```text
convertsToDecimal() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'3.14'.convertsToDecimal()")
// true

result, _ := fhirpath.Evaluate(resource, "'not-a-number'.convertsToDecimal()")
// false

result, _ := fhirpath.Evaluate(resource, "(42).convertsToDecimal()")
// true
```

**Edge Cases / Notes:**

- Returns `false` for empty input.
- `Integer`, `Decimal`, and `Boolean` always convert to Decimal.

---

## toString

Converts the input to a String representation.

**Signature:**

```text
toString() : String
```

**Return Type:** `String`

**Conversion Rules:**

| Input Type  | Conversion                                               |
|-------------|----------------------------------------------------------|
| `String`    | Returns as-is                                            |
| `Boolean`   | `'true'` or `'false'`                                    |
| `Integer`   | Decimal string representation (e.g., `'42'`)             |
| `Decimal`   | Decimal string representation (e.g., `'3.14'`)           |

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(42).toString()")
// '42'

result, _ := fhirpath.Evaluate(resource, "true.toString()")
// 'true'

result, _ := fhirpath.Evaluate(resource, "(3.14).toString()")
// '3.14'
```

**Edge Cases / Notes:**

- Returns empty collection if the input is empty.
- All primitive types can be converted to string using their `.String()` representation.

---

## convertsToString

Returns `true` if the input can be converted to a String using `toString()`.

**Signature:**

```text
convertsToString() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(42).convertsToString()")
// true

result, _ := fhirpath.Evaluate(resource, "true.convertsToString()")
// true

result, _ := fhirpath.Evaluate(resource, "'hello'.convertsToString()")
// true
```

**Edge Cases / Notes:**

- Returns `false` for empty input.
- Returns `true` for all primitive types (`String`, `Boolean`, `Integer`, `Decimal`).
- Returns `false` for complex types (objects).

---

## toDate

Converts the input to a Date value.

**Signature:**

```text
toDate() : Date
```

**Return Type:** `Date`

**Conversion Rules:**

| Input Type   | Conversion                                       |
|--------------|--------------------------------------------------|
| `Date`       | Returns as-is                                    |
| `DateTime`   | Extracts the date portion                        |
| `String`     | Parsed as a date (e.g., `'2024-01-15'`)          |

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'2024-01-15'.toDate()")
// @2024-01-15

result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.toDate()")
// Returns the birth date as a Date type

result, _ := fhirpath.Evaluate(resource, "'not-a-date'.toDate()")
// { } (empty - invalid date string)
```

**Edge Cases / Notes:**

- Returns empty collection if the input cannot be parsed as a date.
- Returns empty collection if the input is empty.
- For `DateTime` input, extracts the first 10 characters (the date portion).

---

## convertsToDate

Returns `true` if the input can be converted to a Date using `toDate()`.

**Signature:**

```text
convertsToDate() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'2024-01-15'.convertsToDate()")
// true

result, _ := fhirpath.Evaluate(resource, "'not-a-date'.convertsToDate()")
// true (basic check -- returns true for any string)

result, _ := fhirpath.Evaluate(resource, "(42).convertsToDate()")
// false
```

**Edge Cases / Notes:**

- Returns `false` for empty input.
- The current implementation performs a basic type check (returns `true` for any string). This may be enhanced in future versions with stricter date format validation.

---

## toDateTime

Converts the input to a DateTime value.

**Signature:**

```text
toDateTime() : DateTime
```

**Return Type:** `DateTime`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'2024-01-15T10:30:00Z'.toDateTime()")
// @2024-01-15T10:30:00Z

result, _ := fhirpath.Evaluate(resource, "'2024-01-15'.toDateTime()")
// Converts date string to DateTime

result, _ := fhirpath.Evaluate(resource, "(42).toDateTime()")
// { } (empty - integer cannot convert to DateTime)
```

**Edge Cases / Notes:**

- Returns empty collection if the input cannot be converted.
- Returns empty collection if the input is empty.
- Currently accepts `String` input for conversion.

---

## convertsToDateTime

Returns `true` if the input can be converted to a DateTime using `toDateTime()`.

**Signature:**

```text
convertsToDateTime() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'2024-01-15T10:30:00Z'.convertsToDateTime()")
// true

result, _ := fhirpath.Evaluate(resource, "(42).convertsToDateTime()")
// false
```

**Edge Cases / Notes:**

- Returns `false` for empty input.
- Returns `true` for `String` input (basic type check).

---

## toTime

Converts the input to a Time value.

**Signature:**

```text
toTime() : Time
```

**Return Type:** `Time`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'14:30:00'.toTime()")
// @T14:30:00

result, _ := fhirpath.Evaluate(resource, "'10:00:00.000'.toTime()")
// @T10:00:00.000

result, _ := fhirpath.Evaluate(resource, "(42).toTime()")
// { } (empty - integer cannot convert to Time)
```

**Edge Cases / Notes:**

- Returns empty collection if the input cannot be converted.
- Returns empty collection if the input is empty.
- Currently accepts `String` input for conversion.

---

## convertsToTime

Returns `true` if the input can be converted to a Time using `toTime()`.

**Signature:**

```text
convertsToTime() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "'14:30:00'.convertsToTime()")
// true

result, _ := fhirpath.Evaluate(resource, "(42).convertsToTime()")
// false
```

**Edge Cases / Notes:**

- Returns `false` for empty input.
- Returns `true` for `String` input (basic type check).

---

## toQuantity

Converts the input to a Quantity value, optionally with a specified unit.

**Signature:**

```text
toQuantity([unit : String]) : Quantity
```

**Parameters:**

| Name     | Type       | Description                                        |
|----------|------------|----------------------------------------------------|
| `unit`   | `String`   | (Optional) The unit for the resulting quantity      |

**Return Type:** `Quantity`

**Conversion Rules:**

| Input Type | Conversion |
| ---------- | ---------- |
| `Quantity` | Returns as-is |
| `Integer` | Converts to Quantity with the given unit (or unitless) |
| `Decimal` | Converts to Quantity with the given unit (or unitless) |
| `String` | Parsed as a quantity string (e.g., `'5.5 mg'`, `"10 'kg'"`) |

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(42).toQuantity('mg')")
// 42 'mg'

result, _ := fhirpath.Evaluate(resource, "'5.5 mg'.toQuantity()")
// 5.5 'mg' (parsed from string)

result, _ := fhirpath.Evaluate(resource, "(3.14).toQuantity()")
// 3.14 (unitless quantity)
```

**Edge Cases / Notes:**

- Returns empty collection if the input cannot be converted.
- Returns empty collection if the input is empty.
- String parsing supports UCUM unit notation.

---

## convertsToQuantity

Returns `true` if the input can be converted to a Quantity using `toQuantity()`. Optionally checks if the quantity can be expressed in the specified unit.

**Signature:**

```text
convertsToQuantity([unit : String]) : Boolean
```

**Parameters:**

| Name     | Type       | Description                                                  |
|----------|------------|--------------------------------------------------------------|
| `unit`   | `String`   | (Optional) Target unit to check compatibility against        |

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(42).convertsToQuantity()")
// true

result, _ := fhirpath.Evaluate(resource, "'5.5 mg'.convertsToQuantity()")
// true

result, _ := fhirpath.Evaluate(resource, "'not-a-quantity'.convertsToQuantity()")
// false
```

**Edge Cases / Notes:**

- Returns `false` for empty input.
- `Integer` and `Decimal` always convert to Quantity.
- When a target unit is specified, checks UCUM unit compatibility between the source and target units using normalization.
