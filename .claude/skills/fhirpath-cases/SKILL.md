---
name: fhirpath-cases
description: Escribe y mide casos de conformidad FHIRPath para un área a partir del texto de la especificación - case-author los escribe a ciegas con cita textual, case-verifier los ataca, ambos motores (este y fhirpath.js) los corren, y case-adjudicator decide cada fallo y divergencia. Úsalo cuando el usuario invoque /fhirpath-cases o pida casos de spec para un área. Entrega casos, líneas base y razones; no corrige el motor.
argument-hint: "<área, p. ej. 'strings'> [alcance en palabras del usuario]"
---

# Spec cases for one area

Area and scope: $ARGUMENTS

You are the orchestrator. Agents write every artifact; you run commands, regenerate baselines and decide the
next step. You never write cases, checks, adjudications or reasons yourself, and you never pass what an engine
answered to case-author or case-verifier: not in a prompt, not in a file they can read. Read
`conformance/testdata/spec-cases/README.md` first.

Delegate one agent at a time. Every prompt contains `Task directory: .workflow/<slug>/` and `Iteration <n>`.

## 0. Setup

1. `git status --porcelain --untracked-files=no` must be empty; otherwise stop and ask.
2. Area in kebab-case (`strings`). Slug `cases-<area>-<YYYYMMDD>`. Create `.workflow/<slug>/context/`.
3. `make spec-sources`. A REFUSED source means a pinned text changed upstream: stop and tell the user.
4. Write `00-requirement.md`: `Base commit: <sha>`, the area, the scope in the user's words, and which version
   to follow where 2.0.0 and 3.0.0 differ (3.0.0 unless the user says otherwise).

## 1. Cases

**case-author**: "Task directory: .workflow/<slug>/. Iteration <n>. Area: <area>." Then:

    cd conformance && SPEC_SOURCES_REQUIRED=1 go test -count=1 -run 'TestSpecCaseCatalog' . \
      > ../.workflow/<slug>/context/catalog-<n>.txt 2>&1

A format error or a quote not found goes back to case-author (n + 1) with that file's errors pasted in the
prompt; they concern the case file only, never an engine. At most 2 rounds.

## 2. Check

**case-verifier**: "Task directory: .workflow/<slug>/. Iteration <n>. Area: <area>."
- CONTINUE → 3. FIX → 1 with n + 1. HUMAN_REVIEW → show the questions and stop. At most 2 rounds of 1–2.

## 3. Run both engines

    cd conformance && go test -count=1 -run '^TestSpecCases$/^<area>$/' -show-known-failures -v . \
      > ../.workflow/<slug>/context/run-<n>.txt 2>&1
    make difftest DIFFTEST_ARGS="-corpus spec -v" > .workflow/<slug>/context/difftest-<n>.txt 2>&1

The first exits non-zero whenever a case fails that the baseline does not list yet — always on an area's
first run. That is expected; its output is what the adjudicator reads. `-show-known-failures` makes it log
the cases already listed too, so a listed case the author changed is judged again rather than skipped. The
anchors matter: without them `strings` would also run an area named `strings-extra`.

## 4. Adjudicate

**case-adjudicator**: "Task directory: .workflow/<slug>/. Iteration <n>. Area: <area>. Runs:
context/run-<n>.txt, context/difftest-<n>.txt."
- FIX_CASES → case-author (n + 1) with "Apply 03-case-defects.md", then 1, **2** (any changed case needs a
  CONTINUE again), 3 and 4. At most 3 rounds.
- HUMAN_REVIEW → show the questions and stop.
- DONE → 5.

## 5. Baselines and gate

    cd conformance && go test -count=1 -run '^TestSpecCases$/^<area>$/' -update-known-failures . ; \
      git status --porcelain --untracked-files=all -- testdata ':!testdata/spec-cases/<area>.*'

The first rewrites this area's baselines and nothing else. It exits non-zero when it lists a new failure,
which is the point of rewriting; judge it by the gate below, not by its exit code. The second must print
nothing: no other area's baselines, and not the official suite's, may change in this run. If it prints
anything, stop and tell the user. Then the full gate:

    cd conformance && SPEC_SOURCES_REQUIRED=1 go test -count=1 ./...

It fails if a failing case has no reason, a reason has no failing case, or a quote is not verbatim. A missing
reason goes back to case-adjudicator; never write one yourself.

## 6. Report

To the user, in Spanish:

- Requirements and cases, by source (2.0.0, 3.0.0, FHIR R4/R5) and strength.
- Results per version: passing, and failing by kind (gap, deliberate, needs-external), from the reasons file.
- Gaps: case, quote, cause (file:line), suggested fix.
- Where fhirpath.js was wrong, and where both engines agreed against a case and how that was settled.
- Questions the specification leaves open (from `01-cases.md`), each a decision to take on purpose.
- Case defects found and corrected, which shows the cases were checked and not only generated.
- Files changed (`git status --porcelain`). Do not commit.
