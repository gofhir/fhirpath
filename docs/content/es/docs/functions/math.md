---
title: "Funciones Matematicas"
linkTitle: "Funciones Matematicas"
weight: 2
description: >
  Funciones numericas para operaciones matematicas sobre valores Integer y Decimal en expresiones FHIRPath.
---

Las funciones matematicas operan sobre valores `Integer` y `Decimal`. Cuando se invocan sobre una coleccion vacia, devuelven una coleccion vacia. Si la entrada no es de tipo numerico, devuelven una coleccion vacia en lugar de generar un error.

---

## abs

Devuelve el valor absoluto del numero de entrada.

**Firma:**

```text
abs() : Integer | Decimal
```

**Tipo de Retorno:** `Integer` si la entrada es `Integer`, `Decimal` si la entrada es `Decimal`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(-5).abs()")
// 5 (Integer)

result, _ := fhirpath.Evaluate(resource, "(-3.14).abs()")
// 3.14 (Decimal)

result, _ := fhirpath.Evaluate(resource, "(42).abs()")
// 42 (positive values unchanged)
```

**Casos Limite / Notas:**

- Preserva el tipo de entrada: una entrada `Integer` devuelve `Integer`, una entrada `Decimal` devuelve `Decimal`.
- Devuelve una coleccion vacia si la entrada esta vacia o no es numerica.

---

## ceiling

Devuelve el menor entero mayor o igual al valor de entrada.

**Firma:**

```text
ceiling() : Integer
```

**Tipo de Retorno:** `Integer`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(3.2).ceiling()")
// 4

result, _ := fhirpath.Evaluate(resource, "(-1.5).ceiling()")
// -1

result, _ := fhirpath.Evaluate(resource, "(5).ceiling()")
// 5 (integers are returned as-is)
```

**Casos Limite / Notas:**

- Si la entrada ya es un `Integer`, se devuelve sin cambios.
- Siempre redondea hacia infinito positivo.
- Devuelve una coleccion vacia si la entrada esta vacia o no es numerica.

---

## floor

Devuelve el mayor entero menor o igual al valor de entrada.

**Firma:**

```text
floor() : Integer
```

**Tipo de Retorno:** `Integer`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(3.8).floor()")
// 3

result, _ := fhirpath.Evaluate(resource, "(-1.2).floor()")
// -2

result, _ := fhirpath.Evaluate(resource, "(7).floor()")
// 7 (integers are returned as-is)
```

**Casos Limite / Notas:**

- Si la entrada ya es un `Integer`, se devuelve sin cambios.
- Siempre redondea hacia infinito negativo.
- Devuelve una coleccion vacia si la entrada esta vacia o no es numerica.

---

## truncate

Devuelve la parte entera del valor de entrada, truncando hacia cero.

**Firma:**

```text
truncate() : Integer
```

**Tipo de Retorno:** `Integer`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(3.9).truncate()")
// 3

result, _ := fhirpath.Evaluate(resource, "(-3.9).truncate()")
// -3

result, _ := fhirpath.Evaluate(resource, "(5).truncate()")
// 5 (integers are returned as-is)
```

**Casos Limite / Notas:**

- A diferencia de `floor`, `truncate` siempre redondea hacia cero. Para valores negativos, `truncate(-3.9)` devuelve `-3` mientras que `floor(-3.9)` devuelve `-4`.
- Si la entrada ya es un `Integer`, se devuelve sin cambios.
- Devuelve una coleccion vacia si la entrada esta vacia o no es numerica.

---

## round

Redondea el valor de entrada al numero especificado de decimales.

**Firma:**

```text
round([precision : Integer]) : Decimal
```

**Parametros:**

| Nombre        | Tipo      | Descripcion                                                    |
|---------------|-----------|----------------------------------------------------------------|
| `precision`   | `Integer` | (Opcional) Numero de decimales. Por defecto es `0`             |

**Tipo de Retorno:** `Decimal` (o `Integer` si la entrada es `Integer`)

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(3.456).round(2)")
// 3.46

result, _ := fhirpath.Evaluate(resource, "(3.5).round()")
// 4 (default precision is 0)

