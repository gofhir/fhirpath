package conformance

import (
	"strings"
	"testing"

	"github.com/gofhir/fhirpath"
)

// Plan 026 (FP-27): un Quantity de FHIR sin code ni unit es una cantidad de
// unidad '1', como un número. Los valores esperados salen de FHIRPath 3.0.0,
// Conversion ("the resulting quantity will have the UCUM default unit '1'";
// "23 = 23 '1'" es true), de la página FHIRPath de FHIR (el mapeo), de la
// decisión del usuario (plan §2 y §3) o de un motor de referencia. Ninguno
// sale de correr este motor.

type unitlessCase struct {
	name     string
	resource string
	expr     string // con {u}, {ua}, ... en lugar de cada lectura del recurso
	want     string // "true", "false" o "" para vacío
	only     string // "" para R4, R4B y R5; "r4" solo R4 y R4B; "r5" solo R5
	noModel  bool   // también se evalúa sin modelo
	note     string
}

const (
	rngR4 = `Observation.valueRange.all(low.empty() or high.empty() or (low <= high))`
	rngR5 = `Observation.valueRange.all(low.value.empty() or high.value.empty() or low.lowBoundary().comparable(high.highBoundary()).not() or (low.lowBoundary() <= high.highBoundary()))`
)

var unitlessReader = strings.NewReplacer(
	"{u}", "Observation.referenceRange[0].low",
	"{h}", "Observation.referenceRange[0].high",
	"{ua}", "Observation.referenceRange[1].low",
	"{ub}", "Observation.referenceRange[2].low",
	"{mg}", "Observation.referenceRange[3].low",
	"{sys}", "Observation.referenceRange[4].low",
	"{eu}", "Observation.referenceRange[5].low",
	"{ec}", "Observation.referenceRange[6].low",
	"{cal}", "Observation.referenceRange[7].low",
	"{cmp}", "Observation.referenceRange[8].low",
	"{big}", "Observation.referenceRange[9].low",
)

