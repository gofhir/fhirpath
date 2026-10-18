# 002 — extension(url) returns typed items

Status: plan, awaiting approval
Issue: #214

## Problem

With a model, `X.extension(u)` returns its items as untyped `Object`, while
`X.extension.where(url = u)` returns them as `Extension`. Measured at `38f8021`
with `gofhir/models` v2.10.0, R4 and R5 alike:

| Expression | Result |
| --- | --- |
| `Patient.extension('http://e/x') is Extension` | `false` |
| `Patient.extension.where(url = 'http://e/x') is Extension` | `true` |
| `Patient.birthDate.extension('http://e/y') is Extension` | `false` |
| `Patient.birthDate.extension.where(url = 'http://e/y') is Extension` | `true` |

The second pair (an extension on a primitive's element) is not in the issue.

It is not only the type. An untyped item reaches `.value` through the fixed
suffix guess list (`polymorphicTypeSuffixes`) instead of the model, so values
are lost or invented (measured, synthetic Patient):

| Model | Expression | `extension(u)` | `.extension.where(url = u)` |
| --- | --- | --- | --- |
| R5 | `.value` over `valueCodeableReference` | empty | `CodeableReference` |
| R4 | `.value` over `valueInteger64` (not an R4 type) | `integer64 5` | empty |
| R4, R5 | `.url` | `String` | `uri` |

On the official R5 examples three real values are lost today
(`Organization-hl7` extended-contact-availability,
`Patient-patient-example-sex-and-gender` sourceDocument and supportingInfo).
Results change type in 16 % (R4), 26 % (R4B) and 22 % (R5) of the example
files.

FHIR R4 and R5 `fhirpath.html` §2.9.1.5 (R5: §2.1.9.1.5) define the function by
the other expression:

> `extension(url : string) : collection` — Will filter the input collection
> for items named "extension" with the given url. This is a syntactical
> shortcut for `.extension.where(url = string)`, but is simpler to write.
> Will return an empty collection if the input collection is empty or the url
> is empty.

## Cause

`fnExtension` (`funcs/fhir.go:448`) reads `obj.GetCollection("extension")`.
Member navigation (`eval/evaluator.go`, `navigateMember`) reads
`GetCollectionWithType(name, fhirType)` with the type `resolveElement` gives
(`Extension` for every `extension`, `evaluator.go:2870`) and records the
element path on each child (`placeAt`), beneath which the next step resolves
`value[x]` and nested extensions.

## Prior review

- `gofhir/server` has no implementation of its own: it calls this library
  (v1.14.0). Nothing to take.
- `children()` and `descendants()` (`funcs/aggregate.go`) already read typed
  children through `TypedChildren(pathOf(ctx, obj), typeResolver(ctx))`.
- `getExtensionValue(url)` (`funcs/fhir.go:508`) has a defect of its own: it
  reads the value untyped and scans a fixed list of 20 suffixes, so
  `valuePositiveInt` (and markdown, id, uuid, unsignedInt, every R5-only type)
  returns empty in R4 and R5. It is not in the R4 or R5 specification. Out of
  this increment (one front at a time): reported as a separate issue.
- `hasExtension` returns a Boolean and is not affected.

## Approach

Make the shortcut literally what the specification says it is: the items
`extension(u)` returns are the items member navigation to `extension` returns,
filtered by `url`. The function stays in `funcs` (arity, registration and
argument evaluation unchanged) and asks the evaluator to navigate the member,
so typing and element placement come from the one place that does them.

- `eval.Context` gains an exported method that navigates a named member of
  a collection with the evaluator's rules (typed read, element path, a
  primitive through its element). It delegates to `navigateMember`, which
  reads only the context (model, `PathOf`), not `This()`, so it is safe on the
  copies `WithThis`/`WithIndex` make. This is an addition to the public API of
  `eval`, documented on the method.
- `fnExtension` calls it with `"extension"` and keeps its URL filter.

Rejected alternatives:

- Repeating the typed read inside `fnExtension`
  (`GetCollectionWithType("extension", "Extension")` plus `SetElementPath`):
  duplicates `resolveElement`'s path rules for one member, which then must
  change together with navigation. Two writers of the same thing.
- Rewriting `extension(u)` into `.extension.where(url = u)` at compile time:
  changes the tree that `Analyze`, location and error messages see.
