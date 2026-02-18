---
title: "Terminology Services"
linkTitle: "Terminology Services"
weight: 4
description: >
  Connect the memberOf() and conformsTo() functions to external terminology servers
  and profile validators by implementing the TerminologyService and ProfileValidator interfaces.
---

## Overview

Two FHIRPath functions require external service integration to produce meaningful
results:

- **`memberOf(valueSetUrl)`** -- checks whether a code, Coding, or CodeableConcept
  belongs to a given ValueSet.
- **`conformsTo(profileUrl)`** -- checks whether a resource conforms to a given
  StructureDefinition (profile).

Without a backing service, both functions return an **empty collection** (meaning
"unknown"), which is safe but not useful for validation. This page shows how to
implement the two interfaces that power these functions.

## The TerminologyService Interface

The `TerminologyService` interface is defined in the `eval` package:

```go
package eval

// TerminologyService handles terminology operations like ValueSet membership.
type TerminologyService interface {
    // MemberOf checks if a code/Coding/CodeableConcept is in the specified ValueSet.
    // Returns true if the code is in the ValueSet, false otherwise.
    // Returns error if the ValueSet cannot be resolved or validation fails.
    MemberOf(ctx context.Context, code interface{}, valueSetURL string) (bool, error)
}
```

The `code` parameter is a `map[string]interface{}` with the following possible
shapes depending on the input type:

| Input Type        | Map Keys                                        |
|-------------------|-------------------------------------------------|
| Simple code string| `{"code": "active"}`                             |
| Coding            | `{"system": "...", "code": "...", "version": "...", "display": "..."}` |
| CodeableConcept   | `{"coding": [{"system": "...", "code": "..."}], "text": "..."}` |

Your implementation should inspect the map and call the appropriate terminology
operation (typically a FHIR® `$validate-code` or `$expand` operation).

## The ProfileValidator Interface

The `ProfileValidator` interface is also defined in the `eval` package:

```go
package eval

// ProfileValidator handles profile conformance validation.
type ProfileValidator interface {
    // ConformsTo checks if a resource conforms to the specified profile.
    // Returns true if the resource conforms, false otherwise.
    ConformsTo(ctx context.Context, resource []byte, profileURL string) (bool, error)
}
```

The `resource` parameter is the raw JSON of the resource being validated. The
`profileURL` is the canonical URL of the StructureDefinition to validate against.

## Using memberOf()

The `memberOf()` function is called on a code, Coding, or CodeableConcept element:

```fhirpath
// Check if a patient's marital status is in a specific ValueSet.
Patient.maritalStatus.coding.memberOf('http://hl7.org/fhir/ValueSet/marital-status')

// Check a simple code value.
Observation.status.memberOf('http://hl7.org/fhir/ValueSet/observation-status')
```

When evaluated:

1. The library extracts the code information from the input element.
2. It calls `TerminologyService.MemberOf()` with the extracted code data and the
   ValueSet URL.
3. The function returns `true` if the code is a member, `false` if not.
4. If no `TerminologyService` is configured, the function returns an empty collection.

## Using conformsTo()

The `conformsTo()` function is called on a resource:

```fhirpath
// Check if a resource conforms to the US Core Patient profile.
conformsTo('http://hl7.org/fhir/us/core/StructureDefinition/us-core-patient')
```

When evaluated:

1. The library extracts the raw JSON of the resource.
2. It calls `ProfileValidator.ConformsTo()` with the JSON and the profile URL.
3. The function returns `true` if the resource conforms, `false` if not.
4. If no `ProfileValidator` is configured, the function returns an empty collection.

## Implementation Example

Below is a complete example that connects to a FHIR® terminology server for
`memberOf()` validation and implements a simple profile validator.

### Terminology Service Implementation

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"

    "github.com/gofhir/fhirpath/eval"
)

// FHIRTerminologyService validates codes against a FHIR terminology server.
type FHIRTerminologyService struct {
    BaseURL    string
    HTTPClient *http.Client
}

// Ensure interface compliance at compile time.
var _ eval.TerminologyService = (*FHIRTerminologyService)(nil)

