# 017 — ofType() of a FHIR primitive by the System type it converts to

Status: closed (step 9), 2026-11-03. The review hit the 3-round cap; the user chose to correct the two doc findings of round 3 and close.
round of what is now plan 018 (finding F1). 018, as() before R5, reuses this
increment's rule.

## 1. Prior review

### The text

The FHIR FHIRPath page, "Using FHIR types in expressions", gives the
conversion:

> The evaluation engine will automatically convert the value of FHIR types
> representing primitives to FHIRPath types when they are used in expressions
> according to the following mapping:

| FHIRPath type | R4 (4.0.1, §2.9.1.2) and R4B (4.3.0, §2.9.1.2) | R5 (5.0.0, §2.1.9.1.2) |
|---|---|---|
| System.Boolean | boolean | boolean |
| System.String | string, uri, code, oid, id, uuid, sid, markdown, base64Binary | string, uri, code, oid, id, uuid, markdown, base64Binary |
| System.Integer | integer, unsignedInt, positiveInt | integer, unsignedInt, positiveInt |
| System.Long | — | integer64 |
| System.Decimal | decimal | decimal |
| System.DateTime | date, dateTime, instant | date, dateTime, instant |
| System.Time | time | time |

R4 and R5 are pinned (`conformance/testdata/spec-cases/.spec/fhir-r4.txt`,
`fhir-r5.txt`). R4B was fetched on 2026-11-02 and is identical to R4. The
Quantity row ("see below") is a complex type, out of scope (FP-16).

On ofType(), R5 says, after the exception for is(): "Note that ofType() does
not have such restrictions - both of the following are valid:
`Patient.name.given.ofType(FHIR.string);`
`Patient.name.given.ofType(System.string);`". R4 and R4B say the same of
as() (plan 018), and nothing of ofType(). FHIRPath defines ofType() as
returning "all items in the input collection that are of the given type or a
subclass thereof".

The packages type each primitive's `value` differently on three rows. date
is `System.Date`. unsignedInt and positiveInt are `System.String` (SPEC-8).
R5's integer64 is `System.Integer`. canonical and url, which the table does
not name, are `System.String`.

**The user chose the table as the source (2026-11-02).** fhirpath.js
implements it as written, on every row measured below. It is a rule-5
exception, and it is recorded in CONFORMANCE.md with a test that reads the
pinned texts.

### Measured (battery, 363 cases, R4 and R5 models, 2026-11-02)

The answers are identical in R4 and R5.

| `ofType(...)` | This engine | fhirpath.js 5.2.0 | HL7 validator 6.10.2 |
|---|---|---|---|
| `gender` (code), `implicitRules` (uri), `language` (code) `.ofType(System.String)` | `[]` | the value | `[]` |
| `meta.profile` (canonical) `.ofType(System.String)` | `[]` | `[]` | `[]` |
| `name.given` (string) `.ofType(System.String)` | the value | the value | `[]` |
| `telecom.rank` (positiveInt) `.ofType(System.Integer)` | `[]` | the value | `[]` |
| `birthDate` (date) `.ofType(System.DateTime)` | `[]` | the value | `[]` |
| `birthDate` (date) `.ofType(System.Date)` | the value | `[]` | error |
| `meta.lastUpdated`, `Observation.issued` (instant) `.ofType(System.DateTime)` | `[]` | the value | `[]` |
| `active` (boolean) `.ofType(System.Boolean)` | the value | the value | `[]` |
| Unqualified `ofType(String)`, `ofType(Integer)`, `ofType(DateTime)` | as qualified, by name | as qualified | error |
| `given.ofType(System.string)` (the R5 example) | the value; `[]` with no model | error | error |

- **The validator** keeps no FHIR primitive by a System type at all, which is
  against R5's text. It errs on unqualified System names. It is not followed
  here, and is cataloged.
- **This engine**, with a model, matches by the FHIR type's name
  (`castMatches` → `primitiveTypeMatches`, `eval/evaluator.go`). So only
  string, boolean, date and decimal are kept, and only because their names
  are spelled like a System name. Without a model, `System.string` in lower
  case is not taken for System.String (`matchesSystemType` asks
  `IsSystemTypeName("string")`).

### gofhir/server

It uses this library and has no rule of its own. Nothing to take.

## 2. Plan

### Decisions

