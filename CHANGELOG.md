# Changelog

## [1.8.1](https://github.com/gofhir/fhirpath/compare/v1.8.0...v1.8.1) (2026-08-23)


### Bug Fixes

* an offset that is missing is not a precision that is missing ([#41](https://github.com/gofhir/fhirpath/issues/41)) ([09634a0](https://github.com/gofhir/fhirpath/commit/09634a07f9274852c78332668b6886b8fc1220e0))

## [1.8.0](https://github.com/gofhir/fhirpath/compare/v1.7.0...v1.8.0) (2026-08-17)


### Features

* the component extractors, under names that can be called ([#37](https://github.com/gofhir/fhirpath/issues/37)) ([9e68aac](https://github.com/gofhir/fhirpath/commit/9e68aac9390394020d6b2f49027299b72c298fb1))

  The ten functions FHIRPath 3.0.0 defines for reading a component out of a
  temporal value: `yearOf`, `monthOf`, `dayOf`, `hourOf`, `minuteOf`,
  `secondOf`, `millisecondOf`, `timezoneOffsetOf`, `dateOf` and `timeOf`.

  ```
  @2024-06-15.monthOf()                              // 6
  @2024.monthOf()                                    // { } -- no month was written
  @2012-01-01T12:30:00.000+08:45.timezoneOffsetOf()  // 8.75
  @2012.dateOf()                                     // @2012 -- the input precision is kept
  ```

  Seven of them were already implemented under the names the specification used
  before — `year`, `month`, `day`, `hour`, `minute`, `second`, `millisecond` —
  and could not be called: those words are calendar units in the grammar, so
  `Patient.birthDate.month()` does not parse and only ``birthDate.`month`()``
  reaches the function. The `Of` names are what 3.0.0 renamed them to, and are
  the ones to use. The older spellings are kept and share the same
  implementations.

  Two answers changed in the process. A component the value does not carry is
  now empty rather than zero — `@2012-01-01.hourOf()` is `{ }`, where `hour()`
  used to answer `0`, which says midnight — and a collection of more than one
  item is an error, as the specification states for all ten.

## [1.7.0](https://github.com/gofhir/fhirpath/compare/v1.6.0...v1.7.0) (2026-08-16)


### Features

* read a resource once for many expressions, with `Document` ([#32](https://github.com/gofhir/fhirpath/issues/32)) ([83244fd](https://github.com/gofhir/fhirpath/commit/83244fd6498b8c13481c78af5bf14a6bb6a63a58))

  `Evaluate` reads the resource it is given, so the invariants of one resource
  each read it again from the top. A `Document` reads it once and shares that
  reading.

  ```go
  doc, err := fhirpath.NewDocument(resource)
  for _, invariant := range invariants {
      result, err := doc.EvaluateCompiled(invariant)
  }
  ```

  Eight invariants over a Bundle of 50 entries: 0.55ms against 0.24ms, and
  under an R4 model 0.54ms against 0.27ms. A document keeps what it reads, so
  it costs memory in proportion to what is navigated and must not be shared
  between goroutines — see the reference page for both.


### Performance Improvements

* compile expressions instead of walking the parse tree ([#32](https://github.com/gofhir/fhirpath/issues/32)) ([83244fd](https://github.com/gofhir/fhirpath/commit/83244fd6498b8c13481c78af5bf14a6bb6a63a58))

  An expression kept the ANTLR tree and walked it on every call, so literals
  were re-parsed, identifiers rebuilt and operators recognised by comparing
  strings — per evaluation. That is settled once at compile time now.

* read an object's type in one pass, and try dates only on dates ([#34](https://github.com/gofhir/fhirpath/issues/34)) ([a18ed4f](https://github.com/gofhir/fhirpath/commit/a18ed4f32b6c400e3d197feb296589e33b7299a7))

  A type that is not written down was worked out by asking the object for one
  field after another, each a scan; and every string was offered to the date
  parsers, which try a regular expression per shape. Both are one pass now.

  Together with the above, against 1.6.0 on an Apple M4 Pro: navigating a
  Bundle of 100 entries takes 0.18ms where it took 0.70ms, and eight
  invariants over a Bundle of 50 take 0.60ms where they took 2.00ms — 3.9x
  and 3.3x. Through a `Document`, those invariants take 0.26ms, 7.8x. No
  conformance case moved.


### Documentation

* write down what a Document is for, and release it as a minor ([#35](https://github.com/gofhir/fhirpath/issues/35)) ([5abfbd0](https://github.com/gofhir/fhirpath/commit/5abfbd0cae92f36b6e0b830fe741879bb8123f45))

## [1.6.0](https://github.com/gofhir/fhirpath/compare/v1.5.3...v1.6.0) (2026-08-03)


### Features

* the regex functions take the flags parameter, and matchesFull is documented ([#28](https://github.com/gofhir/fhirpath/issues/28)) ([6a83f52](https://github.com/gofhir/fhirpath/commit/6a83f52185880a631308125721603eb80f7dacb6))

## [1.5.3](https://github.com/gofhir/fhirpath/compare/v1.5.2...v1.5.3) (2026-08-03)


### Bug Fixes

* string functions count characters, and substring no longer panics ([#25](https://github.com/gofhir/fhirpath/issues/25)) ([82b2264](https://github.com/gofhir/fhirpath/commit/82b22646e574ff1176e1d42eed0797da26fa3232))
* writing the FHIR namespace no longer changes what a type is ([#23](https://github.com/gofhir/fhirpath/issues/23)) ([838d781](https://github.com/gofhir/fhirpath/commit/838d78185c5bc6f5db55baba30ad745445b7c188))

## [1.5.2](https://github.com/gofhir/fhirpath/compare/v1.5.1...v1.5.2) (2026-08-03)


### Bug Fixes

* stop refusing valid regular expressions ([#20](https://github.com/gofhir/fhirpath/issues/20)) ([be2563b](https://github.com/gofhir/fhirpath/commit/be2563b795af5fbcdfb50374507f71e5065cd5a5))

## [1.5.1](https://github.com/gofhir/fhirpath/compare/v1.5.0...v1.5.1) (2026-08-03)


### Bug Fixes

* correct conversion of prefixed temperature units ([#18](https://github.com/gofhir/fhirpath/issues/18)) ([2d64365](https://github.com/gofhir/fhirpath/commit/2d64365f44b5e20d7b166d138c0fdc43afeb7e96))

  A prefix on a unit sitting on an affine scale multiplies the argument of the
  scale's function, not its result (UCUM §22.4). A milli-Celsius is a thousandth
  of a degree, not a thousandth of the distance from absolute zero.

  | Conversion | v1.5.0 | v1.5.1 |
  |---|---|---|
  | `1 'mCel'` in `'Cel'` | -272.87585 | **0.001** |
  | `1 'kCel'` in `'Cel'` | 273876.85 | **1000** |
  | `1 'cCel'` in `'K'` | 2.7415 | **273.16** |

  Only prefixed special units are affected; `Cel`, `[degF]` and `K` on their own
  were always correct. Neither conformance suite prefixes a special unit, so the
  measurement is unchanged at 919/928 (R4) and 1022/1037 (R5).

## [1.5.0](https://github.com/gofhir/fhirpath/compare/v1.4.0...v1.5.0) (2026-08-03)

Conformance against the official HL7 suites went from 91.7% to 99.0% (R4), and
the R5 suite is now measured as well at 98.6%. Both are checked in CI against
vendored copies, with a baseline that can only shrink.

| Suite | v1.4.0 | v1.5.0 |
|---|---|---|
| R4, with the R4 model | 864 / 928 (93.1%) | **919 / 928 (99.0%)** |
| R5, with the R5 model | not measured | **1022 / 1037 (98.6%)** |

### ⚠ Behaviour changes

No API changed and nothing stops compiling, but expressions that already run may
now answer differently. Each of these corrected a departure from the
specification, and every one is cited in `CONFORMANCE.md`.

| Expression | v1.4.0 | v1.5.0 | Why |
|---|---|---|---|
| `1 'kg' = 1 'm'` | `false` | `{ }` | "If this process returns empty … the result of the equality comparison is empty" |
| `1 year = 1 'a'` | `false` | `{ }` | A calendar year and a UCUM year are not comparable |
| `'2015'.toDateTime()` | a `String` | a `DateTime` | It did not convert at all |
| `1 / 0`, `5 div 0`, `5 mod 0` | error | `{ }` | "12 / 0 // empty ({ })" |
| `2.2 div 1.8` | error | `1` | div and mod accept Decimal |
| `('a' \| 'b') in 'c'` | `{ }` | error | "If the left operand has multiple items, an exception is thrown" |
| `Appointment.identifier.startsWith('x')` | `false` | error | A non-String input is not a string to test |
| `(1 \| 2 \| 3) & 'b'` | `'b'` | error | The singleton rule ends in an error |
| `(true \| 'foo').allTrue()` | `false` | error | "Takes a collection of Boolean values" |
| `@2012-04-15T15:00:00Z = @2012-04-15T10:00:00` | `false` | `{ }` | One offset, one none: nothing to convert into |

The pattern is the same throughout: an answer that looked confident where the
specification calls for empty or for an error. A `false` that should have been
empty makes an invariant pass when it should not decide.

### Features

* static analysis against a model ([#15](https://github.com/gofhir/fhirpath/issues/15)) — `Expression.Analyze(model, contextType)` reports faults evaluation cannot see: navigation the model contradicts (`Patient.name.given1`), a positional read of an unordered collection (`children().skip(1)`), and an `iif` criterion that cannot be a Boolean. Opt-in and separate from evaluation, which stays lenient
* read the FHIR element a primitive carries beside its value — `Patient.birthDate.extension(url)` now finds what FHIR stores in `_birthDate`, including a position that has extensions and no value at all
* resolve references that point inside the document — a fragment naming a contained resource, or a relative reference naming a Bundle entry, no longer needs an injected resolver
* cyclic `Time` arithmetic — `@T23:30:00 + 1 hour` wraps to `@T00:30:00`; `Time` had no arithmetic at all
* the functions FHIRPath 3.0.0 adds: `coalesce`, `defineVariable`, `difference`, `duration`, `lastIndexOf`, `repeatAll`
* `conformsTo()` answers for the profiles a model can resolve, without needing a validator
* reject a type specifier that resolves to no type, through the optional `TypeRegistry` interface
* measure against the R5 conformance suite as well ([#14](https://github.com/gofhir/fhirpath/issues/14))

### Bug Fixes

* function arguments are navigated from the scope the call sits in, not from its input — `name.given.combine(name.family)` was looking for `family` inside `given` and silently returning its input
* `repeat()` was registered but never implemented; it returned its input unchanged
* `defineVariable` scoped its variables to the whole expression rather than to the branch that defined them
* quantity conversion and the two duration systems: the calendar puts 365 days in a year against UCUM's 365.25, and the two meet only at a week and below
* `toQuantity()` follows the list the specification gives — a Boolean, the UCUM default unit `'1'`, and the published regex for strings, anchored
* the singleton evaluation rule, which was missing its error branch in three places
* regular expressions run in single line mode, so `.` matches a newline
* calendar arithmetic clamps to the end of the month: a year after the 29th of February is the 28th
* `union()` eliminated duplicates from its argument but not from its input
* `~` on decimals compares at the precision of the less precise operand
* `abs()` reaches Quantity and no longer rounds through a float
* a delimited identifier is read as the name it escapes: ``FHIR.`Patient` ``
* FHIR quantities map onto FHIRPath's calendar units, so `Patient.birthDate + Observation.value` works on data FHIR considers well formed

### Build

* the parser is generated from `grammar/fhirpath.g4`, and CI fails if the committed parser drifts from it — the previous grammar rejected `\"` inside a string
* the hardcoded UCUM table is gone, replaced by `gofhir/ucum`, which fixed a silent wrong answer on `100 '[degF]' > 50 'Cel'`
* golangci-lint migrated to v2 and pinned; CI asked for `latest` through an action that resolves within v1, so it passed while any current install rejected the config outright

## [1.4.0](https://github.com/gofhir/fhirpath/compare/v1.3.1...v1.4.0) (2026-03-09)


### Features

* auto-infer precision for lowBoundary/highBoundary on Decimal and Quantity ([18ce1ef](https://github.com/gofhir/fhirpath/commit/18ce1efa30bb5738fa917c46a359996ee99e1ff4))


### Bug Fixes

* preserve original string representation in Decimal type ([8878874](https://github.com/gofhir/fhirpath/commit/8878874da02ae97c4dbb13d9b7ee143b37cf9225))

## [1.3.1](https://github.com/gofhir/fhirpath/compare/v1.3.0...v1.3.1) (2026-03-09)


### Bug Fixes

* add spec-mandated timezone offsets to DateTime boundary functions ([803d23f](https://github.com/gofhir/fhirpath/commit/803d23f724da24afe900ce5f49cc25282d903a3e))
* discriminate URI subtypes and complex types in ofType() resolution ([27c77f3](https://github.com/gofhir/fhirpath/commit/27c77f32ac51ceeadb81ad37fc3c819a72957f81))

## [1.3.0](https://github.com/gofhir/fhirpath/compare/v1.2.0...v1.3.0) (2026-03-09)


### Features

* add type-aware polymorphic field resolution for ofType() ([8a2fe45](https://github.com/gofhir/fhirpath/commit/8a2fe45808e0f16ffd8e8b319abe2ef6fc839cb0))
* add TypeArgs support for type specifier function arguments ([0caf6e4](https://github.com/gofhir/fhirpath/commit/0caf6e4b5cc715f2f9b062a992a542088e0c14e0))
* implement lowBoundary() and highBoundary() FHIRPath 2.0 functions ([8869165](https://github.com/gofhir/fhirpath/commit/886916574a4a600d1d5803a2f17165995c57d4a8))

## [1.2.0](https://github.com/gofhir/fhirpath/compare/v1.1.0...v1.2.0) (2026-03-01)


### Features

* add FHIR Model interface for version-specific type resolution ([7f6e07f](https://github.com/gofhir/fhirpath/commit/7f6e07f54c2747f9bbe1b3fe157476deefde10d9))
* wire model.IsResource() via isResourceType helper ([c8e76e2](https://github.com/gofhir/fhirpath/commit/c8e76e2c55548382ec90b52c7603e91cc2431b44))

## [1.1.0](https://github.com/gofhir/fhirpath/compare/v1.0.3...v1.1.0) (2026-03-01)


### Features

* add %rootResource built-in variable support ([#6](https://github.com/gofhir/fhirpath/issues/6)) ([9e8f4cc](https://github.com/gofhir/fhirpath/commit/9e8f4ccab3bc6d6338eb8a489b57c5c3cd80fc72))
* implement aggregate() with $total and $index support ([#8](https://github.com/gofhir/fhirpath/issues/8)) ([382ae63](https://github.com/gofhir/fhirpath/commit/382ae63eec3bde180d5b8dcffe4d382392c73bcc))


### Bug Fixes

* as operator now filters collections instead of requiring singleton ([#7](https://github.com/gofhir/fhirpath/issues/7)) ([b5d4f80](https://github.com/gofhir/fhirpath/commit/b5d4f807b6bd7f7f45868fcdedd024b6435162e0))
* **docs:** per-language menus and improved Quick Start styling ([54d0422](https://github.com/gofhir/fhirpath/commit/54d0422d01b59fa3b5ae11a3e0c79f7c83bdf499))
* **docs:** upgrade Hugo to v0.155.3 and add npm ci step ([8d4a255](https://github.com/gofhir/fhirpath/commit/8d4a2556b61510465d105f9557f5f7d80ba2864b))

## [1.0.3](https://github.com/gofhir/fhirpath/compare/v1.0.2...v1.0.3) (2026-02-17)


### Bug Fixes

* remove gofhir/fhir/r4 dependency from tests ([c3bf7e3](https://github.com/gofhir/fhirpath/commit/c3bf7e3b1a025fccf29ee2c1cda2bfe28dd5f022))

## [1.0.2](https://github.com/gofhir/fhirpath/compare/v1.0.1...v1.0.2) (2026-01-26)


### Bug Fixes

* allow as() function to work on collections ([cf3042e](https://github.com/gofhir/fhirpath/commit/cf3042e3a3eb24f0c17a7b863ed0902707eb0998)), closes [#2](https://github.com/gofhir/fhirpath/issues/2)

## [1.0.1](https://github.com/gofhir/fhirpath/compare/v1.0.0...v1.0.1) (2026-01-24)


### Bug Fixes

* resolve golangci-lint issues ([6021b6e](https://github.com/gofhir/fhirpath/commit/6021b6ee2d6fdaf5985529d64cf295ec949fdf62))

## [0.2.0](https://github.com/robertoAraneda/gofhir/compare/fhirpath/v0.1.0...fhirpath/v0.2.0) (2026-01-17)


### ⚠ BREAKING CHANGES

* Package import paths have changed.

### Features

* initial release ([82ec28c](https://github.com/robertoAraneda/gofhir/commit/82ec28c30a38afb26bbf7b2503945573606da517))


### Code Refactoring

* migrate to multi-module monorepo architecture ([42ae0de](https://github.com/robertoAraneda/gofhir/commit/42ae0de8aa2f98cbe6e94fcef4736a6a0184bfb7))
