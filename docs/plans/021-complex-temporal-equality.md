# 021 — `=` between complex values with date/time children (#240)

Status: closed (step 9), 2026-11-05.

## 1. Prior review

**Issue.** #240 says `=` between two complex values compares their date/time
children as text. As a result `|` and `distinct()` keep components that N1
§6.1.1 calls equal: the same instant at another offset, or `.000` milliseconds.

**Spec.**
- N1 §6.1.1: "For complex types, equality requires all child properties to be
  equal, recursively." For DateTime, seconds and milliseconds are one precision,
  and values with offsets compare as instants.
- §6.5.1 Union: duplicates are removed by `=`.

**gofhir/server (frozen).** It has no equality of its own: it imports this
library. Nothing to take.

**This engine today.** `equalJSON` (types/json_equivalent.go) compares each
string child as it reads, without its type. CONFORMANCE.md documents this
("a string as it reads"). `~` reads a date-like string as a date by its shape,
without the model, and CONFORMANCE.md lists that as a known limit.

**References.** Battery `difftest/batteries/complex-temporal-equality`
(15 expressions × R4/R5) against fhirpath.js 5.2.0 and HL7 validator 6.10.2
(sha256 a3addadf…). 24 of 30 cases agree across all three engines. Our values
come from a probe in the scratchpad.

| Expression | here | fhirpath.js | Java |
|---|---|---|---|
| `component[0] = component[1]` (Z vs +02:00, same instant) | false | false | false |
| `component[0] = component[2]` (`.000`) | false | false | false |
| `component[0] = component[5]` (Z vs +00:00) | false | false | false |
| `(component \| component).count()` (7 components) | 7 | 7 | 7 |
| `component.distinct().count()` | 7 | 7 | 7 |
| `component[1] in component.take(1)` | false | false | false |
| `component[0].value = component[1].value` | true | true | true |
| `component[0] ~ component[1]` | true | false | true |
| `identifier[0] ~ identifier[1]` (a `string` child, date-like) | true | false | false |

**Reading.**
- Both references answer exactly what this engine answers on every `=`, `|`,
  `distinct()` and `in` case in #240.
- Each of them, like this engine, calls the children equal and the parents
  unequal. That contradicts "all child properties … equal, recursively".
- Under rule 1 (CONFORMANCE.md), both references agree, so the current answer
  stands. The contradiction is the same kind that #207 treated as a reason not
  to apply rule 1.
- Side finding: `identifier[0] ~ identifier[1]` is true here and false in both
  references. Equivalence types a `string` child as a date by its shape. This is
  the known limit in CONFORMANCE.md, now measured against both references.

## 2. Decision (user, 2026-11-05; revised after adversarial round 1)

