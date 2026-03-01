package fhirpath_test

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/types"
)

// testModel implements fhirpath.Model for integration tests.
type testModel struct {
	choiceTypes      map[string][]string
	typeOf           map[string]string
	referenceTargets map[string][]string
	parentType       map[string]string
	resolvePath      map[string]string
	resources        map[string]bool
}

func (m *testModel) ChoiceTypes(path string) []string      { return m.choiceTypes[path] }
func (m *testModel) TypeOf(path string) string             { return m.typeOf[path] }
func (m *testModel) ReferenceTargets(path string) []string { return m.referenceTargets[path] }
func (m *testModel) ParentType(typeName string) string     { return m.parentType[typeName] }
func (m *testModel) IsSubtype(child, parent string) bool {
	if child == parent {
		return true
	}
	current := child
	for {
		p, ok := m.parentType[current]
		if !ok || p == "" {
			return false
		}
		if p == parent {
			return true
		}
		current = p
	}
}
func (m *testModel) ResolvePath(path string) string {
	if resolved, ok := m.resolvePath[path]; ok {
		return resolved
	}
	return path
}
func (m *testModel) IsResource(typeName string) bool { return m.resources[typeName] }

func newIntegrationModel() *testModel {
	return &testModel{
		choiceTypes: map[string][]string{
			"Observation.value": {"Quantity", "CodeableConcept", "string", "boolean", "integer", "Range", "Ratio", "SampledData", "time", "dateTime", "Period"},
			"Patient.deceased":  {"boolean", "dateTime"},
		},
		typeOf: map[string]string{
			"Patient.name":         "HumanName",
			"Patient.gender":       "code",
			"Patient.active":       "boolean",
			"Observation.subject":  "Reference",
			"Observation.status":   "code",
			"Observation.code":     "CodeableConcept",
			"Patient.contact":      "BackboneElement",
			"Patient.contact.name": "HumanName",
		},
		referenceTargets: map[string][]string{
			"Observation.subject": {"Patient", "Group", "Device", "Location"},
		},
		parentType: map[string]string{
			"Patient":         "DomainResource",
			"Observation":     "DomainResource",
			"Condition":       "DomainResource",
			"Bundle":          "Resource",
			"DomainResource":  "Resource",
			"Age":             "Quantity",
			"Duration":        "Quantity",
			"Distance":        "Quantity",
			"Count":           "Quantity",
			"MoneyQuantity":   "Quantity",
			"SimpleQuantity":  "Quantity",
			"BackboneElement": "Element",
		},
		resolvePath: map[string]string{
			"Questionnaire.item.item": "Questionnaire.item",
		},
		resources: map[string]bool{
			"Patient":     true,
			"Observation": true,
			"Bundle":      true,
			"Condition":   true,
			"Parameters":  true,
		},
	}
}

func TestWithModel_IsExpression(t *testing.T) {
	model := newIntegrationModel()

	tests := []struct {
		name     string
		resource []byte
		expr     string
		useModel bool
		expected bool
	}{
		{
			name:     "Patient is DomainResource without model",
			resource: []byte(`{"resourceType": "Patient", "id": "1"}`),
			expr:     "Patient.is(DomainResource)",
			useModel: false,
			expected: true,
		},
		{
			name:     "Patient is DomainResource with model",
			resource: []byte(`{"resourceType": "Patient", "id": "1"}`),
			expr:     "Patient.is(DomainResource)",
			useModel: true,
			expected: true,
		},
		{
			name:     "Patient is Resource with model",
			resource: []byte(`{"resourceType": "Patient", "id": "1"}`),
			expr:     "Patient.is(Resource)",
			useModel: true,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []fhirpath.EvalOption
			if tt.useModel {
				opts = append(opts, fhirpath.WithModel(model))
			}

			expr, err := fhirpath.Compile(tt.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			result, err := expr.EvaluateWithOptions(tt.resource, opts...)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if result.Empty() {
				t.Fatal("expected non-empty result")
			}

			b, ok := result[0].(types.Boolean)
			if !ok {
				t.Fatalf("expected Boolean, got %T", result[0])
			}

			if b.Bool() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, b.Bool())
			}
		})
	}
}

