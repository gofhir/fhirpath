# Official FHIRPath conformance suite (vendored)

Test data for `TestOfficialSuite` in [`conformance_test.go`](../../conformance_test.go).
Vendored so the suite runs offline, in CI, and against a recorded version — a
conformance number is only meaningful if you know what it was measured against.

## Contents

| Path | Source |
|------|--------|
| `tests-fhir-r4.xml` | [`FHIR/fhir-test-cases`](https://github.com/FHIR/fhir-test-cases) → `r4/fhirpath/tests-fhir-r4.xml` |
| `SUITE_COMMIT` | The `fhir-test-cases` commit the suite and its inputs were taken from |
| `input/*.xml` | `FHIR/fhir-test-cases` → `r4/`, byte for byte |
| `input/*.json` | `FHIR/fhir-test-cases` → `r4/`, for the inputs it publishes as JSON |
| `known-failures.txt` | Generated. Cases that do not pass with no FHIR model supplied |
| `known-failures-model.txt` | Generated. Cases that do not pass with the R4 model supplied |

The suite itself is maintained upstream in the R5 file; the R4 copy is the one
that matches the resources this engine is tested against.

## Every input is the suite's own

The inputs are the files the suite was written against, in the format it ships
them. `parameters-example-types.xml` and the rest are XML, so the harness
converts them on load through `gofhir/models`, whose generated types carry the
cardinality and primitive type of every element — a lone `<name>` becomes a
collection of one, and `value="true"` becomes a boolean rather than the string
`"true"`. A generic XML-to-JSON mapping cannot do either.

This replaced substituting the equivalents published at hl7.org, which cost
twice. Resources hl7.org never published as JSON could not be run at all, and
those it did publish had moved on from the suite's copies — `codesystem-example`
had lost the nested concepts `testCombine1` counts duplicates among, so the case
measured a different resource than its expected result was written for. Reading
the suite's own files removed the whole category: nothing here is transcribed,
patched, or diffed by hand.

## Cases not executed

None. All **935 cases run**.

The harness logs every skip with its input file, so if a future input cannot be
read the coverage says so rather than quietly shrinking.

## Updating

```sh
# refresh the suite and inputs from the pinned commit, then re-baseline
go test -run TestOfficialSuite -update-known-failures
```

Review the diff of `known-failures.txt` before committing: lines disappearing is
progress, lines appearing is a regression.
