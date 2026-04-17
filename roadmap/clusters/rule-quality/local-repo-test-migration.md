# Replace Local Repo Tests with Fixtures

**Cluster:** [rule-quality](./README.md) · **Status:** planned ·
**Supersedes:** [`roadmap/43-replace-local-repo-tests.md`](../../43-replace-local-repo-tests.md)

## What it is

Several test files contain hard-coded paths to local repositories
(`/Users/jason/github/Signal-Android`, `/Users/jason/github/circuit`) with
`t.Skip()` guards. These should be replaced with self-contained fixture files
so tests work on any machine.

## Affected files

- `internal/module/discover_test.go` (lines 132-176)
- `internal/module/depparse_test.go`

## Implementation notes

- Create minimal fixture directories under `tests/fixtures/modules/` with
  representative `build.gradle.kts` and source files
- Replace hard-coded paths with relative fixture paths
- Remove `t.Skip()` guards so tests run in CI
