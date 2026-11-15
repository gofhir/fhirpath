# Object equality is transitive (plan 024)

`battery.tsv` compares `=` and `~` between two extensions of a Basic, in both
directions, in R4 and R5 (`difftest` has no R4B; the three versions are run in
`conformance/object_equality_transitivity_test.go`). One shape a file, because
the Java engine fails on a whole resource.

| File | Extension[0] against extension[1] |
|---|---|
| `ot-00.json` | `{"a":[[1]],"b":0}` against `{"b":0,"a":[[1.0]]}` |
| `ot-01.json` | `{"a":[[1]],"b":0}` against `{"a":[[1.0]],"b":0}` |
| `ot-02.json` | `{"a":[[1]]}` against `{"a":[1]}` |
| `ot-03.json` | `{"a":"x","a":"y"}` against `{"a":"x","a":"z"}` |
| `ot-04.json` | `{"a":null,"a":"x"}` against `{"a":null,"a":"y"}` |
| `ot-05.json` | `{"a":"x","_a":{"resourceType":"X"}}` against the same with `"Y"` |
| `ot-06.json` | `{"a":"x"}` against `{"a":"x"}` |
| `ot-07.json` | `{"n":1,"m":"x"}` against `{"n":1.0,"m":"x"}` |
| `ot-08.json` | a Quantity in a nested array, `1` against `1.0` |
| `ot-09.json` | `{"a":"x","a":"y"}` against `{"a":"x"}` |

Run:

    cd difftest && go run . -battery batteries/object-equality-transitivity/battery.tsv \
      -java ../build/validator/validator_cli-6.10.2.jar -jobs 2 -v

Measured 2026-11-09 against fhirpath.js 5.2.0 and validator 6.10.2: 80 cases,
0 agree, 56 Java alone differs (this engine and fhirpath.js alike), 24 the
three differ. This engine's answers are the same in R4 and R5, and each `=`
and `~` is the same in both directions.

## Differences

1. **Java answers no boolean (every row).** Between two extensions it answers
   `extension=Extension[1 children]`, for `=` and for `~`, so there is nothing
   to compare. The rows still hold this engine against fhirpath.js. With Java
   giving no answer, rule 1 does not apply, as plan 024 §1 says. (56 rows are
   Java alone: `ot-00`, `ot-01`, `ot-02`, `ot-04`, `ot-06`, `ot-07`, `ot-08`.)
2. **A key written twice, this engine true, fhirpath.js false (`ot-03`,
   `ot-09`; the three differ).** CONFORMANCE.md: "of a key written twice, the
   first that is not `null` counts", so `{"a":"x","a":"y"}` is `{"a":"x"}`.
   fhirpath.js reads the resource through `JSON.parse`, which keeps the last,
   so it compares `y` with `z`, and `y` with `x`. Plan 024 §2, first row of the
   table of answers that change. The JSON is not valid for a resource.
3. **An element with content beside a value, this engine true, fhirpath.js
   false (`ot-05`; the three differ).** An element that holds no value adds
   nothing to the value beside it (CONFORMANCE.md, "An element that holds no
   value"), and `{"resourceType":"X"}` holds none. fhirpath.js compares the
   two elements as objects. Plan 024 §2, second row. The JSON is not valid
   FHIR: ele-1 forbids an element with neither a value nor children.
4. **A key written twice after a `null` (`ot-04`).** False here and in
   fhirpath.js, for two reasons: here the first that is not `null` is `x` and
   `y`, there the last is.

No difference comes from the direction of the comparison, from the order of the
members, or from `~` against `=`. A nested array's numbers compare as decimals
in both engines (`ot-00`, `ot-01`, `ot-08`), and a nested array is not
flattened (`ot-02`). Before plan 024 `ot-00` answered false under `=` here
(and `[[1]] ~ [[1.0]]` false under `~`), which fhirpath.js never did.
