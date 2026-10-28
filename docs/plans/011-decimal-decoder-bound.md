# 011 — The decimal range must not depend on the decoder's bound (#235)

## Problem

Since shopspring/decimal v1.5.0, `decimal.NewFromString` refuses an exponent
whose magnitude passes `MaxDecodeExponent` (10,000 by default), whatever the
value. Since v1.6.0 it also reads a Unicode minus (U+2212) as `-`. This
module pins v1.4.0, but a consumer selects the version through MVS, and
go-cql already requires v1.6.0. Under v1.6.0 the engine changes behaviour.

Measured on main after #236, with v1.6.0 in a copy (scratchpad `i235`). Five
packages fail: root `TestDecimalRange`, `types` `TestCompareDecimals`, and
seven conformance tests of #232.

- A resource's `1e20000` is read as absent again, the #232 defect: `exists()`
  false, `> 0` empty. So is `1e999999999`, which R5's regex allows and which
  `TestAnExponentPastInt32IsAKnownLoss` pins as read.
- `1.` followed by 15,000 zeros (exponent -15000), which #236 reads as 1, is
  absent.
- A FHIR Quantity whose value is `1e20000`: `= 1 'mg'` gives false instead of
  empty. The value fails to decode, so the object is taken for no quantity
  rather than an overflowing one (`quantityValue`, `OverflowingQuantity`).
- `types.NewDecimal("1e−2")` and `types.ParseDecimal` give 0.01. With v1.4.0
  they give an error. FHIRPath's string conversions are unaffected: their
  regex checks first, so `'−1'.toDecimal()` is empty under both versions.

The FHIR decimal grammar is ASCII. R4 and R4B give
`-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?`; R5 gives, as informative,
`-?(0|[1-9][0-9]{0,17})(\.[0-9]{1,17})?([eE][+-]?[0-9]{1,9}})?` (18 and 17
digits, a nine-digit exponent, and a stray `}`, SPEC-5). FHIRPath's String to Decimal conversion is
`(\+|-)?\d+(\.\d+)?`. R5's nine-digit exponent, like R4's unbounded one,
passes 10,000; `1.` followed by 15,000 zeros is outside R5's informative
regex (17 places) but inside R4's. The bound is the engine's
`DecimalDigits` (CONFORMANCE.md, "The range of a Decimal").

## Prior review

- **gofhir/server (frozen):** read decimals as float64. Nothing to take.
- **fhirpath.js and Java:** not affected. Neither uses this library, and #232
  measured both reading these values. Nothing changes against them; the
  three-engine battery `decimal-beyond-bound` stays the reference.
- **Upstream:** `MaxDecodeExponent` is documented, deliberate and global. Its
  doc says "Raise it only for trusted input" and "Set it once at program
  start". It is no library defect, so nothing to report. Raising it from
  here would change every other user of the library in the process. This
  engine bounds cost by `DecimalDigits` itself (#232's `InDecimalRange` and
  `CompareDecimals`), so the decoder's bound is not needed.

## Plan

