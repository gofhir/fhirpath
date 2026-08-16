#!/usr/bin/env bash
#
# Compare this working tree's benchmarks against another revision.
#
# A benchmark number on its own says very little: this machine's noise reaches
# ±10% between runs, which is wider than most changes worth making. What tells
# a change from the noise is running both revisions now, on this machine, and
# letting benchstat weigh the samples.
#
# The baseline is a git worktree of BASE, carrying this tree's benchmark files
# so that both sides measure the same thing -- a benchmark added along with the
# change under test does not exist there otherwise, and there would be nothing
# to compare against. Set NOCOPY=1 to measure BASE's own benchmarks instead,
# which is what to do when this tree's use API that BASE does not have.
#
# Usage:
#   scripts/bench-compare.sh [BASE] [BENCH_PATTERN]
#
#   BASE            revision to compare against  (default: main)
#   BENCH_PATTERN   -bench pattern               (default: .)
#
# Environment:
#   COUNT      samples per benchmark  (default: 6, benchstat's minimum for a
#              confidence interval)
#   BENCHTIME  -benchtime             (default: 0.5s)
#   PKG        package to measure     (default: .)
#   NOCOPY     leave BASE's benchmark files alone (default: unset)

set -euo pipefail

BASE="${1:-main}"
PATTERN="${2:-.}"
COUNT="${COUNT:-6}"
BENCHTIME="${BENCHTIME:-0.5s}"
PKG="${PKG:-.}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if ! git rev-parse --verify --quiet "$BASE" >/dev/null; then
	echo "bench-compare: no such revision: $BASE" >&2
	echo "  a tag published by CI is not here until 'git fetch --tags'" >&2
	exit 1
fi

out_dir="build/bench"
worktree="build/bench-base"
mkdir -p "$out_dir"
rm -rf "$worktree"

cleanup() {
	git worktree remove --force "$worktree" >/dev/null 2>&1 || true
	git worktree prune >/dev/null 2>&1 || true
}
trap cleanup EXIT

if ! git diff --quiet || ! git diff --cached --quiet; then
	echo "note: measuring the working tree as it stands, including uncommitted changes"
fi

echo "==> baseline: $BASE"
git worktree add --detach --quiet "$worktree" "$BASE"

# Whether the baseline still builds, which is how a benchmark file that uses
# API the baseline does not have is found.
baseline_builds() {
	(cd "$worktree" && go test -run '^$' -bench '^$' -count=1 "$PKG") >/dev/null 2>&1
}

if [ -z "${NOCOPY:-}" ]; then
	# Both sides measure this tree's benchmarks, including any it adds. A file
	# the baseline cannot compile -- because the change under test is what
	# introduced the API it calls -- is put back as it was, and named, rather
	# than failing the whole comparison.
	skipped=()
	while IFS= read -r bench_file; do
		mkdir -p "$worktree/$(dirname "$bench_file")"
		restore=""
		if [ -f "$worktree/$bench_file" ]; then
			restore="$out_dir/$(echo "$bench_file" | tr / _).orig"
			cp "$worktree/$bench_file" "$restore"
		fi

		cp "$bench_file" "$worktree/$bench_file"
		if ! baseline_builds; then
			if [ -n "$restore" ]; then
				cp "$restore" "$worktree/$bench_file"
			else
				rm -f "$worktree/$bench_file"
			fi
			skipped+=("$bench_file")
		fi
	done < <(git ls-files --cached --others --exclude-standard '*_bench_test.go')

	if [ ${#skipped[@]} -gt 0 ]; then
		echo "note: $BASE cannot build ${skipped[*]}, so it keeps its own; benchmarks"
		echo "      only in those files are reported without a comparison"
	fi
fi

if ! (cd "$worktree" && go test -run '^$' -bench "$PATTERN" -benchmem \
	-benchtime="$BENCHTIME" -count="$COUNT" "$PKG") > "$out_dir/base.txt" 2>"$out_dir/base.err"; then
	cat "$out_dir/base.err" >&2
	echo "bench-compare: the baseline did not run" >&2
	exit 1
fi

echo "==> current: $(git rev-parse --abbrev-ref HEAD)"
go test -run '^$' -bench "$PATTERN" -benchmem \
	-benchtime="$BENCHTIME" -count="$COUNT" "$PKG" > "$out_dir/head.txt"

echo
go run golang.org/x/perf/cmd/benchstat@latest "$out_dir/base.txt" "$out_dir/head.txt"
