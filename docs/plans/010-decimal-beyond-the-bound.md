# 010 — A FHIR decimal beyond the bound is read, not lost

Status: step 5, tests first (user approved approach A with the round's scope, 2026-10-26)
Issue: #232

## Problem

A decimal read from a resource whose exponent puts it beyond
`types.DecimalDigits` (1000) is read as absent: the decimal branch of
`jsonValueToFHIRValueWithType` (`types/object.go`) drops `ErrDecimalRange`
and returns nil. `Patient.extension.value.exists()` over `"valueDecimal":
1e1000` is false, and the caller cannot tell it from an element that is not
there. This was a decision (#135, CONFORMANCE.md "The range of a Decimal":
"A value beyond, read from the input, … is an overflow and reads as empty"),
taken for cost: a decimal is held exactly, and comparing `1E8227822` wrote out
its eight million digits.

## Prior review

### Measured: the three engines

Batteries in the scratchpad (`dec/battery-{a,b,c,d}.tsv`), R4 and R5, on
declared decimals (`Extension.valueDecimal`, `ChargeItem.factorOverride`) and
`Quantity.value`; `Observation.valueDecimal` is no element of R4 or R5 and was
dropped.

| Value | `exists()` here / js / Java | `> 0` here / js / Java | `toString()` js / Java |
| --- | --- | --- | --- |
| `1e999` | true / true / true | true / true / true | `Infinity` / `1E+999` |
| `1e1000`, `1E1000` | **false** / true / true | **empty** / true / true | `Infinity` / `1E+1000` |
| `-1e1000` | **false** / true / true | **empty** / false / false | `-Infinity` / `-1E+1000` |
| `1e-1000` | true / true / true | true / **false** / **false** | `0` / `1E-1000` |
| `1e-1001` | **false** / true / true | **empty** / false / false | `0` / `1E-1001` |
| `0e2000` | **false** / true / true | **empty** / false / false | `0` / `0E+2000` |
| `1e999999999` | **false** / true / true | **empty** / true / true | `Infinity` / `1E+999999999` |

Both references read the value; this engine alone loses it (rule 1: both
agree → follow). Neither is exact: fhirpath.js holds a float64 (`1e1000` is
Infinity, `1e-1000` is 0); Java answers `1e-1000 > 0` false though it prints
`1E-1000` (it compares through a double, apparently). Those go to
`docs/upstream-findings.md`, not followed: this engine compares exactly.

### Measured: the cost is in operating, not in holding

`shopspring/decimal` holds a coefficient and an exponent: `1e8227822` is held
in 166 ns, and comparing it with itself takes 41 ns, but with `1` it takes
1.43 s (both rescaled to one exponent). So reading is cheap; what #135
guarded against is an operation that rescales.

### The specification

- FHIR `datatypes.html` (R4, R4B, R5): decimal is the "union of xs:decimal and
  xs:double", and "JSON is restricted to the precision limits documented in XML
  schema for xs:double and xs:decimal". xs:double stops near 1.8e308, so by
  the text `1e1000` may not be a valid FHIR decimal at all. R5's regex
  (informative) allows `[eE][+-]?[0-9]{1,9}`, a nine-digit exponent; R4's
  allows any. The validator decides validity; FHIRPath reads what it is given.
- FHIRPath (2.0.0, 3.0.0), Decimal: "Implementations can provide support for
  larger decimals and higher precision, but must provide at least the range
  and precision defined here", and "Operations that cause arithmetic overflow
  or underflow will result in empty". Reading an element is no operation.

### gofhir/server

Read decimals with `strconv.ParseFloat` (`internal/storage/sql/search_builder_number.go`,
`internal/operations/convert_units.go`): float64, nothing to take.

## Approach

A decimal beyond the bound read from a resource **is held**, as both
references do, and **operating with it is an overflow**, as #135 decided for
every operation:

1. **Reading.** The decimal branch keeps the value with its text; `read`
   set, FHIR type set. `exists()`, `hasValue()`, `count()`, `type()`, `is`,
   `as`, `ofType`, `toString()` (its text as written) answer as for any
   decimal. The same for a decimal read without a model (`jsonValue`).
2. **Comparison is exact and cheap.** `Decimal.Compare` and `Equal`, and the
   Integer side of them, order by sign and adjusted exponent (exponent +
   digits) first and rescale only where those tie, which then costs at most
   the digits written in the input. So `1e1000 > 0` is true, `1e-1000 > 0`
   true, `1e1000 = 1e1000` true, `distinct()`, `union` and sorting work, and
   none expands.
3. **Every other operation with it overflows to empty.** Arithmetic,
   math functions, aggregates, `~`, conversions (`toInteger`,
   `toQuantity`, `toDecimal` keeps it), boundaries, `precision()`,
   `comparable()`: an operand beyond the bound gives empty, checked before
   computing, at the types layer (one helper, `beyondRange`), not at each
   caller. `~` rounds to a precision, which rescales: empty. (Superseded
   by the classification measured in round 1 of the adversarial review, below:
   `precision()` answers from the text and `~` compares without rescaling.)
4. **A Quantity whose value is beyond the bound** stays as #135 left it (the
   object is there, compares as nothing): out of scope, recorded.
