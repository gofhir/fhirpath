---
title: "Funciones de Cadena"
linkTitle: "Funciones de Cadena"
weight: 1
description: >
  Funciones para manipular e inspeccionar valores de cadena en expresiones FHIRPath.
---

Las funciones de cadena operan sobre valores `String` y proporcionan capacidades comunes de manipulacion de texto. Cuando se invocan sobre una coleccion vacia, todas las funciones de cadena devuelven una coleccion vacia. Si la entrada no es de tipo cadena, las funciones generalmente devuelven una coleccion vacia en lugar de generar un error.

---

## startsWith

Devuelve `true` si la cadena de entrada comienza con el prefijo dado.

**Firma:**

```text
startsWith(prefix : String) : Boolean
```

**Parametros:**

| Nombre     | Tipo     | Descripcion              |
|------------|----------|--------------------------|
| `prefix`   | `String` | El prefijo a verificar   |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.startsWith('Smi')")
// For family = "Smith" -> true

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.startsWith('Jon')")
// For family = "Smith" -> false

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.startsWith('')")
// Any string starts with empty string -> true
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia.
- Un prefijo vacio (`''`) siempre devuelve `true` para cualquier cadena no vacia.
- La comparacion distingue entre mayusculas y minusculas.

---

## endsWith

Devuelve `true` si la cadena de entrada termina con el sufijo dado.

**Firma:**

```text
endsWith(suffix : String) : Boolean
```

**Parametros:**

| Nombre     | Tipo     | Descripcion              |
|------------|----------|--------------------------|
| `suffix`   | `String` | El sufijo a verificar    |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.endsWith('ith')")
// For family = "Smith" -> true

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.endsWith('.pdf')")
// For family = "Smith" -> false

result, _ := fhirpath.Evaluate(resource, "Patient.id.endsWith('')")
// Any string ends with empty string -> true
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia.
- Un sufijo vacio (`''`) siempre devuelve `true`.
- La comparacion distingue entre mayusculas y minusculas.

---

## contains

Devuelve `true` si la cadena de entrada contiene la subcadena dada.

**Firma:**

```text
contains(substring : String) : Boolean
```

**Parametros:**

| Nombre        | Tipo     | Descripcion                    |
|---------------|----------|--------------------------------|
| `substring`   | `String` | La subcadena a buscar          |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.contains('mit')")
// For family = "Smith" -> true

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.contains('xyz')")
// For family = "Smith" -> false

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.contains('')")
// Any string contains the empty string -> true
```

**Casos Limite / Notas:**

- Devuelve una coleccion vacia si la entrada esta vacia.
- Una subcadena vacia (`''`) siempre devuelve `true`.
- La busqueda distingue entre mayusculas y minusculas.

---

## replace

Reemplaza todas las ocurrencias de una cadena patron con una cadena de sustitucion.

**Firma:**

```text
replace(pattern : String, substitution : String) : String
```

**Parametros:**

| Nombre           | Tipo     | Descripcion                            |
|------------------|----------|----------------------------------------|
| `pattern`        | `String` | La cadena literal a buscar             |
| `substitution`   | `String` | La cadena de reemplazo                 |

**Tipo de Retorno:** `String`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.replace('Smith', 'Jones')")
// "Smith" -> "Jones"

result, _ := fhirpath.Evaluate(resource, "'hello-world'.replace('-', '_')")
// "hello-world" -> "hello_world"

result, _ := fhirpath.Evaluate(resource, "'aaa'.replace('a', 'bb')")
// "aaa" -> "bbbbbb"
```

**Casos Limite / Notas:**

- Reemplaza **todas** las ocurrencias, no solo la primera.
- Este es un reemplazo de cadena literal, no un reemplazo de expresion regular. Use `replaceMatches` para expresiones regulares.
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## matches

Devuelve `true` si la cadena de entrada coincide con la expresion regular dada.

**Firma:**

```text
matches(regex : String) : Boolean
```

**Parametros:**

| Nombre    | Tipo     | Descripcion                          |
|-----------|----------|--------------------------------------|
| `regex`   | `String` | Un patron de expresion regular       |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'ABC'.matches('[A-Z]{3}')")
// true

result, _ := fhirpath.Evaluate(resource, "'abc123'.matches('[a-z]+\\d+')")
// true

