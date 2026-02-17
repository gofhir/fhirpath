---
title: "Consultas Básicas"
linkTitle: "Consultas Básicas"
weight: 1
description: >
  Extraer datos demográficos del paciente, navegar estructuras anidadas, trabajar con arreglos y usar sintaxis FHIRPath contra recursos FHIR reales.
---

Esta página recorre las operaciones FHIRPath más comunes que realizará: leer campos simples, recorrer objetos anidados e indexar en arreglos. Cada ejemplo incluye un recurso FHIR JSON completo y el código Go necesario para evaluarlo.

## Extracción de Datos Demográficos del Paciente

Las expresiones FHIRPath más simples navegan desde la raíz del recurso hasta un campo hoja. La ruta siempre comienza con el nombre del tipo de recurso.

### Recurso Patient de Ejemplo

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

### Obtener el ID del Recurso

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

### Obtener el Apellido

```go
family, err := fhirpath.EvaluateToString(patient, "Patient.name.family")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Family:", family)
// Output: Family: Chalmers
```

Cuando la ruta atraviesa un arreglo (como `name`), FHIRPath automáticamente itera sobre cada elemento y recopila el campo `family` de cada uno. Si solo un nombre tiene un valor `family`, obtiene una colección de un solo elemento.

### Obtener la Fecha de Nacimiento

```go
birthDate, err := fhirpath.EvaluateToString(patient, "Patient.birthDate")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Birth date:", birthDate)
// Output: Birth date: 1974-12-25
```

### Obtener Múltiples Campos a la Vez

Puede evaluar varias expresiones contra el mismo recurso. Para mejor rendimiento en producción, use `EvaluateCached` para que cada expresión se compile solo una vez:

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

## Navegación de Estructuras Anidadas

Los recursos FHIR contienen objetos profundamente anidados. FHIRPath usa notación de punto para recorrerlos.

### Extracción de Campos de Dirección

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

### Extracción de Valores de Telecom

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

### Navegación de Anidamiento Multi-Nivel

Algunos recursos FHIR están profundamente anidados. Por ejemplo, una Observation con un componente:

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

## Trabajo con Arreglos

FHIRPath proporciona varias formas de trabajar con arreglos: indexación, filtrado con `where()` e iteración implícita.

### Indexación en Arreglos

Use notación de corchetes para acceder a un elemento específico por su índice basado en cero:

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

### Filtrado con where()

La función `where()` le permite seleccionar elementos que coinciden con un criterio booleano:

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

### Uso de first() y last()

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

## Uso de los Helpers de Conveniencia

La biblioteca proporciona varias funciones de conveniencia con tipo que le evitan inspeccionar manualmente la colección de resultados.

### Verificación de Existencia

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

### Conteo de Resultados

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

### Evaluación Booleana

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

## Ejemplo Completo Funcional

Aquí hay un programa autónomo que demuestra varias consultas básicas contra un recurso Patient realista:

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
