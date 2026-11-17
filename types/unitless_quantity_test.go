package types

import (
	"testing"

	"github.com/shopspring/decimal"
)

// Plan 026: la unidad de un Quantity de FHIR. Sin code ni unit es '1', la
// unidad por defecto de UCUM (FHIRPath 3.0.0, Conversion); un code o unit
// vacío se lee como ausente; un code ganó siempre sobre el unit.
func TestUnitlessQuantityUnit(t *testing.T) {
	calendar, ok := CalendarUnitForUCUMCode("a")
	if !ok {
		t.Fatal("the calendar keyword for UCUM a is missing")
	}
	tests := []struct {
		name, data, want string
	}{
		{"no unit and no code", `{"value":1}`, "1"},
		{"a unit", `{"value":1,"unit":"mg"}`, "mg"},
		{"a code over a unit", `{"value":1,"code":"mg","unit":"milligram"}`, "mg"},
		{"an empty code beside a unit", `{"value":1,"code":"","unit":"mg"}`, "mg"},
		{"an empty unit", `{"value":1,"unit":""}`, "1"},
		{"a calendar code with UCUM", `{"value":1,"system":"http://unitsofmeasure.org","code":"a"}`, calendar},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q, ok, err := NewObjectValue([]byte(tt.data)).AsQuantity()
			if err != nil || !ok {
				t.Fatalf("AsQuantity() = %v, %v, %v", q, ok, err)
			}
			if got := q.Unit(); got != tt.want {
				t.Errorf("Unit() = %q, want %q", got, tt.want)
			}
		})
	}
}

// Tras 026 el motor no produce una unidad vacía, pero los constructores
// exportados sí: una cantidad sin unidad toma la del otro operando y se ordena
// exacta, sin escribir un valor fuera del límite (#232).
func TestAQuantityWithNoUnitTakesTheOthers(t *testing.T) {
	beyond := NewQuantityFromDecimal(decimal.New(1, 1000), "")
	order, err := beyond.Compare(NewQuantityFromDecimal(decimal.NewFromInt(2), DefaultQuantityUnit))
	if err != nil || order != 1 {
		t.Errorf("1e1000 (no unit) against 2 '1' = %d, %v; want 1, nil", order, err)
	}
}