func unitlessCases(version string) []unitlessCase {
	obs := unitlessObsFixture
	rng := rngR4
	if version == "r5" {
		rng = rngR5
	}
	conv := "FHIRPath 3.0.0 Conversion: the resulting quantity has the UCUM default unit '1'"
	cases := []unitlessCase{
		// Lo que cambia (plan §3)
		{"u = 1 mg", obs, "{u} = 1 'mg'", "", "", true, "plan §3: as 1 '1' = 1 'mg' (different units, empty)"},
		{"u ~ 1 mg", obs, "{u} ~ 1 'mg'", "false", "", true, "plan §3: as 1 '1' ~ 1 'mg'"},
		{"u < 2 mg", obs, "{u} < 2 'mg'", "", "", true, "plan §3"},
		{"u + 1 mg", obs, "{u} + 1 'mg'", "", "", true, "plan §3"},
		{"u comparable 1 mg", obs, "{u}.comparable(1 'mg')", "false", "", true, "plan §3; fhirpath.js and Java say false"},
		{"valueQuantity = 1 mg", obs, "Observation.valueQuantity = 1 'mg'", "", "", true, "plan §3, a value of type Quantity"},
		{"u toString", obs, "{u}.toString() = '1 \\'1\\''", "true", "", true, conv},
		{"u toQuantity", obs, "{u}.toQuantity() = 1 '1'", "true", "", true, conv},
		{"u + h", obs, "({u} + {h}) = 3 '1'", "true", "", true, conv},
		{"u + h toString", obs, "({u} + {h}).toString() = '3 \\'1\\''", "true", "", true, conv},
		{"u lowBoundary", obs, "{u}.lowBoundary() = 0.5 '1'", "true", "", true, "plan §3 table; boundary of 1 is 0.5"},
		{"u highBoundary", obs, "{u}.highBoundary() = 1.5 '1'", "true", "", true, "plan §3 table; boundary of 1 is 1.5"},

		// Aritmética con valores esperados propios (revisión, ronda 1): '1'
		// escala como un número, "apart from the UCUM default unit '1'"
		{"u * 2 '1'", obs, "({u} * 2 '1').toString() = '2 \\'1\\''", "true", "", true, conv},
		{"u / 2 '1'", obs, "({u} / 2 '1').toString() = '0.5 \\'1\\''", "true", "", true, conv},
		{"u * 2", obs, "({u} * 2).toString() = '2 \\'1\\''", "true", "", true, conv},
		{"u * 1 mg", obs, "({u} * 1 'mg').toString() = '1 \\'mg\\''", "true", "", true, conv},
		{"u - h", obs, "({u} - {h}).toString() = '-1 \\'1\\''", "true", "", true, conv},
		// Un campo que el modelo no conoce, leído por su forma (plan §3)
		{"unknown field toQuantity", unitlessUnknownFixture, "Observation.x.toQuantity().toString() = '1 \\'1\\''", "true", "", true, conv},
		{"unknown field lowBoundary", unitlessUnknownFixture, "Observation.x.lowBoundary() = 0.5 '1'", "true", "", true, conv},
		{"valueQuantity toQuantity", obs, "Observation.valueQuantity.toQuantity().toString() = '1 \\'1\\''", "true", "", true, conv},

		// Sin cambio (plan §3)
		{"u = 1", obs, "{u} = 1", "true", "", true, "FHIRPath 3.0.0 Conversion: 23 = 23 '1' is true"},
		{"u ~ 1", obs, "{u} ~ 1", "true", "", true, "FHIRPath 3.0.0 Conversion: 23 ~ 23 '1' is true"},
		{"u = 1 '1'", obs, "{u} = 1 '1'", "true", "", true, conv},
		{"ua ~ ub (FP-27)", obs, "{ua} ~ {ub}", "true", "", true, "deliberate: compared as quantities, as with a unit (Java agrees); fhirpath.js false"},
		{"ua = ub", obs, "{ua} = {ub}", "true", "", true, "plan §3, unchanged"},
		{"u comparable h", obs, "{u}.comparable({h})", "true", "", true, "deliberate divergence, plan §3; rng-2 needs it"},
		{"u <= h", obs, "{u} <= {h}", "true", "", true, "plan §3 table, unchanged"},
		{"sys comparable 1 mg", obs, "{sys}.comparable(1 'mg')", "false", "", true, "plan §3: system alone is unitless, '1'"},
		// Casos por texto: la igualdad de arriba ya pasaba con la unidad
		// vacía, que toma la del otro operando; el texto fija la unidad
		{"u toQuantity toString", obs, "{u}.toQuantity().toString() = '1 \\'1\\''", "true", "", true, conv},
		{"u lowBoundary unit", obs, "{u}.lowBoundary().toString().endsWith(' \\'1\\'')", "true", "", true, conv},
		{"u highBoundary unit", obs, "{u}.highBoundary().toString().endsWith(' \\'1\\'')", "true", "", true, conv},
		{"sys toString", obs, "{sys}.toString() = '1 \\'1\\''", "true", "", true, "plan §3: system alone is '1'"},
		{"eu toString", obs, "{eu}.toString() = '1 \\'1\\''", "true", "", true, "plan §3: unit \"\" is absent"},
		{"sys = 1 '1'", obs, "{sys} = 1 '1'", "true", "", true, "plan §3: system alone is '1'"},
		{"eu = 1 '1'", obs, "{eu} = 1 '1'", "true", "", true, "plan §3: unit \"\" is absent"},
		{"ec = 1 mg", obs, "{ec} = 1 'mg'", "true", "", true, "plan §3: code \"\" is absent, the unit mg is read"},
		{"ec toString", obs, "{ec}.toString() = '1 \\'mg\\''", "true", "", true, "plan §3"},
		{"cal toString", obs, "{cal}.toString() = '1 year'", "true", "", true, "FHIR mapping: UCUM code a is the calendar year"},
		{"money comparable 1", unitlessClaimFixture, "Claim.total.comparable(1 '1')", "false", "", false, "plan §3: Money stays no quantity"},
		{"big = 1", obs, "{big} = 1", "", "", true, "A7: a value beyond the decimal bound has no quantity"},
		{"big toQuantity", obs, "{big}.toQuantity()", "", "", true, "A7"},
		{"cmp = 1 '1'", obs, "{cmp} = 1 '1'", "true", "", true, "CONFORMANCE.md: a quantity's comparator is not read"},

		// Perfiles (A6)
		{"Age profile", unitlessConditionFixture, "Condition.onsetAge = 1 '1'", "true", "", true, "plan A6: Age profiles Quantity"},
		{"Ratio denominator", unitlessRatioFixture, "Observation.valueRatio.denominator = 1 '1'", "true", "", true, "plan A6: SimpleQuantity in R5, Quantity in R4"},
		{"Range low", unitlessRangeAscFixture, "Observation.valueRange.low = 1 '1'", "true", "", true, "plan A6: SimpleQuantity"},

		// Fuera de 026: no cambia (FP-28, plan 025 C2)
		{"ua in ub", obs, "{ua} in {ub}", "false", "", true, "pin FP-28: membership compares the objects, id included"},
		{"distinct of ua and ub", obs, "{ua}.combine({ub}).distinct().count() = 2", "true", "", true, "pin FP-28"},
		{"nested ranges ~", obs, "Observation.referenceRange[1] ~ Observation.referenceRange[2]", "false", "", true, "pin plan 025 C2: nested Quantities by shape, id counts"},
		{"nested ranges =", obs, "Observation.referenceRange[1] = Observation.referenceRange[2]", "false", "", true, "pin plan 025 C2"},

		// rng-2, el texto de cada versión (criterio 3)
		{"rng-2 ascending", unitlessRangeAscFixture, rng, "true", "", true, "plan §3 criterion 3"},
		{"rng-2 descending touching", unitlessRangeDescFixture, rng, "false", "r4", true, "criterion 3: R4 and R4B low <= high fails"},
		{"rng-2 descending touching R5", unitlessRangeDescFixture, rng, "true", "r5", true, "criterion 3: R5 boundaries 1.5 and 1.5 overlap"},
		{"rng-2 descending disjoint", unitlessRangeFarFixture, rng, "false", "", true, "criterion 3: boundaries 2.5 and 1.5, comparable and out of order"},
	}
	var out []unitlessCase
	for _, c := range cases {
		if c.only == "" || (c.only == "r5") == (version == "r5") {
			c.expr = unitlessReader.Replace(c.expr)
			out = append(out, c)
		}
	}
	return out
}

