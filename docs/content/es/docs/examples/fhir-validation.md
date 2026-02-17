---
title: "Validación FHIR"
linkTitle: "Validación FHIR"
weight: 3
description: >
  Usar expresiones FHIRPath para evaluar restricciones e invariantes FHIR, realizar validación Boolean y construir una función de validación simple.
---

Muchas restricciones FHIR (invariantes) están formalmente definidas como expresiones FHIRPath que deben evaluarse a `true` para que un recurso sea válido. La biblioteca FHIRPath de Go es ideal para evaluar estas reglas porque `EvaluateToBoolean` le proporciona un resultado `bool` directo de Go.

## Comprensión de los Invariantes FHIR

La especificación FHIR define invariantes sobre recursos y tipos de datos. Cada invariante tiene:

- Una **clave** (por ejemplo, `pat-1`)
- Una **severidad** (`error` o `warning`)
- Una **descripción legible por humanos**
- Una **expresión FHIRPath** que debe evaluarse a `true`

Por ejemplo, el recurso Patient define:

| Clave | Severidad | Expresión |
|-------|-----------|-----------|
| pat-1 | error | `name.exists() or identifier.exists()` |

Esto significa que todo Patient debe tener al menos un nombre o un identificador.

## Verificación Básica de Restricciones

### El Paciente Debe Tener un Nombre o Identificador

```go
package main

import (
	"fmt"
	"log"

	"github.com/gofhir/fhirpath"
)

func main() {
	// Valid patient -- has a name
	validPatient := []byte(`{
		"resourceType": "Patient",
		"id": "valid-1",
		"name": [{"family": "Smith", "given": ["John"]}]
	}`)

	// Invalid patient -- no name and no identifier
	invalidPatient := []byte(`{
		"resourceType": "Patient",
		"id": "invalid-1",
		"gender": "male",
		"birthDate": "1990-01-01"
	}`)

	expr := "Patient.name.exists() or Patient.identifier.exists()"

	valid, err := fhirpath.EvaluateToBoolean(validPatient, expr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Valid patient passes pat-1:", valid)
	// Output: Valid patient passes pat-1: true

	valid, err = fhirpath.EvaluateToBoolean(invalidPatient, expr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Invalid patient passes pat-1:", valid)
	// Output: Invalid patient passes pat-1: false
}
```

### Observation Debe Tener un Valor o una Razón de Ausencia de Datos

Un invariante FHIR común requiere que un Observation contenga un valor o una razón explícita de por qué el valor está ausente:

```
obs-6: Observation.value.exists() or Observation.dataAbsentReason.exists()
```

```go
// Observation with a value -- valid
obsWithValue := []byte(`{
    "resourceType": "Observation",
    "id": "obs-valid-1",
    "status": "final",
    "code": {
        "coding": [{"system": "http://loinc.org", "code": "2339-0", "display": "Glucose"}]
    },
    "valueQuantity": {
        "value": 95,
        "unit": "mg/dL",
        "system": "http://unitsofmeasure.org",
        "code": "mg/dL"
    }
}`)

// Observation with a data absent reason -- also valid
obsWithDAR := []byte(`{
    "resourceType": "Observation",
    "id": "obs-valid-2",
    "status": "final",
    "code": {
        "coding": [{"system": "http://loinc.org", "code": "2339-0", "display": "Glucose"}]
    },
    "dataAbsentReason": {
        "coding": [{
            "system": "http://terminology.hl7.org/CodeSystem/data-absent-reason",
            "code": "not-performed",
            "display": "Not Performed"
        }]
    }
}`)

// Observation with neither -- invalid
obsInvalid := []byte(`{
    "resourceType": "Observation",
    "id": "obs-invalid",
    "status": "final",
    "code": {
        "coding": [{"system": "http://loinc.org", "code": "2339-0", "display": "Glucose"}]
    }
}`)

expr := "Observation.value.exists() or Observation.dataAbsentReason.exists()"

v1, _ := fhirpath.EvaluateToBoolean(obsWithValue, expr)
fmt.Println("Obs with value:", v1)
// Output: Obs with value: true

v2, _ := fhirpath.EvaluateToBoolean(obsWithDAR, expr)
fmt.Println("Obs with DAR:", v2)
// Output: Obs with DAR: true

v3, _ := fhirpath.EvaluateToBoolean(obsInvalid, expr)
fmt.Println("Obs with neither:", v3)
// Output: Obs with neither: false
```

## Verificación de Múltiples Restricciones

La validación real implica verificar varios invariantes a la vez. Aquí hay un patrón para evaluar una lista de reglas:

