package conformance

import (
	"testing"

	"github.com/gofhir/fhirpath"
	"github.com/gofhir/fhirpath/eval"
)

// Plan 025 (#263): con un modelo, `~` entre dos objetos lee cada hijo como lo
// tipa el modelo, igual que la navegación. Los valores esperados vienen de
// fhirpath.js 5.2.0 y del validador HL7 (Java 6.10.2), medidos en
// difftest/batteries/typed-complex-equivalence (battery.tsv), o del contrato
// del plan §2. Ninguno sale de correr este motor. N1 §6.1.3: "For complex
// types, equivalence requires all child properties to be equivalent,
// recursively"; una cadena (Identifier.value, ContactPoint.value, ...) se
// compara como texto, no como instante.

type typedEquivalenceCase struct {
	name     string
	resource string // el JSON del recurso
	expr     string
	want     string // "true" o "false"
	only     string // "" para R4, R4B y R5; "r5" solo para R5
	note     string // de dónde sale el valor esperado
}

// timingResource es el recurso de Timing de la versión: R5 cambia la medicación.
func timingResource(version string) string {
	if version == "r5" {
		return timingR5Fixture
	}
	return timingFixture
}

func typedEquivalenceCases(version string) []typedEquivalenceCase {
	obs, patient, questionnaire := obsFixture, patientFixture, questionnaireFixture
	timing := timingResource(version)
	quantities := `{"resourceType":"Observation","status":"final","code":{"text":"c"},"referenceRange":[` +
		`{"low":{"value":1,"id":"a"}},{"low":{"value":1,"id":"b"}},` +
		`{"low":{"value":1,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg","id":"a"}},` +
		`{"low":{"value":1,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg","id":"b"}}]}`
	unknown := `{"resourceType":"Basic","code":{"text":"c"},"extension":[` +
		`{"url":"urn:e","foo":{"bar":"2020-01-01T10:00:00Z"}},` +
		`{"url":"urn:e","foo":{"bar":"2020-01-01T12:00:00+02:00"}},` +
		`{"url":"urn:e","foo":"2020-01-01T10:00:00Z"},` +
		`{"url":"urn:e","foo":"2020-01-01T12:00:00+02:00"}]}`
	cases := []typedEquivalenceCase{
		// battery.tsv: obs.json. Identifier.value es string en R4, R4B y R5.
		{"identifier 0 against 1: the same instant, another offset", obs, "Observation.identifier[0] ~ Observation.identifier[1]", "false", "", "battery: fhirpath.js and Java false (#263)"},
		{"identifier 2 against 1", obs, "Observation.identifier[2] ~ Observation.identifier[1]", "false", "", "battery"},
		{"identifier 0 against 2: the same text", obs, "Observation.identifier[0] ~ Observation.identifier[2]", "true", "", "battery"},
		{"identifier 0 against 3: Z against z, strings case aside", obs, "Observation.identifier[0] ~ Observation.identifier[3]", "true", "", "battery; N1 §6.1.3 string equivalence ignores case"},
		{"identifier 4 against 5: Period.start is a dateTime", obs, "Observation.identifier[4] ~ Observation.identifier[5]", "true", "", "battery: dateTime child kept (plan 021), Java true"},
		{"identifier 6 against 7: assigner.identifier.value nested", obs, "Observation.identifier[6] ~ Observation.identifier[7]", "false", "", "battery"},
		{"identifier 8 against 9: value with _value", obs, "Observation.identifier[8] ~ Observation.identifier[9]", "false", "", "battery"},
		{"identifier 10 against 11: 2020 against 2020-01", obs, "Observation.identifier[10] ~ Observation.identifier[11]", "false", "", "battery"},
		{"identifier value 0 against 1", obs, "Observation.identifier[0].value ~ Observation.identifier[1].value", "false", "", "battery"},
		{"identifier 4 period against 5 period", obs, "Observation.identifier[4].period ~ Observation.identifier[5].period", "true", "", "battery"},
		{"extension 0 against 1: valueString", obs, "Observation.extension[0] ~ Observation.extension[1]", "false", "", "battery"},
		{"extension 2 against 3: valueDateTime", obs, "Observation.extension[2] ~ Observation.extension[3]", "true", "", "battery: dateTime child, Java true"},
		{"extension 0 against 2: valueString against valueDateTime", obs, "Observation.extension[0] ~ Observation.extension[2]", "false", "", "battery"},
		{"note 0 against 1: Annotation.text", obs, "Observation.note[0] ~ Observation.note[1]", "false", "", "battery"},
		{"identifier against coding", obs, "Observation.identifier[0] ~ Observation.code.coding[0]", "false", "", "battery"},
		{"!~ over identifiers", obs, "Observation.identifier[0] !~ Observation.identifier[1]", "true", "", "battery"},
		{"a collection of identifiers", obs, "Observation.identifier.take(2) ~ Observation.identifier.skip(1).take(1) | Observation.identifier.take(1)", "true", "", "battery"},
		{"children of two identifiers", obs, "Observation.identifier[0].children() ~ Observation.identifier[1].children()", "false", "", "battery"},
		// battery.tsv: patient.json
		{"telecom 0 against 1: ContactPoint.value", patient, "Patient.telecom[0] ~ Patient.telecom[1]", "false", "", "battery"},
		{"name 0 against 1: HumanName.given", patient, "Patient.name[0] ~ Patient.name[1]", "false", "", "battery"},
		{"contained 0 against 1: typed by resourceType", patient, "Patient.contained[0] ~ Patient.contained[1]", "false", "", "battery"},
		// battery.tsv: questionnaire.json (R4B usa los de R4: el mismo recurso)
		{"item 0 against 1: a recursive backbone", questionnaire, "Questionnaire.item[0] ~ Questionnaire.item[1]", "false", "", "battery"},
		{"item.item 0 against 1", questionnaire, "Questionnaire.item[0].item[0] ~ Questionnaire.item[1].item[0]", "false", "", "battery"},
		// battery.tsv: timing. Decisión del usuario: Time se compara como Time.
		{"dosage 0 against 1: timeOfDay 10:00:00 against 10:00:00.000", timing, "MedicationRequest.dosageInstruction[0] ~ MedicationRequest.dosageInstruction[1]", "true", "", "deliberate (user decision): N1 §6.1.3 child equivalence, Time; fhirpath.js says timeOfDay ~ timeOfDay true; Java errors"},
		{"dosage 0 against 2: the same text", timing, "MedicationRequest.dosageInstruction[0] ~ MedicationRequest.dosageInstruction[2]", "true", "", "battery"},
		{"timeOfDay against timeOfDay", timing, "MedicationRequest.dosageInstruction[0].timing.repeat.timeOfDay ~ MedicationRequest.dosageInstruction[1].timing.repeat.timeOfDay", "true", "", "battery: fhirpath.js true, Java errors"},
		// battery.tsv: docref-r5.json. Solo R5: Attachment.size es integer64.
		{"content 0 against 1: integer64 1 against +1", docrefR5Fixture, "DocumentReference.content[0] ~ DocumentReference.content[1]", "true", "r5", "deliberate: integer64 is read as an integer (plan 016 D2); Java reads it as text and says false"},
		{"content 2 against 3: -0 against 0", docrefR5Fixture, "DocumentReference.content[2] ~ DocumentReference.content[3]", "true", "r5", "parser tolerance: \"-0\" is not a valid integer64 (FHIR regex [0]|[-+]?[1-9][0-9]*); the engine reads it as 0"},
		{"content 0 against 3: 1 against 0", docrefR5Fixture, "DocumentReference.content[0] ~ DocumentReference.content[3]", "false", "r5", "battery"},
		{"size against size", docrefR5Fixture, "DocumentReference.content[0].attachment.size ~ DocumentReference.content[1].attachment.size", "true", "r5", "battery: fhirpath.js true, Java false (text)"},

		// Quantity anidada: no cambia en 025 (plan §2, C2).
		{"nested unitless quantity with another id", quantities, "Observation.referenceRange[0] ~ Observation.referenceRange[1]", "false", "", "plan §2 C2 (unchanged): fhirpath.js false, Java errors; not a quantity without a unit, so id counts"},
		{"nested quantity with a unit and another id", quantities, "Observation.referenceRange[2] ~ Observation.referenceRange[3]", "true", "", "plan §2 C2 (unchanged): compares as a quantity, as Java answers"},

		// Objetos que vienen de funciones: la navegación les da su tipo y su lugar.
		{"objects from select($this)", obs, "Observation.identifier.select($this)[0] ~ Observation.identifier.select($this)[1]", "false", "", "same objects as identifier[0] and [1]: N1 §6.1.3, value is a string"},
		{"objects from where(true)", obs, "Observation.identifier.where(true)[0] ~ Observation.identifier.where(true)[1]", "false", "", "same objects as identifier[0] and [1]"},
		{"objects from children().ofType(Identifier)", obs, "Observation.children().ofType(Identifier)[0] ~ Observation.children().ofType(Identifier)[1]", "false", "", "children() types each child through the model; they are identifier[0] and [1]"},
		{"objects from descendants().ofType(Identifier)", obs, "Observation.descendants().ofType(Identifier)[0] ~ Observation.descendants().ofType(Identifier)[1]", "false", "", "descendants() reaches identifier[0] and [1] in order"},

		// Dos backbones en paths distintos tipan el mismo nombre distinto: en
		// ImmunizationRecommendation.recommendation.dateCriterion, value es
		// dateTime; en TestScript.setup.action.operation.requestHeader, string
		// (R4, R4B y R5). Cada lado se tipa bajo su propio path, y un DateTime no
		// es equivalente a un String (N1 §6.1.3; '2020' ~ @2020 es false)
		{"backbones at two paths, the same text under two types", backbonesFixture, "Bundle.entry[0].resource.recommendation[0].dateCriterion[0] ~ Bundle.entry[1].resource.setup.action[0].operation.requestHeader[0]", "false", "", "each side typed under its own path: dateTime against string. Deliberate: fhirpath.js and Java answer true here, while fhirpath.js answers the two values themselves false (plan 025 §7)"},
		{"backbones at two paths, swapped", backbonesFixture, "Bundle.entry[1].resource.setup.action[0].operation.requestHeader[0] ~ Bundle.entry[0].resource.recommendation[0].dateCriterion[0]", "false", "", "string against dateTime"},

		// Un recurso dentro de Bundle.entry.resource (del tipo Resource, que no se instancia) se
		// tipa por su resourceType: los identifier de dos Basic con el mismo
		// instante en otro offset son cadenas distintas (N1 §6.1.3; la batería,
		// contained[0] ~ contained[1], false en fhirpath.js y Java)
		{"entries whose resources differ in a string child", entriesFixture, "Bundle.entry[0] ~ Bundle.entry[1]", "false", "", "Bundle.entry.resource is typed by resourceType: Basic.identifier.value is a string"},

		// Un lado conocido contra uno desconocido: HumanName.assigner no existe,
		// así que a la derecha "2020" se lee por su forma (una fecha), y a la
		// izquierda Identifier.assigner.identifier.value es string. Cada lado
		// conserva su tipado (plan §2), como lo leen sus hijos navegados
		// (code review 025, ronda 1)
		{"a known side against an unknown one", mixedFixture, "Patient.identifier[0] ~ Patient.name[0]", "false", "", "plan §2: each side typed on its own; String against Date"},
		{"an unknown side against a known one", mixedFixture, "Patient.name[0] ~ Patient.identifier[0]", "false", "", "the same, swapped"},
		// Una Quantity con unidad conocida contra la misma forma en una ruta
		// desconocida: se reconoce por su forma, como sin modelo (plan §2, C2;
		// main responde true, 1 mg ~ 1000 ug)
		{"a known nested quantity against an unknown one", mixedQuantityFixture, "Observation.referenceRange[0] ~ Observation.component[0]", "true", "", "plan §2 C2: nested Quantities told by shape, unchanged from main"},
		{"an unknown nested quantity against a known one", mixedQuantityFixture, "Observation.component[0] ~ Observation.referenceRange[0]", "true", "", "the same, swapped"},
		{"the values themselves", mixedFixture, "Patient.identifier[0].assigner.identifier.value ~ Patient.name[0].assigner.identifier.value", "false", "", "navigation types the left as string and reads the right by its shape"},
		// Ronda 2 del review: una extensión bajo un elemento desconocido se tipa
		// como en la navegación (fhirpath.js true; Java descarta el elemento al
		// parsear). Decimales que solo un emparejamiento óptimo aparea: false en
		// fhirpath.js y Java (batería, decimals.json); main respondía true.
		{"an extension under an unknown element", mixedExtensionFixture, "Patient.identifier[0] ~ Patient.name[0]", "true", "", "navigation types the extension on both sides; fhirpath.js agrees"},
		{"an extension under an unknown element, swapped", mixedExtensionFixture, "Patient.name[0] ~ Patient.identifier[0]", "true", "", "the same, swapped"},
		{"decimal lists a greedy pairing does not match", mixedDecimalsFixture, "Observation.referenceRange[0].low ~ Observation.component[0].low", "false", "", "fhirpath.js and Java answer false"},
		{"decimal lists a greedy pairing does not match, swapped", mixedDecimalsFixture, "Observation.component[0].low ~ Observation.referenceRange[0].low", "false", "", "the same, swapped"},

		// Una ruta que el modelo no conoce: los hijos se leen por su forma
		// (contrato del plan §2). La clave desconocida es la única diferencia.
		{"a path the model does not know, nested", unknown, "Basic.extension[0] ~ Basic.extension[1]", "true", "", "plan §2 contract: a child whose path the model does not know is read by its shape (same instant)"},
		{"a path the model does not know, direct", unknown, "Basic.extension[2] ~ Basic.extension[3]", "true", "", "plan §2 contract: read by shape"},
	}

	return filterTypedCases(cases, version)
}

