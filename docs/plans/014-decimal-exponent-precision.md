# 014 — A decimal written with an exponent: its precision (#246)

## Problem

`types.Decimal.ImplicitPrecision` counts the characters after the point in
the text as written, the exponent included, and none where there is no
point. `precision()` and the boundaries of a Decimal read it. A Quantity's
boundaries read the decoded value's exponent instead (`quantityPrecision`),
so the same text gets two answers:

| Value in the resource | `value.value.precision()` | `value.value.lowBoundary()` | `value.lowBoundary()` (Quantity) |
|---|---|---|---|
| `1.5e2` | `3` | `149.99950000` | `149.50000000 'mg'` |
| `1E-17` | `0` | `-0.50000000` | `0.00000000 'mg'` |
| `15e-1` | `0` | `1.00000000` | `1.45000000 'mg'` |
| `1.0e0` | `3` | `0.99950000` | `0.95000000 'mg'` |

`-0.50000000` is no boundary of 1E-17 at any reading, and 15e-1, which is
1.5 written to one place, is bounded as if it were written `2`.

A FHIRPath literal cannot carry an exponent ("Decimal literals cannot use
exponential notation", 3.0.0), and `toDecimal()` refuses one (the String
form is `(\+|-)?\d+(\.\d+)?`). The text reaches the engine from a resource:
FHIR's decimal is "A JSON number" with the regex
`-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?` (R4; R4B alike; R5 limits
the digits, SPEC-5), and through the Go API (`types.NewDecimal`).

## Prior review

- **Specification.**
  - FHIRPath 3.0.0 (published, hl7.org/fhirpath/STU3), `precision()`: "For
    Decimal values, the function returns the number of digits of precision
    after the decimal place in the input value", with
    `100.precision() // 0`; "The function can only be used with Decimal,
    Date, DateTime, and Time values". `lowBoundary()`: "The least possible
    value of the input to the specified precision". The development build
    (HL7/FHIRPath master, `input/pages/index.md`) adds Quantity to both:
    `(1.00 'mg').precision() // 2`.
  - FHIR datatypes (R4, R4B, R5): "The precision of the decimal value has
    significance ... the original precision should be preserved". The
    exponent is in the regex only; no prose.
  - FHIR search, number: "The number of significant digits of the
    implicit range is the number of digits specified in the search
    parameter value", "So 100 and 1.00e2 both have the same number of
    significant digits - three". `1e2` is "to 1 significant figures
    precision": R4 and R4B give its range as "[95 ... 105)", which is not
    one significant figure; R5 corrects it to "[50 ... 150)" (measured on
    the three pages). By that rule `1.5e2` has two significant figures,
    [145, 155). It is search's rule, not FHIRPath's, but it is FHIR's only
    statement of what an exponent says about precision.
- **gofhir/server (frozen):** `calculatePrecision` (MongoDB number search)
  counts the characters after the point, exponent included: the same
  defect. **Discard.**
- **Measured** (`make three-engine BATTERY=batteries/decimal-exponent/battery.tsv`,
  fhirpath.js 5.2.0 and HL7 validator 6.10.2; 15 values × 8 expressions,
  R4 and R5 alike, no case differs by version):
  - **fhirpath.js** reads a JSON number, so the exponent is applied and the
    places are the number's: `1.5e2` → `149.5`, `1.234e1` → `12.335`,
    `15e-1` → `1.45`, `1.5e-2` → `0.0145`, `1E-17` → `0` and `1e-8`. A
    trailing zero is lost with it, `1.0e0` → `0.5` (JS-8). No `precision()`
    (JS-7).
  - **Java** reads the text and ignores the exponent: `precision()` is the
    mantissa's places (`1.5e2` 1, `1.234e1` 3, `2.5e-20` 1); its boundaries
    splice the exponent's digits into the result (`1.5e2` → `1.450000002`,
    `1.5E+2` → `1.5E+15000`); a negative exponent ends the evaluation; and
    `Observation…value = 150` is false over `1.5e2` (JAVA-13).
  - **This engine's Quantity path** already reads the decoded scale and
    answers as fhirpath.js does wherever fhirpath.js keeps the value.
  - The references never agree on these inputs, and neither answers from
    the text with its exponent applied: fhirpath.js has lost the text (its
    places are a double's shortest print: `1.50e-1` is `0.15`, two places;
    `1.0e0` is `1`), and Java ignores the exponent (it agrees with any
    reading only where the exponent is 0, as at `1.0e0`, `0.950000000`).
    So the rule here is the engine's reading of the texts above, not a
    reference's answer.
  - `precision()` over a Quantity (development build only): empty here and
    in Java, an error in fhirpath.js. Separate (FP-13).

