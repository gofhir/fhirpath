# 003 — getExtensionValue(url) reads the value through the model

Status: done, pending commit
Issue: #218
Depends on: #217 (`eval.Context.Member`, typed `extension(url)`)

## Problem

`getExtensionValue(url)` returns an extension's value from a fixed list of 20
`value[x]` field names (`funcs/fhir.go:502` on #217), read untyped. Measured
on #217 with `gofhir/models` v2.10.0:

| Value | `getExtensionValue(u)` | `extension(u).value` |
| --- | --- | --- |
| `valueString` | `String a` | `string a` |
| `valuePositiveInt` | empty | `positiveInt 7` |
| `valueMarkdown` | empty | `markdown **x**` |
| `valueCodeableReference` (R5) | empty | `CodeableReference` |

`Extension.value[x]` allows 50 types in R4, 51 in R4B and 54 in R5
(`StructureDefinition-Extension.json` of each core package): the list covers
20, and is the kind of thing the charter's rule 5 forbids writing down.

## Prior review

- Not in FHIR's FHIRPath page (R4, R5) nor in FHIRPath 2.0.0 or 3.0.0: an
  extension of this engine. Its documented meaning (`docs/content/{en,es}/docs/
  functions/fhir-specific.md`): "Returns the value of extensions matching the
  given URL. Extracts the `value[x]` element from each matching extension."
- `gofhir/server` has no implementation of its own (it uses this library).
- No live repo of the platform calls it as a FHIRPath function: `validator`'s
  `getExtensionValueInfo` and `oncology-workstation`'s TypeScript
  `getExtensionValue` are unrelated identifiers. In this repo: three cases in
  `navigation_primitive_test.go` (an empty `valueCoding` object counts but does
  not exist), the docs, and the example page.
- fhirpath.js and the HL7 Java engine do not define it, so there is no
  reference to compare against; the reference is the expression it is
  documented to equal.

## Approach

Define it as `extension(url).value`: the items `extension(url)` returns
(typed and placed, #217), then the member `value` read through
`ctx.Member`, so `value[x]` resolves as navigation resolves it. With a model
that is the model's choice types, and the function keeps no list. Without
one it is navigation's fixed suffix list (`polymorphicTypeSuffixes`), which
lacks R4B's and R5's `CodeableReference` and R5's `Availability` and
`ExtendedContactDetail`: a known loss of navigation (#219), documented by a
test and not worked around here. The function stays, documented as
non-standard and equal to `extension(url).value`.

Argument handling comes with it from `fnExtension`: an empty url or empty
argument answers empty; a url of several items or no String ends the
evaluation (#217).

## Behaviour that changes, on purpose

Each measured on #217's head, the same under R4, R5 and no model:

- Every value type the model allows is found; the 30-odd missing ones
  answered empty.
- Values carry their FHIR type: `getExtensionValue(u) is String` was true and
  is now false; `is string` / `is FHIR.string` is the migration.
- An extension with only `_valueString: {"id": "i"}` gave nothing; it now
  gives that `string` element, which exists.
- `valueString: ["a", "b"]` (invalid FHIR) gave nothing; now two items.
- Two `value[x]` fields (invalid FHIR) gave the first in the list's order;
  now the first in the model's choice order (string and boolean give the
  boolean).
- `valueCoding: {}` is a `Coding`, no longer an `Object`; it still counts
  1 and does not exist.

Released as `fix:` (#217 typed `extension()` the same way), with a
"Behaviour change" paragraph in the commit, CONFORMANCE.md and both docs
pages. The list and the "first found, in the order listed" note go from
the docs.

Not this function's to fix, reported against navigation: a literal `"value"`
key (invalid FHIR) is read even under a model whose `Extension.value[x]` is a
choice (`String lit`), as `extension(u).value` reads it; added to #219.

## Tests (written before the change)

In `conformance` (runs in CI, no download):

1. Under R4, R4B and R5, a Patient carrying one extension per type of that
   version's `Extension.value[x]`, the types taken from
   `model.ChoiceTypes("Extension.value")` (50, 51, 54; equal to each core
   package's StructureDefinition), each with a real value of its type, not
   `{}`. For every one, `getExtensionValue(u)` gives exactly one item whose
   type is the choice type. The values are built per type from FHIR's
   primitive/complex split, with no per-version list.
2. Hand-written: `valuePositiveInt` gives `positiveInt 7`; R4B and R5
   `valueCodeableReference` found, R4 not; R5 `valueInteger64` found, R4
   not; `valueString` is a `string`.
3. Each behaviour change listed above, pinned.
4. A nested extension with extension children and no value gives empty; a
   primitive input (`birthDate`) reads its element's extensions;
   `modifierExtension` is not searched.
5. No model: R5's `valueCodeableReference` gives empty — the known loss of
   #219, so the test changes when navigation is fixed.
6. The three cases in `navigation_primitive_test.go` keep their results.
7. Arguments as `extension()`.

In `corpusdiff` (`make extension-shortcut`): the StructureDefinition's
`Extension.value[x]` list equals the model's `ChoiceTypes`, per version.

## Validation with varied data

An oracle that does not go through navigation: for every (input, URL) pair
over the official R4, R4B and R5 examples, read the matching extensions in
the raw JSON, take the `value[A-Z]…` key, and expect that value typed as its
suffix (first letter lowered), only if the suffix is in that version's
StructureDefinition list. Compare items, types and order with
`getExtensionValue(u)`. The examples carry few types beyond the old 20
(`valueMarkdown` 17–29 per 400 files, `valueExpression` 0–3), so the
generated resource of test 1 carries the rest.

Oracle mutations: put the fixed list back; read untyped; reverse the choice
order. Each must fail.

## Exit criteria

1. Tests 1–5 fail at #217's head (test 1 and 2 on content, not only the
   type label) and pass after.
2. Corpus comparison: zero differences in the three versions.
3. Docs (en, es) updated; CONFORMANCE.md notes the change.
4. `make check` passes.

## Adversarial round

Independent reviewer; findings 1, 2 and 4 verified with a probe before
accepting.

| Finding | Resolution |
| --- | --- |
| Without a model `.value` uses navigation's fixed suffix list, which lacks R4B/R5 types | Accepted, verified: known loss of #219, pinned by test 5; the rule-5 claim limited to the case with a model |
| Unlisted behaviour changes (`_valueString` alone, arrays, literal `value` key, two values, `{}` typing) | Accepted: listed and pinned; the literal key is navigation's, added to #219 |
| `is String` becomes false: compatibility | Accepted: `fix:` with a behaviour-change paragraph and migration, as #217 |
| Test 1 reading the package would be skipped in CI; corpusdiff's `fetch` has no SHA-256 pin | Accepted: test 1 from `ChoiceTypes` in conformance; an SD-vs-model check in corpusdiff; the missing pin reported separately |
| Comparing with `extension(u).value` is circular; the corpus barely covers new types | Accepted: raw-JSON oracle and generated resource; three oracle mutations |
| Tests barely discriminate on content; missing cases (nested, primitive input, integer64, modifierExtension) | Accepted: tests 2–4 |
| R5 `integer64` held as a Go `types.String` | Out of scope; checked separately before reporting |
| Keep the function or deprecate it | Kept, documented as non-standard and equal to `extension(url).value` |

## Record

- Approved by the user with the adversarial changes; the literal `value` key
  added to #219 as a comment.
- Tests first (`conformance/get_extension_value_test.go`): 190 checks failed
  at #217's head, on content (types outside the old list, the element-only
  value, arrays, two values, nested) and not only on the type label. The
  argument test passed already, inherited from #217. One hand-written
  expectation was wrong, not the engine: a primitive element's `id` is a
  `System.String`, as the core definitions type `Element.id`, and both forms
  answer so.
- Implementation: `fnGetExtensionValue` is `ctx.Member(extension(url),
  "value")`; the list is gone.
- The model's `ChoiceTypes("Extension.value")` equals each core package's
  `Extension.value[x]` (50, 51, 54): `TestTheModelAllowsTheExtensionValuesTheDefinitionDoes`.
- Raw-JSON oracle (`TestGetExtensionValueAnswersAsTheJSONHoldsIt`, in
  `make extension-shortcut`): each value as "type value", the value the
  JSON's own text (numbers kept as written) or an object canonically
  encoded, compared in order. Zero differences in R4 (11,931 pairs, 51,669
  values, 17 types), R4B (8,120, 27,924, 17) and R5 (10,986, 73,778, 20); it
  runs in seconds.
- Mutations, after review round 1 hardened both checks:

  | Mutation | Corpus oracle (R4 / R4B / R5) | conformance |
  | --- | --- | --- |
  | None (control) | 0 / 0 / 0 | pass |
  | The old fixed list | 11,805 / 8,047 / 10,102 | fail |
  | Untyped read, no list | 11,805 / 8,047 / 10,102 | fail |
  | Result reversed | 1,020 / 1,153 / 1,984 | fail |
  | Every item the first extension's value | 1,025 / 1,156 / 1,999 | fail |
  | unsignedInt, uuid, time, base64Binary and id values dropped | 0 / 0 / 0 | fail |
  | Choice order reversed in navigation | 0 / 0 / 0 | fail |

  The examples hold none of those five primitive types as extension values
  and no extension with two values; the generated per-type test and the
  conformance case `two` cover them.
- Docs (en, es) state the definition, the non-standard status, the url
  rules and the behaviour change with its migration; CONFORMANCE.md lists
  the gap closed.

### Code review

| Round | Findings | Resolution |
| --- | --- | --- |
| 1 | Major: the corpus oracle compared types and counts only, blind to order and to which extension a value came from (reversing the result: 1 difference per version). Major: the per-type test wrote primitives as elements only, so dropping real values of five types passed everything. Minor: arrays of nulls over-counted by the oracle; a docs comment still said Decimal/Integer | The oracle compares values, in order; the per-type test writes a real value of each primitive with its element and checks both (test data, failing on a primitive without a sample); the oracle counts only positions present; docs fixed. The oracle's first version also took a regex string starting with `[` for JSON; caught by its own run and fixed |
| 2 | No major; the round-1 fixes re-verified by mutation (and a new one: `uri`/`code` relabelled `String` with the right text, caught by both). Minor: nothing checked that a decimal keeps its text, 1.50 not 1.5: the per-type test also accepted the float rendering, and no extension in the examples holds a decimal with trailing zeros | The per-type test compares the value's text exactly; the corpus oracle does not exercise decimal formatting, which that test covers. Converged in round 2 |
