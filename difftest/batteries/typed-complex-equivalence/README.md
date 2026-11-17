# Typed complex equivalence (plan 025, #263)

`~` between two complex values with a model: whether each child is read as the
model types it, as navigation reads it. There are 71 cases over R4 (4.0.1) and
R5 (5.0.0). `docref-r5.json` and `timing-r5.json` are R5 only, and
`timing.json` is R4 only.

Run with `make three-engine BATTERY=batteries/typed-complex-equivalence/battery.tsv DIFFTEST_ARGS="-java <validator jar> -jobs 2"`. `difftest` has no R4B; the three versions are run in `conformance/typed_complex_equivalence_test.go`.

## Classification (on main, 579a581, before 025)

- **#263, this engine alone differs (26).** A `string` child with a date-like
  value was read as a date: `Identifier.value`, `ContactPoint.value`,
  `Annotation.text`, `HumanName.given`, `valueString`, a contained resource's
  identifier, `Questionnaire.item.item.text`. Also `"…Z"` against `"…z"`,
  which is true as text, case aside. fhirpath.js and Java agree, and 025
  takes their answer.
- **A `dateTime` child (`Identifier.period`, `valueDateTime`); fhirpath.js
  alone differs.** Compared as instants, true, as Java answers. fhirpath.js
  answers false (plan 021). This does not change.
- **A `time` child: Java errors.** On `timeOfDay ~ timeOfDay`, this engine
  and fhirpath.js say true. On the Dosages that hold them, the two say false
  today. After 025 this engine says true, a divergence taken on purpose: N1
  §6.1.3 requires "all child properties to be equivalent, recursively", and
  fhirpath.js answers those children equivalent.
- **R5 `integer64`, `"1"` against `"+1"`.** On `size ~ size`, this engine and
  fhirpath.js say true, and Java says false, since it reads `integer64` as
  text; plan 016 (D2) reads it as an integer. On the Attachments, all three
  say false today. After 025 this engine says true, on purpose, for the same
  reason.
- **Two backbones whose `value` is `dateTime` in one and `string` in the
  other, both written `"2020"`** (`backbones.json`, plan 025 §7).
  fhirpath.js and Java answer true. This engine answers false, typing each
  side under its own path, as it answers the two values themselves, and as
  fhirpath.js answers those values. Java answers `'2020' ~ @2020` true.
  This divergence is on purpose and of the same class as the two above.

## Cases added in code review round 2 (on the 025 branch)

- **An `extension` inside an element the model does not know**
  (`extensions.json`, `HumanName.assigner`). This engine and fhirpath.js
  answer true: navigation types the extension and its `valueString` on both
  sides, as it answers the two values themselves. Java answers false because
  it drops the unknown element when it parses (`Patient.name[0].assigner.exists()`
  is false there), not because of typing.
- **A decimal list under a known path against one under an unknown path,
  where only a pairing that reconsiders candidates finds a match**
  (`decimals.json`, `[1.0, 1.1]` against `[1.1, 1.2]`). All three engines
  answer false, both ways. main answered true, untyped.
