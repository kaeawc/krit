#!/usr/bin/env bash
# Performance regression guard (#1657): time krit at a base revision and at
# the working tree, side by side on this machine, and fail on a regression.
# CI runs it on every pull request (ci.yml, perf-guard); `make perf-guard`
# runs it locally.
#
#   1. compare: base vs head on pinned corpora, interleaved rounds, medians
#      per total, phase and rule (internal/devtools/perfguard compare)
#   2. scale:   head on a small and a 6x larger synthetic Android project;
#      fails on super-linear phases/rules unless known-superlinear.txt lists
#      them with their issue (perfguard scale)
#
# With --fir (#1729) it instead times FIR and the type oracle: base and head,
# each with the krit-fir jar built from its own sources, in one-shot JVMs on
# a synthetic JVM-only multi-module project with a Gradle model (perfguard
# gen -kind jvm, compare -fir). It needs a JDK 21; jars are built once per
# source hash (scripts/build-dev-jar.sh). The report is fir.md.
#
# Usage: scripts/perf-guard.sh [--base REV] [--fir]
#
#   --base REV  revision to compare against (default: merge-base with
#               origin/main)
#   --fir       time FIR and the type oracle instead of the source-only scan
#
# Environment:
#   KRIT_PERF_DIR      work directory (default ~/.krit/perf-guard): binaries,
#                      corpora, compare.md and scale.md (fir.md with --fir)
#   KRIT_PERF_CORPORA  pinned corpora from tests/corpus/repos.json
#                      (default "nowinandroid okhttp"; "" for none)
#   KRIT_PERF_ROUNDS   interleaved rounds per corpus (default 5)
#
# Exit status: 0 no regression, 1 regression, 2 setup error.
set -euo pipefail

base=""
fir=0
while [ $# -gt 0 ]; do
	case "$1" in
		--base) base=$2; shift 2 ;;
		--fir) fir=1; shift ;;
		-h|--help) sed -n '2,32p' "$0"; exit 0 ;;
		*) echo "unknown argument: $1" >&2; exit 2 ;;
	esac
done

root=$(git rev-parse --show-toplevel)
cd "$root"
work=${KRIT_PERF_DIR:-$HOME/.krit/perf-guard}
corpora=${KRIT_PERF_CORPORA-nowinandroid okhttp}
rounds=${KRIT_PERF_ROUNDS:-5}
mkdir -p "$work/bin" "$work/corpus"

if [ -z "$base" ]; then
	base=$(git merge-base HEAD origin/main)
fi
base_sha=$(git rev-parse "$base^{commit}")
echo "perf-guard: base $base_sha, head $(git rev-parse HEAD) (+ working tree)"

go build -o "$work/bin/perfguard" ./internal/devtools/perfguard/
go build -o "$work/bin/krit-head" ./cmd/krit/
rm -rf "$work/base-tree"
git worktree prune
git worktree add -q --detach "$work/base-tree" "$base_sha"
(cd "$work/base-tree" && go build -o "$work/bin/krit-base" ./cmd/krit/)
if [ "$fir" = 1 ]; then
	# Each side runs the krit-fir its own sources build; the same sources
	# hash to the same cached jar.
	base_jar=$(cd "$work/base-tree" && bash scripts/build-dev-jar.sh krit-fir | tail -n 1)
fi
git worktree remove --force "$work/base-tree"

if [ "$fir" = 1 ]; then
	head_jar=$(bash scripts/build-dev-jar.sh krit-fir | tail -n 1)
	for jar in "$base_jar" "$head_jar"; do
		[ -f "$jar" ] || { echo "perf-guard: no krit-fir jar at '$jar'" >&2; exit 2; }
	done
	echo "perf-guard: krit-fir base $base_jar, head $head_jar"
	rm -rf "$work/fir-jvm"
	"$work/bin/perfguard" gen -kind jvm -out "$work/fir-jvm" -modules 8 -files 20 -classes 4
	status=0
	"$work/bin/perfguard" compare -fir -base "$work/bin/krit-base" -head "$work/bin/krit-head" \
		-base-env "KRIT_FIR_JAR=$base_jar" -head-env "KRIT_FIR_JAR=$head_jar" \
		-corpus "jvm=$work/fir-jvm" -rounds "$rounds" -report "$work/fir.md" \
		-title "FIR and type oracle: base ${base_sha:0:10} vs head" >/dev/null || status=$?
	[ "$status" -le 1 ] || exit 2
	cat "$work/fir.md"
	if [ "$status" = 1 ]; then
		echo "perf-guard: FIR regression (report: $work/fir.md)" >&2
		exit 1
	fi
	echo "perf-guard: FIR OK (report: $work/fir.md)"
	exit 0
fi

corpus_args=()
for name in $corpora; do
	read -r url commit < <(python3 -I -c '
import json, sys
for r in json.load(open(sys.argv[1]))["repos"]:
    if r["name"] == sys.argv[2]:
        print(r["url"], r["commit"]); break
else:
    sys.exit("perf-guard: %s is not in tests/corpus/repos.json" % sys.argv[2])
' tests/corpus/repos.json "$name")
	dir="$work/corpus/$name"
	if [ "$(git -C "$dir" rev-parse HEAD 2>/dev/null || true)" != "$commit" ]; then
		rm -rf "$dir"
		git init -q "$dir"
		git -C "$dir" fetch -q --depth 1 "$url" "$commit"
		git -C "$dir" -c advice.detachedHead=false checkout -q FETCH_HEAD
	fi
	corpus_args+=(-corpus "$name=$dir")
done

# Synthetic Android-shaped projects: "small" is also a compare corpus; the
# 6x "large" one only feeds the scaling check.
rm -rf "$work/synthetic-small" "$work/synthetic-large"
"$work/bin/perfguard" gen -out "$work/synthetic-small" -modules 10 -files 20 -classes 6
"$work/bin/perfguard" gen -out "$work/synthetic-large" -modules 60 -files 20 -classes 6
corpus_args+=(-corpus "synthetic=$work/synthetic-small" -corpus "playground-android=$root/playground/android-app")

status=0
"$work/bin/perfguard" compare -base "$work/bin/krit-base" -head "$work/bin/krit-head" \
	"${corpus_args[@]}" -rounds "$rounds" -report "$work/compare.md" \
	-title "Performance: base ${base_sha:0:10} vs head" >/dev/null || status=$?
[ "$status" -le 1 ] || exit 2
scale_status=0
"$work/bin/perfguard" scale -bin "$work/bin/krit-head" -base "$work/bin/krit-base" \
	-small "$work/synthetic-small" -large "$work/synthetic-large" -factor 6 \
	-known internal/devtools/perfguard/known-superlinear.txt \
	-report "$work/scale.md" >/dev/null || scale_status=$?
[ "$scale_status" -le 1 ] || exit 2

cat "$work/compare.md" "$work/scale.md"
if [ "$status" = 1 ] || [ "$scale_status" = 1 ]; then
	echo "perf-guard: regression (reports: $work/compare.md, $work/scale.md)" >&2
	exit 1
fi
echo "perf-guard: OK (reports: $work/compare.md, $work/scale.md)"
