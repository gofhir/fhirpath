---
title: "Basic Queries"
linkTitle: "Basic Queries"
weight: 1
description: >
  Extract patient demographics, navigate nested structures, work with arrays, and use FHIRPath syntax against real FHIR® resources.
---

This page walks through the most common FHIRPath operations you will perform: reading simple fields, traversing nested objects, and indexing into arrays. Every example includes a complete FHIR® JSON resource and the Go code needed to evaluate it.

## Extracting Patient Demographics

The simplest FHIRPath expressions navigate from the resource root to a leaf field. The path always starts with the resource type name.

### Sample Patient Resource

```json
{
  "resourceType": "Patient",
  "id": "example-patient-1",
  "active": true,
  "name": [
    {
      "use": "official",
      "family": "Chalmers",
      "given": ["Peter", "James"]
    },
    {
      "use": "usual",
      "given": ["Jim"]
    }
  ],
  "gender": "male",
  "birthDate": "1974-12-25",
  "telecom": [
    {
      "system": "phone",
      "value": "(03) 5555 6473",
      "use": "work"
    },
    {
      "system": "email",
      "value": "peter.chalmers@example.com",
      "use": "home"
    }
  ],
  "address": [
    {
      "use": "home",
      "line": ["534 Erewhon St"],
      "city": "PleasantVille",
      "state": "VT",
      "postalCode": "3999"
    },
    {
      "use": "work",
      "line": ["100 Corporate Dr"],
      "city": "Metropolis",
      "state": "IL",
      "postalCode": "60007"
    }
  ]
}
```

### Getting the Resource ID

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
		"id": "example-patient-1",
		"active": true,
		"name": [{"use": "official", "family": "Chalmers", "given": ["Peter", "James"]}],
		"gender": "male",
		"birthDate": "1974-12-25"
	}`)

	// Extract the resource id
	id, err := fhirpath.EvaluateToString(patient, "Patient.id")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("ID:", id)
	// Output: ID: example-patient-1
}
```

### Getting the Family Name

```go
family, err := fhirpath.EvaluateToString(patient, "Patient.name.family")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Family:", family)
// Output: Family: Chalmers
```

When the path traverses an array (like `name`), FHIRPath automatically iterates over every element and collects the `family` field from each one. If only one name has a `family` value, you get a single-element collection.

### Getting the Birth Date

```go
birthDate, err := fhirpath.EvaluateToString(patient, "Patient.birthDate")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Birth date:", birthDate)
// Output: Birth date: 1974-12-25
```

### Getting Multiple Fields at Once

You can evaluate several expressions against the same resource. For better performance in production, use `EvaluateCached` so each expression is compiled only once:

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "example-patient-1",
    "active": true,
    "name": [{"use": "official", "family": "Chalmers", "given": ["Peter", "James"]}],
    "gender": "male",
    "birthDate": "1974-12-25"
}`)

expressions := map[string]string{
    "id":        "Patient.id",
    "family":    "Patient.name.family",
    "gender":    "Patient.gender",
    "birthDate": "Patient.birthDate",
}

for label, expr := range expressions {
    result, err := fhirpath.EvaluateCached(patient, expr)
    if err != nil {
        log.Printf("Error evaluating %s: %v", label, err)
        continue
    }
    fmt.Printf("%-10s: %s\n", label, result)
}
```

## Navigating Nested Structures

FHIR® resources contain deeply nested objects. FHIRPath uses dot notation to traverse them.

### Extracting Address Fields

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "patient-nested",
    "address": [
        {
            "use": "home",
            "line": ["534 Erewhon St"],
            "city": "PleasantVille",
            "state": "VT",
            "postalCode": "3999"
        },
        {
            "use": "work",
            "line": ["100 Corporate Dr"],
            "city": "Metropolis",
            "state": "IL",
            "postalCode": "60007"
        }
    ]
}`)

// Get all cities -- traverses all address entries
cities, err := fhirpath.EvaluateToStrings(patient, "Patient.address.city")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Cities:", cities)
// Output: Cities: [PleasantVille Metropolis]
```

### Extracting Telecom Values

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "patient-telecom",
    "telecom": [
        {"system": "phone", "value": "(03) 5555 6473", "use": "work"},
        {"system": "email", "value": "peter.chalmers@example.com", "use": "home"}
    ]
}`)

// Get all telecom values
values, err := fhirpath.EvaluateToStrings(patient, "Patient.telecom.value")
if err != nil {
    log.Fatal(err)
}
for _, v := range values {
    fmt.Println("Telecom:", v)
}
// Output:
// Telecom: (03) 5555 6473
// Telecom: peter.chalmers@example.com
```

