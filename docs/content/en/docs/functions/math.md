---
title: "Math Functions"
linkTitle: "Math Functions"
weight: 2
description: >
  Numeric functions for mathematical operations on Integer and Decimal values in FHIRPath expressions.
---

Math functions operate on `Integer` and `Decimal` values. When invoked on an empty collection, they return an empty collection. If the input is not a numeric type, they return an empty collection rather than raising an error.

---

## abs

Returns the absolute value of the input number.

**Signature:**

```text
abs() : Integer | Decimal
```

**Return Type:** `Integer` if the input is `Integer`, `Decimal` if the input is `Decimal`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(-5).abs()")
// 5 (Integer)

result, _ := fhirpath.Evaluate(resource, "(-3.14).abs()")
// 3.14 (Decimal)

result, _ := fhirpath.Evaluate(resource, "(42).abs()")
// 42 (positive values unchanged)
```

**Edge Cases / Notes:**

- Preserves the input type: `Integer` input returns `Integer`, `Decimal` input returns `Decimal`.
- Returns an empty collection if the input is empty.
- An input that is not a number or a quantity ends the evaluation with an error, as fhirpath.js and HL7's Java engine both refuse it. A FHIR Quantity is read as a quantity.

---

## ceiling

Returns the smallest integer greater than or equal to the input value.

**Signature:**

```text
ceiling() : Integer
```

**Return Type:** `Integer`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(3.2).ceiling()")
// 4

result, _ := fhirpath.Evaluate(resource, "(-1.5).ceiling()")
// -1

result, _ := fhirpath.Evaluate(resource, "(5).ceiling()")
// 5 (integers are returned as-is)
```

**Edge Cases / Notes:**

- If the input is already an `Integer`, it is returned unchanged.
- Always rounds toward positive infinity.
- Returns an empty collection if the input is empty.
- An input that is not a number or a quantity ends the evaluation with an error, as fhirpath.js and HL7's Java engine both refuse it.
- A `Quantity` is rounded by its value and keeps its unit: `1.1 'cm'.ceiling()` is `2 'cm'`, as FHIRPath 3.0.0 has it and fhirpath.js answers.

---

## floor

Returns the largest integer less than or equal to the input value.

**Signature:**

```text
floor() : Integer
```

**Return Type:** `Integer`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(3.8).floor()")
// 3

result, _ := fhirpath.Evaluate(resource, "(-1.2).floor()")
// -2

result, _ := fhirpath.Evaluate(resource, "(7).floor()")
// 7 (integers are returned as-is)
```

**Edge Cases / Notes:**

- If the input is already an `Integer`, it is returned unchanged.
- Always rounds toward negative infinity.
- Returns an empty collection if the input is empty.
- An input that is not a number or a quantity ends the evaluation with an error, as fhirpath.js and HL7's Java engine both refuse it.
- A `Quantity` is rounded by its value and keeps its unit: `1.1 'cm'.floor()` is `1 'cm'`, as FHIRPath 3.0.0 has it and fhirpath.js answers.

---

## truncate

Returns the integer portion of the input value, truncating toward zero.

**Signature:**

```text
truncate() : Integer
```

**Return Type:** `Integer`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(3.9).truncate()")
// 3

result, _ := fhirpath.Evaluate(resource, "(-3.9).truncate()")
// -3

result, _ := fhirpath.Evaluate(resource, "(5).truncate()")
// 5 (integers are returned as-is)
```

**Edge Cases / Notes:**

- Unlike `floor`, `truncate` always rounds toward zero. For negative values, `truncate(-3.9)` returns `-3` while `floor(-3.9)` returns `-4`.
- If the input is already an `Integer`, it is returned unchanged.
- Returns an empty collection if the input is empty.
- An input that is not a number or a quantity ends the evaluation with an error, as fhirpath.js and HL7's Java engine both refuse it.
- A `Quantity` is rounded by its value and keeps its unit: `1.1 'cm'.truncate()` is `1 'cm'`, as FHIRPath 3.0.0 has it and fhirpath.js answers.

---

## round

Rounds the input value to the specified number of decimal places.

**Signature:**

```text
round([precision : Integer]) : Decimal
```

**Parameters:**

| Name          | Type      | Description                                          |
|---------------|-----------|------------------------------------------------------|
| `precision`   | `Integer` | (Optional) Number of decimal places. Defaults to `0` |

**Return Type:** `Decimal` (or `Integer` if input is `Integer`)

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(3.456).round(2)")
// 3.46

result, _ := fhirpath.Evaluate(resource, "(3.5).round()")
// 4 (default precision is 0)