## Plan

1. **The place of the last written digit**, `k`, is read once for every
   Decimal and serves `precision()`, a Decimal's boundaries and a
   Quantity's (`quantityPrecision` calls the same function instead of
   keeping its own copy):
   - for a value read from text, from that text: the mantissa's digits
     after the point minus the exponent, computed in int64 (an exponent past
     int32 is not read, FP-6, so this cannot overflow; `1e-2147483648` gives
     2147483648, where `int(-exponent)` on the int32 scale would give
     −2147483648). The text, not the decoded scale, because past the bound
     the decoder caps the scale at 1000 places (`withinScale`,
     `normalizedZero`: `1.` with 1001 zeros, and `0e-2000`) and the engine
     "holds it as written" (CONFORMANCE.md);
   - for a computed value, from its scale, widened before negating.

   `1.50` 2, `1.5e2` −1, `1e2` −2, `1E-17` 17, `15e-1` 1, `1.0e0` 1,
   `1.234e1` 2, `1.50e-1` 3. For a text without an exponent, `k` is the
   count of characters after the point that the engine gives today.
2. **`precision()` is `max(0, k)`**, an Integer as 3.0.0 has it: "the
   number of digits of precision after the decimal place" (`1.5e2` 0,
   `1E-17` 17). Past the bound it answers from the text (`1e-1001` 1001),
   which computes nothing.
3. **Decision D1, the boundaries when `k` < 0** (`1e2`, `1.5e2`). **Decided: C** (user, 2026-10-29):
   - **C (recommended): the half unit of the last written digit,**
     5 × 10^-(k+1) for every `k`: `1e2` stands for [50, 150] and `1.5e2`
     for [145, 155], as FHIR search's rule and R5's range have it, and as
     "SHALL handle decimal values in ways that preserve and respect the
     precision of the value" asks; `1.00e2` stays [99.5, 100.5], as `100`.
     `precision()` still says 0: it counts places after the point only.
   - **A: clamp at 0 places,** a half unit of 0.5 for any `k` < 0: `1e2`,
     `1.5e2`, `1.50e2` and `150` all [x − 0.5, x + 0.5], claiming more
     precision than was written. fhirpath.js answers so, having lost the
     text.
   - B (a negative `precision()`) is dropped: 3.0.0's `precision()` counts
     digits after the point.
4. **Decision D2, `precision()` over a Quantity** (FP-13): only the
   development build defines it; the published 3.0.0 excludes Quantity, and
   neither reference implements it. **Decided: issue apart** (user,
   2026-10-29): #249.
5. CONFORMANCE.md: a section on the precision of a decimal with an
   exponent, Java's answers under "Measured, and not followed".
   docs/upstream-findings.md: JAVA-13, JS-8 widened (`1.0e0`, `1.50e-1`),
   FP-5 fixed, FP-13, FP-14 (`toString()` writes `1.5e2`, outside the
   Decimal form `toDecimal()` reads back). The godoc of `ImplicitPrecision`
   is rewritten.

## Amendment in implementation (2026-10-29)

Step 1 read `k` once, from the text, for `precision()` and the boundaries.
Implementing it showed that the boundaries cannot take it from the text: a
Quantity holds no text, only the decoded scale, so a zero written past the
bound (`0e2000`, normalized to 0) would be bounded by 5 × 10^1999 as a
Decimal and by 0.5 as a Quantity. So:

- `precision()` answers from the text (`Decimal.LastPlace`): what was
  written, as `toString()` does, past the bound included;
