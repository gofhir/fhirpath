# 018 — as() of a FHIR primitive by its System type, by version (#241)

Status: closed with no code change (2026-11-03). Measured, both references
answer as() as is() in R4 and R5; the user chose to follow them (reading 3,
"valid" means no error), see §4.

## 1. Prior review

### The text

The FHIR FHIRPath page, right after the exception for `is()`:

- **R4** (4.0.1, fhirpath.html §2.9.1.2; pinned in
  `conformance/testdata/spec-cases/.spec/fhir-r4.txt`) and **R4B** (4.3.0,
  §2.9.1.2, fetched 2026-11-02):
  > The primary exception is the is() operation, where the difference is
  > explicit: `Patient.name.given.is(FHIR.string);`
  > `Patient.name.given.is(System.string).not();` … Note that **as()** does
  > not have such restrictions - both of the following are valid:
  > `Patient.name.given.as(FHIR.string);` `Patient.name.given.as(System.string);`
- **R5** (5.0.0, §2.1.9.1.2): the same sentence names **ofType()**, with
  `ofType(FHIR.string)` and `ofType(System.string)` as the examples.

R5 changed the rule, and rewrote `Observation-code-value-date` along with it:
`value.as(DateTime) | value.as(Period)` in R4 and R4B,
`value.ofType(dateTime) | value.ofType(Period)` in R5. Neither official suite
(R4, R5) tests `as()` with a System type. So the text of each version decides.

The mapping table both versions give says which System type each primitive
converts to: boolean → Boolean; string, uri, code, oid, id, uuid, (sid),
markdown, base64Binary → String; integer, unsignedInt, positiveInt → Integer;
(R5: integer64 → Long); decimal → Decimal; date, dateTime, instant →
DateTime; time → Time. The packages give the type of each primitive's
`value`: they agree with the table except that `unsignedInt` and
`positiveInt` are `System.String` (SPEC-8), `date` is `System.Date`, and R5's
`integer64` is `System.Integer`.

### This engine (main, 62dfbe6)

- `as()` with a System type on a FHIR primitive goes through
  `castCollection` → `fhirPrimitiveAsSystemType` → `ValueTypeMatches`. That
  is the `is()` answer, so it is empty in every version. The comment there
  records the reason: "as fhirpath.js and the HL7 validator both answer".
- The `as` operator takes the same path (`eval/evaluator.go`, the `as` case).
- Measured, R4, R4B and R5 alike: `Observation.value.as(DateTime)`,
  `.as(System.DateTime)`, `value as DateTime`, `Patient.gender.as(String)`,
  `birthDate.as(Date)` and `telecom.rank.as(Integer)` all give `[]`.
- Without a model: a plain element's value is a System value, so
  `gender.as(String)` is `[male]`, `birthDate.as(Date)` `[1990-01-01]` and
  `rank.as(Integer)` `[3]`. A choice carries its FHIR type from the key, so
  `value.as(DateTime)` is `[]`.
- The precedent for a version split is `Context.EnforcesR5Rules`
  (`eval/version.go`): `as()`'s singleton rule applies from R5 on, by the
  version a model declares through `VersionedModel`; a model that declares
  none, or no model, is taken for pre-R5. HL7's Java engine splits the same
  rule the same way (CONFORMANCE.md, `as()` singleton section). The three
  gofhir/models (`FHIRVersion()` "4.0.1", "4.3.0", "5.0.0") declare their
  version.

### The references

Battery `difftest/batteries/primitive-type-operators` (272 cases, R4 and R5,
run 2026-11-02): with either model, fhirpath.js 5.2.0 gives `[]` for
`as(DateTime)` and `as(System.DateTime)` on a dateTime. The HL7 validator
6.10.2 gives `[]` for `as(System.DateTime)` and an error for the unqualified
`as(DateTime)`. Neither splits by version. The validator also contradicts
itself (`value is DateTime` false, `value.is(DateTime)` true). The
three-engine rule follows both references when they agree **coherently**.
Here one of them is incoherent, and the text of the version is explicit, with
an example of the same shape. So the text is followed and both references
are cataloged under "Measured, and not followed". The user approved this on
2026-11-02.

### The references, measured again for the revised plan (2026-11-03)

