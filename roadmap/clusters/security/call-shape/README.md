# Security — call-shape heuristics (tier 2)

Parent: [`roadmap/50-security-rules-call-shape.md`](../../../50-security-rules-call-shape.md)

Opt-in advisors keyed on the *shape* of an argument expression: literal
vs interpolation vs concat. All ship `DefaultInactive` behind
experiment flags; messages use "Review" / "Consider" wording.

## SQL shape

- [`sql-injection-raw-query.md`](sql-injection-raw-query.md)
- [`room-raw-query-string-concat.md`](room-raw-query-string-concat.md)
- [`jdbc-statement-execute.md`](jdbc-statement-execute.md)

## Shell / process

- [`runtime-exec-unsafe-shape.md`](runtime-exec-unsafe-shape.md)
- [`process-builder-shell-arg.md`](process-builder-shell-arg.md)

## File / path

- [`file-from-untrusted-path.md`](file-from-untrusted-path.md)
- [`zip-slip-unchecked.md`](zip-slip-unchecked.md)
- [`temp-file-world-readable.md`](temp-file-world-readable.md)

## Logging sensitive data

- [`log-pii.md`](log-pii.md)
- [`print-stack-trace-in-release.md`](print-stack-trace-in-release.md)

## Secret patterns

- [`hardcoded-aws-access-key.md`](hardcoded-aws-access-key.md)
- [`hardcoded-gcp-service-account.md`](hardcoded-gcp-service-account.md)
- [`hardcoded-jwt.md`](hardcoded-jwt.md)
- [`hardcoded-bearer-token.md`](hardcoded-bearer-token.md)
- [`hardcoded-slack-webhook.md`](hardcoded-slack-webhook.md)
- [`google-api-key-in-resources.md`](google-api-key-in-resources.md)

## Broadcast / content provider

- [`unprotected-dynamic-receiver.md`](unprotected-dynamic-receiver.md)
- [`content-provider-query-with-selection-interpolation.md`](content-provider-query-with-selection-interpolation.md)