### Navigating Multi-Level Nesting

Some FHIR® resources are deeply nested. For example, an Observation with a component:

```go
observation := []byte(`{
    "resourceType": "Observation",
    "id": "blood-pressure",
    "status": "final",
    "code": {
        "coding": [
            {
                "system": "http://loinc.org",
                "code": "85354-9",
                "display": "Blood pressure panel"
            }
        ]
    },
    "component": [
        {
            "code": {
                "coding": [
                    {
                        "system": "http://loinc.org",
                        "code": "8480-6",
                        "display": "Systolic blood pressure"
                    }
                ]
            },
            "valueQuantity": {
                "value": 120,
                "unit": "mmHg",
                "system": "http://unitsofmeasure.org",
                "code": "mm[Hg]"
            }
        },
        {
            "code": {
                "coding": [
                    {
                        "system": "http://loinc.org",
                        "code": "8462-4",
                        "display": "Diastolic blood pressure"
                    }
                ]
            },
            "valueQuantity": {
                "value": 80,
                "unit": "mmHg",
                "system": "http://unitsofmeasure.org",
                "code": "mm[Hg]"
            }
        }
    ]
}`)

// Get the display text of the top-level code
display, err := fhirpath.EvaluateToString(observation, "Observation.code.coding.display")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Code display:", display)
// Output: Code display: Blood pressure panel

// Get all component displays
componentDisplays, err := fhirpath.EvaluateToStrings(observation,
    "Observation.component.code.coding.display")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Components:", componentDisplays)
// Output: Components: [Systolic blood pressure Diastolic blood pressure]
```

## Working with Arrays

FHIRPath provides several ways to work with arrays: indexing, filtering with `where()`, and implicit iteration.

### Indexing into Arrays

Use bracket notation to access a specific element by its zero-based index:

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "patient-arrays",
    "name": [
        {"use": "official", "family": "Chalmers", "given": ["Peter", "James"]},
        {"use": "usual", "given": ["Jim"]}
    ]
}`)

// Get the first name entry
result, err := fhirpath.EvaluateToString(patient, "Patient.name[0].family")
if err != nil {
    log.Fatal(err)
}
fmt.Println("First family name:", result)
// Output: First family name: Chalmers

// Get the first given name of the first name entry
firstGiven, err := fhirpath.EvaluateToString(patient, "Patient.name[0].given[0]")
if err != nil {
    log.Fatal(err)
}
fmt.Println("First given name:", firstGiven)
// Output: First given name: Peter

// Get all given names across all name entries (implicit iteration)
allGiven, err := fhirpath.EvaluateToStrings(patient, "Patient.name.given")
if err != nil {
    log.Fatal(err)
}
fmt.Println("All given names:", allGiven)
// Output: All given names: [Peter James Jim]
```

### Filtering with where()

The `where()` function lets you select elements that match a Boolean criterion:

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "patient-where",
    "address": [
        {"use": "home", "city": "PleasantVille", "state": "VT"},
        {"use": "work", "city": "Metropolis", "state": "IL"},
        {"use": "temp", "city": "Somewhere", "state": "CA"}
    ]
}`)

// Get only the home address city
homeCity, err := fhirpath.EvaluateToString(patient,
    "Patient.address.where(use = 'home').city")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Home city:", homeCity)
// Output: Home city: PleasantVille

// Get the state of the work address
workState, err := fhirpath.EvaluateToString(patient,
    "Patient.address.where(use = 'work').state")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Work state:", workState)
// Output: Work state: IL
```

### Using first() and last()

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "patient-first-last",
    "name": [
        {"use": "official", "family": "Chalmers", "given": ["Peter"]},
        {"use": "maiden", "family": "Windsor", "given": ["Anne"]}
    ]
}`)

// Get the first name entry's family
first, err := fhirpath.EvaluateToString(patient, "Patient.name.first().family")
if err != nil {
    log.Fatal(err)
}
fmt.Println("First:", first)
// Output: First: Chalmers

// Get the last name entry's family
last, err := fhirpath.EvaluateToString(patient, "Patient.name.last().family")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Last:", last)
// Output: Last: Windsor
```

## Using the Convenience Helpers

The library provides several typed convenience functions that save you from manually inspecting the result collection.

### Checking Existence

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "patient-exists",
    "name": [{"family": "Doe"}],
    "birthDate": "1990-01-15"
}`)

// Check if the patient has a birthDate
hasBirthDate, err := fhirpath.Exists(patient, "Patient.birthDate")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Has birthDate:", hasBirthDate)
// Output: Has birthDate: true

// Check if the patient has a deceased indicator
hasDeceased, err := fhirpath.Exists(patient, "Patient.deceased")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Has deceased:", hasDeceased)
// Output: Has deceased: false
```