- **D1 (the rule):** `ofType(T)`, where T names a System type, keeps a FHIR
  primitive exactly when the mapping table of the version maps the
  primitive's FHIR type to T. T is a System type when it is written
  `System.X` in any case (`System.string`, as the R5 example writes it), or
  unqualified and capitalized as FHIRPath names its types. The item kept is
  the FHIR primitive itself, with its FHIR type and its extensions.
- **D2 (which table):** before R5 the R4 table applies, and from R5 on the R5
  one, by `enforcesR5Rules()` (`eval/version.go`). That covers a model
  declaring 4.x, a model declaring no version, and no model at all, as the
  precedent has it.
- **D3 (rule 5):** the two tables are written in Go, one map per version,
  keyed by FHIR type name. A conformance test parses the table out of each
  pinned text (`fhir-r4.txt`, `fhir-r5.txt`) and fails on any difference.
  R4B is pinned too: `conformance/testdata/spec-cases/SPEC_SOURCES` gains
  `fhir-r4b` with its sha256. The test skips only when the texts are absent
  and `SPEC_SOURCES_REQUIRED` is unset. With it set, a missing text, anchor
  ("according to the following mapping:") or header fails it, and CI's
  pinned-source step runs it (`-run 'TestSpecCaseCatalog|TestConversionTable'`).
  The Quantity row ("System.Quantity (see below)") is excluded by name.
  sid reaches no model, so the table test alone covers it. CONFORMANCE.md
  records the exception (F1, F8, F9).
- **D4 (rows the table settles):** R5's integer64 maps to System.Long.
  `ofType(System.Long)` keeps it from R5 on, because the item kept is the
  FHIR primitive and needs no Long value. `ofType(System.Integer)` drops it.
  `System.Long` is already accepted as a specifier. The unqualified `Long`,
  which is an error with a model today, is out of scope (F4). canonical,
  url and xhtml, which the table does not name, convert to no System type.
- **D4b (items the model types as a System type):** `Element.id`, R4's
  `Resource.id`, and any element whose model type is a System type name
  (`String`) keep today's System-name match. So `name.id.ofType(System.String)`
  and R4 `Patient.id.ofType(System.String)` keep the value, and R5's
  `Patient.id` (type `id`) goes by the table (F3).
- **D4c (no value):** a FHIR primitive with only extensions (`_birthDate`
  alone) has no value to convert, so `ofType(System.X)` drops it.
  `ofType(date)` and `ofType(System.Any)` keep it, as today (F6).
- **D4d (spelling):** the namespace is matched exactly, `System`, as
  `checkTypeSpecifier` does. The name ignores case, so the R5 example's
  `System.string` is System.String. `system.String` and `SYSTEM.String` name
  no System type. A single helper serves `ofType`, and the three that differ
  today (`fhirPrimitiveAsSystemType`, `systemTypeMatches`,
  `checkTypeSpecifier`) are brought to it (F5).
- **D5 (unchanged):**
  - `is()` and `as()` (018 changes `as()` before R5);
  - `ofType` with a FHIR type name (`ofType(string)`, `ofType(FHIR.code)`),
    which still requires the declared type;
  - System values, such as literals, which match only System types;
  - complex types, including Quantity.
- **D6 (no model):** a plain element read from the resource is a FHIR
  primitive even then (`types/primitive_element.go`), but with no model its
  type is the System type its JSON gives it (`String`, `Date`). So D4b
  answers it by its System name: `birthDate.ofType(System.Date)` keeps it,
  `meta.profile.ofType(System.String)` keeps it, and `ofType(System.DateTime)`
  drops the date. A choice carries its FHIR type from its key
  (`valueDate` → date), so the table answers it: `valueDate.ofType(System.DateTime)`
  keeps it, and `valueUrl.ofType(System.String)` drops it. The two differ,
  because only the choice tells its FHIR type. That is the no-model guess
  CONFORMANCE.md already describes, and the section gains these rows (F2).
  `fhirPathSpecMap` stays as it is: it answers only `is()` and the old
  matcher with no model, and the new tables answer `ofType` alone.
- **D7 (one rule):** `funcs.fnOfType` (`funcs/filtering.go`) is reachable only
  through `funcs.Get("ofType")`, since expressions take the evaluator's path
  (`eval/plan.go`). It calls the same matcher, so the public registry answers
  alike (F7).

### Out of scope

- as() before R5 (018).
- FP-16 (`as` and `is` with System.Quantity).
- FP-17 is resolved by D1, since date maps to DateTime: `birthDate.ofType(Date)`
  becomes `[]`, as in fhirpath.js. It is closed here with that row.

