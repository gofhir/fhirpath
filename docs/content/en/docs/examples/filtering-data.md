---
title: "Filtering Data"
linkTitle: "Filtering Data"
weight: 2
description: >
  Use where(), select(), exists(), count(), and empty() to filter, project, and inspect FHIR® collections.
---

FHIRPath provides a rich set of functions for narrowing collections down to the elements you need. This page covers the most important filtering and inspection functions with complete examples.

## where() -- Filter by Criteria

The `where()` function evaluates a Boolean expression against each element in the input collection and returns only the elements where the expression is `true`.

### Filtering Telecom by System

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
		"id": "pat-filter-1",
		"telecom": [
			{"system": "phone", "value": "+1-555-0100", "use": "home"},
			{"system": "phone", "value": "+1-555-0101", "use": "work"},
			{"system": "email", "value": "jane.doe@example.com", "use": "home"},
			{"system": "fax",   "value": "+1-555-0199", "use": "work"}
		]
	}`)

	// Get only phone numbers
	phones, err := fhirpath.EvaluateToStrings(patient,
		"Patient.telecom.where(system = 'phone').value")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Phones:", phones)
	// Output: Phones: [+1-555-0100 +1-555-0101]

	// Get the home email address
	homeEmail, err := fhirpath.EvaluateToString(patient,
		"Patient.telecom.where(system = 'email' and use = 'home').value")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Home email:", homeEmail)
	// Output: Home email: jane.doe@example.com
}
```

### Filtering Observations by Status

```go
bundle := []byte(`{
    "resourceType": "Bundle",
    "type": "searchset",
    "entry": [
        {
            "resource": {
                "resourceType": "Observation",
                "id": "obs-1",
                "status": "final",
                "code": {"coding": [{"system": "http://loinc.org", "code": "2339-0", "display": "Glucose"}]},
                "valueQuantity": {"value": 95, "unit": "mg/dL"}
            }
        },
        {
            "resource": {
                "resourceType": "Observation",
                "id": "obs-2",
                "status": "preliminary",
                "code": {"coding": [{"system": "http://loinc.org", "code": "2339-0", "display": "Glucose"}]},
                "valueQuantity": {"value": 110, "unit": "mg/dL"}
            }
        },
        {
            "resource": {
                "resourceType": "Observation",
                "id": "obs-3",
                "status": "final",
                "code": {"coding": [{"system": "http://loinc.org", "code": "6299-2", "display": "Urea nitrogen"}]},
                "valueQuantity": {"value": 18, "unit": "mg/dL"}
            }
        }
    ]
}`)

// Get only final observations
finalIDs, err := fhirpath.EvaluateToStrings(bundle,
    "Bundle.entry.resource.where(status = 'final').id")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Final observation IDs:", finalIDs)
// Output: Final observation IDs: [obs-1 obs-3]
```

### Filtering by Nested Fields

You can use `where()` with paths that drill into child objects:

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "pat-filter-nested",
    "name": [
        {"use": "official", "family": "Doe",    "given": ["Jane"]},
        {"use": "maiden",   "family": "Smith",  "given": ["Jane"]},
        {"use": "nickname", "given": ["JD"]}
    ]
}`)

// Get only names that have a family field
namesWithFamily, err := fhirpath.EvaluateToStrings(patient,
    "Patient.name.where(family.exists()).family")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Names with family:", namesWithFamily)
// Output: Names with family: [Doe Smith]
```

## select() -- Project Fields

The `select()` function evaluates an expression against each element and returns the collected results. Unlike `where()` which filters elements, `select()` transforms them.

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "pat-select",
    "name": [
        {"use": "official", "family": "Martinez", "given": ["Carlos", "Alberto"]},
        {"use": "usual", "family": "Martinez", "given": ["Charlie"]}
    ]
}`)

// Project just the given names from all name entries
givenNames, err := fhirpath.EvaluateToStrings(patient,
    "Patient.name.select(given)")
if err != nil {
    log.Fatal(err)
}
fmt.Println("All given names:", givenNames)
// Output: All given names: [Carlos Alberto Charlie]

// Project the family name from each entry
families, err := fhirpath.EvaluateToStrings(patient,
    "Patient.name.select(family)")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Families:", families)
// Output: Families: [Martinez Martinez]
```

## exists() -- Check for Presence

The `exists()` function returns `true` if the collection contains at least one element. With a criteria argument, it returns `true` if any element matches.

