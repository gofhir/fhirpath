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
| R4 suite, no FHIR model supplied | **900 of 935 (96.3%)** |
| R4 suite, with the R4 model | **927 of 935 (99.1%)** |
| R5 suite, no FHIR model supplied | **1007 of 1048 (96.1%)** |
| R5 suite, with the R5 model | **1034 of 1048 (98.7%)** |

```sh
make conformance          # prints both numbers
make conformance-update   # re-baseline after closing a gap
```

The harness ([`conformance/`](conformance)) runs
[`FHIR/fhir-test-cases`](https://github.com/FHIR/fhir-test-cases), vendored under
[`conformance/testdata/`](conformance/testdata) together with the input resources
the suite was written against — all 935 R4 cases execute, and 1048 of the 1051
R5 ones. The three that do not are `ccda.xml`, a CDA document rather than a FHIR
resource; every skip is logged with its reason.

It is a **separate Go module**, so the FHIR model packages it needs to measure
the model-aware run stay out of the engine's own dependency graph. Reading the
suite's XML inputs uses those same packages, which is why it belongs here and not
in the engine: converting FHIR XML correctly needs the cardinality and primitive
type of every element, and the engine deliberately carries no model of its own.

A model is worth less here than expected — 27 cases across the R4 corpus — and
the reason is instructive: most remaining `is()`/`as()` failures are not about
hierarchy but about FHIR primitives being distinct types from their System
counterparts (`Patient.gender.as(string)` must yield empty because gender is a
`code`).

Cases that do not pass yet live in `known-failures.txt` and
`known-failures-model.txt`, one baseline per configuration. CI does not break over
pre-existing gaps, but it fails on a regression **and** when a listed case starts
passing — so the list can only shrink, and it never lies about the number.

## Remaining gaps

| Block | Cases | Notes |
|---|---|---|
| `as()` on a multi-item collection | 1 | Applies from R5; the suite is R4 — see below |
| `htmlChecks()` | 4 (R5) | Needs an XHTML parser and FHIR's element list; one of the four also needs `div` read as an identifier — see below |
| `%terminologies` | 3 (R5) | Needs a terminology server answering, not a stub — see below |
| `lowBoundary` / `highBoundary` | 5 | Three are a suite disagreement (see below); two assume `@2014-01-01T08` carries an implicit minute |

`repeat()` was registered but never implemented — it returned its input
unchanged, so every expression that relied on it answered without erroring. That
is worth the three `testRepeat` cases, and it is normative 2.0.0 rather than a
3.0.0 addition, which is the kind of gap a compile-only corpus cannot see.

Of the functions 3.0.0 adds, all are now implemented except `pathname`:
`coalesce`, `comparable`, `defineVariable`, `difference`, `duration`,
`lastIndexOf`, `repeatAll`, and the ten component extractors `yearOf`,
`monthOf`, `dayOf`, `hourOf`, `minuteOf`, `secondOf`, `millisecondOf`,
`timezoneOffsetOf`, `dateOf` and `timeOf`. None of them appears in the R4
suite, so they move the count not at all; they were implemented for
completeness against the language, and are covered by tests written from the
specification's own examples.

The extractors carry their `Of` for a reason. `year`, `month`, `day`, `hour`,
`minute` and `second` are calendar units in the grammar, so `birthDate.month()`
does not parse — the identifier after the dot is read as the unit. This engine
had registered those bare names, which left the functions reachable only as
``birthDate.`month`()``, and nobody writes that. The 3.0.0 names are the ones
that can be called; the older spellings are kept and share the same
implementations, so the two cannot drift apart.

Writing them settled two questions the older ones had answered differently. A
component the value does not carry is empty rather than zero — `@2012-01-01`
has no hour, and answering `0` would say midnight — and that cannot be read off
the precision alone, because `@2012T12:30:00` carries a time without a day. And
a collection of more than one item is an error, which the specification states
for every one of them.

`toLong` and `convertsToLong` are left out too, for a reason of their own: this
engine's `Integer` is already 64 bits wide, which is what `Long` exists to
provide. That divergence has its own section below.

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

### Seven R5 cases that need something outside the engine

Four `HTMLChecks` cases and three `TerminologyTests` cases fail in the R5 suite
for reasons that have nothing to do with the language.

They are listed as known failures rather than skipped, because they run: the
engine reaches them, answers, and answers wrongly. That is different from the
three `ccda.xml` cases, which have no input the harness can read and are
reported separately, outside the denominator — there is nothing to grade. A case
that runs and fails belongs in the number.

**`htmlChecks()`** validates that a `Narrative.div` holds XHTML FHIR permits —
the element list, no scripts, no external references. It is not implemented: it
needs an XHTML parser and FHIR's own list of what is allowed, neither of which
belongs in an expression engine that has no other reason to read markup.

One of the four runs into something else first. `htmlTest01` asks for
`text.div.htmlChecks()`, and that does not parse at all:

    text.div        // parser errors: mismatched input 'div'
    text.`div`      // [<div xmlns="http://www.w3.org/1999/xhtml">…]

`div` is integer division in FHIRPath, and the grammar admits only `as`,
`contains`, `in` and `is` as identifiers — every other keyword has to be
written as a delimited identifier. So the suite writes an expression its own
grammar does not accept, and an engine that follows the grammar cannot read it.
Navigating to a narrative works, it just takes the backticks the grammar asks
for. Whether to accept more keywords as identifiers than the grammar lists is a
question worth its own answer, and not one to settle by loosening the parser
until a test passes.

The other three read a `Parameters` resource instead —
`%resource.parameter.where(name='goodHtml').value.htmlChecks()` — which parses
cleanly and fails only for the missing function. So `div` accounts for one case,
not four, and implementing `htmlChecks()` alone would close three of them.

**`%terminologies`** is the environment variable FHIR defines for reaching a
terminology server: `expand`, `validateVS`, `translate` and the rest. It is
undefined here.

    %terminologies.expand('http://hl7.org/fhir/ValueSet/administrative-gender')
    // undefined variable: %terminologies

The engine does carry a `TerminologyService` interface, which is what
`memberOf` calls when a caller supplies one. `%terminologies` is a larger
surface — it answers with whole resources for expressions to navigate, and the
suite's three cases expect the contents of real value sets and concept maps, so
passing them means a server answering, not a stub. Worth doing when a caller
needs it, with the service the caller already has.

### An empty regex in `replaceMatches`

    'abc'.replaceMatches('', 'x')

The suite expects `abc`. This engine and fhirpath.js 5.1.0 both give `xaxbxcx`,
and the specification supports them from two directions.

`replace`, the sibling function, states the case outright: "If `pattern` is the
empty string (`''`), every character in the input string is surrounded by the
substitution, e.g. `'abc'.replace('','x')` becomes `'xaxbxcx'`". `replaceMatches`
carries no special rule for an empty regex, only the shared one — "if the input
collection, `regex`, or `substitution` are empty, the result is empty" — and
`''` is not empty in that sense. It is a collection holding one empty string,
which is exactly the distinction `replace` draws by naming the two cases apart.

What settles it is that the expected value answers to neither reading. Were `''`
treated as empty the result would be `{ }`; were it not, an empty pattern matches
at every position and the result is `xaxbxcx`. `abc` follows from neither, so the
case is listed as a known failure.

## Measured against another engine

The suite says what an expression should answer. A second engine says what
another reading of the same specification arrives at, which is a different
question and the one that settled several entries in this file — each run
through fhirpath.js by hand at the time.

    make difftest                          # both corpora
    make difftest DIFFTEST_ARGS="-v"       # every divergence, not a sample

It reports four kinds, and the order is the order worth reading them in: cases
this engine gets wrong by the suite's reckoning, cases where both engines agree
against the suite, cases fhirpath.js gets wrong, and cases the suite states no
result for.

Against fhirpath.js 5.2.0, with each engine given its model:

| | R4 | R5 |
|---|---|---|
| Cases compared | 915 | 1014 |
| Same answer | 881 (96.3%) | 980 (96.6%) |
| **This engine wrong, fhirpath.js right** | **1** | **0** |
| Both agree, suite disagrees | 6 | 13 |
| fhirpath.js wrong, this engine right | 30 | 32 |
| Neither matches, and they differ | 3 | 2 |

The one case where fhirpath.js is right and this engine is not is `as()` over a
collection, a divergence taken on purpose and recorded below.

Not every case can be compared. Those asserting a semantic fault are decided by
static analysis, which this harness does not run — `make conformance` does — and
those where fhirpath.js has no such function say nothing about the expression by
refusing. Both are counted out rather than scored, which is why the totals here
are smaller than the suite's.

Most of what this engine answers and fhirpath.js does not is a function it has
not implemented: `precision`, `conformsTo`, boundaries over a Quantity. The
reverse cases are worth more attention, and the run is how they are found rather
than stumbled into.

Two of the R5 groups documented above as needing something outside the engine —
`htmlChecks` and `%terminologies` — fail in fhirpath.js too, which is some
comfort about how far outside they are.

## Static analysis

A semantic fault is one that evaluation cannot see. `Patient.name.given1`
evaluates to empty against a document that has no such value — the right answer
to the question asked — and is still wrong, because `given1` is not an element
of `HumanName` in any document. A typo in an invariant behaves exactly like a
constraint that happens not to fire.

`Expression.Analyze(model, contextType)` reports these before evaluation. It
covers three rules: navigation the model contradicts, a positional read of a
collection with no defined order (`children().skip(1)`), and an `iif` criterion
that cannot be a Boolean. It is opt-in and separate from evaluation, which stays
lenient — the suite has cases that assert both readings of
`Observation.valueQuantity.exists()`, one as a semantic error and one under
`mode="lenient/polymorphics"` expecting an answer.

The analysis is conservative by design: it reports only what the model makes
certain and stops following a branch as soon as the type becomes unknown. A
false positive rejects an expression that works, which is worse than missing a
fault — the analysis exists to catch typos, not to become one. Silence means
"nothing provably wrong", not "correct".

fhirpath.js does not do this, so it is a second point where the engine goes
past the reference implementation while following the specification.

## A correction neither suite covers

A prefix on a unit that sits on an affine scale — `mCel`, `kCel`, `cCel` —
multiplies the argument of the scale's function, not its result, per UCUM §22.4.
A milli-Celsius is a thousandth of a degree, not a thousandth of the distance
from absolute zero.

Through gofhir/ucum v4.1.0 the engine answered `1 'mCel'` as `-272.87585 'Cel'`,
`1 'kCel'` as `273876.85`, and `1 'cCel'` in kelvin as `2.7415`. The right
answers are `0.001`, `1000` and `273.16`. Fixed upstream in v4.2.0.

Neither conformance suite prefixes a special unit, so the measurement never saw
this and never would have. `TestPrefixOnASpecialUnit` pins it instead. It is
worth recording as the shape of a gap the suites leave: they cover the units
that appear in FHIR data, and a unit that is merely legal UCUM can still be
wrong everywhere.

## Reading the suite's own inputs

The harness used to run against the examples hl7.org publishes, because the suite
names its inputs as `.xml` and this engine consumes JSON. That cost twice, and
both costs were invisible in the percentage.

Resources hl7.org never published as JSON could not be run at all: 7 R4 cases and
11 R5 ones were skipped, and a skipped case is not a passing case but does not
lower the number either. And the copies it did publish had drifted from the
suite's, so four more cases measured a different resource than their expected
result was written for — `codesystem-example` had lost the nested concepts
`testCombine1` counts duplicates among, and R5's `valueset-example-expansion`
had moved its `version` from `20150622` to `5.0.0`, which two cases assert.

The inputs are now the suite's own files, converted on load through
`gofhir/models`. That is not a generic XML-to-JSON mapping and could not be:
deciding that a lone `<name>` element is a collection of one, or that
`value="true"` is a boolean rather than the string `"true"`, takes the
cardinality and type of every element. The generated types carry both, which is
also why this belongs to the conformance module rather than the engine.

Two things came out of it that the old inputs had been hiding:

- **A real defect.** `testTypeA4` asserts that a `valueUuid` `is(FHIR.uri)`, and
  it had never run. Writing the namespace turned out to disable hierarchy
  matching altogether — `Patient.is(FHIR.DomainResource)` was false while
  `Patient.is(DomainResource)` was true. Fixed; see the test in
  [`eval/type_namespace_test.go`](eval/type_namespace_test.go).
- **A limit of the no-model run**, not a defect. `testPeriodInvariantNew` reads
  `Period.start.lowBoundary() < Period.end.highBoundary()` where `start` is
  `2001-05-06` and `end` is `2001-05-06T10:10:10Z`. Without a model `start` is a
  date, its low boundary is the same date, and comparing day precision against
  millisecond precision when everything shared matches is empty — the rule stated
  below. With the model it knows `Period.start` is a `dateTime` and answers true.
  It is listed in the no-model baseline alone, which is what the two baselines are
  for.

## Where the two suites disagree

One case appears in both corpora with the same expression and different expected
results, and it is worth recording because the disagreement is HL7's own.

    @1973-12-25T00:00:00.000+10:00 + 0.1 's'

The R4 suite expects the fractional second to be dropped; the R5 suite expects
`.100`. FHIRPath N1 — the version the R4 suite measures against — says "For
precisions **above** seconds, the decimal portion of the time-valued quantity is
ignored", which leaves the decimal in place at the level of seconds, and 3.0.0
states the same rule the other way round: "only applied for second or
millisecond precisions".

Both specifications therefore call for `.100`, and the R5 suite was corrected to
match. The R4 case is listed as a known failure rather than special-cased: an
engine that dropped the decimal would be wrong under either specification, and
the fix is worth five other cases in the R5 corpus.

### conformsTo, where R5 disagrees with itself

R4 says an unresolvable profile is an error; R5 says the result is empty. The
engine follows the version the model declares, which is what the two
specifications ask for.

The R5 suite, however, marks `conformsTo('http://trash')` as
`invalid="execution"` — an error, the R4 reading. So the R5 case fails here and
is listed as a known failure. Following its own prose over its own suite is the
same call made everywhere else in this file: the specification is the authority,
the suite is evidence about it, and where they part company the reasoning is
recorded rather than the number optimised.

This one differs from the `+ 0.1 's'` case above in a way worth keeping straight.
There, both specifications agreed and one suite was stale, so the suite was the
thing to disregard. Here the two specifications disagree with each other, and
the engine implements both — the disagreement is real, not a lag.

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

## A default timezone offset is the caller's policy, not the engine's

Comparing a `DateTime` that writes a timezone offset with one that does not has
no answer here, and yields empty. The specification permits either behaviour and
says so in as many words:

> For DateTime values that do not have a timezone offsets, whether or not to
> provide a default timezone offset is a policy decision. In the simplest case,
> no default timezone offset is provided, but some implementations may use the
> client's or the evaluating system's timezone offset.

An engine that supplied the local server's offset would therefore be conforming.
This one does not, for a reason the word *policy* points at: a policy belongs to
the environment doing the evaluating, and an engine that picks one on its own
makes the same invariant answer differently on two machines. `dom-6` cannot
depend on which region a process runs in.

What a caller can do is state the offset itself, per value, which is what
`WithDefaultOffset` below is for. What the engine will not do is choose one.

FHIR requires the offset where the comparison would otherwise be undecidable,
which is why this stayed open until a caller arrived with a case it does not
cover:

| Type | Rule |
|---|---|
| `dateTime` | "If hours and minutes are specified, a timezone offset **SHALL** be populated" |
| `instant` | "SHALL include a timezone offset" |
| `time` | "A timezone offset **SHALL NOT** be present" |

So two FHIR `dateTime` values carrying a time both carry an offset, and the
undecidable case cannot arise between them. It arises when one side is a literal
written without one — `@2021-01-01T00:00:00.0` in an expression — or when the
data is invalid for FHIR. In the first case the literal is what to fix; in the
second, the data.

The specification expects to revisit this: "Additional functions to support more
sophisticated timezone offset comparison (such as .toUTC()) may be defined in a
future version."

### Saying what a bare value's offset is

A caller whose own language settles the question can say so, per value:

    period, _ := types.NewDateTime("2020-01-01T00:00:00.0")
    encounter.Compare(period.WithDefaultOffset(0))   // answers, rather than declining

`DateTime.WithDefaultOffset` supplies the offset to assume when the value states
none, and leaves a stated one alone. Nothing applies it on its own, so a
FHIRPath caller keeps seeing empty.

It is supplied to the value, not to a comparison, because that is where the
language that defines it applies it. CQL: "If no timezone offset is supplied,
the timezone offset of the evaluation request timestamp is assumed" — at
construction, which is why extracting the offset from such a value "will be the
timezone offset of the evaluation request, not null".

**The default is remembered apart from what was written.** `String` and `HasTZ`
answer about the value as written, so a literal written without an offset still
evaluates to itself; `EffectiveOffset` answers what to place it at. Both are
needed at once, and a first attempt that wrote the default into the value's own
offset showed why: a CQL engine trying it against its conformance corpus lost
twenty-two cases, because `@2012-03-10T10:20:00` began printing as
`@2012-03-10T10:20:00-05:00` and every rule that reads a value's representation
— precision, boundaries, interval ends — read the default instead.

Ordering and duration both read `EffectiveOffset`, which is what makes them
agree. They do not agree without one, and that split predates the option:

    @2020-01-01T02:00:00Z > @2020-01-01T00:00:00.0                     // { }
    @2020-01-01T02:00:00Z.difference(@2020-01-01T00:00:00.0, 'hour')   // -2

`difference` and `duration` place a bare value at UTC and measure from there,
where the comparison declines to place it at all. Supplying a default settles
both the same way, so the split matters only to callers who do not supply one —
for whom it stays as it is, an inconsistency recorded rather than changed on the
way past.

The offset is a `time.Duration` so that the unit is in the call rather than in
the documentation: `-5 * time.Hour` cannot be read as five minutes, and no bound
could have told the two apart, since both are offsets a timezone can have. Only
whole minutes within the fourteen hours FHIR and XML Schema allow are accepted;
anything else leaves the value as it was, so the comparison declines rather than
answering from an invented instant. So does a value whose precision stops above
a time of day: `@2020` has no instant to place.

#### What a default reaches

An operator consults the default when an operand lacks a stated offset. Where
both operands lack one **and were given the same default** — which is what a
caller applying one evaluation-request offset to every bare value produces — the
two are shifted equally and the shift cancels, so the answer does not move.

The qualifier matters here in a way it does not for such a caller. A default is
supplied per value, so two bare values can carry different ones, and then
nothing cancels:

    a := bare("2020-06-15T12:00:00.0").WithDefaultOffset(-5 * time.Hour)
    b := bare("2020-06-15T12:00:00.0").WithDefaultOffset(9 * time.Hour)
    // hours between them: -14, not 0

Whether that is a sensible thing to do is the caller's business; that it is
possible is why the cancellation is a property of how the defaults were
supplied, not of the operators.

The cancellation is also why this needed measuring rather than reasoning. The
CQL engine that asked for the feature mapped its own operators at three request
offsets and got the case selection wrong twice before the rule fell out —
first with values at midday, where nine hours never cross a date, and then with
interval bounds months apart, where no offset changes containment. Both looked
like properties of the operators and were properties of the examples.

Which operators, here:

| | |
|---|---|
| Consult it | ordering, equality and equivalence, membership and union, `difference`, `duration`, the boundary functions, arithmetic, and `timezoneOffsetOf` — everything that places a value on a clock, or carries a placed value forward |
| Never consult it | `toString`, `dateOf`, `timeOf`, `hourOf` and the rest of the component extractors — everything that reads the value's own digits |

Three of those reached the first row late, and all three were found by checking
this engine against the table rather than by reading the code: arithmetic
rebuilt its result without the default, so a shifted value could no longer be
placed and its durations silently measured from elsewhere; equality consulted
the default while equivalence, `in` and `|` did not, so two values that `=`
called equal collapsed to two items under a union; and the boundary functions
gave a placed value the full 26-hour span of every offset it might have had,
while `timezoneOffsetOf` on the same value named one.

The second row is the point of not writing the default into the value. A value
written `@2020-06-15T23:00:00.0` still reports hour 23 and date 2020-06-15
whatever offset was supplied for it, while an ordering against a stated offset
moves.

`timezoneOffsetOf` sits in the first row deliberately, and did not at first: it
read the stated offset alone, so a value that ordering placed perfectly well
answered empty when asked where it sat. The language that asks for defaults
requires the opposite — extracting the offset from such a value gives the
offset, "not null" — and a value that means one thing to ordering and another to
extraction is two values.

A value with no time of day is reached by none of it. `@2020-01-01` has no
instant to place, so it is compared as written; a comparison against a DateTime
is decided by precision, as it was before defaults existed.

This is not hypothetical. Every published eCQM library declares its measurement
period without an offset — `Interval[@2019-01-01T00:00:00.0, @2020-01-01T00:00:00.0)`
— while FHIR requires one on any dateTime carrying a time, so served data always
has it. An encounter two hours into that period compares against a bare bound,
and without a default the comparison has no answer: a `where` drops what it
cannot confirm, and the patient leaves the population over two hours the
defining language does settle.

## Integer is 64 bits here, and there is no Long

The specification gives `Integer` the range -2^31 to 2^31-1, and adds a `Long`
for -2^63 to 2^63-1 with literals of its own — `45L` — together with `toLong`
and `convertsToLong`. All of that is marked Standard for Trial Use: content the
specification says has not received significant implementation experience.

This engine holds an `Integer` in Go's `int64`, so it already accepts the whole
range `Long` was introduced for, and says `Integer` when asked:

    3000000000                        // 3000000000
    3000000000.type().name            // Integer
    3000000000 is Integer             // true
    2147483647 + 1                    // 2147483648
    '3000000000'.convertsToInteger()  // true

A conforming engine would answer `{ }` to the first and `false` to the last,
since neither value fits an Integer. Narrowing to 32 bits would be a change to
what callers already rely on — FHIR itself has an `integer64` type, and a
`Bundle.total` or an `Observation.valueInteger` beyond two billion is data, not
a mistake — and it would trade a range that works for one that needs a second
type to get it back.

So `Long` is not implemented, and `toLong`/`convertsToLong` are not either:
against a 64-bit Integer they would be `toInteger` and `convertsToInteger` under
another name, and `45L` does not parse. The divergence is the range of `Integer`,
recorded here rather than left for someone to discover from a surprising answer.

One thing that follows from it is a genuine gap rather than a decision.
Arithmetic wraps silently at the end of the range:

    9223372036854775807 + 1           // -9223372036854775808

That is Go's `int64` overflow reaching the surface. A literal too large for
`int64` is read as a `Decimal`, which is why `9223372036854775808` answers with
itself, but a sum that crosses the boundary does not get that treatment. Whether
it should signal an error or return empty is a question the specification does
not settle for Integer either, and it is worth answering deliberately rather
than by widening the type.

## Decisions taken where the specification is silent

Recorded here and at the point of code, so nobody has to guess whether a choice
was reasoned or accidental.

| Question | Decision | Why |
|---|---|---|
| `Integer` range, and `Long` | 64 bits, with no separate `Long` type | The specification's `Integer` is 32 bits and its `Long` — STU content — is 64. An `int64` Integer already spans that range, and FHIR's own `integer64` says the values occur. See above |
| Max precision for `lowBoundary`/`highBoundary` | An implementation limit, documented in the function | 3.0.0 says explicitly: above "the maximum possible precision of the implementation", return empty. FHIR caps decimal at 18 digits |
| `sort` direction syntax | Accept both `desc`/`asc` and the leading `-` | 3.0.0 defines `desc`; the suite tests `-` and marks those cases as prototype |
| `type()` completeness | Emit `SimpleTypeInfo` and `ClassInfo`; omit `ClassInfo.element`, `ListTypeInfo`, `TupleTypeInfo` | They need element enumeration and declared cardinality, which the `Model` interface does not expose. Reading them off the instance would describe the value, not the type |
| `type()` results per element | One per input element | 3.0.0 states this, then contradicts it in its own `ListTypeInfo` example |
| Comparing a temporal that carries a timezone offset with one that does not | No answer — empty, reported as `ErrOffsetMismatch` | The default offset is a policy the specification leaves to the caller, and FHIR requires the offset where it matters, so a bare value is a literal or invalid data. See below |
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
