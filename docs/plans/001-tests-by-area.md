# 001 — Root tests organised by area

Status: done, pending commit

## Why

The root package had 9 source files and 83 test files named after the change
that introduced them (`*_fixes_test.go`, `default_offset_reach_test.go`, ...)
rather than after what they test. Two `testModel` types and two
`BenchmarkCompile` functions coexisted, one in each test package, and the two
benchmarks reported under the same name with different costs.

## Scope

- Every root `*_test.go` is regrouped, by test function, into one file per
  area. No test body changes beyond qualifying root identifiers.
- Out of scope: `eval/`, `funcs/`, `types/`, `parser/`, `internal/`, the tool
  modules (`conformance/`, `corpusdiff/`, `difftest/`) and the test contents.

## Convention

- File names: `<area>_test.go`, `<area>_bench_test.go`, and
  `<area>_<topic>_test.go` when an area is too large for one file.
- Areas are those of `conformance/testdata/spec-cases/*.xml` with `-` written
  `_`: `aggregates`, `boolean_logic`, `collections`, `conversions`, `datetime`,
  `equality_comparison`, `existence`, `fhir_functions`, `filtering_projection`,
  `math`, `strings`. Behaviour the specification sections do not cover goes to
  one of four engine areas: `navigation` (reading elements, primitives, roots),
  `typing` (types, models, `is`/`as`/`ofType`), `evaluation` (singleton rules,
  scopes, variables, short-circuit), `api` (compile, cache, options, documents,
  location, static analysis).
- Tests are black-box (`package fhirpath_test`). The few that need unexported
  identifiers live in `internal_test.go` / `internal_bench_test.go`
  (`package fhirpath`).
- Helpers used by more than one area live in `helpers_test.go`; helpers any
  benchmark uses live in a `*_bench_test.go` file, because
  `scripts/bench-compare.sh` carries only `*_bench_test.go` files into the
  baseline worktree.

## Decisions taken from the adversarial round

Reviewed by an independent agent before any change; findings and resolution:

| Finding | Resolution |
| --- | --- |
| Moving tests to a separate `enginetest/` directory breaks `bench-compare`, splits helpers across the boundary and changes coverage meaning (`-coverpkg=./...` lifts eval 24→87 %, total 40→81 %) | Accepted: tests stay in the root package; coverage flags unchanged |
| Mapping by file name is wrong for `integration_test.go`, `fhirpath_test.go`, `functions_test.go` | Accepted: mapping is by test function |
| A single `bench_test.go` does not match the `*_bench_test.go` glob | Accepted: per-area `*_bench_test.go` |
| Duplicate `BenchmarkCompile` corrupts benchstat samples | Accepted: the one compiling `Patient.name.given.first()` becomes `BenchmarkCompileWithFunction` |
| The two `testModel` types differ and cannot be merged | Accepted: the internal one becomes `typedChildrenModel` |
| No dot imports (revive `dot-imports`) | Accepted: root identifiers are qualified `fhirpath.X` |
| `git log --follow` does not survive merging files | Accepted: history is reachable with `git log -- <old path>` and `git blame -C -C -M` |
| Counting `=== RUN` is too weak | Accepted: acceptance below |
| `make conformance` masks failures (pipe without `pipefail`) | Accepted: `make check` does not use it |

## Exit criteria

Measured against the tree before the change (`e0cbdfd`):

1. Identical sorted list of test and subtest names with their status
   (`go test -v`), apart from the benchmark rename above.
2. Identical `go test -list '.*'` apart from that rename.
3. `go test -race -shuffle=on -count=3 .` passes.
4. Root coverage unchanged (82.8 % at `e0cbdfd`).
5. `make check` passes.
6. `make bench-compare BASE=<this branch>` builds both sides.

## Record

- Baseline at `e0cbdfd`: 1727 tests and subtests, all PASS; 42 benchmarks,
  1 fuzz target; root coverage 82.8 %.
- The regrouping was generated, not edited by hand: a throwaway tool built on
  `go/packages` assigned each declaration from the type information (test
  functions by the mapping, helpers to the file of their users, white-box
  status by use of an unexported root identifier, closed over dependencies) and
  qualified root identifiers as `fhirpath.X`. 83 files became 33; comment lines
  (1834) and code lines (10423) are the same before and after.
- White-box: `TestDocumentMatchesEvaluate`-style tests reading `doc.root`,
  `TestSLLPredictionGivesLLTree` / `FuzzSLLPredictionGivesLLTree` (`parse`),
  `TestInterpretedBooleanOperatorsDoNotEvaluateARightOperandTheLeftDecides`
  (`expr.tree`), and `BenchmarkCompileSuite`, which shares
  `expressionElement` with the fuzz test.
- The contributing guides (en, es) gained "Where Root Tests Live" with the
  convention; the README is the API reference and was left alone.
- `article.md` deleted (published on dev.to, unchanged since the first commit).
- `make check` added: `generate-check`, `lint`, `go test -race` in the root,
  conformance and corpusdiff modules (corpusdiff's tests ran nowhere before,
  neither in `make test` nor in CI).
- `bench-compare` against a base before this change copies the new bench files,
  cannot build them there, and falls back to the base's own benchmarks; those
  with the same name are still compared, except `BenchmarkCompile`: before
  this change two different benchmarks reported under that name, so its
  comparison against such a base mixes them and means nothing. Against a base
  after this change both sides build the same files.

### Oracle mutations

The acceptance comparator (sorted `go test -v` names with status, diffed
against the baseline) was run on a copy with injected defects:

| Mutation | Detected |
| --- | --- |
| None (control) | no difference |
| A test function renamed so it no longer runs | yes |
| A test changed to `t.Skip` | yes |
| A subtest renamed | yes |

### Code review

| Round | Findings | Resolution |
| --- | --- | --- |
| 1 | No high or medium. Verified mechanically: 431 declarations before and after with identical bodies modulo qualifier and the two renames; 1848 comments kept; names and statuses identical with and without `-short`; identical block-level coverage profile; bench files build with every other test file deleted; Hugo anchors resolve. Low: two comments that described a whole file became false once merged; the `BenchmarkCompile` comparison against older bases mixes two benchmarks; `make check` did not test corpusdiff | All three fixed: the stale bench comment removed and the invariants comment reworded; the caveat recorded above; corpusdiff tests added to `make check`. Converged in round 1 |
