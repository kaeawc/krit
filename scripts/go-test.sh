#!/usr/bin/env bash
# Run `go test` under a process cap so a runaway spawn (a test binary
# re-launching itself, leaked daemons or JVMs) fails with fork errors
# instead of filling the machine's process table.
#
# On Linux, RLIMIT_NPROC is ignored for root and processes with
# CAP_SYS_ADMIN or CAP_SYS_RESOURCE. Those runs prefer a systemd scope with
# TasksMax, then a writable cgroup v2 child with pids.max. If neither works,
# the test continues with one warning unless KRIT_TEST_REQUIRE_CAP=1.
#
# `ulimit -u` counts every process the user owns, not just this run's,
# so the cap is the user's current process count plus a budget. Both the
# soft and hard limits are lowered, so nothing under this run can raise
# them back.
#
#   KRIT_TEST_PROC_BUDGET  extra processes allowed for this run (default 2048)
#   KRIT_TEST_PARALLEL     packages tested in parallel, go test -p (default 4)
#   KRIT_TEST_REQUIRE_CAP  fail if a privileged Linux process cannot be capped
#   KRIT_GO_TEST_DRY_RUN   print selected strategy and command without side effects
#
# Arguments are passed to `go test` verbatim; a later -p overrides the default.
set -euo pipefail