result, _ := fhirpath.Evaluate(resource, "'hello'.matches('^[0-9]+$')")
// false
```

**Casos Limite / Notas:**

- Utiliza el paquete `regexp` de Go para la coincidencia de patrones.
- La expresion regular se compila con cache e incluye proteccion contra tiempo de espera de ReDoS (Denegacion de Servicio por Expresion Regular).
- Devuelve una coleccion vacia si la entrada esta vacia.
- La expresion regular debe coincidir con la cadena **completa** (anclada). Use `.*patron.*` para coincidencia parcial si es necesario.

---

## replaceMatches

Reemplaza todas las ocurrencias de un patron de expresion regular con la cadena de sustitucion.

**Firma:**

```text
replaceMatches(regex : String, substitution : String) : String
```

**Parametros:**

| Nombre           | Tipo     | Descripcion                          |
|------------------|----------|--------------------------------------|
| `regex`          | `String` | Un patron de expresion regular       |
| `substitution`   | `String` | La cadena de reemplazo               |

**Tipo de Retorno:** `String`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'hello   world'.replaceMatches('\\\\s+', ' ')")
// "hello   world" -> "hello world"

result, _ := fhirpath.Evaluate(resource, "'abc123def'.replaceMatches('[0-9]+', 'NUM')")
// "abc123def" -> "abcNUMdef"

result, _ := fhirpath.Evaluate(resource, "'2024-01-15'.replaceMatches('(\\\\d{4})-(\\\\d{2})-(\\\\d{2})', '$2/$3/$1')")
// "2024-01-15" -> "01/15/2024"
```

**Casos Limite / Notas:**

- Utiliza el paquete `regexp` de Go con compilacion en cache y proteccion contra tiempo de espera de ReDoS.
- La sustitucion soporta referencias hacia atras (`$1`, `$2`, etc.).
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## indexOf

Devuelve el indice basado en cero de la primera ocurrencia de la subcadena dada, o `-1` si no se encuentra.

**Firma:**

```text
indexOf(substring : String) : Integer
```

**Parametros:**

| Nombre        | Tipo     | Descripcion                    |
|---------------|----------|--------------------------------|
| `substring`   | `String` | La subcadena a buscar          |

**Tipo de Retorno:** `Integer`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'hello world'.indexOf('world')")
// 6

result, _ := fhirpath.Evaluate(resource, "'hello world'.indexOf('xyz')")
// -1

result, _ := fhirpath.Evaluate(resource, "'abcabc'.indexOf('bc')")
// 1 (first occurrence)
```

**Casos Limite / Notas:**

- Devuelve `-1` cuando la subcadena no se encuentra.
- Devuelve `0` cuando se busca una cadena vacia.
- Devuelve una coleccion vacia si la entrada esta vacia.
- La busqueda distingue entre mayusculas y minusculas.

---

## substring

Devuelve una subcadena comenzando en el indice basado en cero dado, opcionalmente limitada a una longitud especificada.

**Firma:**

```text
substring(start : Integer [, length : Integer]) : String
```

**Parametros:**

| Nombre     | Tipo      | Descripcion                                                    |
|------------|-----------|----------------------------------------------------------------|
| `start`    | `Integer` | Indice de inicio basado en cero                                |
| `length`   | `Integer` | (Opcional) Numero maximo de caracteres a devolver              |

**Tipo de Retorno:** `String`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'hello world'.substring(6)")
// "world"

result, _ := fhirpath.Evaluate(resource, "'hello world'.substring(0, 5)")
// "hello"

result, _ := fhirpath.Evaluate(resource, "'abc'.substring(1, 10)")
// "bc" (length exceeds string, returns to end)
```

**Casos Limite / Notas:**

- Si `start` es negativo o mayor o igual a la longitud de la cadena, devuelve una coleccion vacia.
- Si `length` se extenderia mas alla del final de la cadena, devuelve los caracteres hasta el final.
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## lower

Devuelve la cadena de entrada convertida a minusculas.

**Firma:**

```text
lower() : String
```

**Tipo de Retorno:** `String`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'Hello World'.lower()")
// "hello world"

result, _ := fhirpath.Evaluate(resource, "'ABC123'.lower()")
// "abc123"

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.lower()")
// "Smith" -> "smith"
```

**Casos Limite / Notas:**

- Los caracteres no alfabeticos no se modifican.
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## upper

Devuelve la cadena de entrada convertida a mayusculas.

**Firma:**

```text
upper() : String
```

**Tipo de Retorno:** `String`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'Hello World'.upper()")
// "HELLO WORLD"

result, _ := fhirpath.Evaluate(resource, "'abc123'.upper()")
// "ABC123"

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.upper()")
// "Smith" -> "SMITH"
```

**Casos Limite / Notas:**

- Los caracteres no alfabeticos no se modifican.
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## length

