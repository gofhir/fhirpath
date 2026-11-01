# 015 — A resource's number is read by JSON's grammar (#238)

Status: closed (step 9), 2026-11-01; review converged in round 2
Issue: #238

## Problem

A number a resource holds is read by `decodeDecimal` (`types/decimal.go`),
which keeps shopspring/decimal v1.4.0's accept set: a point with digits on
one side only, leading zeros, an exponent after a bare point. So a resource's
`-.5`, `5.`, `01`, `007.50` and `1.e5` are read as numbers, and `01` in an
untyped position becomes the Integer 1. None of them is a FHIR decimal, a
JSON number or a FHIRPath literal. The accept set was inherited, not
decided; #233 settled the same question for dates.

## Prior review

### The specification

- **JSON** (RFC 8259 §6, which FHIR's JSON format builds on):
  `number = [ minus ] int [ frac ] [ exp ]`, `int = zero / ( digit1-9
  *DIGIT )`, `frac = decimal-point 1*DIGIT`, `exp = e [ minus / plus ]
  1*DIGIT`. "Leading zeros are not allowed"; no `+` before the number.
- **FHIR decimal**, the `regex` extension of `decimal.value` in the pinned
  core packages:
  - R4 and R4B: `-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?`, which is
    exactly JSON's number;
  - R5: `-?(0|[1-9][0-9]{0,17})(\.[0-9]{1,17})?([eE][+-]?[0-9]{1,9}})?`
    (a stray `}`), which R5's datatypes page calls "informative, not
    normative … may allow a broader set of values than are actually
    valid". Its digit limits are those #232 decided not to apply: a decimal
    beyond the bound is read (CONFORMANCE.md).
- **FHIR integers**: R4 and R4B `integer` `-?([0]|([1-9][0-9]*))`,
  `unsignedInt` `[0]|([1-9][0-9]*)`, `positiveInt` `[1-9][0-9]*`; R5
  `integer` and `integer64` `[0]|[-+]?[1-9][0-9]*` (admits `+1`, which is
  no JSON, and refuses `-0`, which is).

### This engine (probe, R4, R4B and R5 models alike, and without a model)

| Written | `probabilityDecimal` / `Quantity.value` | untyped number | `valueInteger` |
|---|---|---|---|
| `-.5`, `5.`, `1.`, `007.50`, `1.e5` | Decimal, as written | Decimal | a Decimal typed `integer` |
| `01`, `00`, `-01` | Decimal, as written | Integer 1, 0, -1 | Integer 1, 0, -1 |
| `1.5`, `1e2`, `1.0` | Decimal | Decimal | a Decimal typed `integer`: neither `is Integer` nor `is Decimal`, but `+ 1` is 2.5 |
| `--1`, `1x`, `0x10`, `0e`, `Infinity` | empty | empty | empty |

What reaches the engine (jsonparser v1.1.1, `Get` and `ArrayEach`,
measured): `.5` and `+1` are refused by the tokenizer (unknown type); every
other run of characters up to a delimiter is a "number" token, as written:
`1.e5`, `-`, `1e`, `1.5.5`, `1e5e5`, `1-2`, `0x1`, `1_000`, `-.5`, `01`.
`1 2` is read as `1`, the rest unseen (document validation, out of scope).

A number that fails to decode today drops its primitive's element too:
`"valueDecimal":1x` with `"_valueDecimal":{"extension":[…]}` reads
`extension.value` as nothing, `hasValue()` false, and the extension is lost
(`types/object.go` `pairValueWithElement`); `null` in its place keeps the
element and its extension.

Malformed JSON elsewhere reads as nothing (`"active":tru` → empty; an
unclosed array → empty), and a trailing comma or trailing text is accepted:
the engine reads lazily with jsonparser and validates no document.

### The references (measured 2026-10-31; probe battery and commands in the
scratchpad, to be moved into the repo with the tests)

- **fhirpath.js 5.2.0** takes a parsed object; `JSON.parse` refuses every
  form above (`SyntaxError`), so such a resource never reaches it. The
  difftest harness cannot carry one either: its batch is JSON.
