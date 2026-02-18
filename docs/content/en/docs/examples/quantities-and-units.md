---
title: "Quantities and Units"
linkTitle: "Quantities and Units"
weight: 5
description: >
  Work with UCUM quantities, compare and convert units, perform quantity arithmetic, and extract values from Observations.
---

FHIR® uses the [Unified Code for Units of Measure (UCUM)](https://ucum.org) system to represent physical quantities. The FHIRPath Go library includes built-in support for quantity literals, UCUM unit normalization, comparisons, and arithmetic -- all of which are critical when working with clinical Observations.

## UCUM Unit System Basics

In FHIRPath, a **quantity** is a decimal number paired with a UCUM unit string, written as:

```
<number> '<unit>'
```

For example:

- `70 'kg'` -- seventy kilograms
- `98.6 '[degF]'` -- 98.6 degrees Fahrenheit
- `120 'mm[Hg]'` -- 120 millimeters of mercury
- `500 'mg'` -- 500 milligrams

The library normalizes compatible units internally so that comparisons like `1000 'mg' = 1 'g'` work correctly.

## Quantity Comparisons

### Comparing with Unit Normalization

FHIRPath quantity comparisons automatically normalize units when they belong to the same dimension:

```go
package main

import (
	"fmt"
	"log"

	"github.com/gofhir/fhirpath"
)

func main() {
	// We do not need a real resource for pure quantity expressions.
	// Use a minimal resource to satisfy the API.
	empty := []byte(`{"resourceType": "Basic"}`)

	// 1000 milligrams equals 1 gram
	eq, err := fhirpath.EvaluateToBoolean(empty, "1000 'mg' = 1 'g'")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("1000 mg = 1 g:", eq)
	// Output: 1000 mg = 1 g: true

	// 1 kilogram equals 1000 grams
	eq2, _ := fhirpath.EvaluateToBoolean(empty, "1 'kg' = 1000 'g'")
	fmt.Println("1 kg = 1000 g:", eq2)
	// Output: 1 kg = 1000 g: true

	// Comparison operators
	gt, _ := fhirpath.EvaluateToBoolean(empty, "2 'kg' > 1500 'g'")
	fmt.Println("2 kg > 1500 g:", gt)
	// Output: 2 kg > 1500 g: true

	lt, _ := fhirpath.EvaluateToBoolean(empty, "100 'cm' < 2 'm'")
	fmt.Println("100 cm < 2 m:", lt)
	// Output: 100 cm < 2 m: true

	// Less than or equal
	lte, _ := fhirpath.EvaluateToBoolean(empty, "1000 'mL' <= 1 'L'")
	fmt.Println("1000 mL <= 1 L:", lte)
	// Output: 1000 mL <= 1 L: true
}
```

### Incompatible Units

Comparing quantities with incompatible dimensions (e.g., mass vs. length) returns an empty collection (neither true nor false), following the FHIRPath specification:

```go
empty := []byte(`{"resourceType": "Basic"}`)

// Mass vs. length -- incompatible
result, err := fhirpath.Evaluate(empty, "10 'kg' = 10 'm'")
if err != nil {
    log.Fatal(err)
}
fmt.Println("kg = m result:", result)
// Output: kg = m result: []
// (empty collection -- comparison is undefined for incompatible units)
```

## Quantity Arithmetic

FHIRPath supports addition, subtraction, multiplication, and division on quantities.

```go
empty := []byte(`{"resourceType": "Basic"}`)

// Addition of compatible quantities
sum, _ := fhirpath.Evaluate(empty, "500 'mg' + 500 'mg'")
fmt.Println("500 mg + 500 mg:", sum)
// Output: 500 mg + 500 mg: [1000 'mg']

// Subtraction
diff, _ := fhirpath.Evaluate(empty, "1 'kg' - 200 'g'")
fmt.Println("1 kg - 200 g:", diff)
// Output: 1 kg - 200 g: [800 'g']

// Multiplication by a number
product, _ := fhirpath.Evaluate(empty, "250 'mg' * 4")
fmt.Println("250 mg * 4:", product)
// Output: 250 mg * 4: [1000 'mg']

// Division by a number
quotient, _ := fhirpath.Evaluate(empty, "1 'g' / 4")
fmt.Println("1 g / 4:", quotient)
// Output: 1 g / 4: [0.25 'g']
```

## Working with Observation Quantities

Most clinical Observations carry their result in a `valueQuantity` field. FHIRPath makes it easy to extract and compare these values.

### Extracting an Observation Value

```go
observation := []byte(`{
    "resourceType": "Observation",
    "id": "glucose-1",
    "status": "final",
    "code": {
        "coding": [{
            "system": "http://loinc.org",
            "code": "2339-0",
            "display": "Glucose [Mass/volume] in Blood"
        }]
    },
    "valueQuantity": {
        "value": 95,
        "unit": "mg/dL",
        "system": "http://unitsofmeasure.org",
        "code": "mg/dL"
    },
    "referenceRange": [{
        "low": {
            "value": 70,
            "unit": "mg/dL",
            "system": "http://unitsofmeasure.org",
            "code": "mg/dL"
        },
        "high": {
            "value": 100,
            "unit": "mg/dL",
            "system": "http://unitsofmeasure.org",
            "code": "mg/dL"
        }
    }]
}`)

// Get the value
result, err := fhirpath.Evaluate(observation, "Observation.valueQuantity.value")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Glucose value:", result)
// Output: Glucose value: [95]

// Get the unit
unit, _ := fhirpath.EvaluateToString(observation, "Observation.valueQuantity.unit")
fmt.Println("Unit:", unit)
// Output: Unit: mg/dL

// Check if the value exists
hasValue, _ := fhirpath.Exists(observation, "Observation.valueQuantity")
fmt.Println("Has value:", hasValue)
// Output: Has value: true
```

### Checking Reference Ranges

You can use quantity comparisons to check whether an observation value falls within its reference range:

```go
observation := []byte(`{
    "resourceType": "Observation",
    "id": "hemoglobin-1",
    "status": "final",
    "code": {
        "coding": [{
            "system": "http://loinc.org",
            "code": "718-7",
            "display": "Hemoglobin [Mass/volume] in Blood"
        }]
    },
    "valueQuantity": {
        "value": 14.2,
        "unit": "g/dL",
        "system": "http://unitsofmeasure.org",
        "code": "g/dL"
    },
    "referenceRange": [{
        "low":  {"value": 13.5, "unit": "g/dL", "system": "http://unitsofmeasure.org", "code": "g/dL"},
        "high": {"value": 17.5, "unit": "g/dL", "system": "http://unitsofmeasure.org", "code": "g/dL"}
    }]
}`)

// Check if the value is above the lower bound
aboveLow, _ := fhirpath.EvaluateToBoolean(observation,
    "Observation.valueQuantity.value >= Observation.referenceRange.low.value")
fmt.Println("Above lower bound:", aboveLow)
// Output: Above lower bound: true

// Check if the value is below the upper bound
belowHigh, _ := fhirpath.EvaluateToBoolean(observation,
    "Observation.valueQuantity.value <= Observation.referenceRange.high.value")
fmt.Println("Below upper bound:", belowHigh)
// Output: Below upper bound: true
```

### Blood Pressure (Multi-Component Observation)

Blood pressure is modeled as a single Observation with two components:

```go
package main

import (
	"fmt"
	"log"

	"github.com/gofhir/fhirpath"
)

func main() {
	bp := []byte(`{
		"resourceType": "Observation",
		"id": "blood-pressure-1",
		"status": "final",
		"category": [{
			"coding": [{
				"system": "http://terminology.hl7.org/CodeSystem/observation-category",
				"code": "vital-signs",
				"display": "Vital Signs"
			}]
		}],
		"code": {
			"coding": [{
				"system": "http://loinc.org",
				"code": "85354-9",
				"display": "Blood pressure panel with all children optional"
			}]
		},
		"component": [
			{
				"code": {
					"coding": [{
						"system": "http://loinc.org",
						"code": "8480-6",
						"display": "Systolic blood pressure"
					}]
				},
				"valueQuantity": {
					"value": 142,
					"unit": "mmHg",
					"system": "http://unitsofmeasure.org",
					"code": "mm[Hg]"
				}
			},
			{
				"code": {
					"coding": [{
						"system": "http://loinc.org",
						"code": "8462-4",
						"display": "Diastolic blood pressure"
					}]
				},
				"valueQuantity": {
					"value": 88,
					"unit": "mmHg",
					"system": "http://unitsofmeasure.org",
					"code": "mm[Hg]"
				}
			}
		]
	}`)

	// Extract systolic value
	systolic, err := fhirpath.Evaluate(bp,
		"Observation.component.where(code.coding.code = '8480-6').valueQuantity.value")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Systolic:", systolic)
	// Output: Systolic: [142]

	// Extract diastolic value
	diastolic, _ := fhirpath.Evaluate(bp,
		"Observation.component.where(code.coding.code = '8462-4').valueQuantity.value")
	fmt.Println("Diastolic:", diastolic)
	// Output: Diastolic: [88]

	// Get both units
	units, _ := fhirpath.EvaluateToStrings(bp,
		"Observation.component.valueQuantity.unit")
	fmt.Println("Units:", units)
	// Output: Units: [mmHg mmHg]

	// Count components
	componentCount, _ := fhirpath.Count(bp, "Observation.component")
	fmt.Println("Component count:", componentCount)
	// Output: Component count: 2

	// Check if systolic is high (>= 140 mmHg indicates hypertension stage 2)
	systolicHigh, _ := fhirpath.EvaluateToBoolean(bp,
		"Observation.component.where(code.coding.code = '8480-6').valueQuantity.value >= 140")
	fmt.Println("Systolic >= 140:", systolicHigh)
	// Output: Systolic >= 140: true
}
```

## Unit Normalization Examples

The library normalizes units from common UCUM prefixes so that cross-unit comparisons work transparently:

| Left | Operator | Right | Result | Explanation |
|------|----------|-------|--------|-------------|
| `1000 'mg'` | `=` | `1 'g'` | `true` | milligrams to grams |
| `1 'kg'` | `>` | `999 'g'` | `true` | kilograms to grams |
| `100 'cm'` | `=` | `1 'm'` | `true` | centimeters to meters |
| `1000 'mL'` | `=` | `1 'L'` | `true` | milliliters to liters |
| `1 'min'` | `=` | `60 's'` | `true` | minutes to seconds |
| `1 'h'` | `=` | `60 'min'` | `true` | hours to minutes |

### Practical Unit Normalization

```go
empty := []byte(`{"resourceType": "Basic"}`)

tests := []struct {
    expr     string
    expected bool
}{
    {"1000 'mg' = 1 'g'", true},
    {"1 'kg' = 1000 'g'", true},
    {"100 'cm' = 1 'm'", true},
    {"1000 'mL' = 1 'L'", true},
    {"1 'min' = 60 's'", true},
    {"1 'h' = 3600 's'", true},
    {"2.54 'cm' = 1 '[in_i]'", true},
}

for _, tt := range tests {
    result, err := fhirpath.EvaluateToBoolean(empty, tt.expr)
    if err != nil {
        fmt.Printf("%-30s ERROR: %v\n", tt.expr, err)
        continue
    }
    status := "PASS"
    if result != tt.expected {
        status = "FAIL"
    }
    fmt.Printf("%-30s %s (got %v)\n", tt.expr, status, result)
}
```

## Working with Medication Dosages

Quantities appear frequently in medication dosage instructions:

```go
medRequest := []byte(`{
    "resourceType": "MedicationRequest",
    "id": "med-dose-1",
    "status": "active",
    "intent": "order",
    "medicationCodeableConcept": {
        "coding": [{
            "system": "http://www.nlm.nih.gov/research/umls/rxnorm",
            "code": "197696",
            "display": "Lisinopril 10 MG Oral Tablet"
        }]
    },
    "dosageInstruction": [{
        "sequence": 1,
        "text": "Take 10mg once daily",
        "timing": {
            "repeat": {
                "frequency": 1,
                "period": 1,
                "periodUnit": "d"
            }
        },
        "doseAndRate": [{
            "doseQuantity": {
                "value": 10,
                "unit": "mg",
                "system": "http://unitsofmeasure.org",
                "code": "mg"
            }
        }]
    }]
}`)

// Extract the dose value and unit
doseValue, _ := fhirpath.Evaluate(medRequest,
    "MedicationRequest.dosageInstruction.doseAndRate.doseQuantity.value")
fmt.Println("Dose value:", doseValue)
// Output: Dose value: [10]

doseUnit, _ := fhirpath.EvaluateToString(medRequest,
    "MedicationRequest.dosageInstruction.doseAndRate.doseQuantity.unit")
fmt.Println("Dose unit:", doseUnit)
// Output: Dose unit: mg

// Check if there are dosage instructions
hasDosage, _ := fhirpath.Exists(medRequest, "MedicationRequest.dosageInstruction")
fmt.Println("Has dosage:", hasDosage)
// Output: Has dosage: true

// Get the medication display name
medName, _ := fhirpath.EvaluateToString(medRequest,
    "MedicationRequest.medicationCodeableConcept.coding.display")
fmt.Println("Medication:", medName)
// Output: Medication: Lisinopril 10 MG Oral Tablet
```
