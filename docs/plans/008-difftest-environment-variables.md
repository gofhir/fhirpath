# 008 — difftest gives fhirpath.js the environment FHIR fixes

Status: done, pending commit; approved by the user ("terminar lo pendiente": the harness fix
the #229 adjudication asked for)

## Problem

The difftest harness calls `fhirpath.evaluate(resource, expression, null, model)`.
fhirpath.js defines only `%ucum` and `%context` itself
(`node_modules/fhirpath/src/fhirpath.js:1309-1312`) and throws "Attempting to
access an undefined environment variable" for any other unless its caller
passes it (`:377-380`). This engine defines the variables FHIR's FHIRPath page
fixes. So 49 comparisons (`make difftest DIFFTEST_ARGS="-corpus all -v"`,
measured on main at `1a7abcb`) differ because of the harness, not the
engines: spec cases REQ-FHIRFUNCTIONS-004-e, 021, 022, 025, 026, 030, 065,
the suite's testVariables1, 2, 4, testExtension2, HTMLChecks/htmlTest04.

## Approach

The harness passes what a caller of fhirpath.js passes for an evaluation at a
resource's root, the values this engine gives there:

- `%resource` and `%rootResource`: the resource evaluated (FHIR: "the
  resource that contains the original node that is in %context" and "the
  container resource for the resource identified by %resource"; at the root
  both are it). fhirpath.js turns a variable with a `resourceType` into a
  typed node, as it does the root (`fhirpath.js:347-368`).
- `%sct` and `%loinc`: `http://snomed.info/sct` and `http://loinc.org`, the
  fixed values of FHIR's page, as `%ucum` already is.

Not passed: `%vs-[name]` and `%ext-[name]`. They are parameterised; passing
them would mean the harness parsing expressions for names, a resolution
fhirpath.js does not do. Their divergences stay, as real ones: fhirpath.js
leaves them to its caller.

## Tests

A Go test in difftest runs `evaluateWithFHIRPathJS` (skipped when Node or
`node_modules` is absent, so `make check` stays offline-safe) on
`%resource.id`, `%rootResource.id`, `%resource is Patient`, `%sct`, `%loinc`
and expects `p`, `p`, `true`, the two URLs.

## Validation

`make difftest DIFFTEST_ARGS="-corpus all -v"` before and after: the
divergences caused by these four variables go; what remains for
`%vs-`/`%ext-` and anything else is examined; no previously agreeing case
diverges.

## Record

- Test first: `TestFhirpathJSIsGivenTheEnvironmentFHIRFixes` failed all five
  cases before the change ("undefined environment variable") and passes after.
- `make difftest DIFFTEST_ARGS="-corpus all -v"`, before and after, 7,167
  comparisons: divergences from undefined variables 49 → 10, all of them now
  `%vs-…`/`%ext-…` (REQ-FHIRFUNCTIONS-025-d, 026-a/b, testExtension2,
  testVariables4, in R4 and R5); total divergences 532 → 495 (from the
  per-section summaries the tool prints).
- Two comparisons moved the other way: R5's HTMLChecks/htmlTest02 and
  htmlTest03 (`%resource.parameter.where(…).value.htmlChecks()`) agreed before
  only because both refused — fhirpath.js on the undefined variable, this
  engine for want of an HTML checker — and now diverge: fhirpath.js answers,
  this engine still asks its caller for a checker, as CONFORMANCE.md records.
  The divergence is real and was hidden; it is listed, not a regression. A
  first check diffed only the IDs of diverging cases, and missed it because
  both IDs were already listed under another heading; the independent review
  found it.

## Code review

One independent round: the emulation matches this engine's `builtinVariable`
(variables fixed for the whole evaluation, so a `%rootResource` that stays
the container inside `contained.all(…)` is right for both); fhirpath.js does
not modify the environment; the test skips without Node and discriminates.
Medium: the record's "no comparison that agreed before diverges after" was
false (the two HTML checks above); corrected.
