// Mide fhirpath.js en las cargas de BenchmarkTypedEquivalence de este motor
// (conformance/typed_equivalence_bench_test.go, plan 025): la misma expresión
// `~` entre objetos sobre los mismos recursos, con los contextos R4 y R5.
// Es una referencia de un rival para el plan, no un test.
//
// Uso: node equivalence_timing.js [ms-mínimos-por-caso]   (por omisión 500)
//
// Escribe una línea por carga y versión: iteraciones, mediana de milisegundos
// por evaluación y el resultado. El tiempo de compilación queda fuera; cada
// evaluación corre sobre el recurso ya leído.

const fhirpath = require('fhirpath');
const contexts = { R4: require('fhirpath/fhir-context/r4'), R5: require('fhirpath/fhir-context/r5') };
const minimumMs = Number(process.argv[2] || 500);

const range = (n, f) => Array.from({ length: n }, (_, i) => f(i));
const reversedRange = (n, f) => range(n, (i) => f(n - 1 - i));
const pad = (i) => String(i).padStart(2, '0');

const identifier = (value, extra = {}) => ({ system: 'urn:s', value, ...extra });
const observation = (fields) => ({ resourceType: 'Observation', status: 'final', code: { text: 'c' }, ...fields });

const workloads = {
  digits: {
    expr: 'Observation.identifier[0] ~ Observation.identifier[1]',
    build: () => {
      const make = (value) => ({
        id: 'i', use: 'official', type: { text: 't' }, system: 'urn:s', value,
        period: { start: '2020-01-01', end: '2020-12-31' },
        assigner: { display: 'a', reference: 'Organization/1' },
      });
      return observation({ identifier: [make('2020-01-01T10:00:00+01:00'), make('2020-01-01T09:00:00Z')] });
    },
  },
  integer64: {
    expr: 'DocumentReference.content[0].attachment ~ DocumentReference.content[1].attachment',
    build: (version) => {
      // En R4 size es un número; en R5 es integer64, una cadena JSON.
      const [first, second] = version === 'R5' ? ['+1', '1'] : [1, 1];
      const content = (size) => ({ attachment: { contentType: 'text/plain', size, title: 't' } });
      return { resourceType: 'DocumentReference', status: 'current', content: [content(first), content(second)] };
    },
  },
  escaped: {
    expr: 'Observation.identifier[0] ~ Observation.identifier[1]',
    // JSON.parse lee los escapes - como '-', igual que el motor de Go.
    build: () => JSON.parse(
      '{"resourceType":"Observation","status":"final","code":{"text":"c"},"identifier":[' +
      '{"system":"urn:s","value":"2020\\u002d01\\u002d01"},{"system":"urn:s","value":"2020\\u002d01\\u002d02"}]}'),
  },
  nested: {
    expr: 'Observation.identifier[0] ~ Observation.identifier[1]',
    build: () => {
      const make = (value) => ({ value: 'x', assigner: { identifier: { value: 'y', assigner: { identifier: { value } } } } });
      return observation({ identifier: [make('deep-a'), make('deep-b')] });
    },
  },
  elements: {
    expr: 'Patient.name[0] ~ Patient.name[1]',
    build: () => ({
      resourceType: 'Patient',
      name: [
        { _given: [{ id: 'g0' }, { extension: [{ url: 'u', valueString: 'x' }] }, { id: 'g2' }, null], given: ['a', 'b', 'c', 'd'], family: 'f' },
        { family: 'f', _given: [null, { id: 'g2' }, { extension: [{ url: 'u', valueString: 'x' }] }, { id: 'g0' }], given: ['d', 'c', 'b', 'a'] },
      ],
    }),
  },
  repeats: {
    expr: 'Patient.name[0] ~ Patient.name[1]',
    build: () => ({
      resourceType: 'Patient',
      name: [
        { family: 'f', given: range(40, (i) => `g${pad(i)}`) },
        { family: 'f', given: reversedRange(40, (i) => `g${pad(i)}`) },
      ],
    }),
  },
  identical: {
    expr: 'Patient.extension[0] ~ Patient.extension[1]',
    build: () => {
      const make = (last) => {
        const o = { url: 'u' };
        range(40, (i) => { o[`p${pad(i)}`] = `v${pad(i)}`; });
        o.p99 = last;
        return o;
      };
      return { resourceType: 'Patient', extension: [make('a'), make('b')] };
    },
  },
  lateElement: {
    expr: 'Patient.extension[0] ~ Patient.extension[1]',
    build: () => {
      const make = (id) => {
        const o = { url: 'u' };
        range(30, (i) => { o[`c${pad(i)}`] = `v${pad(i)}`; });
        o._x = { id };
        return o;
      };
      return { resourceType: 'Patient', extension: [make('1'), make('2')] };
    },
  },
  reversedObjects: {
    expr: 'Bundle.entry[0].resource.contact ~ Bundle.entry[1].resource.contact',
    build: () => {
      const patient = (list) => ({ resource: { resourceType: 'Patient', contact: list((i) => ({ name: { text: `c${pad(i)}` }, gender: 'male' })) } });
      return {
        resourceType: 'Bundle', type: 'collection',
        entry: [patient((f) => range(40, f)), patient((f) => reversedRange(40, f))],
      };
    },
  },
  differentTypes: {
    expr: 'Observation.identifier[0] ~ Observation.code.coding[0]',
    build: () => observation({ identifier: [identifier('v')], code: { coding: [{ system: 'urn:s', code: 'v' }] } }),
  },
};

function median(values) {
  const sorted = [...values].sort((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)];
}

console.log(`fhirpath.js ${require('fhirpath/package.json').version}, node ${process.version}`);
console.log('workload\tversion\titerations\tmedian_ms\tresult');
for (const [name, workload] of Object.entries(workloads)) {
  for (const [version, context] of Object.entries(contexts)) {
    const compiled = fhirpath.compile(workload.expr, context);
    const resource = workload.build(version);
    const result = JSON.stringify(compiled(resource)); // calentamiento
    const times = [];
    const started = Date.now();
    while (times.length < 3 || Date.now() - started < minimumMs) {
      const t0 = process.hrtime.bigint();
      compiled(resource);
      times.push(Number(process.hrtime.bigint() - t0) / 1e6);
      if (times.length >= 2000) break;
    }
    console.log(`${name}\t${version}\t${times.length}\t${median(times).toFixed(4)}\t${result}`);
  }
}
