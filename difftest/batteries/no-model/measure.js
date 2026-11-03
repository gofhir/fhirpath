// Measures what fhirpath.js answers for type questions with no model, and
// with the R4 and R5 models, over patient.json. The difftest harness always
// supplies a model to its battery, so the no-model run is measured here.
//
// Usage, from the repository root (after make three-engine has installed
// fhirpath.js): node difftest/batteries/no-model/measure.js
const path = require('path');
const fs = require('fs');
const modules = path.join(__dirname, '..', '..', 'node_modules');
const fhirpath = require(path.join(modules, 'fhirpath'));
const r4 = require(path.join(modules, 'fhirpath/fhir-context/r4'));
const r5 = require(path.join(modules, 'fhirpath/fhir-context/r5'));
const doc = JSON.parse(fs.readFileSync(path.join(__dirname, 'patient.json'), 'utf8'));
const expressions = [
  'Patient.telecom.rank is Integer', 'Patient.gender is String', 'Patient.gender.type().name',
  'Patient.telecom.rank.type().name', 'Patient.gender is code', 'Patient.telecom.value is code',
  'Patient.telecom.rank is positiveInt', 'Patient.descendants().ofType(integer64)',
];
const run = (expression, model) => {
  try {
    return JSON.stringify(fhirpath.evaluate(doc, expression, null, model));
  } catch (err) {
    return 'error: ' + String(err.message).slice(0, 70);
  }
};
for (const expression of expressions) {
  console.log([expression, 'none=' + run(expression), 'r4=' + run(expression, r4), 'r5=' + run(expression, r5)].join('\t'));
}