func TestWithModel_AsExpression(t *testing.T) {
	model := newIntegrationModel()

	patient := []byte(`{"resourceType": "Patient", "id": "1", "active": true}`)

	tests := []struct {
		name      string
		expr      string
		useModel  bool
		wantCount int
	}{
		{
			name:      "Patient as DomainResource with model returns patient",
			expr:      "Patient.as(DomainResource)",
			useModel:  true,
			wantCount: 1,
		},
		{
			name:      "Patient as Resource with model returns patient",
			expr:      "Patient.as(Resource)",
			useModel:  true,
			wantCount: 1,
		},
		{
			name:      "Patient as Observation with model returns empty",
			expr:      "Patient.as(Observation)",
			useModel:  true,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []fhirpath.EvalOption
			if tt.useModel {
				opts = append(opts, fhirpath.WithModel(model))
			}

			expr, err := fhirpath.Compile(tt.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			result, err := expr.EvaluateWithOptions(patient, opts...)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if len(result) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
			}
		})
	}
}

func TestWithModel_OfTypeExpression(t *testing.T) {
	model := newIntegrationModel()

	bundle := []byte(`{
		"resourceType": "Bundle",
		"type": "searchset",
		"entry": [
			{"resource": {"resourceType": "Patient", "id": "1"}},
			{"resource": {"resourceType": "Observation", "id": "2"}},
			{"resource": {"resourceType": "Condition", "id": "3"}}
		]
	}`)

	tests := []struct {
		name      string
		expr      string
		useModel  bool
		wantCount int
	}{
		{
			name:      "ofType Patient with model",
			expr:      "Bundle.entry.resource.ofType(Patient)",
			useModel:  true,
			wantCount: 1,
		},
		{
			name:      "ofType Patient without model",
			expr:      "Bundle.entry.resource.ofType(Patient)",
			useModel:  false,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []fhirpath.EvalOption
			if tt.useModel {
				opts = append(opts, fhirpath.WithModel(model))
			}

			expr, err := fhirpath.Compile(tt.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			result, err := expr.EvaluateWithOptions(bundle, opts...)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if len(result) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
			}
		})
	}
}

func TestWithModel_PolymorphicResolution(t *testing.T) {
	model := newIntegrationModel()

	observation := []byte(`{
		"resourceType": "Observation",
		"status": "final",
		"code": {"coding": [{"system": "http://loinc.org", "code": "1234"}]},
		"valueQuantity": {
			"value": 120,
			"unit": "mmHg",
			"system": "http://unitsofmeasure.org",
			"code": "mm[Hg]"
		}
	}`)

	tests := []struct {
		name      string
		expr      string
		useModel  bool
		wantCount int
	}{
		{
			name:      "Observation.value resolves with model",
			expr:      "Observation.value",
			useModel:  true,
			wantCount: 1,
		},
		{
			name:      "Observation.value resolves without model (fallback)",
			expr:      "Observation.value",
			useModel:  false,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []fhirpath.EvalOption
			if tt.useModel {
				opts = append(opts, fhirpath.WithModel(model))
			}

			expr, err := fhirpath.Compile(tt.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			result, err := expr.EvaluateWithOptions(observation, opts...)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if len(result) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
			}
		})
	}
}