```go
package main

import (
	"fmt"

	"github.com/gofhir/fhirpath"
)

// Invariant represents a FHIR constraint rule.
type Invariant struct {
	Key        string
	Severity   string // "error" or "warning"
	Human      string
	Expression string
}

// ValidationResult holds the outcome of checking one invariant.
type ValidationResult struct {
	Key      string
	Severity string
	Human    string
	Passed   bool
	Error    error
}

func main() {
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "pat-validation",
		"active": true,
		"name": [
			{"use": "official", "family": "Nguyen", "given": ["Anh"]}
		],
		"telecom": [
			{"system": "phone", "value": "555-0199", "use": "home"}
		],
		"gender": "female",
		"birthDate": "1988-03-12",
		"address": [
			{"use": "home", "city": "Portland", "state": "OR"}
		]
	}`)

	// Define invariants to check
	invariants := []Invariant{
		{
			Key:        "pat-1",
			Severity:   "error",
			Human:      "Patient must have a name or an identifier",
			Expression: "Patient.name.exists() or Patient.identifier.exists()",
		},
		{
			Key:        "custom-1",
			Severity:   "warning",
			Human:      "Patient should have a birth date",
			Expression: "Patient.birthDate.exists()",
		},
		{
			Key:        "custom-2",
			Severity:   "warning",
			Human:      "Patient should have at least one telecom entry",
			Expression: "Patient.telecom.exists()",
		},
		{
			Key:        "custom-3",
			Severity:   "error",
			Human:      "Active patient must have a contact method",
			Expression: "Patient.active.not() or Patient.telecom.exists()",
		},
	}

	results := validateResource(patient, invariants)

	for _, r := range results {
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
		}
		if r.Error != nil {
			status = "ERROR"
		}
		fmt.Printf("[%s] %-8s %s -- %s\n", status, r.Severity, r.Key, r.Human)
	}
}

func validateResource(resource []byte, invariants []Invariant) []ValidationResult {
	results := make([]ValidationResult, 0, len(invariants))

	for _, inv := range invariants {
		passed, err := fhirpath.EvaluateToBoolean(resource, inv.Expression)
		results = append(results, ValidationResult{
			Key:      inv.Key,
			Severity: inv.Severity,
			Human:    inv.Human,
			Passed:   passed,
			Error:    err,
		})
	}

	return results
}
```

Salida esperada:

```text
[PASS] error    pat-1 -- Patient must have a name or an identifier
[PASS] warning  custom-1 -- Patient should have a birth date
[PASS] warning  custom-2 -- Patient should have at least one telecom entry
[PASS] error    custom-3 -- Active patient must have a contact method
```

## Construcción de un Validador Simple

A continuación se muestra un tipo `Validator` reutilizable que precompila expresiones para mejor rendimiento:

```go
package main

import (
	"fmt"
	"strings"

	"github.com/gofhir/fhirpath"
)

// Rule defines a single validation constraint.
type Rule struct {
	Key         string
	Severity    string
	Human       string
	Expression  string
	compiled    *fhirpath.Expression
}

// Violation represents a failed validation rule.
type Violation struct {
	Key      string
	Severity string
	Message  string
}

// Validator holds a set of precompiled validation rules.
type Validator struct {
	rules []Rule
}

// NewValidator creates a Validator and precompiles all FHIRPath expressions.
func NewValidator(rules []Rule) (*Validator, error) {
	for i := range rules {
		compiled, err := fhirpath.Compile(rules[i].Expression)
		if err != nil {
			return nil, fmt.Errorf("failed to compile rule %s: %w", rules[i].Key, err)
		}
		rules[i].compiled = compiled
	}
	return &Validator{rules: rules}, nil
}

// Validate checks all rules against a FHIR resource.
// Returns a slice of violations (empty means the resource is valid).
func (v *Validator) Validate(resource []byte) []Violation {
	var violations []Violation

	for _, rule := range v.rules {
		result, err := rule.compiled.Evaluate(resource)
		if err != nil {
			violations = append(violations, Violation{
				Key:      rule.Key,
				Severity: rule.Severity,
				Message:  fmt.Sprintf("%s (evaluation error: %v)", rule.Human, err),
			})
			continue
		}

		// A valid invariant must return a single true Boolean
		passed := false
		if len(result) == 1 {
			if b, ok := result[0].(fhirpath.Value); ok {
				passed = b.String() == "true"
			}
		}

		if !passed {
			violations = append(violations, Violation{
				Key:      rule.Key,
				Severity: rule.Severity,
				Message:  rule.Human,
			})
		}
	}

	return violations
}

// IsValid returns true if no error-level violations were found.
func (v *Validator) IsValid(resource []byte) bool {
	for _, violation := range v.Validate(resource) {
		if violation.Severity == "error" {
			return false
		}
	}
	return true
}

