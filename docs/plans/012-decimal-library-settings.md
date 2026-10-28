# 012 — Results must not depend on the decimal library's settings (#237)

## Problem

shopspring/decimal keeps process-wide settings in package variables. Any
module of the program that embeds this engine may set them. Since #239 the
engine requires v1.6.0, which has eight:

| Variable | Default | Read by (v1.6.0) |
|---|---|---|
| `TrimTrailingZeros` | true | `String`, `Format` (and through `String`: `MarshalJSON`, `MarshalText`, `Value`) |
| `UseScientificNotation` | false | `String` when the exponent is positive |
| `DivisionPrecision` | 16 | `Div`, `Sqrt`, `Root` |
| `PowPrecisionNegativeExponent` | 16 | `Pow` |
| `ExpMaxIterations` | 1000 | `ExpHullAbrham`, `ExpTaylor` |
| `MarshalJSONWithoutQuotes` | false | `MarshalJSON` |
| `MaxDecodeExponent` | 10000 | `NewFromString` and the other decoders |
| `Zero` | 0 | `ExpTaylor` only, as a placeholder it overwrites at once; no result reads it (`IsZero` is `Sign() == 0`) |

The engine calls `String`, `Div` and `NewFromString` and uses `Zero`.
Measured with a probe in a copy of main (scratchpad `p237`): about 90
expressions over literals and a resource, evaluated with defaults and then
with each variable changed.

- `TrimTrailingZeros = false`: 16 differ. `(10.0 / 4).toString()` gives
  `2.5000000000000000`; `(1.0 + 1.00).toString()` gives `2.00`;
  `(1 '[in_i]').toQuantity('cm')` gives `2.5400000000000000000000000000000000 'cm'`.
  `sum()`, `avg()` and a resource's values are also affected.
- `UseScientificNotation = true`: 3 differ. `(5.0).round(-1)` gives `1E1`,
  and a resource's `1e2 * 10` gives `1.0E3`.
- `DivisionPrecision` 4 or 40: 2 differ, `avg()` and a Quantity divided by a
  number. `(10 'mg' / 3)` gives `3.3333 'mg'` with 4.
- `MaxDecodeExponent = 1`: 5 differ. A program may lower it to bound the
  input it decodes. The engine's re-reads of its own text then fail.
  `1 'kg' + 1 'g'` gives empty, and `1.0 'g' ~ 1000.0 'mg'` gives false.
- `Zero` set to 1: `0.0.toBoolean()` gives empty.
- `PowPrecisionNegativeExponent`, `ExpMaxIterations` and
  `MarshalJSONWithoutQuotes`: no difference. The engine computes powers,
  roots, exponentials and logarithms through float64, and it marshals no
  decimal.

No platform repo sets any of these today (go-fhir-server, go-cql,
go-validator, go-searchindex). The defect is latent, but nothing in the
engine prevents it.

The answer each of these expressions should give is today's answer under the
defaults. FHIRPath leaves a Decimal's String form to the implementation.
This engine's choices are recorded and pinned by the conformance suite:
trailing zeros dropped, no exponent, and a division to 16 places
(CONFORMANCE.md). The defaults reproduce v1.4.0, which had no settings for
the first two.

## Prior review

- **gofhir/server (frozen):** it reads decimals as float64. Nothing to take.
- **fhirpath.js and Java:** not affected. JavaScript numbers and BigDecimal
  have no process-wide settings. Nothing changes against them; corpusdiff
  compares this engine with main.
- **The library:** the settings are by design and documented as global
  ("affects all decimals"). They are not a defect to report. The engine
  must simply not reach them. FP-8 in docs/upstream-findings.md records it.

## Plan

Nothing in the engine may call a method that reads a setting, or use a
setting itself. Each site is replaced by a method or computation that does
not read one. Each replacement must give the same value as the defaults.

1. **Formatting.** Add `decimals.Format(d)`, in a new package
   `internal/decimals` that types, funcs and ucum all import (ucum cannot
   import types). It returns the text
   `String()` gives under the defaults: no exponent, trailing zeros of the
   fraction dropped. It is built on `StringFixed` (which reads no setting,
   `string(false, false)`), and the zeros are trimmed in the engine.
   Sites: `funcs.fnAvg`, `fnRound`, `fnSum`, `offsetInHours`,
   `quantityResult`, `types.Decimal.String`, `Quantity.valueText`,
   `dropTrailingZeros`.
