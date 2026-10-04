#!/usr/bin/env bash
# Claude Code PreToolUse hook (Bash): refuse an uncapped `go test`.
#
# An uncapped `go test ./...` once filled the machine's process table (a
# test binary re-launched itself without bound). Tests must run under
# a process cap: `make test`, `scripts/go-test.sh <args>`, or a command
# that sets `ulimit -u` first. Only go test commands in shell command
# position are matched, so commit messages and greps that mention it pass.
#
# This intentionally tokenizes only the outer command text. It does not
# recursively parse shell code passed as a quoted argument to `bash -c` or
# `sh -c`; those nested commands are outside this guardrail's scope.
set -euo pipefail

cmd=$(jq -r '.tool_input.command // empty')
[ -n "$cmd" ] || exit 0

# Classify one unquoted command segment. The prefix accepts simple
# environment assignments, then the wrappers supported by the original hook.
is_ulimit_segment() {
	local segment=$1
	local assign='([A-Za-z_][A-Za-z0-9_]*=[^[:space:]]+[[:space:]]+)*'
	local command='(env|time|nice|timeout[[:space:]]+[0-9]+[smh]?)[[:space:]]+'
	local ulimit='ulimit[[:space:]]+-([u]|[SH]u)[[:space:]]+[0-9]+([[:space:]]|$)'
	printf '%s\n' "$segment" | grep -Eiq "^[[:space:]]*${assign}(${command})*${assign}${ulimit}"
}

is_go_test_segment() {
	local segment=$1
	local assign='([A-Za-z_][A-Za-z0-9_]*=[^[:space:]]+[[:space:]]+)*'
	local command='(env|time|nice|timeout[[:space:]]+[0-9]+[smh]?)[[:space:]]+'
	local go_test='go[[:space:]]+test([[:space:]]|$)'
	printf '%s\n' "$segment" | grep -Eiq "^[[:space:]]*${assign}(${command})*${assign}${go_test}"
}

has_uncapped_test=0
chain_capped=0
after_or=0
segment=''
quote=''
escaped=0
word_start=1
parse_error=0

# A separator flushes the current segment. Semicolon, &&, and a real newline
# retain the cap in the current chain. Pipes, ||, &, and group/subshell
# boundaries reset it. Group boundaries also reset on entry and exit, so
# caps stay local to their current group; nested groups are handled with the
# same conservative reset rule. A ulimit in an || right-hand branch is not
# trusted to cap commands after that branch (which may not execute).
flush_segment() {
	if is_ulimit_segment "$segment"; then
		if [ "$after_or" -eq 0 ]; then
			chain_capped=1
		fi
	elif is_go_test_segment "$segment"; then
		if [ "$chain_capped" -eq 0 ]; then
			has_uncapped_test=1
		fi
	fi
	segment=''
}

reset_chain() {
	chain_capped=0
}

length=${#cmd}
i=0
while [ "$i" -lt "$length" ]; do
	ch=${cmd:$i:1}
	next=${cmd:$((i + 1)):1}

	if [ "$escaped" -eq 1 ]; then
		segment="$segment$ch"
		escaped=0
		word_start=0
		i=$((i + 1))
		continue
	fi

	if [ -n "$quote" ]; then
		segment="$segment$ch"
		if [ "$quote" = "'" ]; then
			[ "$ch" = "'" ] && quote=''
		else
			if [ "$ch" = $'\\' ]; then
				escaped=1
			elif [ "$ch" = '"' ]; then
				quote=''
			fi
		fi
		word_start=0
		i=$((i + 1))
		continue
	fi

	if [ "$ch" = "'" ] || [ "$ch" = '"' ]; then
		quote=$ch
		segment="$segment$ch"
		word_start=0
		i=$((i + 1))
		continue
	fi
	if [ "$ch" = $'\\' ]; then
		segment="$segment$ch"
		escaped=1
		word_start=0
		i=$((i + 1))
		continue
	fi

	# A comment starts only when # begins a new shell word. Its terminating
	# newline is still a sequential-chain separator.
	if [ "$ch" = '#' ] && [ "$word_start" -eq 1 ]; then
		while [ "$i" -lt "$length" ] && [ "${cmd:$i:1}" != $'\n' ]; do
			i=$((i + 1))
		done
		if [ "$i" -lt "$length" ]; then
			flush_segment
			after_or=0
			i=$((i + 1))
		fi
		word_start=1
		continue
	fi

	if [ "$ch" = ';' ] || [ "$ch" = '&' ] || [ "$ch" = '|' ]; then
		flush_segment
		if [ "$ch" = ';' ]; then
			after_or=0
		elif [ "$ch" = '&' ] && [ "$next" = '&' ]; then
			after_or=0
		elif [ "$ch" = '|' ] && [ "$next" = '|' ]; then
			reset_chain
			after_or=1
		else
			reset_chain
			after_or=0
		fi
		if { [ "$ch" = '&' ] || [ "$ch" = '|' ]; } && { [ "$next" = "$ch" ]; }; then
			i=$((i + 2))
		else
			i=$((i + 1))
		fi
		word_start=1
		continue
	fi
	if [ "$ch" = '(' ] || [ "$ch" = ')' ] || [ "$ch" = '{' ] || [ "$ch" = '}' ]; then
		flush_segment
		reset_chain
		after_or=0
		word_start=1
		i=$((i + 1))
		continue
	fi
	if [ "$ch" = $'\n' ]; then
		flush_segment
		after_or=0
		word_start=1
		i=$((i + 1))
		continue
	fi
	if [ "$ch" = ' ' ] || [ "$ch" = $'\t' ] || [ "$ch" = $'\r' ]; then
		segment="$segment$ch"
		word_start=1
	else
		segment="$segment$ch"
		word_start=0
	fi
	i=$((i + 1))
done

# Malformed quoting/escaping is allowed through, as with other unrecognized
# commands. Quoted separators were kept inside their original segment.
if [ -n "$quote" ] || [ "$escaped" -eq 1 ]; then
	parse_error=1
fi
if [ "$parse_error" -eq 0 ]; then
	flush_segment
	else
	# Parsing is intentionally fail-open for an unbalanced quote/escape.
	has_uncapped_test=0
fi

[ "$has_uncapped_test" -eq 1 ] || exit 0

jq -n '{hookSpecificOutput: {
  hookEventName: "PreToolUse",
  permissionDecision: "deny",
  permissionDecisionReason: "Uncapped `go test` is blocked in krit: a runaway test once filled the process table. Rerun it through the process-capped wrapper with the same arguments, e.g. `scripts/go-test.sh ./internal/rules -run TestFoo -count=1`, or `make test` for the full suite."
}}'
