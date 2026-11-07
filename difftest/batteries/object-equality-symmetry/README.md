# Object equality symmetry (#265, plan 023)

`battery.tsv` compares `=` and `~` between two objects, in both directions and
with the members of each in either order, in R4 and R5 (`difftest` has no R4B;
the three versions are run in `conformance/object_equality_symmetry_test.go`).
One shape a file, because the Java engine fails on a whole resource.

| Files | Shape |
|---|---|
| `ex-NN.json` | a Basic with two extensions to compare, `extension[0]` and `extension[1]`, the members as written (00 to 17) |
| `exr-NN.json` | the same for 00 to 07, with the members of each extension reversed |
| `c.json` | the six codings of plan 023 §1: `[0]` and `[1]` one class, `[2]`, `[3]`, `[4]`, `[5]` a class each |
| `nm-NN.json` | a Patient with one name whose `given.count()` navigation sets (the plan's table) |

Run:

    cd difftest && go run . -battery batteries/object-equality-symmetry/battery.tsv \
      -java ../build/validator/validator_cli-6.10.2.jar -jobs 2 -v

Measured 2026-11-06 against fhirpath.js 5.2.0 and validator 6.10.2: 264 cases,
30 agree, 10 fhirpath.js alone differs, 178 Java alone differs, 46 the three
differ. This engine's answers are the same in R4 and R5, and the same with the
members reversed (`exr` against `ex`), and each `=` and `~` is the same in both
directions.

## Expected equivalence classes of `c.json`

`code.coding.distinct().count()` is 5 here: `{[0], [1]}`, `{[2]}`, `{[3]}`,
`{[4]}`, `{[5]}`. fhirpath.js answers 6 and Java 3.

## Differences

1. **Java answers no boolean (208 cases, every `ex` and `exr` row).** Between
   two extensions it answers `extension=Extension[1 children]`, for `=` and for
   `~`, so there is nothing to compare. The rows still hold this engine
   against fhirpath.js.
2. **Element beside a value, this engine and Java true, fhirpath.js false
   (`c[0]`, `c[1]`; `ex-01`, `ex-17`).** An element that holds no value adds
   nothing to the value beside it: the documented reading in CONFORMANCE.md,
   "An element that holds no value". The references disagree, so rule 1 does
   not apply.
3. **An array of one empty element against the element (`ex-02`).** True here,
   false in fhirpath.js, which compares the array against the object. An
   element written `[{}]` and one written `{}` are the one item `given.count()`
   counts, and this engine and fhirpath.js count it alike (`nm-00`, `nm-06`).
4. **`~` between values in another order (`ex-15`).** True here, false in
   fhirpath.js: "Comparison is not order dependent" for `~`, documented in
   CONFORMANCE.md. `=` is false in both.
5. **An element that holds no value against its absence (`c[3] = c[2]`,
   `c[3] = c[5]`).** False in both directions here and in fhirpath.js. Java
   answers true one way and false the other (`c[2] = c[3]` is false), so it is
   not symmetric itself. One item against none is false by the specification
   text, and what navigation counts.
6. **An element with content against none (`c[0]` and `c[1]` against `c[4]`).**
   False here and in fhirpath.js. Java answers true, as CONFORMANCE.md's `~`
   row records for an element against none, which this engine does not
   follow.
7. **`distinct().count()` of the six codings.** 5 here, 6 in fhirpath.js,
   which separates `[0]` from `[1]` (2), and 3 in Java, which answers true for
   pairs the others keep apart (5) and (6). Compared by the classes above, not
   by the count.
8. **Item counts.** `nm-06` (`"_given": {}`): Java fails with an evaluation
   error, this engine and fhirpath.js count 1. `nm-08` (`"given": [null, null],
   "_given": [{}, null]`): 1 here and in Java, 2 in fhirpath.js, which counts
   the second null position as an item (plan 023 §1, the table).

No case differs because of the direction of the comparison or the order of the
members: `=` and `~` answer the same either way round, here and in the other
two engines where they answer a boolean, apart from Java in (5).