2. **Division.** Add one engine constant, `types.DivisionPlaces = 16`, which
   is today's precision. `Div(x)` becomes `DivRound(x, DivisionPlaces)`.
   Sites: `funcs.boundaryInterval`, `fnAvg`, `offsetInHours`,
   `Quantity.Divide`, `convertCalendar`.
3. **Re-reads.** No text is decoded except input.
   - `dropTrailingZeros` uses the library's `Normalize()`, which reads no
     setting. Where that leaves a positive exponent, the value is rescaled
     to exponent 0, which is what today's `NewFromString(String())` gives
     (`3000`, not `3E3`). The exponent is pinned by a test.
   - `quantityResult` and `quantityOf` take the value, not its text.
   - `ucum.toRat` uses `d.Rat()`.
   - `ucum.fromRat` uses the library's `NewFromBigRat(r, exactDigits)`,
     which is `DivRound` (half away from zero, as `FloatString`), and then
     drops trailing zeros as above.

   `rereadDecimalSites` then becomes empty, and the guard allows no decoding
   outside `decodeDecimal`.
4. **Zero.** `decimal.Zero` becomes `decimal.Decimal{}`. Sites:
   `fnConvertsToBoolean`, `fnToBoolean`, `normalizedZero`, `roundTo`.
   `types.Decimal.Divide`'s own `DivRound(…, 16)` takes `DivisionPlaces` too,
   and `pastTheCalendar`, whose error text `ShiftAmount` builds, formats a
   decimal value through `decimals.Format` rather than `%v`.
5. **Guard.** Extend `decimal_sites_test.go`: no production package may use
   a setting variable, or a function or method of the library that reads
   one, directly or through calls. The list is not written by hand. The test
   type-checks the library's source, in the version go.mod selects, and
   closes over its call graph from the setting variables. A new version
   that adds a reader is caught by itself. A `decimal.Decimal` passed to a
   fmt function, or to a parameter of type `any`, is not visible to it.
   That case is covered by the behaviour test below, and `ShiftAmount`'s
   case is removed.

## Tests (first)

- **`TestResultsDoNotDependOnTheDecimalLibrarysSettings`** (root). For every
  site above, an expression that reaches it (the probe's ~90 and one per
  site it misses). Each is evaluated under the defaults, then with each
  setting changed: `TrimTrailingZeros` false, `UseScientificNotation` true,
  `DivisionPrecision` 4 and 40, `PowPrecisionNegativeExponent` 2,
  `ExpMaxIterations` 1, `MarshalJSONWithoutQuotes` true,
  `MaxDecodeExponent` 1, and `Zero` set to 1. The results must be the same.
  The expected values are written as literals, measured with the engine at
  2e1f2b1, which built on v1.4.0, a version with no settings. They are
  checked under the defaults and under each changed setting. The
  conformance suite does not pin them: changing a setting fails at most one
  of its cases (measured). The test changes globals, so it runs in its own
  process: it re-executes the test binary with an environment variable, so
  no parallel test sees the change.
- **Conformance under changed settings.** The spec and HL7 suites run once
  more in a re-executed process with every setting changed at once,
  `MaxDecodeExponent` lowered to 1 included. The set of cases passed must be
  the same, case by case. This is the metamorphic
  test: the same inputs under a different library configuration give the
  same results.
- **`formatDecimal`.** A table against `String()` under the defaults, which
  is the reference (v1.4.0 behaviour): zero, negative zero, exponent
  positive, zero and negative, trailing zeros, integers, and values beyond
  `DecimalDigits`. A Decimal has no negative zero. Zeros are trimmed only
  after the point. Plus `FuzzFormatDecimal` (coefficient, exponent) against
  `string(true, false)` through `String()` with the defaults set.
- **`fromRat` and `dropTrailingZeros`.** A table against today's
  `FloatString` + `NewFromString` and `NewFromString(String())`: repeating
  fractions, halves at the last place, negatives, a tiny negative that
  rounds to zero, integers, and trailing zeros. Coefficient and exponent are
  compared, not the text. Plus a fuzz against the same reference.
- **Guard.** Mutations: a `String()` call in funcs, a `Div` call in types, a
  use of `decimal.DivisionPrecision`, `fmt.Sprint(d)` on a Decimal, and a
  `decimal.Zero` use. Each must be reported.

## Data and validation

- corpusdiff in R4, R4B and R5 against main: 0 differences. The change must
  not move an answer under the defaults.
