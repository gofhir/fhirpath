# 019 — Analyze through an abstract type (#257)

Status: closed (step 9), 2026-11-03. Code review converged in round 2.

## 1. Prior review

### The defect

`analyzer.member` (analyze.go) reports `T has no element X` when the model
"describes" T, which `modelDescribes` decides by asking for `T.id`. The
published gofhir/models do not describe the abstract types (`Resource.id` is
unknown), so the check is skipped for them. A model built from the
StructureDefinition snapshots, as the validator builds one, does describe
`Resource` and `DomainResource`, and then:

| Expression | Context | models/r4 | a model that describes Resource |
|---|---|---|---|
| `contained.contained.empty()` (dom-2) | Patient | no error | `Resource has no element contained` |
| `Bundle.entry.resource.name` | Bundle | no error | `Resource has no element name` |
| `contained.text.empty()` | Patient | no error | `Resource has no element text` |

### The text

- StructureDefinition.abstract (structuredefinition.html, R4/R4B/R5):
  "Whether structure this definition describes is abstract or not - that is,
  whether the structure is not intended to be instantiated."
- An element typed `Resource` (`DomainResource.contained`,
  `Bundle.entry.resource`, `Parameters.parameter.resource`) holds an
  instance of a concrete resource type, which names itself in `resourceType`
  (json.html). The elements it may have are those of that concrete type.
- FHIRPath 2.0.0 §"Path selection" / N1: navigating to an element that does
  not exist is not an error, it returns empty; the type check is an addition
  of this engine's `Analyze`, so it must not reject what an instance can hold.

### What the model can say

`eval.Model` has no notion of an abstract type, and neither has
`gofhir/models` (`FHIRPathModelData`: ChoiceTypes, TypeOf, ReferenceTargets,
ParentType, IsSubtype, ResolvePath, ChildElements, FHIRTypeOf, HasType,
IsResource, FHIRVersion). `StructureDefinition.abstract` is in the packages
the models are generated from. The engine already hard-codes the four
abstract resource types in `types.IsAbstractResourceType` (types/object.go),
used when reading an object held by an abstract element; that list has no
ADR.

The precedent for a capability a model may or may not have is an optional
interface: `fhirpath.VersionedModel` (`FHIRVersion()`) and
`fhirpath.TypeRegistry` (`LookupType`), each mirrored in eval.

### gofhir/server

It does not call `Analyze` (only UCUM's parser has an `Analyze`). Nothing to
take.

### The references

fhirpath.js has no static check. HL7's Java `FHIRPathEngine.check()` resolves
an abstract type's element against the types derived from it (not measured
here: the battery runs evaluation, not `check()`).

## 2. Options (for the user)

- **A. Optional interface** `AbstractTypes { IsAbstract(typeName string) bool }`
  (public in fhirpath, mirrored in eval, as VersionedModel). A model that
  implements it and says T is abstract: a missing element of T proves
  nothing, the step lands on an unknown type. A model that does not: as now.
  The validator's model reads `StructureDefinition.abstract`; gofhir/models
  can gain it later through its own session. Smallest, spec-driven; the
  issue's repro needs the model to implement it.
- **B. A as well, falling back on `types.IsAbstractResourceType`** when the
  model does not implement it. Fixes the repro with no model change, but uses
  a hard-coded list (rule 5: ADR).
- **C. Precise, like Java:** an element of an abstract T is accepted when some
  type derived from T has it, which needs a way to enumerate subtypes
  (`Subtypes(T)`), a larger API. Catches `contained.foo` as an error.

The user chose **A** (2026-11-03).

## 3. Plan

### Decisions

- **D1 (API):** a new optional interface in `model.go`, documented as
  `VersionedModel` and `TypeRegistry` are:
  ```go
  // AbstractTypes is an optional interface a Model may also implement to say
  // which types are abstract (StructureDefinition.abstract).
  type AbstractTypes interface {
      IsAbstract(typeName string) bool
  }
  ```
  Analyze reads it with a type assertion, as it reads `FHIRTypes`. No change
  to `Model`, so every existing model keeps compiling and answering as now.
  No mirror in eval: evaluation does not use it (F6). `HasType`, `ParentType`
  and `IsSubtype` cannot answer it: "has subtypes" would make Quantity
  abstract (Age derives from it), and is right for BackboneElement only by
  accident (F7).
