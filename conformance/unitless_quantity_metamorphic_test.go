package conformance

import (
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// Plan 026, criterio 2: un Quantity sin unidad leído del recurso responde como
// el literal 1 '1' en cada operador y función que toma cantidades, en ambos
// órdenes. Excluidos por diseño: navegación, is/as/type(), toString() (se prueba
// aparte), pertenencia y distinct (FP-28), y un valor más allá de la cota.

func TestUnitlessQuantityAnswersAsOneUnitOne(t *testing.T) {
	const (
		u    = "Observation.referenceRange[0].low"
		lit  = "1 '1'"
		high = "Observation.referenceRange[0].high"
	)
	operands := []string{"1 '1'", "2 '1'", "1 'mg'", "1", "2.5", "2 'g'", high}
	operators := []string{"=", "!=", "~", "!~", "<", "<=", ">", ">=", "+", "-", "*", "/"}
	// lectura del recurso y literal, con la misma forma
	pairs := make([][2]string, 0, (2*len(operators)+1)*len(operands)+3)
	for _, x := range operands {
		for _, op := range operators {
			pairs = append(pairs,
				[2]string{"(" + u + " " + op + " " + x + ")", "(" + lit + " " + op + " " + x + ")"},
				[2]string{"(" + x + " " + op + " " + u + ")", "(" + x + " " + op + " " + lit + ")"})
		}
		pairs = append(pairs, [2]string{u + ".comparable(" + x + ")", lit + ".comparable(" + x + ")"})
	}
	for _, fn := range []string{"toQuantity()", "lowBoundary()", "highBoundary()"} {
		pairs = append(pairs, [2]string{u + "." + fn, lit + "." + fn})
	}

	// Un error no es una respuesta que comparar: falla aparte (revisión, ronda 1)
	answer := func(t *testing.T, model fhirpath.Model, expr string) string {
		t.Helper()
		opts := []fhirpath.EvalOption{}
		if model != nil {
			opts = append(opts, fhirpath.WithModel(model))
		}
		result, err := fhirpath.MustCompile(expr).EvaluateWithOptions([]byte(unitlessObsFixture), opts...)
		if err != nil {
			t.Errorf("%s: unexpected error %v", expr, err)
			return "error"
		}
		return result.String()
	}
	compare := func(t *testing.T, model fhirpath.Model) {
		for _, p := range pairs {
			fromResource, fromLiteral := answer(t, model, p[0]), answer(t, model, p[1])
			if fromResource != fromLiteral {
				t.Errorf("%s = %s, but %s = %s", strings.ReplaceAll(p[0], "Observation.referenceRange[0].", ""), fromResource, p[1], fromLiteral)
			}
		}
	}
	for _, model := range symmetryModels {
		t.Run(model.name, func(t *testing.T) { compare(t, model.model) })
	}
	t.Run("without a model", func(t *testing.T) { compare(t, nil) })
}