### Basic Existence Checks

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "pat-exists",
    "active": true,
    "name": [{"family": "Thompson", "given": ["Sarah"]}],
    "telecom": [
        {"system": "phone", "value": "555-1234"}
    ]
}`)

// Does the patient have a name?
hasName, err := fhirpath.EvaluateToBoolean(patient, "Patient.name.exists()")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Has name:", hasName)
// Output: Has name: true

// Does the patient have a deceased indicator?
hasDeceased, err := fhirpath.EvaluateToBoolean(patient, "Patient.deceased.exists()")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Has deceased:", hasDeceased)
// Output: Has deceased: false

// Using the Exists convenience function
hasPhone, err := fhirpath.Exists(patient, "Patient.telecom.where(system = 'phone')")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Has phone:", hasPhone)
// Output: Has phone: true
```

### Existence with Criteria

When you pass a criteria to `exists()`, it checks whether any element in the collection satisfies that condition:

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "pat-exists-criteria",
    "name": [
        {"use": "official", "family": "Lee", "given": ["David"]},
        {"use": "nickname", "given": ["Dave"]}
    ],
    "telecom": [
        {"system": "phone", "value": "555-0001", "use": "home"},
        {"system": "email", "value": "dave@example.com", "use": "work"}
    ]
}`)

// Does any name have "use = official"?
hasOfficial, err := fhirpath.EvaluateToBoolean(patient,
    "Patient.name.exists(use = 'official')")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Has official name:", hasOfficial)
// Output: Has official name: true

// Is there a work email?
hasWorkEmail, err := fhirpath.EvaluateToBoolean(patient,
    "Patient.telecom.exists(system = 'email' and use = 'work')")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Has work email:", hasWorkEmail)
// Output: Has work email: true
```

## count() -- Count Elements

The `count()` function (and its Go convenience wrapper) returns the number of elements in a collection.

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "pat-count",
    "identifier": [
        {"system": "http://hospital.example.org/mrn", "value": "MRN-001"},
        {"system": "http://hl7.org/fhir/sid/us-ssn", "value": "999-99-1234"},
        {"system": "http://hospital.example.org/mrn", "value": "MRN-002"}
    ],
    "name": [
        {"use": "official", "family": "Brown", "given": ["Alice", "Marie"]},
        {"use": "maiden", "family": "Green"}
    ],
    "address": [
        {"use": "home", "city": "Portland"},
        {"use": "work", "city": "Seattle"}
    ]
}`)

// Count identifiers
idCount, err := fhirpath.Count(patient, "Patient.identifier")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Identifiers:", idCount)
// Output: Identifiers: 3

// Count names using the FHIRPath function
result, err := fhirpath.Evaluate(patient, "Patient.name.count()")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Name count:", result)
// Output: Name count: [2]

// Count all given names across all name entries
givenCount, err := fhirpath.Count(patient, "Patient.name.given")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Total given names:", givenCount)
// Output: Total given names: 3
```

## empty() -- Check for Emptiness

The `empty()` function returns `true` when the collection has zero elements. It is the logical inverse of `exists()`.

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "pat-empty",
    "name": [{"family": "Wilson"}],
    "telecom": []
}`)

// Is the name collection empty?
nameEmpty, err := fhirpath.EvaluateToBoolean(patient, "Patient.name.empty()")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Name is empty:", nameEmpty)
// Output: Name is empty: false

// Is the deceased field empty (not present)?
deceasedEmpty, err := fhirpath.EvaluateToBoolean(patient, "Patient.deceased.empty()")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Deceased is empty:", deceasedEmpty)
// Output: Deceased is empty: true
```

## Combining Filters

You can chain filtering functions together to build powerful queries.

### Chaining where() Calls

```go
encounter := []byte(`{
    "resourceType": "Encounter",
    "id": "enc-combined",
    "status": "finished",
    "class": {
        "system": "http://terminology.hl7.org/CodeSystem/v3-ActCode",
        "code": "IMP"
    },
    "participant": [
        {
            "type": [{"coding": [{"system": "http://terminology.hl7.org/CodeSystem/v3-ParticipationType", "code": "ATND"}]}],
            "individual": {"reference": "Practitioner/pract-1", "display": "Dr. Smith"}
        },
        {
            "type": [{"coding": [{"system": "http://terminology.hl7.org/CodeSystem/v3-ParticipationType", "code": "CON"}]}],
            "individual": {"reference": "Practitioner/pract-2", "display": "Dr. Jones"}
        },
        {
            "type": [{"coding": [{"system": "http://terminology.hl7.org/CodeSystem/v3-ParticipationType", "code": "ATND"}]}],
            "individual": {"reference": "Practitioner/pract-3", "display": "Dr. Williams"}
        }
    ]
}`)

// Get attending practitioners only
attendingNames, err := fhirpath.EvaluateToStrings(encounter,
    "Encounter.participant.where(type.coding.code = 'ATND').individual.display")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Attending:", attendingNames)
// Output: Attending: [Dr. Smith Dr. Williams]

// Count attending practitioners
attendingCount, err := fhirpath.Count(encounter,
    "Encounter.participant.where(type.coding.code = 'ATND')")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Attending count:", attendingCount)
// Output: Attending count: 2
```

