// Mide fhirpath.js en las operaciones de conjunto que mide
// BenchmarkSetOperations de este motor (#243): las mismas expresiones sobre las
// mismas Observations, componentes y cadenas generadas, con n = 10, 100, 1000 y
// 4000. Es una referencia de un rival para el plan, no un test.
//
// Uso: node set_operations_timing.js [ms-mínimos-por-caso]   (por omisión 1000)
//
// Escribe una línea por caso: conjunto de datos, operación, n, iteraciones y la
// mediana de milisegundos por evaluación. El tiempo de compilación queda fuera,
// porque el benchmark de Go compila una vez; cada evaluación corre sobre el
// recurso ya leído, que el benchmark de Go lee en cada evaluación (un costo
// lineal en n).

const fhirpath = require('fhirpath');
const r4 = require('fhirpath/fhir-context/r4');

const sizes = [10, 100, 1000, 4000];
const minimumMs = Number(process.argv[2] || 1000);

function observation(n, item) {
  const component = [];
  for (let i = 0; i < n; i++) component.push(item(i));
  return { resourceType: 'Observation', id: 'o', status: 'final', code: { text: 'c' }, component };
}

const datasets = {
  components: {
    build: (n) => observation(n, (i) => ({
      code: { coding: [{ system: 'http://loinc.org', code: `c${i}` }], text: `t${i}` },
      valueQuantity: { value: i, unit: 'mg' },
    })),
    exprs: {
      union: 'Observation | Observation.component',
      distinct: 'Observation.component.distinct()',
      exclude: 'Observation.component.exclude(Observation.component.tail())',
      subset: 'Observation.component.subsetOf(Observation.component)',
    },
  },
  strings: {
    build: (n) => observation(n, (i) => ({ code: { text: `t${i}` } })),
    exprs: {
      union: 'Observation.component.code.text | Observation.component.code.text',
      distinct: 'Observation.component.code.text.distinct()',
      exclude: 'Observation.component.code.text.exclude(Observation.component.code.text.tail())',
      subset: 'Observation.component.code.text.subsetOf(Observation.component.code.text)',
    },
  },
};

function median(values) {
  const sorted = [...values].sort((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)];
}

console.log(`fhirpath.js ${require('fhirpath/package.json').version}, node ${process.version}`);
console.log('dataset\toperation\tn\titerations\tmedian_ms');
for (const [name, dataset] of Object.entries(datasets)) {
  for (const [operation, text] of Object.entries(dataset.exprs)) {
    const compiled = fhirpath.compile(text, r4);
    for (const n of sizes) {
      const resource = dataset.build(n);
      compiled(resource); // warm-up
      const times = [];
      const started = Date.now();
      while (times.length < 3 || Date.now() - started < minimumMs) {
        const t0 = process.hrtime.bigint();
        compiled(resource);
        times.push(Number(process.hrtime.bigint() - t0) / 1e6);
        if (times.length >= 200) break;
      }
      console.log(`${name}\t${operation}\t${n}\t${times.length}\t${median(times).toFixed(3)}`);
    }
  }
}