func unitlessWant(want string) string {
	if want == "" {
		return "[]"
	}
	return "[" + want + "]"
}

func TestUnitlessQuantity(t *testing.T) {
	for _, model := range symmetryModels {
		for _, tt := range unitlessCases(model.name) {
			t.Run(model.name+"/"+tt.name, func(t *testing.T) {
				if got, want := evaluateWith(t, model.model, tt.resource, tt.expr), unitlessWant(tt.want); got != want {
					t.Errorf("%s = %s, want %s (%s)", tt.expr, got, want, tt.note)
				}
			})
		}
	}
}

func TestUnitlessQuantityWithoutAModel(t *testing.T) {
	// Sin modelo no hay versión, pero rng-2 se escribe distinto en R4 y R5:
	// corren los casos de las dos (revisión, ronda 2)
	for _, version := range []string{"r4", "r5"} {
		for _, tt := range unitlessCases(version) {
			if !tt.noModel {
				continue
			}
			t.Run(version+"/"+tt.name, func(t *testing.T) {
				result, err := fhirpath.MustCompile(tt.expr).Evaluate([]byte(tt.resource))
				if err != nil {
					t.Fatal(err)
				}
				if got, want := result.String(), unitlessWant(tt.want); got != want {
					t.Errorf("without a model %s = %s, want %s (%s)", tt.expr, got, want, tt.note)
				}
			})
		}
	}
}
