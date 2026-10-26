# Resume point

- **Active increment:** none. [009 — FHIR temporal forms](009-fhir-datetime-grammar.md)
  (#231) closed 2026-10-26, uncommitted: `types/object.go`,
  `conformance/fhir_temporal_form_test.go`, `types/fhir_temporal_form_test.go`,
  `CONFORMANCE.md`, `difftest/batteries/fhir-temporal-form/`, this folder.
- **Next:** on the user's word, commit 009 on a branch and open a PR
  (closes #231); then #232.
- **Pending user decisions:** whether to file the drafted fhirpath.js report
  (fraction over 9 digits) and the HL7 note (R5 dateTime regex accepts
  `2020-01-15T10:30:00+`); whether to comment on #231/#232; go-searchindex
  adapts to 009 when it adopts the release (its own parser for values outside
  their form; five values out of `TestFhirpathInvalidFormsKnownLoss`).
- **Queue:** #232 (decimal beyond `DecimalDigits` read as absent).