- **HL7 validator 6.10.2** (`-fhirpath`, R4) reads them: `-.5 + 1` → `0.5`,
  `01 + 1` → `2`, `007.50 + 1` → `8.50`, `1.e5 + 1` → `100001`. A
  `valueInteger` of `1.5`, `1e2` or `1.0` ends the evaluation
  (`NumberFormatException`); `-0` is read.

The references part. By the three-engine rule the text decides: RFC 8259
and FHIR R4/R4B refuse these forms, as fhirpath.js (through `JSON.parse`)
does.

### `gofhir/server` (frozen)

It never reads such a value: every resource it accepts passed
`encoding/json` (`json.Valid`, `Unmarshal`), which refuses these forms, and
it called this engine afterwards. **Discarded** as a model for the engine,
which is given bytes and does not validate a document; kept as the evidence
that a conforming caller never sends them.

### Other callers of the decimal reader

- `toDecimal()` and `convertsToDecimal()` read a String by FHIRPath's own
  form, `(\+|-)?\d+(\.\d+)?`, then `ReadDecimal`; `convertsToDecimal()`
  passes the raw text (`'007'`, `'+1.5'`). So the grammar cannot go into
  `ReadDecimal`; it goes where a resource's number is read.
- `types.NewDecimal` and `ParseDecimal` (the API) and the quantity literal
  parser keep their accept set, as the issue allows; documented.
- Object equality (`json_equal.go`, `json_equivalent.go`) decodes two
  numbers to compare them.

## Plan

1. **One grammar for a resource's number:** `isJSONNumber(text)`, RFC 8259
   §6, written in the engine (the R5 regex is informative and malformed; R4
   and R4B's equals JSON's). It is checked wherever a resource's number
   becomes a value: the untyped path (`jsonValueToFHIRValue`), a declared
   `decimal` (`jsonValueToFHIRValueWithType`), a Quantity's and a Money's
   value (`object.go` 2702, 2793, 2816), the System-typed path
   (`systemValue`), the Integer route, and `quantityType`'s detection; and
   the two comparisons of objects (`compareNumbers` in `json_equal.go`,
   `jsonEquivalent` in `json_equivalent.go`): a number outside the grammar
   compares as one that does not decode does today (undecided, not equal),
   except against its own identical text, which both shortcuts already
   take as alike.
2. **A number outside it** is read per D1.
3. With or without a model alike, **as to the grammar**: what is accepted
   is JSON's, not a FHIR type's. The type a valid number is read as stays
   as today (untyped `1` an Integer, a declared decimal a Decimal), and
   differs by model and version as it does now.
4. CONFORMANCE.md: a section on the rule, the measurements and the
   exception to reading everything from the packages (the grammar written
   in the engine, RFC 8259's, as 009 recorded its forms there; the repo
   keeps no ADR folder).
5. docs/upstream-findings.md: JAVA-14 (the validator reads non-JSON
   numbers), SPEC-7 (R5's decimal regex has a stray `}`; R5's integer regex
   admits `+1` and refuses `-0`).

### D1 — what a number outside JSON's grammar reads as (the user's decision)

**Decided (2026-10-31): A.**

- **A (recommended): no value, as `null` is.** The primitive has no value;
  its element (`_valueDecimal`), when there is one, stays, with its id and
  extensions, as for `null` (`pairValueWithElement`); with none, a path to
  it is empty. `-.5` joins `1x` and `0e`, which this also mends: today they
  lose the element. The engine reads no number the text refuses, and
  validating the document stays the caller's or the validator's work.
- **B: the evaluation ends with an error**, as fhirpath.js's caller does
  (`JSON.parse`). Strongest, but alone: the engine raises nothing for any
  other malformed JSON, and lazy reading means only a path that touches the
  value would fail.
- **C: the text as a String typed as declared**, as #233 does for a date
  outside its form and as a decimal written as a JSON string (`"1.5"`) is
  read today. Keeps the text visible (`toString()`), but makes a String
  out of a token JSON does not admit at all.

### D2 — scope: a JSON number in a declared integer that is no integer

**Decided (2026-10-31): a separate issue, #251.**

`valueInteger: 1.5` (`1e2`, `1.0`) is valid JSON but no FHIR integer; today
it is a Decimal typed `integer`, neither `is Integer` nor `is Decimal`. The
validator ends the evaluation; fhirpath.js reads 1.5.
**Recommended: a separate issue**, with its own measurements; this increment
covers JSON's grammar only, for every type.

