# 009 — A FHIR temporal value is read by FHIR's form, not the literal grammar

Status: step 4, awaiting the user's approval
Issue: #231

## Problem

`parseTypedString` (`types/object.go:2184`) reads a model value declared
`dateTime` or `instant` with `NewDateTime`, whose `dateTimePattern`
(`types/datetime.go:59`) is the FHIRPath *literal* grammar:
`'@' DATEFORMAT 'T' (TIMEFORMAT TIMEZONEOFFSETFORMAT?)?`. That grammar is
right for `@2015-02T`, but a value in a resource is FHIR's `dateTime`, whose
form has no time without a full date, no time without seconds and no
trailing `T`.

## Prior review

### The specification (R4, R4B, R5)

`datatypes.html`, identical in the three versions but for "time zone" →
"timezone offset" in R5 (fetched 2026-10-25; SHA-256 R4 `03d88289…`,
R4B `03614853…`, R5 `d826e181…`):

- `dateTime`: "The format is YYYY, YYYY-MM, YYYY-MM-DD or
  YYYY-MM-DDThh:mm:ss+zz:zz … If hours and minutes are specified, a time zone
  SHALL be populated. Seconds must be provided due to schema type constraints".
- `instant`: "in the format YYYY-MM-DDThh:mm:ss.sss+zz:zz … The time SHALL
  specified at least to the second and SHALL include a time zone."
- `time`: "in the format hh:mm:ss … Seconds must be provided".
- R5: "The regexes are provided to assist with tooling, but are informative,
  not normative … The regexes may allow a broader set of values than are
  actually valid".
- R5's `StructureDefinition-dateTime` comment: "UTC offset is allowed for
  dates and partial dates" (absent in R4 and R4B).

The `regex` extension of `<type>.value` in the pinned core packages
(`build/corpusdiff/cache`):

| Type | R4 = R4B | R5 |
| --- | --- | --- |
| dateTime | `YYYY(-MM(-DD(Thh:mm:ss(.f+)?OFFSET)?)?)?` — offset required with a time | `YYYY(-MM(-DD(Thh:mm:ss(.f{1,9})?)?)?(Z\|(+\|-)(hh:mm)?)?)?` — offset optional, allowed after a month or a day, and `+` alone |
| instant | `YYYY-MM-DDThh:mm:ss(.f+)?OFFSET` | the same, `.f{1,9}` |
| time | `hh:mm:ss(.f+)?` | `hh:mm:ss(.f{1,9})?` |

### The three engines (measured)

`make three-engine` on two probe batteries (fhirpath.js 5.2.0, HL7 validator
6.10.2, R4 and R5 models; R4 and R5 answered alike), expressions
`X > @1900` and `X + 1 year`; "DateTime" = read as a date-time, "refused" =
read as a string / evaluation error:

| Value | Element | Here | fhirpath.js | Java | Text |
| --- | --- | --- | --- | --- | --- |
| `2020-01T10:30:00Z`, `2020T10:30:00Z` | dateTime | DateTime | refused | refused | refused |
| `2020T`, `2020-01T`, `2020-01-15T` | dateTime | DateTime | DateTime | refused | refused |
| `2020-01-15T10:30Z` (no seconds) | dateTime | DateTime | DateTime | refused | refused |
| `2020-01-15T10:30:00` (no offset) | dateTime | DateTime | DateTime | DateTime (assumes local) | SHALL have offset |
| `2020-01-15T10:30:00.1234567890Z` | dateTime | DateTime, ms | wrong instant (reads 1234567890 ms) | DateTime, ms | R5: ≤ 9 digits |
| `2020-01-15T24:00:00Z`, `2020Z` | dateTime | String | refused | refused | refused |
| `2020-01-15Z`, `2020-01+05:00`, `2020-01-15T10:30:00+` | dateTime | String | refused | refused | R5 comment allows the first two |
| `2020`, `2020-01-15` | instant | DateTime | refused | DateTime | refused |
| `2020-01-15T10:30:00` | instant | DateTime | refused | DateTime (assumes local) | refused |
| `2020-01-15T10:30:00Z` | date | DateTime | refused | DateTime | refused |
| `2020-01-15`, `2020-01-15T10:30:00Z` | time | Date / DateTime | refused | refused | refused |
| `10`, `10:30` | time | Time | Time | Time | refused |

By the three-engine rule (CONFORMANCE.md rule 1): both refuse → follow;
they split → follow the one the text bears out; they agree against the
text → follow them and record it.

