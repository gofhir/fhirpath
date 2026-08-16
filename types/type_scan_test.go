package types

import "testing"

// The type of an object is read in one pass, and the pass ends as soon as a
// resourceType is seen. What the object carries besides it must not change the
// answer, wherever the field sits.
func TestTypeFromResourceType(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "declared first, as FHIR writes it",
			json: `{"resourceType":"Patient","name":[{"family":"Chalmers"}]}`,
			want: "Patient",
		},
		{
			name: "declared after fields that would infer another type",
			json: `{"family":"Chalmers","given":["Peter"],"resourceType":"Patient"}`,
			want: "Patient",
		},
		{
			name: "declared among fields of an inferred type",
			json: `{"system":"http://loinc.org","resourceType":"Observation","code":"1234"}`,
			want: "Observation",
		},
		{
			// The scan hands over the name as JSON wrote it, so an escape in
			// it has to be resolved rather than taken literally.
			name: "escaped in the value",
			json: `{"resourceType":"Pat\u0069ent"}`,
			want: "Patient",
		},
		{
			name: "null names nothing, so the fields decide",
			json: `{"resourceType":null,"value":5,"unit":"mg"}`,
			want: typeQuantity,
		},
		{
			name: "a number names nothing either",
			json: `{"resourceType":42,"value":5,"unit":"mg"}`,
			want: typeQuantity,
		},
		{
			name: "nor does an object",
			json: `{"resourceType":{"a":1},"value":5,"unit":"mg"}`,
			want: typeQuantity,
		},
		{
			// Reading this as a type gave the empty string, which is not a
			// type and which Type() would work out again on every call, since
			// an empty answer is how it records having none. The fields decide
			// instead.
			name: "an empty name is no name",
			json: `{"resourceType":"","value":5,"unit":"mg"}`,
			want: typeQuantity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewObjectValue([]byte(tt.json)).Type(); got != tt.want {
				t.Errorf("Type() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Without a resourceType the type is inferred from the fields the object
// carries, which the same pass collects.
func TestTypeInferredFromFields(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"quantity", `{"value":5,"unit":"mg"}`, typeQuantity},
		{"identifier keeps its string value", `{"system":"urn:oid:1.2","value":"12345"}`, typeIdentifier},
		{"coding", `{"system":"http://loinc.org","code":"1234"}`, typeCoding},
		{"codeable concept", `{"coding":[{"code":"1234"}]}`, typeCodeableConcept},
		{"reference", `{"reference":"Patient/1"}`, typeReference},
		{"period", `{"start":"2024-01-01"}`, typePeriod},
		{"range", `{"low":{"value":1},"high":{"value":9}}`, typeRange},
		{"ratio", `{"numerator":{"value":1},"denominator":{"value":2}}`, typeRatio},
		{"attachment", `{"contentType":"text/plain"}`, typeAttachment},
		{"human name", `{"family":"Chalmers","given":["Peter"]}`, typeHumanName},
		{"address", `{"city":"Santiago","postalCode":"8320000"}`, typeAddress},
		{"contact point", `{"system":"phone","use":"home"}`, typeContactPoint},
		{"annotation", `{"text":"a note","time":"2024-01-01"}`, typeAnnotation},
		{"nothing recognizable", `{"foo":"bar"}`, typeObject},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewObjectValue([]byte(tt.json)).Type(); got != tt.want {
				t.Errorf("Type() = %q, want %q", got, tt.want)
			}
		})
	}
}

// looksTemporal decides which strings are worth offering to the date parsers.
// It must not turn away anything they would have accepted.
func TestLooksTemporalAgreesWithTheParsers(t *testing.T) {
	temporal := []string{
		"2024",
		"2024-12",
		"2024-12-25",
		"2024-12-25T10:30:00Z",
		"2024-12-25T10:30:00+01:00",
		"0001-01-01",
	}
	for _, s := range temporal {
		t.Run("temporal/"+s, func(t *testing.T) {
			if !looksTemporal(s) {
				t.Errorf("looksTemporal(%q) = false, but it parses as a temporal value", s)
			}
			if v := tryParseTemporalString(s); v == nil {
				t.Errorf("tryParseTemporalString(%q) = nil, want a value", s)
			}
		})
	}

	// The strings a resource is otherwise full of: codes, names, URLs, ids.
	ordinary := []string{
		"male", "official", "phone", "Chalmers", "Peter",
		"http://loinc.org", "urn:uuid:1", "1234-5", "", "T12:00", "-2024",
	}
	for _, s := range ordinary {
		t.Run("ordinary/"+s, func(t *testing.T) {
			if v := tryParseTemporalString(s); v != nil {
				t.Errorf("tryParseTemporalString(%q) = %v, want nil", s, v)
			}
		})
	}
}

// A string that begins with four digits but is not a date stays a string.
func TestNumericLookingStringsStayStrings(t *testing.T) {
	obj := NewObjectValue([]byte(`{"code":"1234abc","other":"12345"}`))

	for _, field := range []string{"code", "other"} {
		value, ok := obj.Get(field)
		if !ok {
			t.Fatalf("field %q not found", field)
		}
		if _, isString := value.(String); !isString {
			t.Errorf("field %q read as %T (%s), want a String", field, value, value.Type())
		}
	}
}