func filterTypedCases(cases []typedEquivalenceCase, version string) []typedEquivalenceCase {
	out := cases[:0:0]
	for _, c := range cases {
		if c.only == "" || c.only == version {
			out = append(out, c)
		}
	}
	return out
}

func TestTypedComplexEquivalence(t *testing.T) {
	for _, model := range symmetryModels {
		for _, tt := range typedEquivalenceCases(model.name) {
			t.Run(model.name+"/"+tt.name, func(t *testing.T) {
				if got, want := evaluateWith(t, model.model, tt.resource, tt.expr), "["+tt.want+"]"; got != want {
					t.Errorf("%s = %s, want %s (%s)", tt.expr, got, want, tt.note)
				}
			})
		}
	}
}

// Sin modelo, `~` no cambia: cada hijo se lee por su forma, como children()
// (plan §2). Un identifier con la misma hora en otro desfase sigue equivalente.
func TestTypedComplexEquivalenceWithoutAModel(t *testing.T) {
	tests := []struct {
		expr, want string
	}{
		{"Observation.identifier[0] ~ Observation.identifier[1]", "[true]"},
		{"Observation.identifier[0] ~ Observation.identifier[2]", "[true]"},
		{"Observation.identifier[0].children() ~ Observation.identifier[1].children()", "[true]"},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			result, err := fhirpath.MustCompile(tt.expr).Evaluate([]byte(obsFixture))
			if err != nil {
				t.Fatal(err)
			}
			if got := result.String(); got != tt.want {
				t.Errorf("without a model %s = %s, want %s", tt.expr, got, tt.want)
			}
		})
	}
}