- **D2 (rule):** in `analyzer.member`, before the final fault, when the model
  implements `AbstractTypes` and says the input type is abstract, the step is
  not a fault and lands on the unknown type: an instance there is of a
  concrete type derived from it (structuredefinition.html, `abstract`), or, for
  R5's CanonicalResource and MetadataResource, which no type derives from, of
  a resource that follows that interface pattern (F5), which
  may have the element. What follows is unchecked, as after any unknown type.
  **Only where the path does not describe its own elements** (F1):
  `a.path == inputType || !modelDescribes(a.path)`. A backbone element is
  typed `BackboneElement`, which is abstract too, but its path
  (`Questionnaire.item`, `Patient.contact`) defines its elements, so
  `Questionnaire.item.lnkId` keeps its fault.
- **D3 (only the missing-element fault):** the other faults stay as they are
  for an abstract input: a choice named by its suffix
  (`contained.valueQuantity`, if the abstract type had a choice), a resource
  type that is not the context, a System type navigated. An element the
  abstract type does have (`contained.meta`) still resolves to its type.
  `contained.Patient.name` still says "Patient is not reachable from
  Resource", an intended survivor (F8).
- **D4 (evaluation):** unchanged. Only `Analyze` changes.
- **D5 (no list):** `types.IsAbstractResourceType` is not used here; the
  answer comes from the model alone. The hard-coded list stays where it is
  (reading an object held by an abstract element), out of scope.

### Out of scope

- Option C: checking an abstract type's element against its derived types.
- `IsAbstract` in gofhir/models: proposed to that repo's session with the
  evidence, not edited here. Until then, models/r4 does not describe
  `Resource` at all, so it already skips the check.
- `types.IsAbstractResourceType` and its missing ADR (a finding to log).

### Exceptions to rule 5

None: no type is named in production code.

### Exit criterion

1. With a model that describes `Resource`/`DomainResource` and implements
   `IsAbstract` from the packages, the three rows of §1 give no error, in R4,
   R4B and R5.
2. With the same model without `IsAbstract`, the answers are as on main (the
   fault), so the interface is what changes them.
3. `contained.meta.exists()` still resolves; a misspelled element on a
   concrete type (`Patient.nmae`) is still a fault; `contained.valueQuantity`
   style faults unchanged. Backbone misspellings fault as on main, with
   models/r* alone and with the snapshot model: `Questionnaire.item.lnkId`,
   `item.item.lnkId` (context Questionnaire.item), `Patient.contact.nmae`,
   `Parameters.parameter.nmae` (F1, F2).
4. Core constraints (`corpusdiff` `TestCoreConstraintsAnalyze`), with the
   snapshot-built model: no fault beyond the four recorded defects of the
   specification.
5. Mutations caught, `make check` green.

### Tests (step 5)

- In `conformance` (the root module has no gofhir/models dependency, F3),
  `analyze_abstract_test.go`: a model wrapping models/r4, r4b and r5 that adds what the snapshots say of the
  abstract types (`Resource.id`, `Resource.meta`, `DomainResource.contained`,
  `DomainResource.text`), with and without `IsAbstract`. Expected values from
  structuredefinition.html and the snapshots, not from the engine:
  dom-2, `contained.text.empty()`, `Bundle.entry.resource.name`,
  `Parameters.parameter.resource.name`, `contained.meta.exists()`,
  `contained.contained.contained.id`, a concrete misspelling, and a type that
  is not abstract (`HumanName.foo`) still a fault; the backbone misspellings
  of the exit criterion; `contained.Patient.name`; `ofType(DomainResource).foo`;
  `contained.where(nmae.exists())`. A unit test in the root module with the
  hand-written map model `api_analyze_test.go` already has, for the rule
  alone.
- `corpusdiff`: a model built from the core packages' StructureDefinitions
  (elements of the abstract types from their snapshots, `abstract` from
  `StructureDefinition.abstract`) over every core constraint, R4, R4B, R5.
  The data source is the pinned packages, so the abstract set is read, not
  written (`abstract: true`, `derivation: specialization`; constraint
  profiles excluded). Measured (F4): R4 Resource, DomainResource, Element,
  BackboneElement; R4B the same plus DataType; R5 adds Base, BackboneType,
  PrimitiveType, CanonicalResource, MetadataResource. Paired with the backbone
  misspellings, since the core constraints hold none, and the test does not
  run in CI (F2).

### Mutations (step 7)

The rule removed; the rule applied without asking the model (every type
abstract); `IsAbstract` asked of the path instead of the input type; the rule
placed before the choice-suffix check (D3); the step landing on the abstract
type instead of unknown; the corpusdiff model's abstract flag ignored.

