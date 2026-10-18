# 006 — A choice element's bare name is not an element under a model

Status: done, pending commit
Issue: #219

## Problem

#219 reported three things navigation answers that the FHIR logical model
does not have. Measured across the three engines (`make three-engine`,
fhirpath.js 5.2.0, HL7 validator 6.10.2, R4 and R5 models), they part:

| Case | This engine | fhirpath.js | Java |
| --- | --- | --- | --- |
| `Patient.extension.where(url = 'lit').value` over `{"url": "lit", "value": "lit"}` | `lit` | empty | empty |
| `Patient.descendants().count()` on the same Patient | 17 | 16 | 16 |
| `Patient._birthDate`, `Patient._birthDate.id`, `Patient.name._given` | the element | the element | error |
| `Basic.codeFoo` (a field no definition has) | `y` | `y` | empty |
| `Basic.extension` over `extensionCoding`, with a model | empty | empty | empty |

The 17 against 16 is the same key: `Patient.extension.children()` lists the
literal `value` beside the extension's url (measured with and without it).

## What changes, and what does not

A second battery (backbone choices, both forms present, children()) showed
more, and a defect the issue did not name:

| Case | This engine | fhirpath.js | Java |
| --- | --- | --- | --- |
| `extension.value` over `{"valueString": "x", "value": "bare"}` | `bare` | `x` | `x` |
| `Questionnaire.item.item.enableWhen.answer` over `answerCode` (no type of `answer[x]`) | `top` | empty | empty |
| `Observation.component.value` over `valueCode` (no type of `component.value[x]`) and over `{"value": …, "valueString": "vs"}` | `compcode, barecomp` | `vs` | empty |
| `Observation.value` over `{"value": …, "valueString": "vs"}` | `bareobs` | `vs` | empty |
| `extension.children()` over both forms | `s, x, bare` | `s, x, x` | `s, x` |

By the three-engine rule (`CONFORMANCE.md`, rule 1):

1. **Choices inside backbone elements and contentReference paths are read
   by the model.** Today `resolveElement` drops the nested path of a
   backbone choice (its `TypeOf` is empty) and falls back to
   `BackboneElement.value`, which has no choice types, so suffixes are
   guessed even under a model and types the version does not allow are read
   (`component.valueCode`, `enableWhen.answerCode`). Both references refuse
   them. Followed.
2. **A choice element's bare name, under a model, is no element**: not
   navigated to, not a child, and it does not hide the suffixed field. Where
   both are written, the suffixed one is the value. The references agree on
   the extension; on `Observation.value` and `component.value` fhirpath.js
   answers `vs` and the Java engine nothing (it refuses the element), and the
   text bears fhirpath.js out: `valueString` is a valid element. In
   `children()`, fhirpath.js lists the suffixed value twice, a defect of its
   own; the Java engine's `s, x` is what the text gives.
3. **`_x` keys.** fhirpath.js reads them as this engine does; the Java engine
   ends the evaluation. One reference refuses, so this engine keeps its
   answer, catalogued under "Measured, and not followed".