5. Literals and `toDecimal()` of a string beyond the bound stay overflows:
   those are operations on text the expression wrote, not values read.

CONFORMANCE.md "The range of a Decimal" is rewritten: read values are held,
operations overflow; the measurement table goes in its own section. No
exception to rule 5: nothing FHIR-specific is written; `DecimalDigits`
stays the engine's bound.

## Out of scope

- A Quantity's value beyond the bound (item 4).
- Raising or removing `DecimalDigits`.
- Exact comparison of a Quantity beyond the bound.

## Exit criterion

1. `Patient.extension.value.exists()` over `1e1000`, `-1e1000`, `1e-1001`,
   `0e2000`, `1e999999999` is true in R4, R4B, R5, and every row of the
   measurement table where both references agree and the answer does not
   depend on float64 has this engine with them.
2. **Every function in the registry** and every operator, called with a
   decimal beyond the bound as input and as each argument, finishes within a
   time bound (the oracle that no path expands), and answers empty, false or
   the value's own text per the classification above.
3. corpusdiff against main: 0 differences in R4, R4B, R5 (no official example
   holds such a decimal; checked by scan).
4. `make check` green; downstream go-validator and go-searchindex tests run.

## Tests

- `conformance/decimal_beyond_bound_test.go`: reading across placements
  (choice, extension, nested extension, declared element, repeating, contained,
  Bundle entry, `_x` with an extension, without a model), three versions;
  expectations from the reference answers above, citing the text.
- Comparison table, hand-written with expected values from arithmetic (not
  from running the engine): mixed signs, equal adjusted exponents, decimal vs
  integer, `1000…0` (1001 digits written out) vs `1e1000`.
- Registry sweep: enumerate `funcs` registry and the operator list
  dynamically; each with `1e999999999` and `1e1000` in each position, a
  per-call deadline (e.g. 50 ms, measured margin), and the classified answer.
- Fuzz `FuzzDecimalCompare`: random coefficients and exponents within and
  beyond the bound, oracle `big.Rat` comparison on values small enough to
  expand.
- Battery `difftest/batteries/decimal-beyond-bound/` for `make three-engine`.
- Metamorphic: key order, an unknown extension, the same value in R4 and R5.

## Mutations planned

Drop the read (back to nil); compare by rescaling (time oracle must fail);
magnitude compare off by one in the adjusted exponent; forget the guard in
one arithmetic op, one math function, `~`, `sum()`; `toString` rendering the
value instead of the text; treat zero beyond the bound as nonzero.

## Dependencies

None new.

## Record

- 2026-10-26 step 1 done; user chose approach A (read, bound kept for
  operations) over B (evaluation error).

## Adversarial round (2026-10-26, Fable and Opus)

