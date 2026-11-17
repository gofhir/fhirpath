# Unitless FHIR Quantity (plan 026, FP-27)

A FHIR Quantity with neither `code` nor `unit` read as a quantity of the UCUM
default unit `'1'`, as a number is: comparison with a unit, arithmetic,
`comparable()`, `toString()`, boundaries, profiles of Quantity, and the rng-2
invariant of each version on a unitless Range. There are 39 cases, each over R4
(4.0.1, rng-2 as R4 and R4B write it) and R5 (5.0.0, rng-2 as R5 writes it).

Run with `make three-engine BATTERY=batteries/unitless-quantity/battery.tsv DIFFTEST_ARGS="-java <validator jar> -jobs 2"`. `difftest` has no R4B; the three versions are run in `conformance/unitless_quantity_test.go`.

## Classification

Measured on the 026 branch (fhirpath.js 5.2.0, HL7 validator 6.10.2), R4 and
R5 alike except where a version is named. `u` is `{"value":1}`, `h`
`{"value":2}`; the lows of `obs.json` are, in order, `u`, `ua`/`ub` (ids `a`
and `b`), `mg`, `sys` (system alone), `eu` (`unit: ""`), `ec` (`code: ""`
beside `unit: "mg"`), `cal` (UCUM code `a`), `cmp` (comparator `<`) and
`big` (`1e1001`).

- **All three agree (13 rows over the two versions).**
  `u.comparable(1 'mg')` and `sys.comparable(1 'mg')` false, `u <= h` true,
  `Claim.total.comparable(1 '1')` false, `ua in ub` false, the two ranges
  holding `ua` and `ub` unequal, rng-2 true on an ascending unitless Range.
  On main the two `comparable` rows were true here.
- **Changed by 026 toward the references.** `u = 1 'mg'` and
  `valueQuantity = 1 'mg'` are empty, as Java answers (fhirpath.js false);
  `u ~ 1 'mg'` is false, as fhirpath.js answers (Java errors); `u < 2 'mg'`
  and `u + 1 'mg'` are empty where both references raise an error. On main
  all five answered as if `u` were in milligrams.
- **An empty `code` read as absent (plan 026 A5).** `ec`, `code: ""` beside
  `unit: "mg"`, is now in milligrams: `ec = 1 'mg'` true here, false in
  fhirpath.js, empty in Java; `ec.toString()` is `1 'mg'` here (`1` on main),
  and neither reference writes a FHIR Quantity as a quantity.
- **The unit `'1'`, on purpose (plan 026 §3).** `u.toString()`,
  `u.toQuantity()`, `u + h`, the boundaries of `u`, `u = 1`, `u ~ 1`,
  `u = 1 '1'`, `u.comparable(h)`, and `eu`, `cmp`, the Ratio denominator and
  the Range low against `1 '1'`: true here, false in fhirpath.js, which reads
  none of them as a quantity; Java answers `= 1 '1'` empty or errors on the
  arithmetic. Java answers `sys = 1 '1'` and `Condition.onsetAge = 1 '1'`
  true, as this engine does: it reads a unitless Quantity with a UCUM system,
  or an Age, in the unit `'1'`.
- **FP-27, on purpose.** `ua ~ ub` and `ua = ub`, two unitless Quantities
  that differ only in `id`, are true: compared as quantities, as with a unit,
  where Java agrees. fhirpath.js answers false; Java errors under `~` and is
  empty under `=`.
- **Unchanged by 026.**
  - `toString()` of a FHIR Quantity with a calendar code, `cal`: this engine
    writes it as a quantity (`1 year`); both references do not.
  - `big = 1` is empty, a value beyond the decimal bound (#232); both
    references answer false. `big.toQuantity()`: Java alone answers a value.
  - A nested unitless Quantity (plan 025 C2): the ranges holding `ua` and
    `ub` are not equivalent here and in fhirpath.js; Java errors.
  - `ua.combine(ub).distinct().count()` is 2 here and in fhirpath.js, which
    keep the `id` (FP-28); in Java the count is not 2.
  - rng-2, R4 text, on descending unitless Ranges (2 to 1, 3 to 1): false
    here and in Java; fhirpath.js answers true, as it cannot order the two
    objects.
  - rng-2, R5 text: fhirpath.js errors on every unitless Range (it reads no
    boundary of one). On the disjoint descending Range (3 to 1) Java answers
    true, since its `lowBoundary()` of a unitless Quantity is the Quantity
    itself and `comparable()` false; the boundaries 2.5 and 1.5 are
    comparable and out of order, so this engine answers false, as on main.
