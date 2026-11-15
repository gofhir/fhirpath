# Resume point

- **Active increment:** none. 023 (#265, `=` between objects answers the same
  in both directions) merged 2026-11-07 as 81a4b67 (PR #269); release 1.16.8
  is open as PR #267. Plan and evidence:
  `docs/plans/023-object-equality-symmetry.md`.
- **Next:** 024, `=` is transitive (plan
  `docs/plans/024-object-equality-transitivity.md`). Closed 2026-11-14 (step
  9) on branch `fix/object-equality-transitivity`, committed with its PR open
  (stacked on PR #270). `make check` is green, and code review converged in
  round 3. Merge when the user asks. Open issues: #263, #249, #248, #242. MOD-1 is reported as
  gofhir/models#90. FP-16, FP-19, FP-20, FP-21, FP-23, FP-24, FP-25, FP-26 and JP-3 are open;
  FP-22 closed with 023.
  go-cql resumes its plan 019 on v1.16.7 once its 023 closes.
- **Pending user decisions:** None for 016. From 010, still open outside this repo: searchindex SI-3, go-cql
  CQL-1 (docs/upstream-findings.md, unpublished).
