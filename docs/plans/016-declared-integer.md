# 016 — What a declared integer type reads (#251)

Status: closed (step 9), 2026-11-02; review converged in round 3
Issue: #251

## Problem

#251 reports that a `valueInteger` holding `1.5`, `1e2` or `1.0` is "neither
Integer nor Decimal" (`is Integer` false, `is Decimal` false) while `+ 1`
answers `2.5`, and asks what a declared `integer`, `unsignedInt`,
`positiveInt` and R5 `integer64` read a non-integer JSON number as.

## Prior review

### The specification

- **FHIR datatypes** (R5 page; R4 and R4B alike for the first three):
  - `integer`: "A signed integer in the range −2,147,483,648..2,147,483,647
    (32-bit; for larger values, use decimal)"; JSON "JSON number (with no
    decimal point)".
  - `unsignedInt`: "Any non-negative `integer` in the range
    0..2,147,483,647"; `positiveInt`: "Any positive `integer` in the range
    1..2,147,483,647"; both a JSON number.
  - `integer64` (R5 only): "A signed integer in the range
    -9,223,372,036,854,775,808 to +9,223,372,036,854,775,807 (64-bit)"; JSON
    "JSON String (due to issues with precision in floating point
    libraries)".
  - "The regexes are provided to assist with tooling, but are informative,
    not normative."
- **FHIR JSON** (R5 json.html §2.1.6.4.2): "The FHIR types integer,
  unsignedInt, positiveInt and decimal are represented as a JSON number …
  and all other types (including integer64) are represented as a JSON
  string". The page does not say what a primitive of the wrong JSON type
  reads as.
- **The packages** (`StructureDefinition-<type>`, `.value`, read
  2026-11-02):

  | Type | R4 | R4B | R5 |
  |---|---|---|---|
  | `integer` | `System.Integer`, `-?([0]\|([1-9][0-9]*))`, min/max 32-bit | same | `System.Integer`, `[0]\|[-+]?[1-9][0-9]*`, min/max 32-bit |
  | `unsignedInt` | `System.String`, fhir-type `string`, `[0]\|([1-9][0-9]*)` | `System.String`, fhir-type `unsignedInt` | as R4B |
  | `positiveInt` | `System.String`, fhir-type `string`, `[1-9][0-9]*` | `System.String`, fhir-type `positiveInt` | as R4B |
  | `integer64` | — | — | `System.Integer`, `[0]\|[-+]?[1-9][0-9]*`, no min/max |

### This engine and the references (measured 2026-11-02)

Three probe batteries, R4 and R5, this engine at main (`84be8dc`),
fhirpath.js 5.2.0 and the HL7 validator 6.10.2: 448 + 146 + 32 cases
(scratchpad `b251`, `b251b`, `b251c`; they move into the repo with the
tests).

1. **`is Integer` is false for every FHIR integer**, valid ones included
   (`100`), in all three engines: a FHIR `integer` is not `System.Integer`.
   So the issue's headline is not what sets `1.5` apart. `is FHIR.integer`
   is true for `1.5` and for `100` here and in fhirpath.js (the validator
   answers false for both).
2. **A non-integer number in `integer`, `unsignedInt` or `positiveInt`**
   (`1.5`, `1e2`, `1.0`, `1E0`, `-1.5e1`) is read as the number it is,
   typed as declared. This engine and fhirpath.js agree on `+ 1`, `* 2`,
   `div 1`, `> 1`, `= 1.5`, `= 1`, `toDecimal()`, `toInteger()` of `1.5`,
   `type().name`, `exists()`, `hasValue()`. They part on:
   - `toString()` of `1e2`, `1E0`, `-1.5e1`: fhirpath.js has lost the text
     (JS-8);
   - `(1.0).toInteger()` and `convertsToInteger()`: fhirpath.js `1` and
     true, this engine and the validator empty and false (CONFORMANCE.md,
     `toInteger()` takes no Decimal);
   - `positiveInt` `1.5`, `ofType(integer).count()`: fhirpath.js 1, this
     engine and the validator 0 (subtype question, not this issue).

   The validator ends the evaluation on arithmetic over these (it does so
   on a valid `unsignedInt` and `positiveInt` too, already recorded in
   CONFORMANCE.md), and where it answers it reads `1.5`: `= 1.5` true,
   `> 1` true, `toDecimal()` 1.5.
