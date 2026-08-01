# Official FHIRPath conformance suite (vendored)

Test data for `TestOfficialSuite` in [`conformance_test.go`](../../conformance_test.go).
Vendored so the suite runs offline, in CI, and against a recorded version — a
conformance number is only meaningful if you know what it was measured against.

## Contents

| Path | Source |
|------|--------|
| `tests-fhir-r4.xml` | [`FHIR/fhir-test-cases`](https://github.com/FHIR/fhir-test-cases) → `r4/fhirpath/tests-fhir-r4.xml` |
| `SUITE_COMMIT` | The `fhir-test-cases` commit the suite was taken from |
| `input/appointment-examplereq.json`<br>`input/explanationofbenefit-example.json`<br>`input/patient-container-example.json`<br>`input/patient-name-extensions.json` | `FHIR/fhir-test-cases` → `r4/` (already JSON there) |
| `input/patient-example.json`<br>`input/observation-example.json`<br>`input/questionnaire-example.json`<br>`input/valueset-example-expansion.json`<br>`input/codesystem-example.json` | <https://hl7.org/fhir/R4/> — the suite names these inputs as `.xml`; this engine consumes JSON, so the published JSON equivalent is vendored under the same base name |
| `known-failures.txt` | Generated. Cases that do not pass with no FHIR model supplied |
| `known-failures-model.txt` | Generated. Cases that do not pass with the R4 model supplied |

The suite itself is maintained upstream in the R5 file; the R4 copy is the one
that matches the resources this engine is tested against.

## A caveat on the converted inputs

The five inputs taken from hl7.org are the *published* examples, while the suite
runs against the copies in `fhir-test-cases`, and those are not always identical:
`observation-example.xml` there carries an extension
(`http://example.com/fhir/StructureDefinition/patient-age`) that the published
example does not. Three cases exercise it and cannot pass with this input.

Converting the suite's own XML resources to JSON would remove both this
discrepancy and the skipped cases below. It needs a FHIR-aware converter, since
XML gives no hint of which elements are arrays.

## Cases not executed

Two inputs have no published JSON equivalent (`parameters-example-types.xml` and
`patient-example-period.xml`, checked against `hl7.org/fhir/R4`, `build.fhir.org`
and the `fhir-test-cases` repository), so **9 of the 937 cases are skipped**. The
test logs each skip with its input file — coverage is reported, never quietly
reduced. Converting those two resources from XML would close the gap.

## Updating

```sh
# refresh the suite and inputs, then re-baseline
go test -run TestOfficialSuite -update-known-failures
```

Review the diff of `known-failures.txt` before committing: lines disappearing is
progress, lines appearing is a regression.
