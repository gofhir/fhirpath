---
title: "FHIR®-Specific Functions"
linkTitle: "FHIR®-Specific Functions"
weight: 11
description: >
  Functions specific to FHIR® resources, including extension access, reference resolution, terminology validation, and profile conformance.
---

FHIR®-specific functions extend the core FHIRPath specification with operations that are unique to the FHIR® data model. These include accessing extensions, resolving references, checking terminology membership, and validating profile conformance. Several of these functions require external services to be configured via the evaluation context.

---

## extension

Returns extensions matching the given URL from the input elements.

**Signature:**
```
extension(url : String) : Collection
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `url` | `String` | The canonical URL identifying the extension |

**Return Type:** `Collection` of extension objects

**Examples:**

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

**Edge Cases / Notes:**
- Looks for the `extension` array on each input element and filters by the `url` field.
- Only works on complex elements (`ObjectValue`) that have an `extension` field.
- Returns an empty collection if no extensions match or if the input has no extensions.
- Returns an empty collection if the input is empty.

---

## hasExtension

Returns `true` if any element in the input collection has an extension with the given URL.

**Signature:**
```
hasExtension(url : String) : Boolean
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `url` | `String` | The canonical URL identifying the extension |

**Return Type:** `Boolean`

**Examples:**

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

**Edge Cases / Notes:**
- Internally calls `extension(url)` and checks if the result is non-empty.
- Returns `false` if the input is empty.

---

## getExtensionValue

Returns the value of extensions matching the given URL. Extracts the `value[x]` element from each matching extension.

