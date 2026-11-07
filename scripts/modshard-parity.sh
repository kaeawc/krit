#!/usr/bin/env bash
# Compare krit findings and timings with and without module sharding on a repository.
#
#   [MODES="whole shard"] [SHARDS=auto] [KEEP=1] scripts/modshard-parity.sh <repo> <out-dir> [krit-binary]
#
# MODES limits which modes run. SHARDS is the KRIT_MODULE_SHARDS value for the sharded runs (a
# worker count or "auto"). KEEP=1 leaves the last run's .krit state in place instead of restoring.
# Needs a Gradle model under <repo>/.krit/gradle-model and built dependency jars. It saves the
# repository's other .krit cache state to <out-dir>/saved-krit and restores it afterwards, then
# runs cold and warm for each mode and compares the cold outputs with findingsdiff. Run it alone:
# each analysis uses many CPUs.
set -euo pipefail

repo="${1:?usage: modshard-parity.sh <repo> <out-dir> [krit-binary]}"
out="${2:?usage: modshard-parity.sh <repo> <out-dir> [krit-binary]}"
krit="${3:-krit}"
root="$(cd "$(dirname "$0")/.." && pwd)"
repo="$(cd "$repo" && pwd)"
mkdir -p "$out"
out="$(cd "$out" && pwd)"

saved="$out/saved-krit"
if [[ -d "$repo/.krit" && ! -d "$saved" ]]; then
  mkdir -p "$saved"
  for entry in "$repo"/.krit/*; do
    [[ "$(basename "$entry")" == gradle-model ]] && continue
    mv "$entry" "$saved/"
  done
fi
restore() {
  for entry in "$repo"/.krit/*; do
    [[ "$(basename "$entry")" == gradle-model ]] && continue
    rm -rf "$entry"
  done
  if [[ -d "$saved" ]]; then
    mv "$saved"/* "$repo/.krit/" 2>/dev/null || true
    rmdir "$saved" 2>/dev/null || true
  fi
}
[[ "${KEEP:-}" == 1 ]] || trap restore EXIT

clear_cache() {
  for entry in "$repo"/.krit/*; do
    [[ "$(basename "$entry")" == gradle-model ]] && continue
    rm -rf "$entry"
  done
}

# run <label> <shards>: one analysis; wall, user and sys seconds and the load average go to <label>.time
run() {
  local label="$1" shards="$2"
  echo "load before $label: $(uptime | sed 's/.*load averages*: //')" >&2
  (cd "$repo" && KRIT_MODULE_SHARDS="$shards" /usr/bin/time -p "$krit" --no-daemon -no-fir-daemon -v -f json . \
    >"$out/$label.json" 2>"$out/$label.log") || true
  tail -n 3 "$out/$label.log" | grep -E '^(real|user|sys)' >"$out/$label.time" || true
}

for mode in ${MODES:-whole shard}; do
  shards=0
  [[ "$mode" == shard ]] && shards="${SHARDS:-auto}"
  clear_cache
  run "$mode-cold" "$shards"
  run "$mode-warm" "$shards"
done

(cd "$root" && go run ./internal/devtools/findingsdiff -a "$out/whole-cold.json" -b "$out/shard-cold.json" \
  -root "$repo" -a-name whole -b-name module -examples 5) | tee "$out/parity.txt" || true
(cd "$root" && go run ./internal/devtools/findingsdiff -a "$out/whole-warm.json" -b "$out/shard-warm.json" \
  -root "$repo" -a-name whole -b-name module -examples 0) | tail -n 3
for f in "$out"/*.time; do echo "$(basename "$f"): $(tr '\n' ' ' <"$f")"; done
