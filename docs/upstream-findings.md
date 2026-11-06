# Findings by library

What working on this engine turns up in other libraries, in the
specification, and in this engine itself, kept per library as it is found.
Each entry gives what was measured, how to reproduce it, and where it stands.
A finding is not worked around here (charter, principle 10): it is reported
where it belongs, or recorded as a known loss, or left as a decision.

Status: **open** (not reported), **reported** (link), **decided** (a recorded
decision keeps it as it is), **fixed** (link).

## fhirpath.js (5.2.0)

Measured with `make three-engine` (`difftest/`), R4 and R5 models.

| # | Finding | Reproduce | Status |
|---|---|---|---|
| JS-1 | A dateTime fraction of more than 9 digits is read as that many milliseconds: `2020-01-15T10:30:00.1234567890Z + 1 year` gives `2021-01-29T17:26:07.890`, 1,234,567,890 ms later. This engine and the Java engine truncate to `2021-01-15T10:30:00.123Z` | battery `fhir-temporal-form`, `dateTime-19.json`; found in #231 | open (the user chose not to file it yet, 2026-10-26) |
| JS-2 | Adding a calendar duration to a FHIR temporal value it reads as a string fails with an internal error, `otherQuantity.hasSameMappedCalendarUnit is not a function`, not a type error: `Observation.issued + 1 year` over `"issued": "2020"` | probe battery of #231 (`X + 1 year`) | open |
| JS-3 | A FHIR `dateTime` with a trailing `T` or a time without seconds (`2020T`, `2020-01-15T10:30Z`) is read as a DateTime, against FHIR's datatypes page ("Seconds must be provided"); the Java engine refuses it | battery `fhir-temporal-form` | decided: this engine follows the text and Java (CONFORMANCE.md, "A FHIR date, dateTime, instant or time is read by FHIR's form") |
| JS-4 | A FHIR `time` written with a leading `T`, `T10:30:00`, is read as a Time; FHIR's form is `hh:mm:ss` | battery `fhir-temporal-form`, `time-32.json` | decided, as JS-3 |
| JS-5 | A decimal is held as a float64: `1e1000` is `Infinity` (`toString()`, arithmetic), `1e-1001` is 0 (`> 0` false, `abs()` 0), `1e1000 * 0` is `NaN`. FHIRPath's Decimal is exact to at least 28 digits, and "Operations that cause arithmetic overflow or underflow will result in empty" | battery `decimal-beyond-bound`; found in #232 | open |
| JS-6 | Unary minus on a FHIR decimal is an error, `Unary - can only be applied to …`, even on `1.5` | battery `decimal-beyond-bound`, `-V` | open |
| JS-7 | `precision()` is not implemented; `lowBoundary()` of a decimal written with an exponent errs, `Cannot convert string to decimal` | battery `decimal-beyond-bound` | open |
| JS-8 | A decimal's boundary counts the places of a JavaScript number, not of the text, wherever the value is not a positive literal: `(-1.0).lowBoundary()` is `-1.5` and `(-0.10)` `-0.15` (`(0.0)` gives `-0.05`, rightly), `(1.5 + 0.00000000000000000).lowBoundary()` `1.45`, a resource's `1.00000000000000000` `0.5`, and one written with an exponent the double's places, `1.0e0` `0.5` and `1.50e-1` two places (`0.145`). FHIRPath 3.0.0: "The least possible value of the input to the specified precision" | battery `boundary-places` | open |
| JS-9 | The boundaries stop at 8 places: `(1.587).lowBoundary(9)` is empty (`FP_Decimal.MAX_PRECISION = 8`, src/types.js). The text leaves the maximum to the implementation | battery `boundary-places` | decided, as JAVA-12 |
| JS-10 | Boundaries over a Quantity are not implemented: `(1.5 'mg').lowBoundary()` and a resource's `valueQuantity` err, `Expected a Decimal, Date, DateTime, or Time` | battery `boundary-places` | decided: CONFORMANCE.md, "boundaries over a Quantity" (R5's `rng-2` needs them) |
| JS-11 | With no model, a type question naming a FHIR type fails with an internal error, `Cannot read properties of undefined (reading 'availableTypes')`, not a type error or an answer: `Patient.gender is code`, `Patient.telecom.rank is positiveInt`, `Patient.descendants().ofType(integer64)`. System types answer (`Patient.gender is String` true) | `node difftest/batteries/no-model/measure.js`; found 2026-11-02 | open |

