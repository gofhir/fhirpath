# 020 — A default offset for a DateTime coarser than the hour (#256)

Status: closed (step 9), 2026-11-04. Option 1 approved by the user on 2026-11-04.

## 1. Prior review

### The issue

gofhir/cql cannot place a CQL DateTime written to the year, month or day
(`@2012-01-01T`, `DateTime(2012, 1, 1)`, `ToDateTime(@2012-01-01)`) at the
evaluation offset. `WithDefaultOffset` (`types/datetime.go`) returns such a
value unchanged ("@2020 has no instant to place"), so `EffectiveOffset`
reports none. Against a value with an offset, the comparison then reads the
day as written and answers unknown, where both CQL references answer false
(go-cql plan 018, measured at -03:00 against Java 5.4.0 and cql-execution
3.3.2).

### Specification

- FHIRPath (§DateTime, "Comparison"): "For DateTime values that do not have
  a timezone offsets, whether or not to provide a default timezone offset is
  a policy decision." FHIRPath does not settle the policy, and this engine
  supplies no default of its own. A FHIRPath DateTime coarser than the hour
  cannot be written with an offset (`@2012-01-01T` has no time to carry one),
  so the change cannot alter what a FHIRPath expression answers unless a
  caller sets a default.
- CQL 1.5.3, literals: "If no timezone offset is supplied, the timezone
  offset of the evaluation request timestamp is assumed." ToDateTime of a
  Date: "the time components unspecified, except for the timezone offset,
  which will be set to the timezone offset of the evaluation request
  timestamp." Both apply at any precision.

### gofhir/server (frozen)

It has no default offset (`grep DefaultOffset` over go-fhir-server: none).
Nothing to take.

### What reads EffectiveOffset (what the change reaches)

| Reader | Today, below the hour | With a placed day |
|---|---|---|
| `compareTemporal` (`types/temporal_compare.go`) | compared as written | at different offsets: start instants (see the open question) |
| `DateTime.Equal` fast path | precision equal, offsets equal | unchanged: equal offsets compare components |
| `ToTime` (`AddDuration`) | UTC | the default's zone; components are not shifted |
| `temporal_diff` | normalizes only at time-of-day precision | unchanged |
| `timezoneOffsetOf` (`funcs/temporal_components.go`) | empty | the default |
| boundary (`funcs/boundary.go`) | the span of every offset | the default |
| `triedKey`, sort keys | no offset in the key | the default |

### Open question: how a placed day compares with a finer value elsewhere

`compareTemporal` compares two values at different offsets as the instants
they start at (the HL7 validator orders them so; fhirpath.js does not, as
the FHIRPath table below shows).
For a placed day this can decide what CQL leaves unknown:
`@2012-01-01T` at -03:00 starts at 03:00Z, so `< @2012-01-01T05:00:00.000Z`
would be true, though 05:00Z is 02:00 on the same day at -03:00.

#### CQL references (measured 2026-11-04)