### Combining exists() and where()

```go
medicationRequest := []byte(`{
    "resourceType": "MedicationRequest",
    "id": "medreq-1",
    "status": "active",
    "intent": "order",
    "medicationCodeableConcept": {
        "coding": [
            {
                "system": "http://www.nlm.nih.gov/research/umls/rxnorm",
                "code": "1049502",
                "display": "Acetaminophen 325 MG Oral Tablet"
            }
        ]
    },
    "dosageInstruction": [
        {
            "timing": {
                "repeat": {
                    "frequency": 1,
                    "period": 6,
                    "periodUnit": "h"
                }
            },
            "route": {
                "coding": [{"system": "http://snomed.info/sct", "code": "26643006", "display": "Oral route"}]
            },
            "doseAndRate": [
                {
                    "doseQuantity": {
                        "value": 2,
                        "unit": "tablets",
                        "system": "http://terminology.hl7.org/CodeSystem/v3-orderableDrugForm",
                        "code": "TAB"
                    }
                }
            ]
        }
    ]
}`)

// Check if the medication is active
isActive, err := fhirpath.EvaluateToBoolean(medicationRequest,
    "MedicationRequest.status = 'active'")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Is active:", isActive)
// Output: Is active: true

// Check if dosage instructions exist
hasDosage, err := fhirpath.Exists(medicationRequest,
    "MedicationRequest.dosageInstruction")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Has dosage:", hasDosage)
// Output: Has dosage: true

// Get the medication display
medName, err := fhirpath.EvaluateToString(medicationRequest,
    "MedicationRequest.medicationCodeableConcept.coding.display")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Medication:", medName)
// Output: Medication: Acetaminophen 325 MG Oral Tablet
```

## Complete Working Example

A self-contained program showing all the filtering functions together:

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
		"id": "pat-comprehensive",
		"active": true,
		"name": [
			{"use": "official", "family": "Garcia", "given": ["Maria", "Isabel"]},
			{"use": "nickname", "given": ["Isa"]}
		],
		"telecom": [
			{"system": "phone", "value": "+1-555-0100", "use": "mobile"},
			{"system": "phone", "value": "+1-555-0101", "use": "work"},
			{"system": "email", "value": "maria.garcia@example.com", "use": "home"},
			{"system": "email", "value": "m.garcia@hospital.org", "use": "work"}
		],
		"address": [
			{"use": "home", "city": "Austin", "state": "TX"},
			{"use": "work", "city": "Houston", "state": "TX"}
		],
		"identifier": [
			{"system": "http://hospital.example.org/mrn", "value": "MRN-54321"},
			{"system": "http://hl7.org/fhir/sid/us-ssn", "value": "123-45-6789"}
		]
	}`)

	// where() -- filter telecom by system
	phones, _ := fhirpath.EvaluateToStrings(patient,
		"Patient.telecom.where(system = 'phone').value")
	fmt.Println("Phone numbers:", phones)

	// where() with compound condition
	workEmail, _ := fhirpath.EvaluateToString(patient,
		"Patient.telecom.where(system = 'email' and use = 'work').value")
	fmt.Println("Work email:   ", workEmail)

	// select() -- project given names
	allGiven, _ := fhirpath.EvaluateToStrings(patient,
		"Patient.name.select(given)")
	fmt.Println("All given:    ", allGiven)

	// exists() -- presence check
	hasNickname, _ := fhirpath.EvaluateToBoolean(patient,
		"Patient.name.exists(use = 'nickname')")
	fmt.Println("Has nickname: ", hasNickname)

	// count() -- element count
	telecomCount, _ := fhirpath.Count(patient, "Patient.telecom")
	fmt.Println("Telecom count:", telecomCount)

	// empty() -- absence check
	noPhoto, _ := fhirpath.EvaluateToBoolean(patient, "Patient.photo.empty()")
	fmt.Println("No photo:     ", noPhoto)

	// Combined: count phones only
	phoneCount, _ := fhirpath.Count(patient,
		"Patient.telecom.where(system = 'phone')")
	fmt.Println("Phone count:  ", phoneCount)

	// Combined: official family + existence guard
	officialFamily, err := fhirpath.EvaluateToString(patient,
		"Patient.name.where(use = 'official').family")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Official name:", officialFamily)
}
```