is_linux_privileged() {
	local os uid status_file cap_hex cap_value
	os=${1:-$(uname -s)}
	uid=${2:-$(id -u)}
	status_file=${3:-/proc/self/status}
	[ "$os" = Linux ] || return 1
	if [ "$uid" = 0 ]; then
		return 0
	fi
	[ -r "$status_file" ] || return 1
	cap_hex=$(sed -n 's/^CapEff:[[:space:]]*//p' "$status_file" | head -n 1)
	[ -n "$cap_hex" ] || return 1
	cap_value=$((16#$cap_hex))
	(( (cap_value & ((1 << 21) | (1 << 24))) != 0 ))
}

systemd_run_usable() {
	local uid
	uid=${1:-$(id -u)}
	command -v systemd-run >/dev/null 2>&1 || return 1
	if [ "$uid" = 0 ]; then
		systemd-run --scope --quiet -p TasksMax=100 -- true >/dev/null 2>&1
	else
		systemd-run --user --scope --quiet -p TasksMax=100 -- true >/dev/null 2>&1
	fi
}

cgroup_v2_parent() {
	local cgroup_file mount_root cgroup_path
	cgroup_file=${1:-/proc/self/cgroup}
	mount_root=${2:-/sys/fs/cgroup}
	[ -r "$cgroup_file" ] || return 1
	cgroup_path=$(sed -n 's/^0:://p' "$cgroup_file" | head -n 1)
	[ -n "$cgroup_path" ] || return 1
	case "$cgroup_path" in
		/*) ;;
		*) return 1 ;;
	esac
	printf '%s%s\n' "${mount_root%/}" "$cgroup_path"
}

cgroup_v2_usable() {
	local parent
	parent=$(cgroup_v2_parent "${1:-/proc/self/cgroup}" "${2:-/sys/fs/cgroup}") || return 1
	[ -d "$parent" ] && [ -w "$parent" ] &&	[ -f "$parent/pids.max" ] && [ -w "$parent/pids.max" ] &&	[ -f "$parent/cgroup.procs" ] && [ -w "$parent/cgroup.procs" ]
}

configure_cgroup_child() {
	local child budget
	child=$1
	budget=$2
	mkdir "$child" || return 1
	printf '%s' "$budget" > "$child/pids.max" || return 1
	printf '%s' "$$" > "$child/cgroup.procs" || return 1
}

select_strategy() {
	local os uid status_file cgroup_file cgroup_root
	os=${1:-$(uname -s)}
	uid=${2:-$(id -u)}
	status_file=${3:-/proc/self/status}
	cgroup_file=${4:-/proc/self/cgroup}
	cgroup_root=${5:-/sys/fs/cgroup}
	STRATEGY='ulimit'
	STRATEGY_REASON=
	CGROUP_PARENT=
	if ! is_linux_privileged "$os" "$uid" "$status_file"; then
		return 0
	fi
	if [ "${KRIT_GO_TEST_DRY_RUN:-0}" = 1 ]; then
		# A dry run cannot execute systemd-run's usability probe. Presence on
		# PATH selects the command for inspection; real runs always probe it.
		if command -v systemd-run >/dev/null 2>&1; then
			STRATEGY=systemd-run
			return 0
		fi
		STRATEGY_REASON='systemd-run not found or non-functional'
	else
		if systemd_run_usable "$uid"; then
			STRATEGY=systemd-run
			return 0
		fi
		if command -v systemd-run >/dev/null 2>&1; then
			STRATEGY_REASON='systemd-run non-functional'
		else
			STRATEGY_REASON='systemd-run not found'
		fi
	fi
	if CGROUP_PARENT=$(cgroup_v2_parent "$cgroup_file" "$cgroup_root") && cgroup_v2_usable "$cgroup_file" "$cgroup_root"; then
		STRATEGY=cgroup-v2
		return 0
	fi
	if [ -n "$STRATEGY_REASON" ]; then
		STRATEGY_REASON="$STRATEGY_REASON; cgroup v2 not mounted or not writable/usable"
	else
		STRATEGY_REASON='cgroup v2 not mounted or not writable/usable'
	fi
	if [ "${KRIT_TEST_REQUIRE_CAP:-0}" = 1 ]; then
		STRATEGY=unenforced-fail
	else
		STRATEGY=unenforced-warning
	fi
}

print_unenforced_warning() {
	printf 'WARNING: process cap cannot be enforced while running as root or with CAP_SYS_ADMIN/CAP_SYS_RESOURCE (%s).\n' "$1" >&2
}

go_test_command() {
	local parallel
	parallel=${KRIT_TEST_PARALLEL:-4}
	printf 'go test -p %s' "$parallel"
	printf ' %q' "$@"
	printf '\n'
}

run_go_test() {
	local budget parallel current limit hard uid strategy child
	budget=${KRIT_TEST_PROC_BUDGET:-2048}
	parallel=${KRIT_TEST_PARALLEL:-4}
	uid=$(id -u)
	select_strategy "$(uname -s)" "$uid"
	strategy=$STRATEGY
	if [ "${KRIT_GO_TEST_DRY_RUN:-0}" = 1 ]; then
		printf '%s\n' "$strategy"
		case "$strategy" in
			systemd-run)
				if [ "$uid" = 0 ]; then
					printf 'systemd-run --scope --quiet -p TasksMax=%s -- go test -p %s' "$budget" "$parallel"
				else
					printf 'systemd-run --user --scope --quiet -p TasksMax=%s -- go test -p %s' "$budget" "$parallel"
				fi
				printf ' %q' "$@"; printf '\n'
				return 0
				;;
			cgroup-v2)
				child="$CGROUP_PARENT/krit-go-test-$$"
				printf 'mkdir %q; printf %q > %q/pids.max; printf %q > %q/cgroup.procs; exec go test -p %s' "$child" "$budget" "$child" "$$" "$child" "$parallel"
				printf ' %q' "$@"; printf '\n'
				return 0
				;;
			unenforced-warning|unenforced-fail)
				print_unenforced_warning "$STRATEGY_REASON"
				if [ "$strategy" = unenforced-fail ]; then
					return 1
				fi
				go_test_command "$@"
				return 0
				;;
			ulimit)
				# shellcheck disable=SC2016 # Print the literal command for dry-run inspection.
				printf 'current=$(ps -u "$(id -u)" -o pid= | wc -l | tr -d " "); limit=$((current + %s)); hard=$(ulimit -H -u); if [ "$hard" != "unlimited" ] && [ "$limit" -gt "$hard" ]; then limit=$hard; fi; ulimit -u "$limit"; exec ' "$budget"
				go_test_command "$@"
				return 0
				;;
		esac
	fi
	case "$strategy" in
		systemd-run)
			if [ "$uid" = 0 ]; then
				exec systemd-run --scope --quiet -p "TasksMax=$budget" -- go test -p "$parallel" "$@"
			else
				exec systemd-run --user --scope --quiet -p "TasksMax=$budget" -- go test -p "$parallel" "$@"
			fi
			;;
		cgroup-v2)
			child="$CGROUP_PARENT/krit-go-test-$$"
			if ! configure_cgroup_child "$child" "$budget"; then
				STRATEGY_REASON="${STRATEGY_REASON}; cgroup v2 setup failed (directory creation or controller write failed)"
				print_unenforced_warning "$STRATEGY_REASON"
				if [ "${KRIT_TEST_REQUIRE_CAP:-0}" = 1 ]; then
					return 1
				fi
				exec go test -p "$parallel" "$@"
			fi
			# exec keeps signal and file descriptor behavior transparent. The
			# empty child cgroup cannot be cleaned up afterward; the OS/systemd
			# or a human/cron can reap the directory later.
			exec go test -p "$parallel" "$@"
			;;
		unenforced-warning)
			print_unenforced_warning "$STRATEGY_REASON"
			exec go test -p "$parallel" "$@"
			;;
		unenforced-fail)
			print_unenforced_warning "$STRATEGY_REASON"
			return 1
			;;
		ulimit)
			# Preserve the original ulimit path and arithmetic for non-privileged
			# Linux and non-Linux hosts.
			current=$(ps -u "$(id -u)" -o pid= | wc -l | tr -d ' ')
			limit=$((current + budget))
			hard=$(ulimit -H -u)
			if [ "$hard" != unlimited ] && [ "$limit" -gt "$hard" ]; then
				limit=$hard
			fi
			ulimit -u "$limit"
			exec go test -p "$parallel" "$@"
			;;
	esac
}

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
	run_go_test "$@"
fi