1. **Our own decoder, `types.decodeDecimal(s string) (decimal.Decimal, error)`.**
   - It checks the grammar on ASCII bytes: the accept set of
     `NewFromString` v1.4.0, measured, not decided here (#238 decides it):
     - an optional `+` or `-`;
     - digits, with an optional point that has digits on at least one side;
     - an optional `e`/`E`, with an optional sign and any number of digits.
   - Anything else is an error: U+2212, and `.-5`, which v1.4.0 reads as
     -0.05.
   - It computes the exponent in int64, builds the coefficient with
     `big.Int.SetString` on the digits (as `NewFromString` does), and returns
     `decimal.NewFromBigInt`. That constructor is not bound by
     `MaxDecodeExponent`.
   - The written exponent must fit in int32, as v1.4.0 has it, and so must
     the exponent after the places after the point are taken off:
     `0.5e-2147483647` is read, `0.05e-2147483647` refused (FP-6 stays).
   - The result is the same coefficient and exponent v1.4.0 gives for every
     string v1.4.0 accepts.
2. **Every decoding of text that comes from input uses it:**
   - `ParseDecimal` (and so `NewDecimal`), `ReadDecimal` and `withinScale`
     (`types/decimal.go`);
   - the JSON number comparisons in `types/json_equal.go:279` and
     `types/json_equivalent.go:688`.

   The other `NewFromString` calls (`quantity.go:682`, `ucum.go:309/316`,
   `conversion.go:684`, `math.go:247`) re-read a value the engine already
   holds within the bound. Their exponent is at most about 2,000, so they
   keep `NewFromString`. Each gets a test that pins it within the bound.
3. **Bump shopspring/decimal to v1.6.0** (the user's decision) in every module (root,
   conformance, corpusdiff, difftest). Consumers select it already, and the
   suite must run under the version they get. Justification for the
   dependency change: none new; same library, the version go-cql already
   requires. `MaxDecodeExponent` is left at its default.
4. **A guard against new direct decodes.** `decimal_sites_test.go` already
   guards direct shopspring calls (#232). It now also refuses
   `decimal.NewFromString` and `RequireFromString` in production code, except
   at the listed sites of step 2.

## Tests (written first, expectations from the grammar and v1.4.0, not from the new code)

- **The #232 suite runs under v1.6.0:** `1e5000000`, `1e999999999`,
  `1.` + 15,000 zeros, and the FHIR Quantity rows. Today these fail (above).
- **`decodeDecimal` against an oracle.** A table of spellings:
  - signs, leading zeros, `.5`, `5.`, `E`/`e`, signed exponents;
  - exponents at ±10,000, ±10,001, ±2147483647 and ±2147483648;
  - U+2212 in each position;
  - empty, `.`, `e5`, `1e`, `1e+`, spaces, `1_000`, `0x10`, `NaN`, `Inf`.

  The oracle is `big.Rat.SetString` on ASCII input plus the expected
  exponent. Where v1.4.0 accepts a spelling, `decodeDecimal` gives the same
  coefficient and exponent; it is measured against v1.4.0's `NewFromString`
  in a scratch module, and the expectations are written into the table.
- **`FuzzDecodeDecimal`:** for any input either both reject, or the value
  equals `big.Rat`'s and the exponent is the written one. The comparison
  skips exponents past what `big.Rat` reads quickly. The fuzzer runs in
  `make fuzz` if the repo has one; otherwise the seed corpus runs in
  `go test`.
- **Public API:** `NewDecimal`, `ParseDecimal` and `ReadDecimal` refuse
  U+2212 in each position, and `ParseDecimal("1e20000")` is
  `ErrDecimalRange`.
- **Cost:** decoding `1e2000000000` (short text, huge exponent) answers in
  microseconds and is beyond the bound. Long texts cost the same as now,
  measured.
- `make check`, corpusdiff, and downstream go-validator and go-searchindex.
  The three-engine battery `decimal-beyond-bound` is unchanged.

## Out of scope

- FP-6: an exponent past int32. The decimal type holds an int32 exponent,
  and changing that means another representation.
- The cost of `big.Int.SetString` on texts of millions of digits (seconds).
  It predates this increment; it is recorded as known.
- Raising or lowering `MaxDecodeExponent`.

## Exit criterion

`make check` green with decimal v1.6.0, in every module. The #232 suite
passes unchanged. `decodeDecimal` matches v1.4.0's coefficient and exponent
on its table and under fuzzing. U+2212 is refused by the public API. The
guard test fails on a new direct `NewFromString`.

## Adversarial round (2026-10-27, Fable and Opus)

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | Resource numbers outside FHIR's grammar are read today: `-.5` → -0.5, `5.` → 5, `01` → 1, `007.50`, `1.e5` (jsonparser types by the first byte; `.5`, `+1` never arrive). FHIR's regex and FHIRPath's NUMBER refuse them; the grammar is inherited from the library, not decided | both, high/medium | **Out of scope, by the user's decision (2026-10-27):** a separate issue decides the grammar, as #233 did for dates. 011 keeps today's accept set but for `.-5` and U+2212, and pins it with table rows |
| 2 | R5's regex is not R4's: `-?(0|[1-9][0-9]{0,17})(\.[0-9]{1,17})?([eE][+-]?[0-9]{1,9}})?`, 18/17 digits and a 9-digit exponent, informative, with a `}}` typo | Fable high | **Accepted.** The plan cites the three; the conclusion holds (9 digits exceed 10,000). The typo goes to `docs/upstream-findings.md` as a SPEC finding |
| 3 | U+2212 reaches the engine through JSON: jsonparser hands `1e−2` over as a Number; v1.4 reads it as absent, v1.6 as 0.01 | both, high | **Accepted.** A resource-level test, not only the public API |
| 4 | v1.4 accepts `.-5` as -0.05, so "same as v1.4 for everything it accepts" is false | Opus medium | **Accepted.** Refused, named as the exception |
| 5 | `big.Rat.SetString` accepts `0x10`, `1_000`, `1/2`, `0x1p-2`: no oracle for rejection | both, medium | **Accepted.** An independent regex decides acceptance; `big.Rat` checks only the value |
| 6 | The guard scans only `eval/` and `funcs/`, and only method calls; `NewFromString` is a package function. Tests decode with shopspring too (`RequireFromString("1e8227822")` panics under v1.6) | both, medium | **Accepted.** The guard type-checks every production package and package-level identifiers (`NewFromString`, `RequireFromString`, `NewFromFormattedString`, `Unmarshal*`, `Scan`, `GobDecode`, `NullDecimal`); test helpers migrate |
| 7 | Since v1.5, process globals change output: with `TrimTrailingZeros=false`, `10.0 / 4` is `2.5000000000000000` and `1.0 + 1.00` is `2.00`; `UseScientificNotation` makes `String()` write `E`. Any module in the process can set them | Opus medium | **Out of scope, by the user's decision:** a separate issue, with this reproduction |
| 8 | Other differences v1.4 → v1.6: `NumDigits(10^15)` 15 → 16 (`quantity.go:296`), `decimal.Zero`'s exponent 1 → 0 (`normalizedZero`), `Shift` panics on overflow, `Truncate` with a negative precision; `Round`, `String`, `NewFromFloat` the same over 200,000+ random inputs | both, low | **Moot after the bump:** go.mod requires v1.6.0, so by MVS no build of this module runs v1.4.0's `Zero` or `NumDigits`; the suite runs under v1.6.0's, which counts 10^15 right |
| 9 | Exponent edges: `1.5e2147483647` (read), `0.5e-2147483647` (read, exp −2^31), `0.05e-2147483647` (refused), `1e0000000000000000000005` (= 1e5), `1e99999999999999999999` | both, low | **Accepted.** Rows in the table; the decoder does not cap the exponent's digit count, only its value |
| 10 | The bump moves consumers: go-searchindex decodes SampledData with its own `NewFromString` (`adr001.go:798`) and would accept `−1` under v1.6 | both, low/medium | **Accepted.** Recorded in `docs/upstream-findings.md` (searchindex should call `types.ParseDecimal`); the user chose the bump to v1.6.0 |
| 11 | jsonparser reads a bare `-` as Integer 0 | Fable low | Out of scope: `docs/upstream-findings.md` |

## Validation (2026-10-28)

- Tests first: the decoder table (expectations measured on v1.4.0 in a
  scratch module), refusals, the public API, a huge exponent, a resource's
  `1e−2`, `FuzzDecodeDecimal` (an independent regex decides acceptance,
  `big.Rat` the value) and `TestEveryDecodingOfADecimalGoesThroughTheEngine`;
  each failed before `decodeDecimal`, as did the #232 suite under v1.6.0.
- Fuzzing: 6.1 million executions, clean.
- Mutations (`mut/mutate011.py`): 10 of 10 caught. D5, the empty mantissa
  check, survived as equivalent (`big.Int.SetString` refuses it), so the
  check was dropped. D10 duplicated 010's M18 and was merged into it. 010's
  set: 50 of 50 (M18's pattern updated).
- `make check` green under v1.6.0; corpusdiff 0 differences in R4/R4B/R5;
  go-validator green; go-searchindex fails only on the known SI-1/SI-2/SI-3
  tripwires, as before.

## Code review

### Round 1 (2026-10-28, Fable and Opus)

Both fuzzed `decodeDecimal` against v1.4.0's `NewFromString` in a separate
module (2,000,000 and 8,400,000 spellings): no difference in acceptance,
coefficient or exponent but the named `.-5`/`.+5`. Opus ran a 1,994-line
probe of resource spellings through the engine on main and on the change:
identical output.

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | Short decimals, every resource's, cost about twice v1.4.0's decode: `big.Int.SetString` and a copy in `NewFromBigInt` always | both, medium/low | **Accepted.** Eighteen digits or fewer are read into an int64 and `decimal.New` (not bound by `MaxDecodeExponent`): `"123.45"` 73 ns and 2 allocations (v1.6.0's `NewFromString` 48 ns, 2). Rows at 18 and 19 digits; D13, D14 |
| 2 | The decoding guard type-checks the dependency graph once per directory: 5.9 s, and the rescaling guard 3.4 s more | Fable medium | **Accepted.** One importer and one walk for both guards: 2.2 s together |
| 3 | The guard sees only function bodies (a package-level `var` initializer is missed), its directories are a fixed list, and it leaves out `NewFromBigFloat`, `DecodeSpanner`, `UnmarshalBSONValue` | both, low | **Accepted.** Every declaration is inspected; the module's production packages are found by walking it (sub-modules and testdata skipped); the three named. Mutations: a `var` in `types` and a new package's decode, both caught |
| 4 | The re-read sites' comment said "within the bound"; a value beyond it may reach them. The plan promised a test per re-read site | Opus low | **Accepted in part.** The comment now says what holds: `String()` writes no exponent (until #237's setting), so none passes `MaxDecodeExponent`. No per-site test: the guard pins the sites, and #237 takes them off `String()` |
| 5 | `ParseDecimal`'s and `NewDecimal`'s error text changed (`invalid decimal: …` against shopspring's `can't convert …`) | Opus low | Kept; the PR names it. The error kind a caller can test, `ErrDecimalRange`, is unchanged |
| 6 | No row for `1e+0005` or `-0.50E+3` | Fable low | **Accepted.** Rows added, measured on v1.4.0 |

### Round 2 (2026-10-28, Fable and Opus): converged

Neither left a finding of medium severity or higher. Both checked the fast
path against the big.Int path and shopspring's `NewFromString` (300,000 and
200,000 random spellings, and the edges -0, leading zeros, 18 and 19 digits):
no difference. Every guard mutation was caught; the walk finds exactly the
packages `go list ./...` reports. Both guards together: 1.5–3.1 s.

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | The rescaling guard reads only function bodies: a rescale in a package-level table of function literals is missed (as on main) | Opus low | **Accepted.** Both guards read every declaration, named by `declName`. Mutation caught (`funcs.var`) |
| 2 | The decoding guard names a method without its receiver, so a method named like an allowed function passes | Opus low | **Accepted.** Named `pkg.Recv.Method`. Mutation caught (`funcs.zzT.quantityOf`) |
| 3 | The walk does not skip `vendor/` or `_` directories, which the toolchain ignores | Fable low | **Accepted.** Skipped |
| 4 | `parser/grammar` is type-checked too, though it holds no decimal | Fable low | Kept: it is a production package, and a guard that picks packages by hand is what round 1 removed |
| 5 | An inline comment without a final period | Opus low | Rejected: the repo's inline comments carry none (`types/decimal.go:22`) |

## Close (2026-10-28)

After round 2's fixes: `make check` green; mutations 12 of 12 (011) and 50
of 50 (010). Exit criterion met: under shopspring/decimal v1.6.0 the engine
reads every decimal as on v1.4.0, past `MaxDecodeExponent` included, and
refuses a Unicode minus.
