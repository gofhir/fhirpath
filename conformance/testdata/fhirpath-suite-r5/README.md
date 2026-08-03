# Official FHIRPath conformance suite, R5 (vendored)

The second corpus `TestOfficialSuite` runs, alongside
[`../fhirpath-suite`](../fhirpath-suite) which holds the R4 one.

Two suites are two measurements, not one repeated. The R4 corpus is what the
other engines report against and what most deployed data conforms to. This one
is larger — 1051 cases against 935 — covers functions R4 never exercised, and
evaluates under the rules that changed with R5, which this engine applies from
the version the model declares.

`defineVariable` is the clearest case for keeping both: 83 references here,
none in R4. It was implemented against the specification's examples alone, and
running this suite found six defects in it on the first measurement.

## Contents

| Path | Source |
|------|--------|
| `tests-fhir-r5.xml` | [`FHIR/fhir-test-cases`](https://github.com/FHIR/fhir-test-cases) → `r5/fhirpath/tests-fhir-r5.xml` |
| `SUITE_COMMIT` | The `fhir-test-cases` commit the suite and its inputs were taken from |
| `input/*.xml` | `FHIR/fhir-test-cases` → `r5/`, byte for byte |
| `input/*.json` | `FHIR/fhir-test-cases` → `r5/`, for the inputs it publishes as JSON |
| `known-failures.txt` | Generated. Cases that do not pass with no FHIR model supplied |
| `known-failures-model.txt` | Generated. Cases that do not pass with the R5 model supplied |

These are the R5 resources, vendored separately rather than shared with the
other suite: the examples differ between versions, and so does the unmarshaler
that reads them.

## Every input is the suite's own

The inputs are the files the suite was written against, in the format it ships
them. The XML ones are converted on load through `gofhir/models`, whose
generated types carry the cardinality and primitive type of every element, which
a generic XML-to-JSON mapping cannot supply. See the
[R4 README](../fhirpath-suite/README.md) for the detail.

Three divergences used to be recorded here, and reading the suite's own files
closed all three:

- `observation-example` had been reconciled by hand, transcribing an extension
  the published example lacks. No longer transcribed — the suite's file is read
  directly.
- `conceptmap-example` was **not** reconciled and was the furthest apart: the
  suite's copy has four groups, nine elements and thirteen targets where the
  published R5 resource has one, four and four. `dvConceptMapExample` asserts a
  projection holds duplicates, true of one resource and not the other.
- `valueset-example-expansion` was **not** reconciled either: the published
  resource's `version` is `5.0.0` where the suite's is `20150622`, and
  `testFHIRPathAsFunction14` and `19` assert the older one.

All four cases now pass, and none of them ever indicated a defect in the engine.

## Cases not executed

`ccda.xml` is a CDA `ClinicalDocument`, not a FHIR resource, so no FHIR
unmarshaler can read it — **3 of the 1051 cases are skipped**. The harness logs
the reason with the input file, distinguishing an input it cannot read from one
that is missing.

Those three run FHIRPath over non-FHIR XML, which needs a document model this
engine does not have. It is a real gap rather than a data problem.

## Updating

Refresh `tests-fhir-r5.xml` and the inputs from the upstream repository, then
regenerate the baselines:

    go test -run TestOfficialSuite -update-known-failures ./conformance/

A line removed from a baseline must stay passing; a new failure fails the
build.
