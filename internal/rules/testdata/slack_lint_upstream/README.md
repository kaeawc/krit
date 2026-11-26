# slack-lints upstream parity cases

`TestSlackLintUpstreamParity` (`internal/rules/thirdparty_mocking_test.go`) checks the slack-lints 0.11.1 ports whose
detectors only run on test sources. The Lint CLI cannot mark test sources in a directory scan, and its `--project`
descriptor mode does not analyze sources at all in Lint 9.4.0-rc01. So these cases are built from slack-lints' own
detector tests, whose expected output was produced by the real detector under Lint's test harness.

Each directory is one `@Test` method: its files (`test/...` paths moved under `src/test/java/`, unnamed files placed by
package), `checks.txt` (the Lint IDs it checks) and `upstream-0.11.1.txt` (the test's expected findings for those IDs,
with the column read from Lint's `~~~` underline). `divergences.txt` lists deliberate differences, each with a reason.

## Regenerating

```bash
curl -fsSL https://github.com/slackhq/slack-lints/archive/refs/tags/0.11.1.tar.gz | tar xz
T=slack-lints-0.11.1/slack-lint-checks/src/test/java/slack/lint/mocking
python3 -I internal/rules/testdata/slack_lint_upstream/convert.py "$T" internal/rules/testdata/slack_lint_upstream \
  DoNotMockDataClass,DoNotMockObjectClass,DoNotMockPlatformTypes \
  DataClassMockDetectorTest.kt ObjectClassMockDetectorTest.kt PlatformTypeMockDetectorTest.kt
```

The converter rewrites `checks.txt` and `upstream-0.11.1.txt` and leaves `divergences.txt` alone.