## Tests (first)

- `conformance/resource_number_grammar_test.go`, R4, R4B and R5 models and
  no model:
  - every form refused by RFC 8259 that the decoder accepts today (`-.5`,
    `5.`, `1.`, `007.50`, `1.e5`, `01`, `00`, `-01`, `-0.`, `.5e1` where
    jsonparser lets it through) in each placement: a declared decimal, a
    choice, a Quantity, a Money, an extension, a repeating element, a
    contained resource, a Bundle entry, `descendants()`; expected per D1;
  - every JSON number that is valid stays as it is today (`-0`, `0.0`,
    `1E5`, `1e+05`, `1e-5`, `-0.0`, a 1001-digit value): a metamorphic
    check against the current answers;
  - a property test of the grammar alone: random strings over
    `[-+.eE0-9x_ ]`, `isJSONNumber(s) == json.Valid(s)` (Go's
    `encoding/json`, an independent recognizer of RFC 8259's lexical
    grammar, not of what a value can hold); value construction tested
    apart, with the exponent's int32 bound (`1e2147483648`, `1e-2147483649`:
    valid JSON, refused by the decoder as recorded in CONFORMANCE.md);
  - the element kept: an invalid number beside `_valueDecimal` with an id
    and an extension, scalar and in an array (`_x: [null, {...}]`):
    `extension`, `children()`, `hasValue()`, `getValue()`, `ofType()`;
  - comparisons: two objects with an invalid number, identical, reformatted
    (`-.5` against `-0.5`), reordered keys, and against a valid number;
    `=`, `~`, `distinct()`, `union`;
  - the tokens jsonparser lets through (`1.e5`, `-`, `1e`, `1.5.5`, `1-2`,
    `0x1`, `1_000`) count; those it refuses (`.5`, `+1`) are recorded, not
    counted as catching anything;
  - `toDecimal()`, `convertsToDecimal()`, `types.NewDecimal` unchanged
    (rows from FHIRPath's string form).
- `difftest/batteries/resource-number-grammar`: the valid forms through
  `make three-engine`; the refused forms by a direct validator run and a
  `JSON.parse` check recorded in a script in the repo, since the harness
  cannot carry invalid JSON.
- corpusdiff against main: 0 differences expected (the examples are JSON).
  It shows that valid numbers are unchanged, not the refused ones, which the
  tests above cover.
- Downstream: go-searchindex validates its input as JSON first
  (`extractor.go`); go-validator can pass data already parsed. Both suites
  run against the change; `types.ReadDecimal` (exported, used by
  `convertsToDecimal()` on raw strings) is unchanged.

## Out of scope

- Validating the document (trailing commas, trailing text, unclosed
  arrays), and tokens holding a byte of JSON's structure (`1:5`, `1"5"`,
  `1{2`), which jsonparser itself reads inconsistently (round 2, JP-2).
