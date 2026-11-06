# 022 — Union (`|`) and `distinct()` in linear time (#243)

Status: merged 2026-11-06 as fd9597b (PR #266).

## 1. Prior review

**Issue.** `Collection.Union` and `Distinct` (`types/collection.go`) check each
item against the result with `Contains`, a linear scan with `Equal`, so they
take n² comparisons. `Intersect`, `Exclude`, `subsetOf`, `supersetOf` and
`isDistinct` work the same way.

**Spec.** FHIRPath N1 §6.5.1 (union, "eliminating any duplicate values" by `=`)
and §5.1.x (`distinct`, `isDistinct`, `subsetOf`, `supersetOf`). Every one of
these deduplicates by `=`, so a faster version must answer exactly as `=` does.
Since 021, that `=` compares a complex value's date/time children as written.

**gofhir/server (frozen).** It imports this library; nothing to take.

**fhirpath.js 5.2.0** (`src/hash-object.js`, `src/combining.js`,
`src/existence.js`). `union`, `distinct`, `intersect`, `exclude`, `subsetOf`
and `supersetOf` hash each item with `hashObject`: JSON with sorted keys,
numbers rounded to the maximum precision, a Quantity converted to UCUM base
units, and a Date as its ISO string. Here the hash *is* the equality, so the
operations are linear. **Take** the idea of a canonical key. **Improve** on it:
here the key only picks a bucket, and `Equal` still decides within the bucket,
so a key that is coarser than `=` costs time, never a wrong answer.

**HL7 Java engine.** Not read. Its answers already agree with this engine on
every union and set row of plan 021's battery.

**Measured cost** (probe in the scratchpad, `r4.FHIRPathModel()`, load average
about 6 on 12 cores):

| Expression | n=10 | n=100 | n=1,000 | n=4,000 |
|---|---|---|---|---|
| `Observation \| Observation.component` | 12 µs | 0.89 ms | 81 ms | 1.32 s |
| `Observation.component.distinct()` | 6 µs | 0.40 ms | 39 ms | 0.63 s |
| `Observation.component.code.text.distinct()` | 6 µs | 0.06 ms | 1.9 ms | 26 ms |
| `Observation.component` (navigation only) | 2 µs | 0.01 ms | 0.13 ms | 0.51 ms |

**Sizes in real data** (official examples):
- R4 has 49 arrays of 1,000 items or more: `Bundle.entry` up to 6,781,
  `ValueSet.compose.include.concept` up to 6,401, and
  `ImplementationGuide.manifest.page` up to 28,517. R5 has 3 such arrays.
- `Observation.component` has at most 7 items in R4 and R5.

**Published constraints that use set operations.** There are 560 rows in R4 and
R5 core (scratchpad `constraints-set.tsv`).
- **bdl-7** (`isDistinct()` over fullUrl and versionId): 9 ms in total over every
  R4 example.
- **dom-3** (a union of `descendants()` references): 486 ms in total over the R4
  examples, and 461 ms with `|` replaced by `combine()`. The union is about 5%
  of it; the rest is `descendants()`.

**Reading.**
- On the official examples, the n² cost is real but small.
- It becomes large only for collections of about 1,000 items or more. A
  resource with thousands of components is valid FHIR, so a server that indexes
  it with a combo SearchParameter, or validates dom-3, pays seconds per
  resource: a cost an input can impose.
- The difficulty is the key. `=` relates values across types:
  - Integer `1` and Decimal `1.0`;
  - a FHIR Quantity object and a System Quantity, with unit conversion;
  - a DateTime and the same instant at another offset;
  - `string`, `code` and `uri`, compared by text;
  - objects by `equalJSON`: keys by name, numbers as decimals, strings decoded,
    `null` and empty arrays absent, and a primitive paired with its `_element`.

  The key must satisfy: if `a = b`, then key(a) = key(b).

## 2. Decision (user, 2026-11-05)

Use one bucket key for every type, objects included. `Equal` still decides
within a bucket.

## 3. Plan

### D1. The key

A function `equalityKey(v Value) (key string, ok bool)` in `types/`. Its one
required property: **for keyed values a and b, `a.Equal(b)` implies
`key(a) == key(b)`.** A value with no key (`ok == false`) is compared with
everything, as today.

| Value | Key | Why it holds |
|---|---|---|
| `String` | `s` + its text | `String.Equal` compares text, and only with a String |
| `Boolean` | `b` + `true`/`false` | equal only to a Boolean of the same value |
| `Integer`, `Decimal` | `n` + the exact canonical decimal (sign, digits with trailing zeros removed, exponent) | they are equal to each other by value (`CompareDecimals`); exact even beyond the bound (#245) |
| `*ObjectValue` that is not a quantity (`AsQuantity` false) | `o` + a canonical reading of its JSON, as below | `ObjectValue.Equal` is `equalJSON` against an object, and false against anything else |
| `Quantity`, a quantity object, `Date`, `DateTime`, `Time`, anything else | none | they are equal across types and units (`5 '1'` = 5, `1 'g'` = `1000 'mg'`, `@2020-01-01` = a DateTime, offsets), so a key would have to mirror conversion |

**The object's canonical reading** follows `equalJSON` and may be coarser,
never finer:
- Keys sorted by name. Of a name written twice, only the occurrence the full
  reading keeps counts: the first that is not null and not empty
  (`parseJSONObject`, `types/json_equivalent.go:201`).
- Null items inside an array are dropped, as `childValues` drops them
  (`:504`). A token that is not a JSON number reads as null, and is dropped the
  same way.
- Each name that has at least one remaining value is written as the list of
  those values, in order:
  - a number as the canonical decimal; a token that is not a JSON number reads
    as null (#238);
  - a string decoded, so an escape equals its character;
  - an object recursively;
  - `true`, `false`;
  - a nested array as the single token `[]` (coarse).
- A name with only nulls, and every `_name` element, are left out (coarse).
  Pairing a primitive with its element can only split values that the coarse
  key keeps together.

The key is computed by one function used both by the index and by the tests,
from the same readers `equalJSON` uses (`parseJSONObject`, `childValues`,
`readAs`), not from a parallel parser.

### D2. The operations

`Union`, `Distinct`, `IsDistinct`, `Intersect` and `Exclude` in
`types/collection.go`, `subsetOf`/`supersetOf` in `funcs/existence.go`, and
`repeat()`'s deduplication (`containsEqual` in `eval/evaluator.go`). For
`repeat()` the index grows as the result does:
- They use an index: a map from key to the result items in that bucket, plus a
  list of the items with no key.
- An item with a key is compared with its bucket and with the list of items
  with no key. An item with no key is compared with every item.
- **Order and direction are kept, per operation.** `Equal` is not always
  symmetric: `0.0833333333333333 year = 1 month` is true, and the reverse is
  false (measured; recorded as a separate finding). So each comparison keeps
  today's receiver and argument:

  | Operation | Today | Indexed over | Comparison |
  |---|---|---|---|
  | `Union` (`\|`, `union()`) | for each item of c then other: `result.Contains(item)` | the result, as it grows | `resultItem.Equal(item)` |
  | `Distinct`, `IsDistinct` | `result.Contains(item)` | the result, as it grows | `resultItem.Equal(item)` |
  | `Intersect` | `other.Contains(item) && !result.Contains(item)` | `other`, every item kept (no dedup), and the result | `otherItem.Equal(item)`, `resultItem.Equal(item)` |
  | `Exclude` | `!other.Contains(item)`, duplicates in c kept | `other`, every item kept | `otherItem.Equal(item)` |
  | `subsetOf` | `other.Contains(item)` | `other`, every item kept | `otherItem.Equal(item)` |
  | `supersetOf` | `input.Contains(item)` | `input`, every item kept | `inputItem.Equal(item)` |
  | `repeat()` dedupe | `containsEqual(result, candidate)` | the result, as it grows | as `containsEqual` does |

  The first equal item found is the same one as today, so the result keeps the
  same representative: the same object, and the same Integer or Decimal.
  Within a bucket, and among items with no key, items are visited in insertion
  order.
- **Below a size threshold the old scan stays.** The threshold is measured,
  not guessed, so small collections (most of the corpus) pay no allocation for
  the map.

### Out of scope

- Keys for quantities and temporal values: these stay quadratic among
  themselves.
- **What "linear" means here.** The cost is n plus the comparisons within
  buckets plus the comparisons with items that have no key. Objects that
  differ only in an `_element` share a bucket (the key leaves elements out),
  so they still compare with one another. The benchmark measures that case
  too.
- The asymmetry of calendar-unit equality (`year` against `month`), a separate
  finding.
- `in`/`contains` with a single item: already linear.
- #242 (`WithMaxCollectionSize` on navigation) and #263.

### Exit criterion

1. **Same answers.** A differential test compares the new operations with the
   old ones, kept unexported in the test as the reference, over:
   - the `descendants()` of every R4, R4B and R5 official example, under
     `distinct()`, `isDistinct()`, `|` with itself and with half of itself,
     `intersect`, `exclude`, `subsetOf`, `supersetOf` and `repeat(children())`;
   - generated mixed collections: Integer and Decimal of one value; Quantity,
     both System and FHIR objects, in convertible units; DateTimes at other
     offsets; strings with escapes; objects with keys reordered, `1` against
     `1.0`, nulls, `_element`s, numbers outside JSON's grammar, nested arrays.

   Results must be identical item for item, in order: the same length; for
   primitives, the same Go type, `String()`, `Type()` and `IsFHIRPrimitive()`,
   and the same element beside them (pointer identity); for objects, the same
   object (pointer identity). Fixtures include equal primitives with different
   metadata, in both orders: a `code` and a `string` of one text, and one
   primitive with an extension element against one without. That identity shows the same representative
   was kept.

   The reference implementations are copied into the test with their own
   `containsRef`, so they cannot reach the indexed helpers.

   **Threshold.** Every generated case runs at threshold−1, threshold and
   threshold+1. Every operation runs in both operand orders.

   **Asymmetric and nontransitive groups.** Each group runs in every order:
   - `0.0833333333333333 year` and `1 month`;
   - `1`, `1.0`, `1 '1'`, and a FHIR Quantity of 1 with unit `1`;
   - a DateTime at two offsets, beside a Date;
   - a nontransitive group for membership operations:
     - A, the FHIR object `{"value":1,"unit":"g"}`;
     - B, the System Quantity `1 'g'`;
     - C, the FHIR object `{"value":1000,"unit":"mg"}`.

     `A = B` and `B = C`, but `A ≠ C`, because two objects compare as JSON. The
     group runs with `other = [A, B]` against C, above the threshold, under
     `Intersect`, `Exclude`, `subsetOf` and `supersetOf`.

   **R5.** An integer64 child in objects, against the same value as a decimal.
2. **The key's property holds.** `FuzzEqualityKeyAgreesWithEqual` holds the key
   to `Equal`. Independent random pairs are rarely equal, so the target derives
   pairs that are equal by construction from one seed object. The seed is
   transformed by:
   - reordering keys;
   - writing a number as `1`, `1.0` or `1e0`;
   - escaping a character;
   - adding a null, an empty array or a null array item;
   - duplicating a key;
   - adding an `_element` that holds no value;
   - writing a token outside JSON's grammar;
   - nesting the object.

   Each pair is checked in both orientations: whenever `a.Equal(b)`, the keys
   are equal, or one side has no key.

   The seed corpus is committed under `testdata/fuzz/`. `make check` runs the
   seeds only; the plan records a run of 5 minutes with the command used.
3. **Linear time.** A committed benchmark, `BenchmarkSetOperations`, covers
   n = 10, 100, 1,000 and 4,000, plus threshold−1, threshold and threshold+1.
   The data sets are components, strings, objects that differ only in an
   `_element` (one bucket), and quantities (no key).
   - **Comparison:** `scripts/bench-compare.sh` against origin/main, run with
     `-count=10`. The plan records the machine, the Go version, the load
     average, and time and allocations with their spread.
   - **Acceptance:**
     - at n = 4,000, `|` and `distinct()` over components at least 20 times
       faster;
     - below the threshold, no slowdown beyond the spread the comparison
       reports;
     - one-bucket objects and quantities no slower than today.
   - **Rival:** a committed node script in `difftest/` times fhirpath.js on the
     same expressions and data. Its numbers go in the plan.
4. **No regression.** `BenchmarkCorpus` and `make corpusdiff BASE=origin/main`
   show no change in answers, and time within noise.
5. `make check` is green.

### Mutations (each must fail a test)

| Mutation | Meant to be caught by |
|---|---|
| K1: the decimal key keeps trailing zeros (`1` ≠ `1.0`) | differential test (Integer and Decimal), fuzz |
| K2: the string key keeps escapes as written | differential test (escapes), fuzz |
| K3: a quantity object gets an object key | differential test: Integer 1 and Decimal 1.0 against a FHIR Quantity object of 1 with unit `1`, which `Integer.Equal` calls equal |
| K4: `_element` values enter the key | fuzz: an `_element` that holds no value beside a populated primitive (`types/json_equal_property_test.go:98`) |
| K5: a name with only nulls enters the key | differential test (nulls) |
| K6: items with a key are not compared with those without one | differential test (Integer against `Quantity 5 '1'`) |
| K7: order of first occurrence lost (map iteration) | differential test, item for item |
| K8: `Exclude` and `Intersect` build the index from the wrong side | differential test |
| K9: the threshold is ignored, so the index is always built | not a correctness change; the benchmark at n = 10 catches it |
| K10: the keys of objects are not sorted | differential test (reordered keys) |
| K11: items with no key are not compared with keyed items (the other direction from K6) | differential test (`1 '1'` first, then 1) |
| K12: the comparison direction is reversed for items with no key, where the asymmetry lives | differential test (`year` and `month`, in both orders). Reversing it within a keyed bucket is an equivalent mutant, because keyed `Equal` is symmetric |
| K13: `Exclude` deduplicates its output | differential test (duplicates in c) |
| K14: duplicate names take the last occurrence | fuzz (duplicated key) |
| K15: null array items are kept in the key | fuzz (null array item) |
| K16: the index of `other` is deduplicated | differential test: `other = [A, B]` against C, the nontransitive group |

### Spec exceptions and dependencies

None. No FHIR names are involved, and no new dependencies.

## 4. Adversarial round 1 (Codex)

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | D1 did not say which occurrence of a duplicate name counts, nor that null items in arrays are dropped | high | **Accepted**: D1 follows `parseJSONObject` and `childValues`, reusing their readers; K14 and K15 added |
| 2 | Direction matters: `0.0833333333333333 year = 1 month` true, reverse false (measured here); membership indexes must keep every item | high | **Accepted**: per-operation table of receiver and argument; the index of `other` is not deduplicated; asymmetric groups tested in every order; K12 and K16. The asymmetry is logged as a separate finding |
| 3 | "Linear" overclaims: objects differing only in an `_element` share a key | medium | **Accepted**: the claim is qualified; a one-bucket benchmark added |
| 4 | K3's detector could not kill it; K4 needed an element that `=` ignores | medium | **Accepted**: detectors revised; K11–K13 added |
| 5 | Comparing results with `=` misses a changed representative | medium | **Accepted**: identity, type and `String()` compared; reference implementations frozen |
| 6 | Generated data might stay under the threshold; random pairs are rarely equal | medium | **Accepted**: threshold±1 fixtures, equal-by-construction fuzz pairs, R5 integer64, committed seeds |
| 7 | The performance acceptance had no controls and no rival | medium | **Accepted**: `bench-compare.sh` with `-count=10`, spread and environment recorded; a fhirpath.js timing script |

## 4b. Adversarial round 2 (Codex)

No key counterexample was found, and the direction table matches the code.

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | The representative check missed FHIR type, provenance and element metadata | medium | **Accepted**: `Type()`, `IsFHIRPrimitive()` and element identity added, with fixtures |
| 2 | K12's fixture never reached a keyed bucket | medium | **Accepted**: K12 now reverses the comparison for items with no key; reversing it within a keyed bucket is an equivalent mutant |
| 3 | K16 lacked a nontransitive witness | medium | **Accepted**: the A/B/C fixture: `1 'g'` as a FHIR object, as a System Quantity, and `1000 'mg'` as an object |

## 4c. Adversarial round 3 (Codex)

Converged: the three round-2 findings are resolved, and the edits introduced
nothing of medium or higher severity.

## 5. Implementation (2026-11-05)

Not committed. Raw output of every run is in the session scratchpad
(`.../scratchpad/impl022/`): `bench-compare.txt`, `threshold-final.txt`,
`fhirpathjs.txt`, `fuzz2.txt`, `examples-final.txt`.

### Files

- `types/equality_index.go` (new): `equalityKey` / `keyWriter`, and
  `EqualityIndex` (exported: `NewEqualityIndex`, `Contains`, `AddIfAbsent`,
  `UnknownQueries`).
- `types/collection.go`: `Union`, `Distinct`, `IsDistinct`, `Intersect`,
  `Exclude` indexed; new `SubsetOf` and `SupersetOf`.
- `types/json_equivalent.go`: `parseJSONObjectInto`, so the key reads objects
  into one reused arena (`parseJSONObject` is unchanged in behavior).
- `funcs/existence.go`: `subsetOf` / `supersetOf` call the two new methods.
- `eval/evaluator.go`: `repeat()` deduplicates with an `EqualityIndex`;
  `containsEqual` is gone.
- Tests: `types/set_operations_diff_test.go` (frozen references and the
  differential tests), `types/equality_key_test.go`
  (`FuzzEqualityKeyAgreesWithEqual` and its seeds),
  `types/testdata/fuzz/FuzzEqualityKeyAgreesWithEqual/` (31 files),
  `types/equality_index_threshold_bench_test.go`,
  `corpusdiff/set_operations_examples_test.go`.
- Benchmarks: `set_operations_bench_test.go` (`BenchmarkSetOperations`),
  `difftest/set_operations_timing.js`.

### Deviations from the plan

1. **`SubsetOf` / `SupersetOf` live in `types/`** as `Collection` methods;
   `funcs/existence.go` only calls them. The index and its tests are in one
   package, and the differential test reaches them directly.
2. **`EqualityIndex` is public API.** `repeat()` is in `eval/`, so the index
   it grows has to be exported. Four names: see above.
3. **The examples test is in `corpusdiff/`, not `conformance/`.** The fetch of
   the pinned example packages (`fetch`, `versions`) lives in `corpusdiff`'s
   `package main`; `conformance/` has none. It carries its own frozen copy of
   the references (the `types/` copy cannot be imported).
4. **The index is built only when there are enough queries too**, not only
   enough items: `Exclude`, `Intersect` and `subsetOf` over a large `other`
   and a handful of items would otherwise pay one key per item of `other` to
   save a few scans. Both sizes must reach the threshold.
5. **An object with a member named `""` or `"_"` has no key.** Found by the
   fuzzer: `equalObjects` takes the member `"_"` for the element of the member
   `""`, so `{"_":{}}` = `{"":"x","_":{}}` is true and the reverse is false.
   Not a case for the key to mirror. This is an asymmetry of `=` in this
   repository's own `equalJSON`, a separate finding (not fixed here). The
   failing input is committed as a seed.
6. The key hashes (`hash/maphash`) instead of storing the key string, so a
   Bundle of large entries does not keep a copy of its content. A collision is
   a coarser key: it costs a comparison.
7. **Not done:** the 5-minute fuzz run (2 minutes, as asked), `BenchmarkCorpus`
   and `make corpusdiff BASE=origin/main` (exit criterion 4), `make check`.

### Tests (all on a quiet machine, `-count=1`)

- Before the change, on `HEAD` plus the new test files and a stub for the two
  new methods: the `types` differential tests pass, and the examples test
  passes (R4 5,300 resources, R4B 2,837, R5 2,821; 0 differences; 6 min under
  load). The references equal the code they were copied from.
- After: `go vet` on `./types ./funcs ./eval .`; `go test` on `./types/...
  ./funcs/... ./eval/... .` and on `conformance/` pass; `golangci-lint` has no
  issues in the root module and in `corpusdiff/`.
- Examples test, after: same counts, 0 differences, 78,864 + 40,560 + 36,576
  comparisons (collections above 300 items left out: 371 + 302 + 535
  resources), 100 s. The reference compares every pair, so larger collections
  would take hours.
- `types` differential tests: 420 random mixed cases (sizes 0 to 40 around the
  threshold, both orders, every operation), every family at threshold-1,
  threshold and threshold+1 with filler, every order of the four groups
  (year/month, one, DateTime/Date, A/B/C) alone and padded, and 80 cases of
  generated objects (keys reordered, nulls, elements, nested arrays, odd numbers). Items are compared by identity (`==` on the Go value, plus type,
  `String()`, `Type()`, `IsFHIRPrimitive()` and element).
- Fuzz: `go test -run='^$' -fuzz=FuzzEqualityKeyAgreesWithEqual -fuzztime=2m
  ./types` found the `""` / `"_"` case in 4 s; after the fix it ran 134 s,
  3.1 million executions, 1,126 corpus entries, no failure.
  `TestEqualityKeySeedsAreEqualPairs` shows 97 of 120 seed rewrites are equal
  by `=` and every one of those has a key.
- Mutations run and killed: K1, K2, K4, K6, K10, K13, K16. The others were not
  run.

### Threshold

`indexFrom = 32`. `BenchmarkIndexThreshold` (`types/`) times `Distinct` over
all-distinct collections with the index always on and always off; ratio =
index / scan, minimum of 6 samples, final code:

| n | integer | string | object | small object | quantity |
|---|---|---|---|---|---|
| 8 | 2.77 | 1.87 | 1.33 | 0.59 | 1.06 |
| 16 | 1.92 | 1.41 | 0.60 | 0.32 | 0.69 |
| 24 | 1.31 | 0.93 | 0.39 | 0.18 | 1.05 |
| 32 | 1.00 | 0.70 | 0.28 | 0.13 | 0.94 |
| 48 | 0.68 | 0.46 | 0.18 | 0.09 | 0.96 |
| 64 | 0.51 | 0.35 | 0.14 | 0.05 | 0.97 |

32 is the smallest size at which no shape loses. Primitives lose a few
hundred ns below it (integers at 16: 0.35 µs against 0.67 µs); objects win
from about 12. The `components` benchmark crosses over near 30 (n = 31 to 33
rows below).

### Benchmark

`COUNT=10 BENCHTIME=100ms scripts/bench-compare.sh origin/main SetOperations`.
Apple M4 Pro, 12 cores, 24 GiB, go1.27.2 darwin/arm64. `uptime` before: load
averages 8.8 / 38.8 / 42.6; after: 43 / 50 / 42. **Other processes loaded the
machine during the run** (the spreads below show it). An earlier run on a
quiet machine (load 7, spreads under 3%, threshold 24, key without the arena)
showed the same picture and is kept as `bench-compare-run2-threshold24.txt`.

| Benchmark | n | before | after | change | B/op before -> after |
|---|---|---|---|---|---|
| components, `Observation \| Observation.component` | 10 | 34.2 µs ±26% | 34.5 µs ±18% | ~ | 12.8 KiB -> 12.6 KiB |
| | 4000 | 4.33 s ±21% | 33.0 ms ±2% | -99.24% (131x) | 4.73 MiB -> 3.94 MiB |
| components, `.component.distinct()` | 10 | 10.7 µs ±15% | 10.9 µs ±7% | ~ | 3.81 KiB -> 3.81 KiB |
| | 4000 | 1.13 s ±23% | 4.1 ms ±23% | -99.63% (274x) | 1.25 MiB -> 1.49 MiB |
| strings, `\|` | 10 | 10.0 µs ±40% | 9.8 µs ±48% | ~ | 14.4 KiB -> 14.0 KiB |
| | 4000 | 50.1 ms ±18% | 3.9 ms ±63% | -92.3% | 5.44 MiB -> 5.72 MiB |
| strings, `.distinct()` | 10 | 5.1 µs | 5.3 µs | ~ | 7.17 KiB -> 7.17 KiB |
| | 4000 | 25.4 ms ±34% | 1.8 ms ±55% | -93.0% | 2.62 MiB -> 2.86 MiB |

Acceptance:
- **20x at n = 4000 for `|` and `distinct()` over components: met** (131x and
  274x). `exclude` and `subsetOf` over components: 149x and 150x.
- **No slowdown below the threshold: met** at n = 10 for all 8 rows (`~`).
  Around the threshold the index costs components up to 26% (`distinct`
  +26% at n = 32 and +21% at 33; `union` +17% at 31 and +12% at 32), while
  `exclude` and `subsetOf` gain 35 to 42% from n = 32 to 33; strings stay
  within the spread.
- **One-bucket objects and quantities no slower: not cleanly met.**
  One-bucket `distinct()`: +10.7% at n = 10 (spread ±23%), +6.7% at 100, `~` at
  1,000 and 4,000; `union` is 31 to 51% faster. Quantity objects (no key):
  `~` in 5 of 8 rows, +5 to 9% in the others, in this run and in the quiet one.
  The cost is one key attempt per item (a parse for one-bucket objects, the
  quantity test for quantity objects) on top of the same comparisons.
- Allocations: the key reads objects into a reused arena, which cut the
  extra bytes for components from +400 to +900% to +20% at most (distinct,
  n = 4000), with the same count of allocations; `union` over components
  allocates 17% fewer bytes and 49% fewer objects.

### fhirpath.js 5.2.0

`node difftest/set_operations_timing.js 200` (compiled once, median of up to
200 runs, resource already parsed; this engine's benchmark parses the JSON on
each run, a cost linear in n). Same expressions and data as the benchmark.
node v24.22.0, same machine, load 15 to 40. Milliseconds:

| Expression | n=10 | n=100 | n=1000 | n=4000 |
|---|---|---|---|---|
| components `\|` | 0.057 | 0.47 | 4.07 | 37.6 |
| components `distinct()` | 0.013 | 0.12 | 5.16 | 264.7 |
| components `exclude` | 0.084 | 0.54 | 91.8 | 203.6 |
| components `subsetOf` | 0.062 | 0.45 | 39.7 | 294.5 |
| strings `\|` | 0.034 | 9.09 | 525 | 4,460 |
| strings `distinct()` | 0.034 | 0.24 | 35.2 | 2,132 |
| strings `exclude` | 0.024 | 0.18 | 25.4 | 1,235 |
| strings `subsetOf` | 0.011 | 0.17 | 78.1 | 1,106 |

fhirpath.js is not linear either at these sizes: its hash is fast but it
grows faster than n between 1,000 and 4,000 on strings (about 60x for 4x the
items on `distinct()`). This engine, after: components `\|` 33 ms,
`distinct()` 4.1 ms; strings `\|` 3.9 ms, `distinct()` 1.8 ms at n = 4,000.

### Decisions after implementation (user, 2026-11-05)

- `types.EqualityIndex` stays public and documented: `repeat()` in `eval/`
  needs it, and keeping it in `types/` avoids an import cycle.
- The "one-bucket objects and quantities no slower" criterion is amended to
  "at most about 10% slower on cases that are already quadratic", as measured
  above. The cost is one key attempt per item.
- The `=` asymmetry between an object with a member `"_"` and one with a
  member `""` (`{"_":{}}` = `{"":"x","_":{}}` true, the reverse false) is
  already on main. It went to its own issue, #265. 022 gives such objects no key,
  and a fuzz seed covers that.

### Mutations (step 7, 2026-11-05)

`mutate022.py` holds K1 to K22, applied one at a time to a copy of the repo:
first against this increment's tests, then with the whole 010 to 022 set.

- **Caught by this increment's tests (19):** K1, K2, K3, K4, K6, K7, K8, K10,
  K11, K12b, K13, K15, K16a, K16b, K17, K18, K20, K21, K22. K18 (bucket
  compared with `~`) first survived. It is now caught because a family holds
  two objects with one key that differ by `=` and agree by `~`
  (`"_given":[{"id":"x"}]` against `"id":"X"`).
- **Caught by an older test:** K19, the dedupe dropped from `repeat()`, by
  `TestRepeatAllKeepsDuplicates`.
- **Equivalent (3), each with its reason:**
  - K5: a name holding only nulls is dropped by `parseJSONObject` under the
    same `readAs` rule, so `values == 0` is never reached.
  - K9: the index built at every size. With it forced on, the full root and
    `conformance` suites pass, so the indexed path answers as the scan does.
  - K12a: the direction reversed against the items with no key. `Equal`
    between a keyed and an unkeyed value is symmetric by construction:
    `Integer.Equal`, `Decimal.Equal` and `ObjectValue.Equal` delegate to the
    quantity. The one asymmetry, between calendar units, is between quantities,
    which always take the scan, where K12b is caught.
- **K14** (duplicate names taking the last occurrence) would mutate
  `parseJSONObject`, which the key shares with `equalJSON`. The earlier sets
  cover it.
- **Full set 010 to 022:** 216 caught, plus the 3 equivalents.
  - 021's M2, M7 and M8 (union, intersect and exclude by `~`) no longer
    find their text, because those operations go through the index. K17 and
    K18 cover the same defect there.
  - M13 and M23 were first caught by a timeout under machine load. Rerun,
    M13 is caught by
    `TestEveryFunctionAnswersWithoutWritingADecimalBeyondTheBoundOut`. M23 is
    caught by the same test, run directly, in 5 s.

### Corpus (2026-11-05)

- `make corpusdiff BASE=origin/main`: 0 answers differ over 521,007 R4,
  308,577 R4B and 360,675 R5 evaluations.
- `BenchmarkCorpus`, alternating origin/main and this tree, 5 runs each (load
  about 5): the times show no significant difference (geomean -0.32%). B/op is
  +0.05% to +0.14% and allocs/op +0.01%.

### Code review, round 1 (Codex, 2026-11-05)

Codex's sandbox could not write to the copy or initialize a Go build cache,
so the round was source reading only: no tests or mutants ran. The
mutations above were run in the main session.

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | `NewEqualityIndex` keeps the caller's slice. Changing an item afterwards leaves it in the bucket of its old key | medium (hypothesis) | **Accepted, by narrowing the API**: `NewEqualityIndex(queries)` now builds an empty index, filled through `AddIfAbsent`. Its one caller, `repeat()`, passed nil. Indexes over caller data are only built inside `types`, from `init`, whose comment states that the items must not change |
| 2 | English comments in new files (`corpusdiff/set_operations_examples_test.go`, `types/set_operations_diff_test.go:314`, `difftest/set_operations_timing.js`) | low | **Accepted**: translated. `types/json_equivalent.go:205` is left in English, as that file's comments all are |
| 3 | No verdict on keys, order, direction, activation inside `AddIfAbsent`, or arena reuse | — | Covered by the mutations above (K6 to K13, K16 to K18), the indexed path forced on across the full suites (K9), and the fuzz |

### Fuzz, 5 minutes (2026-11-05)

`go test -run '^$' -fuzz '^FuzzEqualityKeyAgreesWithEqual$' -fuzztime 5m
-parallel 4 ./types/`: 6.0M executions, no failure.

### Code review, round 2 (Codex, 2026-11-06)

Source reading, with each suspected defect handed over as a Go test that was
run here. No defect was found in the keys, the arena or activation for inputs
that do not change.

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | `AddIfAbsent` keeps `*ObjectValue` pointers. Writing into `ObjectValue.Data()` after an object is stored leaves it in its old key's bucket | medium (reported) | **Confirmed, resolved by contract**: the reviewer's test gives `scan=true index=false` once the bytes are changed. The engine never writes into `Data()`, and caching objects already keep what they read from it. Rather than a defensive copy, `ObjectValue.Data` now states that the bytes are shared and must not be modified, and `EqualityIndex` states that stored values must not change |

### Code review, round 3 (Codex, 2026-11-06): converged

Source reading. No finding of medium or higher severity. Codex judged the
resolutions of R1.1, R1.2 and R2.1 sound, with the contract stated in the
public documentation. Its review of activation, arena reuse,
`evaluateRepeat`, order, duplicates, direction and the keys found nothing.
