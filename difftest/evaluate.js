// Evaluates a batch of expressions with fhirpath.js and reports what each one
// answered, so that a Go program can compare the two engines case by case.
//
// Reads one JSON batch on stdin, writes one JSON array on stdout. Each case
// answers with either its results or the error it raised — a refusal is an
// answer, and the interesting divergences are often exactly there.

// fhirpath.js writes warnings — a truncated quantity, say — with console.log,
// which would land in the middle of the JSON this writes. They are worth
// keeping, so they go to stderr rather than being silenced.
for (const level of ['log', 'info', 'warn', 'error']) {
  console[level] = (...args) => process.stderr.write(args.join(' ') + '\n');
}

const fhirpath = require('fhirpath');

const models = {
  r4: require('fhirpath/fhir-context/r4'),
  r5: require('fhirpath/fhir-context/r5'),
};

function readStdin() {
  return new Promise((resolve, reject) => {
    let data = '';
    process.stdin.setEncoding('utf8');
    process.stdin.on('data', (chunk) => { data += chunk; });
    process.stdin.on('end', () => resolve(data));
    process.stdin.on('error', reject);
  });
}

function evaluateOne(testCase) {
  const model = testCase.model ? models[testCase.model] : undefined;

  try {
    const results = fhirpath.evaluate(testCase.resource, testCase.expression, null, model);
    return { id: testCase.id, results: results.map(render) };
  } catch (e) {
    return { id: testCase.id, error: String(e && e.message ? e.message : e) };
  }
}

// fhirpath.js answers with JavaScript values, and its temporal and quantity
// types carry their own shapes. Rendering each to the string the value stands
// for is what makes the two engines comparable at all.
function render(value) {
  if (value === null || value === undefined) return 'null';
  if (typeof value === 'object') {
    if (typeof value.asStr === 'string') return value.asStr;
    if (typeof value.toString === 'function' && value.constructor !== Object) {
      const text = value.toString();
      if (text !== '[object Object]') return text;
    }
    return JSON.stringify(value);
  }
  return String(value);
}

readStdin()
  .then((raw) => {
    const batch = JSON.parse(raw);
    process.stdout.write(JSON.stringify(batch.map(evaluateOne)));
  })
  .catch((e) => {
    process.stderr.write(String(e) + '\n');
    process.exit(1);
  });
