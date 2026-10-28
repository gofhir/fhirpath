# Resume point

- **Active increment:** 010 — a FHIR decimal beyond `DecimalDigits` (#232).
  009 (#231) merged as #233.
- **Step:** 9, closing. Seven review rounds (plan 010, "Code review"):
  rounds 3–6 left medium findings, all fixed with failing tests (the user
  asked for rounds 4–6 and chose round 6's fix); round 7 converged.
- **Measured** (scratchpad `dec/`, batteries a–d): fhirpath.js and Java both
  read a decimal beyond the bound (`exists()` true, `1e1000 > 0` true, also
  `1e999999999`); before 010 only this engine read it as absent. fhirpath.js holds
  float64 (`1e1000` → Infinity, `1e-1000` → 0); Java answers `1e-1000 > 0`
  false. FHIR: decimal is "union of xs:decimal and xs:double", JSON
  "restricted to the precision limits … for xs:double and xs:decimal";
  R5's regex allows a 9-digit exponent. gofhir/server read decimals as
  float64: nothing to take.
- **Next:** closing checks green (make check, corpusdiff, mutations 50/50);
  commit and PR only when the user asks. Mutations in
  `mut/mutate010.py`.
- **Pending user decisions:** to flag at
  closing: the four exact agreements not followed (`floor`/`round`/`div` of
  `1e-1001`), the contract change for searchindex (SI-3) and go-cql (CQL-1),
  and the int32 exponent known loss (FP-6).
