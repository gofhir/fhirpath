---
title: "Funciones de Agregacion"
linkTitle: "Funciones de Agregacion"
weight: 12
description: >
  Funciones para reducir colecciones a valores unicos mediante agregacion, suma, promedio y busqueda de extremos.
---

Las funciones de agregacion reducen una coleccion de valores a un unico resultado. Son esenciales para realizar calculos sobre multiples elementos, como sumar cantidades, calcular promedios o encontrar valores minimos y maximos.

---

## aggregate

Realiza una agregacion de proposito general (fold/reduce) sobre la coleccion de entrada. Esta es la funcion de agregacion mas flexible, permitiendo logica de acumulacion personalizada.

**Firma:**
```
aggregate(aggregator : Expression [, init : Value]) : Value
```

**Parametros:**

| Nombre | Tipo | Descripcion |
|--------|------|-------------|
| `aggregator` | `Expression` | Una expresion evaluada para cada elemento. Dentro de la expresion, `$this` se refiere al elemento actual y `$total` se refiere al valor acumulado |
| `init` | `Value` | (Opcional) El valor inicial para `$total`. Por defecto es coleccion vacia |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3 | 4).aggregate($total + $this, 0)")
// 10 (sum: 0 + 1 + 2 + 3 + 4)

result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3 | 4).aggregate($total * $this, 1)")
// 24 (product: 1 * 1 * 2 * 3 * 4)

result, _ := fhirpath.Evaluate(resource, "('a' | 'b' | 'c').aggregate($total + $this, '')")
// 'abc' (string concatenation)
```

**Casos Limite / Notas:**
- La expresion `aggregator` tiene acceso a dos variables especiales:
  - `$this` -- el elemento actual que se esta procesando.
  - `$total` -- el resultado acumulado hasta el momento.
- Si no se proporciona valor `init`, `$total` comienza como una coleccion vacia.
- Esta funcion requiere un manejo especial en el evaluador para el soporte adecuado de lambda/expresion.
- Esto es equivalente a una operacion funcional `fold` o `reduce`.
- Devuelve el valor `init` (o vacio) si la coleccion de entrada esta vacia.

---

## sum

Devuelve la suma de todos los valores numericos en la coleccion de entrada.

**Firma:**
```
sum() : Integer | Decimal
```

**Tipo de Retorno:** `Integer` si todos los valores son `Integer`, `Decimal` si algun valor es `Decimal`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3 | 4).sum()")
// 10 (Integer)

result, _ := fhirpath.Evaluate(resource, "(1.5 | 2.5 | 3.0).sum()")
// 7.0 (Decimal)

result, _ := fhirpath.Evaluate(resource, "{}.sum()")
// 0 (empty collection sums to 0)
```

**Casos Limite / Notas:**
- Una coleccion vacia devuelve `0` (como `Integer`).
- Si todos los elementos son `Integer`, el resultado es `Integer`. Si algun elemento es `Decimal`, el resultado es `Decimal`.
- Devuelve una coleccion vacia si algun elemento no es numerico (segun la especificacion FHIRPath).
- Soporta cancelacion para colecciones grandes mediante verificacion de contexto.
- Utiliza aritmetica decimal precisa mediante la biblioteca `shopspring/decimal`.

---

## avg

Devuelve la media aritmetica (promedio) de todos los valores numericos en la coleccion de entrada.

**Firma:**
```
avg() : Decimal
```

**Tipo de Retorno:** `Decimal`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(1 | 2 | 3 | 4).avg()")
// 2.5

result, _ := fhirpath.Evaluate(resource, "(10 | 20 | 30).avg()")
// 20.0

