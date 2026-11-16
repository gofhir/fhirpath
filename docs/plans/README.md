# Resume point

- **Active increment:** 025 (#263, `~` types a complex value's children with
  the model), step 9 (closing) done: review converged in round 2, CONFORMANCE.md updated, make check green (2026-11-16). Uncommitted. Next: the user's answer on the backbone "2020" divergence, then commit and PR when asked. Plan approved 2026-11-15; the user
  decided C1, C2 (plan §4) and the time/integer64 divergences (plan §5).
  Cost budgets set in plan §5 from the main baseline and fhirpath.js timing.
  Battery `difftest/batteries/typed-complex-equivalence` run and classified.
  Plan: `docs/plans/025-typed-complex-equivalence.md`, branch
  `fix/typed-complex-equivalence`, uncommitted. Next: tests written by a
  Sonnet subagent (conformance + types + property); check they fail on main
  for the right reason, then step 6 (implementation). FP-27 is logged. Release 1.16.8 published 2026-11-15 (PR #267).
- **Previous:** 023 (#265, `=` between objects answers the same
  in both directions) merged 2026-11-07 as 81a4b67 (PR #269). Plan and evidence:
  `docs/plans/023-object-equality-symmetry.md`.
- **Merged:** 024 (`=` is transitive, plan
  `docs/plans/024-object-equality-transitivity.md`) merged 2026-11-15 as PR
  #271. Open issues: #263, #249, #248, #242. MOD-1 is reported as
  gofhir/models#90. FP-16, FP-19, FP-20, FP-21, FP-23, FP-24, FP-25, FP-26 and JP-3 are open;
  FP-22 closed with 023.
  go-cql resumes its plan 019 on v1.16.7 once its 023 closes.
- **Pending user decisions:** None for 016. From 010, still open outside this repo: searchindex SI-3, go-cql
  CQL-1 (docs/upstream-findings.md, unpublished).