4. **Without a model.** fhirpath.js does run without one: it resolves no
   suffix (`Observation.value` over `valueString` is empty) and reads bare
   keys. This engine keeps the field-name heuristic, its documented way to
   evaluate without a model ("Type of a polymorphic element without a
   model"); recorded under "Measured, and not followed".
5. **An invalid suffix in `children()`** (`Observation.valueCode`): this
   engine and fhirpath.js list it, the Java engine does not; navigation
   already gives nothing for it. Unchanged here, as for any key no
   definition has (`Basic.codeFoo`: this engine and fhirpath.js `y`, Java
   empty).

## Prior review

- `gofhir/server` has no implementation of its own.
- Where the key is read: member navigation (`eval/evaluator.go`,
  `navigateMember`: direct `GetCollection(name)` before polymorphic
  resolution) and typed children (`types/object.go`, `TypedChildren`,
  `pairedFields`/`unpairedChildren`), which `children()` and `descendants()`
  read.

## Approach

- `resolveElement` keeps the nested (or contentReference-resolved) path of a
  backbone element's member when the model gives it choice types, so
  `value[x]` under a backbone resolves by the model as it does at the
  resource level.
- Navigation: when the element's path has choice types, only its suffixed
  fields are read (by the model's types); the bare key is not read and does
  not hide them.
- Typed children: `ElementTypeResolver` gains an optional `ChoiceTypes`
  interface, asserted as `ResolvePath` and `ElementOrderResolver` are; a key
  for which `childElement` finds no type, and whose path among the same
  candidates (nested path first) has choice types, is not a child. The extra
  lookup happens only for keys that resolved to nothing.
- The check uses only the model's `ChoiceTypes`; no element name is written
  in code.

## Tests (written before the change)

Under R4, R4B and R5 models (R5's Extension allows 54 types), and without
one where the case applies:

1. Extension: bare `value` alone (empty), beside `valueString` (`x`), and in
   `children()` (`s, x`).
2. Backbone choices: `Observation.component.value` over a disallowed suffix
   (empty) and over bare plus `valueString` (`vs`); a backbone in a datatype
   (`Dosage.doseAndRate.dose`, `Timing.repeat.bounds`); contentReference
   paths (`Questionnaire.item.item.enableWhen.answer`,
   `Parameters.parameter.part.value`).
3. Resource choices: `Observation.value`, `Patient.deceased` bare and beside
   their suffixed fields; in a contained resource and in a Bundle entry.
4. Unchanged: `Quantity.value`, `Identifier.value` (not choices);
   `_birthDate` navigation; a non-choice element; `children()` with an
   invalid suffix; evaluation without a model.

The two batteries are kept in `difftest/batteries/` as the evidence.

## Validation

`make corpusdiff BASE=origin/main`: the official examples are valid FHIR, so
values should not change; types read by a backbone choice may (guessed from
the suffix before, given by the model now). Every difference is examined. The fhir-functions and
navigation spec cases and the official suite: no new failure.

## Exit criteria

1. Tests fail before and pass after.
2. Corpus: no difference, or each explained.
3. CONFORMANCE.md: case 1 as a gap closed, case 2 catalogued.
4. `make check` passes.

## Adversarial round

Independent reviewer; finding 1 confirmed by a second three-engine battery.

| Finding | Resolution |
| --- | --- |
| HIGH: backbone choices never consult the model (`resolveElement` drops the nested path), so the planned check would not fire there and disallowed types are read today | Accepted, measured: `resolveElement` keeps the nested path when it has choice types; cases added |
| Both forms present is the real bug, absent from the first battery | Accepted: measured (table above); tests added |
| `TypedChildren` design unspecified (resolver has only `TypeOf`) | Accepted: optional `ChoiceTypes` assertion, checked only for keys with no type, same candidates as `childCandidates` |
| Invalid suffix in `children()` is uneven but agrees with fhirpath.js | Accepted: stated, unchanged |
| fhirpath.js does run without a model | Accepted, measured: decision 4 recorded as measured and not followed |
| Three-engine readings, non-choice `value` elements, Bundle/contained | Confirmed; cases added |

## Record

- Approved by the user.
- Tests first (`conformance/choice_base_name_test.go`): before the change
  15 cases failed in each version (backbone choices, bare keys, children);
  the unchanged ones passed.
- Implementation: `resolveElement` keeps a backbone member's nested path
  when the model gives it choice types; `navigateMember` reads no bare field
  where the element's path has choice types; `TypedChildren` leaves out a
  field with no type whose path has choice types (`isChoiceName`, an
  optional `ChoiceTypes` on the resolver).
- Mutations, each reverting one of the three: the backbone step fails the
  `dose`, `bounds`, `component.value` and `part.value` cases; the
  navigation step fails `Observation.value`, `Patient.deceased`, contained,
  Bundle and extension cases; the children step fails every `children()` and
  `descendants()` case of an object read field by field; the paired path
  (an object that also holds a `_x` element) had no case until review
  round 1 added two.
- The official suite and the spec cases pass with unchanged baselines;
  `make corpusdiff BASE=origin/main` reports 0 answers differing over
  521,007 (R4), 308,577 (R4B) and 360,675 (R5) evaluations, as the official
  examples are valid FHIR.
- The two three-engine batteries are in `difftest/batteries/choice-base-name`.

### Code review

| Round | Findings | Resolution |
| --- | --- | --- |
| 1 | Medium: the paired-children guard (`appendPairedChild`) untested — disabling it passed everything. Low: three-way `children()` splits and the invalid-suffix and unknown-key cases not catalogued; some claims named cases the battery does not hold. No behaviour defect: types of backbone choices over the official examples unchanged (corpusdiff rerun comparing `Type()`), Analyze already agrees, no performance regression | Cases for an object read paired (`"_status"` beside a bare `value`; a bare `value` with its `_value`), failing without the guard; CONFORMANCE.md gains the three-way `children()` paragraph and separates what the battery measured from the engine-only tests |
| 2 | No medium or higher (converged); the paired guard now pinned (disabling it fails 6 subtests). Low: the `children()` counts in CONFORMANCE.md named the wrong Observation and mixed the invalid-suffix component into the component count | Wording corrected: the Observation written both ways, and the second component alone (2, 3 and 1) |