3. **Out of range or sign** (`integer` 2147483648, -2147483649,
   9223372036854775808, 1e400; `positiveInt` 0, -1; `unsignedInt` -1): read
   as written by this engine and fhirpath.js; the validator ends the
   evaluation. Integer is 64 bits here by decision (CONFORMANCE.md).
4. **An `integer64` written as the JSON string FHIR requires** (`"5"`,
   `"9223372036854775807"`) is a String typed `integer64` here: `+ 1`,
   `* 2`, `div 1` end the evaluation, `> 1` is a type error.
   fhirpath.js reads it as a number (BigInt): `6`, `10`, `5`, true. The
   validator ends the evaluation or answers as for a string. The official
   R5 examples hold four (`Attachment.size`, e.g. `"size":"104274"`). Of
   other string forms, fhirpath.js reads `"+5"`, `"-0"`, `"01"`, `" 5"` and
   `""` (as 0) through BigInt; the validator reads none as a number.
5. **An `integer64` written as a JSON number** (`5`, `1.5`): read as the
   number, typed `integer64`; fhirpath.js reads `5` and errs on `1.5`.
6. **An `integer` written as a JSON string** (`"1"`): a String typed
   `integer` here (`+ 1` ends the evaluation). fhirpath.js and the
   validator both give `+ 1` = 2 and `* 2` = 2, and split on `> 1` and
   `div 1`.

### `gofhir/server` (frozen)

It validates a resource with go-validator before calling this engine, and
go-validator checks each integer type's regex (`pkg/primitive`): `1.5` in
an `integer` never reaches the engine from it. **Discarded** as a model,
since the engine reads what it is given; kept as evidence that a
validating caller never sends findings 2, 3 and 6. Finding 4 is valid
data and does reach it.

### Precedent in this engine

- #231 (CONFORMANCE.md, "A FHIR date, dateTime, instant or time is read by
  FHIR's form"): "A value outside its form is the string it is, typed as
  declared".
- #238: a token outside JSON's grammar is no number and reads as null.
- "Integer is 64 bits here, and there is no Long".

### Take, improve, discard

- **Take** the reading of findings 2, 3 and 5 (the value as written, typed
  as declared): it is #231's rule, fhirpath.js agrees, and the validator
  ends the evaluation rather than answering otherwise. Record it in
  CONFORMANCE.md with tests, which today do not exist.
- **Improve** finding 4: a string declared `integer64` holding an integer
  is an Integer typed `integer64`. Which string forms (R5's regex admits
  `+5` and refuses `-0`, `01`) is a decision for the plan.
- **Leave** finding 6 as #231 reads it (the string it is, typed as
  declared) unless the plan argues otherwise: the references agree only
  on some operations, and the text says nothing.

## Plan

### Decisions (each with the recommendation first; revised after the adversarial round)

