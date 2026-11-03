# Resume point

- **Active increment:** none. 017 — #241 closed as not planned
  (2026-11-02): this engine answers as fhirpath.js and FHIR R5
  fhirpath.html 2.1.9.1.2; the defect is R4/R4B's code-value-date
  expression (SPEC-9), which R5 rewrote. Battery:
  difftest/batteries/primitive-type-operators. New: FP-16 (as/is
  System.Quantity), FP-17 (birthDate.ofType(Date)). Uncommitted here and in
  go-searchindex (known-loss reason cites SPEC-9).
- **Next:** commit and PR in both repos when the user asks; then #240,
  #243, #242, #248 or #249.
- **Pending user decisions:** none for 016. From 010, still open outside this repo: searchindex SI-3, go-cql
  CQL-1 (docs/upstream-findings.md, unpublished).
