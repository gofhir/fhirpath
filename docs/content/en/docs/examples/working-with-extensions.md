---
title: "Working with Extensions"
linkTitle: "Working with Extensions"
weight: 4
description: >
  Access, check, and extract values from FHIR® extensions using the extension(), hasExtension(), and getExtensionValue() functions.
---

FHIR® extensions are the primary mechanism for adding data elements to resources that are not part of the base specification. Because extensions are so pervasive in real-world FHIR® implementations, the FHIRPath Go library provides dedicated functions for working with them efficiently.

## What Are FHIR® Extensions?

Every FHIR® element can carry an `extension` array. Each extension is identified by a URL and holds a typed value (one of the `value[x]` fields). Extensions are how implementation guides like US Core, AU Base, and IPS add country- or use-case-specific data.

An extension looks like this in JSON:

```json
{
  "url": "http://hl7.org/fhir/StructureDefinition/patient-birthPlace",
  "valueAddress": {
    "city": "Springfield",
    "state": "IL",
    "country": "US"
  }
}
```

The key fields are:

- **url** -- a globally unique identifier for the extension definition
- **value[x]** -- the actual value, where `[x]` is replaced by the datatype name (e.g., `valueString`, `valueBoolean`, `valueCode`, `valueAddress`)

## Using extension() to Access Extensions

The `extension(url)` function filters the extension array on the current element to return only extensions whose `url` matches the argument.

### Simple String Extension

```go
package main

import (
	"fmt"
	"log"

	"github.com/gofhir/fhirpath"
)

func main() {
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "pat-ext-1",
		"name": [{"family": "Smith", "given": ["John"]}],
		"birthDate": "1970-03-15",
		"_birthDate": {
			"extension": [
				{
					"url": "http://hl7.org/fhir/StructureDefinition/patient-birthTime",
					"valueDateTime": "1970-03-15T14:35:00-05:00"
				}
			]
		},
		"extension": [
			{
				"url": "http://hl7.org/fhir/StructureDefinition/patient-birthPlace",
				"valueAddress": {
					"city": "Springfield",
					"state": "IL",
					"country": "US"
				}
			},
			{
				"url": "http://hl7.org/fhir/StructureDefinition/patient-mothersMaidenName",
				"valueString": "Johnson"
			}
		]
	}`)

	// Get the mother's maiden name extension
	result, err := fhirpath.Evaluate(patient,
		"Patient.extension('http://hl7.org/fhir/StructureDefinition/patient-mothersMaidenName')")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Maiden name extension found:", len(result) > 0)
	// Output: Maiden name extension found: true
}
```

## Using hasExtension() to Check Presence

The `hasExtension(url)` function returns a Boolean indicating whether the element has an extension with the given URL. This is useful for conditional logic.

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "pat-has-ext",
    "name": [{"family": "Doe"}],
    "extension": [
        {
            "url": "http://hl7.org/fhir/StructureDefinition/patient-mothersMaidenName",
            "valueString": "Williams"
        },
        {
            "url": "http://hl7.org/fhir/us/core/StructureDefinition/us-core-race",
            "extension": [
                {
                    "url": "ombCategory",
                    "valueCoding": {
                        "system": "urn:oid:2.16.840.1.113883.6.238",
                        "code": "2106-3",
                        "display": "White"
                    }
                },
                {
                    "url": "text",
                    "valueString": "White"
                }
            ]
        }
    ]
}`)

// Check if the patient has the mother's maiden name extension
hasMaiden, err := fhirpath.EvaluateToBoolean(patient,
    "Patient.hasExtension('http://hl7.org/fhir/StructureDefinition/patient-mothersMaidenName')")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Has maiden name:", hasMaiden)
// Output: Has maiden name: true

// Check if the patient has the US Core race extension
hasRace, err := fhirpath.EvaluateToBoolean(patient,
    "Patient.hasExtension('http://hl7.org/fhir/us/core/StructureDefinition/us-core-race')")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Has race:", hasRace)
// Output: Has race: true

// Check for a non-existent extension
hasReligion, err := fhirpath.EvaluateToBoolean(patient,
    "Patient.hasExtension('http://hl7.org/fhir/StructureDefinition/patient-religion')")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Has religion:", hasReligion)
// Output: Has religion: false
```

## Using getExtensionValue() to Extract Values

The `getExtensionValue(url)` function goes one step further: it finds the extension by URL and returns its `value[x]` directly, so you do not have to navigate into the extension object yourself.

### Extracting a Simple Value

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "pat-get-value",
    "name": [{"family": "Garcia"}],
    "extension": [
        {
            "url": "http://hl7.org/fhir/StructureDefinition/patient-mothersMaidenName",
            "valueString": "Lopez"
        },
        {
            "url": "http://hl7.org/fhir/StructureDefinition/patient-genderIdentity",
            "valueCodeableConcept": {
                "coding": [{
                    "system": "http://hl7.org/fhir/gender-identity",
                    "code": "female",
                    "display": "Female"
                }]
            }
        }
    ]
}`)

// Get the maiden name value directly
maidenName, err := fhirpath.EvaluateToString(patient,
    "Patient.getExtensionValue('http://hl7.org/fhir/StructureDefinition/patient-mothersMaidenName')")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Mother's maiden name:", maidenName)
