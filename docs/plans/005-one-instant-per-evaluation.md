# 005 — now(), today() and timeOfDay() answer one instant per evaluation

Status: done, pending commit
Issue: #225

## Problem

`now()`, `today()` and `timeOfDay()` each read `time.Now()`, so one
expression can see two instants. Measured at `937cea1`, 20,000 evaluations
each: `now() = now()` false 13 times, `timeOfDay() = timeOfDay()` 8,
`timeOfDay() = now().timeOf()` 4, `(1 | 2 | 3).select(now()).distinct().count() = 1`
19. The spec case REQ-DATETIME-035-a fails intermittently in `make check`.

FHIRPath 2.0.0 and 3.0.0, "Current date and time functions": "these operators
should return the same value regardless of how many times they are evaluated
within any given expression (i.e. now() should always return the same
DateTime in a given expression, timeOfDay() should always return the same
Time in a given expression, and today() should always return the same Date
in a given expression.)" 3.0.0's examples: `timeOfDay()` is "Equivalent to
now().timeOf()", `today()` "equivalent to now().toDate()".

Also: two implementations of each are registered (`funcs/utility.go` and
`funcs/temporal.go`). `temporal.go` says it overrides `utility.go`'s
"placeholder functions", but `init` runs in file-name order and
`utility.go`'s are the ones in use (`funcs.Get("now").Fn` is `funcs.fnNow`).

## Prior review

- `gofhir/server` has no implementation of its own.
- fhirpath.js (`src/constants.js`, `src/datetime.js`): `constants.reset()` at
  the start of each evaluation captures `nowDate` once; `now()`, `today()` and
  `timeOfDay()` are derived from it and cached. Taken.
- The HL7 Java engine: not measured here (the property is about repeated
  calls within one evaluation, which a single battery answer cannot show).

## Approach

- The clock belongs to the evaluation, as fhirpath.js resets it per
  evaluation: `Evaluator.EvaluateNode` (and `Evaluate`), which every entry
  point calls once per evaluation, installs a fresh clock on the context it
  evaluates in when the caller has not fixed one, and removes it when the
  evaluation ends. The copies `WithThis`/`WithIndex` make during the
  evaluation carry the same pointer. A zero-value `eval.Context{}` works the
  same way; a context reused across evaluations gets a new instant each time.
  Nothing builds a context in the middle of an evaluation (resolve() reads
  `%rootResource`), so every function in one evaluation sees one clock. An
  expression a custom function evaluates on its own is another expression
  and reads its own instant, as "within any given expression" allows.
- The clock is a pointer to a struct holding the instant and a `sync.Once`
  (a `sync.Once` must not be copied, and contexts are copied by value). The
  first of the three functions to ask reads the time once; all three derive
  their value from that instant in its own location: `now()` the DateTime,
  `today()` its date, `timeOfDay()` its time. Midnight and DST cannot split
  them, since one instant in one location gives all three.
- `Context.SetNow(t time.Time)` gives the context a clock already set to
  `t`, in `t`'s own location (not converted to Local). It affects that
  context and copies made after it, and takes precedence over the
  evaluation's clock. `WithNow(t)` in `options.go` does the same for
  `EvaluateWithOptions`, like `WithTimeout` and the rest. A public API
  addition, documented.
- The time is read through an unexported `nowSource` (`time.Now` by
  default), which tests in `eval` replace with a clock that moves on every
  read, so a function reading the time per call fails deterministically.
- One implementation of each function, in `temporal.go`; the duplicates in
  `utility.go` and their registrations are removed, with temporal.go's
  comment that claims to override them. The output is unchanged: both print
  `2026-10-18T15:01:52.378Z` / `12:01:52.723-03:00`, `HH:MM:SS.mmm`,
  `YYYY-MM-DD`, at millisecond precision (measured under UTC and
  America/Santiago). fhirpath.js prints a UTC offset as `-00:00` where this
  engine prints `Z`, a rendering difference outside this change.

## Tests (written before the change)

In `eval`, with the moving test clock (deterministic):

1. `now() = now()`, `timeOfDay() = timeOfDay()`, `today() = today()`,
   `timeOfDay() = now().timeOf()`, `today() = now().toDate()` are true.
2. Inside `where()`, `select()`, `repeat()`, `aggregate()` and a function
   argument, the instant is the outer one: `(1 | 2 | 3).select(now()).distinct().count() = 1`.
3. Two evaluations on one context see two instants (the clock moved between
   them); on a zero-value `eval.Context{}` too.

With `SetNow`/`WithNow` at a fixed instant with milliseconds and a non-UTC
offset:

