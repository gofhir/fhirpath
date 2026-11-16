# 025: `~` types a complex value's children with the model (#263)

Status: step 8 (code review round 1), 2026-11-16.

## 1. Prior review

### The defect

`~` between two complex values reads each child by its shape: a string that
looks like a date is read as a date, whatever its type. With a model,
navigation reads the same child as the model types it. So with a model the two
readings disagree:

```json
{"resourceType":"Observation","status":"final","code":{"text":"x"},
 "identifier":[{"value":"2020-01-01T10:00:00Z"},{"value":"2020-01-01T12:00:00+02:00"}]}
```

| Expression (R4 model) | here | fhirpath.js 5.2.0 | HL7 validator 6.10.2 |
|---|---|---|---|
| `identifier[0] ~ identifier[1]` | true | false | false |
| `identifier[0].value ~ identifier[1].value` | false | false | false |

The references are from the battery of plan 021,
`difftest/batteries/complex-temporal-equality`.

### Spec

- FHIRPath N1 §6.1.3 Equivalent (`~`): "For complex types, equivalence
  requires all child properties to be equivalent, recursively."
- String equivalence compares text (case and whitespace aside), not instants.
- `Identifier.value` is a `string` in R4, R4B and R5. What a child is comes
  from its type: FHIR R4/R5 fhirpath.html, "the FHIR types are used".

### gofhir/server (frozen)

It has no equivalence of its own and imports this library (v1.14.0). There is
nothing to take.

### This engine today, measured

Probe: `scratchpad/p263`, R4 model against no model, at 579a581.

| Expression | model | no model |
|---|---|---|
| `identifier[0] ~ identifier[1]` | true | true |
| `identifier[0].value ~ identifier[1].value` | false | true |
| `identifier[0].value.type().name` | string | DateTime |
| `extension[0] ~ extension[1]` (`valueString`, same instant) | true | true |
| `extension[0].value ~ extension[1].value` | false | false |
| `extension[0].valueString ~ extension[1].valueString` | false | true |
| `extension[0].children().type().name` | uri, string | String, DateTime |
| `identifier[0].period ~ identifier[1].period` (`dateTime`) | true | true |

Two observations follow from the table:

- **Without a model**, complex `~` reads each child as `children()` reads it,
  by its shape. The two agree, so this increment does not change that reading.
- **With a model**, `children()` and navigation type every child through the
  model, and complex `~` does not.

`~` with an `extension[0].value` that has no model is typed by the choice
suffix: navigation through `value[x]` names the type, while `children()` and a
key read directly do not. That is a property of navigation, not of `~`, and it
is out of scope.

Where the reading happens:

- `rawEquivalent` (`types/json_equivalent.go`) reads two child values with
  `jsonValue(…, false)`, which tries `tryParseTemporalString` on every string.
- `ObjectValue.Equivalent(other Value)` has no model. The evaluator calls it
  through `eval.Equivalent(left, right)` (`eval/operators.go`), which has no
  model either. `applyEquality` (`eval/evaluator.go`) does not pass one.
- Navigation types a child with `ObjectValue.childElement(basePath, name,
  res)` and reads it with `jsonValueToFHIRValueWithType`.
  `Context.PathOf(obj)` gives an object's path, including the root's.

### What the reading changes for

A child's type changes its equivalence only where JSON holds it as a string
and the FHIR types read differently from the shape:

- `string`, `code`, `id`, `uri`, `markdown` and similar types with a date-like
  value. Typed, they compare as text; read by shape, as dates. This is #263.
- `date`, `dateTime` and `instant`: the shape already reads them as dates.
- `time` (`"10:00:00"`): the shape reads it as a string, the model as a Time.
- `integer64` (R5 only, a JSON string): the shape reads it as a string, the
  model as an Integer64.

A first draft asked the model only for strings that start with a digit. The
adversarial round refuted that: R5 `integer64` is signed (`-5`, `+1`), and an
escape (`"\u0032020"`) hides the first character (§3, A1). No lexical filter
is taken.

### Other implementations

- fhirpath.js and the HL7 validator both type children by their model. Both
  answer false for #263's case.
- On a `dateTime` child (`component[0] ~ component[1]`, plan 021), fhirpath.js
  answers false and Java true; this engine answers true. That case is already
  decided by its type, `dateTime`, and this increment does not touch it.

### Take, improve or discard

