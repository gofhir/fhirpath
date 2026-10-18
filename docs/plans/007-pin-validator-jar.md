# 007 — Three-engine runs measure a pinned HL7 validator

Status: done, pending commit
Issue: #223

## Problem

`make three-engine` runs HL7's Java FHIRPath engine through whatever jar
`HL7_VALIDATOR_JAR` or `-java` names, and prints only its file name. The
answers it gives are cited as evidence (CONFORMANCE.md's "Measured, and not
followed", the plans of #219 and #214), but nothing says which validator gave
them, so they are reproducible only by someone holding the same jar. The
charter (§10) asks for every data source pinned.

## Prior review

- HL7 publishes the validator as a GitHub release asset of
  `hapifhir/org.hl7.fhir.core`, at a stable URL per version:
  `https://github.com/hapifhir/org.hl7.fhir.core/releases/download/6.10.2/validator_cli.jar`
  (187,697,077 bytes). GitHub records the asset's digest:
  `sha256:a3addadfa18dfa23146a0a243b6ede68eaad92157a5407738c468bb3d7e4ccd6`,
  the same as the jar every recent run used (`~/tools/validator_cli.jar`,
  "FHIR Validation tool Version 6.10.2 (Git# d06577dbc5c6)").
- #221 pinned the FHIR packages the same way: a pin beside the name, the
  download hashed and refused on a mismatch.
- `gofhir/server`'s Makefile downloads the validator from the same release
  URL by version (`HAPI_VALIDATOR_VERSION`, into `.cache/`) with `curl`, and
  checks no hash. Taken: the versioned release URL and a cached file named by
  version; improved: the digest checked. `go-fhir-mutator` checks SHA-256 for
  the packages it installs, not for the jar.

## Approach

- `difftest` pins the validator in one place (a `var`, which tests swap): version, URL, size and
  SHA-256, with a comment naming GitHub's recorded digest as the cross-check
  it was set from. A maintainer replacing the asset under the same name would
  change its bytes, and the pin refuses them.
- **The verified bytes are the ones that run.** Before anything is
  evaluated, the jar is copied, through the file opened once, into a private
  per-battery directory (`os.MkdirTemp`, the copy read-only), hashed as it is
  copied (0.4 s for 188 MB, measured), and checked against the pin's size and
  digest. Every JVM of the battery, retries included, is launched from that
  snapshot, removed at the end. Replacing, editing or retargeting the
  original (a symlink) after the check changes nothing measured. A malicious
  process running as the same user is out of scope.
- A jar that is not the pinned validator is refused before fhirpath.js or
  anything else runs, naming the version expected, both digests and `make
  validator-jar`.
- **Unpinned runs, on purpose.** `-java-unpinned sha256:<digest>` runs a jar
  that is not the pinned one, as checking a newer validator before moving the
  pin needs, provided it has the digest given (a typo cannot accept an
  unknown jar). Every line that carries a Java answer, the summary and each
  group heading are marked `[UNPINNED sha256:<first 12>]`, and the header
  names the digest measured and the pin; no version that was not verified is
  printed. Results from such runs are not citable as the pinned engine's.
- **The header names every source:** the validator's verified version and
  digest, fhirpath.js's version (`npm ci` already checks the lock's sha512),
  the JVM (`java -version`, recorded, not pinned) and this engine's revision.
  The revision is read by difftest itself (`git -C .. describe --always
  --dirty`), so direct `go run .` gets it too; without git it prints
  `revision unknown`; a dirty revision says the run needs the working tree's
  patch to be reproduced.
- **The fetch.** `difftest -fetch-validator DIR` installs the pinned release
  as `DIR/validator_cli-6.10.2.jar`, and needs no Java:
  - a file already there that verifies is kept; one that does not is refused
    and kept, the error saying to remove it;
  - otherwise the download (HTTPS, redirects followed up to a bound, as
    GitHub sends assets to `objects.githubusercontent.com`) goes to a
    temporary file in DIR: a known `Content-Length` other than the pinned
    size is refused at once, an absent one accepted; the body is read
    through `LimitReader(size+1)` and must be exactly `size` bytes with the
    pinned digest;
  - the whole request runs under one generous deadline (30 min, injectable
    for tests), simpler than an idle timer and enough for slow links;
  - the file is synced and closed, then published without overwriting: if a
    destination appeared meanwhile, it is verified and kept when it is the
    pinned one, and the temporary file is removed; nothing is ever left
    behind on failure.
