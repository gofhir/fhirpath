# Conformance and specification notes

Where this engine stands against the FHIRPath specification, which specification
actually applies, and what is left. Written for whoever picks up this work next.

## Which specification applies

This is the single most useful thing to know before reading any FHIRPath
document, and it cost this project real time to discover: **three levels of
maturity coexist, and the test suite does not measure the one it names.**

| Document | What it is | Defines `lowBoundary`, `precision`, `sort`, `matchesFull`, `comparable` |
|---|---|---|
| [`spec/index.adoc`](https://github.com/HL7/FHIRPath/blob/master/spec/index.adoc) — **2.0.0** | The normative release | No |
| [`input/pages/index.md`](https://github.com/HL7/FHIRPath/blob/master/input/pages/index.md) — **3.0.0** | In development, marked Standard for Trial Use | **Yes, all of them** |
| Individual test cases | The R5 suite marks some `description="Prototype definition - not part of spec yet"` | `sort` is one of these |

The official suite declares `reference="http://hl7.org/fhirpath|2.0.0"` while
testing 3.0.0 behaviour. Reading only the normative 2.0.0 document makes it look
like the suite tests functions nobody defined — it does not, they are in 3.0.0.

Two further sources matter:

- **FHIR adds its own functions** on top of FHIRPath (`resolve`, `memberOf`,
  `conformsTo`, `hasValue`, …), listed on the FHIR [fhirpath.html](https://hl7.org/fhir/fhirpath.html)
  page, not in the FHIRPath specification.
- **The grammar is a separate artifact** from the prose, and the two disagree:
  the String section lists `\"` as a valid escape, while the `ESC` rule in
  [`spec/N1/fhirpath.g4`](https://github.com/HL7/FHIRPath/blob/master/spec/N1/fhirpath.g4)
  omits it. The suite follows the prose.

## Current conformance

Measured against the official suite, in both configurations a caller can use:

| Configuration | Passing |
|---|---|
| No FHIR model supplied | **892 of 928 (96.1%)** |
| With the R4 model | **909 of 928 (98.0%)** |

```sh
make conformance          # prints both numbers
make conformance-update   # re-baseline after closing a gap
```

The harness ([`conformance/`](conformance)) runs the 937 cases of
[`FHIR/fhir-test-cases`](https://github.com/FHIR/fhir-test-cases), vendored under
[`conformance/testdata/fhirpath-suite`](conformance/testdata/fhirpath-suite).
Nine cases are skipped because two inputs have no published JSON equivalent;
every skip is logged.

It is a **separate Go module**, so the FHIR model packages it needs to measure
the model-aware run stay out of the engine's own dependency graph.

A model is worth less here than expected — four cases — and the reason is
instructive: most remaining `is()`/`as()` failures are not about hierarchy but
about FHIR primitives being distinct types from their System counterparts
(`Patient.gender.as(string)` must yield empty because gender is a `code`), and
three more fail on an input discrepancy rather than on evaluation.

Cases that do not pass yet live in `known-failures.txt` and
`known-failures-model.txt`, one baseline per configuration. CI does not break over
pre-existing gaps, but it fails on a regression **and** when a listed case starts
passing — so the list can only shrink, and it never lies about the number.

## Remaining gaps

| Block | Cases | Notes |
|---|---|---|
| `as()` on a multi-item collection | 1 | Applies from R5; the suite is R4 — see below |
| Errors we should raise and don't | 8 | All `semantic`: they need compile-time typing, which this engine does not do. fhirpath.js does not raise them either |
| `lowBoundary` / `highBoundary` | 8 | Three are a suite disagreement (see below); two assume `@2014-01-01T08` carries an implicit minute |

`repeat()` was registered but never implemented — it returned its input
unchanged, so every expression that relied on it answered without erroring. That
is worth the three `testRepeat` cases, and it is normative 2.0.0 rather than a
3.0.0 addition, which is the kind of gap a compile-only corpus cannot see.

Of the functions 3.0.0 adds, all are now implemented except `pathname`:
`coalesce`, `defineVariable`, `difference`, `duration`, `lastIndexOf` and
`repeatAll`. None of them appears in the R4 suite, so they move the count not at
all; they were implemented for completeness against the language, and are
covered by tests written from the specification's own examples.

`pathname` is left out deliberately. It returns the path at which each item sits
within the input resource, which requires every value to carry where it came
from — infrastructure this engine does not have, and a cost paid on every
navigation whether or not anyone calls the function. It is worth doing when
something needs it, not before.

Five boundary cases are left failing, and running them through fhirpath.js
settles why. Three are decimal: `(-0.0034).lowBoundary(1)` expects `-0.0`, where
rounding down to one digit gives `-0.1`. fhirpath.js answers `-0.1` as well —
its implementation is the same algorithm, ROUND_FLOOR against a half-unit taken
from the decimal places — and it matches the specification's own worked examples,
`(-1.587).lowBoundary(2) // -1.59` among them. The other two are temporal:
`@2014-01-01T08.highBoundary(17)` is expected to be `08:00:59.999`, leaving the
minute at zero while maximizing the second. fhirpath.js gives `08:59:59.999`, as
this engine does, which is the greatest value the hour can hold.

Where the suite and the specification's examples disagree about timezones, the
suite is followed: it expects `@2014-01-01T08.lowBoundary(17)` to carry `+14:00`,
the offset that makes the instant earliest, while the specification's example
shows no offset at all. The suite's reading is the stricter one — a boundary
should bound — and fhirpath.js omits the offset, so it fails that case.

## Rules the FHIRPath suite does not reach

Two defects came out of evaluating what gofhir/ucum's `fhir` package offers, and
neither is visible to the conformance suite — the first because it is a FHIR
rule rather than a FHIRPath one, the second because no case in the suite lands
on it.

**A FHIR Quantity did not map onto FHIRPath's calendar units.** FHIR R5, "Using
FHIRPath with FHIR", requires that when a Quantity carrying a UCUM code is
evaluated as a `System.Quantity`, its time-valued codes become calendar
keywords: `a`→year, `mo`→month, `d`→day, `h`→hour, `min`→minute, `s`→second.
Without it, `Patient.birthDate + Observation.value` failed on data FHIR
considers well formed: the code stayed `'a'`, a definite 365.25 days, which
cannot be added to a calendar and is required to raise an error. The mapping is
what makes the value usable, and it is conditioned on the quantity declaring
UCUM as its system — so without that system the value still, correctly, errors.

**Calendar arithmetic normalized instead of clamping.** Adding a year to
`@2016-02-29` gave `2017-03-01`, and a month to `@2014-01-31` gave `2014-03-03`.
The specification says the opposite, in both the year and the month rows of its
table: "If the month and day of the date or time value is not a valid date in
the resulting year, the last day of the calendar month is used." The answers are
now `2017-02-28` and `2014-02-28`. Go's `AddDate` normalizes, which is the
behaviour that was inherited by writing the shift in terms of it.

## Type specifiers

A type specifier "must resolve to the name of a type in a model", so a name that
resolves to nothing is an error rather than a filter that matches nothing. The
engine reads this through the optional `TypeRegistry` interface, which the
published models implement from r4/v1.4.0 — `conformance/version_test.go` checks
all three, including the root types `Element` and `Resource` that no other method
on `Model` can recognize, since they have no parent.

Worth noting that fhirpath.js does not validate type specifiers at all — its
`TypeInfo.isType` walks the hierarchy comparing names, so an unknown name simply
matches nothing. This is a point where the specification and the conformance
suite are explicit and the reference implementation is not.

## What was wrong, and why nothing caught it

Every one of these produced an answer rather than an error, which is why a corpus
that only checked "does it compile" reported success:

- **A `Reference` swallowed its own field.** Any object with a `reference` field
  was inferred as type `Reference`, and type-name navigation matched the
  identifier case-insensitively, so `reference` returned the enclosing object.
  `startsWith('#')` was therefore always false and the FHIR invariant `ref-1`
  could never fail. Fixed in v1.2.0; the regression test that should have locked
  it did not exist until now.
- **`=` never compared collections.** It required singletons and returned empty
  otherwise. Worth 41 suite cases on its own.
- **Singleton-to-Boolean coercion was missing**, so `age-1` and `drt-1` — which
  open with `(code or value.empty())` — evaluated to empty instead of true.
- **Quantities carried as JSON objects could not be compared**, which is every
  quantity in FHIR. `rng-2` failed outright.
- **`ToQuantity` preferred `unit` over `code`**, comparing against a display
  string like `"milligram"` that no unit conversion can interpret.
- **The parser's grammar source was not in the repository.** The generated lexer
  named `grammar/fhirpath.g4`; that file did not exist. The parser had been
  generated from an older grammar whose `STRING` rule rejected `\"`, and whose
  operator precedence was wrong.
- **String escapes were resolved by successive replacement**, so `'\\n'` — a
  backslash and an n — came out as a line feed.
- **Shifting a date by a duration it did not recognize returned the date
  unchanged.** Only calendar keywords were understood, so `@1973-12-25 + 1 'd'`
  quietly produced `@1973-12-25`.
- **Comparing temporals of different precisions raised an error**, failing the
  whole evaluation where the spec asks only for empty.
- **Units came from a table of about sixty codes with no dimensional analysis.**
  `Cel` and `[degF]` shared a canonical unit with a factor of one, so
  `100 '[degF]' > 50 'Cel'` was silently `true` — 100 °F is 37.8 °C. Compound
  units such as `mg/kg/d` were simply unknown.

## A divergence taken on purpose: `as` over a collection

The specification is unambiguous about the operator: "If there is more than one
item in the input collection, the evaluator will throw an error." The official
suite tests it (`Patient.name.as(HumanName).use`), and this engine does not
comply — it filters by type, as `ofType()` does.

That is deliberate, and the reason is FHIR's own normative content. `dom-3`, a
SHALL invariant on **every** DomainResource, reads:

    contained.where(('#'+id in (%resource.descendants().reference
                              | %resource.descendants().as(canonical)
                              | %resource.descendants().as(uri)
                              | %resource.descendants().as(url))) ...

`descendants()` returns hundreds of items, so under the specification's rule
dom-3 raises an error on every resource it is applied to. FHIR publishes an
invariant its own base language forbids, and an engine has to pick a side.

This is not a lenient reading of an ambiguous rule. Checked against fhirpath.js
5.1.0, the reference implementation, which follows the specification exactly:

    dom-3 as published for R4  ->  Error: Expected singleton on left side of 'as'
    dom-3 as published for R5  ->  [true]

So a specification-exact engine cannot validate dom-3 on R4 at all, and R4 is
what most deployments run.

HAPI, the engine behind the official validator, resolves it by version:

```java
private void initFlags() {
  if (!VersionUtilities.isR5Plus(worker.getVersion())) {
    doNotEnforceAsCaseSensitive = true;
    doNotEnforceAsSingletonRule = true;
  }
}
```

Before R5 it filters, exactly as this engine does; from R5 on it enforces the
singleton rule. So the behaviour here matches the reference validator for R4 —
which is the version the suite and most deployments use — and would need to
become version-aware to match it for R5 as well. HAPI also enforces case
sensitivity on type names only from R5 on.

HL7 came to the same conclusion, in stages. The invariant is worded differently
in each of the three versions:

| | guard | operator | `where(... = '#')` clauses |
|---|---|---|---|
| R4 | — | `as()` | canonical, canonical — duplicated |
| R4B | `id.exists()` | `as()` | canonical, uri — fixed |
| R5 | — | `ofType()` | canonical, canonical — reintroduced |

So R4B fixed a copy-paste defect and added a guard, and R5 switched to the
filtering function while bringing the defect back. An engine has to evaluate
whichever wording the resource's version publishes; `version_aware_test.go` runs
all three against every model configuration.

So the divergence is only needed for **R4** invariants. An engine targeting R5
could follow the specification and still evaluate dom-3. This one sides with the
R4 invariant, since that is the version most deployments validate against, and
one suite case is a smaller loss than dom-3 erroring on every DomainResource.

Worth noting while reading dom-3: one clause is duplicated in both versions —
`descendants().where(ofType(canonical) = '#').exists()` appears twice, where the
symmetry with the first branch calls for `uri` and `url`. That defect is
upstream and unrelated to this engine.

## Decisions taken where the specification is silent

Recorded here and at the point of code, so nobody has to guess whether a choice
was reasoned or accidental.

| Question | Decision | Why |
|---|---|---|
| Max precision for `lowBoundary`/`highBoundary` | An implementation limit, documented in the function | 3.0.0 says explicitly: above "the maximum possible precision of the implementation", return empty. FHIR caps decimal at 18 digits |
| `sort` direction syntax | Accept both `desc`/`asc` and the leading `-` | 3.0.0 defines `desc`; the suite tests `-` and marks those cases as prototype |
| `type()` completeness | Emit `SimpleTypeInfo` and `ClassInfo`; omit `ClassInfo.element`, `ListTypeInfo`, `TupleTypeInfo` | They need element enumeration and declared cardinality, which the `Model` interface does not expose. Reading them off the instance would describe the value, not the type |
| `type()` results per element | One per input element | 3.0.0 states this, then contradicts it in its own `ListTypeInfo` example |
| Temporal comparison across precisions | Component by component, stopping at the first difference; empty only when everything shared matches | Not a judgement call — the spec states it, and it is why `now() > today()` is empty while `now() > @1974-12-25` is true |
| `ofType()` on profiled subtypes without a Model | Keep structural inference | Nothing in the JSON distinguishes an `Age` from a `Quantity`; the information is in the model, not the document |
| `=` and `~` between two quantity objects | Complex-type semantics, not quantity semantics | Comparing complex types compares children; only the object-vs-literal case converts |
| `as()`/`ofType()` on a primitive | Requires the declared type; `is()` keeps hierarchy matching | FHIR: "all primitives are considered to be independent types (so markdown is not a subclass of string)" |
| Telling `FHIR.boolean` from `System.Boolean` | By case: FHIR names primitives in lower camel case, FHIRPath capitalizes its own | The suite requires `active.is(boolean)` true and `active.is(Boolean)` false. Complex types such as `Quantity` are spelled alike in both namespaces, so the rule applies to primitives only |
| Type of a polymorphic element without a model | Taken from the field name, corrected to FHIR's casing | `valueOid` states the element is an `oid`; that is information in the document, not a guess. The value itself says whether the type is primitive or complex |

## Upstream issues

- **Resolved.** `gofhir/ucum` exposed only `float64`, so an issue was filed
  asking for exact arithmetic. `v2.2.0` added `ExactService` — `ConversionFactor`
  returning a `*big.Rat`, `ConvertRat` covering the affine temperature scales,
  and `ErrNotLinear`/`ErrNotRational` for the cases where no exact result exists
  — plus `Divide` for unit algebra. The engine now depends on it and the
  hardcoded unit table is gone.
- **`gofhir/models` v1.0.0 ships no public packages** and its internals import
  another module's `internal/`, so it does not build. The usable models are the
  independently versioned submodules `models/r4`, `models/r4b`, `models/r5`
  (v1.2.0). Note `FHIRPathModelData` is the struct type; the accessor is
  `FHIRPathModel()`.

## Working on this

```sh
make generate        # regenerate the parser after editing grammar/fhirpath.g4
make generate-check  # fails if the committed parser doesn't match the grammar
make conformance     # current conformance number
make test-race
```

The grammar is the source of truth for the parser and is versioned at
[`grammar/fhirpath.g4`](grammar/fhirpath.g4). Never hand-edit `parser/grammar`;
CI verifies the two agree.

After closing a gap, re-baseline and review the diff — lines disappearing is
progress, lines appearing is a regression:

```sh
go test -run TestOfficialSuite -update-known-failures .
```
