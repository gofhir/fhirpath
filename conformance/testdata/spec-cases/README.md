# Spec cases

This project's own conformance cases, written from the specification's text
rather than from what any engine answers. The official suite covers what HL7
chose to test; these cover what it leaves out, one area of the language at a
time. Each case says which sentence of the specification it comes from, and the
build checks that the sentence is really there.

`TestSpecCases` runs them like the official suite, with the R4 and the R5
model. `TestSpecCaseCatalog` checks the format and the quotes. `make difftest
DIFFTEST_ARGS="-corpus spec -v"` puts them to fhirpath.js.

## Files

| File | What it is | Written by |
|---|---|---|
| `<area>.xml` | The cases for one area | the case author |
| `<area>.known-failures-r4.txt`, `-r5.txt` | Generated baselines, as for the suite | `make conformance-update` |
| `<area>.known-failure-reasons.txt` | Why each failing case fails | the adjudicator |
| `SPEC_SOURCES` | The texts a case may cite, pinned by sha256 | by hand |
| `.spec/` | Those texts as plain text; not committed | `make spec-sources` |

## Format

The suite's own XML, so the same runner and difftest read it, with a group per
requirement and the citation inside it:

```xml
<tests name="strings" description="What the area covers, and what it leaves out.">
  <group name="REQ-STRINGS-001" description="indexOf of an empty substring is 0">
    <source spec="fhirpath-3.0.0" section="5.6.1 indexOf" strength="DEFINITION">If `substring` is an empty string (`''`), the function returns 0.</source>
    <test name="REQ-STRINGS-001-a"><expression>'abc'.indexOf('')</expression><output type="integer">0</output></test>
  </group>
</tests>
```

- `<tests name>` is the file's name: lower case and hyphens.
- A **group** is one requirement, named `REQ-<AREA>-NNN` with the area in upper
  case and its hyphens dropped. Numbers are stable: a removed requirement leaves
  its number unused.
- A group with `fhir="r4"` or `fhir="r5"` runs only with that version. Use it
  when the cited text is one version's FHIR page.
- A **source** cites one sentence, or a few consecutive ones:
  - `spec` is a name from `SPEC_SOURCES`: `fhirpath-2.0.0` (normative),
    `fhirpath-3.0.0` (in development, STU), `fhir-r4` or `fhir-r5` (FHIR's own
    FHIRPath page: `resolve`, `memberOf`, `%resource`, …).
  - `section` names the heading the sentence sits under.
  - `strength` is how the text states it: `SHALL`, `SHALL NOT`, `SHOULD`,
    `SHOULD NOT`, `MAY`, `DEFINITION` for what the text describes in the
    indicative, or `EXAMPLE` for one of the specification's worked examples.
  - The text is copied **verbatim** from `.spec/<spec>.txt`. Only whitespace,
    typographic quotes and dashes, and no-break spaces are forgiven.
- A **test** is named after its group, `REQ-STRINGS-001-a`, and is a suite case:
  `<expression>`, the `<output>` values in order, `invalid="execution"` when the
  expression must fail, `predicate="true"` to compare a truth value. No
  `<output>` means the result is empty.
- `inputfile` is empty (the input is `{}`) or names one of the official suite's
  inputs, which must exist for every version the group runs with.
- `<`, `>` and `&` in an expression are written `&lt;`, `&gt;` and `&amp;`.

## Where the specification is silent

A case states what the specification says, never what it leaves open. When the
text does not settle a question — an empty pattern, an offset that is not
written, a precision beyond the value's — there is no case for it. The author
lists the question instead, with the nearest sentence, so that it is decided
deliberately and recorded in `CONFORMANCE.md`.

## When a case fails

A failing case is adjudicated before it is listed. It is an engine gap, a
deliberate divergence already argued in `CONFORMANCE.md`, a need for something
outside the engine, or a defect in the case. A defective case is corrected, never
listed: `suite-defect` is not a kind a spec case can have.