- Special-casing `extension` in `eval/plan.go`, as `where` and `select` are:
  no public API change, but the function's logic would live in `eval` while
  its registration stays in `funcs`, and `fnExtension` would be dead code.

## Out of scope

- `Analyze` keeps typing `extension()` as unknown (`analyze.go:484`); giving
  it `Extension` is a separate change.
- `modifierExtension`: the function reads only items named `extension`.

## Tests (written before the change)

Each must fail at `38f8021` (checked before the change), cites §2.9.1.5
(R5 §2.1.9.1.5), and runs under R4, R4B and R5 models and without one.
Expected values are those of the `where` form the specification defines.

1. `X.extension(u) is Extension` and `X.extension(u).ofType(Extension)` agree
   with the `where` form on a resource, a complex element, a backbone element
   and a primitive's element (`_birthDate`).
2. `X.extension(u).url` is `uri`, as in the `where` form.
3. `.value` follows the model: R5 `valueCodeableReference` is returned; R4
   `valueInteger64` is not.
4. `X.extension(u).children()` types agree with the `where` form.
5. Without a model the result is unchanged (same items and types; both forms
   already agree there).
6. Arguments: empty input and empty url give empty (the third sentence of the
   definition); an URL no extension has gives empty; a multi-item argument
   follows the singleton rule for function arguments, pinned by a test with
   the result the three engines agree on.

Spec cases: the new requirements go to
`conformance/testdata/spec-cases/fhir-functions.xml` through `/fhirpath-cases`
and are measured against fhirpath.js and the HL7 Java engine (three-engine
rule).

## Validation with varied data

Metamorphic, over the official example packages (`hl7.fhir.r4.examples`
4.0.1, `hl7.fhir.r4b.examples` 4.3.0, `hl7.fhir.r5.examples` 5.0.0, corpusdiff
cache): about 100 distinct URLs and 7,000–11,000 (resource, URL) pairs per
version. For every pair, the `extension(u)` and `.extension.where(url = u)`
forms are compared on items, `.url`, `.value` and `.descendants()`, each
item with its `Type()`, from two kinds of input:

- `descendants()`, which places items through `TypedChildren`;
- the path expression to each JSON location holding extensions
  (`Patient.contact.extension(u)`), which places them through member
  navigation.

`maxCollectionSize` is raised for the comparison: `where` checks its input
size and `extension()` does not, and the limit is not part of the semantics.
Pairs where either side errors are counted and reported, not dropped
silently.

Oracle mutations before review: compare only counts; drop the type check;
drop the `.url` check; make `fnExtension` untyped again. Each must make the
comparison fail.

## Exit criteria

1. The tests above pass in R4, R4B and R5, and fail at `38f8021`.
2. The metamorphic comparison shows zero differences in all three versions,
   with error pairs reported.
3. The new spec cases agree with fhirpath.js and Java, or each divergence is
   adjudicated.
4. The official suite and spec cases show no new failures
   (`conformance` known-failure lists unchanged).
5. `make check` passes.

## Adversarial round

Independent reviewer, findings verified with a probe before accepting:

| Finding | Resolution |
| --- | --- |
| Values are lost or invented, not only untyped (R5 CodeableReference, R4 integer64) | Accepted, verified: diagnosis restated |
| Planned tests 2 and 3 (`value` of string/CodeableConcept/Quantity, nested) already pass today | Accepted: replaced by discriminating tests (`.url`, `ofType`, model-dependent `.value`, `children()`) |
| `getExtensionValue` reads untyped over a fixed suffix list | Accepted, verified (`valuePositiveInt` gives empty): separate issue |
| `where` hits `maxCollectionSize` on a 12 MB Bundle, a false difference | Accepted: limit raised, errors counted |
| `.value` alone is a weak oracle; `descendants()` never exercises member navigation | Accepted: `.url`, `.descendants()` added; path-expression inputs added |
| "Hundreds of URLs" is wrong (~100) | Accepted: counts restated |
| Citation lacks the empty-url sentence; no comparison against the other engines | Accepted: citation completed; spec cases and three-engine run added |
| Multi-item argument unpinned | Accepted: test 6 |
| A method on `eval.Context` is a public API addition; `plan.go` special case avoids it | Accepted as a cost, named in the approach; the method is kept for the reason given there |
| "Without a model unchanged" | Confirmed correct |