- **D1, findings 2, 3 and 5 (a number outside the type's form or range):**
  keep reading the number as written, typed as declared, as #231 reads a
  date outside its form. fhirpath.js reads it alike; the validator ends
  the evaluation. Alternatives: null, as #238 reads a token outside JSON's
  grammar (but `1.5` is a JSON number, and no reference drops it); an
  Integer when the value is whole (`1e2` → 100), which neither reference
  does. No code change: CONFORMANCE.md section and tests.
- **D2, finding 4 (`integer64` as the JSON string FHIR requires):** a string
  declared `integer64` reads as an Integer typed `integer64` when D3 reads
  it, and otherwise stays the string it is, typed as declared (#231).
  json.html §2.1.6.4.2 is the rule: "integer, unsignedInt, positiveInt and
  decimal are represented as a JSON number … all other types (including
  integer64) … as a JSON string". Of the types written as strings,
  `integer64` is the only one whose value is an integer (its `value` is
  `System.Integer` in R5's package, and the evaluator already maps it to
  Integer, `eval/evaluator.go` `fhirPathSpecMap`). It is a rule-5
  exception, from prose, as the temporal forms in `fhirString` are.
- **D3, the string's form:** `strconv.ParseInt(s, 10, 64)`, i.e.
  `[+-]?[0-9]+` within int64: `+5`, `-0`, `01` are read; `" 5"`, `""`,
  `1e2`, `1.5` and anything past int64 are not. It is how gofhir/models
  reads an `integer64` (`r5/v2` `Integer64.UnmarshalJSON`), FHIRPath's
  `toInteger()` form, and fhirpath.js's BigInt for those values (BigInt
  also reads `" 5"` and `""`, which json.html rules out: no surrounding
  whitespace, no empty string). Alternative: R5's informative regex
  `[0]|[-+]?[1-9][0-9]*`, which refuses `-0` and `01`; integer64 exists
  only in R5, so no form crosses versions.
- **D1 against D2 past int64:** a JSON number past int64 in an `integer` is
  still a number (a Decimal, D1); a string past int64 in an `integer64` is
  no value of the type's range, and stays a string. Stated and tested.
- **D4, finding 6 (`integer` written as a JSON string, `"1"`):** stays the
  string it is, typed `integer` (#231): json.html writes an `integer` as a
  number, and the references do not read it as one consistently (`= 1`:
  here and fhirpath.js false, the validator true; `> 1` and `div 1` split
  too; fhirpath.js's `+ 1` = 2 is JavaScript's coercion). Documented, with
  its rows.
- **D5, where the rule lives:** in `fhirString`, the single funnel for a
  declared string, beside the temporal types. No exported signature
  changes, no cache key changes, and it holds wherever a type is declared:
  with a model, without one through a choice's suffix (`valueInteger64`,
  `GetCollectionParsedAs`), and for go-searchindex
  (`GetCollectionWithType`, `adr001.go:248`). Rejected: deriving it from
  the model at read time (the first draft): it needs a resolver in every
  typed read, splits the no-model and searchindex readings from the
  evaluator's, and the model's own primitive value types are not reliable
  (SPEC-8).

### Scope

- In: D1 and D4 recorded and tested; D2, D3 implemented (D5).
- Out:
  - A string in a type FHIR writes as a JSON number or boolean (`"1"` in
    an `integer`, `"true"` in a `boolean`, `"1.5"` in a `decimal`): the
    string it is (D4).
  - Whole-object `=` and `~` (`types/json_equal.go`,
    `json_equivalent.go`) compare the JSON as written, untyped: two
    Attachments with `size` `"5"` and `"+5"` stay unequal while their
    sizes are equal. Documented with a row, as a known split.
  - Deriving `fhirPathSpecMap` (eval/evaluator.go) from the model: a
    pre-existing hardcoded table, follow-up issue.
  - `unsignedInt` and `positiveInt` written as strings: the packages type
    their `value` `System.String` (R4, R4B, R5), while the datatypes page
    calls them integers and json.html JSON numbers. Recorded as SPEC-8;
    such a string stays a String.
  - `positiveInt`'s `ofType(integer)` (fhirpath.js 1, this engine and the
    validator 0): subtyping, not this issue.
  - The validator's errors on arithmetic over `unsignedInt`,
    `positiveInt` and `integer64`, already in CONFORMANCE.md.
  - Range checks (a `positiveInt` 0, an `integer` past 32 bits): Integer
    is 64 bits by decision.

### Exit criterion

- `conformance/declared_integer_test.go` passes in R4, R4B, R5 and without
  a model, with each row's expected value from the text or from both
  references where they agree.
- `difftest/batteries/declared-integer` (the three probes, reduced to what
  the plan decides): every difference classified in this plan with the
  reference's own answer.
- `make corpusdiff BASE=main`: the only differences are R5 `integer64`
  values (the four `Attachment.size` of the official examples), each
  listed.
- `make check` green; mutations caught; bench-compare without regression
  (D5 adds a comparison per declared string).

### Tests and data

- Hand-written rows (text cited): each finding's values × `+ 1`, `* 2`,
  `div 1`, `> 1`, `= 1`, `~ 5`, `toInteger()`, `toString()` (`"+5"` gives
  `5`: an Integer keeps no text), `is FHIR.<type>`, `type().name`,
  `hasValue()`; with an element (`_size` with an extension) to check it
  is kept; in an array; in an Extension's `valueInteger64`; with and
  without a model. D3 rows: `5`, `+5`, `-0`, `01`, `-5`,
  `9223372036854775807`, `-9223372036854775808`, `9223372036854775808`,
  `" 5"`, `""`, `1e2`, `1.5`.
- Metamorphic: an R5 `integer64` `"5"` and an `integer` `5` answer alike
  for the arithmetic and comparisons above; a key reordered or an unknown
  extension added changes nothing. Not alone: the battery checks the
  valid forms against fhirpath.js's BigInt answers.
- Fuzz: `FuzzInteger64Form`: a string is read exactly when
  `strconv.ParseInt` reads it, and then to its value.