- the boundaries compute with the value as held, its scale (`heldPlace`),
  through a Decimal and a Quantity alike, as every operation computes:
  `0e2000` is bounded as 0 is, as CONFORMANCE.md already has it ("computes
  as 0 does"). The scale of a value within the bound stays within int32.

The text and the scale differ only where the decoder caps the scale past
the bound: a zero written past it (`0e2000`, `0e-2000`) and trailing zeros
past 1000 places (`1.` with 1001 zeros: `precision()` 1001, bounded at
place 1000, which rounds alike at the 18 places a boundary takes). D1's
answers are unchanged.

### Second amendment, after review round 2 (2026-10-30, decided by the user)

Round 1's fix (R1-1) told a written value from a computed one by the type
that wraps it: a Decimal with text, a Quantity read from a resource. Round
2 showed that this makes an operation that keeps the value change its
boundary (R2-1). The rule now lives in the value: **every value is bounded
by its held scale** (`heldPlace`, no written flag), and **every
computation holds its result at the units or finer**
(`decimals.Computed`: float results of `power()`, `sqrt()`, `exp()`,
`ln()`, `log()`; sums, differences and products of Decimals and
Quantities; products and quotients of two Quantities (UCUM, in float);
`mod`). `div`'s quotient is held at the units by `QuoRem` already. The value is unchanged, only its scale. A
positive exponent is then only ever a digit written left of the point, so
negation, `abs()`, `toQuantity()` and selection keep it, and a computed
value is bounded at the units as before. Options weighed: carrying a
written flag on Quantity and through every conversion (more state to keep
in step), or bounding at the units always (reverses D1).

## Tests (first)

- **The battery is committed**, `difftest/batteries/decimal-exponent/`
  (240 cases), and widened: a negative exponent with trailing zeros
  (`1.50e-1`), upper and lower case `e`, `+` sign, a zero mantissa with an
  exponent (`0.0e5`, `0e-3`), an exponent past the places on both sides of
  zero, values past the bound (`1e-1001`, `1.` with 1001 zeros,
  `0e-2000`, `1e-2147483648`), a value under `valueDecimal`
  (`RiskAssessment.prediction.probabilityDecimal`), not only a Quantity's
  `value`. R4 and R5 both, as every battery: the engine reads a JSON
  number's bytes alike in each, which the run confirms rather than assumes.
- **A property test against independent arithmetic.** Random texts in
  FHIR's decimal form, with and without an exponent (mantissa places m,
  exponent e, both signs, trailing zeros), read from a resource:
  - `precision()` is `max(0, m − e)`, counted from the generated parts,
    past the bound included;
  - within the bound, `lowBoundary(q)`/`highBoundary(q)` of the Decimal
    equal plan 013's exact oracle with a half unit of 5 × 10^-(m−e+1), and
    equal the same value's Quantity boundaries; past it, empty;
  - a text without an exponent gives the precision it gave before, the
    characters after the point.
- **Table test**, rows citing their source: 3.0.0's
  `1.58700.precision() // 5` and `100.precision() // 0`; FHIR search's
  `100`, `1.00e2` and R5's `1e2` ranges; fhirpath.js's values where it
  keeps the text's places (`1.234e1` → `12.33500000`, `15e-1` →
  `1.45000000`); the exact value elsewhere; and through `types.NewDecimal`
  for the Go API.
- **Mutations:** the text count again; the exponent ignored (Java's
  reading); the exponent's sign flipped; `precision()` not clamped at 0;
  the boundary clamped at 0 (option A); the scale used past the bound; the
  int32 negation; the Quantity path apart again.

## Validation

- The battery after the change, every difference classified.
- corpusdiff R4, R4B, R5 against main (no official example is expected to
  change: to be measured); `make check`; mutation sets 010–014; downstream
  (searchindex, validator).

## Out of scope

- `toString()` of such a value keeps the text as written (`1.5e2`), as Java
  answers; fhirpath.js writes `150`. That is outside the Decimal form
  `toDecimal()` reads (`'1.5e2'.toDecimal()` empty): FP-14, apart.
