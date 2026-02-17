---
title: "Funciones Especificas de FHIR"
linkTitle: "Funciones Especificas de FHIR"
weight: 11
description: >
  Funciones especificas de recursos FHIR, incluyendo acceso a extensiones, resolucion de referencias, validacion de terminologia y conformidad de perfiles.
---

Las funciones especificas de FHIR extienden la especificacion base de FHIRPath con operaciones que son unicas del modelo de datos FHIR. Estas incluyen acceso a extensiones, resolucion de referencias, verificacion de membresia en terminologia y validacion de conformidad con perfiles. Varias de estas funciones requieren que se configuren servicios externos a traves del contexto de evaluacion.

---

## extension

Devuelve las extensiones que coinciden con la URL dada de los elementos de entrada.

**Firma:**
```
extension(url : String) : Collection
```

**Parametros:**

| Nombre | Tipo | Descripcion |
|--------|------|-------------|
| `url` | `String` | La URL canonica que identifica la extension |

**Tipo de Retorno:** `Collection` de objetos de extension

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient,
    "Patient.extension('http://hl7.org/fhir/StructureDefinition/patient-birthPlace')")
// Returns birth place extensions

result, _ := fhirpath.Evaluate(patient,
    "Patient.extension('http://example.org/fhir/StructureDefinition/custom-ext')")
// Returns custom extensions matching the URL

result, _ := fhirpath.Evaluate(patient,
    "Patient.name.extension('http://hl7.org/fhir/StructureDefinition/iso21090-EN-representation')")
// Returns extensions on name elements
```

**Casos Limite / Notas:**
- Busca el arreglo `extension` en cada elemento de entrada y filtra por el campo `url`.
- Solo funciona con elementos complejos (`ObjectValue`) que tienen un campo `extension`.
- Devuelve una coleccion vacia si ninguna extension coincide o si la entrada no tiene extensiones.
- Devuelve una coleccion vacia si la entrada esta vacia.

---

## hasExtension

Devuelve `true` si algun elemento en la coleccion de entrada tiene una extension con la URL dada.

**Firma:**
```
hasExtension(url : String) : Boolean
```

**Parametros:**

| Nombre | Tipo | Descripcion |
|--------|------|-------------|
| `url` | `String` | La URL canonica que identifica la extension |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient,
    "Patient.hasExtension('http://hl7.org/fhir/StructureDefinition/patient-birthPlace')")
// true if the patient has a birth place extension

result, _ := fhirpath.Evaluate(patient,
    "Patient.name.hasExtension('http://example.org/some-extension')")
// true if any name entry has the specified extension

result, _ := fhirpath.Evaluate(patient,
    "Patient.hasExtension('http://nonexistent.org/extension')")
// false
```

**Casos Limite / Notas:**
- Internamente llama a `extension(url)` y verifica si el resultado no esta vacio.
- Devuelve `false` si la entrada esta vacia.

---

## getExtensionValue

Devuelve el valor de las extensiones que coinciden con la URL dada. Extrae el elemento `value[x]` de cada extension coincidente.

**Firma:**
```
getExtensionValue(url : String) : Collection
```

**Parametros:**

| Nombre | Tipo | Descripcion |
|--------|------|-------------|
| `url` | `String` | La URL canonica que identifica la extension |

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient,
    "Patient.getExtensionValue('http://hl7.org/fhir/StructureDefinition/patient-birthPlace')")
// Returns the value (e.g., an Address) from the birth place extension

result, _ := fhirpath.Evaluate(patient,
    "Patient.getExtensionValue('http://example.org/fhir/StructureDefinition/score')")
// Returns the value (e.g., a Decimal or Integer) from the score extension

result, _ := fhirpath.Evaluate(patient,
    "Patient.getExtensionValue('http://example.org/fhir/StructureDefinition/preferred-language')")
