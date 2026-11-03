# 018 — as() of a FHIR primitive by its System type, by version (#241)

Status: step 2, plan (2026-11-02). The user chose to follow each version's
text (option 1).

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

### gofhir/server

It uses this library, with no rule of its own for `as()`. Nothing to take.

## 2. Plan

### Decisions

- **D1 (version):** the rule is split by `enforcesR5Rules()`, with no new
  API. Before R5 (a model declaring 4.x, or no model, or a model that does
  not declare its version), `as()` keeps a FHIR primitive asked for by a
  System type it converts to. From R5 on, `as()` stays as it is now.
- **D2 (which System type):** the one its value converts to, which is the
  System value the engine reads (`types.SystemValue(item)`), matched as
  `systemTypeMatches` matches a System value. No table is written into the
  code. The table and the packages differ on three rows, settled as follows:
  - `date`: the engine reads a Date, so `birthDate.as(System.Date)` keeps it
    and `as(System.DateTime)` does not. The table's "date → DateTime" predates
    FHIRPath's own Date type (N1 has System.Date), and the packages type
    `date.value` `System.Date`. FHIRPath's implicit Date → DateTime conversion
    is a conversion, not a type, and `as()` is a type test.
  - `unsignedInt` and `positiveInt`: the engine reads a JSON number as an
    Integer, so `rank.as(Integer)` keeps it. The packages' `System.String`
    (SPEC-8) is not followed, since the value read is an Integer.
  - R5's `integer64`: R5 enforces the rule, so it does not arise.
- **D3 (is and ofType):** both unchanged. `is()` is the explicit exception
  in every version. `ofType()` already keeps a FHIR primitive by its System
  type: R5's text says so, and R4's says nothing.
- **D4 (no model):** pre-R5, as the precedent has it. A choice's
  `value.as(DateTime)` then keeps the value, so a choice reads like a plain
  element. A plain element's answer is unchanged.
- **D5 (what is returned):** the item itself, still the FHIR primitive, with
  its extensions and its FHIR type: `as()` "returns the value of the left
  operand". So `value.as(DateTime).type().name` is still `dateTime`, and
  `.extension` still reaches it.
- **D6 (wrong System type):** `value.as(String)` on a dateTime is empty in
  every version. Its value is a DateTime, not a String.

### Out of scope

- FP-16: `as(System.Quantity)` and `is System.Quantity`, where this engine
  alone differs.
- FP-17: `birthDate.ofType(Date)`, where fhirpath.js differs.
- The validator's `is` operator against its `.is()` function.
- `ofType()` in R4, unchanged by D3.

### Exceptions to rule 5

None. The rule names no FHIR type. The version comes from the model, and the
System type comes from the value read.

### Exit criterion

1. With an R4 or R4B model, and with no model, `as(T)` and `as T` keep a FHIR
   primitive when T is a System type (qualified, or unqualified and
   capitalized) that its value is. That includes
   `Observation.value.as(DateTime)` on a valueDateTime.
2. With an R5 model every answer is unchanged, `is()` and `ofType()` are
   unchanged in every version, and every test on main still passes, except
   those that held the old R4 answer, each changed with its section cited.
3. CONFORMANCE.md has a section with the three texts, the measurements and
   the references cataloged. SPEC-9 is rewritten (R5 changed the rule, which
   is not a defect of R4), and the findings JS-12 and JAVA-17 are recorded.
4. Mutations caught, `make check` and corpusdiff green.

### Tests (step 5)

- **`conformance/as_primitive_by_version_test.go`**, over R4, R4B, R5 and no
  model, with the expected answers taken from the text (D1–D6):
  - each primitive kind: boolean, string, code, uri, id, markdown, integer,
    positiveInt, unsignedInt, decimal, date, dateTime with and without a
    time, instant, time;
  - each kind both as a choice (`value[x]`, `extension.value[x]`) and as a
    plain element (`Patient.gender`, `birthDate`, `telecom.rank`, `active`,
    `Observation.issued`);
  - asked by its System type in four forms (`as(DateTime)`,
    `as(System.DateTime)`, `as DateTime`, `as System.DateTime`), and by a
    wrong System type (D6);
  - a collection of several items before R5, which filters as `as()` does
    there;
  - D5: `type().name`, and `.extension` of the result.
- **Metamorphic:** before R5, `X.as(S)` equals `X.ofType(S)` for a singleton
  FHIR primitive X and every System type S, as the R4 text treats both alike.
  From R5 on, `X.as(S)` is empty exactly when `X is S` is false.
- **Unchanged:** `is()` and `ofType()` give the same answers as on main over
  the same rows, R5's `as()` too. They are compared against main's answers,
  written as the expected values of the rows.
- **Data:** corpusdiff over the official R4, R4B and R5 examples, against
  main. The battery `primitive-type-operators` is extended with the D5 and D6
  rows and run on the three engines with the R4 and R5 models. Every
  difference is classified.

### Mutations (step 7)

The rule disabled; the version test inverted (R5 permissive, R4 strict); the
operator not routed through the rule; `is()` made permissive along with it;
matching by the FHIR type name instead of the value's System type; returning
the System value instead of the item (D5); every System type accepted (D6);
no model taken for R5.

### Dependencies

None.

## 3. Adversarial round (Opus, 2026-11-02; Codex at its limit)

| # | Severity | Finding | Resolution |
|---|---|---|---|
| F1 | high | ofType() does not match by the System type its value converts to: with a model it compares the FHIR type's name (`primitiveTypeMatches`), so `gender.ofType(System.String)`, `issued.ofType(System.DateTime)` and `telecom.rank.ofType(Integer)` are `[]` in R4 and R5, against R5's "ofType() does not have such restrictions". D3 and the metamorphic test rested on it | **Accepted**, confirmed by a probe. The user moved ofType() to its own increment, 017, before this one |
| F2 | high | D2 contradicts R4's own table (date → DateTime), and its reason is false: FHIRPath 2.0.0, which R4 builds on, defines Date | **Accepted.** The user chose the packages as the source of the System type (`X.value`), with no ADR: date → Date. Recorded as a departure from the table, with its rows |
| F3 | medium | D2 reads the System type from how the engine stores the value (`types.SystemValue`): an integer past int64 is a Decimal, `"5"` a String | **Accepted.** The source is the package's `X.value` type, not the value read |
| F4 | medium | The spec's own examples are missing: no-model `given.as(System.string)` is `[]` and `as(System.String)` `[A, B]`; `given.ofType(System.string)` is `[]` | **Accepted**, confirmed. The literal examples go into the tests of 017 and 018, in every version and with no model |
| F5 | medium | The reading of the text is generous: "valid" may mean only that no error is raised; FHIRPath 2.0.0's `as` returns the value "if it is of the type specified"; the R4 example shows only a string | **Accepted** in part: the readings are stated in CONFORMANCE.md. The user chose this reading (option 1) |
| F6 | medium | D4 changes behaviour with no model, R5 data included, with no warning | **Accepted.** The user kept pre-R5 with no model (the precedent); CONFORMANCE.md and the CHANGELOG say so, and R5 data with no model is tested |
| F7 | low | Tests that will flip: `typing_test.go:29-44`, `TestAsAnswersAsIsWithoutAModel` (`:157-168`), the comments at `system_typed_test.go:56` and `castCollection`, `hl7-divergences/r4.tsv` row 115 | **Accepted**: each is changed with its section cited |
| F8 | medium | The battery lacks code, uri, id, positiveInt, unsignedInt and instant, and R4B; corpusdiff only shows change | **Accepted**: 017's battery covers those types; expected values come from the package, not from main |
