---
applyTo: "tests/fixtures/negative/**,tests/fixtures/fixable/**"
---

These files are Krit analyzer fixtures. They intentionally contain suspicious,
non-idiomatic, unsafe, invalid, or fixable code so Krit rules and autofixes can
be tested.

For pull request code review, do not report code-quality, security,
correctness, style, or idiom issues in these files. Treat the fixture contents as
test input, not production code. Review comments should focus on the Go rule
implementation, registry metadata, tests, and positive fixtures instead.

Only comment on these fixture files when the fixture mechanics themselves are
wrong, such as an incorrect path/category, a malformed `.expected` pair, or a
change that prevents the fixture from being parsed by Krit's test harness.