result, _ := fhirpath.Evaluate(resource, "(2.345).round(1)")
// 2.3
```

**Edge Cases / Notes:**

- If no precision is specified, defaults to `0` (rounds to nearest integer).
- An `Integer` input is returned as a `Decimal`, as the signature `round([precision : Integer]) : Decimal` has it.
- A negative precision rounds left of the point, `15.round(-1)` is `20`, as fhirpath.js and HL7's Java engine both answer, though FHIRPath 3.0.0 says "the number of digits of precision must be >= 0".
- A precision that is not an `Integer` ends the evaluation with an error; an empty one gives an empty result.
- A precision of more than 1000 places is beyond the decimals this engine holds, and the result is empty, whatever the input.
- Rounds half away from zero, which is how this engine reads the traditional round 3.0.0 describes: `2.5.round()` is `3`, `(-2.5).round()` is `-3`.
- Returns an empty collection if the input is empty.
- An input that is not a number or a quantity ends the evaluation with an error, as fhirpath.js and HL7's Java engine both refuse it.
- A `Quantity` is rounded by its value and keeps its unit: `1.5 'mg'.round()` is `2 'mg'`, as FHIRPath 3.0.0 has it and fhirpath.js answers.

---

## exp

Returns *e* raised to the power of the input value.

**Signature:**

```text
exp() : Decimal
```

**Return Type:** `Decimal`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(0).exp()")
// 1.0 (e^0 = 1)

result, _ := fhirpath.Evaluate(resource, "(1).exp()")
// 2.718281828... (e^1 = e)

result, _ := fhirpath.Evaluate(resource, "(2).exp()")
// 7.389056099...
```

**Edge Cases / Notes:**

- Always returns a `Decimal`, even if the input is an `Integer`.
- Uses Go's `math.Exp` function.
- Returns an empty collection if the input is empty.
- An input that is not a number ends the evaluation with an error, as fhirpath.js and HL7's Java engine both refuse it.

---

## ln

Returns the natural logarithm (base *e*) of the input value.

**Signature:**

```text
ln() : Decimal
```

**Return Type:** `Decimal`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(1).ln()")
// 0.0 (ln(1) = 0)

result, _ := fhirpath.Evaluate(resource, "(2.718281828).ln()")
// ~1.0

result, _ := fhirpath.Evaluate(resource, "(10).ln()")
// 2.302585093...
```

**Edge Cases / Notes:**

- Returns empty collection if the input value is less than or equal to zero.
- Always returns a `Decimal`.
- Returns an empty collection if the input is empty.
- An input that is not a number ends the evaluation with an error, as fhirpath.js and HL7's Java engine both refuse it.

---

## log

Returns the logarithm of the input value with the specified base.

**Signature:**

```text
log(base : Integer | Decimal) : Decimal
```

**Parameters:**

| Name     | Type                    | Description          |
|----------|-------------------------|----------------------|
| `base`   | `Integer` or `Decimal`  | The logarithm base   |

**Return Type:** `Decimal`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(100).log(10)")
// 2.0

result, _ := fhirpath.Evaluate(resource, "(8).log(2)")
// 3.0

result, _ := fhirpath.Evaluate(resource, "(27).log(3)")
// 3.0
```

**Edge Cases / Notes:**

- An input less than or equal to zero ends the evaluation with an error, as FHIRPath 3.0.0 says.
- A base less than or equal to zero ends the evaluation with an error too; a base of `1` gives an empty result.
- Computed as `ln(value) / ln(base)`.
- Returns an empty collection if the input is empty.
- An input that is not a number ends the evaluation with an error, as fhirpath.js and HL7's Java engine both refuse it. A base that is not a number, `16.log('2')`, or of more than one item, is an error too.

---

## power

Returns the input value raised to the specified exponent.

**Signature:**

```text
power(exponent : Integer | Decimal) : Integer | Decimal
```

**Parameters:**

| Name         | Type                    | Description                         |
|--------------|-------------------------|-------------------------------------|
| `exponent`   | `Integer` or `Decimal`  | The power to raise the input to     |

**Return Type:** `Decimal`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(2).power(3)")
// 8.0

result, _ := fhirpath.Evaluate(resource, "(4).power(0.5)")
// 2.0 (square root)

result, _ := fhirpath.Evaluate(resource, "(10).power(0)")
// 1.0
```

**Edge Cases / Notes:**

- Always returns a `Decimal` value.
- Returns empty collection if the result is `NaN` or `Inf` (e.g., `0.power(-1)`).
- Returns an empty collection if the input is empty.
- An input that is not a number ends the evaluation with an error, as fhirpath.js and HL7's Java engine both refuse it. An exponent that is not a number, or of more than one item, is an error too.

---

## sqrt

Returns the square root of the input value.

**Signature:**

```text
sqrt() : Decimal
```

**Return Type:** `Decimal`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "(16).sqrt()")
// 4.0

result, _ := fhirpath.Evaluate(resource, "(2).sqrt()")
// 1.4142135623...

result, _ := fhirpath.Evaluate(resource, "(0).sqrt()")
// 0.0
```

**Edge Cases / Notes:**

- Returns empty collection if the input value is negative.
- Always returns a `Decimal`.
- Equivalent to `power(0.5)`.
- Returns an empty collection if the input is empty.
- An input that is not a number ends the evaluation with an error, as fhirpath.js and HL7's Java engine both refuse it.
