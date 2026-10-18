# 004 — corpusdiff refuses packages whose bytes are not the pinned ones

Status: done, pending commit
Issue: #221

## Problem

`corpusdiff`'s `fetch` (`corpusdiff/main.go:164`) downloads six FHIR packages
from `https://packages2.fhir.org/packages/<name>/<version>` and extracts
whatever comes back; a cached copy is used as found. Every corpus comparison
(`make corpusdiff`, `make bench-corpus`, the corpusdiff tests) is evidence only if everyone measures the same bytes. The
charter (§10) requires each data source pinned by SHA-256.

## Prior review

- `gofhir/server` (`internal/packages/downloader.go`) checks no hash; it caps a
  download at 500 MB and bounds extraction against zip bombs. Taken: a size
  cap on the download. Not needed: zip-bomb bounds, since nothing is
  extracted before its hash matches the pinned one, and `extract` already
  writes only `package/*.json` at one level.
- `scripts/spec-sources.py` pins the spec texts in `SPEC_SOURCES` and refuses
  a mismatch: the same rule, applied here.
- Measured: the tarballs are byte-for-byte stable — two downloads from
  packages2.fhir.org of `hl7.fhir.r4b.core` 4.3.0 give the same SHA-256 —
  and each of the six extracts to exactly the
  files in the current `build/corpusdiff/cache`, so the pins describe what
  every measurement so far was made on. The tarballs are stored artifacts,
  not built per request: the gzip headers carry the R4 publication time
  (2019-12-03) or zero, never the time they were served.

| Package | SHA-256 | Bytes |
| --- | --- | --- |
| `hl7.fhir.r4.core` 4.0.1 | `b090bf929e1f665cf2c91583720849695bc38d2892a7c5037c56cb00817fb091` | 4,531,911 |
| `hl7.fhir.r4.examples` 4.0.1 | `02aa10ea545301b1e14277123dd43a2a3a9297e6345de677801866df8afe7270` | 19,165,410 |
| `hl7.fhir.r4b.core` 4.3.0 | `56ade6affa774053e8e514d480944568705d93d9252e264888ee224907fe40e7` | 14,379,163 |
| `hl7.fhir.r4b.examples` 4.3.0 | `a64db561a943eab454d85349cfb5a088cbfc57c3d90706db36ae3df64269afa5` | 15,170,171 |
| `hl7.fhir.r5.core` 5.0.0 | `74b27cd1bfce9e80eaceac431edf230b0945a443564fbf5512f82e5fa50a80d4` | 17,057,450 |
| `hl7.fhir.r5.examples` 5.0.0 | `129eb96eaeab3b14b4c3a89a2e3593a0a79ebddb56ef46e15fac6928a0a5976e` | 17,839,488 |

## Approach

- Each package's pin sits beside its name in `corpusdiff/main.go`, in a
  table from package to SHA-256 that `fetch` reads; `versions` and `fetch`'s
  callers do not change.
- The work moves into `fetchFrom(base, cache, pkg, pin, limit)`; `fetch` looks up
  the pin (refusing a package with none) and calls it with the registry's
  base URL. Tests call `fetchFrom` with a test server and their own pin, so
  no package-level URL is mutated.
- The registry stays `https://packages2.fhir.org/packages/<name>/<version>`.
  packages.fhir.org, the one the charter names, does not serve the same R4
  packages: its `hl7.fhir.r4.core` and `hl7.fhir.r4.examples` 4.0.1 are
  re-packed (other gzip, other SHA-256) and differ in content — three more
  StructureDefinitions (`structuredefinition-json-type`, `-rdf-type`,
  `-xml-type`), another `.index.json`, another `package.json` in the
  examples, no `openapi/` in the core. R4B and R5 are identical in both.
  Every measurement so far was made on packages2's bytes; switching would
  change what is measured, so it is not done here.