## HL7 Java FHIRPath engine (validator 6.10.2)

Measured with `make three-engine`, the pinned validator jar.

| # | Finding | Reproduce | Status |
|---|---|---|---|
| JAVA-1 | A FHIR `instant` short of the second or with no offset (`2020`, `2020-01-15`, `2020-01-15T10:30:00`) is read as a DateTime; FHIR: "SHALL specified at least to the second and SHALL include a time zone" | battery `fhir-temporal-form`, `instant-*` | decided: this engine follows the text and fhirpath.js; CONFORMANCE.md, "Measured, and not followed" |
| JAVA-2 | A FHIR `date` with a time, `2020-01-15T10:30:00Z`, is read as a DateTime; FHIR: "YYYY, YYYY-MM, or YYYY-MM-DD" | `date-27.json` | decided, as JAVA-1 |
| JAVA-3 | A FHIR `time` holding a date, `2020-01-15`, answers `> @T09` true and `> @2019` an error: a time, and not one | `time-33.json`, and the probe `> @2019` | decided, as JAVA-1 |
| JAVA-4 | A FHIR `time` with a leading `T`, `T10:30:00`, answers `> @T09` false | `time-32.json` | decided, as JS-4 |
| JAVA-6 | A decimal is read through a double for some operations: `1e-1001 > 0` is false and `abs()`, `sqrt()`, `power(2)` of it `0.0`, though `toString()` gives `1e-1001` | battery `decimal-beyond-bound`; found in #232 | open |
| JAVA-7 | Over a decimal past a long: `ceiling()`/`floor()` clamp to int32 (`2147483647`), `truncate()`, `div`, `sum()`, `avg()`, `min()`, `max()` err (the last four on `1.5` too), `mod` throws `java.lang.ArithmeticException`; `convertsToDecimal()` and `convertsToQuantity()` of `1.5` are false, `toQuantity()` of it empty; `lowBoundary()` of `1e1000` is `0.500000001000` | battery `decimal-beyond-bound` | open |
| JAVA-8 | `'1e1000'.toDecimal()` converts a string with an exponent, which FHIRPath's String form `(\+|-)?\d+(\.\d+)?` does not take | battery `decimal-beyond-bound` | decided: not followed (CONFORMANCE.md, "Measured, and not followed") |
| JAVA-5 | A `dateTime` with a time and no offset is placed in the JVM's local offset: `2020-01-15T10:30:00 + 1 year` gives `2021-01-15T10:30:00-03:00` on a machine in Chile, so the answer depends on where it runs. FHIRPath calls a default offset "a policy decision" | probe battery of #231 | open; this engine leaves the offset to its caller (`WithDefaultOffset`) |
| JAVA-9 | A decimal's boundary is rounded half up to the precision asked, not outward, so it can fall inside the interval the input stands for: `(1.00000000000000000).highBoundary()` is `1.00000000` (the greatest value is 1.000000000000000005), `(1.123456789012345).highBoundary(0)` `1`, `(0.00000000000000001).highBoundary(16)` `0.0000000000000000`, below the input. FHIRPath 3.0.0's examples round outward: `1.587.highBoundary(2) // 1.59` | battery `boundary-places` (`applyPrecision` in `FHIRPathEngine`) | open |
| JAVA-10 | Some decimal boundaries end the evaluation, `Error evaluating FHIRPath expression`: `(123.4567890123456789).highBoundary(16)`, and every boundary of `-0.00000000000000001` measured but at precision 0, where it answers `-1E-18` (low) and `-1E-16` (high) | battery `boundary-places` | open |
| JAVA-11 | A zero stands for anything within 0.5 of it however many places it is written to: `(0.0).lowBoundary()` and `(0.00).lowBoundary()` are `-0.50000000`, though `(0.0).precision()` is 1; `(0).lowBoundary(0)` is `-0` | battery `boundary-places`, and the probe `(0.00).lowBoundary()` | open |
| JAVA-12 | The boundaries stop at 16 places, `precision < 17` in `funcLowBoundary`: `(1.587).lowBoundary(17)` is empty. The text leaves the maximum to the implementation | battery `boundary-places` | decided: this engine goes to 18 (CONFORMANCE.md, "Max precision for `lowBoundary`/`highBoundary`") |
| JAVA-13 | A decimal written with an exponent is read without it for its precision: `1.5e2`'s `precision()` is 1 (the mantissa's places), its `lowBoundary()` `1.450000002` and `highBoundary()` `1.550000002` (the exponent's digits spliced into the result), `1.5E+2`'s `1.5E+15000`; a negative exponent ends the evaluation (`lowBoundary()` over `1.50e-1`); and `Observation.value.value = 150` over `1.5e2` is false. A value computed from it carries the defect: `(value * 2).lowBoundary()` and `(value + value).lowBoundary()` are `3.0E+15000`; `(-value).lowBoundary()` is `-150.50000000`, `value.abs()`'s `149.95000000` | battery `decimal-exponent` | open |
| JAVA-14 | A resource's number outside JSON's grammar is read, and read off by one: `-.5` as 0.5, `5.` as 6, `1.` as 2, `-0.` as 1, `007.50` as 8.50, `1.e5` as 100001, `01` as 2, `00` as 1, `-01` as 0 for `value + 1`; `-`, `1e`, `1.5.5`, `0x1` and `1_000` end the evaluation with an error. RFC 8259 §6 refuses all of them, as does R4 and R4B's decimal regex; fhirpath.js refuses them in `JSON.parse` | `difftest/batteries/resource-number-grammar/refused/measure.sh` | open |
| JAVA-15 | An R5 `integer64` written as the JSON string FHIR requires (json.html §2.1.6.4.2) is read as a string: `size = 5` is false on `"+5"`, `size.toDecimal()` is `+5`, `-0` and empty for `01`, and `+ 1`, `* 2`, `> 1` end the evaluation on `"5"`. R5's package types `integer64.value` `System.Integer` | `difftest/batteries/declared-integer`, validator 6.10.2; found in plan 016 (#251) | open |
| JAVA-16 | `type().name` of a FHIR primitive is capitalized: `Integer`, `PositiveInt`, `Integer64` for `valueInteger`, `rank`, `size`, where FHIR names the types `integer`, `positiveInt`, `integer64` and fhirpath.js answers so | `difftest/batteries/declared-integer`, validator 6.10.2; found in plan 016 (#251) | open |
| JAVA-17 | `ofType` keeps no FHIR primitive by the System type it converts to: `gender.ofType(System.String)`, `rank.ofType(System.Integer)`, `birthDate.ofType(System.DateTime)`, `active.ofType(System.Boolean)` and `name.given.ofType(System.String)` are all empty, against R5 fhirpath.html 2.1.9.1.2 ("ofType() does not have such restrictions - both of the following are valid: `Patient.name.given.ofType(FHIR.string)`, `Patient.name.given.ofType(System.string)`"); `ofType(String)` unqualified is an error | `difftest/batteries/primitive-type-operators`, validator 6.10.2; found in plan 017 | open; not followed (CONFORMANCE.md, "Measured, and not followed") |

## The FHIR specification (HL7)

| # | Finding | Evidence | Status |
|---|---|---|---|
| SPEC-1 | R5's `dateTime` regex (`StructureDefinition-dateTime`, `dateTime.value`) accepts a sign with no offset, `2020-01-15T10:30:00+`, and an offset after a month or a day, `2020-01Z`; R4 and R4B's do not. R5's datatypes page calls the regexes "informative, not normative" | pinned `hl7.fhir.r5.core` 5.0.0 | open (the user chose not to file it yet); to be checked against the current build before drafting |
| SPEC-2 | R5's `dateTime` definition comments "UTC offset is allowed for dates and partial dates"; R4 and R4B do not say so, and neither reference engine reads `2020-01-15Z` | pinned core packages; battery `fhir-temporal-form` | decided: refused, as both engines do |
| SPEC-4 | The decimal's JSON form is "restricted to the precision limits documented in XML schema for xs:double and xs:decimal" (R4, R4B, R5), so past about 1.8e308 a number in exponent form is no xs:double; yet R5's regex allows a nine-digit exponent and R4's any. Which bounds a FHIR decimal is left unsaid | `datatypes.html`; `StructureDefinition-decimal` regex | open |
| SPEC-3 | The text requires an offset on a `dateTime` with a time and the seconds on a `time`; both reference engines read values without them, and none occurs in the R4, R4B or R5 official examples | `datatypes.html`; scan of the pinned example packages | decided: read, as both engines do (user, 2026-10-26) |
| SPEC-5 | R5's decimal regex (`StructureDefinition-decimal`, `decimal.value`) reads `-?(0|[1-9][0-9]{0,17})(\.[0-9]{1,17})?([eE][+-]?[0-9]{1,9}})?`: a stray `}` before the last `)?`, so the pattern as written requires a literal `}` after the exponent | pinned `hl7.fhir.r5.core` 5.0.0; found in the review of plan 011 (#235) | open |
| SPEC-6 | R4 and R4B's search page gives `1e2` as "to 1 significant figures precision ... [95 ... 105)", which is not one significant figure; R5 corrects it to "[50 ... 150)" | `search.html` in R4, R4B and R5 | decided: R5's reading (plan 014) |
| SPEC-7 | R5's integer regex (`StructureDefinition-integer`, `integer.value`) reads `[0]|[-+]?[1-9][0-9]*`: it admits `+1`, which JSON does not, and refuses `-0`, which JSON and R4's `-?([0]|([1-9][0-9]*))` admit | pinned `hl7.fhir.r4.core` 4.0.1 and `hl7.fhir.r5.core` 5.0.0; found in the review of plan 015 (#238) | open; a resource's number is read by JSON's grammar (plan 015) |
| SPEC-8 | `unsignedInt.value` and `positiveInt.value` are typed `System.String` in R4, R4B and R5 (`StructureDefinition-unsignedInt`, `-positiveInt`; R4 also gives their fhir-type extension as `string`), while `integer.value` is `System.Integer`, the datatypes page calls both an `integer`, and json.html writes them as JSON numbers | pinned `hl7.fhir.r4.core` 4.0.1, `hl7.fhir.r4b.core` 4.3.0, `hl7.fhir.r5.core` 5.0.0; found in the review of plan 016 (#251) | open |
| SPEC-9 | The FHIR FHIRPath page moved the type-test exception between versions. R4 and R4B (fhirpath.html §2.9.1.2) say "Note that as() does not have such restrictions", with `Patient.name.given.as(System.string)` valid. R5 (§2.1.9.1.2) says the same of ofType(), and R5 rewrote `Observation-code-value-date` from `value.as(DateTime)` to `value.ofType(dateTime)` along with it. Not a defect of R4: a version-dependent rule, with no note in R5's change list. The R4 sentence is also ambiguous: "valid" may mean only that no error is raised. fhirpath.js 5.2.0 and the HL7 validator 6.10.2 answer `as()` as `is()` in R4 and R5 alike | pinned `fhir-r4.txt`, `fhir-r4b.txt`, `fhir-r5.txt`; batteries `primitive-type-operators`, `as-system-type`; gofhir/fhirpath#241 | open (worth an HL7 note): this engine reads ofType() by the table in every version (017) and as() as is() in every version, with both references (018) |

## gofhir/searchindex

| # | Finding | Evidence | Status |
|---|---|---|---|
| SI-1 | Adopting the release with #231: `TestDateFormsInvalidForTheirType` indexes, with an "invalid form" warning, values outside their type's form (a dateTime without seconds, an instant without a time or an offset, a date with a time). fhirpath now hands them over as strings, so they go unindexed; to keep indexing them searchindex reads the string with its own parser | its tests run against this tree through `go.work` | open, for searchindex to adapt (user, 2026-10-26) |
| SI-2 | `TestFhirpathInvalidFormsKnownLoss` fails as designed once #231 is in: the five values come out of the known loss | as SI-1 | open, as SI-1 |
| SI-3 | Adopting the release with #232: a decimal beyond `types.DecimalDigits` now reaches it. `adr001.go` `decimalOf` hands `n.Value()` to `number()` and `readQuantity` unchecked, and `canonical()` takes it to ucum's `Rat()`, which builds 10^999999999 for `1e999999999`. It checks `types.InDecimalRange` only for SampledData. Needs the same check (report the value, do not index it) and a test with such a value, before it adopts the release | review of plan 010 (#232) | open, for searchindex |
| SI-4 | `adr001.go:798` decodes SampledData with its own `decimal.NewFromString`. Under shopspring/decimal v1.6.0, which fhirpath requires after #235, it reads a Unicode minus (`1e−2` → 0.01) and refuses an exponent past 10,000 before its own range check runs. Should decode through `types.ParseDecimal` | review of plan 011 | open, for searchindex |

## gofhir/cql

| # | Finding | Evidence | Status |
|---|---|---|---|
| CQL-1 | Calls `.Value()` on fhirpath decimals and quantities (`eval/evaluator.go`); after #232 a decimal read beyond the bound reaches it, to check with `types.InDecimalRange` before computing | review of plan 010 | open, to check |

## gofhir/validator

| # | Finding | Evidence | Status |
|---|---|---|---|
| VAL-1 | Its tests pass alike against main and against #231 | `go test ./...` through `go.work` | nothing to do |

## gofhir/models

| # | Finding | Reproduce | Status |
|---|---|---|---|
| MOD-1 | The generated `FHIRPathModel()` has no `IsAbstract` and describes none of the abstract types' own elements (`Resource.id`, `Resource.meta`, `DomainResource.text`, `DomainResource.contained`...; `Element.id` is described), which the StructureDefinition snapshots list. Proposal: an `IsAbstract(typeName)` read from `StructureDefinition.abstract`, plus those elements, so `Analyze` can tell `contained.contained` from a misspelling | plan 019 (`conformance/analyze_abstract_test.go` builds both around the published models) | reported: gofhir/models#90 |

## buger/jsonparser

| # | Finding | Reproduce | Status |
|---|---|---|---|
| JP-1 | jsonparser types tokens RFC 8259 refuses as a Number: `-.5`, `5.`, `01`, `1.e5`, `-`, `1e`, `1.5.5`, `1e5e5`, `1-2`, `0x1`, `1_000` (it refuses `.5` and `+1`). It does not claim to validate a document, so this engine checks the grammar itself (plan 015); a bare `-` was read as Integer 0 through `strconv.ParseInt` | measured in plan 015 (#238) | not a defect: jsonparser documents no validation; nothing to report |
| JP-2 | jsonparser reads a token holding a byte of JSON's structure one way in one call and another in the next. `{"value":1:5,"unit":"mg"}`: `Get(data, "unit")` answers "Key path not found" while `ObjectEach` reads `unit`. `{"x":1"2}` and `{"x":1{2}`: `Get(data)` on the root fails ("can't find closing '}' symbol") while `Get(data, "x")` answers the token and `ObjectEach` reads the object; the engine reads such a root as empty, as on main | found in plan 015's round 2 (#238) | open; invalid JSON, which jsonparser does not claim to validate: to be weighed before reporting |

## gofhir/fhirpath (this engine)

| # | Finding | Status |
|---|---|---|
| FP-1 | A FHIR temporal value outside its form was read by the FHIRPath literal grammar, or by the untyped guess (`birthDate` with a time as a DateTime) | fixed by #231 |
| FP-2 | A FHIR decimal beyond `DecimalDigits` is read as absent, with no error: `NewDecimal`'s `ErrDecimalRange` is dropped in `jsonValueToFHIRValueWithType` (`types/object.go`, the decimal branch) | fixed by #232: held as written, compared exactly, computing with it empty |
| FP-3 | Without a model, a choice variant can read differently by route, which 1.16.6 already did: `valueInstant: "2020"` is a DateTime by `.value` and a Date by `.valueInstant`; `valueTime: "10:30"` a Time and a String | open; pinned by `TestWithoutAModelEveryRouteReadsAValueAsBefore` |
| FP-5 | `ImplicitPrecision` counts the characters after the point in the text, exponent included: `1.5e2` gives `precision()` 3, `lowBoundary()` 149.99950000 | fixed for #246 (plan 014) |
| FP-6 | An exponent past int32 (`1e9999999999`, valid under R4's regex, not R5's) is not read: the decimal library holds it in an int32. fhirpath.js and Java read it | open; pinned by `TestAnExponentPastInt32IsAKnownLoss` |
| FP-4 | The error for a value read as a string under a temporal FHIR type names the FHIR type: `cannot apply '+' to dateTime and Quantity` | open (review of #231, round 1) |
| FP-7 | Under shopspring/decimal v1.5+ a decimal whose exponent passes `MaxDecodeExponent` (10,000) is read as absent again, and v1.6 reads a Unicode minus | fixed by #239 |
| FP-8 | Results depend on shopspring's process-wide `TrimTrailingZeros` (and possibly `UseScientificNotation`): `(10.0 / 4).toString()` is `2.5000000000000000` when another module sets it false | fixed by #245 |
| FP-9 | A resource's decimal is read by the decimal library's grammar, not FHIR's: `-.5`, `5.`, `01`, `007.50`, `1.e5` are read | open: #238 |
| FP-10 | A decimal of 16 places or more moves the wrong distance for its boundaries: the half unit of its last place was a division rounded to 16 places, so `(1.00000000000000000).lowBoundary()` was `1.00000000` (both references `0.99999999`) | #244, plan 013 |
| FP-11 | `(-0.0).toString()` is `-0.0`; the HL7 validator writes `0.0` and fhirpath.js `0`. Its boundaries are right (`-0.05000000`, `0.05000000`) | found in the battery of #244; to be measured against the text before deciding |
| FP-12 | A Quantity quotient drops its trailing zeros, so it loses the places a boundary reads: `(2.0 'mg' / 2)` is `1 'mg'` and its `lowBoundary()` `0.50000000 'mg'`; `(2.0 / 2).lowBoundary()` is `0.99999999` | review of #244 (Fable), round 1 | open |
| FP-13 | `precision()` of a Quantity is empty; FHIRPath's development build defines it (`(1.00 'mg').precision() // 2`), the published 3.0.0 excludes it, and neither reference implements it | open: #249 |
| FP-14 | `toString()` of a decimal written with an exponent writes it, `1.5e2`, which is no Decimal string `toDecimal()` reads back (`'1.5e2'.toDecimal()` empty) | found in the review of plan 014 (Fable); open |
| FP-15 | A quotient's `precision()` is the division's 16 places: `(1.0 / 4).precision()` is 16, the HL7 validator answers 2; its boundaries take the same places | review of #246 (probe); open |
| FP-16 | `Observation.value.as(System.Quantity)` keeps a `valueQuantity` and `is System.Quantity` is true; fhirpath.js and the HL7 validator both answer empty and false | battery `primitive-type-operators` (#241); open |
| FP-17 | `Patient.birthDate.ofType(Date)` and `ofType(System.Date)` keep the FHIR date; fhirpath.js answers empty, as it keeps `valueDateTime` for `ofType(DateTime)`, and the HL7 validator errs | battery `primitive-type-operators` (#241); resolved by plan 017 with a model and for a choice: date maps to DateTime, so `ofType(System.Date)` drops it and `ofType(System.DateTime)` keeps it; with no model a plain `birthDate` is answered by its System name (Date), by D6 |
| FP-18 | `types.IsAbstractResourceType` (`types/object.go`) holds one list across versions (Resource, DomainResource, CanonicalResource, MetadataResource; the last two exist only in R5), with no ADR; the specification gives the set per version in `StructureDefinition.abstract` (R4B adds DataType, R5 five more). Reproduce: read `types/object.go` | found in plan 019 (#257); open: out of scope there, `Analyze` takes the answer from the model (`AbstractTypes`) |
| FP-19 | At mixed precisions across written offsets, this engine compares start instants: `@2020-01-01T10+05:30 = @2020-01-01T04:45Z` is false and `< ` true, where the text converts to a common offset and compares precision by precision (empty), CQL's Java and cql-execution answer null, fhirpath.js answers `=` empty and `<` false, and Java errs on `=` | battery `difftest/batteries/cross-offset-precision`; plan 020 §1 | open |
| FP-20 | Equal values written at and after a leap second compare differently at a coarse precision: `2016-12-31T23:59:60-03:00 = 2017-01-01T00:00:00-03:00` is true, yet a Date `@2016-12-31` compares unknown with the first and before the second, so `min()` over them depends on the order. The leap second is read within its written day at coarse precisions and as the next second by `Equal` | plan 020, code review round 1 | open |
| FP-21 | `=` between a calendar duration in years and one in months is not symmetric: `0.0833333333333333 year = 1 month` is true and `1 month = 0.0833333333333333 year` false. One direction converts years to months and the other months to years, and the 16-place rounding (`types/decimal.go:44`, `types/quantity.go:627`) lands on either side of the exact value. Reproduce: evaluate both expressions | plan 022, adversarial round 1 | open |
| FP-22 | `=` between objects is not symmetric for a member named `"_"` against one named `""`: `{"_":{}} = {"":"x","_":{}}` is true and the reverse false. `childName` treats `"_"` as a plain member, while `equalObjects` and `equalNamed` look up `"_"+name` and read it as the element of `""`. Reproduce: `extension[0] = extension[1]` and the reverse over `{"resourceType":"Basic","extension":[{"url":"u","_":{}},{"url":"u","":"x","_":{}}]}` | plan 022, fuzz (`FuzzEqualityKeyAgreesWithEqual`) | open: #265; 022 gives such objects no key |
