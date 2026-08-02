# Official FHIRPath conformance suite, R5 (vendored)

The second corpus `TestOfficialSuite` runs, alongside
[`../fhirpath-suite`](../fhirpath-suite) which holds the R4 one.

Two suites are two measurements, not one repeated. The R4 corpus is what the
other engines report against and what most deployed data conforms to. This one
is larger — 1053 cases against 928 — covers functions R4 never exercised, and
evaluates under the rules that changed with R5, which this engine applies from
the version the model declares.

`defineVariable` is the clearest case for keeping both: 83 references here,
none in R4. It was implemented against the specification's examples alone, and
running this suite found six defects in it on the first measurement.

## Contents

| Path | Source |
|------|--------|
| `tests-fhir-r5.xml` | [`FHIR/fhir-test-cases`](https://github.com/FHIR/fhir-test-cases) → `r5/fhirpath/tests-fhir-r5.xml` |
| `input/appointment-examplereq.json`<br>`input/diagnosticreport-eric.json`<br>`input/explanationofbenefit-example.json`<br>`input/patient-container-example.json`<br>`input/patient-name-extensions.json` | `FHIR/fhir-test-cases` → `r5/` (already JSON there) |
| `input/patient-example.json`<br>`input/observation-example.json`<br>`input/questionnaire-example.json`<br>`input/valueset-example-expansion.json`<br>`input/codesystem-example.json`<br>`input/conceptmap-example.json` | <https://hl7.org/fhir/R5/> — the suite names these inputs as `.xml`; this engine consumes JSON, so the published JSON equivalent is vendored under the same base name |
| `known-failures.txt` | Generated. Cases that do not pass with no FHIR model supplied |
| `known-failures-model.txt` | Generated. Cases that do not pass with the R5 model supplied |

These are the R5 examples, not the R4 ones: the resources differ between
versions, so the inputs are vendored separately rather than shared with the
other suite.

## Cases not executed

Five inputs have no published JSON equivalent (`parameters-example-types.xml`,
`patient-example-period.xml`, `ccda.xml`, `parameters-example-html.xml` and
`patient-example-name.xml`), so **16 of the 1053 cases are skipped**. The test
logs each skip with its input file — coverage is reported, never quietly
reduced.

The same caveat as the R4 suite applies to the inputs taken from hl7.org: they
are the published examples, and the suite's own copies are not guaranteed
identical. They have not been diffed element by element here.

## Updating

Refresh `tests-fhir-r5.xml` from the upstream repository, then regenerate the
baselines:

    go test -run TestOfficialSuite -update-known-failures ./conformance/

A line removed from a baseline must stay passing; a new failure fails the
build.