### `gofhir/server` (frozen)

`internal/storage/index/extractor.go:1099` `parseDateRange` accepts the Go
layouts `2006-01-02T15:04:05Z07:00`, `2006-01-02T15:04:05`, `2006-01-02`,
`2006-01`, `2006`: full date and seconds before any time, offset optional,
and anything else is unindexed. **Taken:** the same acceptance set for
`dateTime`. **Discarded:** its implementation (Go layouts; the engine
already parses), and its lack of an `instant` rule.

## Approach

`parseTypedString` reads a value declared with a FHIR temporal type by
FHIR's form, and **a value outside it is the typed `String`, returned with
`ok = true`**. Today a failure returns `(nil, false)` and the caller keeps
the heuristic reading (`types/object.go:2122-2131`): that is why
`2020-02-30` is a String (the heuristic fails too) but `birthDate:
"2020-01-15T10:30:00Z"` is a DateTime and `valueTime: "2020-01-15"` a Date
(measured). The forms, checked on the text the parser already validated:

- `dateTime`: no `T`, or `T` at index 10 (a full date) followed by at least
  `hh:mm:ss`. The offset stays optional (the three engines read an unzoned
  `dateTime`; `WithDefaultOffset` depends on it).
- `instant`: `T` at index 10, at least `hh:mm:ss`, and an offset
  (fhirpath.js and the text; Java's reading under "Measured, and not
  followed").
- `date`: `NewDate` only — a date with a time is no date (fhirpath.js and
  the text, "YYYY, YYYY-MM, or YYYY-MM-DD"; Java reads it, listed).
- `time`: `NewTime` only — a date is no time (fhirpath.js and Java both
  refuse). Without seconds stays a Time: the three engines agree against
  the text; recorded.

The check is keyed to FHIR type names (case-sensitive `dateTime`,
`instant`, `date`, `time`). A System type (`System.DateTime`, the type of
`dateTime.value` in the core packages and of custom models) keeps the
FHIRPath grammar: `systemValue` stops sharing the lowercased switch.
Without a model nothing changes (review round 1): a key's suffix
(`valueDateTime`) is a guess at the field, and every route reads the value as
before. Literals and `toDateTime()` keep the FHIRPath
grammar: `'2020T'.toDateTime()` and a refused value's `.toDateTime()` still
convert, as the FHIRPath text says.

### Out of scope

- An offset after a date or partial date (`2020-01-15Z`): R5's comment
  allows it, but the three engines refuse it in R4 and R5 alike (measured).
  Unchanged (a String), recorded as followed.
- `time` without seconds, as above.
- The untyped heuristic (`tryParseTemporalString`) for a key without a
  declared type: not a declared FHIR type. Recorded (`issued: "2020T"` is a
  DateTime without a model).
- More than 9 fraction digits in R5: this engine and Java agree.
- #232: the next increment.

### Rule 5

The forms are written in the engine, not read from the packages: R5 says
the regexes "are informative, not normative" and "may allow a broader set of
values than are actually valid", R5's dateTime regex accepts
`2020-01-15T10:30:00+`, and the `Model` interface exposes no primitive
patterns. **Pending user decision:** record this as an ADR (the charter's
form for a rule-5 exception; the repo has no `docs/adr/` yet) or as a
CONFORMANCE.md section like the other temporal rules.

## Tests (first, from the text and the measurements)

`conformance/fhir_datetime_grammar_test.go`, R4, R4B and R5 models. The
oracle is **the Go type of the value** (DateTime / Date / Time / String,
plus its FHIR type name) **and behaviour** (`X > @1900` true vs a type
error): under a model `is DateTime` and `is String` are false for both
readings (measured), so they prove nothing.

- Refused → String: the issue's six, `2020-01T10:30Z`, `2020-01-15T10`,
  `2020-01-15T10Z`; instant `2020`, `2020-01`, `2020-01-15`,
  `2020-01-15T10:30Z`, `2020-01-15T10:30:00`; date `2020-01-15T10:30:00Z`,
  `2020T`; time `2020-01-15`, `2020-01-15T10:30:00Z`.
- Accepted, value kept exactly (`toString()`): dateTime `2020`, `2020-01`,
  `2020-01-15`, `2020-01-15T10:30:00Z`, `2020-01-15T10:30:00.123+05:00`,
  `2020-01-15T10:30:00`, `2016-12-31T23:59:60Z`, `…+14:00`, `…-12:00`;
  instant with offset and fraction; date `2020-02-29`; time `10:30:00`,
  `10:30`, `10`.
- Paths: choice (`Observation.effective`), declared (`Period.start`,
  `Patient.birthDate`), `meta.lastUpdated`, `Timing.event` (mixed array with
  null slot), extension `valueDateTime`, nested extension, contained,
  `Bundle.entry.resource`, `children()`/`descendants()`, `_x` beside a
  refused value (`.id`, `hasValue()`, extension kept), extension-only
  primitive; without a model `valueDateTime`/`valueInstant`.
- System types: a custom model typing an element `System.DateTime` reads
  `2015-02T` by the FHIRPath grammar.
- Unchanged: literals `@2015-02T`, `@2015T10:30:00Z`; `'2020T'.toDateTime()`;
  a refused value's `.toDateTime()` converts by the literal grammar.
- Each case cites the `datatypes.html` sentence or the measurement.

## Validation

- **Corpus:** `make corpusdiff BASE=main`, R4, R4B, R5 examples. Expected
  none: a grep of the three example packages found no dateTime with `T`
  lacking seconds or trailing `T`; date/time cases to be grepped likewise
  before the run.
- **Three engines:** both probe batteries committed under
  `difftest/batteries/fhir-datetime-grammar/` (R4 and R5; R4B by hand with
  the jar), run before and after.
- **Fuzz:** `FuzzFHIRTemporalForm`, **both directions**: for dateTime and
  instant, read as DateTime ⇔ (matches the R4 package regex, with the
  offset made optional for dateTime — a hand-adapted oracle, said so) ∧ on
  the calendar and clock ∧ offset within ±14:00; String otherwise; never a
  Date or Time.
- **Metamorphic:** the same value under every path above and in the three
  versions reads alike; reordered keys; an unknown extension beside it.
- **Mutations** (copy of the repo, all re-run before each review round):
  drop the check; accept `T` at any index; accept `hh:mm`; drop the instant
  offset; check literals too; return `(nil, false)` on failure (the
  heuristic comes back); require an offset for dateTime; refuse a fraction
  or second 60; refuse `2020` under dateTime; apply the FHIR check to
  `System.DateTime`; refuse everything. Each must fail a test.

## Consumers (not silent, recorded)

- go-searchindex `adr001.go:439` `primitiveRange`: a refused value is now
  "unparseable" and not indexed, where today a value without seconds is
  indexed with an "invalid form" warning; `validForm`'s dateTime/instant
  branches become unreachable. To be pinned there when it adopts the
  release.
- go-validator `constraint.go:316`: `Period.start: "2020-01-15T10:30"` now
  fails per-1 by an evaluation error (it already fails the regex phase);
  two refused Strings compare as strings without error.

## Upstream

Drafted, not filed (each needs approval):

- HL7 (charter principle 10.4): R5's dateTime regex accepts a sign with no
  offset, `2020-01-15T10:30:00+`; to be checked against the current build
  before drafting.
- fhirpath.js 5.2.0 reads `2020-01-15T10:30:00.1234567890Z` as
  1,234,567,890 ms, giving `2021-01-29T17:26:07.890` for `+ 1 year`.

## Record

- Step 1 done 2026-10-25: measurements above (probe batteries in the
  session scratchpad, to be committed in step 7).
- Step 2 done 2026-10-25: plan written.
- Step 3 done 2026-10-25, two reviewers (Fable, Codex), each finding
  verified by measurement before resolving:
  - Accepted: the `(nil, false)` fallback keeps the heuristic reading (both;
    measured `birthDate` with a time → DateTime, `valueTime` with a date →
    Date) → typed String with ok, scope widened to `date` and `time`;
    `is DateTime`/`is String` prove nothing under a model (measured) → Go
    type and behaviour as oracle; System.DateTime shares the lowercased
    switch → keyed to FHIR names; R5's comment allows offsets on partial
    dates → measured, the three refuse, recorded; the regexes are
    "informative, not normative" (R5 text) → rule-5 ground; a one-way fuzz
    misses over-rejection → both directions; missing paths, mutations for
    over-restriction, R4B rows, consumers.
  - Pending the user: ADR or CONFORMANCE.md for the rule-5 exception (Codex
    asked for an ADR or a package-derived interface).
  - Codex's sandbox could not run Go or `gh`; Fable's measurements and mine
    cover its claims.
- Step 4 done 2026-10-26: approved by the user, on condition that
  conformance is followed and the other libraries keep working; the rule-5
  exception goes in CONFORMANCE.md ("A FHIR date, dateTime, instant or time
  is read by FHIR's form"), not an ADR.
- Step 5: `conformance/fhir_temporal_form_test.go` failed 365 checks before
  the change (every refused value in every placement and version), and none
  of the accepted or unchanged ones once three test-format slips were fixed
  (a fraction written back as `.500`; `Type()` names of booleans and ids,
  not under test).
- Step 6: `fhirString` and `fhirDateTimeForm` in `types/object.go`; a string
  under a FHIR type is no longer read by the heuristic first. The battery
  directory is `difftest/batteries/fhir-temporal-form`, not
  `fhir-datetime-grammar`. `time` with a leading `T` (`T10:30:00`, which
  `NewTime` reads) added: fhirpath.js reads it, Java answers otherwise, the
  text refuses it.
- Step 7, validation, 2026-10-26:
  - Three engines, after: no case where this engine alone differs; the
    rows that remain are the splits listed in CONFORMANCE.md.
  - corpusdiff against main: 0 differing answers in R4, R4B and R5.
  - Fuzz: 60 s, no failure.
  - go-validator tests (fhirpath replaced by main and by this tree through
    go.work): pass alike. go-searchindex: `TestFhirpathInvalidFormsKnownLoss`
    fails as designed (it detects the fix); `TestDateFormsInvalidForTheirType`
    changes: values outside their form it indexed with an "invalid form"
    warning are now strings and go unindexed — pending the user.
  - Mutations (copy of the repo, root and conformance tests): 13 of 14
    caught after rewriting M01, which did not compile. Survivors, both
    equivalent: M02 (`T` at any index; `NewDateTime` puts `T` at 4, 7 or 10,
    and at 4 or 7 no valid text has `:` at 13 and 16 with 19 characters;
    90 s of fuzz on the mutant found nothing); M05 (the FHIR check on
    `System.DateTime`: `systemValue` falls back to the heuristic, which reads
    the same DateTime).
- User decisions 2026-10-26: keep the four readings where both libraries
  agree against the text (dateTime with a time and no offset, time without
  seconds, R5 fraction over 9 digits, offset on a partial date in R5) — none
  of them occurs in the R4, R4B or R5 official examples (scan of the pinned
  packages), and go-validator flags their form by regex; keep the change for
  go-searchindex, which adapts when it adopts the release (its own parser for
  values outside their form, and the five values out of
  `TestFhirpathInvalidFormsKnownLoss`).
- `make check` green before review round 1 (lint 0 issues, `-race` tests).
- Step 8, review round 1 (2026-10-26). Codex could not run (its sandbox
  refused writes even in the copy prepared for it: no Go build cache); Opus
  replaced it as the second model, with Fable. Findings and resolution:
  - **Medium, both reviewers, confirmed** (probe without a model over
    `"valueDateTime": "2020-01-15T10:30Z"`: `Observation.value` a String,
    `Observation.valueDateTime` and `children()` a DateTime): without a
    model only the typed-key route (`GetCollectionParsedAs`) reads by FHIR's
    form; `Get` and `Children` keep the heuristic, so one value reads two
    ways where before the change it read one. **Accepted.** Planned fix:
    without a model nothing changes — the typed-key route reads as before
    (FHIRPath grammar), consistent with leaving the untyped heuristic out of
    scope; FHIR's form applies where a model declares the type. To check
    first that `GetCollectionParsedAs` is reached only without a model;
    `TestWithoutAModelATypedKeyIsReadByItsFHIRForm` becomes a test that the
    routes agree (`value`, `valueX`, `children()`), and the plan's "model-free
    typed key gets the FHIR form" is withdrawn.
  - Low (Opus, to confirm): a custom model's `System.Date` holding a date with
    a time reads as a DateTime, `System.Time` holding a date as a Date —
    `systemValue` falls back to the heuristic. **Accepted:** a System
    temporal type outside its grammar is a typed String; rows added to
    `TestTheFHIRPathGrammarIsUnchanged`. M05 is then no longer equivalent and
    must be caught.
  - Low (Opus, measured 3 → 2 allocs for a code, 6 → 5 for a dateTime): the
    value is boxed in `fhirString` and again in `withFHIRType`. **Accepted:**
    tag inside `fhirString`.
  - Low (both): style — `parseTypedString` keeps a `read` parameter always
    true and an unreachable `instant` case, its comment is wrong for the
    default branch; `fhirDateTimeForm` takes a DateTime for one bool;
    `EqualFold` repeats the lowered switch key; `read` set twice; a comment
    without a period. **Accepted.**
  - Low (Fable, mutation `len >= 14` survives; not equivalent:
    `2020-01-15T10:30` would index past the end): no test holds a
    no-offset `hh:mm` value. **Accepted:** `2020-01-15T10:30` and
    `2020-01-15T10:30+05:00` added to the dateTime invalid list and the fuzz
    seeds; the length mutation added to the mutation set.
  - Low (Fable): the error for a refused value names the FHIR type
    (`cannot apply '+' to dateTime and Quantity`). **Rejected for this
    increment:** pre-existing since #145 for off-calendar dates, and the
    wording of evaluation errors is not this change's scope; noted for a
    later one.
  - Fable's three-engine rerun did not finish (the shell failed); the
    battery's columns stay this increment's own measurement.
- Blocked 2026-10-26: every shell command fails with EPERM opening the
  harness tasks directory under `/Volumes/dev/tmp/claude/…`; fixes wait
  until it works, then all mutations are rerun and round 2 is launched.
- Round 1 fixes applied 2026-10-26 (the shell came back):
  - `GetChoiceCollection` is the evaluator's fallback without a model (or
    where the model does not know the path), and types a key by its
    spelling, `DateTime`; a model declares FHIR's names, `dateTime`. So
    `fhirString` applies FHIR's forms to FHIR's names only and returns false
    for `Date`, `DateTime`, `Instant`, `Time`, which keep the FHIRPath grammar
    and the heuristic fallback, as the key read directly does.
    `TestWithoutAModelEveryRouteReadsAValueAsBefore` checks the routes agree
    where they agreed on 1.16.6, and pins the two splits 1.16.6 already had
    (`valueInstant: "2020"`: DateTime by the choice, Date by the key;
    `valueTime: "10:30"`: Time, String), measured on a main snapshot.
  - `systemValue`: a System Date, DateTime or Time outside its grammar is a
    String; rows in `TestTheFHIRPathGrammarIsUnchanged`.
  - `fhirString` tags its value (one boxing); `parseTypedString` lost `read`;
    `fhirDateTimeForm` takes the offset flag; `fhirInstant` constant
    (goconst); comments fixed.
  - Tests: `2020-01-15T10:30`, `2020-01-15T10:30+05:00` in the dateTime
    invalid list; fuzz seeds `2020-01-15T10:30`, `…:0`, `…:00.`.
  - Evidence after the fixes: root and conformance tests green; `make check`
    green; three-engine battery as before (no row where this engine alone
    differs); corpusdiff against main 0 differences in R4, R4B, R5;
    mutations 16 of 17 caught, new ones M03b (length bound), M05 (System
    fallback, no longer equivalent), M15/M16 (guessed suffixes) among them,
    M02 the equivalent one.
- Round 2 (2026-10-26, Fable and Opus, on the round-1 fixes): converged — no
  finding of medium severity or higher.
  - Opus: without a model, 1,188 probe readings (7 keys × 22 values × routes)
    byte-identical to main; under the generated models every route gets
    FHIR's form. Low: `fhirString`'s comment claimed a guessed suffix reads
    as the key read directly, false for `Instant` and `Time` (the pinned
    1.16.6 splits) — comment fixed.
  - Fable: 5,852 probe lines, every change against main is temporal → String
    under a declared FHIR or System temporal type, none the other way. Low:
    the `read` mark on `systemValue`'s String fallback was unpinned (without
    it `getValue()` answers empty) — `getValue()` rows added, mutation M17
    caught. Informational: a custom model whose ChoiceTypes hands
    capitalized names breaks the Model contract ("FHIR type code") and reads
    as on main; no change.
  - Mutations, full set before closing: 17 of 18 caught; M02 equivalent.
- Step 9, closed 2026-10-26: `make check` green (lint 0 issues, `-race`
  tests in every module); go-validator green against this tree;
  go-searchindex fails only the two tests the user decided it adapts
  (`TestDateFormsInvalidForTheirType`, `TestFhirpathInvalidFormsKnownLoss`).
  Not committed, at the user's discretion. Still pending the user: filing the
  drafted fhirpath.js and HL7 reports; commenting on #231/#232.
