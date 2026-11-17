package conformance

// Datos de las pruebas de 026: copia de difftest/batteries/unitless-quantity.
// En unitlessObsFixture, referenceRange[i].low es, en orden: u, ua, ub, mg,
// sys, eu, ec, cal, cmp, big; referenceRange[0].high es h.
const (
	unitlessObsFixture = `{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueQuantity":{"value":1},"referenceRange":[` +
		`{"low":{"value":1},"high":{"value":2}},` +
		`{"low":{"value":1,"id":"a"}},` +
		`{"low":{"value":1,"id":"b"}},` +
		`{"low":{"value":1,"unit":"mg"}},` +
		`{"low":{"value":1,"system":"http://unitsofmeasure.org"}},` +
		`{"low":{"value":1,"unit":""}},` +
		`{"low":{"value":1,"code":"","unit":"mg"}},` +
		`{"low":{"value":1,"system":"http://unitsofmeasure.org","code":"a"}},` +
		`{"low":{"value":1,"comparator":"<"}},` +
		`{"low":{"value":1e1001}}]}`
	unitlessClaimFixture     = `{"resourceType":"Claim","status":"active","type":{"text":"c"},"use":"claim","patient":{"reference":"Patient/p"},"created":"2020-01-01","provider":{"reference":"Practitioner/p"},"priority":{"text":"c"},"insurance":[{"sequence":1,"focal":true,"coverage":{"reference":"Coverage/c"}}],"total":{"value":1,"currency":"USD"}}`
	unitlessConditionFixture = `{"resourceType":"Condition","subject":{"reference":"Patient/p"},"onsetAge":{"value":1}}`
	unitlessRatioFixture     = `{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueRatio":{"numerator":{"value":1},"denominator":{"value":1}}}`
	unitlessRangeAscFixture  = `{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueRange":{"low":{"value":1},"high":{"value":2}}}`
	unitlessRangeDescFixture = `{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueRange":{"low":{"value":2},"high":{"value":1}}}`
	unitlessRangeFarFixture  = `{"resourceType":"Observation","status":"final","code":{"text":"c"},"valueRange":{"low":{"value":3},"high":{"value":1}}}`
)

// unitlessUnknownFixture pone una Quantity sin unidad en un campo que el modelo
// no conoce.
const unitlessUnknownFixture = `{"resourceType":"Observation","status":"final","code":{"text":"c"},"x":{"value":1}}`
