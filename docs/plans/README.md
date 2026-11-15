# Resume point

- **Active increment:** none. 023 (#265, `=` between objects answers the same
  in both directions) merged 2026-11-07 as 81a4b67 (PR #269); release 1.16.8
  is open as PR #267. Plan and evidence:
  `docs/plans/023-object-equality-symmetry.md`.
- **Next:** 024, `=` is transitive. The fast paths must agree with the reader
  on duplicate keys, nested arrays, and elements that hold no value but are
  written with content, through an oracle that disables the fast paths at
  every depth (plan 023, "Transitivity: increment 024"). Open issues: #263, #249, #248, #242. MOD-1 is reported as
  gofhir/models#90. FP-16, FP-19, FP-20, FP-21, FP-23, FP-24, FP-25 and FP-26 are open;
  FP-22 closed with 023.
  go-cql resumes its plan 019 on v1.16.7 once its 023 closes.
- **Pending user decisions:** None for 016. From 010, still open outside this repo: searchindex SI-3, go-cql
  CQL-1 (docs/upstream-findings.md, unpublished).
