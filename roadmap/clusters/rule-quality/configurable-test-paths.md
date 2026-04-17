# ConfigurableTestPaths

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (infra)

## Current problem

`isTestFile(path string) bool` in `internal/rules/naming.go` has
25+ hardcoded path patterns. Projects with non-standard test layouts
(`src/checks/`, `src/verify/`, `testing/`, `src/smokeTest/`) get
no test exemptions. 22 rules depend on this function.

## Proposed fix

1. Move `isTestFile` from `internal/rules/naming.go` to
   `internal/scanner/testpath.go` (it's not a naming concern).
2. Add a `testSourcePaths` field to `internal/config/config.go`:
   ```yaml
   # krit.yml
   testSourcePaths:
     - "/src/checks/"
     - "/src/verify/"
     - "/testing/"
   ```
3. `isTestFile` reads the merged list: hardcoded defaults + config.
4. The function signature stays `isTestFile(path string) bool` — 
   rules don't change. The config is loaded once at startup and
   the merged pattern list is module-level state.

## Migration

No rule changes needed. The 22 callers keep calling `isTestFile`.
Only the function's internal implementation changes to read config.
The defaults match the current hardcoded list exactly, so behavior
is identical for projects without `testSourcePaths` in their config.

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
- 22 callers in: `style_forbidden.go`, `complexity.go`, `naming.go`,
  `potentialbugs_nullsafety.go`, `style_classes.go`,
  `android_correctness.go`, `performance.go`, etc.
