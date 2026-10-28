# Resume point

- **Active increment:** 011 — the decimal range must not depend on
  shopspring's `MaxDecodeExponent` (#235). Plan: `docs/plans/011-decimal-decoder-bound.md`.
  010 (#232) merged as #236; 009 (#231) as #233.
- **Step:** 9, closed. Review converged in round 2; make check green, mutations 12/12 and 50/50. Plan approved by the user's three answers
  (2026-10-27): grammar and globals to issues #238 and #237, bump to v1.6.0.
  Tests written first (failed: guard, decoder, #232 suite under v1.6.0);
  `decodeDecimal` in; root and conformance green under v1.6.0; fuzz 6.1M
  execs clean.
- **Measured** (scratchpad `i235`, main with decimal v1.6.0): five packages
  fail; a resource's `1e20000`, `1e999999999` and `1.` + 15000 zeros read as
  absent again; a FHIR Quantity of `1e20000` gives `= 1 'mg'` false;
  `types.NewDecimal("1e−2")` is 0.01. String conversions are unaffected
  (their regex checks first). go-cql requires v1.6.0.
- **Next:** commit and PR when the user asks (PR text names the changed error text of ParseDecimal/NewDecimal). Then #237 or #238.
- **Pending user decisions:** none for 011. From 010, still open outside this repo: searchindex SI-3, go-cql
  CQL-1 (docs/upstream-findings.md, unpublished).
