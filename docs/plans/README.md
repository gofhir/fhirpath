# Resume point

- **Active increment:** 015 — a resource's number is read by JSON's
  grammar (#238). Plan: `docs/plans/015-resource-number-grammar.md`.
  014 (#246) merged as #250.
- **Step:** 9, closed (2026-11-01). Review converged in round 2 (Codex):
  round 1's three medium findings fixed; round 2's rejected as out of scope
  (tokens holding a byte of JSON's structure, which jsonparser itself reads
  inconsistently, JP-2). `make check` green; three-engine battery all
  agree; corpusdiff 0; mutation sets 010–015 all caught. Committed as 94598c6.
- **Next:** PR #252 open, awaiting review and merge by the user.
- **Pending user decisions:** none for 015 (D2 published as #251). From 010, still open outside this repo: searchindex SI-3, go-cql
  CQL-1 (docs/upstream-findings.md, unpublished).