### Dependencies

None.

## 4. Adversarial round (Opus, 2026-11-03)

Probes in a scratch copy, with D2 applied, and with F1's fix.

| # | Severity | Finding | Resolution |
|---|---|---|---|
| F1 | critical | BackboneElement is abstract (R4, R4B, R5, measured in the pinned packages) and models type every backbone `BackboneElement`; D2 on the input type alone hides `Questionnaire.item.lnkId`, `Patient.contact.nmae`, which fault on main even with models/r4 | **Accepted**, confirmed: the rule applies only where the path does not describe its own elements; the misspellings join the tests and the mutations |
| F2 | high | The core-constraint test is not in CI and has no misspellings: it cannot see a lost fault | **Accepted**: paired with negative cases |
| F3 | high | The root module has no gofhir/models dependency | **Accepted**: model tests in conformance; a map-model unit test in the root |
| F4 | medium | The abstract set per version: DataType is in R4B already; R5 adds five | **Accepted**: listed, read from the packages, constraints excluded |
| F5 | medium | R5's CanonicalResource/MetadataResource are interfaces no type derives from | **Accepted**: D2 cites the interface pattern |
| F6 | low | An eval mirror with no consumer | **Accepted**: no mirror |
| F7 | low | Why not HasType/ParentType/IsSubtype | **Accepted**: stated in D1 |
| F8 | low | `contained.Patient.name` survives | **Accepted**: stated and tested |

## 5. Validation (2026-11-03)

- Tests first: the rows that must change failed on main in R4, R4B and R5
  (dom-2, `contained.text.empty()`, `Bundle.entry.resource.name`,
  `Parameters.parameter.resource.name`, `contained.contained.contained.id`,
  `contained.where(nme.exists())`); the faults that must stay (backbone
  misspellings, `Patient.nme`, `contained.Patient.name`) held before and after.
- corpusdiff `TestCoreConstraintsAnalyzeThroughAbstractTypes` (a model built
  from the pinned core packages' snapshots and `abstract` flags): only the four
  recorded `coreConstraintFaults`, R4, R4B, R5. On main it failed in all three
  (dom-2).
- Mutations (`mutate019.py`): P1 rule removed, P2 every type abstract, P3 the
  path condition of F1 dropped, P4 abstract asked of the path, P5 the step
  landing on the abstract type, P6 every path taken as described: 6/6 caught.
  P5 survived at first; the row `contained.text.meta.nme` (D2: what follows is
  unchecked) now pins it. The corpusdiff oracle with the abstract flag inverted
  fails.

## 6. Code review

### Round 1 (Opus, 2026-11-03)

Holds: every route the plan names, backbone misspellings in all forms, R5
CanonicalResource, `resolve()`; lint clean; corpusdiff passes, and reports
406 faults with the rule removed.

| # | Severity | Finding | Resolution |
|---|---|---|---|
| F1 | medium | The conformance test's model omits snapshot elements (DomainResource.id…, BackboneElement.id…, Element.id…), so `ofType(DomainResource).foo` passed for the wrong reason and the mutant dropping `a.path == inputType` survived | **Accepted**: model completed from the snapshots, row corrected, an abstract-context row added; the mutant joins mutate019 as P7 |
| L1 | low | "The path describes its elements" assumes a model keyed by backbone path | **Accepted**: stated in the docs |
| L2 | low | model.go overstates what Analyze skips; temporal wording | **Accepted** |
| L3 | low | FP-18 inaccurate; no gofhir/models entry | **Accepted** |

### Round 2 (Opus, 2026-11-03): converged

Mutations before the round: 179 (010–019), all caught. F1 holds: the mutant
P7 fails the conformance test in R4, R4B and R5. The snapshot elements match
the pinned snapshots. Two lows, fixed: the test comment claimed id has the
same type in every version (R4 gives string); CONFORMANCE.md said "analyzed as
before".

## 7. Closing (2026-11-03)

- `make check` green; corpusdiff `TestCoreConstraintsAnalyzeThroughAbstractTypes`
  passes (R4, R4B, R5: the four recorded constraint faults only).
- Mutations: mutate019 P1–P7 caught (scratchpad, `mut/mutate019.py`).
- gofhir/models: `IsAbstract` and the abstract types' elements are proposed as
  MOD-1 in docs/upstream-findings.md, unpublished. No session of that repo was
  found to send it to.