### Counting Results

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "patient-count",
    "name": [
        {"use": "official", "family": "Chalmers", "given": ["Peter", "James"]},
        {"use": "usual", "given": ["Jim"]}
    ],
    "telecom": [
        {"system": "phone", "value": "555-1234"},
        {"system": "email", "value": "peter@example.com"},
        {"system": "phone", "value": "555-5678"}
    ]
}`)

// Count the number of name entries
nameCount, err := fhirpath.Count(patient, "Patient.name")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Name entries:", nameCount)
// Output: Name entries: 2

// Count all given names across all entries
givenCount, err := fhirpath.Count(patient, "Patient.name.given")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Given names:", givenCount)
// Output: Given names: 3

// Count telecom entries
telecomCount, err := fhirpath.Count(patient, "Patient.telecom")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Telecom entries:", telecomCount)
// Output: Telecom entries: 3
```

### Boolean Evaluation

```go
patient := []byte(`{
    "resourceType": "Patient",
    "id": "patient-bool",
    "active": true,
    "name": [{"family": "Smith"}]
}`)

// Evaluate a boolean expression
isActive, err := fhirpath.EvaluateToBoolean(patient, "Patient.active")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Is active:", isActive)
// Output: Is active: true

// Evaluate an existence check as a boolean
hasName, err := fhirpath.EvaluateToBoolean(patient, "Patient.name.exists()")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Has name:", hasName)
// Output: Has name: true
```

## Complete Working Example

Here is a self-contained program that demonstrates several basic queries against a realistic Patient resource:

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
		"id": "pat-12345",
		"meta": {
			"versionId": "3",
			"lastUpdated": "2024-01-15T10:30:00Z"
		},
		"active": true,
		"name": [
			{
				"use": "official",
				"family": "Rodriguez",
				"given": ["Maria", "Elena"],
				"prefix": ["Mrs."]
			}
		],
		"gender": "female",
		"birthDate": "1985-07-20",
		"telecom": [
			{"system": "phone", "value": "+1-555-867-5309", "use": "mobile"},
			{"system": "email", "value": "maria.rodriguez@example.com", "use": "home"}
		],
		"address": [
			{
				"use": "home",
				"line": ["742 Evergreen Terrace"],
				"city": "Springfield",
				"state": "IL",
				"postalCode": "62704",
				"country": "US"
			}
		]
	}`)

	// Resource ID
	id, _ := fhirpath.EvaluateToString(patient, "Patient.id")
	fmt.Println("ID:        ", id)

	// Full name
	family, _ := fhirpath.EvaluateToString(patient, "Patient.name.where(use = 'official').family")
	given, _ := fhirpath.EvaluateToStrings(patient, "Patient.name.where(use = 'official').given")
	fmt.Printf("Name:       %s %s\n", given, family)

	// Demographics
	gender, _ := fhirpath.EvaluateToString(patient, "Patient.gender")
	dob, _ := fhirpath.EvaluateToString(patient, "Patient.birthDate")
	fmt.Println("Gender:    ", gender)
	fmt.Println("Birth date:", dob)

	// Contact information
	phone, _ := fhirpath.EvaluateToString(patient, "Patient.telecom.where(system = 'phone').value")
	email, _ := fhirpath.EvaluateToString(patient, "Patient.telecom.where(system = 'email').value")
	fmt.Println("Phone:     ", phone)
	fmt.Println("Email:     ", email)

	// Address
	city, _ := fhirpath.EvaluateToString(patient, "Patient.address.where(use = 'home').city")
	state, _ := fhirpath.EvaluateToString(patient, "Patient.address.where(use = 'home').state")
	fmt.Println("City:      ", city)
	fmt.Println("State:     ", state)

	// Counts
	nameCount, _ := fhirpath.Count(patient, "Patient.name")
	telecomCount, _ := fhirpath.Count(patient, "Patient.telecom")
	fmt.Println("Name entries:   ", nameCount)
	fmt.Println("Telecom entries:", telecomCount)

	// Existence checks
	hasEmail, _ := fhirpath.Exists(patient, "Patient.telecom.where(system = 'email')")
	hasDeceased, _ := fhirpath.Exists(patient, "Patient.deceased")
	fmt.Println("Has email:     ", hasEmail)
	fmt.Println("Has deceased:  ", hasDeceased)

	// Error handling -- invalid expression
	_, err := fhirpath.Evaluate(patient, "Patient.!!!invalid")
	if err != nil {
		log.Printf("Expected error: %v", err)
	}
}
```
