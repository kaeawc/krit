#!/usr/bin/env bash
# Source the script and test its decision functions directly. This keeps the
# suite deterministic on macOS and avoids running go test, ulimit, or cgroup writes.
set -euo pipefail

script_dir=$(cd "$(dirname "$0")" && pwd)
# shellcheck source=scripts/go-test.sh
source "$script_dir/go-test.sh"
real_select_strategy=$(declare -f select_strategy)

tmp=$(mktemp -d "${TMPDIR:-/tmp}/krit-go-test.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1" >&2; exit 1; }

status="$tmp/status"
cgroup_file="$tmp/cgroup"
cgroup_root="$tmp/sys/fs/cgroup"
fakebin="$tmp/bin"
mkdir -p "$fakebin" "$cgroup_root/test.slice"
printf '0::/test.slice\n' > "$cgroup_file"
touch "$cgroup_root/test.slice/pids.max" "$cgroup_root/test.slice/cgroup.procs"

cat > "$status" <<'EOF'
Name: bash
CapEff: 0000000000200000
EOF

cat > "$fakebin/systemd-run" <<'EOF'
#!/usr/bin/env bash
exit "${FAKE_SYSTEMD_RUN_STATUS:-0}"
EOF
chmod +x "$fakebin/systemd-run"

PATH="$fakebin:$PATH"
export PATH
FAKE_SYSTEMD_RUN_STATUS=0
export FAKE_SYSTEMD_RUN_STATUS
if ! systemd_run_usable 0; then fail 'working systemd-run probe'; fi
if ! select_strategy Linux 0 "$status" "$cgroup_file" "$cgroup_root"; then fail 'root systemd strategy selection'; fi
[ "$STRATEGY" = systemd-run ] || fail "expected systemd-run, got $STRATEGY"
export KRIT_TEST_PROC_BUDGET=37 KRIT_GO_TEST_DRY_RUN=1
select_strategy() { STRATEGY=systemd-run; STRATEGY_REASON=; }
# shellcheck disable=SC2329
id() { printf '0\n'; }
output=$(run_go_test ./pkg/example -count=1)
printf '%s\n' "$output" | grep -F 'TasksMax=37' >/dev/null || fail 'systemd dry-run TasksMax value'
printf '%s\n' "$output" | grep -F 'systemd-run --scope --quiet -p TasksMax=37 -- go test -p 4 ./pkg/example -count=1' >/dev/null || fail 'systemd dry-run command or root scope flags'
unset -f id
eval "$real_select_strategy"
unset KRIT_TEST_PROC_BUDGET KRIT_GO_TEST_DRY_RUN
pass 'root with working systemd-run selects TasksMax strategy'

rm "$fakebin/systemd-run"
if ! select_strategy Linux 0 "$status" "$cgroup_file" "$cgroup_root"; then fail 'cgroup strategy selection'; fi
[ "$STRATEGY" = cgroup-v2 ] || fail "expected cgroup-v2, got $STRATEGY"
export KRIT_TEST_PROC_BUDGET=37
export KRIT_GO_TEST_DRY_RUN=1
select_strategy() { STRATEGY=cgroup-v2; STRATEGY_REASON=; CGROUP_PARENT="$cgroup_root/test.slice"; : "$CGROUP_PARENT"; }
output=$(run_go_test ./pkg/example -count=1)
printf '%s\n' "$output" | grep -F 'printf 37' >/dev/null || fail 'cgroup dry-run pids.max value'
printf '%s\n' "$output" | grep -F 'go test -p 4 ./pkg/example -count=1' >/dev/null || fail 'cgroup dry-run go test command'
pass 'root without systemd-run selects writable cgroup v2 with the budget'
eval "$real_select_strategy"