**Signature:**
```
getExtensionValue(url : String) : Collection
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `url` | `String` | The canonical URL identifying the extension |

**Return Type:** `Collection`

**Examples:**

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

**Edge Cases / Notes:**
- Searches for `value[x]` fields on each matching extension object. Supported value field names include:
  `valueString`, `valueBoolean`, `valueInteger`, `valueDecimal`, `valueDate`, `valueDateTime`, `valueTime`, `valueCode`, `valueCoding`, `valueCodeableConcept`, `valueQuantity`, `valueReference`, `valueIdentifier`, `valuePeriod`, `valueRange`, `valueRatio`, `valueAttachment`, `valueUri`, `valueUrl`, `valueCanonical`.
- Returns only the first `value[x]` field found for each extension (in the order listed above).
- Returns an empty collection if no extensions match or if extensions have no value.

---

## resolve

Resolves a FHIR® reference to the referenced resource. A reference into the document being evaluated — a contained resource, or an entry of a Bundle — resolves without help; anything else needs a `ReferenceResolver` configured in the evaluation context.

**Signature:**
```
resolve() : Collection
```

**Return Type:** `Collection`

**Examples:**

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

**Edge Cases / Notes:**
- A reference is resolved from where it is written, as FHIR defines it, before the `ReferenceResolver` is asked:
  - A fragment, `#id`, names a resource contained in the resource that makes the reference; `#` alone names that resource from one of its contained resources, and nothing from a resource that is not contained. Both hold when the expression is evaluated against a contained resource. In a resource a `ReferenceResolver` returned, its own contained resources are searched.
  - In a Bundle, a relative reference, `Type/id`, takes the base of the RESTful `fullUrl` of the entry that holds it (bundle.html#references): `Observation/x` from the entry at `http://b.org/fhir/` names `http://b.org/fhir/Observation/x`, not another server's `Observation/x`. Failing that, it matches by type and id an entry whose `fullUrl` names no server (`urn:uuid:`, or none), as a transaction writes what it creates. Where the referring `fullUrl` is not RESTful (`urn:uuid:`) or absent, the reference is matched on the type and id of the entries' resources. Any other reference is matched on `fullUrl`, and a versioned one, `.../_history/2`, also on `meta.versionId`. An entry whose `fullUrl` is the reference as written always matches, and a `fullUrl` or a reference matches with or without a trailing slash. A `fullUrl` that carries a version, `.../_history/1`, which bdl-8 forbids, matches its resource, and a versioned reference only with the same version. Of several matches, which FHIR calls ambiguous, the first is taken. The Bundle is the nearest one holding the reference, so a reference inside a document held by a collection resolves in the document, and in the Bundle being evaluated, from the same entry, when the document does not hold its target. Evaluated against a resource a Bundle entry holds, the nearest Bundle above it is searched, from that entry, and none above that; evaluated against a Bundle, no Bundle above it is.
  - This takes a Reference object, which knows where it was read from. A reference read as a bare string, `reference.resolve()`, is resolved from the root of the expression.
- Without a `ReferenceResolver`, a reference outside the document resolves to nothing.
- Handles both string references (`"Patient/123"`) and Reference objects (with a `reference` field).
- References that cannot be resolved are silently skipped (no error raised).
- The resolved resource is parsed from JSON into the FHIRPath type system.
- Multiple references in the input collection are each resolved individually.

---

## getReferenceKey

Extracts the resource type and/or ID from a FHIR® reference string.

**Signature:**
```
getReferenceKey([part : String]) : String
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `part` | `String` | (Optional) Which part to extract: `'type'`, `'id'`, or `'key'` (default). `'key'` returns the full `ResourceType/id` |

**Return Type:** `Collection` of `String`

**Examples:**

```go
result, _ := fhirpath.Evaluate(resource, "Observation.subject.getReferenceKey()")
// "Patient/123" (full key)

result, _ := fhirpath.Evaluate(resource, "Observation.subject.getReferenceKey('type')")
// "Patient"

result, _ := fhirpath.Evaluate(resource, "Observation.subject.getReferenceKey('id')")
// "123"
```

**Edge Cases / Notes:**
- Handles both string values and Reference objects (with a `reference` field).
- Strips URL prefixes: `"http://example.org/fhir/Patient/123"` extracts `"Patient/123"`.
- Returns empty collection if the input is empty or contains no valid references.

---

## memberOf

Checks if a code, Coding, or CodeableConcept is a member of a specified ValueSet. This function requires a `TerminologyService` to be configured in the evaluation context.

**Signature:**
```
memberOf(valueSetUrl : String) : Boolean
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `valueSetUrl` | `String` | The canonical URL of the ValueSet to check membership against |

**Return Type:** `Boolean`

**Examples:**

```go
// The terminology service is set on the evaluation context:
ctx := eval.NewContext(resource)
ctx.SetTerminologyService(myTermService)

compiled := fhirpath.MustCompile(
    "Observation.code.memberOf('http://hl7.org/fhir/ValueSet/observation-codes')")
result, _ := compiled.EvaluateWithContext(ctx)
// true if the observation code is in the specified ValueSet
```

**Edge Cases / Notes:**
- Requires a `TerminologyService` to be set in the evaluation context. Without one, returns an empty collection (unknown).
- Supports three input types:
  - **String** -- treated as a simple code value.
  - **Coding object** -- extracts `system`, `code`, `version`, and `display` fields.
  - **CodeableConcept object** -- extracts the `coding` array and `text` field.
- Returns `true` if the single input item is a member, and `false` if it is not. A CodeableConcept is a member when any of its codes is.
- An empty input, or one of more than one item (such as `code.coding.memberOf(...)` on a concept with two codings), returns empty without asking the service, under R4 rules too: R5 says "the input is empty or has more than one value, the return value is empty", and fhirpath.js and HL7's Java engine both answer so under R4.
- Returns empty collection if the check cannot be performed (no terminology service or errors).
- A value set that is no URI, `memberOf('not a uri')`, is an error under R4 rules for a single input item ("If the valueset cannot be resolved as a uri to a value set, an error is thrown") and empty under R5's.

---

## conformsTo

Checks if a resource conforms to a specified FHIR® profile (StructureDefinition). This function requires a `ProfileValidator` to be configured in the evaluation context.

**Signature:**
```
conformsTo(profileUrl : String) : Boolean
```

**Parameters:**

| Name | Type | Description |
|------|------|-------------|
| `profileUrl` | `String` | The canonical URL of the StructureDefinition to validate against |

**Return Type:** `Boolean`

**Examples:**

```go
// The profile validator is set on the evaluation context:
ctx := eval.NewContext(resource)
ctx.SetProfileValidator(myValidator)

compiled := fhirpath.MustCompile(
    "conformsTo('http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient')")
result, _ := compiled.EvaluateWithContext(ctx)
// true if the resource conforms to US Core Patient profile
```

**Edge Cases / Notes:**
- Requires a `ProfileValidator` to be set in the evaluation context. Without one, returns an empty collection (unknown).
- Operates on complex types (`ObjectValue`) with raw JSON data available.
- Returns empty collection if validation cannot be performed (no validator, no raw data, or errors).
- Profile validation may involve network calls depending on the implementation of your `ProfileValidator`.
- More than one item is an error under R4 rules and empty under R5's.

## subsumes

Returns `true` if the input Coding or CodeableConcept is equivalent to, or subsumes, the given one.

**Signature:**
```
subsumes(code : Coding | CodeableConcept) : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(observation, "Observation.category.subsumes(Observation.category.coding[0])")
// true: a Coding is equivalent to itself, by system and code
```

**Edge Cases / Notes:**
- Equivalence needs no terminology: two Codings are equivalent when their `system` and `code` are, and a CodeableConcept counts if any of its Codings does.
- Beyond equivalence, the function asks the `TerminologyService` set on the evaluation context, if it also implements `eval.Subsumer`. Without one the result is empty (unknown).
- Returns empty if the input or the argument is empty.
- More than one item on either side is an error under R4 rules and empty under R5's ("If either the input or the code parameter are not single value collections, the return value is empty").

---

## subsumedBy

Returns `true` if the input Coding or CodeableConcept is equivalent to, or subsumed by, the given one. It is `subsumes()` with the two sides swapped, and behaves the same way otherwise.

**Signature:**
```
subsumedBy(code : Coding | CodeableConcept) : Boolean
```

**Return Type:** `Boolean`

---

## htmlChecks

Returns `true` if a narrative's xhtml, or a string, meets FHIR's rules for HTML, and `false` if it does not.

**Signature:**
```
htmlChecks() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
compiled := fhirpath.MustCompile("text.`div`.htmlChecks()")
result, _ := compiled.EvaluateWithOptions(patient,
    fhirpath.WithModel(r4.FHIRPathModel()),
    fhirpath.WithHTMLChecker(myChecker))
// true if the narrative passes the checker
```

**Edge Cases / Notes:**
- `div` is a keyword, so the narrative is written `` text.`div` ``.
- The rules are FHIR's, so the caller brings them: the function asks the `HTMLChecker` given with `WithHTMLChecker`, or set with `eval.Context.SetHTMLChecker`.
- A string is checked too, a FHIR `string` or a literal, as fhirpath.js and HL7's Java engine both check it, though R5's text makes it empty. A `code` is not, as the text says; the two engines part there.
- What a value is comes from the type the model gives it; without a model, a code read from the resource is a string.
- Returns empty on any other kind of element, and on more than one item.
- On a single xhtml element or string with no checker given, the evaluation ends with an error.

---

## checkModifiers

Returns the input if no item has a modifier extension other than those named, and ends the evaluation with an error otherwise.

**Signature:**
```
checkModifiers(modifier : String) : Collection
```

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient,
    "Patient.checkModifiers('http://example.org/a,http://example.org/b').id")