- D2 (#251); `types.NewDecimal`'s and `ParseDecimal`'s accept set.

## Exit criterion

A resource's number outside RFC 8259's grammar reads as D1 decides, in
every placement, R4, R4B and R5, with and without a model; every valid
number reads as before (corpusdiff 0); the tests above pass and their
mutations are caught.

## Dependencies

None. `encoding/json` (standard library) only in tests, as the oracle.

## Adversarial round (2026-10-31, Codex)

Codex's sandbox refused writes, so its findings rest on code citations; each
was checked with a probe here.

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | The two comparisons of objects decode numbers and are not in the plan's list | high | **Accepted**: added to step 1, with tests (identical, reformatted, reordered, `distinct()`, `union`) |
| 2 | D1-A would drop the primitive's element (`_valueDecimal`) with the value | high | **Confirmed** by probe (`1x` loses the extension today, `null` keeps it); **accepted**: A reads as `null` does, keeping the element; tests added |
| 3 | The property test's "read iff JSON" is false: `1e2147483648` is JSON and not decodable | medium | **Accepted**: the grammar is tested alone against `encoding/json`, value construction apart |
| 4 | What jsonparser lets reach the engine was not measured | medium | **Accepted**, measured (table above): `.5` and `+1` never reach it; `1 2` is read as `1` (out of scope) |
| 5 | "With and without a model alike" overstates: typing differs | medium | **Accepted**: alike as to the grammar only (step 3) |
| 6 | Rule 5 needs an ADR | medium | **Partly accepted**: the exception is recorded in CONFORMANCE.md as 009 did; the repo keeps no ADR folder |
| 7 | corpusdiff 0 cannot show the refused forms; downstream unchecked | medium | **Accepted**: stated, downstream suites added; `ReadDecimal` unchanged |

## Validation (2026-10-31)

- Three engines: `make three-engine BATTERY=batteries/resource-number-grammar/battery.tsv DIFFTEST_ARGS="-v -jobs 2 -java ../build/validator/validator_cli-6.10.2.jar"`,
  every valid number in each placement, R4 and R5: all cases agree.
- The refused forms, which the harness cannot carry (it marshals JSON):
  `difftest/batteries/resource-number-grammar/refused/measure.sh` runs
  fhirpath.js's `JSON.parse` and the validator over each file; the results
  are in CONFORMANCE.md and JAVA-14.
- `make corpusdiff BASE=main`: no answer differs over the R4, R4B and R5
  official examples, as expected (none holds a refused form).
- Downstream, go-searchindex and go-validator against this tree.

## Mutations (2026-10-31)

One at a time, root and conformance suites (`mut/mutate015.py`), with
010–014's sets. The first run left four alive:

- H8 (type inference reads a bad number) and H10 (Money reads one): real
  gaps. `ReadDecimal` reads `-.5`, `5.`, `01`… so Money compared `-.5` as
  -0.5, and an object no model knows with `{"value":-.5,"unit":"mg"}` was
  inferred a Quantity. Tests added: a placement `Patient.x`, Money pairs.
- The bad-number-against-null case in the ordered comparison, and the
  grammar check in `OverflowingQuantity`: no observable effect. The first
  was a shortcut the full reading answers alike, so the ordered comparison
  now leaves any bad number undecided; the second repeated
  `quantityValue`'s check, and was removed. H11, H12 and H15 now mutate the
  remaining check.

| # | Mutation | Caught by |
|---|---|---|
| H1 | Leading zeros allowed | the grammar test, the metamorphic tests |
| H2 | A point without digits | the grammar test, the metamorphic tests |
| H3 | An exponent without digits | the grammar test, the metamorphic tests |
| H4 | Trailing text allowed | the grammar test, the metamorphic tests |
| H5 | The untyped path unchecked | `TestANonJSONNumberReadsAsNull` |
| H6 | The declared decimal unchecked | `TestANonJSONNumberReadsAsNull` |
| H7 | The element dropped with the value | `TestANonJSONNumberKeepsItsElement` |
| H8 | Type inference unchecked | `TestANonJSONNumberReadsAsNull` (`Patient.x`) |
| H9 | A Quantity's value unchecked | `TestANonJSONNumberReadsAsNull` |
| H10 | A Money's value unchecked | `TestObjectsWithANonJSONNumberCompareAsWithNull` |
| H11 | The ordered comparison ignores bad numbers | `TestObjectsWithANonJSONNumberCompareAsWithNull` |
| H12 | Only the left side checked | `TestObjectsWithANonJSONNumberCompareAsWithNull` |
| H13 | Equivalence reads bad numbers | `TestObjectsWithANonJSONNumberCompareAsWithNull` |
| H14 | An object written as held | `TestANonJSONNumberReadsAsNull` |
| H15 | Only the right side checked | `TestObjectsWithANonJSONNumberCompareAsWithNull` |
| H16 | A plus sign allowed | the grammar test |

## Code review

### Round 1 (2026-11-01, Codex)

The first launch could not write to the scratchpad (sandbox), so the copy
went to `build/review015`, inside the repo and ignored by git. Each finding
was reproduced here with a probe against the null relation, R4, R4B, R5 and
without a model.

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | `holdsNoValueAt` reads a bad number as a value: `{"valueQuantity":{"value":01}}` makes `Observation.value.exists()` true | medium | **Confirmed**; fixed, with tests |
| 2 | The ordered comparison checks the token only up to `numberEnd`, so `0x1` is checked as `0`: `[0x1,1]` against `[1]` is unequal | medium | **Confirmed**; the token now ends where jsonparser ends it; tests |
| 3 | An array inside an array is compared as written: `[[-.5]]` against `[[null]]` is unequal | medium | **Confirmed**; its JSON is compared with bad numbers written as null, valid numbers compared as on main; tests |
| 4 | Four mutations of the array readings survive | medium | **Confirmed**; three were killed by the tests for 1–3, the fourth was masked by finding 1 |
| 5 | `String()` of an object rescans it: 141–145 ns against 17–20 ns | low | **Accepted in part**: writing a bad number as null is D1, and `String()` renders output only (no evaluation path calls it). The scan was made faster |

A pre-pass that writes the whole document once was considered and
rejected: the scan runs at about 1.7 GB/s, 20 ms per evaluation of the
35 MB `Bundle-resources.json`, where a lazy read of `Bundle.type` costs
microseconds.

The fixes (2026-11-01): a token ends where jsonparser ends it (`tokenEnd`,
shared by `badNumberAt`, `holdsNoValueAt` and `nullBadNumbers`);
`holdsNoValueAt` reads a bad number as null; arrays compared as written are
compared again with bad numbers written as null (`sameArray`), only when
there is one, so valid numbers compare as on main. Two more were needed for
the null relation to hold: the ordered comparison takes a bad number and a
null as alike, so that `[[-.5],[1]]` and `[[null],[1.0]]` take one path,
and its backtrack to a token's start walks over any byte a bad token holds
(`1_5`). The scan is faster (10 KB example: 6.1 µs to 4.0 µs).

One mutation was judged equivalent: in `addsNothing`, reading an element's
item as written. An element whose items hold no value yields the same
items to `equivalenceItems` whether it is dropped or kept.

Mutations H17–H25 cover the fixes; H11, H12 and H15 were rewritten for
`compareBadNumbers`.

| # | Mutation |
|---|---|
| H17 | A bad number holds a value (`holdsNoValueAt`) |
| H18 | A token ends at the number bytes (`numberEnd`) |
| H19 | Nested arrays compared as written |
| H20 | `writtenAlike` skips the rewrite |
| H21 | `equalValues` skips the rewrite |
| H22 | A bad number unlike null in the fast path |
| H23 | The backtrack stops inside a token |
| H24 | An escaped quote closes a string |
| H25 | A rewrite of null's length taken for none |

Before round 2: all mutation sets 010–015 caught (H1–H25) but H23, which
no test reached: a difference past `_` with a valid number after it on
both sides. `1_5` was added to the refused tokens and `1_6` to the
right-hand sides; H23 is caught. corpusdiff against main: 0 differences;
bench-compare: no change (geomean +0.06 %).

### Round 2 (2026-11-01, Codex)

Given the round-1 findings, the changes and the round-1 diff. It found
valid documents unchanged (36,864 results matching main byte for byte) and
the scanner matching jsonparser's token ends for every byte.

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | Reading from the difference starts inside `1:5` or `1"5"`: `{"a":1:5}` against `{"a":1:6}` is unequal | medium | **Rejected, out of scope**: reproduced, and a guard on what precedes a token's start fixed `=`; but in the same document jsonparser's `Get` loses the keys after `1:5` that `ObjectEach` reads (`{"value":1:5,"unit":"mg"}`: `Get("unit")` "Key path not found"), so `Patient.x + 1` errs where null gives empty. Holding the null relation there means working around jsonparser; such a token breaks the document's structure, as `1 2` does. The guard was withdrawn; JP-2 logged |
| 2 | `1"2` and `1{2` lose the whole resource | medium | **Rejected, out of scope**: `jsonparser.Get` fails on such a root ("can't find closing '}' symbol"), which reads as empty as on main (measured on a main worktree); the same class as `1 2`. JP-2 |
| 3 | Adding `:` to the token's delimiters survives | medium | **Rejected**: it changes only documents of finding 1's class |
| 4 | Two mutations survive (the identity check in `sameArray`; `addsNothing`'s element items) | — | **Equivalent**: the first saves a second comparison only; the second as in round 1 |

Converged: no confirmed finding of medium or higher severity remains. The
tree is the one round 2 reviewed (checked file by file), on which the full
mutation sets ran.
