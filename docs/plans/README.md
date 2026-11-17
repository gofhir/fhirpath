# Resume point

- **Active increment:** 026 (FP-27, a FHIR Quantity without a unit), plan: `docs/plans/026-unitless-fhir-quantity.md`. Batteries in
  the scratchpad (`fp27/`). User chose A (unit '1') 2026-11-17; plan written (§3). Step 3: round 1 done (A1–A8 confirmed, plan revised, FP-28 and FP-29 logged); round 2 done (B1 medium, B2 low, both accepted); round 3 converged. Plan approved by the user 2026-11-17. Step 5 done: tests on branch `fix/unitless-fhir-quantity` fail on main only for the planned reason (15 cases per version; plan §5). Step 6 done: `quantityOf` gives '1' and reads an empty code/unit as absent (types/object.go); one old test updated (planned change); 026 tests and lint pass. Step 7 done: battery classified, corpusdiff 0, U1–U6 caught. Step 8: round 1 done (R1, R2 medium on tests, R3 low; all fixed, plan §8). A root test of the old rule (`TestCalendarDurationOrdering`) failed and made the first subset run void; fixed (plan §8). Subset rerun: 99/101 caught, V10 equivalent, M39 now equivalent for engine-built values and pinned by a new types test. Round 2 converged (one low finding fixed). Step 9 done: CONFORMANCE.md updated, make check green (2026-11-17). Committed on `fix/unitless-fhir-quantity`, PR open. Next: merge when the user asks.
  Other candidates: issues #249, #248, #242, FP-6.
- **Merged:** 025 (#263, `~` types a complex value's children as navigation
  does) merged 2026-11-17 as 2905311 (PR #272). Plan and evidence:
  `docs/plans/025-typed-complex-equivalence.md`. The backbone "2020"
  divergence (dateTime against string, false here, true in both references)
  shipped as recorded in CONFORMANCE.md; the user has not ruled on it
  separately.
- **Previous:** 023 (#265, `=` between objects answers the same
  in both directions) merged 2026-11-07 as 81a4b67 (PR #269). Plan and evidence:
  `docs/plans/023-object-equality-symmetry.md`.
- **Merged:** 024 (`=` is transitive, plan
  `docs/plans/024-object-equality-transitivity.md`) merged 2026-11-15 as PR
  #271. Open issues: #249, #248, #242. MOD-1 is reported as
  gofhir/models#90. FP-16, FP-19, FP-20, FP-21, FP-23, FP-24, FP-25, FP-26 and JP-3 are open;
  FP-22 closed with 023.
  go-cql resumes its plan 019 on v1.16.7 once its 023 closes.
- **Pending user decisions:** None for 016. From 010, still open outside this repo: searchindex SI-3, go-cql
  CQL-1 (docs/upstream-findings.md, unpublished).
