# 023: `=` between objects answers the same in both directions (#265)

Status: merged 2026-11-07 as 81a4b67 (PR #269).

## 1. Prior review

### The defect, measured

#265 reported `{"_":{}} = {"":"x","_":{}}` true and the reverse false. A sweep
shows the defect is wider than names no resource uses. It covers 856 objects
of up to two members, with members `a`, `_a`, `b`, `""`, `_` and the values
`"x"`, `{}`, `{"id":"e"}`, `[null,"x"]`, `["x"]`, `[{"id":"e"},null]`,
`[{}]`, `null` and `[]`, compared in every pair both ways, on main at fd9597b.

| | Asymmetric pairs | With valid names only |
|---|---|---|
| `=` | 3,145 | 360 |
| `~` | 0 | 0 |

Every asymmetric pair with valid names holds an element that holds no value
(`"_a":{}` or `"_a":[{}]`) and no value beside it. `{} = {"_a":{}}` is false,
and `{"_a":{}} = {}` is true. No pair with an element that holds something,
such as `{"id":"e"}`, is asymmetric.

The sweep stops at two members, and round 1 found asymmetries beyond it. Both
were reproduced here:

- `~`: `{"_a":{},"_":{}} ~ {"_a":{},"_":{},"":"x"}` is true and the reverse
  false.
- `=` with overlapping names: for `A={"__a":{}}`, `B={"_a":{}}`, `C={}`,
  `A=B` is false and `B=A` true; `A=C` and `B=C` are true. So `=` is not
  transitive either.

Cause, in `types/json_equivalent.go`, `equalObjects`:

1. The first pass compares each name of the left object through `equalNamed`.
   `equalNamed` pairs each value with the element at its position through
   `pairedItems`, which drops every element that holds no value. Its comment
   says navigation does the same. It does not: navigation, like fhirpath.js,
   reads `"_given":[{}]` as one item (see the item counts below). `~`
   (`equivalenceItems`) already keeps such an element as an item, and drops
   it only beside a value.
2. The second pass only checks that each name of the right object is present
   on the left, as a value or an element. A name written on the right only as
   an empty element fails there, though `equalNamed` reads it as no item.
3. Keys have no single reading. `childName` takes `_x` as the element of `x`
   only when `x` is not empty, so `"_"` is a plain member. But:
   - `equalNamed`, `equivalentNamed` and both second passes build the element
     key as `"_"+name`, which for `""` is `"_"`;
   - `equalNamed(name="_a")` finds the key `_a` as the value of the name `_a`,
     while the first pass reads that same key as the element of `a`;
   - navigation's indexed lookup (`types/object.go:1300`) accepts `"_"` as the
     element of `""`, while its scan (`types/object.go:1218`) does not.

### The specification

- FHIRPath §6.1.1 (`=`): "For complex types, equality requires all child
  properties to be equal, recursively." Two values are equal or not; the
  order of the operands is not part of it.
- FHIRPath §6.1.3 (`~`): "For complex types, equivalence requires all child
  properties to be equivalent, recursively."
- FHIR R4, R4B and R5 JSON ("JSON representation of primitive elements"):
  a primitive's id and extensions go in a property named `_` plus the
  element's name. An element's name never begins with `_` and is never
  empty. This is the same in the three versions.
- CONFORMANCE.md, "An element that holds no value": an element written `{}`
  is an item of its collection (`Patient.active.count()` is 1 over
  `"_active":{}`). Only `exists()` and `empty()` answer as though it were not
  there.
- CONFORMANCE.md, the `=`/`~` row: an element beside a primitive that holds
  no value adds nothing to the value beside it.

So a child property is what navigation reads. A name written only as `{}`
holds one item, which `=` and `~` must compare as an item.

### The references, measured

These batteries run through `difftest -battery` against fhirpath.js 5.2.0
and validator 6.10.2, in R4. The files are in `$S/b265/` and go to the repo
in step 7, run in R4, R4B and R5.

Item counts. `Patient.name[i].given.count()`:

| `name[i]` | Here | fhirpath.js | Java |
|---|---|---|---|
| `{"_given":[{}]}` | 1 | 1 | error[^err] |
| `{"given":[null],"_given":[{}]}` | 1 | 1 | error |
| `{"given":["x",null],"_given":[null,{}]}` | 2 | 2 | error |
| `{"given":[null,"x"],"_given":[{},null]}` | 2 | 2 | error |
| `{"_given":[{"id":"e"}]}` | 1 | 1 | error |
| `{"given":["x"],"_given":[{}]}` | 1 | 1 | error |
| `{"_given":{}}` | 1 | 1 | error |
| `{"_given":[{},{}]}` | 2 | 2 | error |
| `{"given":[null,null],"_given":[{},null]}` | 1 | 2 | error |

[^err]: Java fails on the whole resource. Each shape must be measured in a
    file of its own in step 7.

Equality, over codings and extensions:

| Case | Here | fhirpath.js | Java |
|---|---|---|---|
| `{"system":"s","_code":{}}` = `{"system":"s"}` | true | false | false |
| the reverse | false | false | true |
| `{"system":"s","code":"c"}` = the same with `"_code":{}` | true | false | true |
| the reverse | true | false | true |
| `{"url":"u","_":{}}` = `{"url":"u","":"x","_":{}}` | true | false | not a boolean[^java] |
| the reverse | false | false | not a boolean |
| `code.coding.distinct().count()`, 6 codings | 4 | 6 | 3 |

[^java]: Java answers `extension=Extension[1 children]` for every `=`
    between those extensions. Its parser does not read the member `""`.

How the references read:

- **Navigation:** this engine's navigation and fhirpath.js count items the
  same way in 8 of the 9 shapes. They differ only on a null value beside a
  null element.
- **An element that holds no value, against its absence:** fhirpath.js
  answers false both ways. Java answers false one way and true the other, so
  it is not symmetric itself. False both ways is what the specification text
  and this engine's navigation give (one item against none).
- **An element that holds no value, beside a value:** fhirpath.js answers
  false, Java true. They disagree, so rule 1 does not apply. The engine keeps
  its documented reading, under which the element adds nothing, and the
  answer stays true.
- **The names `""` and `"_"`:** fhirpath.js answers false both ways, and Java
  has no answer.

### `gofhir/server`

It delegates to this library (v1.14.0), so there is nothing to take.

### Pieces

| Piece | Decision |
|---|---|
| `equivalenceItems`' item rule (an element is an item unless it holds no value beside a value) | **Keep**, and make it the only rule |
| `pairedItems` (drops every element that holds no value) | **Discard**: its claim to follow navigation is wrong |
| the per-name comparisons (`equalNamed`, `equivalentNamed`) | **Keep** the pairing: positional for `=`, in any order for `~` |
| the two-pass loops of `equalObjects` and `equivalentWithElements` | **Improve**: compare over the union of names that one reader gives |
| the element key `"_"+name` | **Improve**: one rule, in a single helper used by equality and by navigation |
| the fast paths (`sameJSON`, `orderedEqualFromDifference`, objects without elements) | **Keep**, unchanged |
| 022's "no key for an object with a member `""` or `"_"`" | **Discard**: the key is rewritten to the same reader |

## 2. Plan (revised after adversarial round 1)

### One reading of an object's keys

A key `_x` with `x` not empty is the element of the name `x`, and never the
value of a name `_x`. Every other key, `""` and `"_"` among them, is the value
of a name of its own. This is `childName`'s rule, and it holds for any input:

- `{"_a":{}}` has the name `a`, with an element;
- `{"__a":{}}` has the name `_a`, with an element;
- `{"_":{}}` has the name `_`, with a value.

A single helper, `elementOf(key) (name string, ok bool)`, carries the rule.
It is used by every reader of equality, and by nothing else:

- the parse and its prefilter (`parseJSONObject`, `addsNothing`). The value
  beside an element `__a` is looked up as the value of the name `_a`, which
  no key provides, so `{"_a":{},"__a":{}}` keeps both elements;
- `=` and `~`;
- the equality key (`keyWriter.children`), which skips elements exactly where
  equality pairs them.

**Navigation is out of scope.** Applying the rule there changes answers on
invalid input that the engine reads today:
- `GetCollection("_a")` over `{"_a":{"id":"e"}}` returns the object;
- `children()` takes `{"__a":"x"}`'s `"x"`;
- a duplicate key counts twice.

Its scan and its index also disagree on `"_"` as the element of `""`
(`types/object.go:1218` against `:1300`). These are recorded as FP-23, for an
increment of their own.

### Transitivity: increment 024

023 makes `=` and `~` symmetric, which is what #265 reports. It does not make
`=` transitive. The fast paths and the reader disagree on input that 023 does
not change:

- duplicate keys (§3b, finding 1);
- numbers inside nested arrays (§3c, finding 1);
- an element that holds no value but is written with content, such as
  `"_a":{"resourceType":"X"}` against `"_a":{"resourceType":"Y"}` beside the
  same value. The negative fast path compares the bytes; the reader drops both
  elements (§3d, finding 1).

024 holds the fast paths to the reader through an oracle that disables them
at every depth. The existing helper (`types/json_equal_property_test.go:13`)
disables them only at the root. 024 runs that oracle over these shapes, and
fixes what it finds.

`~` is not transitive by design: `{"a":1.1} ~ {"a":1}` and
`{"a":1} ~ {"a":1.4}`, but not `{"a":1.1} ~ {"a":1.4}`, because decimal
equivalence rounds to the less precise operand. A Quantity compared as a
scalar child converts its units, and as an item of an array it does not
(§3d, finding 2). That is recorded as FP-24, outside both increments.

### Items

A name's items are its positions, each a value and the element at the same
position. A position is an item when its value is not null, or when its
element is not null. An element that holds no value is dropped from an item
that has a value, and kept as the item's element when there is no value.
This is `equivalenceItems`' rule today, and `=` takes it over in place of
`pairedItems`.

An item with no value and an empty element compares equal and equivalent to
another such item.

### Comparison

`equalObjects` and `equivalentWithElements` go over the union of the names of
both objects, each name once, and compare its items:

- `=` pairs items position by position;
- `~` pairs them in any order, as now.

A name present on one side only has items on that side and none on the other,
so the objects are unequal unless that side's items are empty, which no
written key produces. That makes the comparison of two objects symmetric
wherever the comparison of their children is.

Two child comparisons are directional, and objects inherit them:

- `~` between two quantities whose conversion factor is 1. FHIRPath 3.0.0
  says: "If the conversion factors are 1 (the units are equal), then choose
  the unit of the operator's left argument". So `1.5 'Cel' ~ 274.6 'K'` is true
  and the reverse false, as CONFORMANCE.md and `TestQuantityEquivalenceTie`
  hold.
- `=` between a calendar duration in years and one in months (FP-21, open).

023 does not change either of them.

The fast paths (`sameJSON`, `orderedEqualFromDifference`) and the loop for
objects without elements stay as they are.

### What changes

| Case | `=` today | after | `~` today | after |
|---|---|---|---|---|
| `{"_a":{}}` against `{}` | true one way | false both | false | false |
| `{"a":"c","_a":{}}` against `{"a":"c"}` | true | true | true | true |
| `{"_a":[{}]}` against `{"_a":{}}` | true both ways (measured) | true | true both ways (measured) | true |
| `{"a":null}` against `{"_a":{}}` | true one way | false both | false both ways (measured) | false |
| `{"_":{}}` against `{"":"x","_":{}}` | true one way | false both | false both ways (measured) | false |
| `{"_a":{},"_":{}}` against the same with `"":"x"` | true one way (measured) | false both | true one way | false both |
| `A={"__a":{}}`, `B={"_a":{}}`, `C={}` | asymmetric, not transitive (measured: `A=B` false and `B=A` true, `A=C` true and `C=A` false) | all pairs false | false both ways for every pair (measured) | all pairs false |
| 6 codings, `distinct().count()` | 4 | 5: `[0]` and `[1]` still one, while `[2]` (`code` with an empty element), `[3]` and `[5]` (`display` with one) are now distinct | | |

The cells that said "—" were measured before the change, in step 6, on
fd9597b, and are marked as measured.

### Exit criteria

1. `TestObjectEqualityIsSymmetric` (types) checks that comparing objects adds
   no direction of its own. Over every generated pair of objects, it requires
   `a.Equal(b) == b.Equal(a)` and the same for `Equivalent`.
   - The objects are generated up to three members from the keys `a`, `_a`,
     `__a`, `b`, `""` and `_`. The values are the §1 values plus
     `{"resourceType":"X"}` and FHIR Quantities. Each object is also nested
     one level, and with its keys reversed.
   - Only child values whose own comparisons are symmetric are drawn: the
     test checks every value pair in both directions first, and leaves out
     any that is not, by `=` and by `~`. It logs how many it left out and
     why.
   - Today the two-member set alone gives 3,145 violations.
   - The directions objects inherit from children (FP-21, FP-25, FP-26, the
     quantity tie) are pinned by cases of their own. Transitivity is 024's
     criterion.
2. Hand-written cases have their expected values from §1's text and
   measurements, not from running the code:
   - each row of "What changes";
   - pairs that must not change: an element with content against none
     (false), an element beside a value against the value alone (true),
     `"_a":[null,{}]` against `"a"` missing (false);
   - duplicate keys, through both the fast paths and the read path.
   Each case runs in R4, R4B and R5, with its members in reverse order too.
3. Equality agrees with navigation where navigation is defined by FHIR: for
   every generated object whose keys are valid FHIR names (no leading `__`,
   no `""` or `"_"`, no duplicate), and for every name, the number of items
   `=` compares equals what navigation counts for that name.
4. Keys: for every pair of the generated set with `a.Equal(b)` and both
   keyed, the keys are the same. This is checked deterministically, and the
   key's special case is removed. `FuzzEqualityKeyAgreesWithEqual` runs 5
   minutes clean, and the 022 seed now gets a key.
5. A battery, `difftest/batteries/object-equality-symmetry/`, holds the
   6-coding fixture of §1, committed with its expected equivalence classes,
   and every
   case of criterion 2 and the item-count shapes, one shape per resource
   file, in R4, R4B and R5. Each difference from a reference is classified in
   its README.
6. `make corpusdiff BASE=origin/main`: every difference is explained, or
   there are none.
7. Performance:
   - a benchmark of `=` and `~` over objects without elements, equal and
     unequal, and with elements, run against origin/main with `-count=10`:
     no change beyond the spread for objects without elements;
   - BenchmarkCorpus shows no change.
8. CONFORMANCE.md's `=`/`~` row states the item rule and the key rule.
   `docs/upstream-findings.md` FP-22 is closed.
9. `make check` is green.

### Mutations

| Mutant | What must catch it |
|---|---|
| S1: the second pass refuses a name the left lacks, as today | symmetry test |
| S2: `""` has the element `"_"` again, in equality | symmetry test |
| S3: `_a` is read as the value of the name `_a` as well | symmetry test (the `__a` chain) |
| S4: an element that holds no value is dropped from an item with no value (`pairedItems`' rule) | navigation-agreement test, hand-written cases |
| S5: an element that holds no value is kept beside a value | hand-written cases (element beside a value) |
| S6: `~` pairs positionally | existing `~` tests |
| S8: the key skips `"_"` as an element | a key-content test: `{}` and `{"_":{}}` get different keys. Coarser keys only cost time, so criterion 4 alone cannot catch it |
| S10: `addsNothing` looks up `_a` as the value beside `__a` | hand-written case `{"_a":{},"__a":{}}` against `{"_a":{}}` |
| S9: the union of names misses names only on the right | symmetry test |

The full 010–022 set is rerun before the code review.

### ADR exceptions, dependencies

None. No FHIR name is written into the code, and no dependency is added.

## 3. Adversarial round 1 (Codex, 2026-11-06)

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | The direction contradicts the documented reading: `"_active":{}` counts 1, so an empty element is an item, and false both ways follows | high | **Accepted**: measured. `coding[2].children().count()` is 2 against 1, and navigation matches fhirpath.js in 8 of 9 shapes. The direction is now false both ways, and `pairedItems`' rule is discarded |
| 2 | `A={"__a":{}}`, `B={"_a":{}}`, `C={}`: still asymmetric after the proposed fix, and not transitive | high | **Accepted**: reproduced (`A=B` false, `B=A` true, `A=C` and `B=C` true). One key rule, and a comparison over the union of names; transitivity is added to criterion 1 |
| 3 | "`~` is symmetric" was a measurement artifact: `{"_a":{},"_":{}}` ~ the same with `"":"x"` | medium | **Accepted**: reproduced. `~` is in scope, and the sweep goes to three members |
| 4 | Navigation reads `"_"` as the element of `""` in its index but not in its scan | medium | **Accepted**: the code differs (`types/object.go:1218` against `:1300`). A probe with 21 members did not show it, so the step 5 test must build an indexed object. The helper is shared |
| 5 | Missing oracles (transitivity, underscore chains, duplicate keys through the fast paths, null and element alignment), S3 wrongly called equivalent, and key removal resting on one seed | medium | **Accepted**: criteria 1 to 4 and the mutation table were rewritten |
| 6 | Expectations stated for three versions, measured in one | medium | **Accepted**: the battery runs in R4, R4B and R5 (criterion 5) |
| 7 | "Same cost" unsupported | low | **Accepted**: criterion 7 adds a benchmark with a threshold |

## 3b. Adversarial round 2 (Codex, 2026-11-06)

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | The negative fast path breaks transitivity over duplicate keys | high | **Accepted**: reproduced. `{"a":"x","a":"y"}` and `{"a":"x","a":"z"}` each equal `{"a":"x"}` but not each other, and reordered they are equal. Moved to 024 by the split (§3d) |
| 2 | `addsNothing` removes `__a` beside `_a`, against the exclusive key rule | high | **Accepted**: reproduced (`{"_a":{},"__a":{}}` = `{"_a":{}}` is true). The parse and its prefilter take the same rule; S10 |
| 3 | Navigation's scope and its oracle are underspecified: the rule would change `GetCollection("_a")`, `children()` over `__a`, and duplicate counts | medium | **Accepted**: navigation is out of scope and goes to FP-23. Criterion 3 holds only over valid FHIR names |
| 4 | S8 cannot be caught by criterion 4 | medium | **Accepted**: a key-content test |
| — | The 6-coding count of 5 was not checkable | — | The fixture and its classes go into the battery (criterion 5) |

## 3c. Adversarial round 3 (Codex, 2026-11-06): did not converge

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | The positive fast path normalizes numbers inside nested arrays, while the reader compares nested arrays as written (`sameArray`), so `=` is not transitive | high | **Confirmed**: `A={"a":[[1]],"b":0}`, `B={"a":[[1.0]],"b":0}`, `C={"b":0,"a":[[1.0]]}` give `A=B` true, `B=C` true, `A=C` false. Open: the round limit was reached, so the decision went to the user |
| — | The round-2 resolutions (the duplicate-key guard, the key rule in the parse and in `addsNothing`, navigation out of scope, S8) | — | Judged sound. The duplicate-key guard needs the prefix parsed into scopes, ancestors included, since `orderedEqualFromDifference` passes only the suffix today |

Each round found another place where two readers of one JSON text
disagree:

1. the second pass of `equalObjects`;
2. `pairedItems` against navigation;
3. the negative fast path over duplicate keys;
4. `addsNothing`;
5. the positive fast path over nested arrays.

Object equality has some five readers that must agree, and nothing holds
them to one reference.

## 3d. Review of the split plan

The user chose to split the plan (2026-11-06). 023 keeps the key rule, the
item rule and the comparison over the union of names. 024 takes the fast
paths.

### Round 1 (Codex, 2026-11-06)

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | `=` is still not transitive: the negative fast path rejects `"_a":{"resourceType":"X"}` against `"Y"` beside one value, which the reader drops | high | **Confirmed**: `A=C` true, `B=C` true, `A=B` false. It is a disagreement between a fast path and the reader, so it goes to 024. 023 no longer claims transitivity |
| 2 | `~` with a Quantity as a scalar child against one as an array item: `A~B` true, `B~C` true, `A~C` false | high | **Confirmed**. `~` is not transitive by design (decimals round), and this case is recorded as FP-24, out of scope |
| 3 | The oracles do not reach these shapes, and "the reader alone" helper disables the fast paths only at the root | medium | **Accepted**: criterion 1's generator adds both shapes, for symmetry. The recursive oracle is 024's, and criterion 3b moves there |

### Round 2 (Codex, 2026-11-06)

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | `~` stays asymmetric through Quantity children: `{"q":{"value":1.5,"code":"Cel"}}` ~ `{"q":{"value":274.6,"code":"K"}}` is true and the reverse false | high | **Accepted as a correction of the claim; the behavior is kept**. This is the specification's direction ("choose the unit of the operator's left argument"), which CONFORMANCE.md documents and `TestQuantityEquivalenceTie` holds. The claim is now symmetry wherever the children's comparison is symmetric. The generator leaves out tie conversions and calendar durations (FP-21), and a separate case pins the inherited direction |
| — | `=`: no asymmetry found. `sameJSON` and `orderedEqualFromDifference` are symmetric in their arguments, and moving transitivity to 024 is sound | — | — |

### Round 3 (Codex, 2026-11-06): did not converge

The two directions the plan names are not the only directional child
comparisons. The object comparison adds no direction of its own, but these
carry through it:

| # | Finding | Severity | Verified |
|---|---|---|---|
| 1 | `~` between a number and a quantity object dispatches one way: `Integer.Equivalent` takes the object as a quantity, while `ObjectValue.Equivalent` refuses any operand that is not an object | medium | **Confirmed**: `{"q":1} ~ {"q":{"value":1,"code":"1"}}` is true and the reverse false. Recorded as FP-25 |
| 2 | Quantity `~` keeps the left unit whenever there is no conversion factor and no constant step, not only on a tie | medium | **Confirmed** at operator level: `1 'B' ~ 14 '1'` is true and the reverse false. Recorded as FP-26 |
| 3 | Quantity `=` is directional beyond years and months: `0.0166666666666667 minute = 1 second` | medium | **Not reproduced at operator level**: `0.0166666666666667 'min' = 1 's'` is false both ways. The method-level case (rounding to zero at 34 places) was not checked |

The split plan stopped at its round limit too, and the decision went to the
user.

## 4. Approval (user, 2026-11-06)

Approved with a structural claim. Comparing two objects adds no direction
of its own. Directions inherited from children stay as their own findings:
FP-21, FP-25, FP-26, and the quantity tie the text mandates.

## 5. Implementation (2026-11-06)

### What was done

- `types/json_equivalent.go`:
  - `elementOf(key)` is the key rule: `_x` with `x` not empty is the element of
    `x`; every other key is a member. `valueOf` and `elementFor` give a
    name's value and element from it (a name that is itself an element's key
    has no value; the empty name has no element). The parse and `addsNothing`
    (S10), `hasElements`, `=`, `~` and the key writer all use them.
  - `nameItems` (the former `equivalenceItems`) is the one item rule; `pairedItems`
    is gone. `positionalValues` and `equalOptional` stay.
  - `everyName` walks the names of both objects, each once, for `equalNamed`
    and `equivalentNamed`. A name the left does not hold is compared too, so
    the answer cannot depend on which side is asked first.
  - `equalObjects` gained the branch `equivalentObjects` already had for
    objects with no element key (same names and one child each, no lookups by
    `"_"+name`). `equivalentNamed` hands its pairing to `pairItems` (gocyclo).
  - `sameJSON`, `orderedEqualFromDifference`, `orderedEqual` and navigation
    (`types/object.go`) are untouched.
- `types/equality_index.go`: 022's `""`/`"_"` special case is gone; the key
  skips elements through `elementOf`.
- Tests: `types/object_equality_symmetry_test.go` (criteria 1 to 4 and S8),
  `conformance/object_equality_symmetry_test.go` (R4, R4B, R5), and a
  benchmark `types/object_equality_bench_test.go`. Two existing tests changed
  to the new rule: `equality_key_test.go` (`{"_":{}}` and `{"":"x"}` now have a
  key) and `set_operations_diff_test.go` (`{"_":{}}` and `{"":"x","_":{}}` are
  separate families, no longer one).
- Battery `difftest/batteries/object-equality-symmetry/`, with a README that
  classifies every difference.
- CONFORMANCE.md (`=`/`~` row), FP-22 closed.

### Before the change (fd9597b, tests written first)

| Test | Result |
|---|---|
| `TestObjectEqualityIsSymmetric` | fail: 26,084 asymmetric pairs of 2,124,891 (sweep of 2,061 objects) |
| `TestObjectEqualityCases` | fail: 12 of 30 cases (a 31st was added later) |
| `TestEqualityComparesWhatNavigationCounts` | fail: 6,214 of 9,855 objects (the `pairedItems` rule) |
| `TestEqualKeysAreEqualOverGeneratedObjects`, key-content test | fail: no key for `""` and `"_"` |
| `TestObjectEqualityKeepsTheDirectionsOfItsChildren` | pass |
| conformance: pairs, six codings (`distinct()` 4, want 5) | fail; the item-count shapes pass |

### After

All pass: `go test ./types/... ./funcs/... ./eval/... .`, `conformance/`,
`corpusdiff -short`; `golangci-lint` clean in the three modules;
`make corpusdiff BASE=origin/main` has 0 differing answers over 521,007 (R4),
308,577 (R4B) and 360,675 (R5) evaluations. `FuzzEqualityKeyAgreesWithEqual`
ran 2 minutes clean (1.39 million executions). The sweep, resized to 1,296
objects and 840,456 pairs for time (2.7 s), has 0 asymmetric pairs. The
symmetry test leaves out `1`, `{"value":1,"code":"1"}` (FP-25) and the two
tie quantities, and logs them.

Mutants (S1 to S10, run by hand): S2, S3, S4, S5, S8, S9 and S10 are caught.
S5 was not at first (the element beside a value is dropped earlier by
`addsNothing` unless the element array is partly empty), so a case with
`"_a":[{},{"id":"1"}]` beside `"a":["x","y"]` was added. S1 survives, and is
equivalent now: every written name has an item, so refusing a name the left
lacks gives the same answer as comparing it.

### Battery

264 cases (R4 and R5; `difftest` has no R4B, which `conformance/` covers):
30 agree, 10 fhirpath.js alone differs, 178 Java alone differs, 46 the three
differ. Java answers no boolean between extensions (208 cases). This engine
answers the same in R4 and R5, with the members reversed, and in both
directions. The six codings fall into the five classes of §1. Every
difference is classified in the battery's README.

### Benchmark

`COUNT=10 PKG=./types scripts/bench-compare.sh origin/main BenchmarkObjectEquality`,
load average 13.5 to 5.7 on an M4 Pro (the machine was busy with other work):

| | `=` | `~` |
|---|---|---|
| no element, equal | -27% (872 to 635 ns) | ~ (spread) |
| no element, unequal | -27% | ~ |
| with elements, equal | +23% (1.12 to 1.38 µs) | +20% |
| with elements, unequal | ~ | ~ |

Bytes and allocations are identical to main in all eight. A first version
passed the per-name comparison as a function value, which moved both parsed
objects to the heap (+2 allocations); `everyName` now takes a flag. The
element path does a few more lookups per name (the union of names), a cost I
judged acceptable: it only runs for objects that write `_x` keys. Not run:
`BenchmarkCorpus` (corpusdiff shows no answer difference; the time is
measured by `make bench-corpus`).

### Deviations

- Battery in R4 and R5 only: `difftest` accepts no R4B.
- `FuzzEqualityKeyAgreesWithEqual` ran 2 minutes, not the 5 of criterion 4,
  as the instruction said. The 5 are still due before closing.
- The symmetry sweep is smaller than "every generated pair" over three members
  (500 sampled objects of three members, all objects of two with 7 of the
  children, all of one), for run time.
- `go vet ./...` warns about unreachable code in the generated
  `parser/grammar/`, as on main.

### Mutations, verified in the main session (2026-11-06)

`mutate023.py` holds S1 to S13 (S6 and S7 were dropped with the split).
Against this increment's tests:

- **Caught (10):** S2, S3, S4, S5, S8, S9, S10, S11, S12, S13.
- **Equivalent (1):** S1, a right-only name refused outright. A name written
  on one side always has at least one item there, so `equalNamed` answers
  false, as refusing does.

Older mutants retargeted to the new code, each caught:

- 022's K4 (an element enters the key);
- 021's M3, now M3a with elements and M3b without (only the first child is
  compared).

