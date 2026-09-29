#!/usr/bin/env bash
# Claude Code PreToolUse hook (Bash): refuse an uncapped `go test`.
#
# An uncapped `go test ./...` once filled the machine's process table (a
# test binary re-launched itself without bound). Tests must run under
# a process cap: `make test`, `scripts/go-test.sh <args>`, or a command
# that sets `ulimit -u` first. Only `go test` in command position is
# matched, so commit messages or greps that mention it pass through.
set -euo pipefail

cmd=$(jq -r '.tool_input.command // empty')
[ -n "$cmd" ] || exit 0

case "$cmd" in
*scripts/go-test.sh* | *"ulimit -u"*) exit 0 ;;
esac

wrapper='((env|time|nice|timeout[[:space:]]+[0-9]+[smh]?)[[:space:]]+)?'
assign='([A-Za-z_][A-Za-z0-9_]*=[^[:space:]]*[[:space:]]+)*'
if ! printf '%s\n' "$cmd" | grep -Eq "(^|[;&|(]|then|do)[[:space:]]*${assign}${wrapper}${assign}go[[:space:]]+test([[:space:]]|\$)"; then
	exit 0
fi

jq -n '{hookSpecificOutput: {
  hookEventName: "PreToolUse",
  permissionDecision: "deny",
  permissionDecisionReason: "Uncapped `go test` is blocked in krit: a runaway test once filled the process table. Rerun it through the process-capped wrapper with the same arguments, e.g. `scripts/go-test.sh ./internal/rules -run TestFoo -count=1`, or `make test` for the full suite."
}}'
