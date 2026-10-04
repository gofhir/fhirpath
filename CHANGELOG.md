# Changelog

## [1.9.10](https://github.com/gofhir/fhirpath/compare/v1.9.9...v1.9.10) (2026-10-04)


### Bug Fixes

* a shared root can be read from several goroutines ([#74](https://github.com/gofhir/fhirpath/issues/74)) ([718ac2c](https://github.com/gofhir/fhirpath/commit/718ac2c969db51363d888cabb64724db5fb86008))

  Two data races, both found with `go test -race`:

  - **A shared root.** A root read once with `types.JSONToCollection` and
    evaluated against from several goroutines (`eval.NewContextForRoot`) was
    written to: `Type()` kept the type it worked out on the object, and `Get()`
    kept the fields it read. Navigation alone raced. Only an object one
    goroutine reads keeps what it works out now, and what an evaluation returns
    is handed over shared, so results can be shared too.
  - **The regex cache.** A hit wrote when the pattern was last used under the
    read lock, so two goroutines evaluating `matches()` with the same pattern
    raced (fixed in #76).

  **New:** `ObjectValue.MarkPrivate()` and `MarkShared()`. A root built by a
  caller is shared and only ever read; one read by `eval.NewContext` belongs to
  that context, and `Context.Root()` documents how to share it.


### Performance Improvements

* Evaluate reuses compiled expressions, and a cache hit is a read ([#76](https://github.com/gofhir/fhirpath/issues/76)) ([429d881](https://github.com/gofhir/fhirpath/commit/429d88136ec548654917435f5ed7c91e5087bcbd))

  `fhirpath.Evaluate`, the call the README starts with, compiled its expression
  on every call; it goes through `DefaultCache` now: 6.7 µs to 2.2 µs, 123
  allocations to 36. A cache hit takes only the read lock instead of the write
  lock, so goroutines evaluating in parallel no longer queue on it, and the
  expression cache evicts the way CLOCK does, in a few steps rather than a
  pass over every entry.
* index an object's fields on its second read ([#78](https://github.com/gofhir/fhirpath/issues/78)) ([c0fe42a](https://github.com/gofhir/fhirpath/commit/c0fe42aa45f83d3c58b11593b326624515db4027))

  A field read scanned its object to the end, so an object read k times was
  scanned k times; over the R4 examples 88% of what a `Document` scanned was
  such re-reads. An object now indexes its fields on its second read:

  | over the whole R4 example corpus | before | after |
  |---|---|---|
  | `Document` with the R4 model | 5.06 s | 2.45 s |
  | one-shot `Evaluate` | 50.7 s | 20.8 s |
  | `bdl-3` on the 2 MB `Bundle-searchParams` example | timed out at 5 s | 11 ms |

  Memory grows 3–6%. `make bench-corpus` (#73) measures evaluation over a
  sample of the official examples.

## [1.9.9](https://github.com/gofhir/fhirpath/compare/v1.9.8...v1.9.9) (2026-10-03)


### Bug Fixes

* a primitive's id and extensions are reached by navigation ([#71](https://github.com/gofhir/fhirpath/issues/71)) ([e485f9f](https://github.com/gofhir/fhirpath/commit/e485f9f9a38a6f05780756455c9fed925f889af2))

  FHIR writes a primitive's `id` and extensions in the element beside it,
  under `_name`. `extension(url)` reached them; the `extension` property and
  `children()` did not:

  | on `"birthDate":"2020","_birthDate":{"id":"b1","extension":[…]}` | was | is |
  |---|---|---|
  | `birthDate.extension.count()` | 0 | 1 |
  | `birthDate.id` | empty | `b1` |
  | `name.family.extension.count()` | 0 | 1 |
  | `birthDate.children().count()` | 0 | 2 |

  CH Core's `ch-core-hm-3` and `ch-core-hm-4`, which compare
  `descendants().extension` with `family.extension` and `given.extension`,
  failed on every example that puts the eCH-11 name extension there.

  **`children()` and `descendants()` count differently.** An object's children
  read a primitive and its `_name` element together, as navigation does: one
  child carrying the `id` and extensions, and no `_name` node beside it, so
  `children().count()` on a Patient with `birthDate` and `_birthDate` is one
  less than before. `descendants()` reaches the same extensions, now through
  the primitive. A field named with an underscore that is not shaped as an
  element (an object, or an array of objects and nulls) is a child as before.

  Over the official R4, R4B and R5 examples, no answer changes.

## [1.9.8](https://github.com/gofhir/fhirpath/compare/v1.9.7...v1.9.8) (2026-10-02)


### Bug Fixes

* and, or and implies do not evaluate a right operand the left decides ([#69](https://github.com/gofhir/fhirpath/issues/69)) ([f68ddc2](https://github.com/gofhir/fhirpath/commit/f68ddc2b8d583b032ffbf609bf71e70727251698))

  When the left operand alone decides the result, the right one is no longer
  evaluated, as in the HL7 validator (`FHIRPathEngine.preOperate`):

  | | was | is |
  |---|---|---|
  | `true or X`, where `X` raises an error | the error | `true` |
  | `false and X` | the error | `false` |
  | `false implies X` | the error | `true` |

  FHIR's published invariants rely on it: `tim-9` (R4, R4B) failed on a timing
  with two `when` values and no offset, and `eld-11` (R5) on every element with
  more than one type. Over the R4B examples, two `StructureDefinition`
  constraints that raised a `TypeError` behind a false left operand now hold.

  Only a single Boolean decides. An empty left operand, or one that is not a
  Boolean, still evaluates the right one, so `{} or true` is `true` as before,
  and `xor` evaluates both. The official suite is unchanged.

  **An error on the right of a deciding left operand is no longer reported.**
  fhirpath.js evaluates both operands, so this is a divergence taken on
  purpose, recorded in `CONFORMANCE.md`; an expression that must behave the
  same on every engine should guard its right side with `iif()`.

## [1.9.7](https://github.com/gofhir/fhirpath/compare/v1.9.6...v1.9.7) (2026-10-01)


### Bug Fixes

* a root takes the type the model gives the path set with SetPath ([#67](https://github.com/gofhir/fhirpath/issues/67)) ([8d2ca02](https://github.com/gofhir/fhirpath/commit/8d2ca02fed4b4cccad2169a6c57b84840b7fa955))

  A validator evaluates an element's invariants with the element as the root,
  and gives the root's path with `SetPath` so that a model can type it. A
  primitive root was typed from its shape anyway:

  | root, with a model and `SetPath` | was | is |
  |---|---|---|
  | `"2019-12-08"` at `Observation.effectiveDateTime` | `Date` | `dateTime` |
  | `"2019-12-08"` at `Patient.birthDate` | `Date` | `date` |
  | `"final"` at `Observation.status` | `String` | `code` |

  AU Core's `au-core-obs-02` starts with `$this is dateTime`, and failed on a
  dateTime of day precision. A root read by `NewContext` is now read again as
  the type the model gives its path, once both are set, in either order; a
  choice the model knows only by its base name is resolved through its choice
  types. With no model, no path, or a path the model does not know, the root
  is read as before.

  **New:** `types.JSONToCollectionWithType(data, fhirType)` reads JSON as a
  declared type. A root given through `NewContextForRoot` is not retyped, so a
  caller that builds its own root should build it with this.

  **Types change with a model and a path.** An expression evaluated on such a
  root that relied on the guessed type answers differently.

## [1.9.6](https://github.com/gofhir/fhirpath/compare/v1.9.5...v1.9.6) (2026-10-01)


### Bug Fixes

* a resource takes the type it names, not the element's Resource ([#64](https://github.com/gofhir/fhirpath/issues/64)) ([c90d523](https://github.com/gofhir/fhirpath/commit/c90d5234d32eea82a9999baffd7a90d536941fca))

  With a model, a resource held by an element declared as the abstract
  `Resource` — `Bundle.entry.resource`, every `contained`,
  `Parameters.parameter.resource` — was typed as `Resource`, not as the type
  its `resourceType` names:

  | with a model | was | is |
  |---|---|---|
  | `entry.resource.type().name` | `Resource` | `Patient` |
  | `entry.resource is Patient` | `false` | `true` |
  | `entry.resource.ofType(Patient)` | empty | the Patient |
  | a field of it, `contained.birthDate.type().name` | guessed, `Date` | `date`, from the model |
  | `contained.id.type().name`, id `"2020"` | `Date` | `String` |

  FHIR's `bdl-11` failed on every document Bundle, `bdl-12` on every message,
  and `dom-3` raised an error where a contained resource's id looked like a
  date. Over the official R4, R4B and R5 examples, those invariants now hold
  and nothing else changes.

  **Types change with a model.** An expression that asked for `Resource`
  where it meant the resource, or that read a `System.String` element
  (`Resource.id`, `Extension.url`) as the date it looked like, answers
  differently. Such an element is now the System type its type code names, as
  `http://hl7.org/fhirpath/System.String` says, rather than a FHIR type named
  by that URL. Without a model nothing changes.

## [1.9.5](https://github.com/gofhir/fhirpath/compare/v1.9.4...v1.9.5) (2026-09-30)


### Bug Fixes

* functions that take one value end with an error when given more ([#60](https://github.com/gofhir/fhirpath/issues/60)) ([190d543](https://github.com/gofhir/fhirpath/commit/190d54314ad867f1e22f81423235033d04181490))

  Eighteen functions defined on a single value answered for the first item of
  a collection, or answered empty, instead of ending with the error the
  specification requires, as the conversions already did in 1.9.2:

  | | was | is |
  |---|---|---|
  | `abs`, `ceiling`, `exp`, `floor`, `ln`, `log`, `power`, `round`, `sqrt`, `truncate` | the first item's answer: `(1 \| -2).abs()` is `1` | `SingletonExpectedError` |
  | `lowBoundary`, `highBoundary`, `precision` | the first item's answer | `SingletonExpectedError` |
  | `encode`, `decode`, `escape`, `unescape`, `comparable` | empty | `SingletonExpectedError` |

  **An expression that relied on the first item now fails.** Where the first
  item was meant, say so: `first().abs()`. A single item or an empty
  collection answers as before, and no R4 or R5 example relied on it.

  `as()` over a collection is unchanged: it filters by type rather than
  raising the error, a divergence taken on purpose and recorded in
  `CONFORMANCE.md`.

## [1.9.4](https://github.com/gofhir/fhirpath/compare/v1.9.3...v1.9.4) (2026-09-30)


### Performance Improvements

* with a model, an absent field costs one read, and an absent choice two ([#56](https://github.com/gofhir/fhirpath/issues/56)) ([0d01d45](https://github.com/gofhir/fhirpath/commit/0d01d4540e1e5cdb1b585a7240454cc02d5c8eac))

  With a model, a field the model types was read again untyped when it was
  absent, and a choice element was tried one read per choice type. Both now
  cost what 1.9.2 made them cost without a model:

  | on a 1.1 MB resource, with a model | was | is |
  |---|---|---|
  | an absent field | 1.7 ms | 1.1 ms, as a present one |
  | an absent choice (Observation.value's 11 types) | 7.4 ms | 1.7 ms |

  No answer changes: the R4 examples give the same results as 1.9.3.

## [1.9.3](https://github.com/gofhir/fhirpath/compare/v1.9.2...v1.9.3) (2026-09-30)


### Bug Fixes

* an operator given more than one item says which side ([#55](https://github.com/gofhir/fhirpath/issues/55)) ([6b94798](https://github.com/gofhir/fhirpath/commit/6b9479893c132302c34ce4d43af729ba39e682d9))

  `(1 | 2) + 1` reported "got 3 elements", the two operands' counts added
  together. The arithmetic and comparison operators now say which side held
  what: "+ expects a single item on each side, got 2 on the left and 1 on the
  right". The error is the same `SingletonExpectedError`.
* each step of a path resolves against the element before it ([#54](https://github.com/gofhir/fhirpath/issues/54)) ([cbd485d](https://github.com/gofhir/fhirpath/commit/cbd485d8830ad2975ee86fc3113cedbcba8217a9))

  **This corrects a regression in 1.9.2.** With a model, four choice elements
  across the R4 examples answered empty:

  | | 1.9.1 | 1.9.2 | 1.9.3 |
  |---|---|---|---|
  | `Claim.diagnosis.diagnosis` | the variant | empty | the variant |
  | `Claim.procedure.procedure` | the variant | empty | the variant |
  | `ExplanationOfBenefit.procedure.procedure` | the variant | empty | the variant |
  | `ImplementationGuide.definition.page.name` | the variant | empty | the variant |

  The cause is older. A backbone's fields are resolved beneath the path the
  backbone was reached by, and that path was lost between steps, so in `a.b.c`
  the third step was resolved against `a`: `Claim.diagnosis.diagnosis` looked
  up `Claim.diagnosis`, the backbone itself. Guessing type suffixes covered for
  it on choice elements until 1.9.2 stopped guessing for elements the model
  knows. The path now travels with each object, so every step resolves beneath
  the element before it.

  **Types change, not only choices.** The same lost path typed fields three
  levels down, and the children of `children()` and `descendants()`, as if they
  sat under the first step. With a model they now take their own element's
  type: a `date` beneath a backbone is a `date`, not the `string` of a
  same-named field of the resource. One more field stops answering for its
  sibling, as `form` and `conclusion` did in 1.9.2: `TestScript`'s
  `assert.response` no longer returns `responseCode`.

  Without a model nothing changes. Over 505,426 evaluations of the R4 examples
  with the R4 model, the only answers that differ from before 1.9.2's change
  to absent fields are the sibling corrections named here and in 1.9.2; the
  official suite is unchanged at 927/935 (R4) and 1034/1048 (R5).

## [1.9.2](https://github.com/gofhir/fhirpath/compare/v1.9.1...v1.9.2) (2026-09-30)


### Bug Fixes

* conversions end with an error when given more than one item ([#49](https://github.com/gofhir/fhirpath/issues/49)) ([dc78783](https://github.com/gofhir/fhirpath/commit/dc7878352073633ec778259d6e72fc60e489722b))

  `toBoolean`, `toInteger`, `toDecimal`, `toString`, `toDate`, `toDateTime`,
  `toTime` and their `convertsTo` pairs read the first item of a collection and
  answered for it. The specification says more than one item ends the
  evaluation with an error, as `toQuantity` already did here and as fhirpath.js
  does:

  | | was | is |
  |---|---|---|
  | `(1 \| 2).toString()` | `'1'` | `SingletonExpectedError` |
  | `(1 \| 2).toInteger()` | `1` | `SingletonExpectedError` |

  **An expression that relied on the first item now fails.** Where the first
  item was meant, say so: `first().toString()`. A single item or an empty
  collection answers as before.


### Performance Improvements

* an absent field costs two reads of the object, not fifty-four ([#50](https://github.com/gofhir/fhirpath/issues/50)) ([5f8c96c](https://github.com/gofhir/fhirpath/commit/5f8c96cfd4fa5d76e9e7afe4c1df10ee40367bc3))

  A name the object does not hold was tried as a choice element with each of 53
  type suffixes before it was answered empty, and each try read to the end of
  the object. The variants are now found in one read of the keys:

  | | was | is |
  |---|---|---|
  | `subject.exists()`, absent, on a 1.1 MB resource | 29 ms | 1.7 ms |
  | `ref-1` over the 12,085 references of the R4 core ImplementationGuide | 18 min | 42 s |
  | the same, with the root read once through a `Document` or `EnableCaching()` | 126 ms | 18 ms |

  **One answer changes with a model.** An element the model knows and gives no
  choice types is no longer tried as one: `subject` on a Basic holding
  `subjectString` used to answer the string, and is empty now. Without a model,
  and for real choice elements, answers are unchanged; the official suite is
  unchanged at 927/935 (R4) and 1034/1048 (R5).

  *Corrected in 1.9.3:* four choice elements beneath a backbone did change, and
  answered empty. See 1.9.3.

## [1.9.1](https://github.com/gofhir/fhirpath/compare/v1.9.0...v1.9.1) (2026-08-30)


### Bug Fixes

* extraction reads the offset a value is placed at ([#47](https://github.com/gofhir/fhirpath/issues/47)) ([21f8b38](https://github.com/gofhir/fhirpath/commit/21f8b380937e66896480c79bb3af5cf0c6436eda))

  A value given a default offset through `WithDefaultOffset` in 1.9.0 was placed
  by some operators and not by others, so it meant different things depending on
  which one was asked. Four places, all now reading the offset the value is
  placed at:

  | | was | is |
  |---|---|---|
  | `timezoneOffsetOf()` | empty | the offset it is placed at |
  | arithmetic — `v + 1 day` | dropped the default, and durations from the result measured from elsewhere | carries it |
  | `~`, `in`, `\|`, `distinct()` | ignored it, while `=` honoured it — one instant written two ways counted as two | agree with `=` |
  | `lowBoundary` / `highBoundary` | the full 26-hour span of every offset the value might have had | the instant it names |

  **Nothing changes without a default.** A value that states no offset and was
  given none answers exactly as it did in 1.9.0, and the official suite is
  unchanged at 927/935 (R4) and 1034/1048 (R5).

  Found by the CQL engine that asked for defaults: it mapped which of its own
  operators consult one and sent the table, and checking this engine against it
  turned up all four. Three of them predate the feature — writing down what a
  default should reach is what made them checkable.

## [1.9.0](https://github.com/gofhir/fhirpath/compare/v1.8.1...v1.9.0) (2026-08-27)


### Features

* let a caller say what a bare value's offset is ([#44](https://github.com/gofhir/fhirpath/issues/44)) ([ba9242f](https://github.com/gofhir/fhirpath/commit/ba9242f727261075408b24f833df24bcff31a87f))

  Comparing a `DateTime` that states a timezone offset with one that does not has
  no answer, because FHIRPath makes the default offset "a policy decision" and
  this engine picks none. A caller whose own language settles the question can
  now say what the bare value's offset is:

  ```go
  period, _ := types.NewDateTime("2020-01-01T00:00:00.0")
  encounter.Compare(period.WithDefaultOffset(-5 * time.Hour))   // answers
  ```

  | | |
  |---|---|
  | `DateTime.WithDefaultOffset(time.Duration)` | the offset to assume when the value states none; a stated one wins |
  | `DateTime.EffectiveOffset() (int, bool)` | the offset to place the value at — stated, else defaulted, else none |

  Nothing applies a default on its own, so FHIRPath callers see no change: such
  a comparison still yields `{ }`.

  **The default is remembered apart from what was written.** `String`, `HasTZ`
  and `TZOffset` answer about the value as written, so a literal written without
  an offset still evaluates to itself; `EffectiveOffset` answers what to place it
  at, and is what ordering, equality, `difference` and `duration` all read — one
  lookup, so they cannot disagree about one value.

  This came from a CQL engine built on this library, which needs it because CQL
  closes what FHIRPath leaves open: "If no timezone offset is supplied, the
  timezone offset of the evaluation request timestamp is assumed". Every
  published eCQM library declares its measurement period without an offset,
  while FHIR requires one on served data, so an encounter two hours into the
  period compared against a bare bound — and a `where` dropping what it cannot
  confirm took the patient out of the population.

  The shape is theirs as much as ours: they implemented two candidate designs
  against their conformance corpus and reported what broke. A first version
  wrote the default into the value's own offset and cost them twenty-two cases,
  because the value then printed with an offset it was never written with. A
  first signature took minutes as an `int`, where no range can tell "five
  minutes west" from "five hours west written wrong" — `time.Duration` puts the
  unit in the call. Their corpus passes 2084/0 on the merged shape, unchanged
  across four process timezones.

## [1.8.1](https://github.com/gofhir/fhirpath/compare/v1.8.0...v1.8.1) (2026-08-23)


### Bug Fixes

* an offset that is missing is not a precision that is missing ([#41](https://github.com/gofhir/fhirpath/issues/41)) ([09634a0](https://github.com/gofhir/fhirpath/commit/09634a07f9274852c78332668b6886b8fc1220e0))

  Comparing two DateTime values where one writes a timezone offset and the
  other does not reported `ErrPrecisionMismatch` — "temporal values are
  specified to different precisions" — with both values specified to the
  millisecond.

  ```
  2020-03-05T10:00:00.0Z  vs  2021-01-01T00:00:00.0
    err=temporal values are specified to different precisions   // both to the ms
  ```

  Expression results are unchanged: such a comparison has no answer and yields
  `{ }`, which is what a FHIRPath caller already saw. The specification makes
  the default offset "a policy decision" and this engine provides none.

  What changed is what a Go caller of `types.Compare` is told. Two additions:

  | | |
  |---|---|
  | `types.ErrOffsetMismatch` | reported when one value carries an offset and the other does not |
  | `types.IsUnknownTemporalComparison(err)` | true for either sentinel — both mean the comparison has no answer |

  A genuine difference in precision still reports `ErrPrecisionMismatch`, and
  neither sentinel stands in for the other. Reported by a user reading the
  message and looking, reasonably, at precision.

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