// Returns the valueString or valueCoding from the extension
```

**Casos Limite / Notas:**
- Busca campos `value[x]` en cada objeto de extension coincidente. Los nombres de campo de valor soportados incluyen:
  `valueString`, `valueBoolean`, `valueInteger`, `valueDecimal`, `valueDate`, `valueDateTime`, `valueTime`, `valueCode`, `valueCoding`, `valueCodeableConcept`, `valueQuantity`, `valueReference`, `valueIdentifier`, `valuePeriod`, `valueRange`, `valueRatio`, `valueAttachment`, `valueUri`, `valueUrl`, `valueCanonical`.
- Devuelve solo el primer campo `value[x]` encontrado para cada extension (en el orden listado arriba).
- Devuelve una coleccion vacia si ninguna extension coincide o si las extensiones no tienen valor.

---

## resolve

Resuelve una referencia FHIR al recurso referenciado. Esta funcion requiere que se configure un `ReferenceResolver` en el contexto de evaluacion.

**Firma:**
```
resolve() : Collection
```

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
// With a resolver configured:
compiled := fhirpath.MustCompile("Observation.subject.resolve()")
result, _ := compiled.EvaluateWithOptions(resource, fhirpath.WithResolver(myResolver))
// Returns the Patient resource referenced by Observation.subject

compiled = fhirpath.MustCompile("Observation.subject.resolve().name.first().family")
result, _ = compiled.EvaluateWithOptions(resource, fhirpath.WithResolver(myResolver))
// Returns the family name of the referenced patient

compiled = fhirpath.MustCompile("MedicationRequest.medication.resolve()")
result, _ = compiled.EvaluateWithOptions(resource, fhirpath.WithResolver(myResolver))
// Resolves the medication reference
```

**Casos Limite / Notas:**
- Requiere que se establezca un `ReferenceResolver` en el contexto de evaluacion. Sin uno, devuelve una coleccion vacia.
- Maneja tanto referencias de cadena (`"Patient/123"`) como objetos Reference (con un campo `reference`).
- Las referencias que no pueden resolverse se omiten silenciosamente (no se genera error).
- El recurso resuelto se analiza desde JSON al sistema de tipos FHIRPath.
- Multiples referencias en la coleccion de entrada se resuelven individualmente.

---

## getReferenceKey

Extrae el tipo de recurso y/o ID de una cadena de referencia FHIR.

**Firma:**
```
getReferenceKey([part : String]) : String
```

**Parametros:**

| Nombre | Tipo | Descripcion |
|--------|------|-------------|
| `part` | `String` | (Opcional) Que parte extraer: `'type'`, `'id'`, o `'key'` (por defecto). `'key'` devuelve el `ResourceType/id` completo |

**Tipo de Retorno:** `Collection` de `String`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(resource, "Observation.subject.getReferenceKey()")
// "Patient/123" (full key)

result, _ := fhirpath.Evaluate(resource, "Observation.subject.getReferenceKey('type')")
// "Patient"

result, _ := fhirpath.Evaluate(resource, "Observation.subject.getReferenceKey('id')")
// "123"
```

**Casos Limite / Notas:**
- Maneja tanto valores de cadena como objetos Reference (con un campo `reference`).
- Elimina prefijos de URL: `"http://example.org/fhir/Patient/123"` extrae `"Patient/123"`.
- Devuelve una coleccion vacia si la entrada esta vacia o no contiene referencias validas.

---

## memberOf

Verifica si un codigo, Coding o CodeableConcept es miembro de un ValueSet especificado. Esta funcion requiere que se configure un `TerminologyService` en el contexto de evaluacion.

**Firma:**
```
memberOf(valueSetUrl : String) : Boolean
```

**Parametros:**

| Nombre | Tipo | Descripcion |
|--------|------|-------------|
| `valueSetUrl` | `String` | La URL canonica del ValueSet contra el cual verificar membresia |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
// With a terminology service configured:
compiled := fhirpath.MustCompile(
    "Observation.code.memberOf('http://hl7.org/fhir/ValueSet/observation-codes')")
result, _ := compiled.EvaluateWithOptions(resource,
    fhirpath.WithTerminologyService(myTermService))
// true if the observation code is in the specified ValueSet

compiled = fhirpath.MustCompile(
    "Patient.gender.memberOf('http://hl7.org/fhir/ValueSet/administrative-gender')")
result, _ = compiled.EvaluateWithOptions(resource,
    fhirpath.WithTerminologyService(myTermService))
// true if the gender code is in the administrative-gender ValueSet

compiled = fhirpath.MustCompile(
    "Condition.code.coding.memberOf('http://hl7.org/fhir/ValueSet/condition-code')")
result, _ = compiled.EvaluateWithOptions(resource,
    fhirpath.WithTerminologyService(myTermService))
// true if any coding is in the condition-code ValueSet
```

