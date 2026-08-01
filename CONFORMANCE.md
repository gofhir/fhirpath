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

**747 of 928 executed cases (80.5%)**, measured against the official suite.

```sh
make conformance     # prints the number
go test -run TestOfficialSuite -v .   # per-case detail
```

The harness ([`conformance_test.go`](conformance_test.go)) runs the 937 cases of
[`FHIR/fhir-test-cases`](https://github.com/FHIR/fhir-test-cases), vendored under
[`testdata/fhirpath-suite`](testdata/fhirpath-suite). Nine cases are skipped
because two inputs have no published JSON equivalent; every skip is logged.

Cases that do not pass yet live in `known-failures.txt`. CI does not break over
pre-existing gaps, but it fails on a regression **and** when a listed case starts
passing — so the list can only shrink, and it never lies about the number.

## Remaining gaps

| Block | Cases | Notes |
|---|---|---|
| `lowBoundary` / `highBoundary` | 51 | Implemented, but computes the interval from the requested output precision instead of the input value's precision |
| Temporal comparison and precision | ~35 | Needs semantics decided: what yields empty vs false across precisions |
| `is()` and type hierarchy | 15 | Mostly resolvable only with a Model |
| Quantity formatting and conversion | 21 | Literal form is `1 'wk'`, not `1 wk`; `toQuantity` on a bare number should carry unit `'1'` |
| Errors we should raise and don't | 23 | 12 `execution`, 11 `semantic`; the semantic ones need a Model |
| `precision()` | 5 | Defined in 3.0.0, not implemented |

Defined in 3.0.0 and entirely absent here: `coalesce`, `defineVariable`,
`difference`, `duration`, `lastIndexOf`, `pathname`, `precision`, `repeatAll`.

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

## Decisions taken where the specification is silent

Recorded here and at the point of code, so nobody has to guess whether a choice
was reasoned or accidental.

| Question | Decision | Why |
|---|---|---|
| Max precision for `lowBoundary`/`highBoundary` | An implementation limit, documented in the function | 3.0.0 says explicitly: above "the maximum possible precision of the implementation", return empty. FHIR caps decimal at 18 digits |
| `sort` direction syntax | Accept both `desc`/`asc` and the leading `-` | 3.0.0 defines `desc`; the suite tests `-` and marks those cases as prototype |
| `type()` completeness | Emit `SimpleTypeInfo` and `ClassInfo`; omit `ClassInfo.element`, `ListTypeInfo`, `TupleTypeInfo` | They need element enumeration and declared cardinality, which the `Model` interface does not expose. Reading them off the instance would describe the value, not the type |
| `type()` results per element | One per input element | 3.0.0 states this, then contradicts it in its own `ListTypeInfo` example |
| `ofType()` on profiled subtypes without a Model | Keep structural inference | Nothing in the JSON distinguishes an `Age` from a `Quantity`; the information is in the model, not the document |
| `=` and `~` between two quantity objects | Complex-type semantics, not quantity semantics | Comparing complex types compares children; only the object-vs-literal case converts |

## Upstream issues

- **[`gofhir/ucum`](https://github.com/gofhir/ucum) exposes only `float64`.**
  Its internals already compute with `big.Rat`; the precision is lost at the
  public boundary (`1 mol/L → mmol/L` yields `1000.0000000000001`). Until it
  exposes an exact API, [`internal/ucum`](internal/ucum) stays — a hardcoded
  table of ~60 units with no dimensional analysis, in which `Cel` and `[degF]`
  share a canonical unit with factor 1, so `100 '[degF]' > 50 'Cel'` is silently
  `true`. An issue has been filed.
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