- **Keep the references' answer, as an explicit exception to rule 1.**
  - `=` keeps comparing a complex value's date/time children as written, as
    fhirpath.js 5.2.0 and HL7's Java engine 6.10.2 do. That answer is not
    coherent: `children() = children()` is true while the parents are unequal.
    So rule 1 does not cover it (compare #207), and it is taken on purpose.
  - Why: the consumers, gofhir/server and gofhir/validator, must give the same
    `|`, `distinct()` and set results as the HL7 validator. A combo
    SearchParameter (`Observation | Observation.component`) has to index the
    same values.
  - The repo keeps no ADR folder. As in 009 and 015, the exception is a
    CONFORMANCE.md section ("A divergence taken on purpose") with the spec
    section it departs from and the test that pins it.
- **Separate issue** for the side finding: #263, `~` types a `string` child as a
  date by its shape.

## 3. Plan

**Scope.** No production code changes.

1. **CONFORMANCE.md**, a new section "A divergence taken on purpose: a complex
   value's date/time children compare as written". It gives:
   - the text it departs from, N1 §6.1.1 ("all child properties to be equal,
     recursively");
   - the measured table, from `valid.json`;
   - the incoherence, `children()` equal while the parents are not;
   - the reason: it matches the HL7 validator for the server's unions and set
     operations;
   - what it reaches: `=`, `!=`, `|`, `union`, `distinct`, `isDistinct`,
     `in`, `contains`, `intersect`, `exclude`, `subsetOf`, `supersetOf`;
   - what would make it reversible: a reference release that changes the
     answer, caught by the battery;
   - the verifying test, by name, and the battery path, as NUCLEO §2.2
     requires.

   Also: rule 1's paragraph names this exception, and the `=`/`~` row (~2548)
   links to the section.
2. **Test** `TestComplexEqualityComparesDateChildrenAsWritten`, in
   `equality_comparison_test.go`. Its expected values are the references'
   (battery rows 33–74 and F1–F3), with the R4 and R5 models.

   R4B rows reuse those values and are marked as extrapolated, because the
   battery runner does not run R4B. The justification:
   - each element the fixtures use has the same type and cardinality in
     hl7.fhir.r4.core 4.0.1, r4b.core 4.3.0 and r5.core 5.0.0: Observation
     component.code and component.value[x] (dateTime, Period), issued,
     effective[x], identifier, Identifier.value, Period.start/end,
     Element.extension, Extension.url and Extension.value[x] dateTime. This was
     checked against the StructureDefinition snapshots;
   - the engine applies one FHIRPath text to every FHIR version;
   - the rule reads alike in 2.0.0 (`spec/index.adoc:1788`, "all child
     properties to be equal, recursively") and 3.0.0
     (`input/pages/index.md:3384`, "all child elements"). It covers:
   - each `valid.json` shape: another offset, `.000`, another instant, Period,
     extension, instant against dateTime;
   - each consumer listed in item 1, in both operand directions;
   - the positive controls: child `=` and `children() =` true, and
     reordered-key objects equal;
   - fallback fixtures that defeat both fast paths (`sameJSON`, `orderedEqual`
     in types/json_equal.go):
     - F1: outer keys reordered, with extensions `[u, v]` against `[v, u]`.
       Expected false: a repeat is compared in order.
     - F2: outer keys reordered, a matching `code` first, then a
       `valueDateTime` at another offset. Expected false.
     - F3: a string with two equivalent escape spellings (`"\u0061"` against
       `"a"`), written before a date at another offset. Expected false; with
       the date at the same text, expected true.

     The expected values come from the references. F1–F3 are measured in the
     battery before the test is written.
3. **Battery** `difftest/batteries/complex-temporal-equality`, in the repo:
   - `obs.json` rows 1–32 are the first run; they include malformed values,
     kept as such;
   - `valid.json` rows 33–74 are the temporal comparisons.
4. **Close #240** with a comment that links the section and the battery.

**Out of scope.**
- Typed child equality.
- #263.
- #243: its hash can follow text and number equality.

**Exit criterion.** Each item is a command whose report is linked from §5:
- `go test -run TestComplexEqualityComparesDateChildrenAsWritten -count=1 .`
  passes on main with no production change.
- `cd difftest && go run . -battery batteries/complex-temporal-equality/battery.tsv
  -java ../build/validator/validator_cli-6.10.2.jar -jobs 2 -v`, with the
  pinned jar required (no two-engine fallback). The three engines agree on every
  `valid.json` row and on F1–F3. The `obs.json` rows stay as classified in §1.
- The mutation runner (`mutfast.py`) catches M1–M6, recording which assertion
  caught each one. The full 010–021 set runs before the review.
- `make check` is green.

**Mutations.** On a copy, each must fail the new test. Each is listed with the
assertion that is meant to catch it; the run records which one actually did.

| Mutation | Meant to be caught by |
|---|---|
| M1: `equalValues` reads a date-like string as temporal and compares it with `Equal` (#240's ask) | `component[0] = component[3]` false (reordered keys force the fallback); F2; F3 |
| M2: `Union` deduplicates with `Equivalent` | `(component.take(1) \| component.skip(3).take(1)).count()` = 2 |
| M3: `equalObjects` compares only the first child | F2 |
| M4: `equalJSON` answers true when key sets match | `component[0] = component[3]` false; F2 |
| M5: repeats compared in any order | F1 |
| M6: reordered-key objects compare unequal (fallback dropped) | `component[1] = component[3]` true; F3 with the same date |

**Spec exceptions.** One, the section in item 1 (rule 5 does not apply: no
FHIR names are hardcoded).

**Dependencies.** None.

## 4. Adversarial round 1 (Codex) and its resolution

**Second battery (2026-11-05).** Codex found that `obs.json` holds invalid
FHIR values (one without seconds, one with no offset). So a second battery uses
`valid.json`, which holds only valid values. Its 21 expressions × R4/R5 are now
rows 33–74 of `battery.tsv`, and all 42 cases agree across the three engines:

| Expression | All three |
|---|---|
| `component[0].value = component[1].value` (Z vs +02:00) | true |
| `component[0].value = component[2].value` (`.000`) | true |
| `component[0].children() = component[1].children()` | true |
| `component[0].children() = component[2].children()` | true |
| `component[0] = component[1]` | false |
| `component[1] = component[3]` (keys reordered, same text) | true |
| `component[0] = component[4]` (another instant) | false |
| `component[5] = component[6]` (Period.start at another offset) | false |
| `component[7] = component[8]` (date inside an extension) | false |
| `issued = effective.start` (instant vs dateTime) | true |
| `take(3).distinct().count()`, `isDistinct()` | 3, true |
| `intersect`, `exclude`, `subsetOf`, `supersetOf`, `contains`, `in`, `\|` | as text: 0, 1, false, false, false, false, 2 |

**Findings.**

1. **High, accepted. Rule 1 was misapplied.**
   - Rule 1 (CONFORMANCE.md line 42) follows "the same *coherent* answer".
   - #207 (CONFORMANCE.md ~2548) is a case where both references *agreed*,
     yet "rule 1 does not apply", because their answer contradicted their own
     child-by-child comparison.
   - Here the references are incoherent in the same way: `children() =
     children()` is true while the parents are unequal. By the repo's own rule
     and precedent, the text decides.
   - §2 described #207 the wrong way round. The decision goes back to the user
     with the corrected evidence.
2. **High, accepted. M1 could survive.**
   - Both fast paths in `equalJSON` (`sameJSON`, `orderedEqualFromDifference`)
     answer before `equalValues` is reached when keys stand in one order.
   - Fixtures must include reordered keys and escaped strings so that the
     comparison falls back to child-by-child reading.
3. **Medium, accepted.** The first battery lacked the positive child cases, and
   §1's `.value = .value` row was not measured then. It is now measured above:
   true in all three.
4. **Medium, accepted.** Invalid values are kept apart: `obs.json` rows are
   malformed-input cases, and `valid.json` rows are the temporal comparisons.
5. **Medium, accepted.** Nested extension, Period, instant, reordered keys and
   another instant are now measured; the coherence witness is above.
6. **Medium, accepted.** The battery runner covers only R4 and R5, so R4B has
   no reference answer. The R4B rows of the unit test will be marked as
   extrapolated: no R4B change touches these elements or FHIRPath equality.
7. **Medium, accepted.** The tests will cover every consumer, in both operand
   directions, plus mutations that ignore later children, mishandle repeats,
   and reject reordered-key equal objects.

## 4b. Adversarial round 2 (Codex)

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | M5 can survive (no reversed repeat), and M3 needs a later differing child | medium | **Accepted**: fixtures F1 and F2 |
| 2 | Each mutation needs its catching assertion named; an escaped string alone does not force the fallback | medium | **Accepted**: mutation table; F3 puts the equivalent escapes before the date |
| 3 | The R4B extrapolation was labelled but not justified | medium | **Accepted**: element types checked in the three core packages; the rule's text cited in 2.0.0 and 3.0.0 |
| 4 | The section must name its test; the exit criterion lacked commands and report links; Java must be required | medium | **Accepted**: added to §3 items 1 and the exit criterion |

All four are resolved in the plan. None of them changes the decision.

## 4c. Adversarial round 3 (Codex)

Converged: no finding of medium or higher severity. Round-2 findings were
resolved, and M1–M6 are each caught by the assertions named in §3.

## 5. Tests and mutations (2026-11-05)

**F1–F3 measured first.** `fallback.json` gives battery rows 75–94, and the
three engines agree on all 20. When the file was first written, the shell
dropped F3's `a` escape. It was restored, and the F3 rows (`component[4]`)
were measured again: 12 cases agree, with the same answers.

**Test.** `TestComplexEqualityComparesDateChildrenAsWritten` has 31 cases ×
R4, R4B (extrapolated) and R5, 93 subtests, and runs in about 0.5 s.
- **Location:** `conformance/complex_equality_test.go`, not the root
  `equality_comparison_test.go` that §3 named. The root module does not depend
  on gofhir/models; only `conformance/` does, so the deviation avoids adding a
  dependency.
- **Command:** `cd conformance && go test -run
  TestComplexEqualityComparesDateChildrenAsWritten -count=1 .`
- **Expected values:** the rows on which the three engines agree.

**CONFORMANCE.md.**
- A new section, "A divergence taken on purpose: a complex value's date/time
  children compare as written".
- A sentence in rule 1.
- A link in the `=`/`~` row.

**Mutations** (`mutate021.py`):
- With `mutfast.py`, M1–M6 were all caught. Under `-failfast`, older tests
  caught M2–M6 first.
- Run against the new test alone, each mutant failed it. The first failing
  subtests were:
  - M1: `component[0] = component[3]`;
  - M2: the union count;
  - M3 and M4: `component[0] = component[3]`;
  - M5: F1, `component[0] = component[1]`;
  - M6: `component[1] = component[3]`.
- Evidence: scratchpad `mut/result021.txt` and `mut/result021-own.txt`.
- The full 010–021 set caught all 196 mutants. Evidence: scratchpad
  `mut/result021-all.txt`.
  - **Duration:** 7215 s, against 633 s for 020. The machine was at a load
    average of 19.5 on 12 cores from other repositories' test runs
    (`attendance`, `database` and `pipeline` binaries). Targeted runs averaged
    30 s each, against 2.3 s in 020.
  - **Rerun:** six mutants were counted caught only by the 60 s cut-off: M24,
    M25 and M36 (010), H7 (015), Q5 and Q6 (020). They were run again, and each
    was caught by a failing test, the same ones as in 020.

**Full battery (exit criterion).** `battery021.txt` in the scratchpad: 94 cases.
- 88 agree across the three engines.
- The 6 that differ are all `obs.json` rows (25–30), as classified in §1:
  - `~` over a `string` child (#263);
  - `component[0] ~ component[1]`, which fhirpath.js alone answers otherwise;
  - an invalid dateTime without seconds.

## 6. Code review

**Round 1 (Codex).**
- **Finding (medium, accepted):** `!=` and `union()` were untested, and each set
  operation was tested in one direction only.
  - Fixed: 21 expressions were measured against fhirpath.js and the Java engine,
    and all 42 cases agree. They are battery rows 95–136.
  - They were added to the test with row citations, including positive controls
    in which components 1 and 3 are equal objects with keys reordered.
  - The test now has 52 cases × R4, R4B and R5, 156 subtests, and passes.
- **Mutations:** Codex's sandbox refused writes in the review copy, so it ran no
  mutations. They were run here, against the new test alone, with three more
  mutations for set operations (`mut/result021-own2.txt`):
  - M7: `intersect` by equivalence;
  - M8: `exclude` by equivalence;
  - M9: `Contains` by equivalence.

  All of M1–M9 fail the test with real assertion failures.
- **The full 010–021 set is not rerun before round 2.** The rule exists because
  a fix can mask an old mutant. The only change since the full run is new
  assertions in the new test, which cannot let a caught mutant survive.

**Round 2 (Codex).**
- **Finding (medium, accepted):** `exclude()` was still tested in one
  direction only.
- **Fix:** `component.take(1).exclude(component.skip(1).take(1)).count()` was
  measured. Both references answer `[1]`. It is battery rows 137–138, and was
  added to the test, which now has 53 cases and 159 subtests and passes.
- **Lint:** `make check`'s misspell linter flagged three Spanish words in
  comments ("componentes", "extensiones", "controles"). They were reworded.

**Round 3 (Codex): converged.** No finding of medium or higher severity.

## 7. Closing (2026-11-05)

- **`make check`:** green (exit 0). Output in scratchpad `check021.txt`.
- **Battery:** `difftest/batteries/complex-temporal-equality/battery.tsv`, 138
  rows. The 94-row run (`battery021.txt`) agrees on every `valid.json` and
  `fallback.json` row. Rows 95–138 were measured in two later runs, all
  agreeing.
- **Test:** `TestComplexEqualityComparesDateChildrenAsWritten`
  (`conformance/complex_equality_test.go`), 53 cases × R4, R4B (extrapolated)
  and R5.
- **Mutations:**
  - M1–M9 each fail the new test alone (`mut/result021-own2.txt`).
  - The full 010–021 run caught 196 of 196 (`mut/result021-all.txt`).
- **CONFORMANCE.md:**
  - a new section, "A divergence taken on purpose: a complex value's date/time
    children compare as written";
  - rule 1 names it;
  - the `=`/`~` row links to it.
- **Pending:**
  - Close #240 with a comment, after the PR.
  - #263 (typed `~`) and #243 (union cost) remain open.