- `make check`; 010's and 011's mutation sets, plus this plan's.
- Downstream: go-validator and go-searchindex with this branch.
- Cost: `formatDecimal` and `dropTrailingZeros` against today's, measured
  with a benchmark. A result's formatting is on the hot path of `toString()`
  and of every result printed.

## Out of scope

- **A program changing a setting while the engine runs.** Under the plan the
  engine no longer reads any setting, so this cannot race with it. A race
  inside the program's own use of the library is the program's.
- **A caller's own `Resource` type** (`EvaluateResource`,
  `EvaluateResourceCached`, `NewResourceJSON`). It is marshalled by
  `encoding/json`. A `decimal.Decimal` field in it is written as a JSON
  string under the defaults, so the engine reads a String, not a Decimal.
  That is the caller's type and setting. It is documented on the three, not
  changed.
- **The FHIR grammar for a resource's decimals:** #238.

## Exit criterion

With every setting changed, all at once and one at a time, the engine gives
the same results as under the defaults. Verified by the root test and by the
conformance suites in a re-executed process. The guard forbids every method
and variable that reads a setting. corpusdiff shows 0 differences against
main.

## Dependencies

None new.

## Adversarial round (2026-10-28, Fable and Opus)

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | The guard's list was incomplete: `PowInt32`, `PowBigInt`, `Avg`, `Atan`, `Tan`, `BigFloat`, `StringScaled`, `EncodeSpanner`, and the `NullDecimal` encoders also reach a setting | both, medium | **Accepted.** The list is derived by the test from the library's source (step 5) |
| 2 | `StringFixedCash` does not call `String` | Fable | **Accepted.** Verified: `rounded.string(false, false)` |
| 3 | `Zero` is read by `ExpTaylor` (Opus); by nothing (Fable) | both | Resolved from the source: `ExpTaylor` appends `Zero` to its factorial cache and overwrites it on the next line, so no result reads it. The table says so |
| 4 | The conformance run detects little: one case fails with `TrimTrailingZeros`, none with `UseScientificNotation` or `DivisionPrecision`. `MaxDecodeExponent` was left out without reason | Opus, medium-high | **Accepted.** `MaxDecodeExponent` is included, and passes are compared case by case. The main detector is the root test |
| 5 | The root test's oracle is the same binary under the defaults | both, medium | **Accepted.** The expected values are literals measured with the engine at 2e1f2b1 (v1.4.0) |
| 6 | `fromRat` should use `NewFromBigRat`, and `dropTrailingZeros` `Normalize`; `Normalize` leaves positive exponents where today's re-read gives 0 | both | **Accepted.** Rescale to 0, and pin the exponent |
| 7 | A missed site: `ShiftAmount` formats a Decimal with `%v` through an `any` parameter (error text only). `Decimal.Divide` has its own 16 | Fable, low | **Accepted** (step 4). The AST guard cannot see `any`; the behaviour test covers that case |
| 8 | The scope note names only `EvaluateResource`; under the defaults a caller's decimal is already a string | both, low | **Accepted.** All three entry points, reworded |
| 9 | Is it a workaround? | both | No: the engine stops depending on documented process-wide state, through the library's explicit-precision methods |
| 10 | The re-executed child inherits no `-race` or coverage | Fable, low | Noted. The child is the same binary, so a `-race` build stays one. Coverage is not merged, and nothing here reads it: the mutation runner reads test failures |

## Validation (2026-10-28)

- Tests first, each failing before the change:
  - `TestResultsDoNotDependOnTheDecimalLibrarysSettings`: 73 expressions (77
    after round 1),
    answers measured with the engine at 2e1f2b1 (v1.4.0, scratchpad
    `ref237`), equal on main under the defaults. 7 of the 10 settings runs
    failed.
  - `TestTheSuitesAnswerAlikeUnderTheDecimalLibrarysSettings`: 20 subtests
    failed.
  - `TestNoDecimalLibrarySettingIsRead`: 25 sites, exactly the plan's list.
    The derived list has 43 readers in v1.6.0, every one the adversarial
    round named among them, and neither `StringFixedCash` nor `NewFromBigRat`.
  - `TestEveryDecodingOfADecimalGoesThroughTheEngine`: the four re-read
    sites are no longer allowed.
- The `internal/decimals` and `fromRat` tables and fuzz tests compare with
  the library under its defaults and with the former re-read, on
  coefficient and exponent. `FuzzFormatAndNatural` ran for 20 s, clean.
