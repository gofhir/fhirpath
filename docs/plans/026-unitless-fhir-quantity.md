# 026 — A FHIR Quantity without a unit (FP-27)

Status: step 9 (closing), 2026-11-17. Plan approved by the user the same day. The Before column
is measured (probe, scratchpad `fp27/`). The user chose
option A (unit `'1'`).

## 1. Prior review

### The finding

FP-27 (`docs/upstream-findings.md`, found in plan 025 §3c, C2): with a model, a
FHIR Quantity with neither `code` nor `unit` compares under `~` by its value
alone, its `id` and extensions aside. `referenceRange[0].low ~
referenceRange[1].low` over `{"value":1,"id":"a"}` and `{"value":1,"id":"b"}`
is true. fhirpath.js 5.2.0 answers false; the HL7 validator 6.10.2 errors.

### What the code does

`ObjectValue.quantityOf` (`types/object.go`) maps a Quantity by its `code`, or
its `unit` where it has none (the divergence in CONFORMANCE.md, "a FHIR
Quantity maps by its code or unit"). With neither, the unit is `""`. Every
other producer of a quantity without a written unit gives it `'1'`
(`types/quantity.go` for numbers, `funcs/conversion.go` for `toQuantity()`),
so `""` comes only from this path.

A `""` unit "takes the other's" (`unitsAlike`, `inLessGranular`,
`Comparable`, the arithmetic). So a unitless FHIR Quantity is not only
equivalent to another whatever its `id`: it is equal to `1 'mg'`, less than
`2 'mg'` and comparable with a mass. No text and no reference supports that.

### gofhir/server

It has no FHIRPath of its own: it uses this library (v1.14.0). Nothing to
take, improve or drop.

### Specification

- FHIR's FHIRPath page, R4 and R5 alike: "The Mapping from FHIR Quantity to
  FHIRPath System.Quantity can only be applied if the FHIR Quantity has a UCUM
  code - i.e. a system of http://unitsofmeasure.org, and a code is present."
  This engine already departs from it on purpose for local units (rng-2 on
  Ranges in tablets, #112).
- FHIRPath 3.0.0, Conversion: "When an integer, long or decimal is implicitly
  converted to a Quantity, the resulting quantity will have the UCUM default
  unit `'1'`", and `23 = 23 '1'`, `23 ~ 23 '1'` are true.
- Range: `low` and `high` are SimpleQuantity, whose `unit` and `code` are
  optional. rng-2 differs by version (read from the packages): R4 4.0.1 and
  R4B 4.3.0 `low.empty() or high.empty() or (low <= high)`; R5 5.0.0
  `low.value.empty() or high.value.empty() or
  low.lowBoundary().comparable(high.highBoundary()).not() or
  (low.lowBoundary() <= high.highBoundary())`.

### Measured (fhirpath.js 5.2.0, HL7 validator 6.10.2; R4 and R5 alike)

Batteries in the scratchpad (`fp27/battery.tsv`, 42 cases; `fp27/b2.tsv`,
32 cases). `u` is a unitless low `{"value":1}`, `h` a unitless high
`{"value":2}`, `mg` a low `{"value":1,"unit":"mg"}`.

| Expression | here | fhirpath.js | Java |
|---|---|---|---|
| `u ~ u` with different `id`s (FP-27) | true | false | error |
| `u = u` with different `id`s | true | false | empty |
| `u = mg`, `u ~ mg` | true, true | false, false | empty, error |
| `u < 2 'mg'` | true | error | error |
| `u.comparable(mg)` | true | false | false |
| `u.comparable(h)` | true | false | false |
| `u <= h` | true | false | true |
| `u = 1`, `u ~ 1`, `u = 1.0` | true | false | false |
| `u = 1 '1'`, `u ~ 1 '1'` | true | false | empty, error |
| `u.lowBoundary()` | `0.50000000` | error | the object itself |
| rng-2 on a unitless Range | true | error | true |
| `u + h` | `3` | error | error |
| `mg ~ mg` with different `id`s | true | false | true |
| `1 '1' = 1`, `1 '1' ~ 1` | true, true | true, false | false, false |

Where both references agree, they treat a unitless FHIR Quantity as no
quantity: `comparable()` false, not equal to a number. They do not agree with
each other elsewhere, and neither is coherent (Java: `<=` true where
`comparable()` is false).

**Making it no quantity breaks rng-2.** A Money is no quantity here, and the
rng-2 shape over one gives false: `lowBoundary()` is empty, so is
`comparable()`, and `all()` fails (probe on `Claim.total`). A unitless Range
is valid FHIR, and the validator holds rng-2 true on it. That is the #112
trap again.

## 2. Options for the user

- **A, unit `'1'` (recommended).** A FHIR Quantity with no code and no unit
  maps to a quantity of UCUM's default unit `'1'`, as a number does. This
  fixes what no one supports (`u = 1 'mg'`, `u < 2 'mg'`,
  `u.comparable(mg)`), keeps rng-2 true, and makes `u = 1` true, as
  `23 = 23 '1'` is. FP-27 stays true, as with a unit, where Java agrees.
  Divergences on purpose: `u.comparable(h)` true (both references false),
  `u = 1` true (both false), FP-27 (fhirpath.js false, Java errors).
- **B, no quantity.** It compares as the object it is, as fhirpath.js does.
  FP-27 becomes false. rng-2 fails on unitless Ranges unless boundaries change
  too, which would need its own exception.
- **C, only stop `""` taking the other's unit**, and leave everything else as
  today. Smaller, but it keeps a unit no other value has.

**User decision (2026-11-17): A, unit `'1'`.**

## 3. Plan

### Scope

`ObjectValue.quantityOf` gives a FHIR Quantity with neither a `code` nor a
`unit` UCUM's default unit `'1'` (`DefaultQuantityUnit`), the unit FHIRPath
gives a number converted to a quantity. This applies wherever a quantity is
read from an object: with a model (type Quantity or a profile of it) and
without one (a numeric value alone, of no known type, still converts, as
today). Nothing else changes:

- the `""` handling in `types/quantity.go` stays, since `NewQuantity` and
  `NewQuantityFromDecimal` are exported and can still make one; no path
  inside the engine produces it any more;
- a Quantity with a `code` or a `unit` maps as today, a calendar code as today;
- Money stays no quantity;
- `~` between two Quantities still compares them as quantities, so FP-27
  stays true, as with a unit, where Java agrees.

### What changes (all from a unitless FHIR Quantity `u`)

| Expression | Before | After |
|---|---|---|
| `u = 1 'mg'`, `u ~ 1 'mg'` | true, true | empty, false (as `1 '1' = 1 'mg'` today) |
| `u < 2 'mg'`, `u + 1 'mg'` | true, `2 'mg'` | empty, empty (as `1 '1'` against `mg` today) |
| `u.comparable(1 'mg')` | true | false |
| `u.toString()`, `u.toQuantity()` | `1`, `1` | `1 '1'` |
| `u.lowBoundary()` | `0.50000000` | `0.50000000 '1'` |
| `u + h` | `3` | `3 '1'` |
| `u = 1`, `u ~ 1`, `u = 1 '1'` | true | true |
| `u ~ u'` with other `id` (FP-27), `u.comparable(h)`, `u <= h`, rng-2 | true | true |
| A Quantity with `code: ""` beside `unit: "mg"` | unitless (a present `code` blocks the `unit`) | `mg`: an empty string is read as absent (invalid FHIR; robustness) |
| A Quantity with `unit: ""` alone, or `system` alone | unitless | `'1'` |

The "After" answers against `mg` are those the engine already gives for `1 '1'` against the same operand; the tests
assert them against `1 '1'`, not against this engine's output on `u`.

### Exit criterion

1. Every row above holds in R4, R4B and R5, with a model and without one
   (the "without" row set: `u` as a bare `{"value": 1}` under a field of no
   known type, and under `valueQuantity`).
2. `u op x` answers as `1 '1' op x` for the operators and functions that
   take quantities (`=`, `!=`, `~`, `!~`, `<`, `<=`, `>`, `>=`, `+`, `-`,
   `*`, `/`, `comparable`, `toQuantity`, `lowBoundary`, `highBoundary`), in
   both operand orders (a metamorphic test). Excluded, since a resource's
   value and a literal differ there by design: navigation, `is`/`as`/`type()`,
   `toString()` (checked on its own), collection membership and `distinct`
   (finding A3 below), and a value beyond the decimal bound (A7). Every case
   also has an expected value written independently, from the text or a
   reference, not only agreement between the two forms.
3. Each version's own rng-2 (above) holds on an ascending unitless Range and
   fails on a descending one in R4 and R4B (`low` 2, `high` 1); in R5 the
   boundaries of 2 and 1 overlap, so it holds. R4B is run in conformance
   (difftest has no R4B), against the R4 references' answers, since the
   invariant is the same text. Measured today (probe): R4 descending false,
   R5 descending true (`lowBoundary()` of 2 and `highBoundary()` of 1 are
   both 1.5). R5 also needs a disjoint descending Range (`low` 3, `high` 1),
   which fails: boundaries 2.5 and 1.5, comparable and out of order (measured
   false today). Without it, a regression that made unitless boundaries
   incomparable would pass every R5 case.
4. corpusdiff without and with a model against main: every difference is a
   unitless Quantity, listed and classified.
5. Three-engine battery in `difftest/batteries/unitless-quantity/`, each
   difference classified in its README.
6. Mutations caught: the unit left `""`; `'1'` only with a model; `'1'` also
   given to a Quantity with a `unit` and no `code`; Money read as a quantity.
7. `make check` green.

### Tests and data

- `conformance/unitless_quantity_test.go`: the table above, R4, R4B, R5,
  with and without a model, citing FHIRPath 3.0.0 Conversion ("the UCUM
  default unit `'1'`") and FHIR's FHIRPath page (the mapping).
- The metamorphic test of criterion 2, over exactly its list of operations
  and with its exclusions, `u` written in a resource and `1 '1'` as a
  literal. `toString()` is tested on its own; `precision()` is left out, as
  it is empty for both forms.
- A `types` unit test on `AsQuantity` for the unit of each shape: no unit, a
  `unit`, a `code`, a calendar `code`, Money.
- The official examples (R4, R4B, R5) through corpusdiff, for criterion 4.

### Deliberate divergences (CONFORMANCE.md)

- `u.comparable(h)` true and `u <= h` true: both references say
  `comparable()` false, and Java still orders them; rng-2 needs them.
- `u = 1` true: both references say false; FHIRPath's `23 = 23 '1'` is true.
- FP-27, `u ~ u'` with another `id` true: fhirpath.js false, Java errors; with
  a unit Java agrees with this engine.

These extend the existing section "a FHIR Quantity maps by its code or unit";
no new exception to rule 5 (`'1'` is FHIRPath's constant, already in
`DefaultQuantityUnit`). No new dependency.

### Risks

- Output text changes for `toString()` and for any result rendered from a
  unitless Quantity (`1` becomes `1 '1'`). corpusdiff shows how often.
- Callers of the public API that read `Quantity.Unit()` of a FHIR Quantity
  see `'1'` where they saw `""`. Recorded in the changelog as a fix.

## 4. Adversarial round 1 (Codex, 2026-11-17)

Each finding was checked with the probe or the packages.

| # | Sev. | Finding | Verification | Resolution |
|---|---|---|---|---|
| A1 | high | rng-2 is not the same in every version; the plan claimed parity from one ascending Range | Confirmed from the packages: R4 and R4B `low <= high`, R5 with boundaries | Accepted: §1 quotes each text; criterion 3 tests each version's invariant, ascending and descending |
| A2 | high | Unit `'1'` is a policy, not a mapping the text gives; NUCLEO §2.2 asks an ADR | The mapping text requires a UCUM code; `'1'` is the default unit of a number, not of a FHIR object | Partly accepted: the plan lists every consequence (§3 table, deliberate divergences) and CONFORMANCE.md records it under "Text departed from", as every departure in this repo is. Not a rule 5 exception: nothing FHIR is hardcoded, and `'1'` is FHIRPath's constant already in `DefaultQuantityUnit` |
| A3 | high | `=` reads two FHIR Quantities as quantities, while `in`, `contains`, `distinct` and `\|` compare the objects, `id` included | Confirmed, and not unitless only: with `mg`, `low = low'` true, `low in low'` false, `distinct().count()` 2 | Out of 026: it predates it and holds with a unit. Recorded as FP-28. Tests pin that 026 does not change these answers |
| A4 | high | Nested, a unitless Quantity is not one: `referenceRange[0] ~ referenceRange[1]` false while the lows are equivalent | Confirmed: false, as fhirpath.js (Java errors); with `mg` true | Out of 026 by the user's decision C2 in 025 (nested Quantities by shape). Listed as a known limit in CONFORMANCE.md; tests pin it, under `~` and `=` |
| A5 | medium | `code: ""` blocks the fallback to `unit`; `unit: ""` and `system` alone undefined | Confirmed: `{"value":1,"code":"","unit":"mg"}` is unitless today | Accepted: an empty `code` or `unit` is read as absent; rows added to §3 |
| A6 | medium | `quantityTypes` hardcodes seven names; profiles, calendar code and Money uncovered | The list predates 026 | Tests added for Age, Duration, Count, Distance, SimpleQuantity in Range and Ratio (R4/R4B Quantity, R5 SimpleQuantity denominator), the calendar code (`1 year`) and Money (`comparable(1 '1')` false). The hardcoded list is recorded as FP-29 (rule 5), out of 026 |
| A7 | medium | A value beyond the bound and a comparator are missing | `{"value":1e1001}`: `= 1` and `toQuantity()` empty today | Accepted: tests for both, `ad` in R5; excluded from the metamorphic test |
| A8 | medium | The metamorphic test proves less than it seems | `precision()` is empty for both forms | Accepted: criterion 2 restricted to operations that take quantities, with independent expected values |

## 4b. Adversarial round 2 (Codex, 2026-11-17)

| # | Sev. | Finding | Verification | Resolution |
|---|---|---|---|---|
| B1 | medium | Both R5 rng-2 cases hold, so neither detects unitless boundaries turning incomparable | Probe, R5: `low` 3, `high` 1 gives false today | Accepted: the disjoint descending Range added to criterion 3 |
| B2 | low | "Tests and data" still listed `toString()`, `precision()` and every binary operator in the metamorphic test | Text conflict | Accepted: the paragraph points at criterion 2's list |

Round 2 accepted A1–A8's resolutions. R4B against R4's answers is named a
proxy; the fixtures also run against the R4B model (conformance).

## 4c. Adversarial round 3 (Codex, 2026-11-17): converged

B1 and B2 closed; no new finding of medium severity or higher.

## 5. Tests first (2026-11-17)

Written by a Sonnet subagent from the expected values in §3, not from this
engine: `conformance/unitless_quantity_test.go` (table, R4, R4B and R5, with
and without a model), `conformance/unitless_quantity_metamorphic_test.go`
(criterion 2), `types/unitless_quantity_test.go` (the unit of each shape),
and `difftest/batteries/unitless-quantity/` (78 cases, R4 and R5).

On main's code, `TestUnitlessQuantity` fails 15 cases per version, each one a
planned change: `u` against `mg` (`=`, `~`, `<`, `+`, `comparable`), the
unit in `toString()`, `toQuantity()`, the boundaries and `u + h`, `sys` and
`eu` read as `'1'`, `ec` read as `mg`. Without a model, 4 cases fail, the
same against `mg`. The metamorphic test fails where `u` takes the other
operand's unit. Every pin passes: FP-27, FP-28, plan 025 C2, Money, the
comparator, the overflowing value, the profiles and each version's rng-2.

Five rows passed on main only because a `""` unit takes the other operand's
(`toQuantity() = 1 '1'`, the boundaries, `sys`, `eu`); a `toString()`
variant of each was added, and fails on main.

## 6. Implementation (2026-11-17)

A Sonnet subagent changed `ObjectValue.quantityOf` (`types/object.go`) only:
an empty `code` counts as absent; with no code the `unit` is read, and with
neither the unit is `DefaultQuantityUnit`. The calendar mapping is unchanged.
One existing test expected an empty unit for `{"value": 42}`
(`types/types_test.go`, "toQuantity without unit"); it now expects `'1'`, the
planned change. The 026 tests, `types`, `eval`, `funcs` and lint pass.

## 7. Validation (2026-11-17)

- **Battery** (`difftest/batteries/unitless-quantity`, 78 cases, R4 and R5),
  run on the branch: 13 agree, 24 this engine alone, 12 fhirpath.js alone,
  8 Java alone, 21 the three differ. Every difference is classified in its
  README: changed by 026 toward the references (`u` against `mg`), the unit
  `'1'` on purpose (§3), FP-27 on purpose, or unchanged by 026. Java reads a
  unitless Quantity with a UCUM system, and an Age, as `'1'`, as 026 does.
  On R5's rng-2 over a disjoint descending unitless Range (3 to 1) Java
  answers true, since its `lowBoundary()` returns the Quantity itself; this
  engine answers false, as the text gives and as on main (JAVA-18).
- **corpusdiff** against main (`make corpusdiff BASE=origin/main`): 0
  answers differ over 521,007 (R4), 308,577 (R4B) and 360,675 (R5)
  evaluations; no official example reaches a unitless Quantity where the
  answer depends on its unit.
- **Mutations of 026** (`mut/mutate026.py`): U1–U6 caught (the unit left
  `""`, `'1'` only with a model, `'1'` over a written unit, Money read as a
  quantity, an empty code not absent, the calendar code not mapped).
- **Earlier mutations.** A full run takes about 4 hours (250 mutants, 14,831 s
  in `mut/history025r2.txt`). 026 changes `quantityOf` alone and one test
  expectation, so a mutant can only be masked in quantity code: the 95
  mutants of 010–025 in `types/object.go`, `types/quantity.go`,
  `eval/operators.go`, `funcs/boundary.go`, `funcs/conversion.go`,
  `funcs/math.go` and `types/typed_equivalent.go` run again
  (`mut/mutate026sub.py`, about an hour).

## 8. Code review round 1 (Codex)

Codex found no defect in production code (`quantityOf` and the paths that
still handle a `""` unit). Three findings on the tests and the battery.

| # | Sev. | Finding | Verification | Resolution |
|---|---|---|---|---|
| R1 | medium | The metamorphic test has no independent expected values for `*` and `/`, and takes two matching errors as agreement | Read in the test: both answers come from this engine, errors map to `"error"` | Accepted: five arithmetic rows with their own expected values (`u * 2 '1'`, `u / 2 '1'`, `u * 2`, `u * 1 'mg'`, `u - h`, from the default unit's rule); the metamorphic test now fails on any error |
| R2 | medium | Without a model only five rows ran, and the metamorphic test always had a model | Read in the test | Accepted: every row runs without a model too, except Money (without a model a Money is read by its shape); the metamorphic test runs without a model; an unknown field `x` and `valueQuantity.toQuantity()` added |
| R3 | low | The battery README listed `ec.toString()` as unchanged | `ec` is in milligrams now (A5) | Accepted: reclassified as a change of 026 |

The new rows fail on main's `quantityOf` (21 with a model over the three
versions, 7 without, and the metamorphic test without a model) and pass on
the branch.

**A test of the old rule, found by the mutation run.** The subset run marked
every mutant caught, V10 (equivalent) included, by
`TestCalendarDurationOrdering`. That root-package test failed on the branch
without any mutation: it held "a quantity without a unit takes the other's",
`Observation.value < 1 year` over `{"value": 2}` false. The implementation
step had run `types`, `eval`, `funcs` and `conformance` but not the root
package. Under 026 the value is `2 '1'`, and `2 '1' < 1 year` is empty, the
planned change (plan §3, `u < 2 'mg'`); the test now expects empty. The
whole module passes, and the subset runs again (`mut/history026sub2.txt`).

Rerun (`mut/history026sub2.txt`, 2,033 s): 99 of 101 caught. V10 survives,
equivalent as before. **M39** (`Compare` orders exactly only in the same
unit, not where one has no unit) survives: since 026 no path of the engine
makes a `""` unit, so the two conditions agree on everything the engine
builds. The exported constructors still can, so
`TestAQuantityWithNoUnitTakesTheOthers` (`types`) pins the contract from the
public API: 1e1000 with no unit orders above `2 '1'`, exactly. It fails
under M39 and passes on the branch.

### Code review round 2 (Codex)

R1–R3 closed; the `TestCalendarDurationOrdering` change follows §3 and hides
no regression. One low finding: the no-model run used R4's cases only, so
R5's rng-2 never ran without a model; it now runs R4's and R5's. **Round 2
converges:** no finding of medium severity or higher.

## 9. Closing

- CONFORMANCE.md: the rule (unit `'1'`, an empty code or unit absent) in "A
  divergence taken on purpose: a FHIR Quantity maps by its code or unit";
  "still converts, in the unit `'1'`" in "Two FHIR Quantities are equivalent
  as quantities"; two rows in "Measured, and not followed" (unitless against
  a number and another unitless one, FP-27 included; JAVA-18).
- `docs/upstream-findings.md`: FP-27 decided; FP-28, FP-29 and JAVA-18 open.
- Pending, outside 026: FP-28 (membership and `distinct()` keep a Quantity's
  `id`), FP-29 (`quantityTypes` by hand), a nested unitless Quantity (plan
  025 C2).
