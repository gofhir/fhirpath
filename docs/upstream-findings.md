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
| JS-8 | A decimal's boundary counts the places of a JavaScript number, not of the text, wherever the value is not a positive literal: `(-1.0).lowBoundary()` is `-1.5` and `(-0.10)` `-0.15` (`(0.0)` gives `-0.05`, rightly), `(1.5 + 0.00000000000000000).lowBoundary()` `1.45`, and a resource's `1.00000000000000000` `0.5`. FHIRPath 3.0.0: "The least possible value of the input to the specified precision" | battery `boundary-places` | open |
| JS-9 | The boundaries stop at 8 places: `(1.587).lowBoundary(9)` is empty (`FP_Decimal.MAX_PRECISION = 8`, src/types.js). The text leaves the maximum to the implementation | battery `boundary-places` | decided, as JAVA-12 |
| JS-10 | Boundaries over a Quantity are not implemented: `(1.5 'mg').lowBoundary()` and a resource's `valueQuantity` err, `Expected a Decimal, Date, DateTime, or Time` | battery `boundary-places` | decided: CONFORMANCE.md, "boundaries over a Quantity" (R5's `rng-2` needs them) |

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

## The FHIR specification (HL7)

| # | Finding | Evidence | Status |
|---|---|---|---|
| SPEC-1 | R5's `dateTime` regex (`StructureDefinition-dateTime`, `dateTime.value`) accepts a sign with no offset, `2020-01-15T10:30:00+`, and an offset after a month or a day, `2020-01Z`; R4 and R4B's do not. R5's datatypes page calls the regexes "informative, not normative" | pinned `hl7.fhir.r5.core` 5.0.0 | open (the user chose not to file it yet); to be checked against the current build before drafting |
| SPEC-2 | R5's `dateTime` definition comments "UTC offset is allowed for dates and partial dates"; R4 and R4B do not say so, and neither reference engine reads `2020-01-15Z` | pinned core packages; battery `fhir-temporal-form` | decided: refused, as both engines do |
| SPEC-4 | The decimal's JSON form is "restricted to the precision limits documented in XML schema for xs:double and xs:decimal" (R4, R4B, R5), so past about 1.8e308 a number in exponent form is no xs:double; yet R5's regex allows a nine-digit exponent and R4's any. Which bounds a FHIR decimal is left unsaid | `datatypes.html`; `StructureDefinition-decimal` regex | open |
| SPEC-3 | The text requires an offset on a `dateTime` with a time and the seconds on a `time`; both reference engines read values without them, and none occurs in the R4, R4B or R5 official examples | `datatypes.html`; scan of the pinned example packages | decided: read, as both engines do (user, 2026-10-26) |
| SPEC-5 | R5's decimal regex (`StructureDefinition-decimal`, `decimal.value`) reads `-?(0|[1-9][0-9]{0,17})(\.[0-9]{1,17})?([eE][+-]?[0-9]{1,9}})?`: a stray `}` before the last `)?`, so the pattern as written requires a literal `}` after the exponent | pinned `hl7.fhir.r5.core` 5.0.0; found in the review of plan 011 (#235) | open |

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

## buger/jsonparser

| # | Finding | Reproduce | Status |
|---|---|---|---|
| JP-1 | A bare `-` as a value is typed as a Number, and read as Integer 0 (`strconv.ParseInt("-")` path) | found in the review of plan 011 (Fable) | open; to be measured before reporting |

## gofhir/fhirpath (this engine)

| # | Finding | Status |
|---|---|---|
| FP-1 | A FHIR temporal value outside its form was read by the FHIRPath literal grammar, or by the untyped guess (`birthDate` with a time as a DateTime) | fixed by #231 |
| FP-2 | A FHIR decimal beyond `DecimalDigits` is read as absent, with no error: `NewDecimal`'s `ErrDecimalRange` is dropped in `jsonValueToFHIRValueWithType` (`types/object.go`, the decimal branch) | fixed by #232: held as written, compared exactly, computing with it empty |
| FP-3 | Without a model, a choice variant can read differently by route, which 1.16.6 already did: `valueInstant: "2020"` is a DateTime by `.value` and a Date by `.valueInstant`; `valueTime: "10:30"` a Time and a String | open; pinned by `TestWithoutAModelEveryRouteReadsAValueAsBefore` |
| FP-5 | `ImplicitPrecision` counts the characters after the point in the text, exponent included: `1.5e2` gives `precision()` 3, `lowBoundary()` 149.99950000 | open: #246 (review of #232; kept out of #244) |
| FP-6 | An exponent past int32 (`1e9999999999`, valid under R4's regex, not R5's) is not read: the decimal library holds it in an int32. fhirpath.js and Java read it | open; pinned by `TestAnExponentPastInt32IsAKnownLoss` |
| FP-4 | The error for a value read as a string under a temporal FHIR type names the FHIR type: `cannot apply '+' to dateTime and Quantity` | open (review of #231, round 1) |
| FP-7 | Under shopspring/decimal v1.5+ a decimal whose exponent passes `MaxDecodeExponent` (10,000) is read as absent again, and v1.6 reads a Unicode minus | fixed by #239 |
| FP-8 | Results depend on shopspring's process-wide `TrimTrailingZeros` (and possibly `UseScientificNotation`): `(10.0 / 4).toString()` is `2.5000000000000000` when another module sets it false | fixed by #245 |
| FP-9 | A resource's decimal is read by the decimal library's grammar, not FHIR's: `-.5`, `5.`, `01`, `007.50`, `1.e5` are read | open: #238 |
| FP-10 | A decimal of 16 places or more moves the wrong distance for its boundaries: the half unit of its last place was a division rounded to 16 places, so `(1.00000000000000000).lowBoundary()` was `1.00000000` (both references `0.99999999`) | #244, plan 013 |
| FP-11 | `(-0.0).toString()` is `-0.0`; the HL7 validator writes `0.0` and fhirpath.js `0`. Its boundaries are right (`-0.05000000`, `0.05000000`) | found in the battery of #244; to be measured against the text before deciding |
| FP-12 | A Quantity quotient drops its trailing zeros, so it loses the places a boundary reads: `(2.0 'mg' / 2)` is `1 'mg'` and its `lowBoundary()` `0.50000000 'mg'`; `(2.0 / 2).lowBoundary()` is `0.99999999` | review of #244 (Fable), round 1 | open |
