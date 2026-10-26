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

## HL7 Java FHIRPath engine (validator 6.10.2)

Measured with `make three-engine`, the pinned validator jar.

| # | Finding | Reproduce | Status |
|---|---|---|---|
| JAVA-1 | A FHIR `instant` short of the second or with no offset (`2020`, `2020-01-15`, `2020-01-15T10:30:00`) is read as a DateTime; FHIR: "SHALL specified at least to the second and SHALL include a time zone" | battery `fhir-temporal-form`, `instant-*` | decided: this engine follows the text and fhirpath.js; CONFORMANCE.md, "Measured, and not followed" |
| JAVA-2 | A FHIR `date` with a time, `2020-01-15T10:30:00Z`, is read as a DateTime; FHIR: "YYYY, YYYY-MM, or YYYY-MM-DD" | `date-27.json` | decided, as JAVA-1 |
| JAVA-3 | A FHIR `time` holding a date, `2020-01-15`, answers `> @T09` true and `> @2019` an error: a time, and not one | `time-33.json`, and the probe `> @2019` | decided, as JAVA-1 |
| JAVA-4 | A FHIR `time` with a leading `T`, `T10:30:00`, answers `> @T09` false | `time-32.json` | decided, as JS-4 |
| JAVA-5 | A `dateTime` with a time and no offset is placed in the JVM's local offset: `2020-01-15T10:30:00 + 1 year` gives `2021-01-15T10:30:00-03:00` on a machine in Chile, so the answer depends on where it runs. FHIRPath calls a default offset "a policy decision" | probe battery of #231 | open; this engine leaves the offset to its caller (`WithDefaultOffset`) |

## The FHIR specification (HL7)

| # | Finding | Evidence | Status |
|---|---|---|---|
| SPEC-1 | R5's `dateTime` regex (`StructureDefinition-dateTime`, `dateTime.value`) accepts a sign with no offset, `2020-01-15T10:30:00+`, and an offset after a month or a day, `2020-01Z`; R4 and R4B's do not. R5's datatypes page calls the regexes "informative, not normative" | pinned `hl7.fhir.r5.core` 5.0.0 | open (the user chose not to file it yet); to be checked against the current build before drafting |
| SPEC-2 | R5's `dateTime` definition comments "UTC offset is allowed for dates and partial dates"; R4 and R4B do not say so, and neither reference engine reads `2020-01-15Z` | pinned core packages; battery `fhir-temporal-form` | decided: refused, as both engines do |
| SPEC-3 | The text requires an offset on a `dateTime` with a time and the seconds on a `time`; both reference engines read values without them, and none occurs in the R4, R4B or R5 official examples | `datatypes.html`; scan of the pinned example packages | decided: read, as both engines do (user, 2026-10-26) |

## gofhir/searchindex

| # | Finding | Evidence | Status |
|---|---|---|---|
| SI-1 | Adopting the release with #231: `TestDateFormsInvalidForTheirType` indexes, with an "invalid form" warning, values outside their type's form (a dateTime without seconds, an instant without a time or an offset, a date with a time). fhirpath now hands them over as strings, so they go unindexed; to keep indexing them searchindex reads the string with its own parser | its tests run against this tree through `go.work` | open, for searchindex to adapt (user, 2026-10-26) |
| SI-2 | `TestFhirpathInvalidFormsKnownLoss` fails as designed once #231 is in: the five values come out of the known loss | as SI-1 | open, as SI-1 |

## gofhir/validator

| # | Finding | Evidence | Status |
|---|---|---|---|
| VAL-1 | Its tests pass alike against main and against #231 | `go test ./...` through `go.work` | nothing to do |

## gofhir/fhirpath (this engine)

| # | Finding | Status |
|---|---|---|
| FP-1 | A FHIR temporal value outside its form was read by the FHIRPath literal grammar, or by the untyped guess (`birthDate` with a time as a DateTime) | fixed by #231 |
| FP-2 | A FHIR decimal beyond `DecimalDigits` is read as absent, with no error: `NewDecimal`'s `ErrDecimalRange` is dropped in `jsonValueToFHIRValueWithType` (`types/object.go`, the decimal branch) | open: #232 |
| FP-3 | Without a model, a choice variant can read differently by route, which 1.16.6 already did: `valueInstant: "2020"` is a DateTime by `.value` and a Date by `.valueInstant`; `valueTime: "10:30"` a Time and a String | open; pinned by `TestWithoutAModelEveryRouteReadsAValueAsBefore` |
| FP-4 | The error for a value read as a string under a temporal FHIR type names the FHIR type: `cannot apply '+' to dateTime and Quantity` | open (review of #231, round 1) |
