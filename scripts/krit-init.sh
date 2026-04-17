#!/usr/bin/env bash
# Krit onboarding — Phase 1 (gum prototype).
#
# Scans a target directory with each shipped profile, shows a
# comparison table, and asks the user to pick one. Phase 1 currently
# implements steps 1–3 of the gum flow. Steps 4–7 (controversial-rules
# questionnaire, config write-out, autofix pass, baseline) are marked
# with TODO stubs and will be filled in as the roadmap progresses.
#
# Usage:
#   scripts/krit-init.sh [target-directory]
#   scripts/krit-init.sh --profile balanced [target-directory]
#   scripts/krit-init.sh --profile balanced --yes [target-directory]
#
# --profile skips the interactive profile selection step.
# --yes accepts the per-profile defaults for every controversial-rule
#       question (no prompts). Intended for integration tests and CI.
#
# Requires: krit, gum, jq.

set -euo pipefail

preset_profile=""
accept_defaults=0
args=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        --profile)
            preset_profile="${2:-}"
            shift 2
            ;;
        --profile=*)
            preset_profile="${1#*=}"
            shift
            ;;
        --yes|-y)
            accept_defaults=1
            shift
            ;;
        -h|--help)
            sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            args+=("$1")
            shift
            ;;
    esac
done
set -- "${args[@]-}"

# ---------- setup -----------------------------------------------------------

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
profiles_dir="$repo_root/config/profiles"
target="${1:-.}"

if [[ ! -d "$target" ]]; then
    echo "error: target directory '$target' does not exist" >&2
    exit 2
fi
target="$(cd "$target" && pwd)"

require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "error: '$1' is required but not installed" >&2
        echo "       install: $2" >&2
        exit 2
    fi
}

require gum 'brew install gum'
require jq 'brew install jq'
require yq 'brew install yq'

krit_bin=""
for candidate in "${KRIT_BIN:-}" "$repo_root/krit" "$(command -v krit || true)"; do
    if [[ -n "$candidate" && -x "$candidate" ]]; then
        krit_bin="$candidate"
        break
    fi
done

if [[ -z "$krit_bin" ]]; then
    echo "error: krit binary not found" >&2
    echo "       build one with: go build -o krit ./cmd/krit/" >&2
    exit 2
fi

profiles=(strict balanced relaxed detekt-compat)
for p in "${profiles[@]}"; do
    if [[ ! -f "$profiles_dir/$p.yml" ]]; then
        echo "error: profile '$p' missing at $profiles_dir/$p.yml" >&2
        exit 2
    fi
done

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/krit-init.XXXXXX")"
trap 'rm -rf "$tmpdir"' EXIT

# ---------- step 1: scan each profile --------------------------------------

gum style --bold --foreground 212 "krit onboarding"
gum style --faint "Target: $target"
echo

# Scan each profile. `|| true` because krit exits non-zero when it
# finds issues and we want every profile scanned regardless.
for p in "${profiles[@]}"; do
    gum spin --title "Scanning with $p profile..." -- \
        sh -c "\"$krit_bin\" --config \"$profiles_dir/$p.yml\" -f json \"$target\" >\"$tmpdir/$p.json\" 2>\"$tmpdir/$p.log\" || true"
done

# Fail if any profile produced no parseable output.
for p in "${profiles[@]}"; do
    if ! jq -e '.summary.total' "$tmpdir/$p.json" >/dev/null 2>&1; then
        echo "error: profile '$p' did not produce valid JSON output" >&2
        echo "       log: $tmpdir/$p.log" >&2
        exit 2
    fi
done

# ---------- step 2: comparison table ---------------------------------------