- `fetchFrom` downloads to a temporary file (`os.CreateTemp` in the cache),
  hashing as it writes, capped at 200 MB (`io.LimitReader(cap+1)`, ten
  times the largest package), within the existing 10-minute timeout. A hash
  that is not the pin is refused with both digests in the error, and
  nothing is extracted.
- Extraction goes to a temporary directory (`os.MkdirTemp` in the cache),
  the digest is written inside it as `sha256`, and only then is it renamed
  into place, under a name that carries the start of the digest
  (`hl7.fhir.r4.core#4.0.1@b090bf929e1f665c`): a directory in place always
  holds its marker and its files, and whatever sits at that name is this
  very package. Of several processes fetching it at once, the first rename
  wins and the others find it in place. A directory at that name that is
  damaged (no marker, or another digest), which only editing the cache by
  hand produces, is reported and not removed, since another process may be
  reading it: nothing in the cache is removed by a fetch. Temporaries are
  removed on every path; a refused fetch leaves the cache as it was.
- A cached package is used only when its `sha256` file holds the pin. The
  directories of a cache filled before this change, named without the
  digest, are no longer read; the packages are fetched once into the new
  names, and the old directories can be removed by hand or `make clean`, as
  can the temporaries a run killed mid-fetch leaves (`.fetch-*`,
  `.download-*`, and the old `.partial` ones).
- Out of scope: files edited by hand inside the cache after a valid fetch.

Behaviour change: `make check` runs corpusdiff's tests without `-short`, and
`TestCoreConstraintsAnalyze` fetches the core packages. After this change
the first run refetches them once (about 40 MB); offline, with an old cache,
that test skips where it used to run.

Other data sources the charter counts: the FHIRPath suite inputs (git), the
spec texts (`SPEC_SOURCES`) and fhirpath.js (`package-lock.json`, `npm ci`)
are pinned. Not pinned: the HL7 validator jar the three-engine runs use
(`HL7_VALIDATOR_JAR`, whatever the user has), reported as its own issue; the
ANTLR jar the Makefile downloads, a build tool rather than evidence,
recorded here. bench-compare's worktree does not run corpusdiff.

## Tests (written before the change)

In `corpusdiff`, offline against an `httptest` server and a temporary cache,
calling `fetchFrom` (they do not compile before it exists):

1. A tarball whose hash is the given pin is extracted, its digest recorded
   inside the package directory, and no temporary file or directory left.
2. A tarball one byte different is refused; the error names both digests;
   nothing is left in the cache.
3. A truncated download, and a non-200 response, are refused, leaving
   nothing.
4. A download larger than the cap is refused, leaving nothing (the cap a
   parameter the test sets small).
5. A cached package with `package.json` but no marker, and one with another
   digest, are fetched again; one with the pin is not (the server counts
   requests). A refused refetch leaves the existing directory in place.
6. Every package `versions` names has a pin: 64 lowercase hex digits, the
   six distinct.

## Validation with real data

- With the real cache moved aside, `TestCoreConstraintsAnalyze` refetches
  the core packages, accepts them and passes, and `make corpusdiff
  BASE=origin/main` fetches the example packages and reports no
  difference.
- Mutation: change one hex digit of a pin; the fetch is refused and the
  corpus tests fail with the refusal (`errNotPinned`), not skip.

## Exit criteria

1. Tests 1–6 fail before the change (they do not compile against today's
   `fetch`) and pass after.
2. A clean fetch of the six real packages succeeds and the corpus tests and
   `make corpusdiff` pass on it.
3. `make check` passes.

## Adversarial round

Independent reviewer; finding 5 verified (`make check` runs corpusdiff
without `-short` on `main`).