result, _ := fhirpath.Evaluate(resource, "(5).avg()")
// 5.0 (single element)
```

**Casos Limite / Notas:**
- Siempre devuelve un `Decimal`, incluso si todas las entradas son `Integer`.
- Devuelve una coleccion vacia si la entrada esta vacia.
- Devuelve una coleccion vacia si algun elemento no es numerico.
- Se calcula como `sum() / count()` usando aritmetica decimal precisa.
- Soporta cancelacion para colecciones grandes.

---

## min

Devuelve el valor minimo de la coleccion de entrada. Funciona con tipos numericos, cadenas, fechas, fechas-hora y horas.

**Firma:**
```
min() : Value
```

**Tipo de Retorno:** El mismo tipo que los elementos de entrada

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(3 | 1 | 4 | 1 | 5).min()")
// 1

result, _ := fhirpath.Evaluate(resource, "('cherry' | 'apple' | 'banana').min()")
// 'apple' (lexicographic comparison)

result, _ := fhirpath.Evaluate(resource, "(@2024-01-01 | @2024-06-15 | @2024-03-20).min()")
// @2024-01-01
```

**Casos Limite / Notas:**
- Devuelve una coleccion vacia si la entrada esta vacia.
- Devuelve una coleccion vacia si la coleccion contiene tipos no soportados.
- Tipos soportados para comparacion:
  - `Integer` y `Decimal` (comparacion numerica)
  - `String` (comparacion lexicografica)
  - `Date` (comparacion cronologica)
  - `DateTime` (comparacion cronologica)
  - `Time` (comparacion cronologica)
- Todos los elementos de la coleccion deben ser del mismo tipo para obtener resultados significativos.
- Soporta cancelacion para colecciones grandes.

---

## max

Devuelve el valor maximo de la coleccion de entrada. Funciona con tipos numericos, cadenas, fechas, fechas-hora y horas.

**Firma:**
```
max() : Value
```

**Tipo de Retorno:** El mismo tipo que los elementos de entrada

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "(3 | 1 | 4 | 1 | 5).max()")
// 5

result, _ := fhirpath.Evaluate(resource, "('cherry' | 'apple' | 'banana').max()")
// 'cherry' (lexicographic comparison)

result, _ := fhirpath.Evaluate(resource, "(@2024-01-01 | @2024-06-15 | @2024-03-20).max()")
// @2024-06-15
```

**Casos Limite / Notas:**
- Devuelve una coleccion vacia si la entrada esta vacia.
- Devuelve una coleccion vacia si la coleccion contiene tipos no soportados.
- Los tipos soportados son los mismos que `min()`: `Integer`, `Decimal`, `String`, `Date`, `DateTime`, `Time`.
- Todos los elementos de la coleccion deben ser del mismo tipo para obtener resultados significativos.
- Soporta cancelacion para colecciones grandes.

---

## Comparacion de Funciones de Agregacion

| Funcion | Entrada | Coleccion Vacia | Elementos No Numericos | Tipo de Retorno |
|---------|---------|-----------------|------------------------|-----------------|
| `sum()` | Numerico | `0` | Coleccion vacia | `Integer` o `Decimal` |
| `avg()` | Numerico | Coleccion vacia | Coleccion vacia | `Decimal` |
| `min()` | Cualquier comparable | Coleccion vacia | Coleccion vacia | Mismo que la entrada |
| `max()` | Cualquier comparable | Coleccion vacia | Coleccion vacia | Mismo que la entrada |
| `aggregate()` | Cualquiera | Valor `init` | N/A (logica personalizada) | Cualquiera |

### Uso de aggregate para Calculos Personalizados

La funcion `aggregate` puede expresar cualquier reduccion que `sum`, `avg`, `min` o `max` realizan, ademas de logica personalizada:

```go
// Custom: running maximum
result, _ := fhirpath.Evaluate(resource,
    "(3 | 1 | 4 | 1 | 5).aggregate(iif($this > $total, $this, $total), 0)")
// 5

// Custom: count of values greater than 2
result, _ := fhirpath.Evaluate(resource,
    "(3 | 1 | 4 | 1 | 5).aggregate(iif($this > 2, $total + 1, $total), 0)")
// 3
```