- `precision()` over a Quantity (D2, FP-13), FP-11, FP-12, #238.

## Exit criterion

For every decimal in FHIR's form, read from a resource or built through the
API, past the bound included, `precision()` is the places after the point
of its value with the exponent applied, never below 0, and within the bound
its boundaries are the exact ones for its last written digit (as D1
decides), through a Decimal and a Quantity alike (property test); the
battery's differences are each cataloged with their reason.

## Dependencies

None new.

## Measurement after (2026-10-29)

`make three-engine BATTERY=batteries/decimal-exponent/battery.tsv`, 416
cases (24 values under a Quantity's `value`, 4 under
`RiskAssessment.prediction.probabilityDecimal`), R4 and R5 alike. Every
difference is one of:

- **Java ignores the exponent** (JAVA-13): its `precision()` is the
  mantissa's places, its boundaries splice the exponent's digits in, a
  negative exponent errs, `1.5e2 = 150` is false.
- **fhirpath.js has lost the text** (JS-8): `1.5e2` bounded as 150, `1e2`
  as 100; no `precision()` (JS-7), no Quantity boundaries (JS-10).
- **Where this engine alone differs**, a zero written with an exponent:
  `0.0e5` ±5000, `0e-3` ±0.0005, `0e-2000` ±0.00000001 at 8 places; both
  references ±0.5, from JS-8 and JAVA-11. D1 decides.

The new tests take 0.45 s and 17 MB (`-run Exponent`, built binary).

After round 3 (2026-10-30), the same command over the battery widened with
round 2's and 3's rows (operations over a resource's `1.5e2` that keep the
value or compute a new one): the earlier rows answer as before, and the
computed Decimal rows agree with fhirpath.js (`value * 2`, `value + value`,
`value mod value`). The validator bounds them as `3.0E+15000` (JAVA-13,
widened); neither reference bounds a Quantity there.

## Mutations (2026-10-29)

