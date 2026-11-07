# Resume point

- **Active increment:** 023, `=` between objects answers the same in both
  directions (#265). Closed 2026-11-06 (step 9), in PR #269. `make check` is
  green, and code review converged in round 1. Plan:
  `docs/plans/023-object-equality-symmetry.md`. Preceded by 022 (#243), merged
  2026-11-06 as fd9597b (PR #266). Release 1.16.8 (PR #267) is open.
- **Next:** record 023 as merged. Then 024:
  `=` is transitive, so the fast paths must agree with the reader (duplicate
  keys, nested arrays, elements that hold no value written with content; see
  the plan). Open issues: #265, #263, #249, #248, #242. MOD-1 is reported as
  gofhir/models#90. FP-16, FP-19, FP-20, FP-21, FP-23, FP-24, FP-25 and FP-26 are open;
  FP-22 closes with 023.
  go-cql resumes its plan 019 on v1.16.7 once its 023 closes.
- **Pending user decisions:** None for 016. From 010, still open outside this repo: searchindex SI-3, go-cql
  CQL-1 (docs/upstream-findings.md, unpublished).
