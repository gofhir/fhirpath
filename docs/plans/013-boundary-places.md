# 013 — A decimal's boundaries at 16 or more places (#244)

## Problem

`lowBoundary()` and `highBoundary()` place a decimal half a unit of its last
written place away from it, and round that out to the precision asked for.
`funcs.boundaryInterval` computes the half unit as `10^-p / 2` through a
division rounded to 16 places (`types.DivisionPlaces`). With 16 places or
more in the input, the half unit (5e-17 or less) rounds to 1e-16 or to 0.
The boundary then moves the wrong distance, or equals the input.

This is not confined to precisions above the default:

- `(1.00000000000000000).lowBoundary()` gives `1.00000000`. The least value
  the input stands for is 0.999999999999999995, so the result should be
  `0.99999999`, as fhirpath.js and Java give.
- `(1.00000000000000000).lowBoundary(16)` gives `1.0000000000000000`
  (Java: `0.9999999999999999`).
- `(1.1234567890123456).lowBoundary(17)` gives `1.12345678901234550`
  (exact: `…555`).

FHIRPath 3.0.0, `lowBoundary`: "The least possible value of the input to the
specified precision"; `highBoundary`: "The greatest possible value". Its
examples round outward: `1.587.lowBoundary(2) // 1.58`,
`1.587.highBoundary(2) // 1.59`. "If the precision is greater than the
maximum possible precision of the implementation, the result is empty." R4
and R5 carry the function alike (it is FHIRPath's, not FHIR's); 2.0.0 does
not define it.

## Prior review