// Output: Mother's maiden name: Lopez
```

### Extracting and Navigating Complex Values

When the extension value is a complex type (like CodeableConcept or Address), you can chain further path navigation:

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "pat-complex-ext",
    "name": [{"family": "Kim"}],
    "extension": [
        {
            "url": "http://hl7.org/fhir/StructureDefinition/patient-birthPlace",
            "valueAddress": {
                "city": "Seoul",
                "country": "KR"
            }
        }
    ]
}`)

// Get the birth place extension and navigate into the address
result, err := fhirpath.Evaluate(patient,
    "Patient.extension('http://hl7.org/fhir/StructureDefinition/patient-birthPlace')")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Birth place extension count:", len(result))
// Output: Birth place extension count: 1
```

## Real-World Example: US Core Race Extension

The US Core implementation guide defines a complex extension for patient race that uses nested extensions (sub-extensions). Here is a realistic example:

```go
package main

import (
	"fmt"
	"log"

	"github.com/gofhir/fhirpath"
)

func main() {
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "us-core-patient",
		"meta": {
			"profile": [
				"http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient"
			]
		},
		"name": [
			{"use": "official", "family": "Washington", "given": ["George"]}
		],
		"gender": "male",
		"birthDate": "1990-06-15",
		"extension": [
			{
				"url": "http://hl7.org/fhir/us/core/StructureDefinition/us-core-race",
				"extension": [
					{
						"url": "ombCategory",
						"valueCoding": {
							"system": "urn:oid:2.16.840.1.113883.6.238",
							"code": "2054-5",
							"display": "Black or African American"
						}
					},
					{
						"url": "detailed",
						"valueCoding": {
							"system": "urn:oid:2.16.840.1.113883.6.238",
							"code": "2058-6",
							"display": "African American"
						}
					},
					{
						"url": "text",
						"valueString": "Black or African American"
					}
				]
			},
			{
				"url": "http://hl7.org/fhir/us/core/StructureDefinition/us-core-ethnicity",
				"extension": [
					{
						"url": "ombCategory",
						"valueCoding": {
							"system": "urn:oid:2.16.840.1.113883.6.238",
							"code": "2186-5",
							"display": "Not Hispanic or Latino"
						}
					},
					{
						"url": "text",
						"valueString": "Not Hispanic or Latino"
					}
				]
			},
			{
				"url": "http://hl7.org/fhir/us/core/StructureDefinition/us-core-birthsex",
				"valueCode": "M"
			}
		]
	}`)

	raceURL := "http://hl7.org/fhir/us/core/StructureDefinition/us-core-race"
	ethnicityURL := "http://hl7.org/fhir/us/core/StructureDefinition/us-core-ethnicity"
	birthSexURL := "http://hl7.org/fhir/us/core/StructureDefinition/us-core-birthsex"

	// Check which US Core extensions are present
	hasRace, _ := fhirpath.EvaluateToBoolean(patient,
		fmt.Sprintf("Patient.hasExtension('%s')", raceURL))
	hasEthnicity, _ := fhirpath.EvaluateToBoolean(patient,
		fmt.Sprintf("Patient.hasExtension('%s')", ethnicityURL))
	hasBirthSex, _ := fhirpath.EvaluateToBoolean(patient,
		fmt.Sprintf("Patient.hasExtension('%s')", birthSexURL))

	fmt.Println("Has race extension:     ", hasRace)
	fmt.Println("Has ethnicity extension:", hasEthnicity)
	fmt.Println("Has birth sex extension:", hasBirthSex)
	// Output:
	// Has race extension:      true
	// Has ethnicity extension: true
	// Has birth sex extension: true

	// Get the race extension and access its sub-extensions
	raceExt, err := fhirpath.Evaluate(patient,
		fmt.Sprintf("Patient.extension('%s')", raceURL))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Race extension found: %v (items: %d)\n", len(raceExt) > 0, len(raceExt))
	// Output: Race extension found: true (items: 1)

	// Get the birth sex value directly
	birthSexValue, _ := fhirpath.Evaluate(patient,
		fmt.Sprintf("Patient.getExtensionValue('%s')", birthSexURL))
	if len(birthSexValue) > 0 {
		fmt.Println("Birth sex:", birthSexValue[0])
	}
	// Output: Birth sex: M
}
```

## Extension Patterns Summary

Here is a quick reference of the three extension functions and when to use each one:

| Function | Returns | Use When |
|----------|---------|----------|
| `extension(url)` | The full extension object(s) | You need the entire extension, including nested sub-extensions |
| `hasExtension(url)` | Boolean `true` / `false` | You only need to know if the extension is present |
| `getExtensionValue(url)` | The `value[x]` content directly | You want the value and do not need the extension wrapper |

### Performance Tip

When you need to check for an extension and then read its value, it is more efficient to call `extension(url)` once and inspect the result than to call `hasExtension` followed by `getExtensionValue`:

```go
// Less efficient: two evaluations
hasExt, _ := fhirpath.EvaluateToBoolean(resource,
    "Patient.hasExtension('http://example.org/ext')")
if hasExt {
    value, _ := fhirpath.Evaluate(resource,
        "Patient.getExtensionValue('http://example.org/ext')")
    // use value
}

// More efficient: one evaluation, check the result
result, _ := fhirpath.Evaluate(resource,
    "Patient.getExtensionValue('http://example.org/ext')")
if len(result) > 0 {
    // use result[0]
}
```

For repeated evaluations, use `EvaluateCached` so the expression is compiled only once:

```go
result, _ := fhirpath.EvaluateCached(resource,
    "Patient.getExtensionValue('http://example.org/ext')")
```