- Official examples: corpusdiff over R4, R4B and R5.
- Three engines: the battery above.

### Exceptions to rule 5

- D2 and D3: `integer64` named in `fhirString`, from json.html's prose,
  as the temporal types are; recorded in CONFORMANCE.md.

### New dependencies

None.

## Adversarial round (2026-11-02)

Codex was at its usage limit until 2026-11-04; the user allowed an
independent Opus reviewer until it returns. Ten findings:

| # | Finding | Resolution |
|---|---|---|
| 1 | High: without a model a choice's suffix still types the value (`GetCollectionParsedAs`), so "nothing changes" was false | Accepted, verified (types/object.go 2355): D5 now in `fhirString`, which every declared read passes, with and without a model |
| 2 | High: D5 as drafted changed the exported `GetCollectionWithType`, which go-searchindex calls (`adr001.go:248`) | Accepted, verified: no signature changes |
| 3 | Medium: the per-type cache and cache-key change were not needed | Accepted: no model lookup at read time at all |
| 4 | Medium: R5's regex applied to `integer` in every version through D4; gofhir/models reads `integer64` with `ParseInt` | Accepted, verified (`r5/v2` `integer64.go:56`): D3 is `ParseInt`; D4 dropped, so no form crosses versions |
| 5 | Medium: D1 and D2 disagree past int64 and on `-0` | Accepted: `-0` now read (D3); past int64 stated and tested |
| 6 | Medium: D4's evidence was weak (`= 1` splits; fhirpath.js coerces) and it reversed #231 | Accepted: D4 now keeps the string (#231) |
| 7 | Medium: reading only Integer from the model was a hardcoded choice; `fhirPathSpecMap` hardcodes unsignedInt/positiveInt | Accepted in part: the rule is now json.html's prose, a stated exception; the map is a follow-up |
| 8 | Medium: whole-object `=`/`~` compare JSON untyped and go out of step with the children | Accepted as a known split, documented with a row (out of scope) |
| 9 | Low: `"+5".toString()` gives `5`; rows for `_size`, arrays | Accepted: rows added |
| 10 | Low: probes ran on a dirty tree, no R4B in the batteries; the metamorphic test checks only itself | Accepted in part: batteries rerun clean and moved into difftest at step 7, with fhirpath.js's answers as the outside oracle. R4B rejected for the batteries: the harness and fhirpath.js have no R4B model (`difftest/battery.go` `fhirVersion`); R4B is covered by the Go tests |

## Validation (2026-11-02)

- `make corpusdiff BASE=main`: 0 answers differ over R4, R4B and R5
  (521,007, 308,577 and 360,675 evaluations). The exit criterion expected
  the four R5 `Attachment.size` values to differ; they cannot, since no
  corpus expression reaches them: the corpus lists a resource's own
  element paths (`DocumentReference.content.attachment`), not a
  datatype's beneath them (`build/corpusdiff/r5-head.tsv` has no
  `attachment.size` row). The change is covered by the conformance tests
  and the battery below. Whether the corpus should descend into datatypes
  is a follow-up, not this increment.
