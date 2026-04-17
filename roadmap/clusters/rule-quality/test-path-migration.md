# TestPathMigration

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (refactor)

## What it is

Move `isTestFile` from `internal/rules/naming.go` to
`internal/scanner/testpath.go` and update all 22 callers.

## Steps

1. Create `internal/scanner/testpath.go` with:
   - `var defaultTestPaths = [...]string{...}` (current list)
   - `func InitTestPaths(config []string, override []string)`
   - `func IsTestFile(path string) bool`
2. Call `scanner.InitTestPaths(cfg.TestSourcePaths, cfg.TestSourcePathsOverride)`
   in `cmd/krit/main.go` after config loading.
3. Replace all 22 `isTestFile(file.Path)` calls with
   `scanner.IsTestFile(file.Path)`.
4. Delete the old `isTestFile` from `naming.go`.
5. Update `internal/rules/naming_test.go` tests that exercise
   `isTestFile` to use the new location.

## Acceptance criteria

- All existing tests pass.
- `krit --rule-audit` on the 6-repo set shows identical results
  before and after the migration.
- A project with `testSourcePaths: ["/src/checks/"]` in its
  `krit.yml` correctly skips those paths.

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
- Depends on: [`configurable-test-paths.md`](configurable-test-paths.md),
  [`test-path-config-schema.md`](test-path-config-schema.md)
