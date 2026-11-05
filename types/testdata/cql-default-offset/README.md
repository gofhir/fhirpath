# fhirpath#256 probes at four evaluation offsets: Java 5.4.0 and cql-execution 3.3.2

Each run evaluates at 2026-01-01T00:00:00 at the offset named. Each cell is `Java / cql-execution`, or a single value when the two agree.

| # | CQL | -03:00 | +14:00 | -14:00 | +05:30 |
|---|---|---|---|---|---|
| 1 | `@2012-01-01T < @2012-01-01T05:00:00.000Z` | null | null | false | null |
| 2 | `@2012-01-01T = @2012-01-01T01:00:00.000Z` | false | null | false | null |
| 3 | `@2012-01-01T = @2012-01-02T02:00:00.000Z` | null | false | null | false |
| 4 | `@2012-01-01T > @2011-12-31T23:00:00.000Z` | true | null | true | null |
| 5 | `@2012-01T = @2012-02-01T02:00:00.000Z` | null | false | null | false |
| 6 | `@2012T = @2013-01-01T02:00:00.000Z` | null | false | null | false |
| 7 | `@2012-01-01T = @2012-01-01T` | true | true | true | true |
| 8 | `@2012-01-01T = @2012-01-01T10:00:00.000` | null | null | null | null |
| 9 | `@2020-01-01T10+05:30 < @2020-01-01T04:45Z` | null | null | null | null |
| 9b | `@2020-01-01T10:00+05:30 < @2020-01-01T04:45Z` | true | true | true | true |
| 10 | `@2020-01-01T10:00+05:30 = @2020-01-01T05:15Z` | false | false | false | false |
| 11 | `days between @2012-01-01T and @2012-01-03T02:00:00.000Z` | 2 / Uncertainty [0, 1] | 2 / Uncertainty [1, 2] | 2 / Uncertainty [0, 1] | 2 / Uncertainty [1, 2] |
| 12 | `hours between @2012-01-01T and @2012-01-02T05:00:00.000Z` | Interval [3, 26] / Uncertainty [2, 26] | Interval [20, 43] / Uncertainty [19, 43] | Interval [-8, 15] / Uncertainty [-8, 15] | Interval [11, 34] / Uncertainty [10, 34] |
| 13 | `@2012-01-01T same day as @2012-01-01T01:00:00.000Z` | true | true | true | true |
| 14 | `ToDateTime(@2012-01-01) < @2012-01-01T05:00:00.000Z` | null | null | false | null |
| 15 | `@2016-12-31T = @2017-01-01T02:59:59.999Z` | null | false | null | false |
| 16 | `@2012T < @2013-01-01T00:00:00.000Z` | null | true | null | true |
| 17 | `@2012-12-31T < @2013-01-01T10:00:00.000Z` | true | true | null | true |
| 18 | `@2012-01-01T = DateTime(2012, 1, 1, 0, 0, 0, 0, 14.0)` | false | null | false | false |
| 19 | `@2012-01-01T = DateTime(2012, 1, 1, 23, 0, 0, 0, -14.0)` | false | false | null | false |
| 20 | `days between @2012-01-01T and @2012-01-03T02:00:00.000Z` | 2 / Uncertainty [0, 1] | 2 / Uncertainty [1, 2] | 2 / Uncertainty [0, 1] | 2 / Uncertainty [1, 2] |
| 21 | `weeks between @2012-01-01T and @2012-01-08T02:00:00.000Z` | 1 / 0 | 1 / Uncertainty [0, 1] | 1 / 0 | 1 / Uncertainty [0, 1] |

## Notes

- **Rerun at -03:00:** rows 1-14 give the same answers as the first run (identical).
- **Where Java and cql-execution disagree:** 11, 12, 20, 21. Every one is a duration row, and a cell for one of them shows `Java / cql-execution`.
- **How the offsets were set:** the only change from go-cql's `internal/referenceharness` is the `EVALUATION` constant in `Evaluate.java` and `evaluate.js`, made in copies under `h<offset>/` beside this file.
- **Raw outputs:** `java<k>.json` and `js<k>.json`, where `k` is m0300, p1400, m1400 or p0530.