### Code review, round 1 (Codex, 2026-11-06): converged

Source reading. No finding of medium or higher severity within the claim.
Codex found no pair whose child comparisons are symmetric and whose object
comparison is not, and no unlisted change on valid FHIR JSON.

| # | Finding | Severity | Resolution |
|---|---|---|---|
| 1 | The symmetry test's pre-filter checks children through object comparison, so it is weaker than an independent oracle. Its exclusions are logged but not asserted | low | Noted. The hand-written cases pin each inherited direction (FP-21, FP-25, FP-26, the quantity tie) on their own |
| 2 | English comments in new test files, against the repo's convention | low | **Accepted**: translated |

### Closing checks (2026-11-06)

- Full mutation set, 010 to 023: 227 caught, with no timeout. The survivors
  are the four equivalents: 022's K5, K9 and K12a, and 023's S1. 021's M2, M7
  and M8 were retired in 022.
- `make corpusdiff BASE=origin/main`: 0 answers differ over 521,007 R4,
  308,577 R4B and 360,675 R5 evaluations.
- `BenchmarkCorpus`, alternating origin/main and this tree, 6 runs each: no
  significant change in time (geomean -0.41%), B/op or allocs/op.
- `FuzzEqualityKeyAgreesWithEqual`, 5 minutes: no failure.
