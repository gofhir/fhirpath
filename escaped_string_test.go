package fhirpath_test

import (
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
	"github.com/gofhir/fhirpath/types"
)

// escaped writes a JSON document with a backslash wherever it holds BS, so
// that the escapes under test are in the document and not decoded on the way
// into the source.
func escaped(json string) []byte {
	return []byte(strings.ReplaceAll(json, "BS", string(rune(92))))
}

// A JSON string may be written with escapes, and it is the same string:
// "PatiBSu0065nt" is Patient. Navigation decoded the root's resourceType but
// Location took it as it was written, so a validator comparing Location with
// its own path found nothing. And a root whose first resourceType names
// nothing is named as navigation types it, by the one that does.
func TestARootIsNamedByItsDecodedResourceType(t *testing.T) {
	for _, tt := range []struct {
		json, want string
	}{
		{`{"resourceType":"PatiBSu0065nt","name":[{"family":"A"}]}`, "Patient.name[0]"},
		{`{"resourceType":"BSu0050atient","name":[{"family":"A"}]}`, "Patient.name[0]"},
		{`{"resourceType":null,"resourceType":"Patient","name":[{"family":"A"}]}`, "Patient.name[0]"},
		{`{"resourceType":"Patient","nBSu0061me":[{"family":"A"}]}`, "Patient.name[0]"},
	} {
		data := escaped(tt.json)
		for how, read := range map[string]func([]byte) (types.Collection, error){
			"JSONToCollection": types.JSONToCollection,
			"ReadRoot":         types.ReadRoot,
			"ReadRootWithType": func(d []byte) (types.Collection, error) { return types.ReadRootWithType(d, "Patient") },
		} {
			col, err := read(data)
			if err != nil {
				t.Fatal(err)
			}
			result, err := fhirpath.MustCompile("Patient.name").EvaluateWithContext(eval.NewContextForRoot(col))
			if err != nil || len(result) != 1 {
				t.Fatalf("%s %s: %v %v", how, data, err, result)
			}
			if got := result[0].(*types.ObjectValue).Location(); got != tt.want {
				t.Errorf("%s %s: name is at %q, want %q", how, data, got, tt.want)
			}
		}
	}
}

// A Quantity's code, unit and system are strings like any other, and read
// decoded: "http:BS/BS/unitsofmeasure.org", as PHP writes a slash, is UCUM, so
// a code of a is a year; "mBSu0067" is mg; "BSu00b5g", as Python writes what
// is not ASCII, is µg. A code that is not a string names no unit, which is
// left to the unit.
func TestAQuantityReadsItsUnitDecoded(t *testing.T) {
	for _, tt := range []struct {
		quantity, expr, want string
	}{
		{`{"value":1,"system":"http:BS/BS/unitsofmeasure.org","code":"a"}`, "valueQuantity = 1 year", "[true]"},
		{`{"value":1,"system":"http:BS/BS/unitsofmeasure.org","code":"a"}`, "valueQuantity.toQuantity().toString()", "[1 year]"},
		{`{"value":5,"system":"http://unitsofmeasure.org","code":"mBSu0067"}`, "valueQuantity = 5 'mg'", "[true]"},
		{`{"value":5,"system":"http://unitsofmeasure.org","code":"mBSu0067"}`, "valueQuantity.toQuantity().toString()", "[5 'mg']"},
		{`{"value":5,"unit":"BSu00b5g"}`, "valueQuantity.toQuantity().toString()", "[5 'µg']"},
		{`{"value":5,"code":null,"unit":"mg"}`, "valueQuantity.toQuantity().toString()", "[5 'mg']"},
		{`{"value":5,"code":7,"unit":"mg"}`, "valueQuantity.toQuantity().toString()", "[5 'mg']"},
	} {
		data := escaped(`{"resourceType":"Observation","valueQuantity":` + tt.quantity + `}`)
		result, err := fhirpath.MustCompile(tt.expr).Evaluate(data)
		if err != nil {
			t.Fatal(err)
		}
		if got := result.String(); got != tt.want {
			t.Errorf("%s on %s = %s, want %s", tt.expr, data, got, tt.want)
		}
	}
}