func main() {
	rules := []Rule{
		{
			Key:        "pat-1",
			Severity:   "error",
			Human:      "Patient must have a name or an identifier",
			Expression: "Patient.name.exists() or Patient.identifier.exists()",
		},
		{
			Key:        "pat-contact",
			Severity:   "warning",
			Human:      "Patient should have a contact method",
			Expression: "Patient.telecom.exists()",
		},
		{
			Key:        "pat-gender",
			Severity:   "warning",
			Human:      "Patient should have a gender",
			Expression: "Patient.gender.exists()",
		},
	}

	validator, err := NewValidator(rules)
	if err != nil {
		panic(err)
	}

	// Test a valid patient
	validPatient := []byte(`{
		"resourceType": "Patient",
		"id": "good-patient",
		"name": [{"family": "Smith", "given": ["Alice"]}],
		"gender": "female",
		"telecom": [{"system": "phone", "value": "555-1234"}]
	}`)

	violations := validator.Validate(validPatient)
	fmt.Printf("Valid patient: %d violations\n", len(violations))
	// Output: Valid patient: 0 violations

	// Test a patient missing required fields
	incompletePatient := []byte(`{
		"resourceType": "Patient",
		"id": "incomplete-patient",
		"birthDate": "2000-01-01"
	}`)

	violations = validator.Validate(incompletePatient)
	fmt.Printf("Incomplete patient: %d violations\n", len(violations))
	for _, v := range violations {
		fmt.Printf("  [%s] %s: %s\n", v.Severity, v.Key, v.Message)
	}

	isValid := validator.IsValid(incompletePatient)
	fmt.Println("Is valid:", isValid)

	// Expected output:
	// Incomplete patient: 3 violations
	//   [error] pat-1: Patient must have a name or an identifier
	//   [warning] pat-contact: Patient should have a contact method
	//   [warning] pat-gender: Patient should have a gender
	// Is valid: false

	// Summarize by severity
	errors := 0
	warnings := 0
	for _, v := range violations {
		switch v.Severity {
		case "error":
			errors++
		case "warning":
			warnings++
		}
	}
	fmt.Printf("Summary: %d error(s), %d warning(s)\n", errors, warnings)

	_ = strings.Builder{} // suppress import warning in example
}
```

## Invariantes FHIR Comunes

Aquí hay una tabla de referencia de invariantes FHIR de uso común que puede evaluar con esta biblioteca:

| Recurso | Clave | Expresión |
|---------|-------|-----------|
| Patient | pat-1 | `Patient.name.exists() or Patient.identifier.exists()` |
| Observation | obs-6 | `Observation.value.exists() or Observation.dataAbsentReason.exists()` |
| Observation | obs-7 | `Observation.component.value.exists() or Observation.component.dataAbsentReason.exists()` |
| Bundle | bdl-1 | `Bundle.total.empty() or (Bundle.type = 'searchset') or (Bundle.type = 'history')` |
| AllergyIntolerance | ait-1 | `AllergyIntolerance.clinicalStatus.exists() or AllergyIntolerance.verificationStatus.coding.where(code = 'entered-in-error').exists()` |
| Condition | con-3 | `Condition.clinicalStatus.exists() or Condition.verificationStatus.coding.where(code = 'entered-in-error').exists()` |

### Evaluación de Invariantes Estándar

```go
// Bundle total invariant: total is only allowed for searchset and history bundles
bundle := []byte(`{
    "resourceType": "Bundle",
    "id": "search-bundle",
    "type": "searchset",
    "total": 3,
    "entry": [
        {"resource": {"resourceType": "Patient", "id": "p1"}},
        {"resource": {"resourceType": "Patient", "id": "p2"}},
        {"resource": {"resourceType": "Patient", "id": "p3"}}
    ]
}`)

bdl1 := "Bundle.total.empty() or (Bundle.type = 'searchset') or (Bundle.type = 'history')"
passes, err := fhirpath.EvaluateToBoolean(bundle, bdl1)
if err != nil {
    log.Fatal(err)
}
fmt.Println("Bundle passes bdl-1:", passes)
// Output: Bundle passes bdl-1: true
```

## Validación con la Función de Conveniencia Exists

Para verificaciones de presencia simples, la función `fhirpath.Exists` es la opción más concisa:

```go
allergyIntolerance := []byte(`{
    "resourceType": "AllergyIntolerance",
    "id": "allergy-1",
    "clinicalStatus": {
        "coding": [{
            "system": "http://terminology.hl7.org/CodeSystem/allergyintolerance-clinical",
            "code": "active"
        }]
    },
    "verificationStatus": {
        "coding": [{
            "system": "http://terminology.hl7.org/CodeSystem/allergyintolerance-verification",
            "code": "confirmed"
        }]
    },
    "code": {
        "coding": [{
            "system": "http://snomed.info/sct",
            "code": "227493005",
            "display": "Cashew nuts"
        }]
    },
    "patient": {"reference": "Patient/example"}
}`)

hasClinicalStatus, _ := fhirpath.Exists(allergyIntolerance,
    "AllergyIntolerance.clinicalStatus")
fmt.Println("Has clinical status:", hasClinicalStatus)
// Output: Has clinical status: true

hasCode, _ := fhirpath.Exists(allergyIntolerance,
    "AllergyIntolerance.code")
fmt.Println("Has code:", hasCode)
// Output: Has code: true

hasOnset, _ := fhirpath.Exists(allergyIntolerance,
    "AllergyIntolerance.onset")
fmt.Println("Has onset:", hasOnset)
// Output: Has onset: false
```
