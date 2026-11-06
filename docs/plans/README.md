# Resume point

- **Active increment:** 022, union and `distinct()` in linear time (#243).
  Closed 2026-11-06 (step 9), uncommitted. `make check` is green, and code
  review converged in round 3. Plan:
  `docs/plans/022-linear-set-operations.md`, where §5 has the evidence.
  Preceded by 021 (#240), merged 2026-11-05 as f56a48c (PR #264).
- **Next:** commit and PR of 022 when the user asks, closing #243. Then the
  next issue. Open issues: #265 (`=` asymmetry, found by 022), #263, #249, #248, #243,
  #242. MOD-1 is reported as gofhir/models#90. FP-16, FP-19, FP-20, FP-21 and
  FP-22 (#265) are open. go-cql resumes its plan 019 on v1.16.7 once its 023 closes.
- **Pending user decisions:** none for 022 (decided 2026-11-05, plan §5). None for 016. From 010, still open outside this repo: searchindex SI-3, go-cql
  CQL-1 (docs/upstream-findings.md, unpublished).