### Exit criterion

1. For every row of every table, in R4, R4B and R5, and for a choice with
   no model: `ofType(System.X)` keeps a primitive of each FHIR type the table
   maps to X, and drops one it maps elsewhere. Items typed by a System name,
   and plain elements with no model, are answered as D4b and D6 say.
2. Every answer listed under D5 is unchanged. Tests on main keep passing,
   except those that held the old answers, each changed with its section
   cited.
3. The test comparing each Go table with its pinned text passes, and fails
   when a row is changed.
4. CONFORMANCE.md has a section with the table, the measurements and the
   rule-5 exception, and catalogs the validator's answers. FP-17 is closed,
   and JAVA-17 is recorded (the validator keeps no primitive by its System
   type).
5. Mutations caught, `make check` and corpusdiff green.

### Tests (step 5)

- **`conformance/oftype_system_type_test.go`**, over R4, R4B, R5 and no model.
  The expected values come from the table row, not from main.
  - A plain element and a choice for every FHIR type the tables name, and
    for canonical, url and xhtml. Each is asked by every System type in
    three forms: `System.X`, unqualified `X`, and `System.x` in lower case.
  - The examples from the text, verbatim: `given.ofType(FHIR.string)`,
    `given.ofType(System.string)`.
  - A collection mixing types (`Patient.descendants().ofType(System.String)`)
    keeps exactly the items whose type maps there.
  - The item kept keeps its type and extensions (`type().name`,
    `.extension`).
  - The D5 answers are held unchanged.
- **Table test** `conformance/conversion_table_test.go`: parses the mapping
  table out of `fhir-r4.txt`, `fhir-r4b.txt` and `fhir-r5.txt` and compares it
  with `eval`'s maps.
- **Metamorphic:** before and from R5, for a singleton FHIR primitive X,
  `X.ofType(S)` keeps X exactly when `X.ofType(S).count() = 1`. Reordering
  the keys of the resource changes nothing.
- **Battery:** the 363 cases are extended with oid, uuid, markdown,
  base64Binary, unsignedInt, time, integer64 (R5), `Element.id`, `_element`
  alone, and the spelling variants, and run on the three engines. The
  harness takes R4 and R5, so R4B rests on the conformance tests (F9). Every
  difference is classified with the reference's answer.
- **corpusdiff** over the official examples, against main. Each difference
  is explained by a table row.

### Mutations (step 7)

- a table row removed;
- R4's table used for R5 (sid kept, integer64 lost), and the reverse;
- the version test inverted;
- the System-type test case-sensitive again (`System.string` lost);
- the old name match kept alongside the table;
- `is()` routed through the table;
- the System value returned instead of the item;
- the table test made to skip a version.

### Dependencies

None. `fhir-r4b.txt` is a new pinned source fetched by the existing
`scripts/spec-sources.py`.

## 3. Adversarial round (Opus, 2026-11-02; Codex at its limit)