# extract_row NAME: prints "Name|Total|Fixable|Rules|Top rules".
extract_row() {
    local name="$1"
    local json="$tmpdir/$name.json"
    local total fixable rules top
    total=$(jq -r '.summary.total // 0' "$json")
    fixable=$(jq -r '.summary.fixable // 0' "$json")
    rules=$(jq -r '.summary.byRule // {} | length' "$json")
    top=$(jq -r '.summary.byRule // {}
                 | to_entries
                 | sort_by(-.value)
                 | .[:3]
                 | map("\(.key)(\(.value))")
                 | join(" ")' "$json")
    printf '%s|%s|%s|%s|%s\n' "$name" "$total" "$fixable" "$rules" "$top"
}

rows_file="$tmpdir/rows.txt"
: >"$rows_file"
for p in "${profiles[@]}"; do
    extract_row "$p" >>"$rows_file"
done

# Render table with printf + gum style. Column widths are fixed so
# readers can scan results on an 80-column terminal.
{
    printf '%-14s %8s %8s %6s  %s\n' "Profile" "Findings" "Fixable" "Rules" "Top rules"
    printf '%-14s %8s %8s %6s  %s\n' "-------" "--------" "-------" "-----" "---------"
    while IFS='|' read -r name total fixable rules top; do
        printf '%-14s %8s %8s %6s  %s\n' "$name" "$total" "$fixable" "$rules" "$top"
    done <"$rows_file"
} | gum style --border rounded --padding "0 1"
echo

# ---------- step 3: profile selection --------------------------------------

if [[ -n "$preset_profile" ]]; then
    # Non-interactive: used by integration tests.
    selected="$preset_profile"
    valid=0
    for p in "${profiles[@]}"; do
        [[ "$p" == "$selected" ]] && valid=1
    done
    if [[ $valid -eq 0 ]]; then
        echo "error: unknown profile '$selected' (valid: ${profiles[*]})" >&2
        exit 2
    fi
else
    options=()
    while IFS='|' read -r name total fixable rules top; do
        options+=("$(printf '%-14s — %s findings, %s fixable' "$name" "$total" "$fixable")")
    done <"$rows_file"

    choice=$(printf '%s\n' "${options[@]}" | gum choose --header "Which profile fits your team?")
    if [[ -z "$choice" ]]; then
        echo "cancelled." >&2
        exit 1
    fi
    selected=$(echo "$choice" | awk '{print $1}')
fi

gum style --foreground 82 "Selected: $selected"
echo

# ---------- step 4: controversial-rules questionnaire --------------------

registry="$repo_root/config/onboarding/controversial-rules.json"
if [[ ! -f "$registry" ]]; then
    echo "error: registry missing at $registry" >&2
    exit 2
fi

# answers_file: one "question_id<TAB>yes|no" line per resolved question.
# cascade_file: question_ids whose answer was derived from a parent and
# should NOT be prompted.
answers_file="$tmpdir/answers.tsv"
cascade_file="$tmpdir/cascaded.txt"
: >"$answers_file"
: >"$cascade_file"

is_cascaded() {
    grep -qxF "$1" "$cascade_file"
}

record_answer() {
    printf '%s\t%s\n' "$1" "$2" >>"$answers_file"
}

# apply_cascades PARENT_ID ANSWER: looks up all children whose
# cascade_from == PARENT_ID, records their derived answer, and adds
# them to cascade_file so the main loop skips them.
apply_cascades() {
    local parent="$1" answer="$2"
    local children
    children=$(jq -r --arg p "$parent" \
        '.questions[] | select(.cascade_from == $p) | .id' "$registry")
    [[ -z "$children" ]] && return 0
    while IFS= read -r child; do
        [[ -z "$child" ]] && continue
        # Derive the child's answer from the parent. Convention: if the
        # parent is answered "yes" (enforce), each child uses the
        # opposite of its own per-profile default, but in practice we
        # simply use the STRICT defaults for children when the parent
        # is yes, and the RELAXED defaults when the parent is no.
        local derived
        if [[ "$answer" == "yes" ]]; then
            derived=$(jq -r --arg c "$child" \
                '(.questions[] | select(.id == $c) | .defaults.strict) | if . then "yes" else "no" end' "$registry")
        else
            derived=$(jq -r --arg c "$child" \
                '(.questions[] | select(.id == $c) | .defaults.relaxed) | if . then "yes" else "no" end' "$registry")
        fi
        record_answer "$child" "$derived"
        echo "$child" >>"$cascade_file"
        gum style --faint "  → $child → $derived (linked to $parent)"
    done <<<"$children"
}

gum style --bold "Controversial-rule questionnaire"
echo

# Iterate questions in declaration order.
qids=$(jq -r '.questions[].id' "$registry")
while IFS= read -r qid; do
    [[ -z "$qid" ]] && continue
    if is_cascaded "$qid"; then
        continue
    fi

    question=$(jq -r --arg i "$qid" '.questions[] | select(.id == $i) | .question' "$registry")
    rationale=$(jq -r --arg i "$qid" '.questions[] | select(.id == $i) | .rationale' "$registry")
    default=$(jq -r --arg i "$qid" --arg p "$selected" \
        '(.questions[] | select(.id == $i) | .defaults[$p]) | if . then "yes" else "no" end' "$registry")

    echo
    gum style --foreground 212 "$question"
    gum style --faint "$rationale"

    if [[ $accept_defaults -eq 1 ]]; then
        answer="$default"
        gum style --faint "  → $answer (default for $selected)"
    else
        if [[ "$default" == "yes" ]]; then
            if gum confirm "$question" --default=yes; then answer="yes"; else answer="no"; fi
        else
            if gum confirm "$question" --default=no; then answer="yes"; else answer="no"; fi
        fi
    fi
    record_answer "$qid" "$answer"
    apply_cascades "$qid" "$answer"
done <<<"$qids"
echo

# ---------- step 5: write config ------------------------------------------

target_config="$target/krit.yml"
if [[ -f "$target_config" ]]; then
    gum style --foreground 214 "warning: $target_config already exists; moving aside to krit.yml.bak"
    mv "$target_config" "$target_config.bak"
fi

# Collect overrides as tab-separated "ruleset\tRule\tactive" lines, then
# group them by ruleset when emitting YAML to avoid duplicate top-level
# keys (the YAML parser warns on those even though merge still works).
raw_overrides="$tmpdir/overrides.tsv"
: >"$raw_overrides"

while IFS=$'\t' read -r qid answer; do
    [[ -z "$qid" ]] && continue

    rules=$(jq -r --arg i "$qid" '.questions[] | select(.id == $i) | .rules[]' "$registry")
    [[ -z "$rules" ]] && continue

    # allow-bang-operator is the only "yes = disable" question; future
    # inverted questions should follow the "allow-*" naming convention.
    invert=0
    if [[ "$qid" == allow-* ]]; then
        invert=1
    fi

    if [[ $invert -eq 1 ]]; then
        [[ "$answer" == "yes" ]] && active="false" || active="true"
    else
        [[ "$answer" == "yes" ]] && active="true" || active="false"
    fi

    # Derive ruleset from the positive_fixture path:
    # tests/fixtures/positive/<ruleset>/<Rule>.kt
    ruleset=$(jq -r --arg i "$qid" \
        '.questions[] | select(.id == $i) | .positive_fixture
         | if . then (split("/") | .[3]) else "style" end' "$registry")

    while IFS= read -r rule; do
        [[ -z "$rule" ]] && continue
        printf '%s\t%s\t%s\n' "$ruleset" "$rule" "$active" >>"$raw_overrides"
    done <<<"$rules"
done <"$answers_file"

override_count=$(wc -l <"$raw_overrides" | tr -d ' ')

# Emit a single block per ruleset, sorted by ruleset name then rule.
overrides_file="$tmpdir/overrides.yml"
: >"$overrides_file"
if [[ $override_count -gt 0 ]]; then
    sort -k1,1 -k2,2 "$raw_overrides" | awk -F'\t' '
        {
            if ($1 != prev) {
                if (prev != "") print ""
                printf "%s:\n", $1
                prev = $1
            }
            printf "  %s:\n", $2
            printf "    active: %s\n", $3
        }
    ' >"$overrides_file"
fi

# Deep-merge the profile template with the overrides using yq. This
# produces a single well-formed YAML document with no duplicate keys.
if [[ $override_count -gt 0 ]]; then
    yq eval-all '. as $item ireduce ({}; . * $item)' \
        "$profiles_dir/$selected.yml" "$overrides_file" >"$target_config"
else
    cp "$profiles_dir/$selected.yml" "$target_config"
fi

# Prepend a header comment so the file documents its origin.
{
    printf '# Generated by scripts/krit-init.sh\n'
    printf '# Profile: %s\n' "$selected"
    printf '# Overrides applied: %d\n' "$override_count"
    printf '# Edit this file to change rule state; krit merges it on top\n'
    printf '# of config/default-krit.yml.\n\n'
    cat "$target_config"
} >"$target_config.tmp" && mv "$target_config.tmp" "$target_config"

gum style --foreground 82 "Wrote $target_config"
gum style "  based on: $selected profile"
gum style "  rule overrides: $override_count"

# Validate the merged config is loadable.
if ! "$krit_bin" --validate-config --config "$target_config" >/dev/null 2>"$tmpdir/validate.log"; then
    echo "error: generated config failed validation; see $tmpdir/validate.log" >&2
    cat "$tmpdir/validate.log" >&2
    exit 2
fi
gum style --faint "  validated OK"
echo

# ---------- step 6: autofix pass -----------------------------------------

gum style --bold "Autofix"

if [[ $accept_defaults -eq 1 ]] || gum confirm "Apply safe autofixes now?" --default=yes; then
    # Use the merged krit.yml + the selected profile's finding count as
    # the pre-fix baseline. Re-scan with the merged config first so the
    # "before" number reflects the actual overrides the user chose.
    gum spin --title "Scanning with your new config..." -- \
        sh -c "\"$krit_bin\" --config \"$target_config\" -f json \"$target\" >\"$tmpdir/prefix.json\" 2>\"$tmpdir/prefix.log\" || true"
    prefix_total=$(jq -r '.summary.total // 0' "$tmpdir/prefix.json")

    # krit --fix mutates files in place and does not emit JSON, so we
    # invoke it for its side effect then re-scan to count remainders.
    gum spin --title "Applying safe autofixes..." -- \
        sh -c "\"$krit_bin\" --config \"$target_config\" --fix \"$target\" >/dev/null 2>\"$tmpdir/fix.log\" || true"
    gum spin --title "Counting remaining findings..." -- \
        sh -c "\"$krit_bin\" --config \"$target_config\" -f json \"$target\" >\"$tmpdir/postfix.json\" 2>\"$tmpdir/postfix.log\" || true"

    if ! jq -e '.summary.total' "$tmpdir/postfix.json" >/dev/null 2>&1; then
        echo "error: post-fix scan produced no valid JSON output" >&2
        echo "       log: $tmpdir/postfix.log" >&2
        exit 2
    fi

    postfix_total=$(jq -r '.summary.total // 0' "$tmpdir/postfix.json")
    fixed=$((prefix_total - postfix_total))
    if [[ $fixed -lt 0 ]]; then fixed=0; fi

    gum style --foreground 82 "Fixed $fixed findings"
    gum style "  remaining: $postfix_total"

    # Top 5 fixed rules by delta.
    jq -r --slurpfile pre "$tmpdir/prefix.json" \
        '.summary.byRule as $post
         | ($pre[0].summary.byRule // {}) as $preRule
         | (($preRule | keys) + ($post | keys))
         | unique
         | map({rule: ., delta: (($preRule[.] // 0) - ($post[.] // 0))})
         | map(select(.delta > 0))
         | sort_by(-.delta)
         | .[0:5]
         | map("  - \(.rule): -\(.delta)")
         | .[]' "$tmpdir/postfix.json" 2>/dev/null || true
else
    gum style --faint "skipped autofix"
    # Populate postfix.json so step 7 can still print a count.
    cp "$tmpdir/$selected.json" "$tmpdir/postfix.json"
fi
echo

# ---------- step 7: baseline ---------------------------------------------

gum style --bold "Baseline"

baseline_dir="$target/.krit"
baseline_file="$baseline_dir/baseline.xml"

if [[ $accept_defaults -eq 1 ]] || gum confirm "Write a baseline so only new findings are flagged going forward?" --default=yes; then
    mkdir -p "$baseline_dir"
    gum spin --title "Writing baseline..." -- \
        sh -c "\"$krit_bin\" --config \"$target_config\" --create-baseline \"$baseline_file\" \"$target\" >/dev/null 2>\"$tmpdir/baseline.log\" || true"

    if [[ ! -f "$baseline_file" ]]; then
        echo "error: baseline was not written to $baseline_file" >&2
        echo "       log: $tmpdir/baseline.log" >&2
        exit 2
    fi

    suppressed=$(jq -r '.summary.total // 0' "$tmpdir/postfix.json")
    gum style --foreground 82 "Baseline written to $baseline_file"
    gum style "  existing findings suppressed: $suppressed"
    gum style "  new findings from this point forward will be flagged"
else
    gum style --faint "skipped baseline"
fi
echo

# ---------- done ----------------------------------------------------------

gum style --bold --foreground 212 "Onboarding complete."
gum style "  config:   $target_config"
if [[ -f "$baseline_file" ]]; then
    gum style "  baseline: $baseline_file"
fi
echo
gum style --bold "Next steps:"
gum style "  git add ${target_config#$target/} ${baseline_file#$target/}"
gum style "  git commit -m 'chore: configure krit'"
