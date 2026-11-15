# 024: `=` between objects is transitive

Status: closed 2026-11-14; merged 2026-11-15 (PR #271).

## 1. Prior review

### The defect, measured

`equalJSON` decides most pairs through two fast paths, and leaves the rest to
the reader (`parseJSONObject` and `equalObjects`):

- `sameJSON`: texts alike apart from layout are equal;
- `orderedEqualFromDifference`: texts whose keys agree up to their first
  difference, and that differ there in a value, are decided at that value.

A fast path that answers differently from the reader makes `=` intransitive.
Plan 023 found three such shapes (§3b, §3c, §3d there).

**Measurement.** A probe in a copy of the repo at 81a4b67 makes `equalJSON`
skip both fast paths at every depth, so nested objects take the reader too.
Over 70,175 pairs of generated objects (depth 2, values in several
writings, members reordered, duplicate keys, and elements beside values)
it compares that against `equalJSON` as it is:

| Shape | Pairs that disagree | Fast path | Reader |
|---|---|---|---|
| a nested array, `[[1]]` against `[[1.0]]` | 1,161 | true: `orderedEqual` reads numbers as decimals at every depth | false: `sameArray` compares a nested array as written |
| a duplicate key, `{"b":"b","b":10e-1}` against `{"b":"b","b":1.00e-1}` | 149 | decides at the ignored second occurrence | keeps the first that is not null |
| an element that holds no value, written with content, `"_b":{"resourceType":"X"}` against `{"resourceType":"Y"}` beside one value | 55 | false: the bytes differ | true: both elements add nothing (`holdsNoValue`, `addsNothing`) |

No other shape disagreed. None of the three shapes is valid FHIR. Nested arrays and
duplicate keys are not valid JSON for a resource, and ele-1 forbids an
element with neither a value nor children.

### The specification and the engine's reading

- FHIRPath §6.1.1: "For complex types, equality requires all child
  properties to be equal, recursively". Equality is a relation between
  values. Which path computes it cannot change the answer.
- CONFORMANCE.md, the `=`/`~` row between complex values:
  - "a number as a decimal, so `1` equals `1.0`";
  - "of a key written twice, the first that is not `null` counts";
  - "a nested array is not flattened".

  It does not say that numbers inside a nested array compare as written. The
  fast path reads them as decimals and the reader does not.
- CONFORMANCE.md, "An element that holds no value", and the row on an
  element beside a primitive: such an element "adds nothing to a value
  beside it". That is the reader's answer.

### The references

The three shapes are not valid FHIR. Java fails on them (plan 023, §1). They
are measured in step 7 anyway, and fhirpath.js's answers are recorded. The
expected values come from the engine's own documented reading above. Rule 1
cannot apply: Java gives no answer.

### What fhirpath.js and Java answer (measured 2026-11-08)

A battery in the scratchpad puts each shape in a Basic as two extensions.
Java fails on every one ("Extension[1 children]"). fhirpath.js reads the
JSON through `JSON.parse`, so the last of a duplicate key counts and a number
out of range is `Infinity`. R4 and R5 answer alike.

| Shape | `=` here | `=` js | `~` here | `~` js |
|---|---|---|---|---|
| `[[1]]` against `[[1.0]]` | true (fast path) | true | false (reader) | true |
| `"x":"a","x":"b"` against `"x":"a","x":"c"` | false (fast path) | false | true (reader) | false |
| `"_valueString":{"resourceType":"X"}` against `"Y"`, beside one value | false (fast path) | false | true (reader) | false |
| `1e2147483648`, layout changed | true (fast path) | true | true | true |

Navigation, probed on the same resource, reads a number that does not
decode as nothing: `extension[0].n` is empty, and `children()` counts only
`url`. Yet `{"url":"u","n":1e2147483648} = {"url":"u"}` is false, while the
reader reads the pair `{"n":1,"m":1e2147483648}` against
`{"n":1.0,"m":1e2147483648}` as unequal. The reader does not read that
number as navigation does. Such a number is valid FHIR, so it is left out of
024 (§2, "Out of scope").

### Removing the negative fast path is not an option (measured 2026-11-08)

Before guarding `orderedEqualFromDifference`, I measured dropping it. A copy
without it, against main, 6 runs each, alternating:

- the set-operation benchmarks on objects are 2 to 10 times slower: `distinct`
  on 31 components is +1,887%, and `union` in one bucket is +237% to +638%;
- `BenchmarkCorpus/context-valid` is +15% (p=0.041).

The fast path stays.

### Pieces

| Piece | Decision |
|---|---|
| the reader (`parseJSONObject`, `equalObjects`, `equalChildren`) | **Keep** as the reference. The fast paths must agree with it. **Improve** in two places, where it disagrees with navigation or with every reference (§2, 1 and 2) |
| `sameArray`, used for a nested array | **Improve**: items compare as `=` compares values, as fhirpath.js reads them |
| `sameJSON`, `orderedEqual`, `orderedEqualFromDifference`, and the `sameJSON`/`sameArray` shortcut in `writtenAlike` | **Improve**: trusted only on plain text (§2, 3) |
| `equalJSONByChildren`, which disables the fast paths only at the root | **Replace** by a switch that disables every shortcut at every depth |

## 2. Plan

### Scope

The rule: **a shortcut answers only where the reader reads the text as
written.** The reader stays the reference, and every shortcut is gated by
one syntactic check, instead of guards placed where it decides. The check may
send more pairs to the reader than it must. That never changes an answer,
only the time, which criterion 5 measures.

1. **A nested array compares item by item, as `=` compares values**: numbers
   as decimals, strings decoded, objects through `equalJSON`, arrays in turn,
   in order, a null as it is written. Nesting and positions are kept, so a
   nested array is still not flattened. `=` already answers this way where
   the fast path decides, and now answers it on every path:
   `{"a":[[1]],"b":0} = {"b":0,"a":[[1.0]]}` goes from false to true, while
   the same pair in one key order was already true (measured 2026-11-09). **`~` changes**:
   `{"a":[[1]]} ~ {"a":[[1.0]]}` goes from false to true, as fhirpath.js
   answers, because `writtenAlike` shares `sameArray`.
2. **Plain text.** `plainJSON(data)` reports a text where none of these
   occur, at any depth:
   - (P1) a key that starts with `_`, or that holds a `\`;
   - (P2) a key written twice in one object, compared byte for byte, which
     is exact once P1 has excluded escapes;
   - (P3) a `null`.

   `equalJSON` trusts `sameJSON` and `orderedEqual*` only when both texts
   are plain, and otherwise reads by child. `writtenAlike` trusts its
   `sameJSON`/`sameArray` shortcut on the same condition. The check runs once
   per `equalJSON` call, and a child object read by the reader is checked
   again, so a pair whose deepest object is not plain costs a scan per
   level, O(bytes × depth). Criterion 5 measures that case. If it shows a
   cost, the check reports where it failed, and a child that lies outside
   the failing part keeps its shortcuts.
3. **The test switch.** An unexported `var shortcuts = true`, set only from
   tests, turns off every shortcut of item 2, at every depth. The oracle
   checks itself with a test-only counter of shortcut entries, which must
   stay at zero under the switch over pairs that reach each shortcut: a plain
   object nested in a plain object, an array of objects, and the
   `writtenAlike` path. Gated pairs cannot prove it, since the gate already
   sends them to the reader.

Answers that change, all on JSON that is not valid FHIR (keys written twice,
an element with neither a value nor children, ele-1, a nested array). Each follows the
reading CONFORMANCE.md already documents, where the fast path contradicted
it:

| Pair | `=` before | `=` after | fhirpath.js |
|---|---|---|---|
| `{"x":"a","x":"b"}`, `{"x":"a","x":"c"}` | false | true: of a key written twice, the first that is not null counts | false, as `JSON.parse` keeps the last |
| `{"v":"a","_v":{"resourceType":"X"}}`, the same with `"Y"` | false | true: an element that holds no value adds nothing beside one | false |
| `{"a":[[1]],"b":0}`, `{"b":0,"a":[[1.0]]}` | false | true: a nested array's numbers compare as decimals, as in the other key order | true |

Each fhirpath.js difference is classified in the battery's README with the
engine's documented reading. Java gives no answer.

Out of scope:
- **a decimal that `decodeDecimal` refuses for its exponent**: the written
  exponent does not fit int32 (`1e2147483648`), or the written exponent less
  the digits after the point does not (`1.0e-2147483648`; two identical
  objects holding it are equal through `sameJSON`, measured 2026-11-09).
  It is valid FHIR, since the decimal grammar has no bound. Navigation reads
  it as nothing, while equality's shortcuts take an identical text as equal
  and its reader refuses it. Any answer changes valid FHIR, so it is decided
  apart, with FP-6 and navigation (user decision, 2026-11-09). The claim of
  024 is transitivity over texts without such a number, by both conditions;
  the generator writes none, and a test checks that each number it writes
  decodes;
- the transitivity of `~`;
- navigation (FP-23);
- the directional child comparisons (FP-21, FP-25, FP-26).

### Exit criteria

1. `TestEqualJSONFastPathsAgreeWithTheReader` (types): over the §1 generator,
   widened to depth 3 and to every value writing, and given key escapes, keys
   written twice in place (not through a map), and nulls, at least 70,000
   pairs, the
   answer of `equalJSON` equals the reader's with the fast paths off at every
   depth. Today 1,366 pairs disagree.
2. `TestObjectEqualityIsTransitive` (types): over triples drawn from families
   of mutually equal writings, with each shape of §1 among them, `a=b` and
   `b=c` imply `a=c`. The child values are pre-filtered to symmetric and
   transitive ones, as in 023, leaving out quantities and calendar units.
3. Hand-written cases, expected from §1's reading:
   - `{"a":[[1]],"b":0}`, `{"a":[[1.0]],"b":0}`, `{"b":0,"a":[[1.0]]}`: all
     equal;
   - `{"a":"x","a":"y"}` and `{"a":"x","a":"z"}`: equal, as each is
     `{"a":"x"}`;
   - `{"a":null,"a":"x"}` against `{"a":null,"a":"y"}`: unequal;
   - `{"a":"x","_a":{"resourceType":"X"}}` against the same with `"Y"`: equal;
   - shortcuts that decide positively on plain text: `{"n":1,"m":"x"}`
     against `{"n":1.0,"m":"x"}`;
   - a key written as `"\u0061"` against `"a"`; `{}`, `[]` and `null` as a
     member against nothing; an element array longer than its value array;
     nulls inside arrays;
   - `[[1]] ~ [[1.0]]`: true, and a Quantity object inside a nested array
     under `=` and `~`;
   - the same differences nested in an object and in an array of objects.
4. The existing `TestEqualJSONFastPathsAgreeWithReadingByChild` and
   `FuzzSameJSON` pass, the first moved to the new switch.
5. Performance: `BenchmarkCorpus` and the 022/023 object benchmarks (equal and
   unequal pairs, with and without elements) against origin/main,
   `-count=10`. No change beyond the spread on unequal pairs without
   duplicates or elements. That is the negative fast path's case, which now
   reads the whole text for the check. Two new benchmarks: two long Patients
   that differ late, with many keys and escaped narrative; and objects nested
   eight deep, with the keys reordered, whose deepest object has an `_x` key.
6. `make corpusdiff BASE=origin/main`: 0 differences.
7. A battery `difftest/batteries/object-equality-transitivity/`, with the
   hand-written cases, measured against fhirpath.js and Java and classified.
8. CONFORMANCE.md's row says that a nested array's items compare as values,
   and that the fast paths answer as reading by child does.
9. `make check` is green.

### Mutations

| Mutant | What must catch it |
|---|---|
| T1: `sameArray` back to comparing as written | criterion 1 and `[[1]] ~ [[1.0]]` |
| T2: P1 dropped (keys `_x`) | criterion 1, elements with content |
| T3: P1 without the escape test | the `"\u005fa"` case |
| T4: P2 dropped (keys twice) | criterion 1, duplicates |
| T5: P2 compared only in the root object | a duplicate key nested two deep |
| T6: P3 dropped (null) | `{"a":null,"a":"x"}` |
| T8: only one of the two texts checked | criterion 1 |
| T9: `writtenAlike` not gated | the nested case of finding 2 |
| T11: the switch only at the root | the counter of shortcut entries |
| T12: the check always fails, so every pair is read | equivalent for answers; criterion 5 shows the cost |

The full 010–023 set is rerun before the code review.

### ADR exceptions, dependencies

None.

## 3. Adversarial round 1 (Codex, 2026-11-08)

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | Positive decisions disagree too: `sameJSON` and a positive `orderedEqual` accept `1e2147483648`, which the reader refuses | high | **Accepted**: the gate covers positive decisions too. The number itself is left out after round 2 |
| 2 | The oracle stays biased: `writtenAlike` runs `sameJSON`/`sameArray` before descending | medium | **Accepted**: the switch covers it, with a self-check (§2, 4) |
| 3 | Guards placed where the fast path decides cannot track scope over the suffix: a difference in `n` before a key written twice | medium | **Accepted, and the design changed**: one syntactic gate over the whole text instead of guards (§2, 3) |
| 4 | Changing `sameArray` changes `~` | medium | **Accepted**: stated, and tested. It moves `~` to fhirpath.js's answer |
| 5 | The generator cannot write keys twice or escaped; no benchmark for the negative path | low | **Accepted**: criteria 1, 3 and 5 |

## 3b. Adversarial round 2 (Codex, 2026-11-08)

Codex found no disagreement left between a shortcut and the reader on plain
texts, and no asymmetry in nested arrays compared by value.

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | Reading an undecodable number as null must happen when parsing, not in `equalValues`: `{"n":1e2147483648}` against `{}` fails on the child count first | high | **Moot**: the change was dropped (finding 3) |
| 2 | The self-check uses a gated pair, so it cannot catch a switch that does nothing | medium | **Accepted**: a counter of shortcut entries over plain pairs (§2, 3) |
| 3 | `1e2147483648` is valid FHIR: the decimal grammar has no bound (json_number.go, decimal.go). Verified | medium | **Accepted**. Asked the user, who chose to leave such numbers out of 024: no answer on valid FHIR changes, and they are decided with FP-6 and navigation |
| 4 | A deep object whose innermost level is not plain is scanned once per level | low | **Accepted**: benchmarked (criterion 5), with a remedy if it costs |

## 3c. Adversarial round 3 (Codex, 2026-11-09): two text findings

Codex found no disagreement on plain text for the numbers the generator
writes (`-0`, `1E2`, `0.0e0`, `1e-2147483648`, long mantissas): both paths use
the same decoder and comparator.

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | The numeric exclusion names one of `decodeDecimal`'s two exponent checks: `1.0e-2147483648` fails the other | medium | **Accepted**, verified with the engine: "Out of scope" names both, and a test checks the generator's numbers decode |
| 2 | "The `=` answers do not change" is false: `{"a":[[1]],"b":0} = {"b":0,"a":[[1.0]]}` is false today and true after | medium | **Accepted**, verified: in §2, 1 and in the table of answers that change |

Both are corrections to the plan's text, applied without a fourth round (the
cap is three).

## 4. Approval

The user approved the plan on 2026-11-09, after the three rounds.

## 5. Implementation (2026-11-09)

Every command below runs from the repository root. Long outputs are in the
scratchpad `impl024/` of the session.

### What was done

- `types/json_equal.go`: the test switch `shortcuts` and the counter
  `shortcutEntries` (written only with the switch off, so comparing shares no
  writes); `plainJSON` (P1 to P3 in one pass, keys in a stack of ranges per
  open object, no allocation: a test counts it; an object of more than 128 keys
  is not plain); `trustsShortcuts(a, b)` = switch on and both texts plain.
- `types/json_equivalent.go`: `equalJSON` trusts `sameJSON` and
  `orderedEqualFromDifference` only through `trustsShortcuts`; `writtenAlike`
  trusts its `sameJSON` the same way; `sameArray` compares item by item as `=`
  compares values (a null equals a null and a number outside JSON's grammar, not
  `[]`); `equalValues` compares a null as a null and a Boolean as written.
- Deviation, found by `make check`: `writtenAlike` calls `sameArray` for an
  array on any JSON text, plain or not, instead of gating it. By value it is the
  reading of `=` and not a shortcut, and gating it broke
  `TestANonJSONNumberInAnArrayComparesAsNull` (#238): `[[-.5]] ~ [[null]]`
  went from true to false, because the reading of `~` does not read a number
  outside the grammar as the null it stands for. The `sameJSON` half stays gated.
- Tests: `types/object_equality_transitivity_test.go` (criteria 1 to 3, the
  self-check, `plainJSON`, the generator's numbers), `equalJSONReader` replaces
  `equalJSONByChildren`, `conformance/object_equality_transitivity_test.go`
  (R4, R4B, R5), benchmarks `BenchmarkObjectEqualityPlainCheck`.
  `conformance/resource_number_grammar_test.go`: four rows of
  `TestNestedArraysOfJSONNumbersCompareAsBefore` now expect `~` true, as §2, 1
  says.
- Battery `difftest/batteries/object-equality-transitivity/`; CONFORMANCE.md's
  `=`/`~` row (criterion 8).

### Before and after (tests first)

`GOFLAGS=-p=2 go test -count=1 -v ./types/ -run 'TestEqualJSONFastPaths|TestObjectEqualityIsTransitive|TestObjectEqualityOfTransitivityCases|TestNestedArrayEquivalence|TestSwitchReaches|TestGeneratorNumbers'`

- Before (`impl024/tests-before.txt`, switch added and shortcuts still
  ungated): 5 fail. Criterion 1: 282 of 72,000 pairs disagree; criterion 2: 16
  intransitive triples of 76,588 chains; the hand cases of criterion 3 fail for
  nested arrays (`=` and `~`), a key written twice, an element with content and
  an escaped `_`; `[[1]] ~ [[1.0]]` is false; the self-check counts 3 shortcut
  entries under the switch.
- After (`impl024/tests-after.txt`): all pass, 72,000 pairs with 0
  disagreements, 472 writings and 76,656 chains with 0 intransitive triples.
  `GOFLAGS=-p=2 go test -count=1 ./types/` takes 6 s and 123 MB peak
  (`/usr/bin/time -l`); criterion 1 takes 0.4 s.
- `make check`: green (`impl024/make-check.txt`).

### Mutations

Run by hand over the tests above: T1, T2, T3, T4, T5, T11 and T9 with the
switch ignored are caught. Three survive and are equivalent for answers:
- T6 (P3 dropped), 560,000 more generated pairs and an exhaustive sweep of
  two-member objects over 15 values with nulls: no disagreement. A `null` in a
  text that P1 and P2 leave plain never made a shortcut answer otherwise;
  `{"a":null,"a":"x"}` is not plain because of P2.
- T8 (only one text checked): the shortcuts walk the two texts token by token,
  so a pair that decides has the same keys on both sides up to the decision.
- T9 as written (the `sameJSON` of `writtenAlike` ungated, the switch kept):
  the shortcut only decides positively, on texts that are alike apart from
  layout, which the reader reads alike.

### corpusdiff

`make corpusdiff BASE=origin/main`: r4 521,007 evaluations, r4b 308,577, r5
360,675, 0 answers differ (`impl024/corpusdiff.txt`).

### Benchmarks

`COUNT=10 BENCHTIME=0.2s PKG=./types scripts/bench-compare.sh origin/main '.'`
(`impl024/bench-types.txt`), geomean of sec/op +48.9%, B/op +0.2%, allocs/op +0.4%. It ran before the last change to `writtenAlike` (the deviation above), which only touches `~` over arrays; `BenchmarkCorpus` below ran after it.
**Criterion 5 is not met.** The check reads whole texts, where the shortcuts
read to the first difference:

| Benchmark | base | head |
|---|---|---|
| `ExcludeOneFromMany` (the same bytes in memory) | 12.7 ns | 3.4 µs |
| `ExcludeEightResourcesDifferingEarly` | 390 µs | 14.0 ms |
| `DistinctResourcesDifferingEarly` | 26 ms | 68 ms |
| `IndexThreshold/object/n=32/scan` | 72 µs | 212 µs |
| `ObjectEquality/Equal/NoElementsUnequal` | 632 ns | 1.0 µs |
| `ObjectEqualityPlainCheck/LongPatientsDifferLate` | 11.4 µs | 29.7 µs |
| `ObjectEqualityPlainCheck/NestedElementKeysReordered` | 4.6 µs | 10.3 µs |

`BenchmarkCorpus` is unchanged: the compiled test binaries of origin/main and
of the branch, run alternately 10 times (`-test.bench BenchmarkCorpus
-test.benchtime 1x`, `impl024/c/`, benchstat): no sec/op change beyond the
spread (document ~, oneshot ~, context ~, context-valid ~; geomean +1.2%, B/op
+0.07%).

A variant that scans only the part of each text the negative shortcut read, and
trusts the positive shortcuts without a scan, was measured in a copy
(`impl024/protoB-vs-plan.diff`, 6 alternating runs): it passes criteria 1 and 2
and has no change on `ExcludeOneFromMany`, `DistinctResourcesDifferingEarly`,
`ObjectEquality/*` and `NestedElementKeysReordered`, but the scans of the
prefix still cost about +110% on `ContainsLarge*`, `IndexThreshold/*/scan`
and `ExcludeEightResourcesDifferingEarly`. It is not in the tree: it changes the
design of §2, 2. The decision is the user's.

### Battery

`cd difftest && go run . -battery batteries/object-equality-transitivity/battery.tsv -java ../build/validator/validator_cli-6.10.2.jar -jobs 2 -v`
(`impl024/battery.txt`): 80 cases, 0 agree, 56 Java alone differs (it answers
`extension=Extension[1 children]` for all), 24 the three differ: a key written
twice (`ot-03`, `ot-09`) and an element with content (`ot-05`), the two rows of
§2's table where fhirpath.js answers false. Classified in the battery's
README.

## 6. Revision after implementation (2026-11-14)

The plan's gate reads both texts whole, where the negative shortcut read to
the first difference. That cost +49% in geomean over the types benchmarks,
and up to 280 times on one pair. Two variants, measured 2026-11-09 against
the base, 6 alternating runs:

- the check over the prefix the negative shortcut read, in both texts: +107%
  on the worst benchmarks;
- the same prefix checked once, since it is the same bytes in both texts:
  +60%.

`BenchmarkCorpus` did not change.

**User decision (2026-11-14):** speed up the check, with a ceiling of +20% on
those benchmarks:

- `ContainsLarge*`;
- `ExcludeEightResourcesDifferingEarly`;
- `DistinctSharedPrefix*`;
- `IndexThreshold/*/scan`;
- `ObjectEqualityPlainCheck/LongPatientsDifferLate`.

If the check cannot meet that, 024 falls back to nested arrays alone. In that
case keys written twice and elements with content stay documented as known
intransitivity on invalid FHIR, pinned by tests.

Design changes, all within the claim, which covers JSON text that jsonparser reads (JP-3: a key with an unpaired surrogate escape is refused upstream), RFC 8259 only. Text that is not JSON has no defined reading, as with the out-of-scope decimals. For example `{"\q":1}` equals itself and equals `{"\q":1.0}` through the shortcuts, while the reader rejects both because jsonparser refuses an invalid key escape. Positive decisions stay ungated:

- **Only the negative decision is gated.** A positive answer from `sameJSON`
  means the two texts differ only in layout, so the reader reads them alike.
  A positive answer from `orderedEqual` means every token is equal, in place,
  as a value, so the reader also finds every child equal. Criterion 1 checks
  both claims.
- **The prefix is checked once.** It is checked up to the end of the token
  that decided, and the decisive tokens of both texts are checked too.
- **`writtenAlike` compares arrays by value on any JSON text.** Gating that
  comparison broke #238's `[[-.5]] ~ [[null]]`, which reads a number outside
  JSON's grammar as null. So only its positive `sameJSON` stays, ungated.

### Result (2026-11-14)

The ceiling is met, and the design needs no fallback. The check is faster than
the base on most of the benchmarks, because it replaces a pass the negative
shortcut already made.

**Implementation** (`types/json_equal.go`, `types/json_equivalent.go`):

- Only the negative decision of `orderedEqualFromDifference` is gated.
  `sameJSON`, a positive `orderedEqual` and `writtenAlike`'s `sameJSON` are
  trusted on any JSON text; they answer only under the test switch `shortcuts`.
  `trustsShortcuts` is gone. `shortcutEntries` still counts at every depth.
- One pass reads the bytes the two texts share (`plainScan.prefix`). It stops
  at the opening quote of a string that holds the first difference, or else
  the token start is found as before (`tokenStart`). That pass replaces
  `stringAt`, which only the prefix read: its cost was already in the base.
  Strings are skipped with `bytes.IndexByte`, the bytes outside them with a
  table, and the scan is a value, so its key stacks (96 keys, 8 open objects)
  stay on the stack and `plainJSON` still allocates nothing on the benchmark
  Patient. Keys are `int32` ranges.
- On a negative decision, `tailsPlain` continues the scan over what each text
  read after the shared bytes, up to the end of the token that decided, with
  the scope the shared bytes left open (the stacks are copied once).
- A prefix that is not plain falls back to `stringAt` to find the start, and
  the pair goes to the reader if it decided negatively.

**Speed** (6 rounds alternating, `-test.benchtime 0.2s`, benchstat; compiled
test binaries of origin/main and of the tree, `impl024/speed/benchstat.txt`):

| Benchmark | base | tree | sec/op |
|---|---|---|---|
| `ContainsLargeSameLength` | 617.6 µs | 361.0 µs | -41.6% |
| `ContainsLargeDifferentLength` | 612.2 µs | 356.2 µs | -41.8% |
| `ExcludeEightResourcesDifferingEarly` | 383.4 µs | 376.2 µs | -1.9% |
| `DistinctSharedPrefix1K` | 1.399 ms | 1.035 ms | -26.0% |
| `DistinctSharedPrefix10K` | 13.31 ms | 9.54 ms | -28.3% |
| `IndexThreshold/object/n=8/scan` | 3.98 µs | 3.33 µs | -16.3% |
| `IndexThreshold/object/n=32/scan` | 70.8 µs | 58.9 µs | -16.8% |
| `IndexThreshold/small-object/n=8/scan` | 2.67 µs | 2.75 µs | +2.9% |
| `IndexThreshold/small-object/n=32/scan` | 48.3 µs | 47.5 µs | -1.5% |
| `ObjectEqualityPlainCheck/LongPatientsDifferLate` | 11.0 µs | 9.66 µs | -12.0% |

B/op and allocs/op do not change. The other object benchmarks (`ObjectEquality`,
`ExcludeOneFromMany`, `DistinctResources*`, `IndexThreshold/*`), 4 rounds: geomean
-0.8%, nothing significantly slower. The first attempt, which scanned the
shared bytes apart from `stringAt` and zeroed two 2 KB buffers per call, was
+91% to +756% on these benchmarks; the second pass and the buffers were most
of that excess.

**Correctness.**

- `GOFLAGS=-p=2 go test -count=1 ./types/`: green. Criterion 1 and 2 hold on
  the 72,000 generated pairs and the 76,656 chains.
- `make corpusdiff BASE=origin/main`: r4 521,007, r4b 308,577, r5 360,675
  evaluations, 0 answers differ (`impl024/speed/corpusdiff.txt`).
- New case `TestNullBesideAKeyTwice`: `{"a":[null]}` against `{"a":[]}` is
  true, `{"a":null,"b":1}` against `{"a":2,"b":1}` is false, as the reader
  reads them with the switch off, and `equalJSON` agrees in both directions.
  A null against another value is left undecided by the negative shortcut, so
  P3 is not needed for these pairs. It stays in the check as a cheap guard,
  caught only by `TestPlainJSON`.
- golangci-lint v2.15.0: 0 issues. `make check`: green (`impl024/speed/make-check.txt`).

**Mutants** (`impl024/mut2.py`, `mut3.py`, in a copy at `impl024/mutcopy`):

| Mutant | Result |
|---|---|
| T1 `sameArray` as written | caught, criterion 1 |
| T2 P1 underscore dropped | caught, criterion 1 |
| T3 P1 escape dropped | caught, criterion 1 |
| T4 P2 dropped | caught, criterion 1 |
| T5 P2 only in the root object | caught, criterion 1 |
| T6 P3 dropped | caught by `TestPlainJSON` only: equivalent for answers, as measured in §5 |
| T8 prefix not checked | caught, criterion 1 |
| T8 tails not checked at all | caught, criterion 1 |
| T8 tails without the prefix's keys | caught, criterion 1 |
| T8 only one tail checked (a or b) | survives, equivalent: the tokens before the decisive one are alike, so both tails hold the same keys |
| T9 `writtenAlike` ignores the switch | caught, `TestSwitchReachesEveryShortcut` |
| T11 `equalJSON` ignores the switch | caught, `TestSwitchReachesEveryShortcut` |
| T12 the gate always fails | survives, equivalent for answers: every pair is read, and only the time changes |
| T13 the decisive token left out of the tail | survives, equivalent: a decisive token is a scalar value with no key, no null, and no duplicate to check |

T9 as written in §2 (`writtenAlike` not gated) no longer applies: its `sameJSON`
is trusted by design.

**What proved wrong in the plan.** §2, 2 said the check "runs once per
`equalJSON` call" over both whole texts, and §5 measured +49% geomean for it.
Reading to the end is not needed: a negative decision depends only on the keys
open at the decision and on what lies up to the decisive token. The cost was
two further passes, not the one pass.

## 7. Mutations, verified in the main session (2026-11-14)

`mutate024.py` holds U1 to U11. Every one is caught:
- U1 to U5 and U7 to U9 by `TestEqualJSONFastPathsAgreeWithTheReader`;
- U6 (P3 dropped) only by `TestPlainJSON`. It is equivalent for answers,
  since the negative shortcut leaves a null against another value undecided;
- U10 by `TestSwitchReachesEveryShortcut`;
- U11 by the oracle.

The full 010–024 set caught 233. The survivors are 022's K5, K9 and K12a and
023's S1, all equivalent, and 015's H22.

H22 is now equivalent. It makes a number outside JSON's grammar unlike a null
in `orderedEqual`. Such a pair always has a null in a decisive tail, so the
gate sends it to the reader.

015's mutants retargeted to the rewritten code:
- **Caught after retargeting:** H20 (`writtenAlike` skips arrays by value)
  and H23 (`tokenStart`'s backtrack).
- **Retired:** H19 and H25, because 024 removed their code. U1 covers H19.

## 8. Code review, round 1 (Codex, 2026-11-14)

Each finding was confirmed by a test that failed.

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | Positive shortcuts accept text that is not JSON. `{"\q":1}` equals itself through `sameJSON`, and `{"\q":1.0}` through `orderedEqual`. The reader refuses both. The engine loads such a resource | medium | **Accepted, as a narrower claim**: the guarantee covers JSON text that jsonparser reads (JP-3: a key with an unpaired surrogate escape is refused upstream), RFC 8259, the same treatment as the refused decimals. Positive decisions stay ungated, since gating them costs. A test pins today's answers |
| 2 | `plainJSON` claims to reject text that is not JSON, and it does not. Production code uses it only in tests. `tokenStart` has a broken doc comment | low | **Accepted**: moved to the test file, doc corrected |
| 3 | The escaped-key cases hold no escapes | low | **Accepted**: real `\u` escapes |

## 9. Code review, round 2 (Codex, 2026-11-14)

| # | Finding | Severity | Resolution |
|---|---|---|---|
| - | The four round-1 fixes | - | Confirmed |
| 1 | An RFC 8259 lone-surrogate key (`{"\uDEAD":1}`), refused by jsonparser. The reader answers false and the shortcuts true | medium | Confirmed by a failing test and resolved as JP-3: documented, pinned by `TestALoneSurrogateKeyIsOutsideTheClaim`, no workaround |

## 10. Code review, round 3 (Codex, 2026-11-14): converged

The round found no defect of medium or higher severity within the claim, which
covers JSON text that jsonparser reads, without decimals that `decodeDecimal`
refuses. A backslash in a key leaves a negative decision to the reader. That
covers keys equal only once decoded, `\/`, `\u0000` and surrogate pairs.

Three wording findings of low severity were fixed:
- the CONFORMANCE row said both fast paths answer only on plain text, while
  positive answers stand on any text (§6);
- the row needed to call a lone-surrogate key JSON that jsonparser refuses,
  not "not JSON";
- one test comment needed the jsonparser restriction.

Before this round the full mutation set was not run again. The change since
round 2 was a test and documentation only, and a test added cannot let a
mutant through.