| Finding | Resolution |
| --- | --- |
| Pins correct; tarballs are stored artifacts (gzip mtimes) | Accepted: evidence added; tarball pin kept over a content hash, which would need a defined manifest and about 1 s per run for no observed risk |
| Test 1 impossible offline with "callers unchanged" | Accepted: `fetchFrom(base, cache, pkg, pin)`, pins in their own table, no mutable URL |
| Marker written after extraction; fixed `.partial` name races between processes | Accepted: marker inside the temporary directory before the rename; `MkdirTemp`/`CreateTemp`; cleanup on every path |
| `make check` does not pass `-short`: core tests already fetch; one refetch after merge; offline skip | Accepted, verified: recorded as a behaviour change |
| Missing tests: truncated, non-200, leftovers, marker-less cache; test 5 weak | Accepted: tests 1–6 |
| Code uses packages2, the charter names packages.fhir.org | Rejected after measuring all six: packages.fhir.org serves re-packed R4 4.0.1 packages with different content (three more StructureDefinitions); R4B and R5 match. packages2 kept, the source of every measurement so far |
| Validator jar and ANTLR jar unpinned | Accepted: validator jar reported as its own issue; ANTLR jar recorded |
| Other callers, cap, timeout | Confirmed fine |

## Record

- Approved by the user, packages2.fhir.org kept.
- Tests first (`corpusdiff/fetch_test.go`): they did not compile against
  `fetch`. On the first run three passed for the wrong reason — the test
  server's URL lacked its trailing slash, so every fetch failed parsing it —
  and the tests were tightened to require the reason of each refusal
  (`404`, `is not the pinned`, `larger than`).
- Added on the way: a download that is not the pinned bytes fails with
  `errNotPinned`, and the corpus tests and `BenchmarkCorpus` fail on it
  instead of skipping, as they still do when the network fails; a pin
  mutated by one digit is refused with both digests and leaves nothing.
- Real data: with the old cache moved aside, the six packages were fetched,
  verified and recorded; `TestCoreConstraintsAnalyze` passes, and
  `make corpusdiff BASE=origin/main` reports 0 answers differing over
  521,007 (R4), 308,577 (R4B) and 360,675 (R5) evaluations.
- The HL7 validator jar the three-engine runs use (locally validator 6.10.2,
  SHA-256 `a3addadf…d7e4ccd6`) is not pinned: reported as its own issue.

### Code review

| Round | Findings | Resolution |
| --- | --- | --- |
| 1 | Medium: four goroutines fetching one package into a fresh cache failed 327 of 800 times (`rename … directory not empty`), though the cache ended valid. Low: the temporary directory's cleanup and a pinned tarball without resources untested; `fetchedOrSkip` untested; `extract`'s doc comment deleted by accident; the plan named a target this branch lacks, a test that does not fetch, a four-argument signature and a skip where the code fails | A directory holding the pinned package is never removed and the install is retried until the package is in place, whoever put it there; `TestConcurrentFetchesOfOnePackageAllSucceed` (failed before the fix, which also took a second attempt: re-checking only after a failed rename still lost one round in 50); tests for a pinned tarball without JSON and for `fetchedOrSkip`; doc comment restored; plan corrected. The marker-after-rename mutation passes every test, as a crash-window property: recorded, not tested |
| 2 | Medium: a fetcher could remove a package another had just installed and returned (the check and the removal are not one step): in a stress run, 17 readers saw their files vanish though the cache ended valid. Low: the concurrency test did not catch dropping the retry loop; `TestFetchedOrSkip` relied on a zero-value `testing.T` | The cache directory carries the start of the digest, so what sits at the name is the same package; the loop renames and, only for a damaged directory, removes and retries. The concurrency test runs 100 rounds of 12 fetchers and checks each returned directory's files stay; it catches the round-2 race at once (mutation: remove before rename). `fetchedOrSkip` takes an interface and is tested with a recorder |
| 3 | No medium or higher (converged). Low: with a directory damaged by hand at the pinned name, one fetcher's removal could still delete the valid copy another had installed and returned (4 readers in one of 300 stress rounds), so the comment and plan overstated the guarantee; leftovers of killed runs never swept | A damaged directory is reported ("remove it") and never removed, so no fetch removes anything from the cache and the retry loop is gone; tests pin the report and that the directory stays. Leftovers documented with `make clean`. Converged in round 3 |