One at a time, root and conformance suites (`mut/mutate014.py`, rerun with 010–013's sets before each review round and after round 3, 2026-10-30); all caught. G14–G16 of round 1 were replaced by the second amendment's:

| # | Mutation | Caught by |
|---|---|---|
| G1 | The text count again, exponent included | the rows, the property test, the API test |
| G2 | The exponent ignored (Java's reading) | the rows, the property test, the API test |
| G3 | The exponent's sign flipped | the rows, the property test, `TestADecimalBeyondTheBoundIsHeldNotComputed` |
| G4 | `precision()` not clamped at 0 | the rows, the property test |
| G5 | The boundary clamped at 0 places (option A) | the rows, the property test |
| G6 | `precision()` from the scale, past the bound too | the rows (`1.` with 1001 zeros, `0e-2000`) |
| G7 | The text's place truncated to int32 | the rows (`1e-2147483648`) |
| G8 | The Quantity bounded apart again | the property test |
| G9 | The Decimal bounded from its text | timeout: `0e-2147483648` (the reason for `heldPlace`) |
| G10 | A lower-case `e` only | the rows, the property test, the API test |
| G11 | The half unit a place off | 013's and 014's tests |
| G12 | A computed value without places | `TestComputedDecimalPrecision` |
| G13 | The exponent read in base 0 | the rows, the property test |
| G14 | `decimals.Computed` keeps a positive exponent | `TestComputedDecimalBoundariesStayAtTheUnits`, `FuzzFormatAndNatural` |
| G15–G21 | Each place a result is held left as made: float results, Decimal product, sum, difference, Quantity product, sum, difference | `TestComputedDecimalBoundariesStayAtTheUnits` |
| G23 | `mod`'s remainder as made | `TestComputedDecimalBoundariesStayAtTheUnits` |
| G24–G27 | Decimal and Quantity negation and `abs()` drop the written place | `TestWrittenExponentSurvivesValuePreservingOperations` |
| G28 | A UCUM product or quotient as made (R3-1) | `TestComputedDecimalBoundariesStayAtTheUnits` |

G22 (`div`'s quotient) was dropped as equivalent: `QuoRem(·, 0)` holds it at
exponent 0 (shopspring/decimal 1.6.0), so the wrap was removed.

## Adversarial round (2026-10-28, Fable and Opus)

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | R5's search page corrects `1e2` to "[50 ... 150)"; SPEC-6 holds for R4 and R4B only, and D1's support rested on it | Opus, high | **Accepted**, measured on the three pages. SPEC-6 records R4 and R4B's row, corrected in R5; D1 rewritten with option C |
| 2 | D1 lacks the option that keeps `precision()` an Integer and bounds by the last written digit (C); A claims more precision than written | Opus, medium | **Accepted**: C added and recommended |
| 3 | The rule `int(-exponent)` overflows: `1e-2147483648` is readable and would give −2147483648 | Fable, high | **Accepted**: `k` from the text in int64 |
| 4 | Past the bound the decoded scale is capped at 1000 (`1.` with 1001 zeros, `0e-2000`): "a text without an exponent gives the count it gave" was false, and the oracle `max(0, m − e)` with it | both, medium | **Accepted**: `k` from the text for a read value; the property test covers past the bound |
| 5 | "No engine answers so" understated: neither reference answers from the text with its exponent, so A is not "fhirpath.js's" but a double's print | Fable, medium | **Accepted**: the prior review says the rule is the engine's reading |
| 6 | The published 3.0.0 excludes Quantity from `precision()` and the boundaries; `(1.00 'mg').precision() // 2` is the development build's | Fable, medium | **Accepted**, measured on hl7.org/fhirpath/STU3 and master; D2 and FP-13 cite the build |
| 7 | `precision()` past the bound would answer 1001 for `1e-1001` | Opus, low | Accepted and stated: answered from the text, which computes nothing |
| 8 | `toString()` writes `1.5e2`, which `toDecimal()` does not read back | Fable, low | **Accepted** as FP-14, out of scope |
| 9 | The battery has no `valueDecimal`; Java's `1.0e0` agreement is a coincidence; fhirpath.js's places are a double's print (`1.50e-1` → 0.15) | both, low | **Accepted**: battery widened; the table cites fhirpath.js only where it keeps the text's places |
| 10 | The R5 half of the battery measures nothing new: one reader for every version | Fable, low | **Rejected**: the run confirms it at a cost of minutes, and the process asks for every version from the first test |
| 11 | No other reader of `ImplicitPrecision`; `~` and `toString()` unaffected; computed values unchanged | both, low | Confirmed, nothing to do; the godoc is rewritten |
| 12 | Java's `1.0e0` boundary is `0.950000000`, 9 places | Fable, low | **Accepted**, corrected |

## Code review

### Round 1 (2026-10-29, Fable and Opus)

| # | Finding | From | Resolution |
|---|---------|------|------------|
| R1-1 | `heldPlace` took a computed value's positive exponent for a written place: `(100.0).power(2).lowBoundary()` 9999.5 → 5000, `(22500.0).sqrt()` 149.5 → 145, a resource's `1.5e2 * 2` 299.5 → 295; adversarial #11's "computed values unchanged" was false | Fable, high | **Confirmed** (probe against main), **fixed**: only a written value (a Decimal with text, a Quantity read from a resource) has a digit left of the point; a computed one is bounded at the units, as on main and as fhirpath.js answers each row (measured). New test and mutations G14–G16 |
| R1-2 | The computed branch of `LastPlace` had no test (G12 survived), and its comment's reason was stale | Opus, medium | **Confirmed**, fixed: `TestComputedDecimalPrecision`, rows from Java (3, 2, 3, 2); comment rewritten |
| R1-3 | No exponent with leading zeros: `ParseInt` in base 0 (G13) survived; `1e-08` would give 0 | Opus, medium | **Confirmed**, fixed: the generator pads exponents; rows `1e-08`, `2.50E-007`, `1.5e+02` (fhirpath.js agrees); battery widened |
| R1-4 | A zero with a large positive exponent within the bound is bounded as written (`0e999` ±5e998), past it as 0 (`0e1001` ±0.5) | both, medium/low | **Accepted as decided**: D1 within the bound, #232's "computes as 0 does" past it; now stated in CONFORMANCE.md |
| R1-5 | `precision()` clamped to `math.MaxInt` fails the `1e-2147483648` row on 32-bit | Opus, low | **Fixed**: `precision()` computed in int64 from `LastPlace` |
| R1-6 | The amendment's "only zeros past the bound" omits trailing zeros past 1000 places | both, low | **Fixed** in the text |
| R1-7 | FP-5's status cited the issue as a fix; the resume point was stale | Fable, low | **Fixed** |
| R1-8 | `LastPlace` exported with one caller | Fable, low | Rejected: `fnPrecision` (package funcs) reads it as well, as `IsWritten` is read there |
| R1-9 | `int64(-d.Exponent())` survives | Fable, low | Rejected as equivalent: the int32 minimum is reached only by a normalized zero |
| R1-10 | FP-15: a quotient's `precision()` is 16, the validator's 2 | probe, low | Recorded, out of scope (predates the change) |

### Round 2 (2026-10-30, Opus and Fable)

| # | Finding | Who, severity | Resolution |
|---|---|---|---|
| R2-1 | Whether a value is written lived in the wrapping type, so a value-preserving operation changed the boundary of a resource's `1.5e2`: `(-value.value).lowBoundary()` −155 but `(-value).lowBoundary()` −150.5 'mg'; `value.value.abs()` 149.5; `value.toQuantity()` 149.5 'mg' against `value.lowBoundary()` 145 'mg' | both, medium | **Confirmed** (probe), **fixed** by the second amendment: all give 145 / −155. `TestWrittenExponentSurvivesValuePreservingOperations`; mutations G24–G27 |
| R2-2 | The System-Quantity path (`written=false`) had no test: a mutant bounding it by its scale survived and brought R1-1 back on `((100.0).power(2) * 1 'mg')` | both, medium | **Confirmed**, **fixed**: computed-Quantity rows in `TestComputedDecimalBoundariesStayAtTheUnits` (product, sum, difference, `div`, `mod`, power); mutations G14–G23, one per place a result is held |
| R2-3 | Bounding by an unbounded place could hang rather than fail | Fable, low | Not reachable: a value within the bound holds a scale within int32, and `boundaryInterval` takes it there; the rows `1e-2147483648` and `0e-2147483648` run in milliseconds |
| R2-4 | The `abs()` row of the computed-values test was fhirpath.js's 149.5, which bounds a written `1.5e2` at the units as well (JS-8) | review of the fix | **Moved** to the value-preserving test, expected from D1 (R5 search) |


### Round 3 (2026-10-30, Opus and Fable): the last allowed

| # | Finding | Who, severity | Resolution |
|---|---|---|---|
| R3-1 | A Quantity × Quantity or ÷ Quantity result (`internal/ucum` `combine`, float) kept `NewFromFloat`'s positive exponent: `(100 'm' * 100 'm').lowBoundary()` 5000 'm2', main 9999.5 — a regression from removing `quantityPrecision`'s clamp | both, Opus medium, Fable high | **Confirmed** (probe), **fixed**: `combine` holds its result with `decimals.Computed`. Rows in `TestComputedDecimalBoundariesStayAtTheUnits` (products, a quotient, a written value by `1 'm'`); mutation G28 |
| R3-2 | The float path of `ucum.Convert` (logarithmic units) unwrapped as well | Opus, unprobed | Rejected with evidence (Fable): `valueIn` passes it through `Natural`, so `(10 'B').toQuantity('dB').lowBoundary()` is 99.5 'dB' (row added) |
| R3-3 | `heldPlace`'s comment said a converted value keeps its place; to another unit it is computed anew (`toQuantity('ug')` of 1.5e2 'mg' 149999.5) | both, low, as on main | **Fixed** in the comment; the loss predates the change |
| R3-4 | `Decimal.Round` (public API, unused by the engine) returns a positive exponent for a negative precision | Fable, low | Out of scope |

Round 3 left one confirmed finding of medium or higher, so by the process the
review does not converge; its fix is a single site and is verified. The
user decided (2026-10-30) to close with that fix, after the full mutation
sets, the battery and make check.