Battery `difftest/batteries/as-system-type` (44 cases, R4 and R5 models,
fhirpath.js 5.2.0 and the HL7 validator 6.10.2): 22 agree, 20 the validator
alone, 2 fhirpath.js alone. On `as()` with a System type:

- Both references give `[]` in R4 and R5 for `gender.as(System.String)`,
  `birthDate.as(System.DateTime)`, `telecom.rank.as(System.Integer)`,
  `given.first().as(System.String)`, `Observation.value.as(System.DateTime)`
  and `issued.as(System.DateTime)`: as() answers as is(), in both versions.
  Neither splits by version.
- The validator raises an error for the unqualified `as(String)`,
  `as(Boolean)`, `as(Date)`, `as(DateTime)` and, in R4 only, keeps the value
  for the operator `Observation.value as DateTime`: not coherent.
- fhirpath.js keeps R5 `Patient.id.as(System.String)` (`[p1]`), alone.
- ofType() with a System type: this engine and fhirpath.js keep, the
  validator gives `[]` (JAVA-17).

### gofhir/server

It uses this library, with no rule of its own for `as()`. Nothing to take.

## 2. Plan (revised on top of 017, merged as #258)

017 settled the hard part: which System type a FHIR primitive is, for a type
test that lets it be one. It is the conversion table of the version
(`primitiveConversion`, held to the pinned texts by `TestConversionTable`),
with the namespace spelled exactly `System` (D4d), valueless items dropped
(D4c), items typed by a System name kept by that name (D4b), and with no model
plain elements by their System value, choices by the table (D6). The first
draft's D2 (the System type of the value read) is replaced by it: F2 and F3.

### Decisions

- **D1 (rule):** before R5, `as(T)`, `as T` and their qualified forms, where
  T names a System type, keep exactly the items `ofType(T)` keeps: R4 and R4B
  say "as() does not have such restrictions" in the place R5 says it of
  ofType(). From R5 on, `as()` is unchanged: the is()-style answer.
- **D2 (version):** `enforcesR5Rules()`, as `as()`'s singleton rule and 017's
  table. No model, or a model that declares no version, is pre-R5.
- **D3 (one matcher):** the pre-R5 branch of `castCollection` calls 017's
  `ofTypeMatches` with the R4 table, so the two cannot drift. Names that are
  no System type (`System.code`, FHIR names, complex types) keep their present
  path in every version.
- **D4 (what is returned):** the item itself, with its FHIR type and its
  extensions, as ofType() returns it (017 D5).
- **D5 (singleton):** unchanged. Before R5 `as()` filters a collection, which
  is what makes D1's "keeps exactly the items ofType keeps" hold for a
  collection too; from R5 on, more than one item is an error before any type
  test.
- **D6 (no model):** pre-R5 by D2. A plain element already answers by its
  System value (`gender.as(String)` is `[male]` on main); a choice now answers
  by the table, so `Observation.value.as(DateTime)` on a valueDateTime keeps
  it where main gave `[]`. R5 data read with no model gets the R4 answer;
  CONFORMANCE.md and the CHANGELOG say so (first-draft F6).
- **D7 (funcs):** the evaluator never reaches `funcs.fnAs` (`plan.go` takes
  the type name from the tree and `evalLazy` intercepts `as`), but library
  users can call it through `funcs.Get("as").Fn`, and it matches `Type()`
  exactly with no version and no singleton rule. It goes through an exported,
  nil-safe `Context` matcher, as `fnOfType` does since 017, and is tested.

### Visible change

The R4/R4B split of `Resource.id` (R4 types it System.String, R4B and R5 FHIR
`id`), measured on main: `Patient.id.as(String)` and `as(System.String)` are
`[p1]` in R4 already and `[]` in R4B. After D1 R4 is unchanged for those two
(only `as(System.string)` changes) and R4B gives `[p1]` for all three
spellings.


Before R5 only (R4, R4B, no model), the rows where `ofType()` keeps an item
and `as()` gives `[]` on main, measured by the battery: for example R4
`Patient.gender.as(System.String)` and `gender as String`, `birthDate.as(DateTime)`,
`telecom.rank.as(Integer)`, `Observation.issued.as(System.DateTime)`, and the
spec's `Patient.name.given.as(System.String)`. `given.as(System.string)` (lower
case) is kept too, by 017's D4d, which both references refuse.

### Readings of the text (A2)