cat > "$fakebin/systemd-run" <<'EOF'
#!/usr/bin/env bash
exit 1
EOF
chmod +x "$fakebin/systemd-run"
rm -rf "$cgroup_root/test.slice"
unset KRIT_GO_TEST_DRY_RUN
if ! select_strategy Linux 0 "$status" "$cgroup_file" "$cgroup_root"; then fail 'unenforced warning selection'; fi
[ "$STRATEGY" = unenforced-warning ] || fail "expected warning strategy, got $STRATEGY"
# Force only the host-dependent inputs so this end-to-end branch is stable on
# macOS; selection logic itself remains the real function above.
# shellcheck disable=SC2329
uname() { printf 'Linux\n'; }
# shellcheck disable=SC2329
id() { printf '0\n'; }
# shellcheck disable=SC2034
select_strategy() {
	STRATEGY_REASON='systemd-run not found; cgroup v2 not mounted or not writable/usable'
	if [ "${KRIT_TEST_REQUIRE_CAP:-0}" = 1 ]; then STRATEGY=unenforced-fail; else STRATEGY=unenforced-warning; fi
}
export KRIT_GO_TEST_DRY_RUN=1
stderr="$tmp/warning"
if ! output=$(run_go_test ./pkg/example 2>"$stderr"); then fail 'unenforced default should continue'; fi
grep -F 'WARNING: process cap cannot be enforced while running as root or with CAP_SYS_ADMIN/CAP_SYS_RESOURCE' "$stderr" >/dev/null || fail 'warning text'
grep -F 'go test -p 4 ./pkg/example' <<< "$output" >/dev/null || fail 'warning branch go test command'
pass 'root without cgroup/systemd warns and continues by default'

export KRIT_TEST_REQUIRE_CAP=1
select_strategy
[ "${STRATEGY:-}" = unenforced-fail ] || fail 'required-cap strategy should be failure'
if output=$(run_go_test ./pkg/example 2>"$stderr"); then fail 'required-cap should exit 1'; fi
grep -F 'WARNING: process cap cannot be enforced while running as root or with CAP_SYS_ADMIN/CAP_SYS_RESOURCE' "$stderr" >/dev/null || fail 'required-cap warning text'
pass 'root without cgroup/systemd fails when KRIT_TEST_REQUIRE_CAP=1'
unset KRIT_TEST_REQUIRE_CAP KRIT_TEST_PROC_BUDGET KRIT_GO_TEST_DRY_RUN
unset -f uname id
eval "$real_select_strategy"
rm -f "$fakebin/systemd-run"

cat > "$status" <<'EOF'
Name: bash
CapEff: 0000000000000000
EOF
if is_linux_privileged Linux 1000 "$status"; then fail 'empty capabilities should not be privileged'; fi
cat > "$status" <<'EOF'
Name: bash
CapEff: 0000000001000000
EOF
if ! is_linux_privileged Linux 1000 "$status"; then fail 'CAP_SYS_RESOURCE bit detection'; fi
cat > "$status" <<'EOF'
Name: bash
CapEff: 0000000000200000
EOF
if ! is_linux_privileged Linux 1000 "$status"; then fail 'CAP_SYS_ADMIN bit detection'; fi
cat > "$status" <<'EOF'
Name: bash
CapEff: 0000000000000000
EOF
if ! select_strategy Linux 1000 "$status" "$cgroup_file" "$cgroup_root"; then fail 'non-root strategy selection'; fi
[ "$STRATEGY" = ulimit ] || fail "expected ulimit for non-root, got $STRATEGY"
if ! select_strategy Darwin 0 "$status" "$cgroup_file" "$cgroup_root"; then fail 'non-Linux strategy selection'; fi
[ "$STRATEGY" = ulimit ] || fail "expected ulimit for macOS, got $STRATEGY"
export KRIT_GO_TEST_DRY_RUN=1
output=$(run_go_test ./pkg/example -count=1)
printf '%s\n' "$output" | grep -F 'ulimit' >/dev/null || fail 'ulimit dry-run strategy and command'
printf '%s\n' "$output" | grep -F 'go test -p 4 ./pkg/example -count=1' >/dev/null || fail 'ulimit dry-run go test command'
pass 'non-root Linux selects ulimit regardless of cgroup/systemd; non-Linux dry-run uses ulimit'