- `make validator-jar` runs it into `build/validator/` (removed by `make
  clean`, like the corpus packages). `make three-engine` uses
  `HL7_VALIDATOR_JAR` if set (missing file: an error, not a fallback), else
  `build/validator/validator_cli-6.10.2.jar` if present (resolved from the
  directory the recipe runs in), else runs against fhirpath.js only, as
  today, its summary saying so and how to fetch the jar.
- `make check` runs `difftest`'s offline tests (those that need neither
  Node, Java nor the network), so the new ones gate changes.
- Moving to another validator version is one change: the pin, and a note in
  CONFORMANCE.md that the measured answers were rechecked. CONFORMANCE.md's
  citations of the Java engine name 6.10.2.

## Tests (written before the change)

Offline, in `difftest`, with fake jars and their own pins, the evaluators
injected so a test can count their calls:

1. Snapshot verification: the pinned bytes are accepted; one byte changed,
   another size, a zero-length file, a directory and a missing file are
   refused, the error naming the version and both digests; a symlink is
   followed and its target's bytes are what is checked.
2. The snapshot is what runs: a fake `java` on `PATH` records the digest of
   the jar it is given; the original is modified after verification; every
   recorded digest is the pinned one.
3. A jar that does not verify fails with zero evaluator calls; with
   `-java-unpinned sha256:<its digest>` it runs, and every output line that
   carries a Java answer is marked; with a wrong digest it is refused.
4. The fetch, against httptest servers: installs the pinned bytes; refuses
   altered bytes, a wrong `Content-Length`, one extra byte, a truncated body,
   a non-200, a deadline passed (short injected deadline), leaving no file;
   accepts a chunked response with no length; follows a redirect; keeps a
   verified file already there without a request; refuses and keeps a wrong
   one; two concurrent installers both succeed and leave one verified file.
5. The pin is well formed: 64 lowercase hex digits, an `https` URL naming
   the version, a size above zero.

## Validation

- `make three-engine` with `~/tools/validator_cli.jar` runs; its header names
  validator 6.10.2 and the engine's revision. A copy with one byte changed is
  refused before any case runs; with `-java-unpinned` it runs, marked.
- `make validator-jar` downloads the release once and verifies it; run again,
  it keeps it.

## Exit criteria

1. Tests fail before the change and pass after.
2. The validation above.
3. `make check` passes.

## Adversarial round

Independent reviewer; the hash cost measured (0.4 s).

| Finding | Resolution |
| --- | --- |
| The digest pin holds; hashing is cheap | Accepted: hash every run, no cache; the pin is ours, GitHub's digest a cross-check |
| Refusing every other jar blocks checking a newer validator | Accepted: `-java-unpinned`, with runs marked UNPINNED and no unverified version printed |
| Fetching on first use changes `make three-engine` | Accepted: the fetch is `make validator-jar`; three-engine uses the jar if present, else fhirpath.js only with a hint; Java checked before any download |
| `fetchValidator` safeguards unnamed | Accepted: versioned name, verify-or-refuse an existing file, temp file and link, exact size as cap, stall timeout, cleanup |
| Test 1 impossible with the real pin | Accepted: `verifyJar(path, pin, size)` tested with a fake jar; the real pin checked under Validation; verification before any evaluation |
| The header names no engine revision | Accepted: `git describe --always --dirty` passed by the Makefile |

### Second round (Codex, requested by the user)

Run with `codex exec` (read-only), independently of the first round. A first
attempt through an agent did not reach Codex; its own review agreed with
Codex on findings 1, 2, 4–7 and is folded in with it. Two claims checked in
the code: the existing missing-jar test accepts any error
(`difftest/battery_test.go:52`), and `make check` does not run difftest.

| Finding | Resolution |
| --- | --- |
| High: hashing once does not bind the bytes the many `java -jar` launches read (replacement, edits, symlink retargeting) | Accepted: a private read-only snapshot, hashed as copied, launched every time |
| UNPINNED marks only the header; excerpts lose it | Accepted: every Java answer line, summary and heading marked; the flag takes the expected digest |
| Rename does not guarantee no-clobber; Windows and open handles | Accepted: sync, close, publish without overwriting, verify a competing destination |
| "Capped" is ambiguous; missing `Content-Length` | Accepted: `LimitReader(size+1)`, exactly `size`, absent length accepted, known wrong length refused |
| Stall and redirect behaviour underspecified | Accepted: bounded redirects over HTTPS; one generous injectable deadline for the whole request rather than an idle timer |
| Revision only from the Makefile; git absent; dirty | Accepted: difftest reads it itself, `revision unknown`, dirty runs need the patch |
| Tests can pass for the wrong reason, and escape CI | Accepted: injected evaluators with call counts; offline difftest tests in `make check` |
| Default path from the recipe's directory; JVM version; fetch without Java | Accepted: tested resolution; `java -version` recorded; the fetch needs no Java |