CONFORMANCE.md lists them:
1. **Followed:** R4/R4B let as() keep a FHIR primitive by its System type;
   R5 moved the permission to ofType(), revoking it for as(). Support: R5
   rewrote `Observation-code-value-date` from `value.as(DateTime)` to
   `value.ofType(dateTime)`, on a 0..1 element, so the change is not only about
   the singleton rule.
2. R5 moved the sentence only because its example, `given` (0..*), would
   break as()'s new singleton rule, so a singleton keeps the permission in R5.
   Against it: the invariant rewrite above.
3. "valid" means only "no error": as() still answers as is() (first-draft F5).

### Out of scope

- FP-16 (`as`/`is System.Quantity`).
- `is()` in every version, and `ofType()`.
- The validator's `is` operator against its `.is()` function.

### Exceptions to rule 5

None new: the table is 017's, already recorded and tested against the texts.

### Exit criterion

1. With an R4 or R4B model, and with no model, `as(T)` and `as T` for a System
   type T answer exactly as `ofType(T)` on the same input, singleton or not.
2. With an R5 model every `as()` answer is unchanged; `is()` and `ofType()`
   are unchanged in every version. The tests that hold the old pre-R5 answer,
   measured by a prototype (A1), are changed with the section cited:
   `conformance/oftype_system_type_test.go:528` (R4, R4B
   `gender.as(System.String)`) and `conformance/system_typed_test.go:81`
   (`name.family.as(System.String)`). The comments of `typing_test.go:29-44`,
   `TestAsAnswersAsIsWithoutAModel` and `castCollection`, which state the old
   rule though their rows do not flip, are rewritten.
3. CONFORMANCE.md: the section on as() by version, with the three texts and
   the references cataloged; the CHANGELOG notes the no-model change.
4. Mutations caught, `make check` green, corpusdiff classified.

### Tests (step 5)

- `conformance/as_primitive_by_version_test.go`, R4, R4B, R5 and no model,
  expected values from the table and the text, not from the engine:
  each primitive kind as a plain element and as a choice; the four forms
  (`as(X)`, `as(System.X)`, `as X`, `as System.X`); a wrong System type; the
  spec's own `given.as(FHIR.string)` and `given.as(System.string)`; a
  collection before R5; D4 (`type().name`, `.extension`); a valueless item.
- **Independent rows (A6),** since the as = ofType and as ⇔ is identities
  hold by construction once D3 shares the matcher (they stay, as a guard
  against the branches drifting, and prove nothing alone): with a model,
  `given.as(FHIR.string).count()` 3 (an item with only an extension) and
  `given.as(System.String).count()` 2; `birthDate.as(DateTime) + 1 day` is a
  Date; R4 against R4B `Patient.id` (A3); the model/no-model mismatch of A4
  (no model `birthDate.as(Date)` kept, `as(DateTime)` empty; R4 the reverse),
  inherited from 017's D6 and pinned.
- `funcs.Get("as").Fn` over R4, R5 and a nil Context (D7).
- **Unchanged:** R5 `as()`, and `is()`/`ofType()` everywhere, against main's
  answers over the same rows.
- **References:** the battery `primitive-type-operators` extended with the new
  rows, on the three engines, R4 and R5 models. Both references give `[]`
  before R5; every row is classified (expected: this engine alone, by the R4
  text, cataloged as JS-12 and JAVA-18).
- corpusdiff over the official R4, R4B and R5 examples against main.

### Mutations (step 7)

The pre-R5 branch removed; the version inverted; the operator not routed
through it; `is()` made permissive with it; the R5 table used before R5;
`System.code` let through; the System value returned instead of the item; R5
models taken for pre-R5; and 017's 15 kept running.

### Dependencies

None.

## 3. Adversarial round (Opus, 2026-11-02; Codex at its limit)