| Piece | Decision |
|---|---|
| `childElement` / `childCandidates` (navigation's typing) | **Take**: the same function types a child for `~`, so `~` and navigation cannot disagree on a type |
| `jsonValueToFHIRValueWithType` | **Take**: reads a typed child as navigation reads it |
| `rawEquivalent`'s ASCII fast path | **Improve**: today it is skipped for a string that looks temporal; it also stands for two strings the model types as string-like on both sides |
| `tryParseTemporalString` on a child | **Keep** where there is no model, or where the model does not know the path |

## 2. Plan (revised after round 1)

### Scope

With a model, `~` between two objects reads every child as navigation types
it, recursively, on each side independently. The state carried down each side
is an object built as navigation builds it, `jsonValueToFHIRValueWithType`
(which gives it its declared type and dispatches an abstract `Resource` by
`resourceType`), plus the path `childElement` resolved (which asks
`ResolvePath` for recursive backbones). The child's type is the one
`childElement` returns under that state, and a primitive child is read with
`jsonValueToFHIRValueWithType`. No lexical filter decides which children are
typed (§3, A1).

**Typing state and precedence.** The state of each side is every input that
navigation's resolution reads: the object's type (`Type()`) and the path it
was reached by (`Context.PathOf`, an element path or the root's `SetPath`).
Children are typed in navigation's own order, the one `childCandidates` and
`Evaluator.resolveElement` share, not a new one. For a `BackboneElement`, the
reached path comes first. For any other object, its type comes first
(`Type.name`), then the reached path. A child whose path the model does not
know falls back to the reading by shape, as today. An object from a function
(`children()`, `descendants()`, `resolve()`, `select()`) is typed through the
same order from whatever type and placement it carries (§3b, B2).

**One typing for both (user decision, C1).** The typing alone is extracted
from navigation into one helper that builds no value. It resolves a field's
path and type through `childElement`, detects a choice through `ChoiceTypes`,
asks `ResolvePath` for recursive backbones, and places an `_x` element at its
primitive's path. Navigation (`appendPairedChild`, `unpairedChildren`) and
`~` both call it, so they cannot disagree on a type. The comparison stays
lazy: it types a field only when it reads it, never builds values for
children it pairs unread, and types each field of an object once per
comparison, however many repeat candidates read it.

**Weaker resolvers.** `EquivalentIn` takes the resolver navigation takes and
uses the same optional interfaces (`ChoiceTypes`, `ResolvePath`), detected the
same way. With a resolver that lacks one, it degrades exactly as navigation
does with that resolver. A test covers a minimal resolver and a full one
(§3b, B5).

**A child that reads as nothing** under its type, such as a declared decimal
outside the grammar, is absent on that side, as navigation omits it. Today's
reading without a model is unchanged (§3b, B4).

**Shortcuts.** Answers that skip reading (`sameJSON` in `Equivalent` and
`equivalentJSON`, `writtenAlike` in `equivalentChildren`, `pairInAnyOrder` and
`pairItems`) stand only where both sides have the same typing state, the same
type and the same reached path. Otherwise the pair is read typed (§3, A2;
§3b, B1). An object compared with itself shares its state, so `a ~ a` stays
true unread. Without a model nothing changes.

**Types that differ.** Each side is typed under its own state. Two values of
different types compare as `~` compares those values (`'2020' ~ @2020` is
false), so identical text under different types can be not equivalent.

Unchanged:

- `~` without a model;
- `=` (plan 021: date/time children compare as written);
- the quantity rule (`typedQuantity`) itself.

**Nested Quantities are out of 025 (user decision, C2).** A nested object
carries the model's type so that its own children are typed. Whether the pair
compares as Quantities is still decided by today's test, its unit, code or
system. A unitless nested Quantity stays false, and one with a unit compares
as a quantity, as Java answers. The top level's unitless answer, true where
fhirpath.js says false and Java errors, is FP-27 in
`docs/upstream-findings.md`, to be decided separately. Hand cases pin the
following as they stand:

- nested unitless Quantities with metadata;
- nested Quantities with a unit;
- a Quantity profile (`Age`, `Duration`);
- an object shaped like a Quantity whose type is not one.
- the pairing of repeats.

### Architecture (to explain to the user before implementing)

- **In `types`:** a new exported function,
  `EquivalentIn(left, right *ObjectValue, leftPath, rightPath string, res ElementTypeResolver) bool`.
  `ObjectValue.Equivalent(other Value)` keeps the reading without a model. It
  is documented as that reading, the one `eval.Equivalent` and external callers
  without a model get.
- **In `eval`:** `applyEquality` (`~`, `!~`) uses a new unexported variant of
  `Equivalent` that takes the context. Only in that variant, when the model is
  set and both values are objects, does it call `EquivalentIn` with
  `ctx.PathOf` of each operand. Collections `~` pairs through that variant.
- **Caller audit (Codex round 1, to be repeated at code review):** in
  production code, only `applyEquality` and `valuesEquivalent` call
  equivalence on objects. No function (`intersect`, `distinct`, ...) uses
  `~`.
- **Structure and dependencies:** `ObjectValue` gains no field, so it stays at
  128 bytes, and there is no new dependency.

### Exit criterion

1. **The issue's case:** with an R4, R4B or R5 model, `identifier[0] ~
   identifier[1]` is false, as fhirpath.js and Java answer.
2. **Expected values come from outside the code.** Hand cases take them from
   the spec, citing N1 §6.1.3 and the type's definition. The battery takes
   them from fhirpath.js and Java, with every difference classified.
3. **A consistency property, stated narrowly:** for objects that a model
   placed, over each single-valued (non-repeating) child name, logical names
   only, `a ~ b` implies `a.name ~ b.name` for each name both hold. Here a
   choice is read through `value[x]`, and `_x` belongs to `x`. If every
   non-repeating name is equivalent and the objects have no repeats, `a ~ b`
   holds. Children whose type is Quantity or one of its profiles are left
   out, since they compare as quantities. This checks the property; it does
   not certify a resolver bug that both readings share, which criterion 2
   covers. Typed fuzzing (with a model) checks the following, apart from the
   documented direction of the left quantity's unit:

   - `a ~ a`;
   - the same answer with the operands swapped;
   - key reordering;
   - children that fail to decode.
4. **No change without a model:** corpusdiff without a model against main
   shows 0 differences. With a model, every difference is classified.
5. **Cost:** new benchmarks with a model in R4, R4B and R5:
   - children with changed digits;
   - signed `integer64`;
   - escaped strings;
   - nested paths;
   - primitive metadata (`_x`);
   - adversarial repeats;
   - different operand types;
   - objects with mostly identical children;
   - a late `_x`;
   - reversed repeated objects.

   These are the workloads where building values for children that are never
   read would show (C1).

   They report `ns/op`, `allocs/op` and resolver lookups, with resolver work
   measured apart from repeat pairing, `-count=10`, benchstat against main.
   **Budgets come from measurement**, before the code: the baseline on main
   for each workload, and the rival, fhirpath.js, timed on the same workloads
   by a script in the repo (`difftest/equivalence_timing.js`, after plan
   022's `set_operations_timing.js`). Each workload's budget is set from those
   two numbers and recorded in the plan before implementing. `BenchmarkCorpus`
   and the `~` benchmarks without a model must show no significant change. A
   cache, if needed, is keyed by the whole typing state (type, reached path,
   child name), never by path alone; any lexical shortcut needs an ADR (§3b,
   B6).
6. `make check` green.

### Tests and data (step 7)

**Hand cases, R4, R4B and R5, citing N1 §6.1.3 and the type definitions.** Each
is marked as valid FHIR or as parser tolerance.

- #263's Identifier;
- `ContactPoint.value`, `HumanName.text` and `Annotation.text` with date-like
  text;
- the same with the digit written as an escape (`"\u0032020-…"`);
- an Extension with `valueString` against `valueDateTime`, and the same choice
  with the same value;
- `_x` beside a string child (an extension only, and an id plus a value);
- nested: `Identifier.assigner.identifier.value`, and `Identifier.period.start`,
  whose `dateTime` keeps date semantics;
- `time`: `"10:00:00"` against `"10:00:00.000"`, equal as Time but different
  as text, on R4 `Timing.repeat.timeOfDay`;
- R5 `integer64` on `Attachment.size`: `"1"` against `"+1"`, `"-0"` against
  `"0"`, and the int64 bounds. R4 and R4B type the same element as a number.
- a contained resource and `Bundle.entry.resource`, typed by `resourceType`;
- a recursive backbone (R4 `Questionnaire.item.item`);
- a nested Quantity, and an object with a numeric `value` that is no Quantity;
- the root placed with `SetPath` (a bare HumanName);
- objects from `children()`, `descendants()`, `resolve()` and `select()`;
- the same reached path with different types, and a type-based path that
  disagrees with a placement-based one;
- a minimal resolver (`TypeOf` only) and a full one;
- a declared decimal that reads as nothing, on one side and on both;
- an object whose path the model does not know;
- identical bytes under different types: at the root, nested, in an array,
  and in an element pairing;
- `!~` and collections of objects;
- missing children, empty children and primitives that are only extensions.

**Property:** criterion 3, over the official examples of the three versions
(`hl7.fhir.r4.examples` 4.0.1, `hl7.fhir.r4b.examples` 4.3.0,
`hl7.fhir.r5.examples` 5.0.0), with string children mutated: another offset,
another precision, case changes, signs on `integer64`.

**Metamorphic:** reordering keys and adding an unknown extension do not change
`~`.

**Battery** `difftest/batteries/typed-complex-equivalence`, against
fhirpath.js and Java, with the hand cases.

**Mutations:**

- the model is ignored and children are read by shape;
- `rightPath` takes the left path;
- a nested object loses its path or its type;
- `ResolvePath` is not asked;
- an abstract `Resource` is not dispatched;
- a shortcut stands across different typing states;
- the resolver returns `string` for every path;
- an unknown path errors instead of falling back.

Dropping a cost shortcut is not a correctness mutation; the benchmarks judge
it.

### Rule-5 exceptions

None. Every type comes from the model. The only typing prose that is not read
from the packages, integer64 written as a string, is plan 016's D2, which
already exists.

### New dependencies

None.

## 3. Adversarial round 1 (Codex, 2026-11-15)

All seven findings were checked against the code and the spec, and all are
accepted.

| # | Sev. | Finding | Verification | Resolution |
|---|---|---|---|---|
| A1 | high | The digit filter is false: R5 `integer64` is signed, and an escape hides the first digit | `fhirString` reads `integer64` through `parseInteger64` (`types/object.go`); the R5 regex is `[0]\|[-+]?[1-9][0-9]*` | Filter dropped; any lexical shortcut needs an ADR |
| A2 | high | Byte shortcuts answer before typing, so identical text under different types counts as equivalent | `Equivalent` calls `sameJSON` first; `equivalentChildren` and `pairInAnyOrder` call `writtenAlike`; `pairItems` calls `optionalAlike` | Shortcuts stand only under the same typing state; tests with identical bytes at four levels |
| A3 | high | "Carry the path" was underspecified: abstract resources, recursive backbones, nested Quantity | `jsonValue` builds an untyped `NewObjectValue`; `jsonValueToFHIRValueWithType` dispatches `Resource`; `childCandidates` asks `ResolvePath` | State = typed object plus resolved path; tests for contained, Bundle, backbone, Quantity |
| A4 | high | The navigation oracle shares the resolver, so it can certify a shared bug; repeats and logical names were not handled | Criterion 2 of the draft | Expected values from the spec and the references; property narrowed to non-repeating logical names; a mutation breaks the resolver |
| A5 | medium | Version parity untested; the `"10:00"` fixture is invalid FHIR | R5 `Attachment.size` is `integer64`, R4 `unsignedInt`; FHIR `time` requires seconds | Version-specific cases; valid time pair; valid FHIR kept apart from tolerance |
| A6 | medium | Two equivalence semantics with no stated contract | `PathOf` knows only element paths and the root | Fallback precedence and contract stated; caller audit recorded |
| A7 | medium | The cost gate could pass without exercising the change | `BenchmarkCorpus` includes model-free work | Targeted benchmarks with a model; filter mutation dropped |

## 3b. Adversarial round 2 (Codex, 2026-11-15)

All six findings were checked against the code, and all are accepted. The round
did not converge.

| # | Sev. | Finding | Verification | Resolution |
|---|---|---|---|---|
| B1 | high | "Same typing state" ignored the object's type, which resolution reads first | `childCandidates` asks `Type.name` before the reached path, except for backbones | State = type plus reached path; test with the same path and different types |
| B2 | high | The stated precedence (path first) was the opposite of navigation's | `resolveElement` (`eval/evaluator.go`) and `childCandidates` put the type first, except for `BackboneElement` | Precedence is navigation's, through its own functions |
| B3 | high | A nested Quantity typed by the model compares as a quantity, which ignores `id`, a change the plan called "unchanged" | `typedQuantity` → `AsQuantity` keeps value and unit | Stated as a change on purpose, matching the top level; CONFORMANCE updated; property excludes Quantity children; cases added |
| B4 | medium | A child that fails to read had no contract; reflexivity was tested only untyped | `declaredDecimal` can return nil; navigation omits it; `valueEquivalent(nil, nil)` is false | Absent, as navigation reads it; typed fuzzing of reflexivity, symmetry and reordering |
| B5 | medium | `ElementTypeResolver` alone lacks `ChoiceTypes` and `ResolvePath`; `_x` placement unspecified | `isChoiceName`, `childCandidates` use optional interfaces | Same functions and interfaces as navigation; degraded behavior identical; tests for minimal and full resolvers |
| B6 | medium | The +25% ceiling was arbitrary; a cache by path alone is unsafe; no rival | NUCLEO §10 names fhirpath.js as the rival | Per-workload budgets from the main baseline and fhirpath.js timing, set before the code; cache keyed by the full state |

## 3c. Adversarial round 3 (Codex, 2026-11-15): not converged

B1, B2, B4 and B5 are resolved against the code. B6 is resolved as a plan; its
budgets still have to be measured before implementing. Two medium findings
remain. Per the three-round cap, both go to the user.

| # | Sev. | Finding | Verification |
|---|---|---|---|
| C1 | medium | Reusing `appendPairedChild` and `unpairedChildren` "field by field" builds every child value. That breaks `~`'s documented cost contract: unread pairing of children written alike, selective decoding | Confirmed: `unpairedChildren` scans the whole object and builds a `TypedChild` per field; `appendPairedChild` builds the whole `pairedCollection` |
| C2 | medium | The nested-Quantity change (unitless, compared without `id`) has no support in the references | Confirmed by a battery (scratchpad `b263q`, R4 and R5), see below |

C2 battery. The `low` values are `{"value":1,"id":"a"}` against
`{"value":1,"id":"b"}`; with units, they also carry `mg`.

| Expression | here (main) | fhirpath.js 5.2.0 | Java 6.10.2 |
|---|---|---|---|
| `referenceRange[0].low ~ referenceRange[1].low` (unitless) | true | false | error |
| `referenceRange[0] ~ referenceRange[1]` (unitless, nested) | false | false | error |
| `referenceRange[2] ~ referenceRange[3]` (with `mg`, nested) | true | false | true |

Today a nested unitless Quantity is not recognized (false). The plan would make
it true, in line with the top level. Neither reference answers true. The top
level's unitless answer is itself unsupported. A nested Quantity with a unit
already compares as a quantity, as Java does.

**Proposed resolutions (for the user):**

- **C1:** extract from navigation the typing alone (`childElement`, choice
  detection, `_x` placement), which builds no value. Navigation and `~` share
  it, and the comparison stays lazy, reading only what it must. The new
  workloads: mostly identical children, a late `_x`, reversed repeated
  objects.
- **C2:** take nested Quantities out of 025. Recognition stays as it is today,
  so a model's type on a nested object does not make it a Quantity. The
  unitless Quantity at the top (true, where fhirpath.js says false and Java
  errors) becomes a finding of its own, to be decided separately.

## 4. User decisions (2026-11-15)

- **C1:** extract the typing alone, so `~` stays lazy. Applied in §2.
- **C2:** nested Quantities stay out of 025; the top level's unitless answer is
  FP-27. Applied in §2.

## 4b. Approval

The user approved the plan on 2026-11-15.

## 5. Tests first

### Cost baseline and budgets (set before any code)

Command: `cd conformance && go test -run '^$' -bench BenchmarkTypedEquivalence -count 10 -benchtime 0.3s .`
on 579a581 (main). Rival: `cd difftest && node equivalence_timing.js 500`.

On main, with a model, `~` costs about 1.1 to 2.3 times what fhirpath.js
takes for the same comparison on the small workloads:

- `digits`, `nested`, `elements`: 4.2–5.1 µs here against about 2.2 µs;
- `differentTypes`: 1.9 µs against 1.7 µs.

(fhirpath.js times an already parsed resource, while this engine reads its
JSON on each evaluation.) The repeat workloads are not comparable:
fhirpath.js pairs repeats in order and stops at the first difference.

Budgets, per workload, against main with the same model:

- **Typing is new work** (`digits`, `integer64`, `escaped`, `nested`,
  `elements`, `differentTypes`): at most +30% in time and +15 allocations.
  That keeps them within about 2.5 times fhirpath.js, where they are now at
  about 1.9.
- **Nothing new to read** (`repeats`, `identical`, `lateElement`,
  `reversedObjects`): at most +10% in time. This is where building values
  for children that are never read would show (C1).
- **No model** (`none` variants) and `BenchmarkCorpus`: no significant change
  (benchstat, p < 0.05).

The `identical` and `lateElement` workloads use an Extension with names that
are not FHIR elements: no FHIR type has 40 scalar children. They measure the
paths the model does not know.

### Reference battery

`difftest/batteries/typed-complex-equivalence` holds 57 cases over R4 and R5
(classified in its README).

Run on main:

- 17 cases agree.
- 26: this engine alone differs; all of them are #263's defect.
- 8: fhirpath.js alone differs.
- 6: Java alone differs.

Expected after 025:

- **The 26 #263 cases** take the answer the two references share. String
  children with a date-like value are compared as text, so they come out
  false, and `"…Z"` against `"…z"` comes out true (case aside).
- **Dates already typed `dateTime`** (`Identifier.period`,
  `valueDateTime`): unchanged, true, as Java answers. fhirpath.js says false
  here, as plan 021 recorded.
- **Two divergences taken on purpose, where the references agree with each
  other.** In both, `~` would read a child unlike navigation reading the
  same child. The rule of three engines yields here, as in #207 (CONFORMANCE
  row `~`), because N1 §6.1.3 says "equivalence requires all child
  properties to be equivalent, recursively":
  - **`time`:** `dosageInstruction[0] ~ dosageInstruction[1]`, with
    `"10:00:00"` against `"10:00:00.000"`, becomes true. fhirpath.js says
    false and Java errors, yet fhirpath.js answers the children themselves
    (`timeOfDay ~ timeOfDay`) true.
  - **`integer64`** (R5): `content[0] ~ content[1]`, with `"1"` against
    `"+1"`, becomes true. fhirpath.js and Java both say false. fhirpath.js
    answers `size ~ size` true. Java answers it false, because it reads
    `integer64` as text; plan 016 (D2) reads it as an integer.
  - The user confirmed both on 2026-11-16: type them like navigation.

### Tests written first

- **`conformance/typed_complex_equivalence_test.go`** (fixtures in
  `typed_complex_equivalence_fixtures_test.go`): 34 cases in R4, 34 in R4B
  (on R4's data) and 38 in R5, plus cases without a model and with the root
  placed by `SetPath`. On main, 18 fail in R4, 18 in R4B and 20 in R5, all of
  them #263 cases, `time` and `integer64`, or objects from functions. The
  cases expected unchanged pass on main: without a model, `dateTime`,
  nested Quantities, unknown paths.
- **`conformance/typed_complex_equivalence_property_test.go`**: criterion 3
  over 300 examples per version (seed 25), about 6,700 children, 1.7 s and
  114 MB RSS. It skips when `build/corpusdiff/cache` is absent. On main it
  finds 749 violations, all from the mutation "date-like text, another
  offset".
- **`types/typed_equivalence_test.go`**: tests against `EquivalentIn` with
  fake resolvers. The cases are:
  - identical text under different types, at four levels;
  - which comes first, backbone or type;
  - a minimal resolver against a full one, with `TypedChildren` as the
    oracle;
  - a decimal that does not read, treated as absent;
  - reflexivity and symmetry.

  It does not compile on main (`EquivalentIn` does not exist yet).

## 6. Implementation

A Sonnet subagent implemented the plan.

- **Typing helper** (`types/object.go`): `typeFieldOf(objType, basePath,
  name, res)` returns a field's path, type and whether it is a choice,
  without building a value. `appendPairedChild` and `unpairedChildren` call
  it, so navigation and `~` type a field through one function.
  `childCandidates` returns a fixed-size array instead of a slice.
- **Comparison** (`types/typed_equivalent.go`): `EquivalentIn` types each side
  under its own state, lazily. A field is typed when it is read, once per
  state and field. A shortcut that reads nothing stands only under the same
  state, or when the field's own typing is the same. A nested Quantity is
  told by shape (C2).
- **Evaluator** (`eval`): `applyEquality` takes the context. `compileEquality`
  (`eval/plan.go`) and `valuesEquivalentIn` (`eval/operators.go`) send `~`
  between two objects to the typed comparison when there is a model; a
  quantity is told first. `eval.Equivalent` is unchanged.
- **Deviation, confirmed by the user on 2026-11-16:** a new exported type,
  `types.Equivalence`, keeps the typing of each field across the pairs of one
  evaluation. `eval.Context` holds one, and `SetModel` resets it. Without it,
  `~` between 40 reversed objects allocated about 25,000 more times.
  `EquivalentIn` is a wrapper around it. It is documented as one evaluation's
  state, not safe across goroutines.
- **One lint fix in the tests written first:** `%q` in place of `"%s"`
  (gocritic), with no change in behavior.

### Cost against budgets

The machine had a load near 6, so main and the branch binaries were run
interleaved, 8 rounds (`go test -c` on each tree, `benchstat` in the
scratchpad, `ab-cmp.txt`). Times against main:

| Workload | Time | Allocs | Budget |
|---|---|---|---|
| digits | −10% | 59 → 55 | +30%, +15 |
| integer64 (R4 / R5) | +7% / +14% | 61 → 62 / 63 → 70 | +30%, +15 |
| nested | +15% | 67 → 72 | +30%, +15 |
| elements | +11–12% | 67 → 79 | +30%, +15 |
| differentTypes | +26% | 49 → 62 | +30%, +15 |
| repeats | +3–5% | 85 → 87 | +10% |
| reversedObjects | no significant change | 5,004 → 3,451 | +10% |

The subagent's run shows no change in the `none` variants.

## 7. Validation

- **corpusdiff:** `make corpusdiff BASE=origin/main` gives r4 521,007
  evaluations, r4b 308,577 and r5 360,675, with 0 answers that differ
  (scratchpad `impl025/corpusdiff.txt`).
- **Battery on the branch** (`impl025/battery.txt`): 41 cases agree. The 26
  #263 cases now take the answer the two references share. What still
  differs:
  - 4 `integer64` and `time` cases, deliberate (§5);
  - 8 `dateTime` cases, where fhirpath.js alone differs (plan 021);
  - 4 where Java errors or reads `integer64` as text;
  - `"-0"` against `"0"`, which is parser tolerance.
- **Mutations, `mutate025.py`:** in the first run, 10 of 14 were caught.
  - **V2** (the right side takes the left path) survived, and so did **V6**
    (an abstract `Resource` is not dispatched). Tests added:
    - two backbones at different paths whose `value` is `dateTime` in one and
      `string` in the other (`ImmunizationRecommendation…dateCriterion`
      against `TestScript…requestHeader`, found in the R4 core package);
    - two `Bundle.entry` whose resources differ in a string child.

    Both are now caught.
  - **V9** (a value that does not read is not absent) survived. The `01` case
    never reaches that branch, because the reading by shape already makes a
    number outside the grammar null. A case was added: a decimal whose
    exponent the decoder refuses (`1e2147483648`), absent as navigation
    leaves it. It is now caught.
  - Beside its element, that decimal also drops the element, in navigation
    (`TypedChildren`) as in `~`. That belongs to FP-6 and is not pinned.
  - **V10** (field typings equal by path alone) survives, and is equivalent:
    a field's type is the resolver's `TypeOf` of its resolved path, so two
    typings with the same path have the same type.
- **A divergence of the same class as `time` and `integer64`.** The new
  backbone case gives `"2020"` as `dateTime` against `"2020"` as `string`;
  `impl025/battery-extra.txt` and `battery-direct.txt` hold the runs.
  - fhirpath.js and Java answer the objects true.
  - fhirpath.js answers the values themselves false, and so does this
    engine; Java answers them true, since it takes `'2020' ~ @2020` as true.
  - This engine answers the objects false, as its children compare. The
    case is in the battery (rows 58–63) and follows the user's decision of §5
    (type like navigation).
- **Full set 010–024, run again:** 235 caught. The same five equivalent
  mutants survive (H22, K5, K9, K12a, S1); M2, M7 and M8 no longer match the
  code, as in 024. Output in `mut/history025full.txt`, 3,583 s.
- **`make check`:** green, 507 s, 660 MB RSS (`impl025/check.txt`). Lint first
  flagged the new tests: `misspell` read Spanish comments as English words,
  one variable was unused, and the case table built with `append` drew
  `prealloc`. They were reworded and the table became one literal; no case
  changed.

## 8. Code review round 1 (Codex)

- **First attempt:** Codex's sandbox could not write to the copy, so it
  verified nothing.
- **Second run:** read-only; it proposes reproductions, which I run. One
  suspicion I checked myself and discarded: repeated nested Quantities
  (`Device.property.valueQuantity`) answer as they do on main.

| # | Sev. | Finding | Verification | Resolution |
|---|---|---|---|---|
| R1 | medium | When one side's type is unknown, `objectsEq` compares both untyped: the known side loses its typing, and the untyped `sameJSON` shortcut answers across states | Confirmed with the probe. `Patient.identifier[0] ~ Patient.name[0]`, the same `assigner` JSON under `Identifier` (known) and `HumanName` (unknown), gives true, while the navigated values give false. 3 cases added; they fail in R4, R4B and R5 | Fixed (Sonnet): `unknownEq` reads the unknown side by shape under an untyped state, and the known side keeps its own. The 3 cases pass |

**A regression the R1 fix introduced, found while verifying it:** in the
mixed pair, a nested Quantity with a unit was no longer told by its shape.
`referenceRange[0] ~ component[0]`, with `low` 1 mg (known) against 1000 ug
(`component.low` unknown), gave false where main gives true, against C2.
`unknownEq` now tells Quantities by shape first, as the typed path does. Two
cases were added (both directions), and mutants V15 (mixed pair read
untyped) and V16 (mixed pair skips the Quantity shape) are in
`mutate025.py`. All mutations run again before round 2
(`mut/history025r2.txt`).
Result (2026-11-16): V1–V16 caught except V10 (equivalent, as before);
010–024 unchanged (235 caught; the same equivalents survive, and M2, M7 and
M8 still find no pattern, as in the run before R1). `make check` green.

### Code review round 2 (Codex)

Input: the R1 finding, the change above, and the new diff.

| # | Severity | Finding | Verification | Resolution |
|---|---|---|---|---|
| R2-1 | medium | An `extension` inside an element the model does not know is still typed as `Extension`, so its `valueString` reads as a string and not by shape: `Patient.identifier[0] ~ Patient.name[0]` with the same extension under both `assigner`s gives true; Codex expects false | Probe: true, and navigation answers the same (`Patient.name[0].assigner.extension.value.type().name` is `string`, and `… .value ~ … .value` is true). fhirpath.js answers true both ways; Java answers false because it drops the unknown element when it parses (`Patient.name[0].assigner.exists()` is false there) | Rejected: 025 types each child as navigation does, and navigation types this extension. Two guard cases added (true, both ways) and the battery case (`extensions.json`) |
| R2-2 | medium | Under different typings, `pairValues` skips the pass that pairs values written alike, so a greedy pairing can miss: `[1.0, 1.1]` against `[1.1, 1.2]` | Codex's own reproduction (`Identifier.value`) does not hold: the model types `Identifier.value` as string, and navigation gives false. On a case where both sides read as decimals (`referenceRange[0].low ~ component[0].low`, `decimals.json`), this engine answers false; main answered true; fhirpath.js and Java answer false in R4 and R5, both ways | Rejected as a regression: the branch now agrees with both references. Two guard cases added (false, both ways) and the battery case. The navigated lists (`low.value ~ low.value`) give false here and in fhirpath.js and true in Java; that collection-level difference predates 025 and is outside it |

**Round 2 converges:** no confirmed finding of medium severity or higher.
The battery has 71 cases.

## 9. Closing

- CONFORMANCE.md: a section, "`~` types a complex value's children as
  navigation does"; the whole-object paragraph now separates `=` (untyped)
  from `~` with a model; three rows in "Measured, and not followed" for the
  deliberate divergences (backbones `dateTime` against `string`, R5
  `integer64` `"1"` against `"+1"`, Dosages with `time` children).
- Evidence: `make check`, `TestTypedComplexEquivalence` (R4, R4B, R5), the
  battery (`difftest/batteries/typed-complex-equivalence`, 71 cases), corpusdiff,
  mutations (`mutate025.py`, V1–V16).
- Pending, outside 025: FP-27 (`docs/upstream-findings.md`); the navigated
  decimal lists of R2-2, where Java alone answers true; whether the user
  keeps the backbone `"2020"` divergence (recorded under the time/integer64
  decision).