// Un HumanName suelto, con la raíz colocada en Patient.name (SetPath): sus
// extension se tipan por el modelo igual que dentro de un Patient. Se
// comparan dos extension de la raíz; un HumanName con `given` no se puede
// comparar con otro con una sola raíz, así que el par va en extension.
func TestTypedComplexEquivalenceRootPlacedWithAPath(t *testing.T) {
	root := `{"text":"n","extension":[` +
		`{"url":"urn:e","valueString":"2020-01-01T10:00:00Z"},` +
		`{"url":"urn:e","valueString":"2020-01-01T12:00:00+02:00"},` +
		`{"url":"urn:e","valueDateTime":"2020-01-01T10:00:00Z"},` +
		`{"url":"urn:e","valueDateTime":"2020-01-01T12:00:00+02:00"}]}`
	tests := []struct {
		expr, want, why string
	}{
		{"extension[0] ~ extension[1]", "[false]", "valueString is a string: N1 §6.1.3"},
		{"extension[2] ~ extension[3]", "[true]", "valueDateTime is a dateTime"},
	}
	for _, model := range symmetryModels {
		for _, tt := range tests {
			t.Run(model.name+"/"+tt.expr, func(t *testing.T) {
				ctx := eval.NewContext([]byte(root))
				ctx.SetModel(model.model)
				ctx.SetPath("Patient.name")
				result, err := fhirpath.MustCompile(tt.expr).EvaluateWithContext(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if got := result.String(); got != tt.want {
					t.Errorf("%s at Patient.name = %s, want %s (%s)", tt.expr, got, tt.want, tt.why)
				}
			})
		}
	}
}