Devuelve el numero de caracteres en la cadena de entrada.

**Firma:**

```text
length() : Integer
```

**Tipo de Retorno:** `Integer`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'hello'.length()")
// 5

result, _ := fhirpath.Evaluate(resource, "''.length()")
// 0

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().family.length()")
// "Smith" -> 5
```

**Casos Limite / Notas:**

- Devuelve la longitud en bytes de la cadena (`len` de Go), no el conteo de runas. Para cadenas ASCII, ambos son identicos.
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## toChars

Convierte una cadena en una coleccion de caracteres individuales, donde cada caracter es un elemento `String` separado.

**Firma:**

```text
toChars() : Collection
```

**Tipo de Retorno:** `Collection` de `String`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'abc'.toChars()")
// { "a", "b", "c" }

result, _ := fhirpath.Evaluate(resource, "'Hi'.toChars().count()")
// 2

result, _ := fhirpath.Evaluate(resource, "''.toChars()")
// { } (empty collection)
```

**Casos Limite / Notas:**

- Itera sobre runas Unicode, por lo que los caracteres multibyte se manejan correctamente.
- Una cadena vacia devuelve una coleccion vacia.
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## trim

Elimina los espacios en blanco iniciales y finales de la cadena de entrada.

**Firma:**

```text
trim() : String
```

**Tipo de Retorno:** `String`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'  hello  '.trim()")
// "hello"

result, _ := fhirpath.Evaluate(resource, "'\\thello\\n'.trim()")
// "hello"

result, _ := fhirpath.Evaluate(resource, "'nospace'.trim()")
// "nospace" (unchanged)
```

**Casos Limite / Notas:**

- Utiliza `strings.TrimSpace` de Go, que elimina todos los caracteres de espacio en blanco Unicode (espacios, tabulaciones, saltos de linea, etc.).
- No elimina los espacios en blanco del medio de la cadena.
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## split

Divide la cadena de entrada por el separador dado y devuelve una coleccion de subcadenas.

**Firma:**

```text
split(separator : String) : Collection
```

**Parametros:**

| Nombre        | Tipo     | Descripcion                      |
|---------------|----------|----------------------------------|
| `separator`   | `String` | El delimitador para dividir      |

**Tipo de Retorno:** `Collection` de `String`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "'a,b,c'.split(',')")
// { "a", "b", "c" }

result, _ := fhirpath.Evaluate(resource, "'hello world'.split(' ')")
// { "hello", "world" }

result, _ := fhirpath.Evaluate(resource, "'no-delimiter'.split(',')")
// { "no-delimiter" } (single element)
```

**Casos Limite / Notas:**

- Si el separador no se encuentra, devuelve una coleccion con la cadena completa como un solo elemento.
- Un separador vacio divide en caracteres individuales (comportamiento de `strings.Split` de Go).
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## join

Une una coleccion de cadenas en una sola cadena, opcionalmente separada por un separador dado.

**Firma:**

```text
join([separator : String]) : String
```

**Parametros:**

| Nombre        | Tipo     | Descripcion                                                                                   |
|---------------|----------|-----------------------------------------------------------------------------------------------|
| `separator`   | `String` | (Opcional) La cadena a colocar entre elementos. Por defecto es cadena vacia (`''`)             |

**Tipo de Retorno:** `String`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.name.first().given.join(', ')")
// Given names "John", "James" -> "John, James"

result, _ := fhirpath.Evaluate(resource, "'a,b,c'.split(',').join('-')")
// "a-b-c"

result, _ := fhirpath.Evaluate(resource, "'a,b,c'.split(',').join()")
// "abc" (no separator)
```

**Casos Limite / Notas:**

- Si la coleccion de entrada esta vacia, devuelve una cadena vacia (`""`), no una coleccion vacia.
- Los elementos que no son cadenas en la coleccion se convierten a su representacion de cadena usando `.String()`.
- Si no se proporciona argumento separador, los elementos se concatenan directamente.

---

## encode / decode

{{% alert title="No Implementado" color="warning" %}}
Las funciones `encode(encoding)` y `decode(encoding)` para codificacion/decodificacion base64 estan definidas en la especificacion FHIRPath pero **aun no estan implementadas** en esta biblioteca. Llamar a estas funciones resultara en un error.
{{% /alert %}}

**Firma:**

```text
encode(encoding : String) : String
decode(encoding : String) : String
```

**Descripcion:**

- `encode('base64')` -- Codifica la cadena de entrada a base64.
- `decode('base64')` -- Decodifica una cadena codificada en base64.

Estas funciones estan planificadas para una version futura.