4. `now()`, `today()`, `timeOfDay()` give exactly the expected strings, in
   the instant's location; `SetNow` on a copy does not change the parent.

Elsewhere:

5. Concurrency: many goroutines evaluating on one shared `Document`, under
   `-race`.
6. `funcs.Get` for the three names points at the single implementation.
7. As evidence against today's code only, the 20,000-iteration loop on the
   real clock.

## Validation

The spec cases REQ-DATETIME-035 and the datetime area run 200 times under
`-race` without a failure. Corpus comparison (`make corpusdiff BASE=origin/main`)
shows no difference (no official constraint uses the current time, so this
checks nothing else moved).

Oracle mutation: read the time per call again; tests 1 and 2 must fail
every run with the moving clock.

## Exit criteria

1. Tests fail before and pass after.
2. `make check` passes, and the datetime spec cases pass 200 times in a row.
3. CONFORMANCE.md notes the gap closed.

## Adversarial round

Independent reviewer; finding 6 checked (on this machine the 20,000 loop
caught today's defect in 20 of 20 runs, but a coarser clock would miss it).

| Finding | Resolution |
| --- | --- |
| "Per context" is not "per evaluation": a zero-value context has no clock; a reused one would freeze | Accepted: the evaluation installs the clock; zero-value contexts and reuse work |
| An eager clock per context costs an allocation | Accepted: none with the evaluation's clock |
| Output format, precision, offset | Confirmed unchanged; `Z` vs fhirpath.js's `-00:00` noted, out of scope |
| Midnight and DST; location of a fixed instant | Accepted: one instant in its own location; `SetNow` keeps `t`'s location |
| `SetNow` semantics undefined after the first read and on copies | Accepted: it replaces the context's clock with a set one, precedence over the evaluation's; `WithNow` added |
| The iteration test does not discriminate on a coarse clock | Accepted: moving test clock through `nowSource`; the loop kept as evidence only |
| Duplicates referenced nowhere; misleading override comment | Accepted: removed with the comment |

## Record

- Approved by the user.
- Tests first: `eval/clock_test.go` (external package, with `SetNowSource`
  from `export_test.go`) and `funcs/clock_registration_test.go`; they did not
  compile before (`Context.SetNow`, `WithNow`, `nowSource`).
- Implementation: `eval.Context` holds a `*clock`; `startClock`, called by
  `Evaluator.EvaluateNode` and `Evaluate`, installs a fresh one unless
  `SetNow` fixed one, and restores the context's own afterwards, unless
  `SetNow` replaced it during the evaluation. `Context.Now`
  is what `now()`, `today()` and `timeOfDay()` read. The duplicates in
  `utility.go` are gone; the surviving functions are `temporal.go`'s, renamed
  `fnNow`, `fnToday`, `fnTimeOfDay`.
- Mutations (moving clock): reading the time per call fails every case of
  `TestOneEvaluationSeesOneInstant` and the fixed-instant test; not
  resetting the clock between evaluations fails
  `TestEachEvaluationReadsTheTimeAgain`. The first version of the `repeat()`
  case used a projection that never converges under the per-call defect, so
  the mutation hung until the timeout; it was replaced by one that stops
  after one level either way.
- The datetime spec cases pass 200 runs in a row under `-race`; before the
  change REQ-DATETIME-035-a failed in one of eight.
- `make corpusdiff BASE=origin/main`: 0 answers differ over 521,007 (R4),
  308,577 (R4B) and 360,675 (R5) evaluations.

### Code review

| Round | Findings | Resolution |
| --- | --- | --- |
| 1 | Medium-low: a clock fixed by `SetNow` during an evaluation was undone when it ended. Low: two doc comments inaccurate (`WithNow` said the time is read when the evaluation starts; `Context.Now` ignored a fixed instant); a nested expression a custom function evaluates reads its own instant; a context shared across goroutines now races on the clock (sharing was already unsupported); `Evaluate(tree)`'s clock untested | The restore puts the previous clock back only if the evaluation's is still in place; `TestAnInstantFixedDuringAnEvaluationStays` (fails with the unconditional restore). Docs corrected; the nested case documented on `Context.Now` and in the plan; `Context` documents one evaluation at a time; `TestEvaluatingAParseTreeSeesOneInstant` (fails without the clock in `Evaluate`) |
| 2 | No medium or higher (converged); round-1 fixes re-verified by mutation; the registered test function does not leak. Low: a stale sentence in the record; `SetNow` called partway through an evaluation splits it in two instants | Record corrected; `SetNow`'s comment says it is meant to be called before evaluating. Converged in round 2 |