Measured by the go-cql session: Java 5.4.0 and cql-execution 3.3.2, evaluated
at 2026-01-01T00:00:00-03:00, with go-cql's `internal/referenceharness`
(probe `probe256.cql`; go-cql's scratchpad `r256/answers.md`). The options:
**A** converts the finer value to the coarser value's offset, then compares
precision by precision. **B** converts both to UTC. **C** compares start
instants, as this engine does today.

| # | CQL | Java | cql-execution | A | B | C |
|---|---|---|---|---|---|---|
| 1 | `@2012-01-01T < @2012-01-01T05:00:00.000Z` | null | null | null | null | true |
| 2 | `@2012-01-01T = @2012-01-01T01:00:00.000Z` | false | false | false | null | false |
| 3 | `@2012-01-01T = @2012-01-02T02:00:00.000Z` | null | null | null | false | false |
| 4 | `@2012-01-01T > @2011-12-31T23:00:00.000Z` | true | true | true | true | true |
| 5 | `@2012-01T = @2012-02-01T02:00:00.000Z` | null | null | null | false | false |
| 6 | `@2012T = @2013-01-01T02:00:00.000Z` | null | null | null | false | false |
| 7 | `@2012-01-01T = @2012-01-01T` | true | true | true | true | true |
| 8 | `@2012-01-01T = @2012-01-01T10:00:00.000` | null | null | null | null | null |
| 9 | `@2020-01-01T10+05:30 < @2020-01-01T04:45Z` | null | null | null | null | true |
| 9b | `@2020-01-01T10:00+05:30 < @2020-01-01T04:45Z` | true | true | true | true | true |
| 10 | `@2020-01-01T10:00+05:30 = @2020-01-01T05:15Z` | false | false | false | false | false |
| 13 | `@2012-01-01T same day as @2012-01-01T01:00:00.000Z` | true | true | | | |
| 14 | `ToDateTime(@2012-01-01) < @2012-01-01T05:00:00.000Z` | null | null | null | null | true |

Both references follow A in every row. Durations (rows 11, 12) split them:
`days between @2012-01-01T and @2012-01-03T02:00:00.000Z` is 2 in Java and
the uncertainty [0, 1] in cql-execution; `hours between @2012-01-01T and
@2012-01-02T05:00:00.000Z` is the Interval [3, 26] and the uncertainty
[2, 26]. Durations with uncertainty are go-cql's (its plans 016 and 020), not
this engine's.

#### CQL references at four offsets (measured 2026-11-04)

The go-cql session reran the probe, rows 15–21 added, evaluated at -03:00,
+14:00, -14:00 and +05:30: `types/testdata/cql-default-offset/README.md`, its
report as written, with the probe `probe256.cql` and the raw outputs of both engines beside it. Java and
cql-execution agree on every comparison at every offset, and every one
follows A. They disagree on every duration (rows 11, 12, 20, 21). The tests
take their expected values from that file.

#### FHIRPath references at the hour (measured 2026-11-04)

Row 9 is reachable in FHIRPath with written offsets, so I measured it and its
neighbours against fhirpath.js 5.2.0 and the HL7 validator 6.10.2, in R4 and
R5 (`difftest/batteries/cross-offset-precision`; same answers in both
versions):

| FHIRPath | here (C) | fhirpath.js | Java | A |
|---|---|---|---|---|
| `@2020-01-01T10+05:30 < @2020-01-01T04:45Z` | true | false | true | empty |
| `@2020-01-01T10+05:30 = @2020-01-01T04:45Z` | false | empty | error | empty |
| `@2020-01-01T10+05:30 = @2020-01-01T04:30Z` | empty | empty | error | empty |
| `@2020-01-01T10+05:30 = @2020-01-01T05:15Z` | false | false | error | empty |
| `@2020-01-01T10+05:30 < @2020-01-01T04:29Z` | false | false | false | false |
| `@2020-01-01T10+05:30 > @2020-01-01T04:29Z` | true | true | true | true |
| `@2020-01-01T10:00+05:30 < @2020-01-01T04:45Z` | true | true | true | true |
| `@2020-01-01T10+05:30 < @2020-01-01T05:31Z` | true | true | true | true |

The FHIRPath references split three ways where the precisions differ across
offsets. Java answers `<` by start instants (C) and errs on `=`; fhirpath.js
answers `=` as UTC hours (B) and `<` false, which none of A, B or C gives.
The text backs A: "both values are converted to a common timezone offset",
then compared precision by precision. This engine's C diverges from the text
and from CQL at mixed precisions across offsets. That is a FHIRPath question
in its own right, which this increment does not settle: recorded as FP-19.

## 2. Options

1. **A for a value coarser than the hour, C unchanged elsewhere
   (recommended).** `WithDefaultOffset` accepts year, month and day.
   `compareTemporal` converts the finer value to the coarser value's offset
   when the offsets differ and either value is coarser than the hour. Coarser
   than the hour a value has an offset only from a default, so no FHIRPath answer
   changes. From the hour down C stays, as FP-19, for its own increment, with the three
   engines measured across precisions.
2. **A at every precision.** It settles FP-19 in this increment, and changes
   FHIRPath answers at mixed precisions across offsets (the first, second and
   fourth rows above), where the references disagree with each other.
3. **An opt-in variant** (`WithDefaultOffsetAtAnyPrecision`), as the issue
   offers. It leaves `WithDefaultOffset` as is. Not recommended: the default
   exists for callers whose language settles the policy, and CQL settles it
   at every precision; two entry points would document one policy twice.

## 3. Plan (option 1)

- **D1.** `WithDefaultOffset` accepts a DateTime at any precision. The offset
  checks stay: whole minutes, within fourteen hours. `String`, `HasTZ` and
  `TZOffset` stay as written.
- **D2.** `compareTemporal`, for a value coarser than the hour that has an
  offset (only a default gives it one):
  - **D2a.** Against a finer value at another offset, the finer value's start
    instant is re-expressed at the coarser value's offset, and the two are
    compared precision by precision (rows 1–6). A finer value written with a
    leap second is converted as the second before the one it is held as, so
    it stays in the minute it was written in (adversarial F3).
  - **D2b.** Against a value of the same unit at another offset (two
    different defaults), the two are compared as the spans they name: one
    ends before the other starts, or the order is unknown (F4).
  - **D2c.** Against a value that has no offset, the order is unknown, as
    `ErrOffsetMismatch` already says from the hour down. Without it, a bare
    day would equal two placed days that differ from each other (F1).
- **D3.** The other readers of `EffectiveOffset` change for a placed day:
  - `timezoneOffsetOf` gives the default;
  - the boundaries place the day at it, and a boundary at a date precision
    (`lowBoundary(8)`) gives a Date, which carries no offset;
  - `triedKey` carries the offset, and the sort keys carry its presence;
  - arithmetic keeps the default, as it does at the hour;
  - the integer durations in days and weeks (`temporal_diff`, `instant`)
    measure a placed day from its start at the default:
    `@2012-01-01T` at -03:00 to `@2012-01-03T02:00:00Z` is 1 day, where it is
    2 unplaced (F2). The CQL references split on this (row 11), and
    uncertainty is go-cql's, so the change is documented and pinned, not
    chosen by them.
### Out of scope

- FP-19: mixed precisions across written offsets, from the hour down.
- Durations and `difference` with uncertainty (go-cql).

### Exit criterion

- Rows 1–8 and 14–19 give both CQL references' answers at -03:00, +14:00,
  -14:00 and +05:30. The order goes through `Compare`, three-valued equality
  through `EqualTemporal` (F6). Expected values come from the references,
  measured at each offset by the go-cql session, not from rule A.
- `Equal` is transitive over a set of placed, bare and written values, so
  `distinct()` and `union()` do not depend on the input order (F1).
- Every FHIRPath answer is unchanged: the spec catalog, every difftest
  battery and the conformance suites give main's answers.
- The new tests fail on main, the mutations below are caught, and
  `make check` is green.

### Tests and data

- `types`, for both operand orders:
  - year, month and day precision against hour, minute, second and
    millisecond;
  - year boundaries at +14:00 and -14:00;
  - a leap second (F3);
  - two placed days at different defaults;
  - a placed day against a bare day and a Date.
- Order-independence of `distinct()` and `union()` over placed, bare and
  written values.
- Through the public API (`datetime_offset_test.go`): `timezoneOffsetOf`, and
  `lowBoundary`/`highBoundary` with and without a precision.
- Day and week durations of a placed day (F2).
- The FHIRPath battery `cross-offset-precision`, as evidence for FP-19.

### Mutations

- D1 left as it is (below the hour refused).
- D2a with the coarser value converted instead of the finer, or with UTC.
- The leap-second rule dropped.
- D2b always unknown.
- D2c dropped.
- Arithmetic dropping the default.

### Rule-5 exceptions, dependencies

None.

## 4. Adversarial round (Codex, 2026-11-04)

The first run died on a shell command and left no review. The second traced
the code; its sandbox refused the probe files, so I checked F1–F3 by tracing
`compareTemporal` and `temporal_diff` myself.

| # | Finding | Severity | Resolution |
|---|---|---|---|
| F1 | A placed day equals a bare day (the offset guard needs both values at the hour), so a bare day bridges two placed days that differ: `Equal` is not transitive, and `distinct()` depends on the order | high | accepted: D2c |
| F2 | Day and week durations change for a placed day: `@2012-01-01T` at -03:00 to `@2012-01-03T02:00Z` is 1 day, 2 unplaced; D3 said `temporal_diff` did not change | high | accepted: D3 documents and pins it; go-cql told |
| F3 | A leap second converted as the second it is held as moves to the next day: two equal values compare differently against a placed day | high | accepted: D2a converts it within its written minute |
| F4 | Two coarse values at different defaults were always unknown, even eight years apart | medium | accepted: D2b compares spans |
| F5 | The audit of readers was incomplete: `lowBoundary(8)` gives a Date; the sort key carries presence only; arithmetic keeps the default | medium | accepted: D3 |
| F6 | `Equal` is boolean and cannot give null; expected values derived from A at other offsets do not test A; mutations missing | medium | accepted: `EqualTemporal`, references at four offsets, mutations listed |
| F7 | No FHIRPath expression without a default changes its answer, as long as the guards hold | low | confirms the plan; the battery and catalog runs stay in the exit criterion |

## 5. Implementation, validation and mutations

Tests and implementation by a Sonnet subagent, on the approved plan.

- Tests first: the oracle test (`types/default_offset_oracle_test.go`)
  failed in 23 of 64 cases on main and passes in all after. Every case
  requires Java and cql-execution to agree before it compares this engine.
- D8 (`@2020-01-01T10+05:30 < @2020-01-01T04:45Z`), null in both
  references, is true here: FP-19, out of scope, pinned by
  `TestOracleHourPrecisionAcrossOffsetsIsFP19`.
- Without a default, `@2012-01-01T = @2012-01-01T01:00:00Z` and
  `@2012-01 < @2012-01-01T10:00Z` stay empty, as on main
  (`TestCoarseDateTimeWithoutADefaultStaysEmpty`).
- Mutations (`mutate020.py`, scratchpad): Q1 below the hour refused,
  Q2 the coarser value converted, Q3 both to UTC, Q4 leap rule dropped,
  Q5 spans always unknown, Q6 bare against placed dropped, Q7 arithmetic
  drops the default, Q8 leap metadata kept, Q9 span one day too long. Q9
  survived until the rows "adjacent days/months, an hour apart in UTC" were
  added; then all nine are caught. The full run over 010–020 caught 188 of
  188.

## 6. Code review

### Round 1 (Codex, 2026-11-04)

| # | Finding | Severity | Resolution |
|---|---|---|---|
| R1 | `x = 2017-01-01T02:59:60Z` equals `y = 2017-01-01T03:00:00Z`, yet a day placed at -03:00 compares unknown with x and before y, so `min()` depends on the order | high | not introduced: on main a Date `@2016-12-31` already compares unknown with `2016-12-31T23:59:60-03:00` and -1 with `2017-01-01T00:00:00-03:00`, which `Equal` finds equal (probe of 2026-11-04). D2a keeps the leap rule main has at coarse precisions (adversarial F3). Recorded as FP-20, out of scope |
| R2 | A month held as thirty days survived the tests | medium | accepted: row "adjacent months that overlap by six hours"; mutation Q10 caught |
| R3 | Spans that touch were untested (`Before` for `!After` survived) | low | accepted: row "spans that touch"; mutation Q11 caught |

Not covered by the reviewer, checked by a probe: years 1 and 9999 at ±14:00
compare without overflow (`0001-01-01` at -14:00 is after `0001-01-01T05Z`,
`9999-12-31` at +14:00 before `9999-12-31T20Z`).

### Round 2 (Codex, 2026-11-04)

R2 and R3 verified fixed: each mutation fails `go test ./types/`. No medium
or higher finding. With main's `types/temporal_compare.go` and
`types/datetime.go` overlaid, the conformance module still passes and the
root fails only in the tests that require the new behaviour. The round
converges.

## 7. Closing

- `make check`: exit 0 (2026-11-04).
- The battery `difftest/batteries/cross-offset-precision` gives this engine
  the same answers before and after the change.
- Mutations: Q1–Q11 caught; 010–020 run 188/188 before round 1, Q10–Q11
  added after it.
- Open: FP-19 (mixed precisions across written offsets), FP-20 (leap second
  at coarse precisions). Durations of a placed day with uncertainty remain
  go-cql's.