result, _ := fhirpath.Evaluate(resource, "(2.345).round(1)")
// 2.3
```

**Casos Limite / Notas:**

- Si no se especifica la precision, el valor por defecto es `0` (redondea al entero mas cercano).
- Si la entrada es un `Integer`, se devuelve sin cambios.
- Utiliza redondeo bancario (redondeo a par) mediante la biblioteca `shopspring/decimal`.
- Devuelve una coleccion vacia si la entrada esta vacia o no es numerica.

---

## exp

Devuelve *e* elevado a la potencia del valor de entrada.

**Firma:**

```text
exp() : Decimal
```

**Tipo de Retorno:** `Decimal`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(0).exp()")
// 1.0 (e^0 = 1)

result, _ := fhirpath.Evaluate(resource, "(1).exp()")
// 2.718281828... (e^1 = e)

result, _ := fhirpath.Evaluate(resource, "(2).exp()")
// 7.389056099...
```

**Casos Limite / Notas:**

- Siempre devuelve un `Decimal`, incluso si la entrada es un `Integer`.
- Utiliza la funcion `math.Exp` de Go.
- Devuelve una coleccion vacia si la entrada esta vacia o no es numerica.

---

## ln

Devuelve el logaritmo natural (base *e*) del valor de entrada.

**Firma:**

```text
ln() : Decimal
```

**Tipo de Retorno:** `Decimal`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(1).ln()")
// 0.0 (ln(1) = 0)

result, _ := fhirpath.Evaluate(resource, "(2.718281828).ln()")
// ~1.0

result, _ := fhirpath.Evaluate(resource, "(10).ln()")
// 2.302585093...
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si el valor de entrada es menor o igual a cero.
- Siempre devuelve un `Decimal`.
- Devuelve una coleccion vacia si la entrada esta vacia o no es numerica.

---

## log

Devuelve el logaritmo del valor de entrada con la base especificada.

**Firma:**

```text
log(base : Integer | Decimal) : Decimal
```

**Parametros:**

| Nombre   | Tipo                    | Descripcion            |
|----------|-------------------------|------------------------|
| `base`   | `Integer` o `Decimal`   | La base del logaritmo  |

**Tipo de Retorno:** `Decimal`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(100).log(10)")
// 2.0

result, _ := fhirpath.Evaluate(resource, "(8).log(2)")
// 3.0

result, _ := fhirpath.Evaluate(resource, "(27).log(3)")
// 3.0
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si el valor de entrada es menor o igual a cero.
- Devuelve una coleccion vacia si la base es menor o igual a cero, o igual a `1`.
- Se calcula como `ln(valor) / ln(base)`.
- Devuelve una coleccion vacia si la entrada esta vacia o no es numerica.

---

## power

Devuelve el valor de entrada elevado al exponente especificado.

**Firma:**

```text
power(exponent : Integer | Decimal) : Integer | Decimal
```

**Parametros:**

| Nombre       | Tipo                    | Descripcion                                  |
|--------------|-------------------------|----------------------------------------------|
| `exponent`   | `Integer` o `Decimal`   | La potencia a la que elevar la entrada       |

**Tipo de Retorno:** `Decimal`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(2).power(3)")
// 8.0

result, _ := fhirpath.Evaluate(resource, "(4).power(0.5)")
// 2.0 (square root)

result, _ := fhirpath.Evaluate(resource, "(10).power(0)")
// 1.0
```

**Casos Limite / Notas:**

- Siempre devuelve un valor `Decimal`.
- Devuelve una coleccion vacia si el resultado es `NaN` o `Inf` (por ejemplo, `0.power(-1)`).
- Devuelve una coleccion vacia si la entrada esta vacia o no es numerica.

---

## sqrt

Devuelve la raiz cuadrada del valor de entrada.

**Firma:**

```text
sqrt() : Decimal
```

**Tipo de Retorno:** `Decimal`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(16).sqrt()")
// 4.0

result, _ := fhirpath.Evaluate(resource, "(2).sqrt()")
// 1.4142135623...

result, _ := fhirpath.Evaluate(resource, "(0).sqrt()")
// 0.0
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si el valor de entrada es negativo.
- Siempre devuelve un `Decimal`.
- Equivalente a `power(0.5)`.
- Devuelve una coleccion vacia si la entrada esta vacia o no es numerica.