func TestWithModel_BackwardCompatibility(t *testing.T) {
	// All these expressions should work identically with and without a model.
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "example",
		"active": true,
		"name": [{"family": "Smith", "given": ["John"]}],
		"birthDate": "1990-01-15"
	}`)

	expressions := []struct {
		expr      string
		wantCount int
	}{
		{"Patient.name.family", 1},
		{"Patient.active", 1},
		{"Patient.id", 1},
		{"Patient.name.given", 1},
		{"Patient.name.where(family = 'Smith')", 1},
		{"Patient.name.exists()", 1},
		{"Patient.name.all(family.exists())", 1},
		{"Patient.name.select(family)", 1},
	}

	model := newIntegrationModel()

	for _, tt := range expressions {
		t.Run(tt.expr+"_without_model", func(t *testing.T) {
			expr, err := fhirpath.Compile(tt.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			result, err := expr.EvaluateWithOptions(patient)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if len(result) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
			}
		})

		t.Run(tt.expr+"_with_model", func(t *testing.T) {
			expr, err := fhirpath.Compile(tt.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			result, err := expr.EvaluateWithOptions(patient, fhirpath.WithModel(model))
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if len(result) != tt.wantCount {
				t.Errorf("expected %d results, got %d", tt.wantCount, len(result))
			}
		})
	}
}

func TestWithModel_ModelAuthoritative(t *testing.T) {
	model := newIntegrationModel()

	// Patient JSON with a name (HumanName type)
	patient := []byte(`{
		"resourceType": "Patient",
		"id": "1",
		"name": [{"family": "Smith", "given": ["John"]}]
	}`)

	tests := []struct {
		name     string
		resource []byte
		expr     string
		useModel bool
		expected bool
	}{
		{
			// Model-authoritative: HumanName is NOT a Resource (model doesn't define it as one)
			// Without model, the heuristic would incorrectly say true (PascalCase)
			name:     "HumanName is NOT Resource with model",
			resource: patient,
			expr:     "Patient.name.first().is(Resource)",
			useModel: true,
			expected: false,
		},
		{
			// Age is Quantity via model's parentType chain
			name:     "Age is Quantity with model (via parentType)",
			resource: patient,
			expr:     "Patient.is(DomainResource)",
			useModel: true,
			expected: true,
		},
		{
			// Patient is NOT Observation with model
			name:     "Patient is not Observation with model",
			resource: patient,
			expr:     "Patient.is(Observation)",
			useModel: true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []fhirpath.EvalOption
			if tt.useModel {
				opts = append(opts, fhirpath.WithModel(model))
			}

			expr, err := fhirpath.Compile(tt.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			result, err := expr.EvaluateWithOptions(tt.resource, opts...)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if result.Empty() {
				t.Fatal("expected non-empty result")
			}

			b, ok := result[0].(types.Boolean)
			if !ok {
				t.Fatalf("expected Boolean, got %T", result[0])
			}

			if b.Bool() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, b.Bool())
			}
		})
	}
}

func TestWithModel_TypeExpressionOperator(t *testing.T) {
	model := newIntegrationModel()

	patient := []byte(`{"resourceType": "Patient", "id": "1"}`)

	tests := []struct {
		name     string
		expr     string
		useModel bool
		expected bool
	}{
		{
			name:     "is Patient with model",
			expr:     "Patient is Patient",
			useModel: true,
			expected: true,
		},
		{
			name:     "is DomainResource with model",
			expr:     "Patient is DomainResource",
			useModel: true,
			expected: true,
		},
		{
			name:     "is Resource with model",
			expr:     "Patient is Resource",
			useModel: true,
			expected: true,
		},
		{
			name:     "is Observation with model (false)",
			expr:     "Patient is Observation",
			useModel: true,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []fhirpath.EvalOption
			if tt.useModel {
				opts = append(opts, fhirpath.WithModel(model))
			}

			expr, err := fhirpath.Compile(tt.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			result, err := expr.EvaluateWithOptions(patient, opts...)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if result.Empty() {
				t.Fatal("expected non-empty result")
			}

			b, ok := result[0].(types.Boolean)
			if !ok {
				t.Fatalf("expected Boolean, got %T", result[0])
			}

			if b.Bool() != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, b.Bool())
			}
		})
	}
}
