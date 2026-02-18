---
title: "Trabajo con Extensions"
linkTitle: "Trabajo con Extensions"
weight: 4
description: >
  Acceder, verificar y extraer valores de extensions FHIR® usando las funciones extension(), hasExtension() y getExtensionValue().
---

Las extensions FHIR® son el mecanismo principal para agregar elementos de datos a los recursos que no forman parte de la especificación base. Debido a que las extensions son tan frecuentes en implementaciones FHIR® del mundo real, la biblioteca FHIRPath de Go proporciona funciones dedicadas para trabajar con ellas de manera eficiente.

## ¿Qué Son las Extensions FHIR®?

Todo elemento FHIR® puede contener un arreglo `extension`. Cada extension se identifica por una URL y contiene un valor tipado (uno de los campos `value[x]`). Las extensions son la forma en que las guías de implementación como US Core, AU Base e IPS agregan datos específicos de país o caso de uso.

Una extension se ve así en JSON:

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

Los campos clave son:

- **url** -- un identificador único global para la definición de la extension
- **value[x]** -- el valor real, donde `[x]` se reemplaza por el nombre del tipo de dato (por ejemplo, `valueString`, `valueBoolean`, `valueCode`, `valueAddress`)

## Uso de extension() para Acceder a Extensions

La función `extension(url)` filtra el arreglo de extensions en el elemento actual para retornar solo las extensions cuya `url` coincida con el argumento.

### Extension de Cadena Simple

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

## Uso de hasExtension() para Verificar Presencia

La función `hasExtension(url)` retorna un Boolean que indica si el elemento tiene una extension con la URL proporcionada. Esto es útil para lógica condicional.

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

## Uso de getExtensionValue() para Extraer Valores

La función `getExtensionValue(url)` va un paso más allá: encuentra la extension por URL y retorna su `value[x]` directamente, para que no tenga que navegar dentro del objeto de extension usted mismo.

### Extracción de un Valor Simple

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

### Extracción y Navegación de Valores Complejos

Cuando el valor de la extension es un tipo complejo (como CodeableConcept o Address), puede encadenar navegación de ruta adicional:

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

## Ejemplo del Mundo Real: Extension US Core Race

La guía de implementación US Core define una extension compleja para la raza del paciente que usa extensions anidadas (sub-extensions). Aquí hay un ejemplo realista:

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

## Resumen de Patrones de Extensions

Aquí hay una referencia rápida de las tres funciones de extension y cuándo usar cada una:

| Función | Retorna | Usar Cuando |
|---------|---------|-------------|
| `extension(url)` | El/los objeto(s) de extension completo(s) | Necesita la extension completa, incluyendo sub-extensions anidadas |
| `hasExtension(url)` | Boolean `true` / `false` | Solo necesita saber si la extension está presente |
| `getExtensionValue(url)` | El contenido `value[x]` directamente | Quiere el valor y no necesita el contenedor de la extension |

### Consejo de Rendimiento

Cuando necesita verificar si existe una extension y luego leer su valor, es más eficiente llamar a `extension(url)` una vez e inspeccionar el resultado que llamar a `hasExtension` seguido de `getExtensionValue`:

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

Para evaluaciones repetidas, use `EvaluateCached` para que la expresión se compile solo una vez:

```go
result, _ := fhirpath.EvaluateCached(resource,
    "Patient.getExtensionValue('http://example.org/ext')")
```