| # | Severity | Finding | Resolution |
|---|---|---|---|
| F1 | high | ofType() does not match by the System type its value converts to: with a model it compares the FHIR type's name (`primitiveTypeMatches`), so `gender.ofType(System.String)`, `issued.ofType(System.DateTime)` and `telecom.rank.ofType(Integer)` are `[]` in R4 and R5, against R5's "ofType() does not have such restrictions". D3 and the metamorphic test rested on it | **Accepted**, confirmed by a probe. The user moved ofType() to its own increment, 017, before this one |
| F2 | high | *(superseded by 017's table, see round 2 A4)* D2 contradicts R4's own table (date → DateTime), and its reason is false: FHIRPath 2.0.0, which R4 builds on, defines Date | **Accepted.** The user chose the packages as the source of the System type (`X.value`), with no ADR: date → Date. Recorded as a departure from the table, with its rows |
| F3 | medium | D2 reads the System type from how the engine stores the value (`types.SystemValue`): an integer past int64 is a Decimal, `"5"` a String | **Accepted.** The source is the package's `X.value` type, not the value read |
| F4 | medium | The spec's own examples are missing: no-model `given.as(System.string)` is `[]` and `as(System.String)` `[A, B]`; `given.ofType(System.string)` is `[]` | **Accepted**, confirmed. The literal examples go into the tests of 017 and 018, in every version and with no model |
| F5 | medium | The reading of the text is generous: "valid" may mean only that no error is raised; FHIRPath 2.0.0's `as` returns the value "if it is of the type specified"; the R4 example shows only a string | **Accepted** in part: the readings are stated in CONFORMANCE.md. The user chose this reading (option 1) |
| F6 | medium | D4 changes behaviour with no model, R5 data included, with no warning | **Accepted.** The user kept pre-R5 with no model (the precedent); CONFORMANCE.md and the CHANGELOG say so, and R5 data with no model is tested |
| F7 | low | Tests that will flip: `typing_test.go:29-44`, `TestAsAnswersAsIsWithoutAModel` (`:157-168`), the comments at `system_typed_test.go:56` and `castCollection`, `hl7-divergences/r4.tsv` row 115 | **Accepted**, list corrected by round 2 A1 |
| F8 | medium | The battery lacks code, uri, id, positiveInt, unsignedInt and instant, and R4B; corpusdiff only shows change | **Accepted**: 017's battery covers those types; expected values come from the package, not from main |

## 3b. Adversarial round of the revised plan (Opus, 2026-11-03)

Probe matrix: 18 paths × 38 specifiers × no model, R4, R4B, R5, comparing as(),
the operator and ofType(); a prototype of D3 run against the suites.

| # | Severity | Finding | Resolution |
|---|---|---|---|
| A1 | high | F7's list of tests that flip is wrong: with D3 prototyped, `typing_test.go:29-44` and `TestAsAnswersAsIsWithoutAModel` stay green; `oftype_system_type_test.go:528` and `system_typed_test.go:81` fail in R4/R4B | **Accepted**, exit criterion 2 lists the two and the stale comments |
| A2 | medium | A third reading: R5 moved the sentence only to keep its 0..* example valid under as()'s new singleton rule, so a singleton keeps the permission in R5 | **Accepted**: the readings are listed and the invariant rewrite (0..1 element) cited for D1. Which one to follow is the user's |
| A3 | medium | R4 types `Resource.id` System.String: `Patient.id.as(String)` is `[p1]` in R4 on main, `[]` in R4B | **Accepted**, confirmed by a probe; rows added |
| A4 | medium | §3 F2 says date → Date, §2 follows the table; with no model `birthDate.as(Date)` is kept, with R4 `as(DateTime)` | **Accepted**: F2 marked superseded, the mismatch (017 D6) pinned by a test |
| A5 | medium | `funcs.fnAs` is unreachable from the evaluator but exported, with an exact-name match | **Accepted**, confirmed (`plan.go:275`); D7 routes it through a Context matcher |
| A6 | low | The metamorphic identities hold by construction | **Accepted**: independent rows added |

Checked and fine: D3 changes no R5 answer; outside the System-type branch as()
and ofType() agree today in every model (System.Any, Quantity, complex and FHIR
names, valueless items, checkTypeSpecifier errors).

## 4. Decision (2026-11-03)

The battery `as-system-type` (the references, measured again) showed that
fhirpath.js and the HL7 validator both answer `as()` as `is()` in R4 and R5,
and neither splits by version. With A2's reading, the version split rests on a
text that admits three readings. The user chose reading 3, with both
references: no code change. §2's plan is not implemented.

- CONFORMANCE.md: the section "as() of a FHIR primitive by a System type, in
  every version", with the three readings and the measurement.
- SPEC-9 rewritten: ambiguous as well as moved between versions.
- #241 closes with this decision; ofType() (017) is the way to keep a FHIR
  primitive by its System type.
- After the merge, the go-searchindex session gets the finding (no edits
  there).