## Record

- Approved by the user after the second (Codex) round.
- Tests first (`difftest/validator_test.go`): they did not compile against
  the old code (`jarPin`, `prepareJar`, `fetchValidator`, the injectable
  engines). One test caught an unmarked header line, because its temporary
  directory's name contains "Java": the header now carries the marker too,
  so an excerpt of it keeps it.
- Implementation: `difftest/validator.go` (pin, snapshot, fetch, revision,
  JVM); `runBattery` checks the jar before anything runs and launches every
  JVM from the snapshot; `-java-unpinned sha256:<hex>`; `-fetch-validator`;
  `make validator-jar`; `make three-engine` uses the installed jar when
  `HL7_VALIDATOR_JAR` is unset; `make check` runs difftest's tests, which
  no longer need Node.
- The engine revision uses `git describe --tags`: release-please's tags are
  lightweight, and without `--tags` the header read `v1.0.0-217-…` for
  `v1.16.6`.
- difftest is not linted by `make lint` (it was not before either); the
  findings golangci-lint reports in the new code were fixed, except two
  `sloppyReassign` the named result needs and the jar path a CLI is given.
- Validation with the real jar: `HL7_VALIDATOR_JAR=~/tools/validator_cli.jar
  make three-engine` names "HL7 validator 6.10.2 (sha256 a3addadf…)", the
  revision and the JVM; a copy with one byte changed is refused in 1.2 s,
  before any case runs; `make validator-jar` downloaded the release (188 MB)
  and verified it, and run again kept it in 0.5 s; without
  `HL7_VALIDATOR_JAR`, `make three-engine` used the installed jar.

### Code review

| Round | Findings | Resolution |
| --- | --- | --- |
| 1 | No high. Medium: safeguards untested (Rename for Link, the https-downgrade check, the redirect cap, the announced-length check, a bare digest without `sha256:`, the snapshot's chmod, the snapshot outside the battery's directory all survived mutation); an interrupt leaked the 188 MB snapshot and a download's temporary file. Low: the missing-jar test still accepted any error; `-java-unpinned` without a jar was ignored; the second header line's marker untested; no fallback without hard links; a doc comment displaced; `java -version`'s first line can be "Picked up …"; the plan said `const` | Tests for each, every listed mutation now failing; an interrupt runs registered cleanups (the battery's directory, the download) before exiting; fetch removes downloads left over a day ago; the missing-jar test stubs Java and asserts the jar is named; `-java-unpinned` without a jar is an error; every header line asserted marked; an exclusive-create copy where links fail; doc comment moved; the JVM line is the one naming a version; plan corrected. Interrupt checked by hand with the real jar: the battery's directory and snapshot are gone after SIGINT |
| 2 | Medium: the download's interrupt cleanup was registered only after the transfer, so a Ctrl-C during the 188 MB left its temporary file. Low: the exclusive-copy fallback made a racing installer see a partial file and fail falsely (reproduced: 3 of 6), and was untested; the redirect bound unpinned (40 passed); a done() in progress could be cut short by exit; a second signal was swallowed; SIGTERM exited 130; the test swapped `http.DefaultTransport` | The temporary file is registered when created; an interrupt mid-download test (fails with the late registration); the fallback dropped, the error naming the filesystem's lack of hard links, with a test; the hops counted (at most 5); done() cleans under the lock; `signal.Stop` after the first signal; SIGTERM exits 143; an injectable `transport`. The first version returned a nil release on error paths (named result), caught by the existing tests' panic |
| 3 | No medium or higher (converged); each round-2 fix re-verified by mutation, no deadlock, the temporary file removed exactly once. Low: the signal handler's details (the lock in done(), `signal.Stop`, exit 143) are not unit-tested, since it exits the process; a SIGTERM sent to difftest alone leaves running JVMs to finish their case on the unlinked snapshot (a terminal's Ctrl-C reaches the whole process group); a stale "rename" in the adversarial table | Recorded: the handler was checked by hand (SIGINT to a running battery removed its directory and snapshot; the JVM finished its case and exited); the orphaned JVMs documented as such; the table corrected. Converged in round 3 |