func (ts *FHIRTerminologyService) MemberOf(
    ctx context.Context,
    code interface{},
    valueSetURL string,
) (bool, error) {
    codeMap, ok := code.(map[string]interface{})
    if !ok {
        return false, fmt.Errorf("unexpected code type: %T", code)
    }

    // Build the $validate-code request parameters.
    params := url.Values{}
    params.Set("url", valueSetURL)

    if system, ok := codeMap["system"].(string); ok {
        params.Set("system", system)
    }
    if codeVal, ok := codeMap["code"].(string); ok {
        params.Set("code", codeVal)
    }
    if version, ok := codeMap["version"].(string); ok {
        params.Set("version", version)
    }

    reqURL := fmt.Sprintf("%s/ValueSet/$validate-code?%s", ts.BaseURL, params.Encode())
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
    if err != nil {
        return false, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := ts.HTTPClient.Do(req)
    if err != nil {
        return false, fmt.Errorf("terminology request: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return false, fmt.Errorf("read response: %w", err)
    }

    // Parse the Parameters response.
    var result struct {
        Parameter []struct {
            Name         string `json:"name"`
            ValueBoolean *bool  `json:"valueBoolean,omitempty"`
        } `json:"parameter"`
    }
    if err := json.Unmarshal(body, &result); err != nil {
        return false, fmt.Errorf("parse response: %w", err)
    }

    for _, param := range result.Parameter {
        if param.Name == "result" && param.ValueBoolean != nil {
            return *param.ValueBoolean, nil
        }
    }

    return false, fmt.Errorf("no result parameter in response")
}
```

### Profile Validator Implementation

```go
// SimpleProfileValidator checks resource conformance using a FHIR server's
// $validate operation.
type SimpleProfileValidator struct {
    BaseURL    string
    HTTPClient *http.Client
}

// Ensure interface compliance at compile time.
var _ eval.ProfileValidator = (*SimpleProfileValidator)(nil)

func (pv *SimpleProfileValidator) ConformsTo(
    ctx context.Context,
    resource []byte,
    profileURL string,
) (bool, error) {
    // Determine resource type from the JSON.
    var meta struct {
        ResourceType string `json:"resourceType"`
    }
    if err := json.Unmarshal(resource, &meta); err != nil {
        return false, fmt.Errorf("parse resource: %w", err)
    }

    reqURL := fmt.Sprintf("%s/%s/$validate?profile=%s",
        pv.BaseURL, meta.ResourceType, url.QueryEscape(profileURL))

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL,
        bytes.NewReader(resource))
    if err != nil {
        return false, fmt.Errorf("create request: %w", err)
    }
    req.Header.Set("Content-Type", "application/fhir+json")
    req.Header.Set("Accept", "application/fhir+json")

    resp, err := pv.HTTPClient.Do(req)
    if err != nil {
        return false, fmt.Errorf("validation request: %w", err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return false, fmt.Errorf("read response: %w", err)
    }

    // Parse the OperationOutcome.
    var outcome struct {
        Issue []struct {
            Severity string `json:"severity"`
        } `json:"issue"`
    }
    if err := json.Unmarshal(body, &outcome); err != nil {
        return false, fmt.Errorf("parse outcome: %w", err)
    }

    // The resource conforms if there are no error- or fatal-level issues.
    for _, issue := range outcome.Issue {
        if issue.Severity == "error" || issue.Severity == "fatal" {
            return false, nil
        }
    }
    return true, nil
}
```

### Wiring Everything Together

Since there are no `WithTerminologyService` or `WithProfileValidator` functional
options, you wire these services by creating an `eval.Context` directly:

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "time"

    "github.com/gofhir/fhirpath"
    "github.com/gofhir/fhirpath/eval"
)

func main() {
    terminologyService := &FHIRTerminologyService{
        BaseURL:    "http://tx.fhir.org/r4",
        HTTPClient: &http.Client{Timeout: 10 * time.Second},
    }

    profileValidator := &SimpleProfileValidator{
        BaseURL:    "http://hapi.fhir.org/baseR4",
        HTTPClient: &http.Client{Timeout: 10 * time.Second},
    }

    patient := []byte(`{
        "resourceType": "Patient",
        "maritalStatus": {
            "coding": [{
                "system": "http://terminology.hl7.org/CodeSystem/v3-MaritalStatus",
                "code": "M"
            }]
        }
    }`)

    // Compile the expression.
    expr := fhirpath.MustCompile(
        "Patient.maritalStatus.coding.memberOf('http://hl7.org/fhir/ValueSet/marital-status')",
    )

    // Create an eval.Context with the services attached.
    ctx := eval.NewContext(patient)
    ctx.SetTerminologyService(terminologyService)
    ctx.SetProfileValidator(profileValidator)
    ctx.SetContext(context.Background())
    ctx.SetLimit("maxDepth", 100)
    ctx.SetLimit("maxCollectionSize", 10000)

    result, err := expr.EvaluateWithContext(ctx)
    if err != nil {
        fmt.Println("error:", err)
        return
    }
    fmt.Println(result) // [true] if the code is in the ValueSet
}
```

## Behavior When Services Are Not Configured

| Scenario                        | memberOf() returns | conformsTo() returns |
|---------------------------------|--------------------|----------------------|
| No service configured           | empty collection   | empty collection     |
| Service returns an error        | empty collection   | empty collection     |
| Code is a member / conforms     | `[true]`           | `[true]`             |
| Code is not a member / does not conform | `[false]`  | `[false]`            |
| Input is empty                  | empty collection   | empty collection     |

This behavior follows the FHIRPath specification's treatment of unknown results:
when the system cannot determine the answer, it returns an empty collection rather
than raising an error.

## Summary

| Interface              | Method                                                          | Used By        |
|------------------------|-----------------------------------------------------------------|----------------|
| `eval.TerminologyService` | `MemberOf(ctx, code, valueSetURL) (bool, error)`            | `memberOf()`   |
| `eval.ProfileValidator`   | `ConformsTo(ctx, resource, profileURL) (bool, error)`        | `conformsTo()` |

Both services are attached to an `eval.Context` via `SetTerminologyService()` and
`SetProfileValidator()` respectively. Create the context, attach the services, and
call `expr.EvaluateWithContext(ctx)` to use them.