// the id, if Patient's modifierExtension urls are among those two
```

**Edge Cases / Notes:**
- The modifiers are given as a comma-separated string, or, as R4 writes it, as a collection of strings.
- Only the items' own `modifierExtension` is checked, not their children's.
- An empty modifier is an error under R5 rules.

---

## elementDefinition

Returns the FHIR element definition of each item of the input.

**Signature:**
```
elementDefinition() : Collection
```

**Edge Cases / Notes:**
- Returns empty for an empty input.
- The engine does not hold element definitions, so a non-empty input ends the evaluation with an error.

---

## slice

Returns the items of the input that match a slice of a structure definition.

**Signature:**
```
slice(structure : String, name : String) : Collection
```

**Edge Cases / Notes:**
- Returns empty for an empty input.
- A structure that is no URI, or an empty argument, cannot be resolved: an error under R4 rules, empty under R5's.
- The engine does not hold structure definitions, so a structure that is a URI ends the evaluation with an error.

---

## hasValue

Returns `true` if the input is a single FHIR primitive that has a value, or a single System value of a primitive type.

**Signature:**
```
hasValue() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.active.hasValue()")
// true (active is a boolean primitive)

result, _ := fhirpath.Evaluate(patient, "Patient.name.first().hasValue()")
// false (a complex value is no FHIR primitive)

result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.hasValue()")
// true (birthDate is a date primitive)
```

**Edge Cases / Notes:**
- Returns `false` for empty input.
- Returns `false` for more than one item.
- A primitive written with extensions but no value has none: `false`.
- A System value of a primitive type has one, `'a'.hasValue()` is `true`, as fhirpath.js and HL7's Java engine answer; a quantity has none. `getValue()` of a System value stays empty.
- A complex value has none: `false`, as FHIR defines the function and fhirpath.js answers, where HL7's Java engine answers `true`. See [CONFORMANCE.md](https://github.com/gofhir/fhirpath/blob/main/CONFORMANCE.md#measured-and-not-followed).

---

## getValue

Returns the primitive values from the input collection. Complex types are filtered out.

**Signature:**
```
getValue() : Collection
```

**Return Type:** `Collection`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.active.getValue()")
// Returns the boolean value of active

result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.getValue()")
// Returns the date value

result, _ := fhirpath.Evaluate(patient, "Patient.name.getValue()")
// { } (empty - name entries are complex types, not primitives)
```

**Edge Cases / Notes:**
- Returns empty collection if the input is empty.
- Filters the input collection to only include primitive types: `Boolean`, `String`, `Integer`, `Decimal`, `Date`, `DateTime`, `Time`.
- Complex types (`ObjectValue`) are excluded from the result.
