---
title: "Funciones Especificas de FHIR®"
linkTitle: "Funciones Especificas de FHIR®"
weight: 11
description: >
  Funciones especificas de recursos FHIR®, incluyendo acceso a extensiones, resolucion de referencias, validacion de terminologia y conformidad de perfiles.
---

Las funciones especificas de FHIR® extienden la especificacion base de FHIRPath con operaciones que son unicas del modelo de datos FHIR®. Estas incluyen acceso a extensiones, resolucion de referencias, verificacion de membresia en terminologia y validacion de conformidad con perfiles. Varias de estas funciones requieren que se configuren servicios externos a traves del contexto de evaluacion.

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

Resuelve una referencia FHIR® al recurso referenciado. Una referencia dentro del documento evaluado — un recurso contenido, o una entrada de un Bundle — se resuelve sin ayuda; cualquier otra necesita un `ReferenceResolver` configurado en el contexto de evaluacion.

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
- Una referencia se resuelve desde donde esta escrita, como la define FHIR, antes de consultar al `ReferenceResolver`:
  - Un fragmento, `#id`, nombra un recurso contenido en el recurso que hace la referencia; `#` solo nombra a ese recurso desde uno de sus recursos contenidos, y a nada desde un recurso que no esta contenido. Ambos valen tambien cuando la expresion se evalua sobre un recurso contenido. En un recurso devuelto por un `ReferenceResolver`, se buscan sus propios recursos contenidos.
  - En un Bundle, una referencia relativa, `Type/id`, toma la base del `fullUrl` RESTful de la entrada que la contiene (bundle.html#references): `Observation/x` desde la entrada en `http://b.org/fhir/` nombra `http://b.org/fhir/Observation/x`, no el `Observation/x` de otro servidor. Si no encuentra nada, se compara por tipo e id con una entrada cuyo `fullUrl` no nombra un servidor (`urn:uuid:`, o ninguno), como escribe una transaccion lo que crea. Si el `fullUrl` de la entrada que la escribe no es RESTful (`urn:uuid:`) o falta, la referencia se compara con el tipo e id de los recursos de las entradas. Cualquier otra referencia se compara con el `fullUrl`, y una versionada, `.../_history/2`, tambien con `meta.versionId`. Una entrada cuyo `fullUrl` es la referencia tal como esta escrita siempre coincide, y un `fullUrl` o una referencia coinciden con o sin barra final. Un `fullUrl` con version, `.../_history/1`, que bdl-8 prohibe, coincide con su recurso, y con una referencia versionada solo si es la misma version. De varias coincidencias, que FHIR llama ambiguas, se toma la primera. El Bundle es el mas cercano que contiene la referencia, asi que una referencia dentro de un documento contenido en una coleccion se resuelve en el documento, y en el Bundle evaluado, desde la misma entrada, cuando el documento no contiene su destino. Si se evalua sobre el recurso de una entrada de un Bundle, se busca en el Bundle mas cercano por encima, desde esa entrada, y en ninguno por encima de ese; si se evalua sobre un Bundle, no se busca en ninguno por encima.
  - Esto aplica a un objeto Reference, que sabe desde donde se leyo. Una referencia leida como cadena, `reference.resolve()`, se resuelve desde la raiz de la expresion.
- Sin un `ReferenceResolver`, una referencia fuera del documento no resuelve a nada.
- Maneja tanto referencias de cadena (`"Patient/123"`) como objetos Reference (con un campo `reference`).
- Las referencias que no pueden resolverse se omiten silenciosamente (no se genera error).
- El recurso resuelto se analiza desde JSON al sistema de tipos FHIRPath.
- Multiples referencias en la coleccion de entrada se resuelven individualmente.

---

## getReferenceKey

Extrae el tipo de recurso y/o ID de una cadena de referencia FHIR®.

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
// The terminology service is set on the evaluation context:
ctx := eval.NewContext(resource)
ctx.SetTerminologyService(myTermService)

compiled := fhirpath.MustCompile(
    "Observation.code.memberOf('http://hl7.org/fhir/ValueSet/observation-codes')")
result, _ := compiled.EvaluateWithContext(ctx)
// true if the observation code is in the specified ValueSet
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
- Un value set que no es una URI, `memberOf('not a uri')`, es un error con las reglas de R4 y vacio con las de R5.

---

## conformsTo

Verifica si un recurso conforma a un perfil FHIR® especificado (StructureDefinition). Esta funcion requiere que se configure un `ProfileValidator` en el contexto de evaluacion.

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
// The profile validator is set on the evaluation context:
ctx := eval.NewContext(resource)
ctx.SetProfileValidator(myValidator)

compiled := fhirpath.MustCompile(
    "conformsTo('http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient')")
result, _ := compiled.EvaluateWithContext(ctx)
// true if the resource conforms to US Core Patient profile
```

**Casos Limite / Notas:**
- Requiere que se establezca un `ProfileValidator` en el contexto de evaluacion. Sin uno, devuelve una coleccion vacia (desconocido).
- Opera sobre tipos complejos (`ObjectValue`) con datos JSON sin procesar disponibles.
- Devuelve una coleccion vacia si la validacion no puede realizarse (sin validador, sin datos sin procesar o errores).
- La validacion de perfiles puede involucrar llamadas de red dependiendo de la implementacion de su `ProfileValidator`.
- Mas de un elemento es un error con las reglas de R4 y vacio con las de R5.

## subsumes

Devuelve `true` si el Coding o CodeableConcept de entrada es equivalente al dado, o lo subsume.

**Firma:**
```
subsumes(code : Coding | CodeableConcept) : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(observation, "Observation.category.subsumes(Observation.category.coding[0])")
// true: a Coding is equivalent to itself, by system and code
```

**Casos Limite / Notas:**
- La equivalencia no necesita terminologia: dos Coding son equivalentes cuando lo son su `system` y su `code`, y un CodeableConcept cuenta si alguno de sus Coding lo es.
- Mas alla de la equivalencia, la funcion consulta al `TerminologyService` del contexto de evaluacion, si tambien implementa `eval.Subsumer`. Sin uno el resultado es vacio (desconocido).
- Devuelve vacio si la entrada o el argumento estan vacios.
- Mas de un elemento en cualquiera de los lados es un error con las reglas de R4 y vacio con las de R5.

---

## subsumedBy

Devuelve `true` si el Coding o CodeableConcept de entrada es equivalente al dado, o esta subsumido por el. Es `subsumes()` con los lados invertidos, y por lo demas se comporta igual.

**Firma:**
```
subsumedBy(code : Coding | CodeableConcept) : Boolean
```

**Tipo de Retorno:** `Boolean`

---

## htmlChecks

Devuelve `true` si el xhtml de una narrativa cumple las reglas de FHIR para HTML, y `false` si no.

**Firma:**
```
htmlChecks() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
compiled := fhirpath.MustCompile("text.`div`.htmlChecks()")
result, _ := compiled.EvaluateWithOptions(patient,
    fhirpath.WithModel(r4.FHIRPathModel()),
    fhirpath.WithHTMLChecker(myChecker))
// true if the narrative passes the checker
```

**Casos Limite / Notas:**
- `div` es una palabra clave, asi que la narrativa se escribe `` text.`div` ``.
- Las reglas son de FHIR, asi que las aporta quien llama: la funcion consulta al `HTMLChecker` dado con `WithHTMLChecker`, o fijado con `eval.Context.SetHTMLChecker`.
- Un elemento es xhtml por el tipo que le da el modelo; sin modelo ninguno lo es.
- Devuelve vacio en cualquier otro tipo de elemento, y con mas de un elemento.
- En un unico elemento xhtml sin checker, la evaluacion termina con un error.

---

## checkModifiers

Devuelve la entrada si ningun elemento tiene una extension modificadora distinta de las nombradas, y termina la evaluacion con un error en otro caso.

**Firma:**
```
checkModifiers(modifier : String) : Collection
```

**Tipo de Retorno:** `Collection`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient,
    "Patient.checkModifiers('http://example.org/a,http://example.org/b').id")
// the id, if Patient's modifierExtension urls are among those two
```

**Casos Limite / Notas:**
- Los modificadores se dan como una cadena separada por comas o, como lo escribe R4, como una coleccion de cadenas.
- Solo se revisa el `modifierExtension` propio de cada elemento, no el de sus hijos.
- Un modificador vacio es un error con las reglas de R5.

---

## elementDefinition

Devuelve la definicion de elemento FHIR de cada elemento de la entrada.

**Firma:**
```
elementDefinition() : Collection
```

**Casos Limite / Notas:**
- Devuelve vacio para una entrada vacia.
- El motor no tiene las definiciones de elementos, asi que una entrada no vacia termina la evaluacion con un error.

---

## slice

Devuelve los elementos de la entrada que coinciden con un slice de una definicion de estructura.

**Firma:**
```
slice(structure : String, name : String) : Collection
```

**Casos Limite / Notas:**
- Devuelve vacio para una entrada vacia.
- Una estructura que no es una URI, o un argumento vacio, no se puede resolver: error con las reglas de R4, vacio con las de R5.
- El motor no tiene las definiciones de estructura, asi que una estructura que es una URI termina la evaluacion con un error.

---

## hasValue

Devuelve `true` si la entrada es un único primitivo FHIR con valor, o un único valor System de un tipo primitivo.

**Firma:**
```
hasValue() : Boolean
```

**Tipo de Retorno:** `Boolean`

**Ejemplos:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.active.hasValue()")
// true (active is a boolean primitive)

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().hasValue()")
// false (a complex value is no FHIR primitive)

result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.hasValue()")
// true (birthDate is a date primitive)
```

**Casos Limite / Notas:**
- Devuelve `false` para entrada vacia.
- Devuelve `false` para mas de un elemento.
- Un primitivo escrito con extensiones pero sin valor no tiene valor: `false`.
- Un valor System de un tipo primitivo tiene valor, `'a'.hasValue()` es `true`, como responden fhirpath.js y el motor Java de HL7; una cantidad no lo tiene. `getValue()` de un valor System sigue vacio.
- Un valor complejo no tiene valor: `false`, como define FHIR la funcion y responde fhirpath.js, mientras el motor Java de HL7 responde `true`. Ver [CONFORMANCE.md](https://github.com/gofhir/fhirpath/blob/main/CONFORMANCE.md#measured-and-not-followed).

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