- Mutations (`mut/mutate012.py`): 18 of 18. The guard's sanity row used
  `MarshalJSON`, which reads a setting directly, so a guard without
  transitive readers survived. It now uses `MarshalText`, which reaches a
  setting only through `String`. A planted transitive reader in funcs is
  caught. The calendar error's `%v` (E10) is caught by the behaviour test
  alone, as the plan expects. 010's set: 50 of 50 (M11's pattern updated);
  011's: 12 of 12.
- `make check` green; corpusdiff 0 differences in R4, R4B and R5;
  go-validator green; go-searchindex fails only its known tripwires.
- Cost: `Natural` costs 143 ns for four values against 552 ns for the
  re-read it replaces. `Format` costs 364 ns against `String()`'s 252 ns,
  about 28 ns and one allocation more per value. The re-executed processes
  add about 1.5 s (root, 188 MB) and 1 s (conformance, 508 MB, as the
  suite alone).

## Code review

### Round 1 (2026-10-28, Fable and Opus)

Neither found a change under the defaults or a site still reading a setting.
Opus ran the 73 cases against main, and logged at run time every call from
outside the library to the 43 readers while both suites ran: none from
production code. Fable found, over every production package, one decimal
passed to an interface parameter, `pastTheCalendar`'s, which formats it
first.

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | `lowBoundary`/`highBoundary` round the half-unit interval to 16 places, so at 16 places or more it is wrong: `1.12345678901234567.lowBoundary(18)` gives the input itself | Opus, medium | **Confirmed, and out of scope.** main gives the same answers: it is not a regression, and fixing it changes answers under the defaults, which needs the three engines measured. Kept at 16 here; issue #244 |
| 2 | Four sites only the guard caught: `normalizedZero` and `roundTo` under `Zero`, `valueText` under `UseScientificNotation`, the calendar conversion under `DivisionPrecision` | Opus, low | **Accepted.** Four cases added, answers measured with the engine at 2e1f2b1; E13, E19, E20, E21 now fail the behaviour test itself |
| 3 | Run with `-update-known-failures`, the conformance child compares with the baselines being rewritten | Opus, low | **Accepted.** The re-executed run is skipped then |
| 4 | A child that runs no test passes silently | Fable, low | **Accepted.** The root test runs the child with `-test.v` and requires its PASS line |
| 5 | `TestToRatIsExact` compared `toRat` with its own body | Fable, low | **Accepted.** Compared with the former `big.Rat` read of `String()` under the defaults |
| 6 | Stale comments (`Decimal.String`, `toRat`); the guard's sanity list on one line; plan text naming `types.formatDecimal` and `ShiftAmount` | both, low | **Accepted** |
| 7 | `pastTheCalendar(value any, …)` could take a string so the compiler enforces it | Opus, low | Kept: its other callers pass integers, and the behaviour test covers it (E10) |
| 8 | `Natural`'s rescale to exponent 0 is invisible through the engine | Fable, low | Noted: pinned by the unit tests, as the former re-read's exponent was by none |
| 9 | The re-executed processes cost less than the plan said: about 0.1 s for the ten roots, 0.4 s for conformance | Fable | Noted; the validation's 1.5 s included the parent's own run |

### Round 2 (2026-10-28, Fable and Opus): converged

Neither left a finding of medium severity or higher. Both reverted each site
to the library's `Zero`, `String` or `Div` (13 mutants) and the behaviour
test failed for every one; both re-measured the 77 answers on a `git archive`
of 2e1f2b1. The PASS check held against a skipped, missing or renamed child
test.

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | The conformance child is checked by its exit code alone: suites that all skipped would pass | both, low | **Accepted.** The child runs with `-test.v`; `TestOfficialSuite` and `TestSpecCases` must pass in it, and nothing may skip. Mutation: `TestSpecCases` skipped in the child, caught |
| 2 | Stale counts in this plan (75, 74) | Fable, low | **Accepted.** 73 before round 1, 77 now |
| 3 | The guard's sanity comment is terse | Fable, low | **Accepted** |
| 4 | `TestToRatReadsAsBefore` holds only under the defaults | Opus, low | Kept: its comment says so, and no test of that package changes a setting |

## Close (2026-10-28)

After round 2's fixes: `make check` green; mutations 21 of 21 (012), 12 of
12 (011) and 50 of 50 (010); corpusdiff 0 differences against main. Exit
criterion met: with every setting changed, one at a time and all at once,
the root test's 77 answers and both conformance suites are as under the
defaults, and the guard forbids each of the library's 43 setting readers.
Left out: #244 (boundaries at 16 or more places), #238 (FHIR decimal grammar).
