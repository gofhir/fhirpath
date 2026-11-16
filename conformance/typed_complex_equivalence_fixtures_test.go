package conformance

// Datos de las pruebas de 025: copia de difftest/batteries/typed-complex-equivalence.
const (
	obsFixture           = `{"resourceType":"Observation","id":"o","status":"final","code":{"coding":[{"system":"http://loinc.org","code":"2020-01-01T10:00:00Z"}],"text":"x"},"identifier":[{"system":"urn:s","value":"2020-01-01T10:00:00Z"},{"system":"urn:s","value":"2020-01-01T12:00:00+02:00"},{"system":"urn:s","value":"2020-01-01T10:00:00Z"},{"system":"urn:s","value":"2020-01-01T10:00:00z"},{"system":"urn:s","value":"2020-01-01T10:00:00Z","period":{"start":"2020-01-01T10:00:00Z"}},{"system":"urn:s","value":"2020-01-01T10:00:00Z","period":{"start":"2020-01-01T12:00:00+02:00"}},{"system":"urn:s","value":"x","assigner":{"identifier":{"value":"2020-01-01T10:00:00Z"}}},{"system":"urn:s","value":"x","assigner":{"identifier":{"value":"2020-01-01T12:00:00+02:00"}}},{"system":"urn:s","value":"2020-01-01T10:00:00Z","_value":{"extension":[{"url":"urn:e","valueString":"a"}]}},{"system":"urn:s","value":"2020-01-01T12:00:00+02:00","_value":{"extension":[{"url":"urn:e","valueString":"A"}]}},{"system":"urn:s","value":"2020"},{"system":"urn:s","value":"2020-01"}],"extension":[{"url":"urn:e","valueString":"2020-01-01T10:00:00Z"},{"url":"urn:e","valueString":"2020-01-01T12:00:00+02:00"},{"url":"urn:e","valueDateTime":"2020-01-01T10:00:00Z"},{"url":"urn:e","valueDateTime":"2020-01-01T12:00:00+02:00"}],"note":[{"text":"2020-01-01T10:00:00Z"},{"text":"2020-01-01T12:00:00+02:00"}]}`
	patientFixture       = `{"resourceType":"Patient","id":"p","telecom":[{"system":"phone","value":"2020-01-01T10:00:00Z"},{"system":"phone","value":"2020-01-01T12:00:00+02:00"}],"name":[{"text":"2020-01-01","given":["2020-01-01T10:00:00Z","b"]},{"text":"2020-01-01","given":["b","2020-01-01T12:00:00+02:00"]}],"contained":[{"resourceType":"Basic","id":"b1","code":{"text":"c"},"identifier":[{"value":"2020-01-01T10:00:00Z"}]},{"resourceType":"Basic","id":"b1","code":{"text":"c"},"identifier":[{"value":"2020-01-01T12:00:00+02:00"}]}]}`
	questionnaireFixture = `{"resourceType":"Questionnaire","id":"q","status":"active","item":[{"linkId":"1","type":"group","item":[{"linkId":"1.1","type":"string","text":"2020-01-01T10:00:00Z"}]},{"linkId":"1","type":"group","item":[{"linkId":"1.1","type":"string","text":"2020-01-01T12:00:00+02:00"}]}]}`
	timingFixture        = `{"resourceType":"MedicationRequest","id":"m","status":"active","intent":"order","subject":{"reference":"Patient/p"},"medicationCodeableConcept":{"text":"x"},"dosageInstruction":[{"timing":{"repeat":{"timeOfDay":["10:00:00"]}}},{"timing":{"repeat":{"timeOfDay":["10:00:00.000"]}}},{"timing":{"repeat":{"timeOfDay":["10:00:00"]}}}]}`
	timingR5Fixture      = `{"resourceType":"MedicationRequest","id":"m","status":"active","intent":"order","subject":{"reference":"Patient/p"},"medication":{"concept":{"text":"x"}},"dosageInstruction":[{"timing":{"repeat":{"timeOfDay":["10:00:00"]}}},{"timing":{"repeat":{"timeOfDay":["10:00:00.000"]}}},{"timing":{"repeat":{"timeOfDay":["10:00:00"]}}}]}`
	docrefR5Fixture      = `{"resourceType":"DocumentReference","id":"d","status":"current","content":[{"attachment":{"contentType":"text/plain","size":"1"}},{"attachment":{"contentType":"text/plain","size":"+1"}},{"attachment":{"contentType":"text/plain","size":"-0"}},{"attachment":{"contentType":"text/plain","size":"0"}}]}`
)

// backbonesFixture tiene dos backbones de paths distintos con un mismo hijo
// value escrito igual: dateTime en uno, string en el otro.
const backbonesFixture = `{"resourceType":"Bundle","type":"collection","entry":[
 {"resource":{"resourceType":"ImmunizationRecommendation","patient":{"reference":"Patient/p"},"date":"2020-01-01",
  "recommendation":[{"forecastStatus":{"text":"due"},"dateCriterion":[{"value":"2020"}]}]}},
 {"resource":{"resourceType":"TestScript","url":"urn:t","name":"t","status":"active",
  "setup":{"action":[{"operation":{"requestHeader":[{"value":"2020"}]}}]}}}]}`

// entriesFixture tiene dos entradas cuyos Basic difieren solo en un
// identifier.value con el mismo instante en otro offset.
const entriesFixture = `{"resourceType":"Bundle","type":"collection","entry":[
 {"resource":{"resourceType":"Basic","code":{"text":"c"},"identifier":[{"value":"2020-01-01T10:00:00Z"}]}},
 {"resource":{"resourceType":"Basic","code":{"text":"c"},"identifier":[{"value":"2020-01-01T12:00:00+02:00"}]}}]}`

// mixedFixture pone el mismo assigner bajo Identifier, donde el modelo lo
// conoce, y bajo HumanName, donde no existe.
const mixedFixture = `{"resourceType":"Patient","identifier":[{"assigner":{"identifier":{"value":"2020"}}}],"name":[{"assigner":{"identifier":{"value":"2020"}}}]}`

// mixedQuantityFixture pone la misma Quantity bajo referenceRange.low, donde el
// modelo la conoce, y bajo component.low, que no existe.
const mixedQuantityFixture = `{"resourceType":"Observation","status":"final","code":{"text":"c"},` +
	`"referenceRange":[{"low":{"value":1,"unit":"mg","system":"http://unitsofmeasure.org","code":"mg"}}],` +
	`"component":[{"low":{"value":1000,"unit":"ug","system":"http://unitsofmeasure.org","code":"ug"}}]}`

// mixedExtensionFixture pone la misma extensión dentro de un assigner que el
// modelo conoce (Identifier) y de uno que no existe (HumanName).
const mixedExtensionFixture = `{"resourceType":"Patient","identifier":[{"assigner":{"extension":[{"url":"urn:e","valueString":"2020"}]}}],` +
	`"name":[{"assigner":{"extension":[{"url":"urn:e","valueString":"2020"}]}}]}`

// mixedDecimalsFixture pone dos listas de decimales bajo low.value, conocida en
// referenceRange y desconocida en component; solo un emparejamiento que
// reconsidera candidatos las aparea.
const mixedDecimalsFixture = `{"resourceType":"Observation","status":"final","code":{"text":"x"},` +
	`"referenceRange":[{"low":{"value":[1.0,1.1]}}],"component":[{"code":{"text":"x"},"low":{"value":[1.1,1.2]}}]}`
