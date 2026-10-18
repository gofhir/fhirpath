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

### When 2.0.0 and 3.0.0 disagree

FHIR R4 and R5 both say "The FHIR Specification uses FHIRPath (release 2)",
2.0.0, and no published FHIR version adopts 3.0.0. Neither reference engine has
a switch between the two: fhirpath.js 5.2.0 answers alike with its R4 and R5
models, and the HL7 validator switches by FHIR version, not by FHIRPath
version. This engine is measured against both reference libraries, fhirpath.js
and HL7's Java engine, the one the validator runs (#167). Where the texts, or
the libraries, part ways, it decides in this order:

1. Where the two libraries give the same coherent answer, it gives that one.
   Where they part ways, it gives the answer of the one the text bears out:
   `hasValue()` on a complex value is false, as R4's text and fhirpath.js have
   it, where the Java engine answers true and R5's text empty. Two cases give the Java engine's
   answer, the validator's, instead: where the text says nothing, and where
   the validator reaches a published constraint's verdict through that
   answer, so that following the text would accept or reject a resource the
   validator does not. pat-1 and bdl-4 fail in the validator through
   `exists()` on an element written `{}`, which fhirpath.js and the text
   answer true; `hasValue()` on a complex value decides no such verdict, since
   the validator accepts `"maritalStatus": {"id": "x"}` whatever it answers
   there (#161). See
   [Where this engine follows the HL7 validator](#where-this-engine-follows-the-hl7-validator-over-the-specification-text).
   A coherent answer is set aside only for a reason recorded under
   [Measured, and not followed](#measured-and-not-followed): one that rests on
   a choice this engine made on its own grounds, the 64-bit Integer, or one no
   published constraint reaches and that would cost a rewrite to match.
2. Where it does not, it follows 3.0.0 where 3.0.0 fixes a plain defect of
   2.0.0, or defines what 2.0.0 does not: `sort()`, `comparable()`,
   `lowBoundary()` and the rest above.
3. Otherwise it follows 2.0.0: it is normative, and the release R4 and R5
   cite.

Each entry below that departs from a text names the text. Among the cases
measured so far, the two FHIRPath versions part ways in seven:

| | 2.0.0 | 3.0.0 | This engine |
|---|---|---|---|
| `power()` on Integers | an Integer | always a Decimal | a Decimal, as the validator, R4 too |
| `toString()` of a calendar duration: `1 week.toString()` | `1 'week'`, the unit quoted as for any quantity | `1 week`, the keyword unquoted | `1 week`, 3.0.0, as fhirpath.js, the Java engine and the official suite answer |
| Calendar durations against UCUM's: `1 day = 1 'd'`, `1 week = 1 'wk'`, and hour, minute | unequal: "calendar durations and definite quantity durations above seconds are considered unequal" | equal: "for practical reasons it considers them to be equal" | equal, 3.0.0, as fhirpath.js and the Java engine answer; years and months stay apart |
| Ordering calendar durations against UCUM's: `1 day > 23 'h'` | "un-comparable" above seconds | "years and months are not comparable across UCUM definite durations and calendar units", so a day is | ordered, 3.0.0, as fhirpath.js and the Java engine answer |
| `implies` with an empty left operand: `{} implies false` | empty, by the truth table | empty by its table, but its note says "the result of the operation is the right side" | empty, as both tables, fhirpath.js and the Java engine answer |
| `~` between quantities in different units | in "the most granular unit of either input" | "converted to the least granular unit" | the least granular, 3.0.0: converting to the finer unit made the value look more precise than it was written, the defect 3.0.0 names |
| `~` between strings | whitespace "normalized": the characters of the Whitespace lexical category, tab, space, line feed and carriage return, equivalent | all characters of the Unicode White_Space class equivalent, a no-break space among them, and "Multiple whitespace characters in sequence are not collapsed", so `'a     b' ~ 'a b'` is false | 3.0.0, as fhirpath.js answers: any whitespace character for another, no run collapsed and no end trimmed. 2.0.0's Whitespace lexical category is the grammar's, for the expression's own text; 3.0.0 names the set for a string's content, the defect it fixes. The Java engine tells a tab from a space; see [Measured, and not followed](#measured-and-not-followed). Case is set aside one character for another, by Unicode's simple folding: fhirpath.js, comparing upper case, also takes `ß` for `SS` and `ﬁ` for `FI`, and tells the Kelvin sign from `K`, where this engine does not |

### What depends on the FHIR version

The validator answers by FHIR version, read here from a model that implements
`VersionedModel`. Over 203 expressions measured with `-version 4.0.1` and
`-version 5.0.0` (6.10.2), the batteries of the entries below, it answers
differently in two places:

| Expression | Validator R4 | Validator R5 | This engine |
|---|---|---|---|
| `Patient.name.as(HumanName)`, three names | filters | an error | as the validator, by the model's version |
| `Patient.id.is(id)` / `is(System.String)` | false / true | true / false | as the validator, by what the model's definitions name |

The second is no rule of the version. The definitions name the FHIR type of
`Resource.id` in `structuredefinition-fhir-type`, and R4's name a string where
R4B's and R5's name an id; the validator reads the extension in every version.
A model that implements `FHIRTypes` hands the engine that type: see
[Where this engine follows the HL7 validator](#where-this-engine-follows-the-hl7-validator-over-the-specification-text).
Every other expression measured this engine answers alike for R4 and R5, as
the validator does. One rule the batteries did not reach also depends on the
version here: `conformsTo()` on a profile it cannot resolve; see
[conformsTo, where R5 disagrees with itself](#conformsto-where-r5-disagrees-with-itself).

## Current conformance

Measured against the official suite, in both configurations a caller can use:

| Configuration | Passing |
|---|---|
| R4 suite, no FHIR model supplied | **900 of 935 (96.3%)** |
| R4 suite, with the R4 model | **926 of 935 (99.0%)** |
| R5 suite, no FHIR model supplied | **1007 of 1048 (96.1%)** |
| R5 suite, with the R5 model | **1033 of 1048 (98.6%)** |

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

A model is worth less here than expected — 26 cases across the R4 corpus — and
the reason is instructive: most remaining `is()`/`as()` failures are not about
hierarchy but about FHIR primitives being distinct types from their System
counterparts (`Patient.gender.as(string)` must yield empty because gender is a
`code`).

Cases that do not pass yet live in `known-failures.txt` and
`known-failures-model.txt`, one baseline per configuration. CI does not break over
pre-existing gaps, but it fails on a regression **and** when a listed case starts
passing — so the list can only shrink, and it never lies about the number.

The list says which cases fail; `known-failure-reasons.txt`, next to each suite's
baselines, says why. Every case in a model baseline needs a line there with its
kind — `suite-defect`, `deliberate`, `needs-external` or `gap` — and the heading
of this file that argues it, and `TestKnownFailuresAreExplained` fails on a case
without one, on a line for a case that no longer fails, and on an anchor this
file does not have. A case that fails only without a model needs no line: the
model run passing it is the explanation.

## Remaining gaps

| Block | Cases | Notes |
|---|---|---|
| `as()` on a multi-item collection | 1 | Applies from R5; the suite is R4 — see below |
| `htmlChecks()` | 4 (R5) | Needs an XHTML parser and FHIR's element list; one of the four also needs `div` read as an identifier — see below |
| `%terminologies` | 3 (R5) | Needs a terminology server answering, not a stub — see below |
| `lowBoundary` / `highBoundary` | 5 | Three are a suite disagreement (see below); two assume `@2014-01-01T08` carries an implicit minute |
| `sort()` with an empty key, descending | 1 | A prototype case that contradicts FHIRPath 3.0.0 — see below |

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

`toLong` and `convertsToLong` read what `toInteger` and `convertsToInteger`
read: this engine's `Integer` is already 64 bits wide, which is what `Long`
exists to provide. That divergence has its own section below.

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
the offset that makes the instant earliest, while 3.0.0's example shows no
offset at all. The suite's reading is the stricter one — a boundary should
bound — and it is FHIR R5's own: "the lowBoundary of 2010-10-10 is
2010-10-10T00:00:00.000+14:00". The HL7 validator adds the offsets too, and
fhirpath.js omits them. What decides it is a published constraint: R5's per-1
is `start.lowBoundary() <= end.highBoundary()`, and on a period from
`2001-05-06` to `2001-05-06T10:10:10Z` both reference engines answer true.
This engine orders a DateTime with no offset against one with an offset only
where the caller places it (see
[A default timezone offset is the caller's policy](#a-default-timezone-offset-is-the-callers-policy-not-the-engines)),
so a boundary without one would make per-1 fail a valid period; with the
widest offsets it holds. Measured while working the datetime spec cases.

### An empty sort key is the lowest value

FHIRPath 3.0.0 defines where an empty key goes: "An empty value is considered
lower than all other values, meaning they will appear before others when sorted
ascending." Sorted descending, it comes last. `testSort10`, in both suites,
expects the opposite: `Patient.name.sort(-family, -given.first()).first().use`
to be `'usual'`, the name without a family. The R5 suite marks the case
"Prototype definition - not part of spec yet", and it writes descending with a
leading `-`, prototype syntax that 3.0.0 spells `desc`. fhirpath.js 5.2.0
follows 3.0.0: `Patient.name.sort(family desc).use` gives `maiden, official,
usual` there as here.

The same release brought `sort()` to 3.0.0 on the points no suite case reaches.
A key evaluates with singleton semantics, so a key of several items ends the
evaluation with an error. Keys compare as `<` compares them, and "values that
would result in comparison errors must be filtered" before sorting, so a pair
that does not compare ends it too: one `<` refuses, such as `1` and `'a'`, and
one whose order is unknown, such as quantities whose units do not convert or
`@2020` against `@2020-01-15`, for which `<` answers empty. The sort used to
leave such pairs where they stood without a word, which could return
`(1 'mg' | 2 'cm' | 0.5 'mg').sort()` unsorted. fhirpath.js raises an error for
a key of several items and for a pair that does not compare. A later key is
compared only between items the earlier keys tie, as in fhirpath.js. Whether a
sort fails does not depend on the order its input comes in: where a key mixes
temporals of different precisions or offsets, or quantities of different units,
whose comparisons are not always transitive, every pair is compared. What it
costs: nothing for keys of one kind, such as dates of one precision or
quantities of one unit, which sort as fast as before; for 1000 dates of mixed
precision, about 34 ms against 2.6 ms.

Booleans do not sort here, since `<` does not compare them; fhirpath.js sorts
`false` before `true`. Complex values, such as `Patient.name.sort()`, do not
sort either, here or in fhirpath.js; earlier releases returned them in input
order. A collection of one item is returned as it is, its key never evaluated,
as in fhirpath.js.

### R5 cases that need something outside the engine

Three `TerminologyTests` cases fail in the R5 suite for want of a terminology
server, and two `HTMLChecks` cases for want of the checker `htmlChecks()`
asks, which the caller brings. A third `HTMLChecks` case does not parse.

They are listed as known failures rather than skipped, because they run: the
engine reaches them, answers, and answers wrongly. That is different from the
three `ccda.xml` cases, which have no input the harness can read and are
reported separately, outside the denominator — there is nothing to grade. A case
that runs and fails belongs in the number.

**`htmlChecks()`** validates that a `Narrative.div` holds XHTML FHIR permits —
the element list, no scripts, no external references. That needs an XHTML
parser and FHIR's own list of what is allowed, neither of which belongs in an
expression engine that has no other reason to read markup, so the caller
brings them: `WithHTMLChecker` (or `eval.Context.SetHTMLChecker`) hands the
engine an `HTMLChecker`, which it asks about a single xhtml element, and
about a single string: fhirpath.js and the HL7 validator both check a string
too, a FHIR `string` or a literal, `'<b>x</b>'.htmlChecks()` true and a
`<button>` false, though R5's text makes it empty, "empty on any other kind of
element, or a collection of xhtml elements". On a `code` the two part,
fhirpath.js checking it and the validator answering empty, and the text's
empty stays. Anything else, and more than one item, is empty, R4's "undefined
( null )". What a value is comes from its type, so without a model a code read
from the resource is a string. Without a checker, a single xhtml element or
string ends the evaluation with an error, rather than pass a narrative nobody
read; up to 1.14.7 the function did not exist. fhirpath.js carries a checker
of its own, and the HL7 validator its own.

One of the three runs into something else first. `htmlTest01` asks for
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

The other two read a `Parameters` resource instead —
`%resource.parameter.where(name='goodHtml').value.htmlChecks()` — whose value
is a `valueString`, which the engine hands to the checker, as both reference
engines check it; the suite gives none. A fourth, `htmlTest04`, asks for
empty on a value that is no HTML at all, an integer, and passes.

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

The run is the same on every machine. fhirpath.js is pinned by
`difftest/package-lock.json` and installed with `npm ci`, and the report names
the release that answered. It runs with `TZ=UTC`, because fhirpath.js renders
the result of date arithmetic in the process's timezone:
`@1973-12-25T00:00:00.000+10:00 + 0.1 's'` came back as `-03:00` on one machine
and `-00:00` on another. Answers are still compared as written rather than as
instants, since an offset one engine drops is a difference worth seeing.

A battery of one's own, outside any suite, is put to three engines: this one,
fhirpath.js, and HL7's Java engine, the one the validator runs, through the
validator's jar. It reports, first, where this engine alone differs from the
other two, the cases rule 1 under
[When 2.0.0 and 3.0.0 disagree](#when-200-and-300-disagree) puts on this
engine to change or justify (#167):

    HL7_VALIDATOR_JAR=~/tools/validator_cli.jar make three-engine
    make three-engine BATTERY=batteries/hl7-divergences/r5.tsv DIFFTEST_ARGS="-v -jobs 6"

A battery is `resource<TAB>expression<TAB>version` a line, the resource a JSON
file beside it, the version 4.0.1 or 5.0.0. `difftest/batteries/hl7-divergences`
holds the expressions measured for #151, #153 and #155. The Java engine takes
seconds a case, nearly all of it loading packages, and runs `-jobs` at a time.
Objects are compared only as objects, since the Java engine names them by type
and the others write their JSON. The Java engine writes a result as one line,
its items joined by commas, so a Java answer is compared as that line: an item
holding a comma reads alike on both sides, but the single string `'a,b'` and
the two items `'a'` and `'b'` cannot be told apart, nor an empty collection
from a single empty string. A run of the validator that writes no result, a
wrong jar or too little memory, stops the battery rather than passing for an
answer.

Against fhirpath.js 5.2.0, with each engine given its model:

| | R4 | R5 |
|---|---|---|
| Cases compared | 915 | 1014 |
| Same answer | 881 (96.3%) | 980 (96.6%) |
| **This engine wrong, fhirpath.js right** | **1** | **0** |
| Both agree, suite disagrees | 6 | 13 |
| fhirpath.js wrong, this engine right | 29 | 31 |
| Neither matches, and they differ | 4 | 3 |

The one case where fhirpath.js is right and this engine is not is `as()` over a
collection, a divergence taken on purpose and recorded below, as is short-circuiting
`and`, `or` and `implies`, which no case in the suite exercises.

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

## Cases written from the specification

The suite and fhirpath.js have little left to say: one case in R4 where this
engine is wrong and fhirpath.js right, and that one on purpose. New gaps can only
come from new cases, so the project writes its own, one area of the language at
a time, under [`conformance/testdata/spec-cases/`](conformance/testdata/spec-cases).

Each requirement quotes the sentence it comes from, in the 2.0.0 or 3.0.0
specification or in FHIR's FHIRPath page, and the build checks that the
sentence is there, verbatim, in a copy pinned by hash. Where the text is silent
there is no case; the question is listed for a decision instead. The cases run
in the suite's format, so `make conformance` measures them and `make difftest
DIFFTEST_ARGS="-corpus spec"` puts them to fhirpath.js. A failing one needs a
reason, as a suite case does, except that `suite-defect` is not available: a
wrong case of our own is corrected, not listed.

They are written by the `/fhirpath-cases` workflow in [`.claude/`](.claude),
whose authors cannot read this engine, fhirpath.js or this file — a case
written by someone who has seen an engine's answer tends to agree with it —
and adjudicated by an agent that reads all three.

An area's cases are a battery too, so the same run puts them to HL7's Java
engine, once with the R4 model and inputs and once with the R5 ones (#167):

    HL7_VALIDATOR_JAR=~/tools/validator_cli.jar make three-engine \
      BATTERY=../conformance/testdata/spec-cases/existence.xml DIFFTEST_ARGS="-jobs 2 -v"

The areas are taken in the order published constraints use them, counted over
the 610 distinct constraint expressions of the R4, R4B and R5 core and the IGs
in the validator's package cache: existence first (`exists` in 353 of them,
`empty` in 168, `count` in 50, `all` in 43), then Boolean logic, equality and
comparison, filtering, strings, collections, the FHIR functions, math, date and time,
conversions, aggregates.

| Area | Requirements | Cases | Pass, R4 and R5 | Alone against fhirpath.js and Java |
|---|---|---|---|---|
| existence | 34 | 134 | 134 | 0 |
| boolean-logic | 17 | 91 | 91 | 0 |
| equality-comparison | 46 | 325 | 318 | 1, `23 ~ 23 '1'`, following the text |
| filtering-projection | 40 | 114 | all, 112 in R4 and 114 in R5 | none where the other two answer; `repeatAll()` and `System.Any`, which both refuse |
| strings | 88 | 317 | 316 | on two points, each as 3.0.0 states: characters counted as Unicode scalar values, and `encode('hex')` in lower case |
| collections | 35 | 173 | 173 | none where the other two answer; `combine(other, preserveOrder)`, which both refuse |
| fhir-functions | 56 | 196 | 160 of 168 in R4, 172 of 185 in R5 | none where the other two answer, `htmlChecks()` on a narrative or a string aside, which takes a checker here; `subsumes()`, `subsumedBy()`, `checkModifiers()`, `elementDefinition()`, `trace()`'s projection and `%'vs-…'`, which both refuse |
| aggregates | 29 | 87 | 87 | none where the other two answer: `$total` of several items and `init` on the outer context follow both texts and fhirpath.js, the validator implementing neither `sum()` nor `avg()` |
| conversions | 79 | 497 | 494 | on `toInteger()` of a FHIR decimal written without a fraction, `185`, or of a quotient or a rounding, `(10 / 2).toInteger()` and `1.5.round().toInteger()`, which both convert by how the decimal is written, the validator answering `(10 / 2).convertsToInteger()` false besides; on the type of a computed number, `(5 + 10.0) is System.Decimal`, which each gets wrong its own way; on `convertsToString()` of a complex value, which both answer true with strings that differ; and on `toDate()` and `toDateTime()` with a format, which neither implements; `is System.Long` and a calendar duration's `toString()` fail on purpose |
| datetime | 68 | 345 | 337 | on `duration()` and `difference()`, which both refuse, and on a Date past year 9999, which both write as `10000-12-31`; a Date plus a time unit and the offset of an unzoned boundary fail on purpose |
| math | 72 | 404 | 391 | on `10.log(0)`, which both answer 0, on Decimal arithmetic exact to the eighth place and beyond, which both lose, and on the type of a computed number, `(2 * 1.0) is Decimal`, which each gets wrong its own way; the cases written for 2.0.0's 32-bit Integer fail on purpose |

**Existence.** One gap, closed: a criteria that evaluates to more than one
item, `(1 | 2).exists(true | false)` or `Patient.name.exists(given)` on a name
with two given names, read as not true where `where()`'s text ends the
evaluation: "If the result of evaluating the condition is other than a single
boolean value, the evaluation will end and signal an error to the calling
environment". `where()`, `exists()` and `all()` now raise it, as the Java
engine does in all three; fhirpath.js reads the first item in `where()` and
`exists()`. No constraint over the R4, R4B and R5 examples reaches it.
fhirpath.js is wrong in two more cases: `subsetOf()` and `supersetOf()`
compare the collections' sizes first, so `1.combine(1).subsetOf(1)` is false. Where the Java engine alone differs, see
[Measured, and not followed](#measured-and-not-followed).

**Boolean logic.** No gap: the three engines agree on all 182 evaluations, the
truth tables of `and`, `or`, `xor` and `implies`, `not()`, empty and non-Boolean
operands, an operand of more than one item, and the specification's worked
examples. The cases leave out what the text leaves open, an operand that raises
an error where the other operand decides the result alone; measured apart, the
engines part there as
[`and`, `or` and `implies` decide from the left](#a-divergence-taken-on-purpose-and-or-and-implies-decide-from-the-left)
records.

**Equality and comparison.** Three gaps, closed:

- `=` between collections ended at the first pair whose equality is unknown, so
  `(@2012 | @2013) = (@2012-01 | @2014)` was empty; "if the result is false for
  any pair, returns false" (3.0.0), and fhirpath.js and the Java engine answer
  false. An unknown pair now leaves the answer empty only where no pair is
  unequal.
- An Integer or a Decimal compared with a quantity was never converted, so
  `23 = 23 '1'` was false and `2 < 3 '1'` an error. 2.0.0 makes both
  conversions to Quantity implicit, `toQuantity()` giving "the default unit
  ('1')", and 3.0.0 gives `23 = 23 '1'` and `23 ~ 23 '1'` as true. Both now
  convert, under `=`, `~` and ordering, and since the math area under
  arithmetic too: see [Math](#math). The two references part here: fhirpath.js converts under `=` and
  ordering and not under `~`, and the Java engine converts under none, `=`
  false and `<` empty; this engine follows the text, also under `~`, where it
  stands against both. A number is no quantity of another dimension:
  `2 = 2 'mg'` is false, as both references answer. The operators that look
  for an item agree with `=`: `23 in (23 '1')` is true and `(23 | 23 '1')`
  holds one item, as in fhirpath.js, where the Java engine finds none and
  keeps two.
- A FHIR Quantity was found by `=` and not by the operators that look for an
  item: `Observation.value = 5 'mg'` was true, and `5 'mg' in
  Observation.value` false, `(Observation.value | 5 'mg')` two items, where
  fhirpath.js and the Java engine find it. They now compare a FHIR Quantity
  as `=` does.

The seven cases that fail are written to 2.0.0 where this engine takes 3.0.0,
with both references: whitespace in `~` and the calendar durations above. Of
the evaluations where the Java engine alone differs, the coherent ones are
under [Measured, and not followed](#measured-and-not-followed); fhirpath.js
alone is wrong in five a version, among them `~` over collections, which it
pairs in order.

**Filtering and projection.** `where()`, `select()`, `repeat()`, `repeatAll()`
and `ofType()`. Two gaps, closed:

- `ofType(System.Any)` kept nothing, where System.Any is "an ultimate root
  type" of the System types, so every value is one, and this engine's own
  `type()` names it the base of `System.String`. It now keeps every item, and
  `is(System.Any)` is true of any value, a FHIR resource or element among
  them: the text names no other root, though `type()` gives a FHIR type the
  base FHIR's own hierarchy does. The namespace is written `System.`, as for
  every System type; unqualified, `Any` names no FHIR type and is System.Any,
  and `FHIR.Any` with a model is a type that does not resolve, an error. fhirpath.js and the Java engine both
  refuse the type name, and an error is no answer to follow.
- [Static analysis](#static-analysis) rejected `Questionnaire.repeat(item |
  item.item)`, which evaluation answers: see there.

fhirpath.js is wrong in two: `repeat(item | item.item)` keeps duplicates past
six items, and `ofType(string)` keeps a `markdown`, which FHIR says is no
subclass of string. Neither it nor the Java engine implements `repeatAll()`,
3.0.0's trial-use function, which this engine does.

### Strings

Six gaps, closed:

- An argument of more than one item, `'abc'.startsWith('a' | 'b')`, was read
  as no argument; "If the function expects the argument passed to a parameter
  to be a single value ... and it is passed an argument that evaluates to a
  collection with multiple items, ... the evaluation of the expression will
  end and an error will be signaled to the calling environment" (2.0.0). It
  now does, for the string and encoding functions, as fhirpath.js does, where
  the Java engine answers from one of them.
- A surrogate pair written as two escapes, `'\uD83D\uDD25'`, read as two
  characters; it is now one, `'🔥'`.
- `encode('ascii')`, 3.0.0's, was not implemented: "any character above
  character code 127 replaced with `?`", a character being a scalar value.
- `decode()` turned bytes that are no UTF-8 into replacement characters: "If
  the content being decoded is not a valid UTF-8 value, the result will be
  empty".
- `trim()` removed every Unicode space; see the decision below.
- `{}.join()` was the empty string; "If the input is empty, the result is
  empty".

Characters are Unicode scalar values, as 3.0.0 states with its examples,
`'🔥'.length()` 1 and `'a🔥b'.indexOf('b')` 2. fhirpath.js and the Java engine
count UTF-16 code units, 2 and 3, and cut a surrogate pair apart in
`substring()` and `toChars()`, which leaves no string; see
[Measured, and not followed](#measured-and-not-followed).

`trim()` removes tab, line feed, vertical tab, form feed, carriage return and
space. 3.0.0, which defines it, trims "whitespace characters as defined in the
Whitespace lexical category", which names four of those: tab, space, line feed
and carriage return. fhirpath.js and the Java engine both remove the vertical
tab and the form feed too, and this engine follows them, so `'\fa'.trim()` is
`a`, against the case written to the text. Where the two part, the text's
reading stays: the Java engine trims every character up to space, `\u0000`
and `\u001F` among them, and fhirpath.js every Unicode space, the no-break
space and `\u2003` among them; this engine trims neither.

### Collections

`|`, `in`, `contains`, `union()`, `combine()`, and the subsetting functions,
`single()`, `first()`, `last()`, `tail()`, `skip()`, `take()`, `intersect()`
and `exclude()`. Three gaps, closed:

- `combine()` refused 3.0.0's second argument, `preserveOrder`, a trial-use
  addition: "When `preserveOrder` is `true`, the items of the other
  collection are appended to the items of the input collection, preserving
  the order of items in both collections". It now takes it. The result was
  already in that order, so `true` and `false` give the same items; a
  `preserveOrder` of more than one item, or no Boolean, ends the evaluation.
  The Java engine refuses the second argument, and fhirpath.js logs the
  arity and answers empty; neither is an answer to follow.
- `single()` of an empty input ended with an error; "If the input collection
  is empty (`{ }`), the result is empty", as fhirpath.js answers. The Java
  engine raises an exception.
- `skip()` and `take()` read the first item of a `num` of several,
  `skip(1 | 2)`; the argument now ends the evaluation, as for the string
  functions above and as fhirpath.js does, where the Java engine answers from
  the first.

fhirpath.js is wrong with a negative `num`: `take(-1)` keeps all but the last
item and `skip(-1)` the last alone, where "If num is less than or equal to
0" `take()` returns empty, and "If `num` is less than or equal to zero, the
input collection is simply returned" by `skip()`.

### FHIR functions

What FHIR's FHIRPath page adds, R4's and R5's: `extension()`, `hasValue()`,
`getValue()`, `resolve()`, `conformsTo()`, `memberOf()`, `subsumes()`,
`subsumedBy()`, `htmlChecks()`, `checkModifiers()`, `elementDefinition()`,
`slice()`, `comparable()`, `lowBoundary()` and `highBoundary()` on decimals,
the FHIR type model, `%resource`, `%rootResource`, `%context`, `%ucum`,
`%sct`, `%loinc`, `%vs-` and `%ext-`, and `trace()`. Where the two pages
part, R4 ending the evaluation where R5 answers empty, the cases are written
per version. Gaps closed:

- Six functions did not exist: `subsumes()`, `subsumedBy()`, `htmlChecks()`,
  `checkModifiers()`, `elementDefinition()` and `slice()`. Each now answers
  what its text settles without anything outside the engine: equivalence by
  `system` and `code` for subsumption, "for Coding values, equivalence is
  defined based on the code and system elements only", empty on what is no
  single xhtml element or string, the input back when every `modifierExtension` is
  named, empty for an empty input or an unresolvable structure under R5. For
  the rest each asks the caller: an `eval.Subsumer`, which a
  `TerminologyService` may also be, for a hierarchy, and an `HTMLChecker`
  for xhtml; see [R5 cases that need something outside the engine](#r5-cases-that-need-something-outside-the-engine).
  Element and structure definitions are not the engine's, so
  `elementDefinition()` of an element and `slice()` with a structure that is
  a URI end the evaluation with an error. fhirpath.js and the Java engine
  both raise an error on `subsumes()`, `subsumedBy()`, `checkModifiers()` and
  `elementDefinition()`, so this engine answers alone there.
- `conformsTo()` of more than one item ended the evaluation under R5 too,
  where R5 says "If the input is not a single item ... the result is empty".
- `memberOf()` of a value set that is no URI was empty under R4 too, where R4
  says "If the valueset cannot be resolved as a uri to a value set, an error
  is thrown". A URI with no terminology service to ask stays empty, unknown,
  under both: the published constraints that use `memberOf()` would
  otherwise fail wherever no server is configured.
- `memberOf()` of several items answered whether any was a member, where R5
  says "the input is empty or has more than one value, the return value is
  empty". It is now empty, under R4 too, which says nothing of several items:
  measured against tx.fhir.org, fhirpath.js and the Java engine both answer
  `('final' | 'bogus').memberOf(...)` and `category.memberOf(...)` over two
  categories empty under both versions, and true or false for one item. The
  two constraints in R5's core that use `memberOf()`, exs-1 and opd-3, call
  it on an element of at most one item, and those of the IGs in the
  validator's package cache call it per item or on a single element, so no
  verdict of theirs changes.
- `trace()`'s projection was evaluated once, from the scope the call sits in,
  not "with respect to each of the items in the input collection" (2.0.0), so
  `(1 | 2).trace('x', $this * 10)` ended in an error. fhirpath.js and the Java
  engine raise one too. It is evaluated whether or not a logger is set, as
  it was before, so an error in it ends the evaluation either way.
- `comparable()` did not read a number as a quantity of unit `'1'`, so
  3.0.0's `1.comparable(2)` was false, and ended the evaluation for an input
  of several items, where "either input is not a single Quantity value, the
  result is empty".
- [Static analysis](#static-analysis) rejected an expression opening with a
  supertype of its context's type, `DomainResource.text` on a Patient, which
  2.0.0 allows: the name "must resolve to the type of the context (or a
  supertype)".
- `extension(url)` read its items untyped, so with a model they were not
  Extensions, their `url` was no `uri`, and `value[x]` was found by a list of
  suffixes rather than the model: R5's `valueCodeableReference` was lost, and
  R4 answered a `valueInteger64` it does not have. FHIR defines it as "a
  syntactical shortcut for `.extension.where(url = string)`", and it now reads
  through the same navigation; over every official R4, R4B and R5 example the
  two forms answer alike, items and types (`make extension-shortcut`, #214).
- `extension(url)` with a url of several items used the first, and with a url
  that is no String answered empty. Both pages declare `extension(url :
  string)`, and an argument of several items, or of a type that does not
  convert implicitly, to a single-valued parameter ends the evaluation; both
  now do, as fhirpath.js answers. The validator answers otherwise: see
  [Measured, and not followed](#measured-and-not-followed).
- `getExtensionValue(url)`, this engine's own and not FHIR's, looked for 20
  `value[x]` names, read untyped: the other 30 (R4) to 34 (R5) types an
  Extension's value may have answered empty. It is now `extension(url).value`,
  typed by the model; over every official example it answers the values the
  JSON holds, of the types their names give (#218). A `valueString` is a FHIR
  `string`, no longer a `String`. Without a model the types navigation does
  not know yet are still missed (#219).

`hasValue()` of a System value of a primitive type, `'a'.hasValue()`, is now
true, as fhirpath.js and the Java engine both answer, against both pages'
text, which speaks of FHIR primitives alone, R4 then false and R5 empty. It
was false, a decision recorded as following the text; rule 1 reverses it.
The cases written to the text fail on purpose: 008-e, R4's not true, and
009-e, R5's empty. So does R5's "Otherwise, the return value is empty"
elsewhere: for a primitive with no value, several items or an empty input
all three engines answer false (009-a, c, d), and for a complex value
fhirpath.js and this engine answer false where the Java engine answers true
(009-b), as [Measured, and not followed](#measured-and-not-followed) argues.

The other cases that fail on purpose: a primitive's value is not among its
`children()`, as [A primitive's value is not a child](#a-primitives-value-is-not-a-child)
records, and `true.comparable(1 'mg')` is false, as all three engines answer,
where 3.0.0 makes it empty. `htmlChecks()` on a narrative, 053, needs a
checker the cases do not give.

The cases leave open what the text does not settle, among them the System
type a FHIR `date` maps to: FHIR's table says `System.DateTime`, while
FHIRPath has a `System.Date` that is no subtype of it. This engine maps it to
`System.Date`; fhirpath.js says it is neither, and the Java engine says it is
both. `%resource` inside a contained resource, which FHIR's pages and 3.0.0
define differently, and `lowBoundary()` on integers and dates, where R5's
page and 3.0.0 part, are left open too.

Where the text asks for what only a service could tell, the engine answers
empty, unknown, without one: `subsumes()` of two codes that are not
equivalent, where the text says "and false otherwise", and of codes from
different code systems, where R4, and R5 for `subsumedBy()`, say "a run-time
error is thrown" unless "the relationships between the code systems" are
"well-defined", which only a terminology service knows. `memberOf()` whose
service fails for an item is empty too, not false; up to 1.14.7 it was
false. `checkModifiers()` under R5 ends the evaluation on an empty modifier
whatever its input, reading "an error is thrown, including if modifier is
empty" as a rule on the argument, not on the check.

fhirpath.js does not define `%resource`, `%rootResource`, `%sct`, `%loinc`,
`%vs-` or `%ext-` unless the caller passes them, gives `'a'.getValue()` as
`a`, and answers `htmlChecks()` on a `code`. The Java engine's coherent
answers set aside are under [Measured, and not followed](#measured-and-not-followed).

### Math

The Math functions, `abs()`, `ceiling()`, `exp()`, `floor()`, `ln()`,
`log()`, `power()`, `round()`, `sqrt()` and `truncate()`, and the operators
`*`, `/`, `+`, `-`, `div`, `mod` and unary `+` and `-`, on numbers and
quantities. Published constraints barely use them, `+` in 18 expressions and
most of those on strings. Gaps closed:

- A Math function given what is no number, `'-5'.abs()` or `true.ceiling()`,
  or a number argument that is none, `16.log('2')`, or of several items,
  `2.power(2 | 3)`, answered empty or read the first item. Each now ends the
  evaluation, as fhirpath.js and the Java engine both refuse such an input:
  the input is "not compatible with the expected type".
- `ceiling()`, `floor()`, `round()` and `truncate()` refused a quantity,
  which 3.0.0 defines them on: `1.1 'cm'.ceiling()` is `2 'cm'`, as
  fhirpath.js answers; the Java engine answers empty. `abs()` reads a FHIR
  Quantity too.
- `log()` of 0 or a negative number, or to such a base, was empty, where 3.0.0
  says "the evaluation will end and signal an error"; a base of 1 stays empty.
  `10.log(0)` ends with the error, where both references answer 0, a
  floating-point accident: ln 0 is minus infinity.
- `round()` of an Integer returned an Integer; "round([precision : Integer])
  : Decimal".
- A leading `+` took any value; `+'a'` now ends the evaluation, as `-'a'` does.
  `-` and `+` take a FHIR Quantity, `-Observation.value`, which `-` refused.
- `+` and `-` between quantities in commensurable units gave the left
  operand's unit, where 2.0.0 says "The unit of the result will be the most
  granular unit of either input": `3 'm' + 3 'cm'` is `303 'cm'`, and on a
  tie the left operand's, as 3.0.0 chooses. The operand that is converted
  loses the zeros conversion leaves, the other keeps its precision:
  `2.00 'mg' + 1 'g'` is `1002.00 'mg'`. A calendar duration with a UCUM one
  is in the calendar, 3.0.0: "When the operands come from different
  time-valued unit systems, the result is expressed in calendar units", so
  `60 's' + 2 minutes` is `180 seconds` and `1 minute - 30 's'` `30 seconds`,
  where 3.0.0's own subtraction example, `0.5 minute`, takes the coarser unit
  against its addition rules; the two are equal.
- A quantity divided by zero ended the evaluation, where `/` answers empty
  for a zero divisor; `*` and `/` with a calendar duration, `12 day * 45 'm'`,
  ended it too, where 3.0.0 says "Multiplication involving calendar units
  (apart from the UCUM default unit `'1'`) will return empty". A quantity of
  unit `'1'` scales the other, as a number does: `2 day * 3 '1'` is `6 day`.
- A special unit, Cel or [degF], was converted in arithmetic, where 3.0.0 says
  "Attempting to operate on quantities with invalid or "special" units will
  result in empty"; comparison and `~` still convert them. Between two in the
  same unit, `1 'Cel' + 1 'Cel'`, or with a number, the text leaves it open,
  and they compute.
- A FHIR Quantity written with a unit and no value, which FHIR allows, holds
  no amount: the operators and the Math functions read it as empty, as they
  read a primitive with no value.

Two consequences of those rules, where an error used to tell an author
something: `aggregate($total + $this, 0)` over quantities is empty, `0` and a
quantity in `'mg'` not combining, where it ended the evaluation; starting from
`0 'mg'` adds them. And FHIR maps a FHIR Quantity's UCUM `s`, `min`, `h` or
`d` to the calendar's keyword, so `10 'mL/s' * Observation.value`, with a
value in `s`, is empty, a calendar duration in `*`, where `10 'mL/s' * 2 's'`
is `20 'mL'`.

A year or a month added to a smaller calendar unit is answered by 2.0.0's
factors, which give `1 year` as "`12 months` or `365 days`", in the finer unit: `1 year +
1 day` is `366 day`, `1 year + 12 months` `24 months`. 3.0.0's trial-use
note says "Quantity addition with month or year units requires explicit
conversion, otherwise the result is empty", a change rather than a fix; the
cases are written to 2.0.0, and the question stays open.

Units cost the operators more than they did: `+` and `-` between two units
convert one of each unit to the other's to find the finer before converting
an operand, and `*` and `/` check each unit for a special one first. Numbers, which constraints compute with, take
none of this.

Three decisions taken with the maintainer, each by rule 1:

- A number beside a quantity in arithmetic is a quantity of unit `'1'`,
  `2 + 2 '1'` is `4 '1'` and `60 / 1 's'` is `60 's-1'`, 3.0.0's `60 '/s'`, as fhirpath.js
  answers and 3.0.0's own examples read it; the Java engine raises an error.
  Up to 1.15.0 only comparison converted.
- `round()` with a negative precision rounds left of the point,
  `15.round(-1)` is `20`, as fhirpath.js and the Java engine both answer,
  though both versions say "the number of digits of precision must be >= 0
  or the evaluation will end and signal an error". Up to 1.15.0 it ended the
  evaluation. The cases written to the text, 025, fail on purpose.
- `power()` on Integers stays a Decimal, as recorded under
  [When 2.0.0 and 3.0.0 disagree](#when-200-and-300-disagree): 3.0.0 and the
  Java engine have it, and fhirpath.js's Integer comes from typing a value by
  whether it has a fraction, which says nothing of the operator.

The cases written for 2.0.0's 32-bit Integer fail on purpose: see
[Integer is 64 bits here, and there is no Long](#integer-is-64-bits-here-and-there-is-no-long).
Those written for 3.0.0's Long, built with `toLong()`, pass since the
conversions area made it an alias of `toInteger()`.
`*` and `/` between quantities gave UCUM's base units, `12 'cm' * 3 'cm'` as
`0.0036 'm2'`, where 2.0.0's own examples give `36 'cm2'` and `4.0 'cm'`. The
unit comes from `gofhir/ucum`, which from 4.3.0 writes a product in the
operands' own units: the same atom adds its exponents, and written with two
prefixes it takes the finer, so `2 'cm' * 2 'm'` is `400 'cm2'`. Both
references give a base unit.

Of the results where this engine alone differs from both references, most
are checks of a computed value's type, `(1 + 2) is Integer`, on which the two
agree by unrelated defects: fhirpath.js types a value by whether it has a
fraction, and the Java engine answers false for any computed value.

### Date and time

Date, DateTime and Time arithmetic with calendar durations and UCUM time
quantities, `now()`, `today()` and `timeOfDay()`, 3.0.0's component functions
(`yearOf()` to `millisecondOf()`, `timezoneOffsetOf()`, `dateOf()`,
`timeOf()`), `duration()` and `difference()`, and the temporal boundaries and
`precision()`. No gap: the engine answered each case as its text, or as a
decision recorded before, with one exception found later. `now()`, `today()`
and `timeOfDay()` read the clock on every call, so one expression could see
two instants, against "these operators should return the same value
regardless of how many times they are evaluated within any given expression"
(2.0.0 and 3.0.0): REQ-DATETIME-035-a, `timeOfDay() = now().timeOf()`, failed
about once in eight runs. Each evaluation now reads the time once, when one
of them first asks, and all three answer that instant; `WithNow` or
`Context.SetNow` fixes it (#225).

Two cases fail on purpose. A Date plus a time unit, `@2014-01-01 + 1 hour`,
leaves the Date as it is, as fhirpath.js and the HL7 validator both answer,
where 3.0.0 makes it an error: see the row on a quantity finer than the value
under [Decisions taken where the specification is silent](#decisions-taken-where-the-specification-is-silent).
And the boundary of a DateTime with no offset carries `+14:00` or `-12:00`, as
FHIR R5's page, the validator and the official suite have it, where 3.0.0's
example writes none: see [Remaining gaps](#remaining-gaps), where per-1 decides
it.

The UCUM codes `'wk'`, `'d'`, `'h'` and `'min'` shift a Date or a DateTime as
the calendar's week, day, hour and minute, as 3.0.0 has them and both
reference engines answer; 2.0.0 lists them as equivalent to the calendar
units and also calls a definite duration above seconds an error. `'a'` and
`'mo'` stay errors, as 3.0.0's year and month rows say.

`duration()` and `difference()` both reference engines refuse, so this engine
answers alone there, as 3.0.0 defines them. fhirpath.js writes an offset as
`-00:00`, refuses the quoted keywords `'month'` and `'day'`, reads
`@0001 - 1 year` as 1900 and has no `precision()`; the validator's coherent
answers set aside are under [Measured, and not followed](#measured-and-not-followed).

### Conversions

The Conversion functions, `toBoolean()` to `toQuantity()`, their
`convertsToX()` counterparts and 3.0.0's `toLong()`, implicit conversions, and
the literal syntax of each type. Gaps closed:

- `convertsToX()` of an empty input answered false, where both versions say
  "If the input collection is empty, the result is empty", as fhirpath.js
  answers; the Java engine answers false.
- `toInteger()` and `convertsToInteger()` took a Decimal, truncating it, so
  `1.5.toInteger()` was `1`; neither version lists a Decimal among what
  converts, "the item is an Integer ... a String ... a Boolean", and both
  references refuse `1.0`. They answer the FHIR decimal `185` with `185`,
  typing a decimal by whether its text has a point, which is no reading of
  the function.
- `toDecimal()` read `'1.'`, `'.5'` and `'1e5'`, which the form a decimal
  string takes, `(\+|-)?\d+(\.\d+)?`, refuses, and wrote `'+1.5'` back as
  `+1.5`, where `(-)?#0.0#` writes `1.5`, as fhirpath.js does; `toTime()`
  read `'T14:34:28'`, the T a literal opens with, which fhirpath.js still
  reads and the validator has no `toTime()` to answer.
- `toQuantity({})`, an empty unit, gave the quantity unconverted, a unit of
  several items its first, and a unit that is no String the quantity as it
  was; an empty argument now gives empty, from `convertsToQuantity()` too, and
  several items, or an item "not of the required type", end the evaluation.
  So does a format that is no String.
- `toString()` of a FHIR Quantity was empty: it is now the quantity it maps
  to, `185 '[lb_av]'`.
- `toDate()`, `toDateTime()`, `convertsToDate()` and `convertsToDateTime()`
  took no format, 3.0.0's trial-use template: `'15-01-2024'.toDate('dd-MM-yyyy')`
  is `@2024-01-15`. The codes the text marks required are read, and the
  optional ones that need no locale; `MMM`, `MMMM` and `z` are not. A
  template that names a component without the one above it, a minute without
  an hour, reads nothing, as does `hh` without `a`, which does not say which
  half of the day, `a` beside `HH`, and two hour codes. A one-digit code
  takes two digits where it can, so `'202415'` does not match `yyyyMd`.
  Neither reference engine implements the argument.
- `toLong()` and `convertsToLong()` did not exist; they are `toInteger()` and
  `convertsToInteger()`, decided with the maintainer: see
  [Integer is 64 bits here, and there is no Long](#integer-is-64-bits-here-and-there-is-no-long).
  The math cases built on `toLong()` pass with them.

Two cases fail on purpose: `is System.Long`, for want of the type, and
`toString()` of a calendar duration, written `1 week` as 3.0.0 and all three
engines have it, where the case follows 2.0.0's `1 'week'`; see the table under
[When 2.0.0 and 3.0.0 disagree](#when-200-and-300-disagree).

No published constraint changes its verdict: the eight that convert apply
`toString()` and `toInteger()` to strings, dates and a Count's decimal value.

### Aggregates

`aggregate()` with `$this`, `$total` and `$index`, and 3.0.0's `sum()`,
`min()`, `max()` and `avg()`. Gaps closed:

- `$total` held only the first item of the aggregator's result, where it "is
  set to the result of the aggregator expression": `(1 | 2 | 3).aggregate($total
  | $this, {}).count()` was 2, where fhirpath.js and the Java engine answer 3.
- `init` was evaluated on aggregate()'s own input, where 3.0.0 says it is
  "evaluated on the outer context": inside `select()`, its `$this` is the item
  `select()` is at, `3.select((10 | 20).aggregate($total + $this, $this))`
  being 33, as fhirpath.js answers; the validator answers empty there.
- `sum()` of an empty input was 0, where "If the input collection is empty
  (`{ }`), the result is empty."
- `sum()` and `avg()` refused quantities, which 3.0.0 lists for both and gives
  examples of: `( 5.5 'cm' | 4.7 'cm' | 4.8 'cm' ).avg() // 5.0 'cm'`. They
  add as `+` adds two quantities, a FHIR Quantity among them, in the more
  granular unit; units that do not combine give empty. A number beside them
  is an item of another type, though `+` reads one as of unit `'1'`.
- An item with nothing to add, a primitive with no value or a FHIR Quantity
  with a unit and none, makes the sum empty, as `+` answers for such an
  operand, not an item of another type.
- `sum()` and `avg()` over items of different types, `(1 | 'a').sum()`, were
  empty, where "All items in the input collection SHALL be the same type,
  otherwise an exception is thrown". Integers and Decimals together add as
  Decimals, whether that rule allows the implicit conversion being a question
  the text leaves open; `min()` and `max()` already raised the error.

`init` is evaluated in a scope of its own, so a variable it defines does not
reach the aggregator, as in fhirpath.js. `$total` grows with the aggregator,
`$total.combine($total)` doubling it, and is held to the maximum collection
size an evaluation is given, checked with the timeout on every item.

The validator implements neither `sum()` nor `avg()`, and its `$index` in
`aggregate()` answers what cannot all hold; see
[Measured, and not followed](#measured-and-not-followed).

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

Each operand of an operator and each argument of a function is navigated from
where the expression stands, and a path defined elsewhere as the model resolves
it, `Questionnaire.item.item` by `Questionnaire.item`, as evaluation navigates
them. Up to 1.14.4 the path leaked from one operand into the next, `item |
item.item` reading its right side from `Questionnaire.item`, and from one of
`iif()`'s arguments into the next, and such paths were not resolved; so the
analysis rejected 55 of the R4, R4B and R5 core constraints, all valid (#180).
Now it rejects four, each a defect of the specification: `cid-0` names
`ChargeItemDefinition.name` in R4 and R4B, and `cnl-0` `Evidence.name` and
`EvidenceReport.name` in R4B, elements those resources do not have.
`TestCoreConstraintsAnalyze` in `corpusdiff` holds it, over every constraint of
the core type definitions.

Four gaps closed since, none of which a core constraint met (#188):

- The index of an indexer navigated from its base and left its path there:
  `Patient.contact[link.count()].name` reported that BackboneElement has no
  element `name`. The index is now navigated from where the indexer stands,
  as evaluation reads it, and the base's path carries on.
- A context that is itself a path defined elsewhere, `Questionnaire.item.item`,
  was not resolved, so `foo.exists()` there reported nothing. The context is
  resolved once, as `Questionnaire.item`.
- A choice element written with its type suffix under a backbone element,
  `Observation.component.valueQuantity` or
  `Questionnaire.item.answerOption.valueCoding`, passed, while
  `Observation.valueQuantity` was rejected. Both are now.
- An element after a value of a System primitive type, `Patient.name.count().given`
  or `exists().value`, reported nothing, though such a value has no elements.
  A literal or a function whose result type is fixed, `count()`, `exists()`,
  `toString()` and the like, now types its result as a System value, and
  navigating into one is a fault. A FHIR primitive that `ofType()` or `as()`
  returns keeps its `id` and extensions, and is not one.

Two false faults went with them. `union()` and `combine()` kept their input's
type, though the argument's items join it, so `Patient.name.union(Patient.telecom).system`
reported that HumanName has no element `system`; their result is now of no
known type, as the `|` operator's is. And `aggregate()`'s `init` was navigated
from each item, though it is read on the outer context:
`Patient.name.aggregate($total + 1, telecom.count())` reported that HumanName
has no element `telecom`.

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

## A choice element is read only by the suffixes the model allows

FHIR's JSON writes a choice element only with its type suffix, `valueString`
for `value[x]`. Under a model, this engine read two things that are no
element, and both conformance suites missed them, since neither writes
invalid JSON nor puts a disallowed type inside a backbone element:

- A choice written bare, `"value": …`, was read as the element, and beside
  `valueString` it hid it: `extension.value` over both answered the bare one,
  and `children()` listed it.
- A choice inside a backbone element or a contentReference path
  (`Observation.component.value`, `Questionnaire.item.enableWhen.answer`,
  `Dosage.doseAndRate.dose`) was resolved by guessing the suffix, not by the
  model, so a type the version does not allow there was read:
  `component.valueCode`, `enableWhen.answerCode`.

HL7's Java engine and fhirpath.js both refuse the disallowed types
(`component.valueCode`, `enableWhen.answerCode`) and the bare extension value
(`difftest/batteries/choice-base-name`). Where a resource's choice is written
both ways, fhirpath.js answers the suffixed field and the Java engine refuses
the element; the text bears fhirpath.js out. Now a choice is read by the
model's types wherever it sits, and a bare key under a model is neither
navigated to nor a child (#219). `TestAChoiceIsReadOnlyByTheSuffixesTheModelAllows`
pins it under R4, R4B and R5, with cases the battery does not cover
(`Dosage.doseAndRate.dose`, `Timing.repeat.bounds`, `Parameters.parameter.part.value`,
`Patient.deceased`, contained resources, Bundle entries).

`children()` parts three ways on those resources. Over a choice written both
ways, this engine lists the suffixed field once, fhirpath.js lists it twice
(a defect of its own) and the Java engine refuses the element: on the
battery's Observation written both ways (`obs2.json`) 4, 5 and 3; on the
component written both ways (`obs.json`'s second) 2, 3 and 1. A suffix no
definition allows (`Observation.valueCode`) stays a child here and in
fhirpath.js, as any key no definition has does (`Basic.codeFoo` reads `y` in
both), where the Java engine leaves it out; navigation to the element gives
nothing for it in all three. Measured, and kept: the engine reads what a
document holds unless the model says it is something else, and a key it
does not know is not one it can say that of.

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

fhirpath.js 5.2.0 does the same: `1 is FHIR.Any` and `Patient.gender is
strng` end with "cannot be resolved to a valid type identifier". HL7's Java
engine answers false. The operators `is` and `as` read their specifier as
`is()` and `as()` do: up to 1.16.4 the operator form checked no specifier, so
`1 is FHIR.Any` was false where `1.is(FHIR.Any)` was an error, and kept the
backticks that delimit a part, so ``1 is System.`Integer` `` was false. Now
both forms answer alike, as fhirpath.js does; the Java engine answers ``is
FHIR.`code` `` false on a code too (#188).

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

Text departed from: FHIRPath 2.0.0 and 3.0.0 alike. The validator follows it
from R5 on; see [What depends on the FHIR version](#what-depends-on-the-fhir-version).

The specification is unambiguous about the operator: "If there is more than one
item in the input collection, the evaluator will throw an error." The official
suite tests it (`Patient.name.as(HumanName).use`), and with a model older than
R5 this engine does not comply — it filters by type, as `ofType()` does. With
an R5 model it raises the error, as the validator does.

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

HL7's Java FHIRPath engine (`org.hl7.fhir.r5.fhirpath.FHIRPathEngine`, in
org.hl7.fhir.core), the one the official validator runs, resolves it by version:

```java
private void initFlags() {
  if (!VersionUtilities.isR5Plus(worker.getVersion())) {
    doNotEnforceAsCaseSensitive = true;
    doNotEnforceAsSingletonRule = true;
  }
}
```

Before R5 it filters, exactly as this engine does; from R5 on it enforces the
singleton rule. This engine does the same, by the version a model declares
through `VersionedModel`; a model that declares none is taken for pre-R5. The
Java engine also sets a flag for case sensitivity on type names only from R5
on, but a name in the wrong case is no type in either version there:
`as(humanName)` is an error with `-version 4.0.1` and with `-version 5.0.0`, as
it is here with a model that tells which names are types, `TypeRegistry`, as
the generated models do; with one that does not, the name is taken at face
value and the cast filters to nothing.

HL7 came to the same conclusion, in stages. The invariant is worded differently
in each of the three versions:

| | guard | operator | `where(... = '#')` clauses |
|---|---|---|---|
| R4 | — | `as()` | canonical, canonical — duplicated |
| R4B | `id.exists()` | `as()` | canonical, uri — fixed |
| R5 | — | `ofType()` | canonical, canonical — reintroduced |

So R4B fixed a copy-paste defect and added a guard, and R5 switched to the
filtering function while bringing the defect back. An engine has to evaluate
whichever wording the resource's version publishes; `TestDom3AcrossVersions`
(`fhir_functions_test.go`) runs all three against every model configuration.

Measured in the HL7 validator 6.10.2: `Patient.name.as(HumanName).count()` over
three names is 3 with `-version 4.0.1` and an error, "Attempt to use 'as()' on
more than one item", with `-version 5.0.0`, and this engine answers the same with
the R4 and the R5 models.

So the divergence is only needed for **R4** invariants, and only there is it
taken: one suite case is a smaller loss than dom-3 erroring on every
DomainResource of the version most deployments validate against.

Worth noting while reading dom-3: one clause is duplicated in both versions —
`descendants().where(ofType(canonical) = '#').exists()` appears twice, where the
symmetry with the first branch calls for `uri` and `url`. That defect is
upstream and unrelated to this engine.

## A divergence taken on purpose: `and`, `or` and `implies` decide from the left

Text departed from: FHIRPath 2.0.0 and 3.0.0 alike, read strictly: both allow
short-circuit evaluation and say "implementations must not change semantics
with short-circuit evaluation", which an error in the right operand the left
one decides makes this do. The validator short-circuits, in R4 and R5.

When the left operand of `or` is `true`, of `and` is `false`, or of `implies`
is `false`, this engine answers without evaluating the right one. fhirpath.js
evaluates both, and an error the right operand raises ends its evaluation.

The specification allows either. Section 6.5 says implementations "may use
short-circuit evaluation to reduce computation", then that "authors should not
rely on such behavior, and implementations must not change semantics with
short-circuit evaluation". Read strictly, the second half would make the right
operand's error the answer, which is what fhirpath.js does.

FHIR's published invariants are written for the other reading:

    tim-9 (R4, R4B)  offset.empty() or (when.exists() and
                       ((when in ('C' | 'CM' | 'CD' | 'CV')).not()))
    eld-11 (R5)      binding.empty() or type.code.empty() or
                       type.code.contains(':') or ...

On a timing with two `when` values and no offset, `in` raises an error although
`offset.empty()` is true. On every element with more than one type, which is
every choice element, `contains` raises an error although `binding.empty()` is
true. R5 rewrote tim-9 to `when.select($this in ...).allFalse()`, which does not
depend on it; eld-11 still does.

The HL7 validator short-circuits, in every FHIR version:

```java
private List<Base> preOperate(List<Base> left, Operation operation, ExpressionNode expr) {
  if (left.size() == 0) return null;
  switch (operation) {
  case And:     return isBoolean(left, false) ? makeBoolean(false) : null;
  case Or:      return isBoolean(left, true) ? makeBoolean(true) : null;
  case Implies: return asBool(left, expr) == Equality.False ? makeBoolean(true) : null;
  ...
```

and does not evaluate the right operand when that returns a result. This engine
follows it: only a single Boolean decides a result, an empty left operand leaves
the right one to three-valued logic, which `{} or true` needs, and so does one
that is not a Boolean. No case in the official suite asserts an error behind a
deciding left operand, so its results are unchanged.

A left operand of more than one item is the one place the two part. Either way
the evaluation ends in an error, since the singleton rule refuses the operand
whatever the right one holds; what differs is which error. The validator raises
it before the right operand only for `implies`, through `asBool`; for `and` and
`or` it evaluates the right one first, so an error the right one raises is the
one reported. This engine refuses the left operand of all four operators, `xor`
included, before evaluating the right one: the error names the operand at fault,
and no time is spent on an operand whose answer cannot change the outcome.

Measured in the HL7 validator 6.10.2, R4 and R5: `true or
Patient.name.given.startsWith('A')`, over a name with two given names, is true,
and so are `false and ...` false and `false implies ...` true, here as there.
Measured again across the three engines alongside the boolean-logic cases,
which leave these expressions out (#167): `false and (true | false)`, `true or
(true | false)` and `false implies (true | false)` are false, true and true here
and in the Java engine, and an error in fhirpath.js. A left operand of more
than one item is an error in all three whatever the right one holds, as in
`(true | false) and false` and `(true | false) xor {}`; so is an empty left
operand beside a right one of more than one item, under each of `and`, `or`,
`xor` and `implies`.

An expression that has to run on engines that evaluate both operands should
still guard the right side with `iif()`, as the specification advises.

## A divergence taken on purpose: a FHIR Quantity maps by its code or unit

Text departed from: FHIR's FHIRPath page, R4 and R5 alike. The validator
reports nothing on the Ranges in local units the strict reading fails.

FHIR conditions the mapping from a FHIR Quantity to a `System.Quantity`:

> The Mapping from FHIR Quantity to FHIRPath System.Quantity can only be
> applied if the FHIR Quantity has a UCUM code - i.e. a system of
> http://unitsofmeasure.org, and a code is present.

Read strictly, a quantity in a local unit — tablets, a code from another
system, a unit with no code — is no `System.Quantity` at all, so it neither
compares nor bounds. This engine applies the condition to the calendar units
only, as the section above describes, and otherwise maps a quantity by its code,
or its unit where it has none.

The strict reading breaks FHIR's own invariants on ordinary data. R5 writes
`rng-2` as

    low.value.empty() or high.value.empty() or
      low.lowBoundary().comparable(high.highBoundary()).not() or
      (low.lowBoundary() <= high.highBoundary())

and with no `System.Quantity` the boundaries are empty, so the whole expression
is empty and the invariant fails on every Range in a local unit — a dose of 1 to
2 tablets. The HL7 validator reports nothing there. It was tried, in the
review of #112, and withdrawn for that reason; a test now holds `rng-2` true on
such Ranges.

What the condition does decide is kept. A **Money** is not a quantity: it has a
value, but a currency is no unit, and mapping it with none made a sum of money
comparable to a mass (`Claim.total.comparable(10 'mg')` was true) and equal to a
unitless quantity. fhirpath.js refuses both; the HL7 validator maps only types
named Quantity. A Money compares with no quantity, so `comparable()` on one is
false, as fhirpath.js and the HL7 validator answer; up to 1.14.1 it was empty.
Two Money values compare under `=` by amount and currency, so 30 USD equals 30.0 USD however each is written, and — new, since a Money read
as a quantity had no unit — no longer equals 30 EUR. Under `~` they compare as
the objects they are, as every complex type does.

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

So `Long` is not implemented, and `45L` does not parse. `toLong()` and
`convertsToLong()` are `toInteger()` and `convertsToInteger()` under another
name, decided with the maintainer in the conversions area: they give an
Integer of the same 64-bit range, so `'5000000000'.toLong()` is `5000000000`,
and only `is System.Long` tells them apart. Up to 1.15.1 they did not exist.
The divergence is the range of `Integer`, recorded here rather than left for
someone to discover from a surprising answer.

Arithmetic that crosses either end of that range answers empty, as the
specification says it does: "Operations that cause arithmetic overflow or
underflow will result in empty (`{ }`)".

    9223372036854775807 + 1           // { }
    -(-9223372036854775807 - 1)       // { }
    (-9223372036854775807 - 1).abs()  // { }
    (9223372036854775807 | 1).sum()   // { }
    -9223372036854775807 - 1          // -9223372036854775808

The same holds for `-`, `*`, `div`, unary `-`, `abs()`, `sum()` and
`aggregate()` over `+`; the ends of the range are values. `toInteger()` takes
no decimal at all: see [Conversions](#conversions). `div` and `mod` over decimals truncate the
decimal itself: `100000000000000000000.0 div 1.0` is
`100000000000000000000`, where it wrapped to `7766279631452241920`, and
exactly, where a quotient rounded to 16 places first made
`0.99999999999999999 div 1.0` 1 and its `mod` negative. The least Integer has
no literal: a literal has no sign, `9223372036854775808` past the range is read
as a Decimal, and `-` negates it, so `-9223372036854775808` is a Decimal;
`-9223372036854775807 - 1` is the Integer. Up to 1.13.4 the sum
wrapped, as Go's `int64` does, to `-9223372036854775808`. `ceiling()`,
`floor()` and `truncate()` read a decimal as itself, where a float held
`9007199254740993.5` as `9007199254740994`, and answer empty past the range. A
literal too large for `int64` is read as a `Decimal`, which is why
`9223372036854775808` answers with itself.

The HL7 validator holds an Integer in 32 bits and wraps: `2147483647 + 1` is
`-2147483648` there. That is not followed; see
[Measured, and not followed](#measured-and-not-followed).

## Where this engine follows the HL7 validator over the specification text

The HL7 validator is what a FHIR server's users check their resources against.
Where its FHIRPath answers differ from what the specification text says, an
engine that follows the text accepts what the validator rejects, or the other
way round, and the server built on it cannot be used in its place. So where
the two disagree, this engine answers as the validator does, and each such
case is recorded here: what the validator answers, measured, the version it
was measured in, and the text it departs from. A test pins each answer, so a
validator release that changes one shows up when it is measured again.

Only an answer can be followed. Where the validator ends in an exception, or
gives two answers that cannot both hold — an equality that is not symmetric,
a string that converts to a date it cannot make — the engine keeps its own.
Where fhirpath.js answers otherwise and the text bears it out, the engine
gives that answer, as rule 1 under
[When 2.0.0 and 3.0.0 disagree](#when-200-and-300-disagree) has it; a coherent
answer is set aside only for the reasons that rule names. Either way the case
is recorded under [Measured, and not followed](#measured-and-not-followed).
The entries below are those where the text says nothing the libraries'
answers can be checked against, where fhirpath.js answers as the validator
does, or where the validator reaches a published constraint's verdict through
its answer: an element written `{}`, through which pat-1 and bdl-4 fail there,
where fhirpath.js answers `exists()` true as the text does.

Each answer was measured with `validator_cli.jar -fhirpath "<expression>"
<file> -version 4.0.1`.

### An element that holds no value

Measured in the HL7 validator 6.10.2.

Text departed from: FHIRPath 2.0.0 and 3.0.0 alike, `exists()` and `empty()`.

An element written as `{}`, or as an object under which nothing holds a
value — `{"given": []}`, `{"extension": [{}]}` — has neither a value nor
children, which ele-1, inherited by every element, forbids: "All FHIR elements
must have a @value or children". The validator reads it as an item of its
collection, located where the JSON writes it, for every function but two:
`exists()` and `empty()` answer as though it were not there.

| On | Expression | Validator and this engine |
|---|---|---|
| `"entry": [{}, {"response": {"status": "200"}}]` | `Bundle.entry.count()` | 2 |
| | `Bundle.entry.all(response.exists())` (bdl-4) | false |
| | `Bundle.entry[0].count()` | 1 |
| | `Bundle.entry[0].exists()` | false |
| | `Bundle.entry[0].empty()` | true |
| | `Bundle.entry.first().exists()` | false |
| `"contact": [{"name": {}}]` | `Patient.contact.name.exists()` (pat-1) | false |
| | `Patient.contact.name.exists(true)` | true |
| | `Patient.contact.name.where(true).exists()` | false |
| | `Patient.contact.exists()`, `Patient.exists()` | false |
| `{"resourceType": "Patient"}` | `Patient.exists()`, `%resource.exists()` | false |
| `"_active": {}` | `Patient.active.count()` / `.exists()` | 1 / false |
| `{"family": ""}`, `{"id": "x"}` as `name` | `name.exists()` | true: a string, even empty, is a value |

FHIRPath defines both functions on the collection alone. 2.0.0: `empty()`
"Returns `true` if the input collection is empty (`{ }`) and `false`
otherwise", and `exists()` "is the opposite of `empty()`, and as such is a
shorthand for `empty().not()`", and with criteria "shorthand for
`where(criteria).exists()`"; `count()` "Returns 0 when the input collection is
empty". By that text a collection that counts 1 is not empty, `exists(true)` and
`where(true).exists()` agree, and a resource that holds only its
`resourceType`, which ele-1 does not reach, exists. FHIR says such an element is
invalid, not that it is absent.

A root with no type written `{}`, which the validator does not read, holds no
value either, as a resource with only its `resourceType` does: `exists()` on
it is false and `empty()` true. Nothing else about it changes: it is still
the input, and an expression that does not ask whether it exists reads it as
before.

`iif()`, `coalesce()` and the boolean reading of a collection see such an
element as an item, as the validator does: on a contact whose `name` is `{}`,
`iif(Patient.contact.name, 'a', 'b')` is `a`, and
`coalesce(Patient.contact.name, 'x').exists()` false.

A primitive written with no value, only the element beside it, is the
exception: its value is null, and where a value is wanted it is empty, as in
the validator.

It is still of the primitive's type, as in the validator: `"_active"` holding
only a data-absent-reason is a `boolean`, so `Patient.active is boolean` is true
and `ofType(boolean)` keeps it, and a `_given` entry beside a `null` is a
`string`. The type is the one the model declares for the field, or, read as a
choice element, `Observation.value` over `_valueString`, the one its name
tells, with or without a model. Without either there is none to give, and it is
an `Object`: `_active` with no model, and `_valueString` read by its own name,
`Observation.valueString`, with no model to say it is a choice.
Up to 1.13.3 it was an `Object` everywhere, and filters by FHIR type left it
out. Its namespace is FHIR's, as for a primitive with a value, where the
validator answers `System.Boolean` from `type()` and false to `is
FHIR.boolean`: see [Measured, and not followed](#measured-and-not-followed).
Having no value, it is no System value: `ofType(Boolean)` leaves it out, where
it keeps a `boolean` with one, and the validator raises an exception on both.

`Resource.id`, `Element.id` and `Extension.url` have `System.String` as their
type code, and name their FHIR type in `structuredefinition-fhir-type`, which
"specifies the applicable FHIR type": `Extension.url` a uri in every version,
`Resource.id` and a datatype's id a string in R4 and an id from R4B, a
primitive's id and a backbone element's a string, and `Element.id` a string
but in R4B, which makes it an id. Each path is its own: R5 makes
`BackboneElement.id` an id and `Patient.contact.id` a string. The validator
types each field by what its version's definitions name, measured with
`-version 4.0.1`, `4.3.0` and `5.0.0` (6.10.2), and so does this engine with a
model that implements `FHIRTypes`, as gofhir/models' generated ones do:

| Expression | Validator | fhirpath.js | This engine |
|---|---|---|---|
| R5 `Patient.id.is(id)`, `.as(id)`, `.ofType(id)` | true, the id | false, empty | as the validator |
| R5 `Patient.id.is(System.String)` | false | true | as the validator |
| `Extension.url.is(uri)` | true | false | as the validator |
| R4 `Patient.id.is(System.String)`, `type()` | true, `System.String` | true, `System.String` | as both |
| R4 `Patient.id.is(id)`, `is(uri)`; `Extension.url.is(string)` | false | false | as both |
| R4 `Patient.id.is(string)` | true | false | as the validator |

So a field its definitions name a uri or an id is that FHIR type, and one they
name a string stays a System.String, as both engines read it. A System.String
is none of the FHIR types that convert to it, and with a model a FHIR type is
none of the others that share its System type: a uri is no string. Over 76
expressions on R4 and R5, this engine differs from both engines at once in one:
the `type().namespace` of R5's `Patient.id`, `System` in the validator, which
names it `Id`, and in fhirpath.js, which names it `String`; see the `type()`
row of [Measured, and not followed](#measured-and-not-followed).

Up to 1.14.0 no model named those types, and a System.String from the model
was taken for every FHIR type that converts to it: `Patient.id.is(id)`,
`is(uri)` and `is(code)` true in every version. That reading hid a walk that
lost its types beneath a path defined elsewhere: `descendants()` beneath R4's
`PlanDefinition.action.action` read `definitionCanonical` as a System.String,
which dom-3's `descendants().as(canonical)` found only by that reading. The
walk now types it a canonical, and dom-3 answers as it did over the R4, R4B
and R5 examples. A model that does not implement `FHIRTypes` has such a field
read as a System.String.

Written with no value, such a field is still a FHIR primitive, of the type its
definitions name: `Patient.id is string` true, `is id` false in R4 and true
from R4B, `is String` false, and `Extension.url is uri` true, as the validator
answers with a value and without one.

| On | Expression | Validator and this engine |
|---|---|---|
| `"_active": {}` | `Patient.active or false`, `Patient.active.not()` | empty |
| | `iif(Patient.active, 'a', 'b')` | `b` |
| `"_active": {"extension": [...]}` | `Patient.active or false`, `Patient.active.not()` | empty |
| `"_active": {}` | `Patient.active = true`, `Patient.active = false` / `!= true` | false / true |
| | `allTrue()`, `anyTrue()`, `allFalse()`, `anyFalse()` | false |
| `"given": [null, "a"], "_given": [{}, null]` | `Patient.name.given.join(',')` | `a` |

`=` compares the null, which equals no value. Where the validator raises a
NullPointerException, `<`, the arithmetic operators, unary `-`, the String
functions, `duration()` and `difference()`, this engine reads the value as
empty: `"_birthDate": {}` makes `birthDate < @2020` empty. `given + 'x'` is
empty too, where the validator answers `nullx`, Java's concatenation of a
null; `&` reads it as the empty string, as it does an empty operand, so
`gender & 'x'` is `x`. Up to 1.13.2 a primitive with
only an extension was read as an object: a boolean reading took it for true,
`join()` wrote it as `{}`, and `allTrue()` raised a type error.

`~` compares children as navigation reads them, so a member written `{}` is a
child: `contact[0] ~ contact[1]` is false between a contact whose `name` is `{}`
and one with no `name`, as in the validator. An element beside a primitive
that holds no value adds nothing to a value beside it: `"given": ["J"],
"_given": [{}]` is equivalent to `"given": ["J"]`. Beside a `null` it is the
item, and is not. A value and the element beside it are one item, which pairs
with an item of the other side in any order: `"given": ["x", "y"], "_given":
[null, {"id": "e"}]` is equivalent to `"given": ["y", "x"], "_given": [{"id":
"e"}, null]`, and not to `"given": ["x", "y"], "_given": [{"id": "e"}, null]`.
Up to 1.16.2 the values and the elements were paired apart, each in any order,
so both were equivalent, as was a position holding only an element against a
value with that element (#175).

### A primitive's value is not a child

Measured in the HL7 validator 6.10.2.

Text departed from: FHIR's FHIRPath page, R4 and R5 alike.

A FHIR primitive's children are its `id` and its extensions, and `descendants()`
reaches each primitive once. Its value is not a child; it is still read by
name, `birthDate.value`, and by `hasValue()` and `getValue()`.

| On | Expression | Validator and this engine | 1.10.5 to 1.13.x |
|---|---|---|---|
| `"birthDate": "2020"` | `Patient.birthDate.children().count()` | 0 | 1 |
| `"birthDate": "2020", "_birthDate": {"id": "x", "extension": [{"url": "u", "valueString": "v"}]}` | `Patient.birthDate.children().count()` | 2 | 3 |
| | `Patient.birthDate.descendants().count()` | 4 | 8 |
| `"name": [{"family": "F", "given": ["A", "B"]}]` | `Patient.name.descendants().count()` | 3 | 6 |
| | `Patient.descendants().where($this = 'A').count()` | 1 | 2 |
| `"_active": {"extension": [{"url": "u", "valueBoolean": false}]}` | `Patient.active.descendants().count()` | 3 | 5 |

FHIR's FHIRPath page says the opposite: "FHIR primitives have a value child,
but, as described above, they are automatically cast to FHIRPath primitives
when comparisons are made, and that the primitive value will be included in
the set returned by children() or descendants()." The validator's `children()`
lists the element's child nodes, and the value is not one. Read as a child, a
filter by value over `descendants()` found a primitive and its value both, and
any count of `children()` or `descendants()` differed from the validator's.

### A quantity's comparator is not read

Measured in the HL7 validator 6.10.2.

Text departed from: none states a rule. FHIR marks `comparator` a modifier;
neither FHIRPath version nor FHIR's FHIRPath page says how a bound maps.

A FHIR Quantity with a `comparator` is read as its value: `< 5 'mg'` compares,
is equivalent and is bounded as `5 'mg'`, and converts with `toQuantity()`.
It ended the evaluation with an error, so a constraint over such a quantity
failed where the validator answers. The comparator is still read by name.

| `"valueQuantity": {"value": 5, "comparator": "<", ...UCUM mg}` | Validator and this engine | Before |
|---|---|---|
| `Observation.value = 5 'mg'` | true | error |
| `Observation.value < 6 'mg'` | true | error |
| `Observation.value.lowBoundary()` | `4.50000000 'mg'` | error |
| `Observation.value.toQuantity()` | `5 'mg'` | empty |
| `Observation.value.convertsToQuantity()` | true | false |

FHIR marks `comparator` a modifier, "How the value should be understood and
represented", and states no FHIRPath rule for it; read as its value, a bound
answers for a measurement nobody made: `< 3 'mg' > 1 'mg'` is true. fhirpath.js
refuses it with the error this engine raised.

Arithmetic reads the value as well: `< 5 'mg'` `+ 1 'mg'` is `6 'mg'`. The
validator raises an exception on arithmetic with any FHIR Quantity, comparator
or none, so there is no answer of its to follow there; see
[Measured, and not followed](#measured-and-not-followed).

### Two FHIR Quantities are equivalent as quantities

Measured in the HL7 validator 6.10.2.

Text departed from: none in comparing two FHIR Quantities as quantities, which
FHIRPath 2.0.0 and 3.0.0 alike, FHIR's FHIRPath page and the validator all do;
the engine departed from them before. Across units the four part ways on the
precision compared at: 2.0.0 in the most granular unit, 3.0.0 in the least,
which this engine follows, the validator in UCUM's base unit; see
[When 2.0.0 and 3.0.0 disagree](#when-200-and-300-disagree) and
[Measured, and not followed](#measured-and-not-followed). A quantity in a local
unit, which the page does not map, compares here by its unit: see
[a FHIR Quantity maps by its code or unit](#a-divergence-taken-on-purpose-a-fhir-quantity-maps-by-its-code-or-unit).

`~` between two FHIR Quantities compares them as quantities, as `=` does and as
it did against a literal: `1 'g'` written in one is equivalent to `1000 'mg'`
written in the other, and `ms` (millisecond) is not `mS` (millisiemens). Both
read as quantities only objects whose type is Quantity or one of its profiles
(SimpleQuantity, MoneyQuantity, Age, Distance, Duration, Count), as the
validator maps only those: with a model, two adjudications with the same
`value` and another category are neither equal nor equivalent, where `=` had
read them as equal quantities. Without a model an object is a Quantity when it
has a numeric value beside a unit, a code or a system, or is read from a field
that names it, `valueQuantity`; one with a numeric value alone is of no known
type and still converts, so a Range written without units orders, and two such
adjudications compare as quantities, as before.

A Quantity inside a complex value is equivalent as a quantity too, so the
answer does not depend on how deep it sits: a reference range whose `low` is
`1 'g'` is equivalent to one whose `low` is `1000 'mg'`, as in the validator.
There it has to be a Quantity by its type, model or unit: an adjudication
nested in a complex value compares as the object it is.

FHIRPath's text agrees: a FHIR Quantity maps to a System.Quantity, and "the
dimensions of each quantity must be the same, but not necessarily the unit".

### Measured, and not followed

Where the validator answers with an error, or two of its answers cannot both
hold, there is no behavior to follow. Where it answers coherently, its answer
is set aside only for a reason given in the row: fhirpath.js answers
otherwise and the text bears it out, it rests on a choice this engine made on
its own grounds, the 64-bit Integer, or no published constraint reaches it and
matching it would cost a rewrite. These were measured in 6.10.2 and the engine
keeps its own answer.

| Expression | Validator | This engine | Why not followed |
|---|---|---|---|
| `hasValue()` on a single complex value: `"maritalStatus": {"id": "x"}` | true, for a BackboneElement too | false, as fhirpath.js answers | FHIR's FHIRPath page, R4 and R5 alike: "Returns true if the input collection contains a single value which is a FHIR primitive". 1.13.4 followed the validator. The validator accepting such an element is no evidence for it: with `hasValue()` true, ele-1 holds on every complex element, so it holds there whether or not the validator evaluates it. Here ele-1 fails on it (#161) |
| `'a b' ~ 'a\tb'`, `'a\nb' ~ 'a\rb'`, a no-break space against a space: whitespace characters of different kinds | false, a tab is not a space | true, as fhirpath.js answers | FHIRPath 2.0.0 and 3.0.0 alike: whitespace is "normalized", every whitespace character equivalent. 1.13.4 followed the validator. A run of whitespace is not collapsed, `'a b' ~ 'a  b'` false, nor an end trimmed, `' a' ~ 'a'` false, in the three |
| `a = b` between complex values, `b` with a child more | `a = b` true, `b = a` false | false, false | Equality that is not symmetric |
| `~` between complex values whose primitive carries an element beside another value: `"given": ["x", "y"], "_given": [null, {"id": "e"}]` against `"_given": [{"id": "e"}, null]` | true, a primitive compared by its value alone | false, as fhirpath.js answers | "For complex types, equivalence requires all child properties to be equivalent, recursively", and the element beside a value is that item's: an extension on the second given is not one on the first. The validator answers true too for an element against none, `"given": ["x"], "_given": [{"id": "e"}]` against `"given": ["x"]`. Where a position holds only an element, `"given": [null, "y"], "_given": [{"id": "e"}, null]` against `"given": ["y"], "_given": [{"id": "e"}]`, it answers false, as fhirpath.js and this engine do (#175) |
| `~` between quantities in different units: `2.4 'm' ~ 200 'cm'` | true; `1.501 'kg' ~ 1500 'g'` false | false; false | Coherent: both sides in the UCUM base unit, rounded to the less precise. Matching it means rewriting quantity equivalence, and no published constraint compares quantities in different units with `~` |
| `comparable()` on quantities in a local unit, `Range.low` and `high` in tablets | false, though `low < high` is true | true | Not comparable, yet compared. rng-2 holds in both |
| `extension(url)` with a url of several items, `extension('zz' \| 'a')`, or one that is no String, `extension(1)` | the first item's extensions: `'zz' \| 'a'` empty, `'a' \| 'zz'` the `a` extension; `[]` for `1` or `true` | an error, as fhirpath.js answers | Both FHIR pages declare `extension(url : string)`, and FHIRPath 2.0.0 and 3.0.0 alike end the evaluation for an argument of several items, or one not of the required type, passed to a single-valued parameter |
| `Patient._birthDate`, `Patient.name._given`: the element FHIR's JSON writes beside a primitive, named as the JSON names it | an error | the element, as fhirpath.js answers | The validator refuses the name rather than answering otherwise, so there is no behaviour to follow; `Patient.birthDate.id` and `.extension` read the same element by the model's name |
| `'2021-02-30'.convertsToDate()` | true, though `@2021-02-30` does not parse | false, as fhirpath.js answers | Converts to nothing |
| `is()` and `type()` across FHIR and System types | `family.is(System.String)` true, though `family.as(System.String)` is empty; `type()` of a FHIR string `System.String`, and of an R5 `Patient.id` `System.Id` | false, as fhirpath.js answers; `FHIR.string` | Is, and is not. `as()` is followed: it answers as `is()`, empty with a model, as in fhirpath.js, where up to 1.14.1 it was the value |
| `~` with a calendar duration or a temperature: `1 year ~ 1 'a'` | NullPointerException | true | An exception |
| `~` between FHIR Quantities in units no conversion knows, `"unit": "Tablet"` and `"unit": "tablet"` | NullPointerException | false | An exception |
| Arithmetic with a FHIR Quantity: `Observation.value + 1 'mg'` | an exception | the sum | An exception |
| `2147483647 + 1` | `-2147483648` | `2147483648` | 32-bit overflow |
| `1.2 / 1.8 = 0.67` | true | false, as fhirpath.js answers | 3.0.0's own example is false: the quotient is not rounded to the other side's precision under `=` |
| Times and date/times at different precisions: `@T10 = @T10:30`, `@T10 < @T10:30` | false, true: a missing minute read as zero | empty, as fhirpath.js answers | "If one input has a value for the precision and the other does not, the comparison stops and the result is empty" |
| `@T10:30:31.000 = @T10:30:31` | false | true, as fhirpath.js answers | Seconds and milliseconds are one precision, compared as a decimal |
| `~` between decimals or quantities of one unit at different precisions: `1.10 ~ 1.14`, `1.10 'mg' ~ 1.14 'mg'` | false | true, as fhirpath.js answers | Equivalence rounds "to the precision of the least precise operand" |
| `<`, `>`, `<=` and `>=` with an operand of more than one item: `Patient.name.given >= 'A'` | empty | an error, as fhirpath.js raises | "the evaluator will throw an error if either collection has more than one item"; a constraint that compares a repeating element passes silently there |
| Ordering quantities in commensurable units: `4 'm' > 4 'cm'` | false, and `4 'm' <= 4 'cm'` true | true, false, as fhirpath.js answers | The numbers compared without their units, though `4 'm' = 400 'cm'` |
| `2 years = 2 year` | empty | true, as fhirpath.js answers | The plural and the singular of one calendar keyword |
| An Age in UCUM `'a'` against calendar years: `Observation.extension.value = 41 years` | false, `> 40 years` empty | true, true, as fhirpath.js answers | FHIR maps UCUM `'a'` to the calendar `year`: see [Rules the FHIRPath suite does not reach](#rules-the-fhirpath-suite-does-not-reach) |
| An Integer against a quantity: `23 = 23 '1'`, `2 < 3 '1'`; `2 '1'.comparable(3)`, `1.comparable(2)` | false, empty; false, false | true, true; true, true, as fhirpath.js answers | 2.0.0's implicit conversion to Quantity, as above; 3.0.0 gives both `comparable()` examples as true |
| DateTime literals to the minute, `@2012-01-01T10:30 = @2012-01-01T10:30`; `2 years > 1 year`; `Patient.telecom.rank < 2`, a positiveInt against an Integer | an exception | the answer, as fhirpath.js gives it | An exception |
| Date and time arithmetic by an amount past the calendar or the int range: `@2020-01-01 + 18446744073709551617 days`, `@2020-01-01 + 7980 years`, `@T10:00 + 9223372036854775807 hours` | wraps, `@2020-01-02`; no bound on the year, `10000-01-01`; `@T09:00`. fhirpath.js answers `NaN-NaN-NaN` and `10000-01-01` | an error, "If the resulting year is out of range, an error is thrown"; `@T17:00`, the amount modulo a day. A clock amount within the calendar but past what Go's `time.Duration` holds, about 292 years, shifts too: `@2020-01-01T10:00:00Z + 3000000 hours` is `@2362-03-29T10:00:00Z`. A date component on a time is an error whatever its amount, `+ 0 days` and `+ 0.5 days` included: "This includes attempting to add date components to a Time". Amounts are Go `int`s, so on a 32-bit platform a clock amount past about 24 days in milliseconds is past the calendar too | Wraps past the range; a Date past year 9999, which the type does not hold and the text makes an error. `@9999-12-31 + 1 year` is `10000-12-31` in both reference engines |
| `$index` in `exists()`, `all()` and `where()`, a `where()` within `select()` among them: `(10 \| 20 \| 30).exists($index = 2)`, `Patient.name.where($index = 1)` | false, `all($index < 2)` true, `where($index = 1)` empty: 0 in every iteration, where `select()` sets it | true, false, the second name, as fhirpath.js answers | Not set there |
| `subsetOf()` and `supersetOf()` with decimals written to other precisions: `1.0.subsetOf(1.00 \| 2.0)` | false | true, as fhirpath.js answers | Membership is by `=`, under which "trailing zeroes after the decimal are ignored" |
| `distinct()`, `isDistinct()`, `subsetOf()` and `repeat()` over dates at different precisions: `@2012.combine(@2012-01)`, `@2012.subsetOf(@2012-01)`, `1.repeat(@2012.combine(@2012-01)).count()` | `distinct().count()` 0, `isDistinct()` empty, `subsetOf()` true, `repeat()` 1 | 2, true, false, 2, as fhirpath.js answers | No item kept of two, neither said equal; a member of what holds no equal to it. 3.0.0: an empty `=` is not a match |
| `ofType(System.Quantity)` on a quantity literal: `'a'.combine(5 'mg').ofType(System.Quantity).count()` | 0 | 1, as fhirpath.js answers | A quantity literal is a System.Quantity |
| Characters counted as UTF-16 code units: `'🔥'.length()`, `'a🔥b'.indexOf('b')`, `'a\uD83D\uDD25b'.toChars()` | 2, 3, four items, the pair cut in two | 1, 2, three items: Unicode scalar values, as 3.0.0 states | fhirpath.js counts as the Java engine does, and both cut a pair into halves that are no string |
| A string function given more than one item: `Patient.name.given.startsWith('P')` | true, from one of them; `upper()`, `toChars()` empty | an error, as fhirpath.js raises | "If the input collection contains multiple items, the evaluation of the expression will end and signal an error": a constraint over a repeating element passes silently there |
| A string function given a value that is no String: `1.startsWith('1')`, `true.upper()` | empty | an error, as fhirpath.js raises | The input must be a String |
| A string or encoding function, `skip()` or `take()` given an argument of more than one item: `'abc'.startsWith('a' \| 'b')`, `'a,b;c'.split(',' \| ';')`, `'abc'.substring(0 \| 1)`, `Patient.telecom.use.take(1 \| 2)` | false, `a` and `b;c`, `abc`, the first item: an answer from the first | an error, as fhirpath.js raises | An argument "that evaluates to a collection with multiple items" ends the evaluation |
| `decode()` of bytes that are no UTF-8: `'ff'.decode('hex')` | the replacement character | empty; fhirpath.js raises "URI malformed" | "If the content being decoded is not a valid UTF-8 value, the result will be empty" (3.0.0) |
| `substring(3, {})`, `substring(3, 0)`, `''.upper()`, `''.lower()` | empty | `defg`, `''`, `''`, `''`, as fhirpath.js answers | An empty length is no length, and the empty string is a string |
| `matchesFull()` with an empty input or argument | false | empty, as fhirpath.js answers | Empty in, empty out |
| A Math function on what is no number: `'-5'.abs()`, `true.ceiling()`, `1 'cm'.exp()` | empty | an error, as fhirpath.js raises | The input is "not compatible with the expected type" |
| A Math function's number argument given a String: `16.log('2')`, `2.power('3')`, `3.14159.round('1')` | converts it: `4.0`, `8.0`, `3.1` | an error, as fhirpath.js raises | An Integer or a Decimal is expected, and a String converts to one only explicitly, as for `skip()` |
| `ceiling()`, `floor()`, `round()` and `truncate()` of a quantity: `1.1 'cm'.ceiling()` | empty | `2 'cm'`, as fhirpath.js answers | 3.0.0 defines each on a Quantity, its unit kept |
| `0 / 0` | `0` | empty | "12 / 0 // empty ({ })" |
| A calendar duration in `*` and `/`: `12 day * 45 'm'`, `2 weeks * 3 days` | in seconds: `46656000 'm.s'`, `313528320000 's2'` | empty | 3.0.0: "Multiplication involving calendar units (apart from the UCUM default unit `'1'`) will return empty" |
| Decimal arithmetic at eight places and beyond: `0.00000001 + 0.00000001 = 0.00000002`, `5.5 div 0.7 = 7`, `5.5 mod 0.7 = 0.6`, `0.00000005.round(7) = 0.0000001` | false | true, as fhirpath.js answers | Decimals add, divide and round as decimals; the validator's own answer differs from the exact one in its last places |
| `+` and `-` between literal quantities: `3 'cm' + 2 'cm'`, `5 'mg' - 2 'mg'` | an exception, and empty | `5 'cm'`, `3 'mg'`, as fhirpath.js answers | An exception, and two answers that cannot both hold |
| `is Integer` and `is Decimal` on a computed value: `(1 + 2) is Integer`, `2.5.power(2) is Decimal` | false | true, as fhirpath.js answers | Every computed number is of one of the two types; fhirpath.js reads which from the value itself, `(2 * 1.0) is Decimal` false there, where the result of a Decimal operand is a Decimal |
| A partial date shifted at full precision: `@2016 + 365 days`, `@2026-02 + 4 weeks`, `@2014 - 23 months`, `@2014-01-01 - 1 second` | `2016`, `2026-03`, `2012`, `2013-12-31` | `2017`, `2026-02`, `2013`, `2014-01-01`, as fhirpath.js answers | 3.0.0's own examples: the duration is converted to the value's precision first, `@2014 - 1 month // @2014` |
| Time arithmetic gaining a precision: `@T10:30 + 45 minutes` | `11:15:00` | `11:15`, as fhirpath.js answers | The result keeps the precision of the value |
| A negative quantity added as its magnitude: `@2014-03-31 + (-1 month)`, `@2014-06-15 - (-2 years)` | `2014-04-30`, `2012-06-15` | `2014-02-28`, `2016-06-15`, as fhirpath.js answers | Adding a negative amount moves back |
| `is Date`, `is DateTime` and `is Time` on a computed value: `(@2014-01-01 + 1 day) is Date`, `now() is DateTime` | false | true, as fhirpath.js answers | As for `is Integer` on a computed number, above |
| `toBoolean()` and `convertsToBoolean()` of `'yes'`, `'t'` and the other strings both versions list | empty, false: only `'true'` and `'false'` | the Boolean each names, `'yes'` true and `'no'` false, as fhirpath.js answers | Both versions list `'true'`, `'t'`, `'yes'`, `'y'`, `'1'`, `'1.0'` and their false counterparts |
| A signed integer string: `'+12'.toInteger()` | empty | `12`, as fhirpath.js answers | The form an integer string takes allows a sign |
| A DateTime to the hour written: `@2014-01-25T14.toString()` | `2014-01-25T14:00` | `2014-01-25T14`, as fhirpath.js answers | Written at its precision |
| A DateTime string to the minute: `'2012-01-01T10:00'.toDateTime()` | empty | `2012-01-01T10:00`, as fhirpath.js answers | Partial date/times convert |
| `convertsToX()` of a FHIR primitive: `Patient.active.convertsToBoolean()`, `Patient.birthDate.convertsToDateTime()` | false, while `toBoolean()` and `toDateTime()` convert them | true, as fhirpath.js answers | Two answers that cannot both hold |
| A conversion function given several items: `(1 \| 0).toBoolean()` | empty | an error, as fhirpath.js raises | "If the input collection contains multiple items, the evaluation of the expression will end and signal an error" |
| `toInteger()` of a decimal written without a fraction: `Observation.value.value.toInteger()` on `185`, `(10 / 2).toInteger()`, `1.5.round().toInteger()` | `185`, `5`, `2`, and empty for `1.0` and `5.0`; `(10 / 2).convertsToInteger()` false | empty, for any Decimal | Neither version lists a Decimal among what converts; fhirpath.js answers as the validator does, both typing a decimal by how it is written. An expression that divides or rounds and wants an Integer has one: `10 div 2` |
| A DateTime literal ending in T: `@2014-03-25T is System.DateTime` | false | true, as fhirpath.js answers | The T makes it a DateTime |
| `toQuantity()` of a Boolean: `true.toQuantity()` | empty | `1.0 '1'`, as fhirpath.js answers | "the item is a Boolean, where `true` results in the quantity `1.0 '1'`" (3.0.0) |
| `convertsToString()` of a partial DateTime: `@2014-01-25T10:00.convertsToString()` | false | true, as fhirpath.js answers | A DateTime converts to a String |
| A decimal string with an exponent: `'1e5'.toDecimal()` | `1e5` | empty, as fhirpath.js answers | The form a decimal string takes has no exponent |
| `convertsToString()` of a complex value: `Patient.name.first().convertsToString()` | true, writing `name=HumanName[4 children]` | false | 3.0.0's own example says false, and fhirpath.js's true writes `[object Object]`: no one string |
| `$this` in `aggregate()`'s init, within `select()`: `3.select((10 \| 20).aggregate($total + $this, $this))` | empty | 33, as fhirpath.js answers | 3.0.0: init is "evaluated on the outer context" |
| `$index` in `aggregate()`: `Patient.name.given.aggregate(iif($index = 2, $this, $total))`, `(1 \| 2 \| 3).aggregate(iif($index = 0, {}, ...), 5)` | empty, `5`: `$index = 2` and `$index = 0` never match, though `$total + $index` adds the indexes | `Jim`, `100`, as fhirpath.js answers | Answers that cannot all hold |
| `skip()` and `take()` given a String: `Patient.telecom.use.skip('1')` | converts it, all but the first item | an error, as fhirpath.js raises | `num` is an Integer, and a String converts to one only through `toInteger()` |
| `{}.single()`; `skip(-1)` | an exception | empty; the input, as "If `num` is less than or equal to zero, the input collection is simply returned" | An exception |
| `getValue()`: `Patient.active.getValue()`; `extension({})` | an exception | `true`, as fhirpath.js answers; empty | An exception |
| An undefined environment variable: `%fhirpathNoSuchVariable`, `` %`fhirpath-no-such-variable`.exists() `` | empty, false | an error, as fhirpath.js raises | 2.0.0: "Attempting to access an undefined environment variable will result in an error" |
| `` %`ucum` `` against `%ucum` | empty, where `%ucum` is the url | the url, as fhirpath.js answers | Two spellings of one name, answered apart |
| `conformsTo()` of more than one item: `Patient.name.conformsTo(...)` | false | an error in R4, empty in R5 | R4: "If the input contains more than one element, an error is thrown"; R5: "If the input is not a single item ... the result is empty" |
| `memberOf()` and `slice()` with a value set or structure that is no URI, under R4: `memberOf('not a uri')`, `slice('not a uri', 'x')` | empty | an error | R4: "If the valueset cannot be resolved as a uri to a value set, an error is thrown", and of `slice()` "an error is thrown"; R5 makes both empty, as here |
| `comparable()` with an empty operand or one of several items: `{}.comparable(1 'mg')`, `(1 'mg' \| 2 'mg').comparable(1 'mg')` | false | empty | 3.0.0: "If either or both inputs are empty, or either input is not a single Quantity value, the result is empty"; fhirpath.js raises an error |
| `1 'Cel'.comparable(1 '[degF]')` | false | true, as fhirpath.js answers | 3.0.0's own example is true: both are temperatures |
| `hasValue()` of a quantity: `1 'mg'.hasValue()` | true | false, as fhirpath.js answers | A quantity is no primitive, FHIR or System |
| `encode('hex')` | upper case, `7A3F` | lower case, `7a3f` | 3.0.0: "hexadecimal characters (base 16) in lowercase"; fhirpath.js mixes the cases |
| `timeOfDay()`, `yearOf()` and the other component functions, `duration()`, `difference()`, `sum()`, `avg()`, `min()`, `max()`, `toDate()`, `toTime()`, `toQuantity()`'s unit, the format of `toDate()` and `toDateTime()`, `repeatAll()`, `lastIndexOf()`, regular expression flags, `encode('ascii')`, `combine()`'s `preserveOrder`; `ofType(System.Any)` | not known functions or formats; System.Any no type; `combine()` of two arguments refused | | Not implemented there; fhirpath.js has neither `repeatAll()`, the `ascii` format nor `preserveOrder` |

## Decisions taken where the specification is silent

Recorded here and at the point of code, so nobody has to guess whether a choice
was reasoned or accidental.

| Question | Decision | Why |
|---|---|---|
| `Integer` range, and `Long` | 64 bits, with no separate `Long` type | The specification's `Integer` is 32 bits and its `Long` — STU content — is 64. An `int64` Integer already spans that range, and FHIR's own `integer64` says the values occur. See above |
| The range of a Decimal | Up to 1000 digits before the point, and none further than 1000 places after it. A value beyond, read from the input, written as a literal, from a string by `toDecimal()`, as a FHIR Quantity's value, or reached by arithmetic, `round()` or a unit conversion, is an overflow and reads as empty; a FHIR Quantity whose value overflows is still there, an object, but compares as nothing: `=` and `<` against a quantity empty, `~` false unless the two are written alike, and it sorts as an empty key. A function computed in float64 whose result is infinite, `1000.exp()`, overflows the same way, as does a product or quotient of quantities, which the UCUM library computes in float64 with each unit's scale, where an operand or the result is past float64's range. `round()` with a negative precision past the bound rounds to zero, found without rounding there; see [Math](#math) | "Implementations can provide support for larger decimals and higher precision, but must provide at least the range and precision defined here", (-10^28+1)/10^8 to (10^28-1)/10^8, and "Operations that cause arithmetic overflow or underflow will result in empty". FHIR's decimal allows an exponent of nine digits, so the bound is the engine's. A decimal is held exactly, so one written `1E8227822` was a number of eight million digits: every comparison wrote them out, half a second for each `=`, more than five for `round()`, and more than eight for `~`; a zero written `0E2000000000` was two billion zeroes. What it costs: a range check on each decimal read or computed, which counts digits only near the bound; arithmetic over 500 values measures no different from before |
| Max precision for `lowBoundary`/`highBoundary` | An implementation limit, documented in the function | 3.0.0 says explicitly: above "the maximum possible precision of the implementation", return empty. FHIR caps decimal at 18 digits |
| `sort` direction syntax | Accept both `desc`/`asc` and the leading `-` | 3.0.0 defines `desc`; the suite tests `-` and marks those cases as prototype |
| `type()` completeness | Emit `SimpleTypeInfo` and `ClassInfo`; omit `ClassInfo.element`, `ListTypeInfo`, `TupleTypeInfo` | They need element enumeration and declared cardinality, which the `Model` interface does not expose. Reading them off the instance would describe the value, not the type |
| `type()` results per element | One per input element | 3.0.0 states this, then contradicts it in its own `ListTypeInfo` example |
| `min()` and `max()` over items some of which have no order | The item that orders against every other and stands below, or above, them all; empty where no item does. `(@2019 | @2020-01-01T10Z | @2020-01-01T10:30Z).min()` is `@2019`, and `.max()` empty. Items `<` refuses, `1` and `'a'`, end the evaluation | The specification defines the aggregates by "the Comparison Operators for the type of value being aggregated", which answer empty for such a pair, and says "All items in the input collection SHALL be the same type, otherwise an exception is thrown". Skipping such pairs answered by input order, the same value as both minimum and maximum. fhirpath.js skips them too; it gives `.max()` above as `@2020-01-01T10Z` |
| A date off the calendar or a time off the clock | Not a date or a time: `@2021-02-30`, `@2021-13-01`, `@T24:00` and an offset beyond ±14:00 are parse errors; `'2021-02-30'.toDate()` is empty and `convertsToDate()` false; in the input, such a string is read as the string it is, of the type its element declares, so comparing it with a date, as `per-1` does, is the type error comparing any string with a date is, as in fhirpath.js. A leap second, `23:59:60`, which FHIR's dateTime, instant and time allow, is read, as fhirpath.js and the HL7 validator read it, and held as the validator holds it: a DateTime as the second after it, so `@2016-12-31T23:59:60Z = @2017-01-01T00:00:00Z` and `> @2016-12-31T23:59:59Z` are true and a second later is `00:00:01`, written back as it was, its components too: `toDate()` is `2016-12-31`, the day it falls on, and `secondOf()` 60, as fhirpath.js answers, where the validator raises an exception, and compared at a coarser precision it is within its day and its minute, `> @2016-12-31` empty, as fhirpath.js answers, where the validator answers true; a Time as written, a shift of nothing leaving it so, after `@T23:59:59` and equal to no other, a second later `00:00:01`. fhirpath.js reads it as neither, `> 23:59:59Z` false and a second later `00:00:00`. A leap second needs a day to fall on, and one within FHIRPath's range: `@2012T12:30:60` and `@9999-12-31T23:59:60Z`, whose next second is in year 10000, are no DateTime. Up to 1.14.1 it was refused, and a FHIR instant written with one compared as a string. Arithmetic past year 9999 or before year 1 ends the evaluation with an error, as 3.0.0 asks | FHIRPath's Date runs "@0001-01-01 to @9999-12-31" and its Time "@T00:00:00.000 to @T23:59:59.999"; fhirpath.js rejects such literals too. A leap second is past that end, and both engines read it all the same, as FHIR, whose data it is, allows it. Accepted as written, February 30 was compared as itself component by component and as March 2 at an offset, where time.Date normalizes it, which made equality intransitive |
| A quantity finer than the date, time or date/time it shifts | Converted to the value's precision first, truncating toward zero, at 12 months or 365 days to a year and 30 days to a month: `@2016 + 365 days` is `@2017`, `@2026-02 + 4 weeks` is `@2026-02`, `@2000 - 1 day` is `@2000`, `@T10 - 30 minutes` is `@T10`. Seconds and milliseconds are one precision: a value written to the second is shifted to the millisecond and the millisecond then dropped, so `@2000-01-01T00:00:00 - 1 'ms'` is `@1999-12-31T23:59:59`, and there a remainder of less than a second rounds down rather than toward zero. A Date takes a time unit the same way, `@2014-01-01 + 1 hour` being `@2014-01-01`, as fhirpath.js and the HL7 validator both answer, by rule 1, though 3.0.0's table gives hours and finer to DateTime and Time alone and makes "an unsupported unit for the type" an error | 3.0.0: "converting the time-valued quantity to the highest precision in the partial (truncating any decimal fraction) and then adding", with these examples. Shifting at full precision and dropping the finer components afterwards moved a value a whole unit back for any subtraction, `@2000 - 1 day` to `@1999`. Seconds and milliseconds are one precision in comparison too, and fhirpath.js also moves into the previous second, though it keeps the millisecond, `09:59:59.999` |
| Comparing temporals written at two different offsets | As the instants they start at, "converted to a common timezone offset", as fhirpath.js and the HL7 validator compare them: `@2020-01-01T10+05:30` is 04:30 at UTC, so it is after `@2020-01-01T04Z` and `04:29Z`, before `05Z`, unequal to `04Z`, and equal to `@2020-01-01T11+06:30`. The same instant at different precisions has no answer, as at one offset: `@2020-01-01T10+05:30 = @2020-01-01T04:30Z` is empty. An offset follows a time and nothing else, as in the grammar and in FHIR's dateTime, so `'2020-01-01+05:00'` is no DateTime and `toDateTime()` of it is empty | Up to 1.14.1 they compared as the spans of time they name, the hour from 04:30 to 05:30, and two that overlapped had no order: `> 04Z` was empty where both engines answer true. Measured with validator 6.10.2 and fhirpath.js 5.2.0. Where the two part ways, a different precision on each side, `10+05:30 < 04:31Z` (fhirpath.js false, the validator true) and `10:15+05:30 > 04Z` (fhirpath.js empty, the validator true), this engine orders the instants, as the validator does. `=` between two at different offsets, where the validator raises an exception, answers as fhirpath.js: false for two instants, empty for one instant at two precisions |
| Comparing a temporal that carries a timezone offset with one that does not | No answer — empty, reported as `ErrOffsetMismatch`. Under `~`, which always answers, false, as for values at different precisions, and `union()`, `distinct()` and `in` hold the two as different values. A Date compares with a DateTime at the same precision, `@2020-01-01 ~ @2020-01-01T` true, as it converts to one | The default offset is a policy the specification leaves to the caller, and FHIR requires the offset where it matters, so a bare value is a literal or invalid data. See below |
| Temporal comparison across precisions | Component by component, stopping at the first difference; empty only when everything shared matches | Not a judgement call — the spec states it, and it is why `now() > today()` is empty while `now() > @1974-12-25` is true |
| `ofType()` on profiled subtypes without a Model | Keep structural inference | Nothing in the JSON distinguishes an `Age` from a `Quantity`; the information is in the model, not the document |
| `~` between two quantities | In the less granular unit, rounded to the precision of the less precise value, trailing zeroes carrying none: `4 'g' ~ 4040 'mg'` and `4040 'mg' ~ 4 'g'` are both true, `1 year ~ 11 months` true. A converted value keeps the precision it was written with in its own unit: `200 'cm'` is given to the centimeter, two places in meters, so `2.4 'm' ~ 200 'cm'` is false. Where the conversion factor is 1, the left operand's unit, so `1.5 'Cel' ~ 274.6 'K'` is true and `274.6 'K' ~ 1.5 'Cel'` false. A calendar year or month against UCUM takes `'a'` or `'mo'`, so `1 year ~ 1 'a'` is true where `=` is empty | 3.0.0: "the quantity values must be converted to the least granular unit", and "If the conversion factors are 1 (the units are equal), then choose the unit of the operator's left argument"; its worked examples all hold either way round. 2.0.0 said the most granular unit, which fhirpath.js follows, so it answers `1 year ~ 6 'mo'` differently, and three of 3.0.0's examples, `4 'g' ~ 4040 'mg'`, `21 'mm' ~ 2 'cm'` and `1 year ~ 11 months`, false. The engine converted into the left operand's unit and counted trailing zeroes, which made `4040 'mg' ~ 4 'g'` false while `4 'g' ~ 4040 'mg'` was true. A step, not a point, measures a unit on a scale with an offset: a degree Fahrenheit is 5/9 of a kelvin, though 1 '[degF]' is 255.9 'K'. What it costs: a factor, or a temperature's step, is found once per pair of units and reused. `4 'g' ~ 4040 'mg'` runs about 15% faster than before, `4040 'mg' ~ 4 'g'` about 1.45 times slower, where the old code needed no rounding there; a temperature runs about as fast as before, and two calendar units about 1.7 times slower. A scale on which a step has no constant size, a logarithm or an angle, has no unit more granular than another, and keeps the left unit, as on a tie |
| `=` and `~` between two quantity objects | Both compare them as quantities, when their type is Quantity or one of its profiles | Where the HL7 validator answers over the specification text: see [Two FHIR Quantities are equivalent as quantities](#two-fhir-quantities-are-equivalent-as-quantities) |
| `=` and `~` between two complex values | Under `=`, their children, each found by its key wherever each object writes it and compared as `=` compares its kind: a string as it reads, so an escaped character equals the character itself; a number as a decimal, so `1` equals `1.0`; a repeating child in order; a `null` or an empty array no child, as navigation reads it. `~` compares each child with the equivalence of its type, recursively: keys and repeats in any order, a primitive's value and the element beside it as one item, a string ignoring case, any whitespace character for another, no run collapsed (see [When 2.0.0 and 3.0.0 disagree](#when-200-and-300-disagree)), a decimal as decimals are equivalent, a date as dates are equivalent, so one at another precision or with an offset against none is not; a child on one side only is not equivalent. Two FHIR Quantities compare as quantities, as the row below says | "For complex types, equality requires all child properties to be equal, recursively": a property is found by its name, not by where a text writes it. fhirpath.js and HL7's Java engine both answer keys in another order equal, and fhirpath.js `1` and `1.0`, as "trailing zeroes after the decimal are ignored"; the Java engine answers a `null` member no child. Up to 1.13.6 this engine compared content as written, apart from layout, for speed: one serializer writes a value's keys in FHIR's order throughout a document, so only values from two serializers told the readings apart (#167). It now reads by key, through two fast paths that read no further than the first difference and build nothing: texts alike apart from layout are equal, and texts whose keys agree up to their first difference, found a word at a time, and differ there in a value are unequal. Only the rest, keys in another order among them, are read child by child. What it costs: evaluating every constraint over the R4, R4B and R5 examples, about 2% more time, 0.1% more memory. Over objects alone, `=`, unions and `distinct()` run 2 to 3 times slower than comparing bytes did, large objects that share a long prefix up to 8 times, 10 KB objects that differ only at their end up to 40 times (a narrative dense with escaped quotes, 7 µs against 0.18 µs), and `intersect()` and `repeat()` over objects with keys in another order about 20 times, which is the reading that tells them equal. A test holds the fast paths to reading by child over 20,000 pairs made to differ every way they take apart, and `FuzzSameJSON` holds the comparison of layout to a reference. For `~`, "equivalence requires all child properties to be equivalent, recursively": fhirpath.js and the HL7 validator apply string equivalence to each child too, so two Codings that differ in the case of a code are equivalent. fhirpath.js compares repeats in order and trims no whitespace; here each child takes this engine's own equivalence of its type, under which collections compare in any order, as "Comparison is not order dependent". The Java engine compares a repeating child in order too, while it compares the same values as collections in any order: two names whose givens are `["a", "b"]` and `["b", "a"]` are not equivalent to it, while their givens are, and so for two CodeableConcepts with their codings reversed. That answer contradicts its own, since equivalence "requires all child properties to be equivalent, recursively", and fhirpath.js's rests on pairing in order, which it does over collections too, so rule 1 does not apply where the two agree (#207). Two limits. A child is typed as navigation without a model types it, so with a model `a ~ b` can differ from `a.x ~ b.x` where the model calls a date-like value a string. And repeats pair greedily, those written alike first, so with decimals, whose equivalence rounds and is not transitive, the answer can depend on order, and can differ from `~` between the same values as collections: `{"v":[1, 1.4]}` is equivalent to `{"v":[1.4, 0.6]}`, while `(1 | 1.4) ~ (1.4 | 0.6)` is false, as in fhirpath.js. JSON no valid resource holds is compared as written rather than as navigation reads it: a `null` inside an array is a place of its own, a nested array is not flattened, and of a key written twice, the first that is not `null` counts. Comparing Quantities as quantities was first tried on any object with a numeric `value`, an adjudication among them, and dropped; it is done now for objects whose type is Quantity or one of its profiles, as the HL7 validator does. What it costs: a child written alike on both sides pairs unread, and a string that is no date compares as written, but each pair reads the keys of both objects. Each object in a repeating child is read once however many it is compared with, and found by key through an index once it has more than 16. Against comparing as `=` did, `~` between 40 names runs about 1.5 times slower, two objects of 200 keys that differ in the last about 3 times (19 µs against 53 µs), 100 Codings in reversed order about 5 times (15 µs against 75 µs), and 40 objects of 40 strings that differ in case, both levels reversed, about 5 times (128 µs against 640 µs); in the last two `=`'s reading answered false. Pairing a value with the element beside it (#175) costs 40 names with an element beside a given, in another case and reversed, about 5% more time and 8% more allocations (946 µs against 992 µs); objects without one compare as before |
| A FHIR Quantity with a `comparator` (`<`, `<=`, `>=`, `>`, `ad`) | Read as its value | Where the HL7 validator answers over the specification text: see [A quantity's comparator is not read](#a-quantitys-comparator-is-not-read) |
| `as()`/`ofType()` on a primitive | Requires the declared type; `is()` keeps hierarchy matching | FHIR: "all primitives are considered to be independent types (so markdown is not a subclass of string)" |
| Telling `FHIR.boolean` from `System.Boolean` | By case: FHIR names primitives in lower camel case, FHIRPath capitalizes its own | The suite requires `active.is(boolean)` true and `active.is(Boolean)` false. Complex types such as `Quantity` are spelled alike in both namespaces, so the rule applies to primitives only |
| Type of a polymorphic element without a model | Taken from the field name, corrected to FHIR's casing | `valueOid` states the element is an `oid`; that is information in the document, not a guess. The value itself says whether the type is primitive or complex. fhirpath.js run without a model resolves no suffix, `Observation.value` over `valueString` is empty, and reads a key written bare (measured in #219's adversarial review, not in a battery, since the batteries give every engine its model); not followed, since a name the document states is information to use |
| An element written as an empty object, `"name": {}` or an item `{}` of an array | An item of its collection that `exists()` and `empty()` answer as though it were not there | Where the HL7 validator answers over the specification text: see [An element that holds no value](#an-element-that-holds-no-value) |
| A FHIR primitive's `value`, and `hasValue()`/`getValue()` | The System value the primitive holds, without its FHIR type or element. `hasValue()` is true of a single FHIR primitive with a value, and of a single System value of a primitive type: a complex value, or a primitive with only an id or extensions, has none, however the object was made. An object typed as a FHIR primitive, as a validator builds `"_family"` to evaluate it as the root, written `string`, `FHIR.string` or R4's `http://hl7.org/fhirpath/System.String`, is a primitive with no value; the element beside a primitive that has one is not | FHIR: "FHIR.string … has the properties id, extension, and also the implicit value property that is actually of type of System.String", so `birthDate.value is System.Date`. The HL7 validator answers `value` with a FHIR `string` of the text whatever the primitive's type; that is not what the text says. `hasValue()` on a complex value: see [Measured, and not followed](#measured-and-not-followed) |
| What is a FHIR primitive, as opposed to a System value | By where the value came from: one read from the resource is a FHIR primitive, with or without a model, and even where the model types it a System one; a literal or a function's result is a System value | `'abc'.value` and `'abc'.getValue()` are empty with or without a model, as the text says, and so for `active.toBoolean()`. `'abc'.hasValue()` is true, as fhirpath.js and the HL7 validator both answer, though the text speaks of FHIR primitives alone: the two agree on every System value of a primitive type, `1`, `true`, `@2020`, `@T10:00`, and `''`, which is a value, where only the validator answers false; a quantity is none. Up to 1.14.7 it was false. A primitive at a path the model does not know is still a FHIR primitive, so ele-1 holds on it. R4 gives `Resource.id`, `Element.id` and `Extension.url` the type code `System.String` and names their FHIR type, `id`, `string` and `uri`, in `structuredefinition-fhir-type`; the two parts of the definition disagree, and we follow the one that names the FHIR type, as the HL7 validator does, so `Patient.id.hasValue()` is true. Their type is the one the definitions name, where the model hands it: see [Where this engine follows the HL7 validator](#where-this-engine-follows-the-hl7-validator-over-the-specification-text) |
| A primitive's value in `children()` and `descendants()` | Not a child | Where the HL7 validator answers over the specification text: see [A primitive's value is not a child](#a-primitives-value-is-not-a-child) |
| `is()`, `as()` and `ofType()` on a System value | Only System types match, with a model or without one | A literal, a function's result or a primitive's value is a System value: `'abc'.is(String)` and `'abc'.is(System.string)` are true, `'abc'.is(string)` and `'abc'.is(FHIR.string)` false. Without a model only a value read from the resource is taken for what its type name might be in FHIR, so a literal or a function's result is never a FHIR string. `ofType(System.string)` still finds FHIR strings too: "ofType() does not have such restrictions" |
| Order of `children()` and `descendants()` | With a model that implements `ElementOrder`, the order the element's definition lists its children in; otherwise as the JSON writes them | The text leaves it undefined — "using functions like first() on the result may return different results on different platforms" — and allows "the logical order implied by the object model", which is the order the HL7 validator's parser builds. JSON keys are unordered, so without it one instance written two ways answers `children().first()` two ways. A field the definition does not list comes last, as written, where HL7 rejects it; an array keeps its order; an object the model does not know, or whose type is only guessed from its fields, stays as written. A primitive's element, `_birthDate`, is ordered and its fields typed by its primitive's type, `date`: id, then extension; and every element's `extension` is an `Extension`, as `Element` defines it, where a model does not list a primitive type's elements. An instance already in the definition's order, as HL7 and HAPI write, costs no allocation |

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

A case that leaves a model baseline takes its line in `known-failure-reasons.txt`
with it; one that enters needs a line, and a section here if none argues it yet.