- Three engines, `make three-engine BATTERY=batteries/declared-integer/battery.tsv DIFFTEST_ARGS="-v -jobs 2 -java ../build/validator/validator_cli-6.10.2.jar"`
  (170 cases, R4 and R5): 82 agree; 73 differ in the validator alone, 8
  in fhirpath.js alone, 2 in this engine alone, 5 in all three. Each class,
  with the references' own answers:
  - **This engine alone (2): `"1"` in an `integer`, `+ 1`.** Both
    references give 2; this engine ends the evaluation. Deliberate, D4: the
    string it is (#231), since json.html writes an `integer` as a number;
    the references split on `= 1` (the validator true, fhirpath.js false
    with this engine) and on `> 1`.
  - **fhirpath.js alone (8):** its BigInt reads `"9223372036854775808"`,
    past the type's range ("-9,223,372,036,854,775,808 to
    +9,223,372,036,854,775,807"), which this engine and the validator keep
    a string (D3); it errs on `"1e2"` and `"1.5"`, which the other two read
    as strings; and its float loses `9223372036854775807.toDecimal()`
    (JS-8's class).
  - **The validator alone (73):** it ends the evaluation on arithmetic and
    ordering over `integer`, `positiveInt` and `integer64` values, valid
    ones included (`"5" + 1`; already in CONFORMANCE.md for
    positiveInt); it reads `integer64` as a string (`"+5" = 5` false,
    `toDecimal()` `+5`; JAVA-15); it capitalizes `type().name`
    (`Integer64`, JAVA-16); and it reads `"1" = 1` true in an `integer`.
  - **All three (5):** `"9223372036854775807" + 1` and `* 2` are empty
    here, as the specification has overflow ("Operations that cause
    arithmetic overflow or underflow will result in empty"), where
    fhirpath.js's BigInt goes past int64 and the validator errs;
    `"1e2".toDecimal()` is empty here, since FHIRPath's string form for a
    Decimal has no exponent, where the validator reads `1e2` and
    fhirpath.js errs; and the type name of `"1e2"` and `"1.5"` (fhirpath.js
    errs, the validator capitalizes).

## Mutations (2026-11-02)

Sets 010–016, one mutant at a time on a copy. 016's set (`mut/mutate016.py`):

| # | Mutant | Caught by |
|---|---|---|
| I1 | integer64 string not read | `FuzzInteger64Form`, conformance rows |
| I2 | read but not typed integer64 | `FuzzInteger64Form` (its type check restored in round 2), `TestAnInteger64StringIsRead` |
| I3 | not read without a model (`parseTypedString`) | `FuzzInteger64Form` |
| I4 | form widened to trimmed text (`" 5"`) | `TestAnInteger64OutsideTheFormStaysAString` |
| I5 | form narrowed: no `+` | `TestAnInteger64StringIsRead` |
| I6 | form narrowed to 32 bits | `TestAnInteger64StringIsRead` |
| I7 | an `integer` string read too (D4 reversed) | `TestAnIntegerWrittenAsAStringStaysAString` |
| I8 | `ParseInt` in base 0 (`0x5`) | `FuzzInteger64Form` |
| I9 | a whole number in an `integer` read as Integer (D1 reversed) | `TestANumberOutsideAnIntegerFormIsReadAsWritten` |

All 157 mutants of 010–016 are caught. 010's M01 had lost its pattern when
015 moved the declared decimal's read into `declaredDecimal`; repointed
there, it is caught.

The runner was rewritten for cost (user, 2026-11-02): the first run copied
the repo per mutant and ran both whole suites, about 4 h 25 min for the
157. `mut/mutfast.py` keeps one working copy, applies and reverts each
mutant in place, runs first only the tests that caught it before, in
their packages, with `-failfast`, and the whole suites (`-failfast`) only
when those do not catch it. A whole suite takes about 15 s, so a hang is
cut at 60 s (G9 and D8 loop forever). Measured: 1,537 s for the 157 with an
8-minute cut, of which G9 and D8 took 966 s; with the 60 s cut they take
61 s and 63 s.

## Code review

Codex was still at its usage limit; an independent Opus reviewer, as the
user allowed (2026-11-02).

### Round 1 (2026-11-02)

No high finding; the reading held on every path probed (children(),
descendants().ofType(integer64), repeat(), as, is, a Bundle entry,
Parameters.part, an element's extension).

| # | Finding | Resolution |
|---|---|---|
| 1 | Medium: CONFORMANCE.md cites the battery for answers it does not hold (`" 5"`, `""`, `"5x"`, `"-9223372036854775808"`; `toString()`, `toInteger()`, `div`, `~`) | Confirmed (battery.tsv had none). 94 rows added and run on the three engines |
| 2 | Medium: without a model the rule runs in a second copy (`parseTypedString`), and holds only through a choice's `.value`: `Patient.extension.valueInteger64 + 1` is an error, its type `String` | Confirmed by a probe (`valueDecimal` is `Integer` alike without a model, pre-existing). One copy, in `fhirString`, matched case-insensitively; the reach stated in CONFORMANCE.md, with a row |
| 3 | Low: `parseTypedString`'s comment stale | Gone with 2 |
| 4 | Low: no rows pin the navigation paths | Rows added (R5) |
| 5 | Low: a value outside the form without a model only in the fuzz seeds | Rows added |
| 6 | Low: the fuzz test loops over one type and checks less without a model | Fixed |
| 7 | Low: dropping `read = true` in `parseInteger64` survives | Equivalent: a value typed `integer64` is a FHIR primitive by its name. Kept, as every typed read marks it; not in the mutation set |

Seen in passing, out of scope: without a model, `ofType(integer64)` keeps
any Integer (a `valueDecimal` 5 among them), through the hardcoded
`fhirPathSpecMap`; the follow-up that derives it from the model covers it.

The 94 rows added after round 1 (run alone, then kept in battery.tsv; the
battery now holds 264): 29 agree; 36 differ in fhirpath.js alone, 25 in the
validator alone, 4 in all three; none in this engine alone. They fall in
the classes above:
- fhirpath.js alone: its BigInt reads `" 5"` and `""` (as 0), which
  json.html rules out ("no leading and trailing spaces for non-strings";
  "String property values can never be empty") and this engine and the
  validator keep strings; it errs on `"5x"`, `"1e2"`, `"1.5"`; its float
  loses `-9223372036854775808` and past it; it has lost the text of `1e2`
  (JS-8); and it converts a Decimal with `toInteger()` (`1.0` → 1, `1e2` →
  100), which this engine and the validator do not (CONFORMANCE.md,
  Conversions).
- The validator alone: it ends the evaluation on `div` over these types;
  it reads `integer64` as a string (`"+5".toString()` `+5`, `"01"` `01`,
  `"-0"` `-0`, `"+5" ~ 5` false; JAVA-15); it capitalizes the type
  (JAVA-16).
- All three: overflow is empty here (`"-9223372036854775808" * 2`), where
  fhirpath.js goes past int64 and the validator errs; `toInteger()` of the
  int64 bounds is the bound here, empty in the validator (a 32-bit Integer,
  CONFORMANCE.md), and rounded by fhirpath.js's float; `"5x".type().name`
  is `integer64` here, capitalized in the validator, an error in
  fhirpath.js.

Mutations after round 1's fixes (`mutfast.py`, 802 s): all 157 of 010–016
caught; I3 and I7 repointed at the single rule in `fhirString`.

Found in passing, out of scope: `TestACoefficientFarBeyondTheBoundIsToldWithoutCounting`
(#236, types/decimal_beyond_bound_test.go) asserts a wall-clock bound
(`took > 50*time.Millisecond`). Under load it failed with an unrelated
mutant (I2, which `TestAnInteger64StringIsRead` catches in 4 s when run
alone), so it can mark a survivor caught. It is how M32 (digits counted)
is caught; a bound that does not depend on the machine's load is a
follow-up.

### Round 2 (2026-11-02)

No high finding. The reviewer reran the whole battery (264 cases) and
found the plan's counts and classes exact, and the single rule alike with
and without a model (`type().name` `integer64` in both).

| # | Finding | Resolution |
|---|---|---|
| 1 | Medium: CONFORMANCE.md still stated reference answers the battery did not hold: `is Integer` false in the three engines, `is FHIR.integer`, `1.0 = 1` | Confirmed. 14 rows added and run alone: `is Integer` false in all three for `1.5`, `1.0`, `100`; `is FHIR.integer` true here and in fhirpath.js, false in the validator; `1.0 = 1` true here and in fhirpath.js, false in the validator. CONFORMANCE.md now says both validator answers. The battery holds 278 cases |
| 2 | Low: the fuzz test no longer checked the type, so I2 was caught only in conformance | Fixed: it checks the type is integer64; I2 caught by it alone in 2 s |

### Round 3 (2026-11-02)

No finding of medium or higher severity: the review converged. Round 2's
two findings resolved (the 14 rows are in battery.tsv and agree with the
two sentences; a mutation of the integer64 type fails the fuzz test).
Every reference answer the CONFORMANCE.md section gives is a battery row.
One low left as is: that R4 and R4B read a string `Attachment.size` as a
String typed `unsignedInt` is a claim about this engine only, checked by
`conformance/declared_integer_test.go`, with no reference answer stated.

## Closing (2026-11-02)

- `make check`: green (exit 0).
- `make bench-compare BASE=main`: geomean +1.19% in time, +0.01% in bytes,
  +0.00% in allocations; three benchmarks flagged at ±18–30% spread
  (ResolveBundleURN, two AbsentField) were measured again alone with
  `COUNT=10 scripts/bench-compare.sh main 'ResolveBundleURN|AbsentField'`:
  none differs (p 0.35–1.0).
- Follow-ups, not filed: derive `fhirPathSpecMap` from the model (it also
  makes `ofType(integer64)` keep any Integer without a model); a bound
  independent of the machine's load for #236's wall-clock test; corpusdiff
  descending into datatypes. Upstream, unpublished: SPEC-8, JAVA-15,
  JAVA-16.
