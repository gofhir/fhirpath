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
mkdir -p build/bench

# Pinned so that a comparison does not change because the tool did, and fetched
# now: it is needed after both runs, and finding out then that it cannot be
# downloaded wastes the measurements.
BENCHSTAT="golang.org/x/perf/cmd/benchstat@v0.0.0-20260813145340-fd4a688df892"
benchstat_bin="$repo_root/build/bench/benchstat"
if ! GOBIN="$repo_root/build/bench" go install "$BENCHSTAT"; then
	echo "bench-compare: cannot install $BENCHSTAT" >&2
	exit 1
fi

if ! git rev-parse --verify --quiet "$BASE" >/dev/null; then
	echo "bench-compare: no such revision: $BASE" >&2
	echo "  a tag published by CI is not here until 'git fetch --tags'" >&2
	exit 1
fi

out_dir="build/bench"
worktree="build/bench-base"
rm -rf "$worktree"
# A run that was killed leaves the worktree registered without its directory,
# which makes the next 'worktree add' fail rather than replace it.
git worktree prune

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
	# Both sides measure this tree's benchmarks, including any it adds.
	#
	# Every file is copied before anything is judged. Copying and checking one
	# at a time judges the baseline in a state that is half its files and half
	# this tree's, where a benchmark this tree moved between files exists in
	# both and the duplicate fails a build that the finished copy would have
	# passed.
	bench_files=()
	while IFS= read -r bench_file; do
		# The list names what git knows; a tracked file deleted from disk is
		# not there to copy.
		[ -f "$bench_file" ] || continue
		bench_files+=("$bench_file")

		saved="$out_dir/$(echo "$bench_file" | tr / _).orig"
		if [ -f "$worktree/$bench_file" ]; then
			cp "$worktree/$bench_file" "$saved"
		else
			rm -f "$saved"
		fi

		mkdir -p "$worktree/$(dirname "$bench_file")"
		cp "$bench_file" "$worktree/$bench_file"
	done < <(git ls-files --cached --others --exclude-standard '*_bench_test.go')

	# Restores what the baseline had at that path, which is either its own file
	# or nothing.
	restore_baseline() {
		local bench_file="$1"
		local saved="$out_dir/$(echo "$bench_file" | tr / _).orig"
		if [ -f "$saved" ]; then
			cp "$saved" "$worktree/$bench_file"
		else
			rm -f "$worktree/$bench_file"
		fi
	}

	if ! baseline_builds; then
		# Something here calls API the baseline does not have. Find which:
		# putting one file back and rebuilding says whether that file was the
		# reason, and a file that was not is copied again before the next is
		# tried, so that an innocent one is not given up along the way.
		kept_own=()
		dropped=()
		for bench_file in "${bench_files[@]}"; do
			restore_baseline "$bench_file"
			if baseline_builds; then
				if [ -f "$out_dir/$(echo "$bench_file" | tr / _).orig" ]; then
					kept_own+=("$bench_file")
				else
					dropped+=("$bench_file")
				fi
				break
			fi
			# Not this one on its own; put this tree's back and try the next.
			cp "$bench_file" "$worktree/$bench_file"
		done

		# More than one file is responsible, so they go back together.
		if ! baseline_builds; then
			kept_own=()
			dropped=()
			for bench_file in "${bench_files[@]}"; do
				restore_baseline "$bench_file"
				if [ -f "$out_dir/$(echo "$bench_file" | tr / _).orig" ]; then
					kept_own+=("$bench_file")
				else
					dropped+=("$bench_file")
				fi
				baseline_builds && break
			done
		fi

		if ! baseline_builds; then
			echo "bench-compare: $BASE does not build even with its own benchmarks" >&2
			exit 1
		fi

		if [ ${#kept_own[@]} -gt 0 ]; then
			echo "note: $BASE cannot build this tree's ${kept_own[*]},"
			echo "      so it keeps its own; benchmarks named alike are still compared"
		fi
		if [ ${#dropped[@]} -gt 0 ]; then
			echo "note: $BASE cannot build ${dropped[*]} and has none of its own,"
			echo "      so those benchmarks are reported without a comparison"
		fi
	fi
fi

# The two are interleaved, a sample of each in turn, rather than all of one and
# then all of the other. A laptop measured for a minute is not the machine it
# was at the start of it: running the baseline first and this tree second gave
# this tree a steady 20-25% penalty on identical code. Alternating spreads that
# drift across both sides instead of charging it to one.
echo "==> $BASE and $(git rev-parse --abbrev-ref HEAD), a sample of each in turn"
: > "$out_dir/base.txt"
: > "$out_dir/head.txt"

for sample in $(seq 1 "$COUNT"); do
	if ! (cd "$worktree" && go test -run '^$' -bench "$PATTERN" -benchmem \
		-benchtime="$BENCHTIME" -count=1 "$PKG") >> "$out_dir/base.txt" 2>"$out_dir/base.err"; then
		cat "$out_dir/base.err" >&2
		echo "bench-compare: the baseline did not run" >&2
		exit 1
	fi

	go test -run '^$' -bench "$PATTERN" -benchmem \
		-benchtime="$BENCHTIME" -count=1 "$PKG" >> "$out_dir/head.txt"

	printf '  sample %d of %d\n' "$sample" "$COUNT"
done

echo
"$benchstat_bin" "$out_dir/base.txt" "$out_dir/head.txt"
