#!/usr/bin/env bash
#
# Compare what this working tree and another revision answer over the official
# FHIR examples.
#
# The conformance suite and difftest both stayed green through a release that
# made four choice elements answer empty across twenty R4 examples. Evaluating
# what a validator evaluates -- every element path, every constraint -- over
# the examples a validator is given, on both revisions, is what found it. See
# corpusdiff/main.go for what the corpus holds.
#
# The same corpusdiff source is built twice: once against a git worktree of
# BASE, once against this tree. Both sides therefore evaluate the same
# expressions, and BASE only needs the API corpusdiff uses, which every release
# from 1.7.0 (Document) has.
#
# Usage:
#   scripts/corpusdiff.sh [BASE] [FHIR]
#
#   BASE   revision to compare against   (default: main)
#   FHIR   r4, r5 or both                (default: both)
#
# Environment:
#   VERBOSE   list every differing evaluation, not one per expression

set -euo pipefail

BASE="${1:-main}"
FHIR="${2:-both}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

out_dir="build/corpusdiff"
worktree="build/corpusdiff-base"
cache="$out_dir/cache"
mkdir -p "$out_dir"

if ! git rev-parse --verify --quiet "$BASE" >/dev/null; then
	echo "corpusdiff: no such revision: $BASE" >&2
	echo "  a tag published by CI is not here until 'git fetch --tags'" >&2
	exit 1
fi

case "$FHIR" in
	r4 | r5) versions=("$FHIR") ;;
	both) versions=(r4 r5) ;;
	*) echo "corpusdiff: FHIR must be r4, r5 or both, not $FHIR" >&2; exit 1 ;;
esac

rm -rf "$worktree" "$out_dir/tool-base"
git worktree prune

cleanup() {
	git worktree remove --force "$worktree" >/dev/null 2>&1 || true
	git worktree prune >/dev/null 2>&1 || true
	rm -rf "$out_dir/tool-base"
}
trap cleanup EXIT

if ! git diff --quiet || ! git diff --cached --quiet; then
	echo "note: evaluating the working tree as it stands, including uncommitted changes"
fi

echo "==> building against $BASE and against this tree"
git worktree add --detach --quiet "$worktree" "$BASE"

# The baseline's copy of the tool is this tree's source pointed at the
# worktree, since BASE may predate corpusdiff or carry an older corpus.
cp -R corpusdiff "$out_dir/tool-base"
(cd "$out_dir/tool-base" && go mod edit -replace "github.com/gofhir/fhirpath=$repo_root/$worktree" \
	&& go build -o ../corpusdiff-base .)
(cd corpusdiff && go build -o "../$out_dir/corpusdiff-head" .)

for version in "${versions[@]}"; do
	# Fetched once, before the two runs, so that they do not race to fill the
	# cache.
	"$out_dir/corpusdiff-head" fetch -fhir "$version" -cache "$cache"

	echo "==> $version: evaluating both, side by side"
	# trace() writes to stderr; what the runs print there goes to a log.
	"$out_dir/corpusdiff-base" eval -fhir "$version" -cache "$cache" -out "$out_dir/$version-base.tsv" \
		2>"$out_dir/$version-base.log" &
	base_pid=$!
	"$out_dir/corpusdiff-head" eval -fhir "$version" -cache "$cache" -out "$out_dir/$version-head.tsv" \
		2>"$out_dir/$version-head.log"
	if ! wait "$base_pid"; then
		tail -5 "$out_dir/$version-base.log" >&2
		echo "corpusdiff: the baseline did not run" >&2
		exit 1
	fi

	echo
	echo "==> $version: $BASE against $(git rev-parse --abbrev-ref HEAD)"
	"$out_dir/corpusdiff-head" compare ${VERBOSE:+-v} "$out_dir/$version-base.tsv" "$out_dir/$version-head.tsv"
	echo
done