| # | Severity | Finding | Resolution |
|---|---|---|---|
| F1 | high | The table test would skip silently: `.spec/` is not vendored, and CI requires the texts only for `TestSpecCaseCatalog` (`ci.yml:131-134`). The pins live in `SPEC_SOURCES`, not in the script | **Accepted**, confirmed. D3: pin in `SPEC_SOURCES`, fail under `SPEC_SOURCES_REQUIRED`, run in CI's step |
| F2 | high | No model: plain elements and choices answer differently (`birthDate.ofType(System.Date)` keeps it; `valueDate`'s answer depends on the table). Exit 1 cannot hold for plain elements; `fhirPathSpecMap` conflicts | **Accepted**, confirmed by a probe. D6 rewritten, exit 1 narrowed, `fhirPathSpecMap` left to `is()` |
| F3 | high | Items the model types as a System name (`Element.id`, R4 `Resource.id`) have no table key; a literal implementation drops what is kept today | **Accepted**, confirmed (`name.id`, R4 `Patient.id` keep `p1`; R5 `[]`). D4b |
| F4 | medium | D4 dropped integer64, though keeping the item needs no Long; so the version mutations were equivalent (sid reaches no model) | **Accepted**: D4 keeps integer64 under System.Long from R5; sid is covered by the table test. Unqualified `Long` stays out of scope |
| F5 | medium | The spelling rule is ambiguous; three helpers disagree (`System.STRING`, `system.String`) | **Accepted**: D4d, a single helper |
| F6 | medium | A primitive with only extensions has no rule | **Accepted**, confirmed: D4c |
| F7 | low | `funcs.fnOfType` matches by exact type name | **Accepted**, confirmed reachable only through `funcs.Get`: D7 |
| F8 | low | The Quantity row would be parsed | **Accepted**: excluded by name (D3) |
| F9 | low | No R4B in the battery; sid circular | **Accepted**: R4B by the conformance tests, sid by the table test |

## 4. Implementation (steps 5 and 6, 2026-11-02)

Written by a Sonnet subagent from the approved plan; verified here.

- **Tests:** `conformance/oftype_system_type_test.go` (9 groups) and
  `conformance/conversion_table_test.go` (3). Nine failed before the change;
  the valueless (D4c), unchanged (D5) and metamorphic groups are guards the old
  code already met.
- **Existing tests:** `typing_test.go` `TestOfTypePolymorphic`:
  `ofType(Date)` on a valueDate went from 1 to 0, citing the table (date →
  DateTime), and `ofType(DateTime)` on it was added.
- **Pin and CI:** `fhir-r4b` in `SPEC_SOURCES`. CI's pinned-source step runs
  `TestSpecCaseCatalog|TestConversionTable`.
- **Visible change beyond the table:** a namespace not spelled exactly
  `System` (`system.X`, `SYSTEM.X`) names no System type (D4d), so it matches
  nothing. With no model `is` is false and `as`/`ofType` are empty. With a model
  the bare name is looked up in the model like an unqualified name: an error
  ("does not resolve to a type in the model") where the model has no such type
  (`String`, `Date`), and no error but no match where it has one (`Quantity`).
  Rows, main (no model / R4 / R5) to now (`Patient.id` is System.String in R4,
  `id` in R5):
  - `Patient.id.is(system.String)`: true / true / false to false / error / error;
  - `Patient.id.ofType(system.String)` and `.as(...)`: `[String p1]` /
    `[String p1]` / `[]` to `[]` / error / error;
  - `Patient.gender.is(system.String)`: true / false / false to false / error / error;
  - `Patient.birthDate.is(SYSTEM.Date)`: true / false / false to false / error / error;
  - `'a'.is(system.String)`: false everywhere to false / error / error;
  - `(1 'mg').is(SYSTEM.Quantity)`: true everywhere to false everywhere;
  - `Observation.value.is(SYSTEM.Quantity)` on a valueQuantity: true everywhere
    to false everywhere;
  - `Observation.value.ofType(SYSTEM.Quantity)`: `[Quantity]` everywhere to `[]`.

## 5. Validation (step 7)

### Battery `difftest/batteries/oftype-system-type` (620 cases, R4 and R5)

Run on 2026-11-03 with fhirpath.js 5.2.0 and the HL7 validator 6.10.2. There
are no rows where fhirpath.js alone differs: on every row of the table this
engine answers as fhirpath.js does.

| Class | Cases | Explanation |
|---|---|---|
| The three agree | 307 | — |
| The validator alone | 283 | It keeps no FHIR primitive by its System type, and errs on unqualified System names (JAVA-17). Against R5 §2.1.9.1.2; measured, and not followed |
| This engine alone | 24 + 2 | `ofType(System.string)` (R5's own example) and `System.STRING`: both references err on the name; D4d reads the name in any case |
| This engine alone | 2 | `ofType(System.Any)` on a primitive with only extensions keeps it; both references err. Unchanged from main |
| The three differ | 2 | `birthDate.ofType(date)` with only extensions: kept here, `[]` in fhirpath.js, `[null]` in the validator. Unchanged from main (D4c) |

### Mutations

`mutate017.py`, 13 mutants:
- On the first run, 12 were caught. O11 (`funcs.fnOfType` by exact name)
  survived, because no test reached the registry's ofType with a FHIR
  primitive.
- A test was added: `funcs/funcs_test.go`, "ofType by the System type a FHIR
  primitive converts to", with its values taken from the table. It catches
  O11.
- O13 (the table test skipping R4B) was dropped: it mutates the oracle
  itself, and no test can catch it.
- **Full run, 010–017:** 170 mutants, all caught, in 582 s (mutfast.py).

### corpusdiff

Run against main over the official examples, 2026-11-03. Differences:
- R4: 0 of 521,007 evaluations;
- R4B: 0 of 308,577;
- R5: 0 of 360,675.

No corpus expression asks ofType() by a System type of a primitive, so this
is evidence that nothing else moved, not evidence for the change.

## 6. Code review

### Round 1 (Opus, 2026-11-03)

| # | Severity | Finding | Resolution |
|---|---|---|---|
| R1 | medium | `fhirPrimitiveAsSystemType` was narrowed. So `gender.as(System.code)` and `gender as System.code` keep the code with a model (main gave `[]`), while `is(System.code)` is false; R5 `Patient.id.as(System.id)` likewise. This breaks D5 | **Accepted**, confirmed by a probe. Any exact `System.` specifier takes the is()-style path again, and rows are added |
| R2 | low | `gender.ofType(System.code)` keeps the code with a model and drops it without one. `System.code` names no System type | **Accepted**: dropped in every version, with rows |
| R3 | low | The exact `System.` prefix in `TypeMatchesWithModel` is untested (a mutation survives) | **Accepted**: a unit test with a stub model |
| R4 | low | CONFORMANCE.md says is() and as() are unchanged, though `system.String` is now an error with a model. §4 wrongly said no-model `'a'.is(system.String)` went from true to false | **Accepted**: both corrected |
| R5 | low | The resume point was stale; FP-17's no-model plain case was not stated; `extension.valueDate` paths are not navigable | **Accepted** |
| R6 | low | The bare bool of `PrimitiveConversion`; `systemTypeLong` and `systemTypeAny` duplicate `types.TypeName*` | **Accepted**: documented, and moved to `types` |
| — | gap | `TestOfTypeSystemTypeInteger64` covered "before R5" only with no model | **Accepted**: R4 and R4B added |

### Round 2 (Opus, 2026-11-03)

The round-1 fixes hold. The reviewer removed each fix in turn and the new
tests failed. A probe of 18,480 answers against main (22 specifiers × 14
paths × is/as/ofType and the operators × no model, R4, R4B, R5) moved only
the intended ofType rows.

| # | Severity | Finding | Resolution |
|---|---|---|---|
| F1 | medium | §4 and CONFORMANCE.md said that, with no model, `system.String` and `SYSTEM.String` matched nothing on main too. That is false for primitives read from a resource. On main, `gender.is(system.String)` was true and `gender.as(SYSTEM.String)` `[male]`; now they are false and `[]`. No test pinned the is() and as() rows | **Accepted**: the behaviour is D4d's, while the docs and tests are corrected. Mutations ran before the round: 172, all caught |

### Round 3 (Opus, 2026-11-03): did not converge

Mutations before the round: 172, all caught. The round-2 tests hold: run
against a copy of main, every new row fails. Two medium findings remain, both
about how the docs describe main's answers to a misspelled namespace. The
behaviour itself is D4d's, and the 18,480-answer probe of round 2 checked it.

| # | Severity | Finding |
|---|---|---|
| F2 | medium | §4 and CONFORMANCE.md say that with a model main answered `is` false and `ofType` `[]`. Not in R4, whose model types `Resource.id` System.String: main gave `Patient.id.is(system.String)` true and `ofType`/`as` `[p1]`; now an error |
| F3 | medium | The change is described for `system.String` and `SYSTEM.String`, but it holds for any misspelled namespace. The claim that literals were already unmatched is false: main gave `(1 'mg').is(SYSTEM.Quantity)` true and no-model `birthDate.is(SYSTEM.Date)` true; now false |

The 3-round cap is reached, so the user is consulted.

### After round 3 (2026-11-03)

The user chose to correct and close without a fourth round. F2 and F3 are
corrected: §4 and CONFORMANCE.md now state the general rule (any namespace not
spelled `System`) and list the rows, main to now, without the false claims.
Rows added to `TestOfTypeSystemTypeSpelling`, in no model, R4, R4B and R5:
`(1 'mg').is(SYSTEM.Quantity)` false; `Observation.value.is(SYSTEM.Quantity)`
false; `Observation.value.ofType(SYSTEM.Quantity)` `[]`;
`Patient.birthDate.is(SYSTEM.Date)` false with no model, an error with a model.
The R4 `Patient.id` is/ofType/as(system.String) error rows were already pinned.

## 7. Closing (2026-11-03)

- `make check` is green.
- The battery `oftype-system-type` (620 cases) has no row where fhirpath.js
  alone differs; the rest is classified in §5.
- Mutations 010–017: 172, all caught.
- corpusdiff: 0 differences.
- SPEC-9 is rewritten: R4 is not at fault; the FHIR page moved the exception
  between versions.
- Next: plan 018, as() before R5, which reuses `systemTypeSpecifier` and the
  tables.