- **gofhir/server (frozen):** no boundary functions. Nothing to take.
- **Measured** (`make three-engine BATTERY=batteries/boundary-places/battery.tsv`,
  fhirpath.js 5.2.0 and HL7 validator 6.10.2; 352 cases, R4 and R5 alike):
  204 agree. In 92 this engine alone differs: 88 at precision 17 and 18,
  where both references answer empty, and 4 from the half unit. In 42
  fhirpath.js alone differs, in 4 Java alone, and in 10 all three differ.
  - **Maximum precision.** fhirpath.js answers up to 8 places and empty
    from 9 (`FP_Decimal.MAX_PRECISION = 8`, src/types.js:3145; run:
    `(1.587).lowBoundary(9)` is empty). Java answers up to 16 and empty
    from 17 (`precision < 17` in `FHIRPathEngine.funcLowBoundary`). This
    engine goes to 18 (CONFORMANCE.md, "Max precision for
    lowBoundary/highBoundary"). The references split from 9 to 16; their
    agreement on empty at 17 and 18 is two unrelated constants, not a
    shared answer. The battery measured 8 and 16 only, which hid this.
  - **The half unit.** Wherever an input has 16 or more places, the
    references move it the exact half unit and this engine does not:
    `(1.00000000000000000).lowBoundary()`, both `0.99999999`.
  - **Where the references split, the exact half unit decides.**
    `(1.00000000000000000).highBoundary()`: fhirpath.js `1.00000001`, Java
    `1.00000000`; the greatest value is 1.000000000000000005, rounded
    outward `1.00000001`. `(123.4567890123456789).highBoundary()`: Java
    `123.45678901`, fhirpath.js and here `123.45678902`.
    `(0.00000000000000001).highBoundary(16)`: Java `0.0000000000000000`,
    below the input; here `0.0000000000000001`. Java builds the boundary in
    text and rounds it half up rather than outward (`applyPrecision`), which
    accounts for these, and fails outright on
    `(123.4567890123456789).highBoundary(16)`. fhirpath.js writes `1e-8`
    for `0.00000001`: the same value, a JavaScript number's form.

## Plan

1. `boundaryInterval(p)` is `decimal.New(5, -(p+1))`: exactly half a unit
   of the p-th place, with no division. This reads no setting and needs no
   `DivisionPlaces`.
2. **The maximum precision: kept at 18** (user's decision, 2026-10-28).
   - **B, keep 18 (chosen):** the references split (8 and 16), so the
     text decides, and it leaves the maximum to the implementation. Java's
     16 is a constant with no stated reason, and its values at 16 are not
     exact (its boundaries round half up, not outward). After step 1 this
     engine's values at 17 and 18 are exact. The 88 cases go under
     "Measured, and not followed", with both constants. CONFORMANCE.md's
     reason is corrected: FHIR's 18 is digits in all, not places.
   - **A, 16 (Java's):** it would agree with Java's cutoff, though not with
     its values, and `(1.1234567890123456).lowBoundary(17)` would become
     empty rather than right.
3. **A decimal written with an exponent** (FP-5, open): `ImplicitPrecision`
   counts the characters after the point, exponent included, so a
   resource's `1.5e2` takes a half unit of 0.0005 and `1E-17` one of 0.5,
   while the Quantity path reads the exponent. It is the input's precision,
   not the half unit, and it changes `precision()` too. **Out of scope:
   issue #246** (user's decision), and the exit criterion says so.
4. CONFORMANCE.md: the half unit, and which inputs change (Fable measured
   760,000 pairs: none below 16 places; at 16 only for precisions 17 and
   18; from 17 places at any precision, `6.20086127000000000.lowBoundary()`
   6.20086127 → 6.20086126). Java's answers below the greatest value, its
   error, and both references' maxima under "Measured, and not followed";
   Java's error and rounding as JAVA-n in docs/upstream-findings.md.

## Tests (first)

- **The battery is committed** as `difftest/batteries/boundary-places/`, so
  the measurement is reproducible with one command. Added: precisions 0 and
  9 to 15, integers, zero, `-0.0`, trailing zeros, a value just below zero,
  computed values, and values read from a resource and from a Quantity.
- **`TestDecimalBoundaryAtManyPlaces`** (funcs or root), each row citing its
  source:
  - where both references agree, their answer
    (`(1.00000000000000000).lowBoundary()` → `0.99999999`);
  - where they split, the text-backed one, with the exact value written in
    the comment;
  - the spec's own examples at 6, 2 and 0 places, unchanged.
- **A property test against exact arithmetic.** Random decimals of 0 to 30
  places, positive and negative, integers and zero among them, written as
  text; the places are counted from that text, not by the engine. For each
  precision 0 to the maximum:
  - the low boundary is the floor of value − 5·10^-(p+1) at the precision
    asked, and the high the ceiling of value + 5·10^-(p+1), with floor and
    ceiling by `big.Int.DivMod`, which floors negatives;
  - independently of that formula, low ≤ value − half < low + 10^-q, and
    symmetrically for high;
  - the result is written at q places exactly (the rendered scale).

  Through the literal, the Integer path and a Quantity.
- **Mutations:** the interval through a division again; the interval a
  place off; floor and ceiling swapped; the maximum off by one.

## Validation

- The battery, after the change: every remaining difference classified.
- corpusdiff R4, R4B, R5 against main; `make check`; the mutation sets of
  010, 011 and 012, plus this one's.

## Out of scope

- Temporal boundaries: unchanged.
- The rounding of `-1.587.lowBoundary()` and the other suite disagreements
  CONFORMANCE.md already settles.

## Exit criterion

For every decimal written without an exponent, and every precision up to
the maximum, both boundaries equal the exact computation (property test),
through a literal, an Integer and a Quantity; the battery's remaining
differences are each cataloged with their reason. Decimals written with an
exponent are #246's.

## Dependencies

None new.

## Measurement after (2026-10-28)

`make three-engine BATTERY=batteries/boundary-places/battery.tsv`, 832
cases, R4 and R5 alike (no case answers differently by version). Every
difference is one of:

- **Past a reference's maximum** (JS-9: 8; JAVA-12: 16). Where this engine
  alone differs, every case is this, and the answer here is exact.
- **fhirpath.js counts a JavaScript number's places** (JS-8): a negative
  literal (`(-1.0).lowBoundary()` `-1.5`), a computed value
  (`(1.5 + 0.00000000000000000)` `1.45`), a resource's
  `1.00000000000000000` (`0.5`).
- **fhirpath.js has no Quantity boundaries** (JS-10), already recorded.
- **Java rounds half up** (JAVA-9), **errs** (JAVA-10), **widens a zero**
  (JAVA-11), and reads `(1.00 / 3)` with fewer places.
- `(-0.0).lowBoundary()`: both references `-0.5`, from JS-8 and JAVA-11, two
  unrelated defects (probe: `(0.0)` gives `-0.05` in fhirpath.js and `-0.5`
  in Java). Here `-0.05000000`, exact.

Which inputs change, measured again after the fix over 400,000 random pairs
against the former interval (Fable's 760,000 in the plan round, and 800,000
in review round 1, agree): none of 15 places or fewer; at 16 places, only at precision 17 and
18; from 17 places, at any precision.

The new tests take 0.58 s and 31 MB.

## Mutations (2026-10-28)

Eleven defects injected one at a time in a copy of the repo, root and
conformance suites run; all caught:

| # | Mutation | Caught by |
|---|---|---|
| F1 | The half unit through a division again | `TestDecimalBoundariesAreExact`, `TestDecimalBoundaryAtManyPlaces` |
| F2 | The half unit a place short | the two, `TestFHIRQuantityBoundaries` |
| F3 | The half unit a place long | the two, `TestFHIRQuantityBoundaries` |
| F4 | The low boundary by ceiling | the two, `TestIntegerBoundariesAreExact`, `TestLowBoundary` |
| F5 | The high boundary by floor | the two, `TestHighBoundary` |
| F6 | The maximum 17 | the two, `TestIntegerBoundariesAreExact` |
| F7 | The maximum 19 | `TestADecimalBoundaryPastTheMaximumIsEmpty` |
| F8 | An Integer read as one place | `TestIntegerBoundariesAreExact`, `TestFHIRQuantityBoundaries` |
| F9 | A Quantity's places one more | `TestDecimalBoundariesAreExact`, `TestFHIRQuantityBoundaries` |
| F10 | The result unpadded | the two, `TestFHIRQuantityBoundaries` |
| F11 | The high boundary rounded half up, as Java | the two, `TestIntegerBoundariesAreExact`, the official suite |
| F12 | An Integer read through a float64 (review round 1) | `TestDecimalBoundariesAreExact`, `TestIntegerBoundariesAreExact`; survived before the generator was widened |
| F13 | A value past 10^7 capped at 15 places (review round 1) | `TestDecimalBoundariesAreExact`; survived before the generator was widened |

## Adversarial round (2026-10-28, Fable and Opus)

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | fhirpath.js's maximum is 8, not 16: the references split from 9 to 16, and their agreement at 17 and 18 is two constants | both, high | **Accepted**, confirmed in its source and by running. The prior review is rewritten; the recommendation moves to B |
| 2 | Java's values at 16 are not all exact (half up, not outward; an error), so "borne out" was wrong | both, medium | **Accepted.** Java's rounding and error go to upstream-findings |
| 3 | A decimal with an exponent read from a resource gets the wrong half unit (FP-5) | both, medium | **Out of scope, issue #246** (user's decision); the exit criterion is narrowed to say so |
| 4 | Which results change, measured over 760,000 pairs | Fable, medium | **Accepted.** Written into CONFORMANCE.md and the PR; corpusdiff checked for it |
| 5 | The property oracle must count places from its own text, floor negatives by DivMod, check a fit independent of the formula and the rendered scale, and reach Integer and Quantity | both, medium | **Accepted** |
| 6 | Computed values change too, `(1.5 + 0.00000000000000000).lowBoundary()` 1.50000000 → 1.49999999 | Opus, low | **Accepted**: battery rows, and the PR names it |
| 7 | FHIR's 18 is digits in all, not places | Fable, low | **Accepted** in CONFORMANCE.md's row |
| 8 | No existing test pins the defect, so the division mutation needs the new test | Opus, low | **Accepted**: mutation listed |

## Code review

### Round 1 (2026-10-28, Fable and Opus)

| # | Finding | From | Resolution |
|---|---------|------|------------|
| R1-1 | The generators stay below 10^6 and always pass a precision: an Integer read through a float64 and a cap of 15 places past 10^7 survive | Opus, medium | **Confirmed** (F12, F13 survive the round-1 tests), **fixed**: integer parts up to 18 digits, Integers to 2^63−1, one call in five with no precision; both now caught. Still 0.53 s, 31 MB |
| R1-2 | The battery's header says 17 places; `1.123456789012345678` has 18 | Fable, low | **Confirmed**, fixed |
| R1-3 | 760,000 and 400,000 pairs both stated | Fable, low | Both measured (plan round, after the fix); the text now says which is which |
| R1-4 | A Quantity quotient loses its places: `(2.0 'mg' / 2)` is `1 'mg'`, `lowBoundary()` `0.50000000 'mg'`, where `(2.0 / 2)` gives `0.99999999` | Fable, low | **Confirmed**, predates this change: FP-12, out of scope |
| R1-5 | The generator draws no computed or resource values; one table row each | Fable, low | Accepted as is: their places reach the same `decimalBoundary` the property test covers (F9, F13), and the rows pin the paths |

Converges on findings: none of medium or more left after R1-1's fix. Round 2
on the changed tests.

### Round 2 (2026-10-28, Fable and Opus)

Given only round 1's findings, the changes and their diff. Both measured the
widened generator (Fable: of 3000 decimals, 101 with 18-digit integer parts,
706 past 10^7 with more than 15 places, 590 on the default) and ran 22
further mutants between them, the two round-1 survivors among them: all
killed.

| # | Finding | From | Resolution |
|---|---------|------|------------|
| R2-1 | `randomPrecision`'s comment says "none (-1)"; it returns 8 and no argument | both, low | **Confirmed**, fixed |
| R2-2 | −2^63 and −(2^63−1) are never drawn | both, informational | Accepted as is: `-9223372036854775808` is unary minus on a Decimal, and its boundary was measured right |

**Converged:** no finding of medium or more in round 2.

## Close (2026-10-28)

`make check` green; mutations 013 13/13, 012 21/21, 011 12/12, 010 50/50;
corpusdiff BASE=main 0 differences; go-validator green, go-searchindex only
its 4 known tripwires. Left open: #246 (FP-5), FP-11, FP-12; JS-8, JAVA-9 to
JAVA-11 unpublished.

