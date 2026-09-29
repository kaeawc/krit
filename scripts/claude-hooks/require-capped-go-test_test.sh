#!/usr/bin/env bash
set -euo pipefail

hook="$(dirname "$0")/require-capped-go-test.sh"
passed=0
failed=0

# command<TAB>expected. The here-doc preserves spaces, quotes, and backslashes
# in command text; jq --arg safely creates the hook's JSON input payload.
# The || right side may not run, and its ulimit must not protect the later
# semicolon-separated test. Treat the || branch as unable to establish a cap.
# Deliberate limitation: nested shell code in a quoted bash -c argument is
# not recursively tokenized, so the outer command remains allowed.
while IFS= read -r row; do
	[ -n "$row" ] || continue
	expected=${row%%$'\t'*}
	command=${row#*$'\t'}
	input=$(jq -cn --arg command "$command" '{tool_input:{command:$command}}')
	if output=$(printf '%s\n' "$input" | bash "$hook"); then
		rc=0
	else
		rc=$?
	fi
	case "$expected" in
		ALLOW)
			if [ "$rc" -eq 0 ] && [ -z "$output" ]; then
				printf 'PASS  ALLOW  %s\n' "${command//$'\n'/\\n}"
				passed=$((passed + 1))
			else
				printf 'FAIL  ALLOW  %s (exit=%s stdout=%s)\n' "${command//$'\n'/\\n}" "$rc" "$output"
				failed=$((failed + 1))
			fi
			;;
		DENY)
			if [ "$rc" -eq 0 ] && printf '%s' "$output" | jq -e '.hookSpecificOutput.permissionDecision == "deny"' >/dev/null 2>&1; then
				printf 'PASS  DENY   %s\n' "${command//$'\n'/\\n}"
				passed=$((passed + 1))
			else
				printf 'FAIL  DENY   %s (exit=%s stdout=%s)\n' "${command//$'\n'/\\n}" "$rc" "$output"
				failed=$((failed + 1))
			fi
			;;
	esac
done <<'CASES'
DENY	go test ./...
DENY	go test ./...; echo scripts/go-test.sh
DENY	go test ./... # ulimit -u later
DENY	echo hi && go test ./x
DENY	(go test ./...)
DENY	FOO=1 go test ./x
DENY	timeout 60 go test ./x
DENY	go test ./x | tee log
DENY	ulimit -u 100 | go test ./x
DENY	true || ulimit -u 100; go test ./x
ALLOW	bash -c 'go test ./...'
ALLOW	scripts/go-test.sh ./internal/rules -run X
ALLOW	./scripts/go-test.sh ./...
ALLOW	make test
ALLOW	ulimit -u 4096; go test ./x
ALLOW	ulimit -u 4096 && go test ./x
ALLOW	cd tools && ulimit -u 100 && go test ./x
ALLOW	git commit -m 'run go test ./... later'
ALLOW	grep 'go test' docs
ALLOW	echo "go test"
ALLOW	
DENY	go test ./...; echo "ulimit -u"
ALLOW	echo hi # go test ./y is only comment text
ALLOW	echo hi \; go test ./x
ALLOW	echo hi; echo "; go test ./y"
DENY	ulimit -u 100; (true); go test ./y
ALLOW	go test ./x 'unbalanced
CASES

# A tool_input with no command field is an explicit allow case too.
if output=$(printf '%s\n' '{"tool_input":{}}' | bash "$hook"); then
	rc=0
else
	rc=$?
fi
if [ "$rc" -eq 0 ] && [ -z "$output" ]; then
	printf 'PASS  ALLOW  missing command field\n'
	passed=$((passed + 1))
else
	printf 'FAIL  ALLOW  missing command field (exit=%s stdout=%s)\n' "$rc" "$output"
	failed=$((failed + 1))
fi

printf '\n%d passed, %d failed\n' "$passed" "$failed"
[ "$failed" -eq 0 ]