**Casos Limite / Notas:**
- Requiere que se establezca un `TerminologyService` en el contexto de evaluacion. Sin uno, devuelve una coleccion vacia (desconocido).
- Soporta tres tipos de entrada:
  - **String** -- se trata como un valor de codigo simple.
  - **Objeto Coding** -- extrae los campos `system`, `code`, `version` y `display`.
  - **Objeto CodeableConcept** -- extrae el arreglo `coding` y el campo `text`.
- Devuelve `true` si algun elemento en la coleccion de entrada es miembro.
- Devuelve `false` si los elementos fueron verificados pero ninguno es miembro.
- Devuelve una coleccion vacia si la verificacion no puede realizarse (sin servicio de terminologia o errores).

---

## conformsTo

Verifica si un recurso conforma a un perfil FHIR especificado (StructureDefinition). Esta funcion requiere que se configure un `ProfileValidator` en el contexto de evaluacion.

**Firma:**
```
conformsTo(profileUrl : String) : Boolean
```

**Parametros:**

| Nombre | Tipo | Descripcion |
|--------|------|-------------|
| `profileUrl` | `String` | La URL canonica del StructureDefinition contra el cual validar |

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
// With a profile validator configured:
compiled := fhirpath.MustCompile(
    "conformsTo('http://hl7.org/fhir/StructureDefinition/Patient')")
result, _ := compiled.EvaluateWithOptions(resource,
    fhirpath.WithProfileValidator(myValidator))
// true if the resource conforms to the Patient profile

compiled = fhirpath.MustCompile(
    "conformsTo('http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient')")
result, _ = compiled.EvaluateWithOptions(resource,
    fhirpath.WithProfileValidator(myValidator))
// true if the resource conforms to US Core Patient profile
```

**Casos Limite / Notas:**
- Requiere que se establezca un `ProfileValidator` en el contexto de evaluacion. Sin uno, devuelve una coleccion vacia (desconocido).
- Opera sobre tipos complejos (`ObjectValue`) con datos JSON sin procesar disponibles.
- Devuelve una coleccion vacia si la validacion no puede realizarse (sin validador, sin datos sin procesar o errores).
- La validacion de perfiles puede involucrar llamadas de red dependiendo de la implementacion de su `ProfileValidator`.

---

## hasValue

Devuelve `true` si la entrada contiene un valor primitivo (Boolean, String, Integer, Decimal, Date, DateTime o Time).

**Firma:**
```
hasValue() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.active.hasValue()")
// true (active is a boolean primitive)

result, _ := fhirpath.Evaluate(patient, "Patient.name.hasValue()")
// false (name is a complex type, not a primitive)

result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.hasValue()")
// true (birthDate is a date primitive)
```

**Casos Limite / Notas:**
- Devuelve `false` para entrada vacia.
- Devuelve `true` si **algun** elemento en la coleccion tiene un valor primitivo.
- Los tipos complejos (objetos) no cuentan como si tuvieran un valor, incluso si contienen campos primitivos.

---

## getValue

Devuelve los valores primitivos de la coleccion de entrada. Los tipos complejos se filtran.

**Firma:**
```
getValue() : Collection
```

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.active.getValue()")
// Returns the boolean value of active

result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.getValue()")
// Returns the date value

result, _ := fhirpath.Evaluate(patient, "Patient.name.getValue()")
// { } (empty - name entries are complex types, not primitives)
```

**Casos Limite / Notas:**
- Devuelve una coleccion vacia si la entrada esta vacia.
- Filtra la coleccion de entrada para incluir solo tipos primitivos: `Boolean`, `String`, `Integer`, `Decimal`, `Date`, `DateTime`, `Time`.
- Los tipos complejos (`ObjectValue`) se excluyen del resultado.
