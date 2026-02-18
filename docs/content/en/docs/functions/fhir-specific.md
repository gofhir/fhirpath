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

Resolves a FHIR® reference to the referenced resource. This function requires a `ReferenceResolver` to be configured in the evaluation context.

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
- Requires a `ReferenceResolver` to be set in the evaluation context. Without one, returns an empty collection.
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

**Edge Cases / Notes:**
- Requires a `TerminologyService` to be set in the evaluation context. Without one, returns an empty collection (unknown).
- Supports three input types:
  - **String** -- treated as a simple code value.
  - **Coding object** -- extracts `system`, `code`, `version`, and `display` fields.
  - **CodeableConcept object** -- extracts the `coding` array and `text` field.
- Returns `true` if any element in the input collection is a member.
- Returns `false` if elements were checked but none are members.
- Returns empty collection if the check cannot be performed (no terminology service or errors).

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

**Edge Cases / Notes:**
- Requires a `ProfileValidator` to be set in the evaluation context. Without one, returns an empty collection (unknown).
- Operates on complex types (`ObjectValue`) with raw JSON data available.
- Returns empty collection if validation cannot be performed (no validator, no raw data, or errors).
- Profile validation may involve network calls depending on the implementation of your `ProfileValidator`.

---

## hasValue

Returns `true` if the input contains a primitive value (Boolean, String, Integer, Decimal, Date, DateTime, or Time).

**Signature:**
```
hasValue() : Boolean
```

**Return Type:** `Boolean`

**Examples:**

```go
result, _ := fhirpath.Evaluate(patient, "Patient.active.hasValue()")
// true (active is a boolean primitive)

result, _ := fhirpath.Evaluate(patient, "Patient.name.hasValue()")
// false (name is a complex type, not a primitive)

result, _ := fhirpath.Evaluate(patient, "Patient.birthDate.hasValue()")
// true (birthDate is a date primitive)
```

**Edge Cases / Notes:**
- Returns `false` for empty input.
- Returns `true` if **any** element in the collection has a primitive value.
- Complex types (objects) do not count as having a value, even if they contain primitive fields.

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