Both probed a copy that holds the value on read. The magnitude-first
comparison checks out (sign, zero, adjusted exponent, tie rescales within the
input's digits). The rest of the plan underestimated the surface. Verified
here: 28 direct shopspring operations on decimals in `funcs/` and `eval/`;
go-searchindex `adr001.go` `decimalOf` hands `n.Value()` to `number()` and
`readQuantity` unchecked; go-cql calls `.Value()`; the grammar's `NUMBER` has
no exponent.

| # | Finding (by) | Resolution |
| --- | --- | --- |
| 1 | High (both): "one helper at the types layer" cannot work: `sum` 49 s, `round()` 49 s, `lowBoundary()` 66 s, `toBoolean()` 2.4 s, `= 1 '1'` 0.6 s, `toQuantity()`/`abs()` print millions of digits; `Add`/`Abs` return a bare Decimal | Accepted: guard each site that would rescale; a static test fails on any shopspring operation on a decimal outside `types/` not routed through the guarded helpers |
| 2 | High (both): callers outside the repo receive the huge value (searchindex `number()`, `readQuantity` → ucum `Rat()` builds 10^999999999; go-cql) | Accepted: a contract change. `Decimal.Value()` documents it may be beyond the bound; CHANGELOG; a searchindex follow-up (check `types.InRange`) with a test using such a value, landing before it adopts the release; go-cql checked |
| 3 | Medium (Opus): object `=`/`~` and `money()` compare numbers through `ParseDecimal` (`types/json_equal.go:275`, `json_equivalent.go:687`, `object.go:2752`) | Accepted: routed to the new comparison |
| 4 | Medium (both): "everything else empties" is coarser than the text; `-x`, `abs()`, `precision()`, `comparable()`, `log(10)`, `power(0)` need no rescale and Java answers them | Accepted: classify by "would rescale" (empty) vs "answers"; every row measured against both references before writing the expectation |
| 5 | High (Fable): `~` returns a Boolean, never empty; exact equivalence is cheap (`Round(places)` is the identity where it would expand) and `1e-1001 ~ 0` is true | Accepted: exact, cheap `~` |
| 6 | High (Fable): keeping `toDecimal()` an overflow contradicts holding (`value.toString().toDecimal()` empty); literals cannot write an exponent at all | Accepted: hold wherever parsed (input, `toDecimal`, `convertsToDecimal` true) |
| 7 | Medium (Fable): `0e2000 + 1` is 1 in both references; the plan empties it | Accepted: a read zero is normalized to a scale within the bound, its text kept |
| 8 | Medium (both): a 50 ms deadline cannot stop `big.Int` work, and `1e999999999` takes minutes and hundreds of MB | Accepted: sweep with `1e5000000`/`1e8227822` (fails in 0.25–66 s) under `go test -timeout` and an allocation bound; `1e999999999` only in read-only tests |
| 9 | Medium (both): registry enumeration misses evaluator-handled names (`iif`, `aggregate`, `repeat`, `select`, `where`, `coalesce`, `sort`, `defineVariable`) and grammar operators; arities vary | Accepted: list = registry + evaluator cases + grammar operators, with a completeness test and a hand-written name → call shape → expected class table; unknown names fail |
| 10 | Medium (Opus): `1e-1001 > 0` true diverges from both references | Accepted: both read through a float (js float64; Java to verify): recorded as a deliberate divergence, exact per FHIRPath's Decimal |
| 11 | Medium/Low (both): exponents past int32 (`1e9999999999`, valid under R4's regex) still read as absent | Accepted: measured in both references, recorded as this engine's parse limit |
| 12 | Low (Opus): `ImplicitPrecision` counts exponent text (`1.5e2` → `precision()` 3) | Out of scope: `docs/upstream-findings.md` FP-5 |
| 13 | Low (Opus): `Quantity.value > 0` true while `Quantity > 1 'mg'` empty | Accepted: test pins the asymmetry (item 4 out of scope) |
| 14 | Low (Fable): "Java compares through a double" unverified | Accepted: verified or dropped before it is recorded |

## Classification (step 5, measured)

Battery of 65 expressions × `1e1000`, `-1e1000`, `1e-1001`, `0e2000`,
`1.5` over `Extension.valueDecimal`, R4 (scratchpad `dec2/`; kept in the repo
as `difftest/batteries/decimal-beyond-bound/`). The references part on almost
every row that depends on the value: fhirpath.js computes in float64
(`Infinity`, `NaN`, `0`), Java is exact for `round`, `+`, `-`, `/` and fails
elsewhere (`ceiling` clamps to int32, `truncate`, `div`, `sum()`, `avg()`,
`min()`, `max()` error even on `1.5`, `mod` throws). Java converts through a
double too: `abs()`, `sqrt()`, `power(2)` of `1e-1001` give `0.0`, and
`1e-1001 > 0` is false (finding 14, verified).

Where they agree only because both lose the value to a float (an underflow to
0: `1e-1001 > 0` false, `abs()` 0, `exp()` of `-1e1000` 0 and of `1e-1001`
1), the text decides: the value is exact, and "Operations that cause
arithmetic overflow or underflow will result in empty".

| Class | Rows | Answer |
| --- | --- | --- |
| Comparison | `=`, `!=`, `<`, `>`, `<=`, `>=`, `in`, `contains`, `distinct`, `union`, `intersect`, `exclude`, `subsetOf`, `isDistinct`, `sort`, `min()`/`max()` (they select the item), object `=`/`~` | exact, no rescale (a number against a quantity in another unit: `<` and `=` empty, `~` false); both references agree on every row not lost to a float |
| Equivalence | `~`, `!~` | exact: `1e-1001 ~ 0` true (rounded to 0 places), `V ~ V` true |
| Value-blind | `exists`, `count`, `hasValue`, `getValue`, `type()`, `is`, `as`, `ofType`, `iif`, `where`, `select`, `first`…, `trace`, `children` | as for any decimal |
| Text | `toString()` | the text as written, `1e1000`; Java answers the same on the four forms the battery holds (`1e1000`, `-1e1000`, `1e-1001`, `0e2000`, battery rows 48, 113, 178, 243), not measured on others such as `1E1000` |
| Identity | `toDecimal()`, `convertsToDecimal()`, `'1e1000'.toDecimal()` | the value held, true |
| Computing | arithmetic, unary minus, `abs`, `ceiling`, `floor`, `truncate`, `round`, `sqrt`, `exp`, `ln`, `log`, `power(n≠0)`, `sum`, `avg`, `aggregate` over it, `lowBoundary`, `highBoundary`, `toQuantity`, `toInteger`, `toLong`, `toBoolean` | empty; `convertsTo{Integer,Long,Boolean,Quantity}` false. An operand beyond the bound is outside the range this engine computes with, even where the exact result would be within it (`1e1000 / 2`): a cost limit of this engine, recorded as such |
| Exception | `power(0)` | 1, as both references answer |
| Kept | `precision()` | from the text, as today (FP-5 out of scope), Java answers 0 alike |
| Zero | `0e2000` | normalized on read to a scale within the bound, text kept: an ordinary zero (`0e2000 + 1` is 1, as both answer) |

Supersedes Approach items 2–3 and findings 4 and 7 where they differ: computing
functions empty uniformly (one rule, one helper per site), not "answer where no
rescale is needed".

## Implementation (step 6)

- `types.ReadDecimal`: reads a decimal as written, beyond the bound too; a
  zero is normalized to a scale within the bound, its text kept. The read
  paths (`jsonValue`, the decimal branch of `jsonValueToFHIRValueWithType`)
  and `toDecimal()`/`convertsToDecimal()` of a string use it.
- `types.CompareDecimals`: exact; the plain `Cmp` where the exponents lie
  within 2·DecimalDigits of each other (every pair within the bound, as
  before), else sign and leading place first. `Decimal.Equal`, `Compare`,
  `Equivalent` (with `roundTo`, which never rescales up), `toBoolean`,
  `min()`/`max()`, object `=`/`~` over JSON numbers use it.
- Guards: `beyondOperand` at the top of `Add`, `Subtract`, `Multiply`,
  `Divide`, `IntegerDivide`, `Modulo`, `Negate`; `noNumber` (every math
  function's input and number arguments) and `holdsNoAmount` (`sum`, `avg`)
  count a decimal beyond the bound as nothing to compute; `boundaryOf`,
  `toQuantity` refuse one; `Quantity.Compare` orders one in the same unit or
  against no unit and refuses it across units, `Equal`/`Equivalent` answer
  false and `EqualityUndecided` makes `=` empty across units (round 4).
  `power(0)` answers 1 before the guard.
- An exponent past int32 stays a known loss (both references read it):
  `TestAnExponentPastInt32IsAKnownLoss`, FP-6.
- A spec point corrected while testing: `'1e1000'.toDecimal()` is empty — the
  String form has no exponent (fhirpath.js agrees; Java converts it, JAVA-8).
  Finding 6 is accepted for a string written out digit by digit.
- Tests that pinned #135's reading as absent (`math_test.go`
  `TestDecimalRange`) now pin the value held and compared, and computing
  with it empty.

## Validation (step 7)

- `make three-engine BATTERY=batteries/decimal-beyond-bound/battery.tsv`
  after the change: 14 rows where this engine alone differs, all over
  `1e-1001` or `exp()` of `-1e1000`. Ten are a float's loss in both
  references (`> 0` false, `abs()`/`sqrt()`/`power(2)`/`ceiling()` 0,
  `exp()` 0 or 1, and the `iif` and `toString().toDecimal()` built on them);
  four are exact agreements this engine does not follow (`floor()`,
  `round()`, `round(2)`, `div 2` of `1e-1001` give 0): the uniform rule,
  recorded in CONFORMANCE.md "Measured, and not followed".

### Mutations (2026-10-27)

23 mutations, one at a time on a copy of the repo, root and conformance
suites run after each. First run: M03 (sign ignored on magnitude) and M11
(no shortcut to zero in `roundTo`) survived; hardened with negative rows far
apart in `TestCompareDecimals`, fuzz seeds, and
`TestEquivalenceOfADecimalBeyondTheBoundDoesNotRescale`. Second run: all
caught but M18 (object numbers left undecided beyond the bound), which only
changes the route: the slow path reads both objects and answers the same.
Hardened with `TestObjectNumbersBeyondTheBoundAreDecided`, which checks the
verdict itself; M18 then caught.

Mutations: M01 object number read with `NewDecimal`; M02 compare always by
rescaling; M03 sign ignored when leading places differ; M04 leading place
ignores the digits; M05–M07 `+`, unary `-`, `div` unguarded; M08 math
functions unguarded; M09 `sum()` unguarded; M10 `~` rounds by rescaling; M11
`roundTo` without the shortcut to zero; M12 boundaries unguarded; M13
`toQuantity()` unguarded; M14 Quantity comparison unguarded; M15
`toBoolean()` rescales; M16 zero beyond the bound not normalized; M17 no
`power(0)` exception; M18 object numbers undecided beyond; M19 `max()`
rescales; M20 text not kept; M21 a new unguarded rescaling site; M22
`toDecimal()` of a string refuses beyond; M23 `Decimal.Equal` rescales.
M02, M20 and M23 are caught by the time bounds (the suite times out).

### Corpus, downstream and check (2026-10-27)

- `make corpusdiff BASE=main` (working tree against main): 0 answers
  differ in R4 (521,007 evaluations), R4B (308,577) and R5 (360,675).
- go-validator against this tree (go.work): green.
- go-searchindex against this tree (go.work): four of its own known-loss
  tripwires fail, which is their purpose. `TestNumber` and
  `TestQuantityCanonical` now get the value beyond the bound that they
  recorded as lost (SI-3: searchindex has to check `InDecimalRange` before
  it adopts the release). The two date ones (`TestDateFormsInvalidForTheirType`,
  `TestFhirpathInvalidFormsKnownLoss`) come from #231, already on main.
- `make check`: green, after fixing gofmt and three lint findings in the new
  tests (prealloc, the error returned last, a regex anchor).

## Code review

### Round 1 (2026-10-27, Fable and Opus, each in its own copy)

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | A FHIR Quantity whose value is beyond the bound raised errors (compare, arithmetic, `abs`/`round`/…, `sqrt`/`exp`/`power`) where #135 answered empty: its value now reads, so `IsValuelessQuantity` no longer held | both, high/medium | **Accepted.** `IsValuelessQuantity` counts a value beyond the bound as no amount. `TestAQuantityWithAValueBeyondTheBoundComparesAsNothing` now runs 30 expressions over four values and two units in R4/R4B/R5; it failed before the fix |
| 2 | A number `=` a FHIR Quantity of unit '1' beyond the bound was false (the decimal side now reads) | Opus, medium | **Accepted.** `equalPair`: a number facing a quantity whose value overflows is unknown, as two quantities are (`overflowsAgainstNumber`). A number beyond the bound against a quantity within it stays false: the two cannot be equal, and = says so exactly (sweep `V = 1 '1'`, `1 '1' = V`) |
| 3 | `money()` still refused an amount beyond the bound, so `MoneyEqual` never saw one: two equal Money values, one with an id, were unequal | both, medium/low | **Accepted.** `money()` reads with `ReadDecimal`; `TestMoneyBeyondTheBoundComparesByAmount` (failed before) |
| 4 | `convertsToDecimal()` of a written-out string untested: the row applied it to `toDecimal()`'s result | Opus, low | **Accepted.** The row tests the string, a negative one too; M24 caught |
| 5 | The static guard sees only direct shopspring calls, not `types.Decimal`/`Quantity` methods that rescale | Opus, low | **Rejected as a change, kept as a limit:** those methods are reached at runtime by the sweep over every function and operator with a 1 MiB allocation bound, which is the oracle for them |
| 6 | `1e-5000000 'mg' ~ 0 'mg'` false while `1e-5000000 ~ 0` true | Opus, low | Already out of scope (Quantity stays as #135 left it) |
| 7 | `power(n)` for negative n unpinned | Fable, medium | **Accepted.** Rows `power(1)`, `power(-1)`, `power(-2.5)`; M29 caught |
| 8 | Object `=`/`~` had no cost oracle; reordered keys never reached `equalValues` | Fable, medium (and Opus' surviving mutation) | **Accepted.** `TestObjectsHoldingADecimalBeyondTheBoundCompareExactly` has `1e5000000` against `1`, keys in either order, under a 50 ms bound; M28, M30 caught |
| 9 | `Quantity.inRange` checking one side survived | Fable, low | **Accepted.** Sweep `1 '1' = V`, `1 '1' ~ V`; M31 caught |
| 10 | `InDecimalRange` counted the digits of a coefficient far beyond the bound, 0.5 s per guard on ten to the five millionth written out | Fable, low | **Accepted.** A lower bound on the digits from the bits rejects it first; `TestACoefficientFarBeyondTheBoundIsToldWithoutCounting` (536 ms before, under 50 ms after) checks every length to 1100 digits against the count; M32, M33 caught. `leadingPlace` still counts (0.46 s against `1e-3000`): only on a comparison of such a value, left |
| 11 | Plan said Java's `toString()` is the text as written | Fable, low | **Accepted in part.** Measured on the battery's four forms it is; the plan now says so and that others (`1E1000`) are not measured |
| — | `roundTo` `>=` → `>` survives | Fable | Equivalent: `Round` at the value's own exponent is the identity |

New mutations M24–M33 (scratchpad `mut/mutate010.py`); the full set runs before round 2.

### Round 2 (2026-10-27, Fable and Opus, on round 1's findings and the new diff)

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | A number against a FHIR Quantity beyond the bound of another dimension became empty; HEAD and the in-range rule (`2 = 2 'mg'` false) say false, which the unit alone decides | Opus medium, Fable low | **Accepted.** `numberAgainstOverflow`: of another dimension unequal, of the same unknown, from the unit of `ObjectValue.OverflowingQuantity` (the quantity holding the value as read; `AsQuantity` split into `quantityValue` and `quantityOf`). Rows `Observation.value = 2`, `2 = …`, `!= 2` by unit; M34 caught |
| 2 | A decimal beyond the bound against a quantity of unit '1' was exact under `=` but empty under `<`, `<=`, `>`: `Quantity.Compare` refused the range before seeing the units agree | Fable medium | **Accepted.** In the same unit `Compare` orders with `CompareDecimals`, nothing converted. Rows `V > 1 '1'`, `1 '1' < V`, … over four values; M37 caught |
| 3 | Without a model, an object holding a value alone beyond the bound still raised errors in `abs`/`round`/`ceiling`/… (also at HEAD) | Opus low | **Accepted.** `IsValuelessQuantity`: such an object, of no known type, holds no amount, as one within the bound converts as a quantity of no unit. `TestAValueAloneBeyondTheBoundWithoutAModelComputesNothing`; M35 caught |
| 4 | The `UnitOne` guard against a non-number was unpinned | Opus low | **Accepted.** Rows `= 'a'`, `= true`, `= @2020`; M36 caught |
| 5 | CONFORMANCE.md said `~` of a quantity beyond the bound is false "unless the two are written alike"; it is false either way (same at HEAD) | Fable low | **Accepted:** the text fixed; row `Observation.value ~ Observation.value` false |
| 6 | Two quantities, one overflowing, as unequal survived the conformance package (caught only by `TestDecimalRange`) | Fable low | **Accepted.** Rows `Observation.value = 1 'mg'` and reversed; M38 caught by both |
| 7 | `1e-1001 ~ 0 '1'` false while `1e-1001 ~ 0` true | Fable low | Left: `~` of a quantity rounds in its unit, the out-of-scope asymmetry of round 1, #6 |
| 8 | `log(2)`, `power(2)`, `@2020 + Q` over a quantity beyond the bound give empty where in range they raise a type error | Fable low | Left: the same as a quantity written without a value (#135's order), not a regression |
| — | InDecimalRange's lower bound | both | Proved sound by both (b-bit coefficient ≥ 2^(b-1); 0.30102 < log10 2); Fable brute-forced b ≤ 20000 |

Mutations M34–M38 added (38 in all).

### Round 3 (2026-10-27, Fable and Opus, on round 2's findings and the new diff)

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | Without a model, a Money with an amount beyond the bound answered empty where with 5, and at HEAD, it is a type error: round 2's no-model branch in `IsValuelessQuantity` took it for a quantity | Opus medium | **Accepted.** The branch leaves out an object with a currency, as `quantityValue` does; the dead `t == typeObject` dropped. `TestAMoneyBeyondTheBoundWithoutAModelIsNoQuantity` (failed before); M42 caught |
| 2 | Round 2's same-unit shortcut in `Quantity.Compare` missed a quantity with no unit, which takes the other's: `V > {"value":2}` empty, `V = {"value":2}` false | both, medium | **Accepted.** The shortcut takes the condition `valueIn` already uses (same unit, or either without one), so nothing changes in range. `TestADecimalBeyondTheBoundOrdersAgainstAQuantityWithNoUnit` (failed before); M39 caught |
| 3 | `OverflowingQuantity`'s guard unpinned: reporting every FHIR Quantity as overflowing made `{"value":2} = 2` empty | both, medium (test gap) | **Accepted.** `TestANumberAgainstAFHIRQuantityByItsUnit` pins a number against in-range quantities of no unit, '1', and 'mg'; M40 caught |
| 4 | The dimensionless units other than '1' (`%`, `{count}`, no unit) against overflow unpinned | Fable low | **Accepted.** Rows in the same test (unknown); M41 caught |
| 5 | Without a model, an object with a value beyond the bound whose other fields infer no quantity (`{"value":1e1000,"reference":"x"}`) raises the overflow, not the type error it raises in range | Fable low | Left: the same at HEAD (`AsQuantity` reports the overflow before the type), an error either way |
| 6 | `in`, `|`, `distinct()` take an overflowing FHIR Quantity as the same item as itself while `=` is empty | Fable low | Left: the same at HEAD; set membership needs a Boolean |
| — | `CompareDecimals` against `Cmp` in the shortcut; the Coding case of the no-model branch | both | Equivalent mutations, as reported |

Round 3 left medium findings, all fixed with a failing test each. By the
process the review stops at three rounds and the user decides; the user asked
for a fourth.

### Round 4 (2026-10-27, Fable and Opus, on round 3's findings and its fixes)

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | A decimal beyond the bound against a quantity within it in another unit answered `=` false and `!=` true between equal values: `1e1000 = 1e997 '10*3'`, `1e-1001 = 1e-999 '%'`, where `>=` was empty; at HEAD empty | Opus medium | **Accepted.** `Quantity.EqualityUndecided` (one value beyond, units differ, neither empty) makes `=` unknown; in the same unit or against no unit false stays exact. `TestADecimalBeyondTheBoundAgainstAQuantityInAnotherUnitIsUnknown` (failed before); M43, M44 |
| 2 | A zero written beyond the bound in a FHIR Quantity (`0e2000 'mg'`) raised the overflow error from `abs()`, `round()` and the others, and `=` was empty: `quantityValue` parsed with `ParseDecimal` where `Get` reads with `ReadDecimal`; at HEAD empty | Fable medium | **Accepted.** `quantityValue` reads with `ReadDecimal` and checks the range after, so the zero is a zero. `TestAZeroWrittenBeyondTheBoundInAQuantityIsAZero` (failed before), with a model and without; M45 |
| 3 | CONFORMANCE said `=` against a number is empty for a FHIR Quantity beyond the bound; of another dimension it is false | Opus low | **Accepted.** Reworded, with the case of finding 1 |
| 4 | This plan said `Quantity.Equal`/`Equivalent`/`Compare` refuse one | Opus low | **Accepted.** Reworded |
| 5 | `V = 1 '%'` false while `V > 1 '%'` empty | Fable low | Resolved by finding 1: both empty |
| 6 | `V.toQuantity() > 1 '1'` empty while `V > 1 '1'` true | Opus low | Left: `toQuantity()` computes (the Computing class), the comparison does not |

After round 4: `make check` green, corpusdiff 0 differences in R4/R4B/R5,
mutations 45 of 45 caught (M43–M45 new; M35's pattern updated after round 3).
Round 4 left medium findings, both fixed with a failing test; the user
decides whether to run a fifth.

### Round 5 (2026-10-27, Fable and Opus, on round 4's findings and its fixes)

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | `(V | 1 '10*3').sort()` raises "decimal beyond the range this engine holds", where at HEAD V was absent | Opus low | Left: `sort` fails on a pair `<` leaves empty, as `(1 'cm' | 1 's').sort()` raises "incompatible units", also untyped |
| 2 | `V = 1 '{beats}'` (or `'10*0'`) empty while `V = 1 '1'` false: `EqualityUndecided` compares unit text, not the factor | Opus low | Left: empty where false would be exact is no wrong answer, and deciding it means converting |
| 3 | `(2 | {"value":1e1000,"code":"mg"}).max()` errors where in range it is empty | Opus low | Left: the same at HEAD (#135), untouched by round 4 |
| 4 | `quantityValue`'s new comment hard to read; `EqualityUndecided` is new public API used by `eval` only | Opus low | Comment reworded; the method stays exported, as `Comparable` is, for `eval` to call |
| 5 | A decimal beyond the bound by its scale alone (1 with 1001 zeros after the point, which is 1) answered `= 1 '1'` false and `~` false while `= 1` was true and `>=`/`<=` both true: `Quantity.Equal`/`Equivalent` refused any value beyond the bound, on the premise that it equals none within | Fable medium | **Accepted.** Beyond the bound, `Equal` and `Equivalent` compare exactly in alike units (same, or one without), as a decimal does, `unitsAlike` shared with `Compare` and `EqualityUndecided`; across units unchanged. `TestADecimalBeyondTheBoundByItsScaleEqualsAQuantityOfItsValue` (failed before); M46–M48 |
| 6 | `quantityValue` returned a bare `ErrDecimalRange`, without the value `ParseDecimal` named | Fable low | **Accepted.** Wrapped with the value again |
| 7 | A zero written past the bound on the small side reads at scale −1000 and prints 1000 zeros as a quantity | Fable low | Left: `ReadDecimal`'s nearest scale within, as the decimal path has it |

After round 5: `make check` green, corpusdiff 0 differences in R4/R4B/R5,
mutations 48 of 48 caught (M46–M48 new; M37, M39, M44, M45 patterns updated
for `unitsAlike`). Round 5 left one medium finding, fixed with a failing test;
the user decides whether to run a sixth.

### Round 6 (2026-10-27, Fable and Opus, broader than the diff)

Both ran generated sweeps of the invariants between operators (trichotomy,
`!=` as not `=`, `=` true implying `~` true, symmetry, `in`/`distinct`/`union`
following `=`) over value shapes beyond the bound by magnitude and by scale,
against numbers, System quantities, FHIR Quantity objects (R4/R4B/R5, no
model) and Money: 0 violations, no definite wrong answer against an exact
oracle; HEAD against the increment over in-range values, 0 differences.

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | A value beyond the bound by its scale alone (1 written with 1001 zeros after the point) is held as beyond everywhere though it is 1: as a FHIR Quantity it compares as nothing (`= 1 'mg'` empty, `~` false, `abs()` empty), and as a decimal `+ 1` is empty | both, medium | **Accepted** (the user chose it, 2026-10-27). `ReadDecimal` reads a value whose places past the bound are all zeros at the scale of the bound, as it reads a zero, keeping its text; the zeros are counted on the written digits, so nothing is divided. `TestADecimalPastTheBoundByZerosAloneIsReadWithin` (failed before) and `TestADecimalPastTheBoundByZerosIsReadAtTheBound`; round 5's test now uses a value past the bound by a digit (`TestADecimalPastTheBoundByADigitComparesAsAgainstTheNumber`) |
| 2 | Such a value `~ 100 '%'` is false where in range it is true | Fable low | Fixed by 1 for such values; past the bound by a digit, `~` across units stays false |
| 3 | The plan says `precision()` overflows to empty; it answers from the text | Fable low | **Accepted.** Step 3 marked superseded |
| 4 | `OverflowingQuantity` parses the value twice | Fable low | Cost only, on the overflow path |
| 5 | `(V | -1).min()` with a FHIR Quantity beyond, `V in (Q)` true while `=` empty, `sort` errors | Opus low | Known: rounds 3 #6, 5 #1 and #3 |

After round 6's fix: `make check` green, corpusdiff 0 differences in
R4/R4B/R5, mutations 49 of 49 caught (M49–M51 new; M46 and M48 dropped as
equivalent now that no value past the bound equals one within it; M16 and M20
patterns updated). A seventh, short round reviews the reading change.

### Round 7 (2026-10-27, Fable and Opus, short, on the reading change)

Both checked the text handling against `big.Rat` over 20,000 and 55,055
spellings (signs, leading zeros, a point on either side, `e`/`E`, exponents
to int32 and past it): the value never changes and the text is kept. Callers
(`Get`, `quantityValue`, `money`, `toDecimal`) behave as for a zero.
**Converged: no finding of medium severity or higher.**

| # | Finding | From | Resolution |
|---|---------|------|------------|
| 1 | No case with trailing zeros exactly the excess: `<` → `<=` survived | both, low | **Accepted.** Rows `10e-1001` and `1.0…050` (a zero, fewer than the excess); M51, M52 |
| 2 | A value held past the bound with an exponent below −1000 copied its text though its last digit is no zero | Opus low | **Accepted.** The last digit is checked first |
| 3 | `ImplicitPrecision` counts exponent text (`100.0e-3` → 4), now reachable for values read within | Opus low | Left: FP-5, out of scope, as round 1 #12 |
| 4 | `'1.' + 1001 zeros` as a string: `toDecimal()` reads it, `toQuantity()` refuses it (`ParseDecimal`), as it refuses a zero written so | Fable low | Left: the same for a zero before round 7; a string beyond the bound converts as an overflow, as CONFORMANCE has it |

At closing (2026-10-27): `make check` green, corpusdiff 0 differences in
R4/R4B/R5 (521,007, 308,577 and 360,675 evaluations), mutations 50 of 50
caught (`mut/mutate010.py`).

