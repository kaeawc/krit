# TestPathConfigSchema

**Cluster:** [rule-quality](README.md) · **Status:** planned ·
**Severity:** n/a (infra)

## What it is

The YAML schema for `testSourcePaths` in `krit.yml` and its
integration with the JSON Schema generator in `internal/schema/`.

## Shape

```yaml
# krit.yml
testSourcePaths:
  - "/src/checks/"
  - "/src/verify/"
  - "/testing/"
  - "/src/smokeTest/"
```

Each entry is a substring match against the file path (same
semantics as the current hardcoded patterns). Entries are additive —
they extend the defaults, not replace them. To replace, use:

```yaml
testSourcePathsOverride:
  - "/test/"
  - "/src/checks/"
```

`testSourcePathsOverride` replaces the entire default list. Only
one of `testSourcePaths` (additive) or `testSourcePathsOverride`
(replace) should be set. If both are set, `Override` wins.

## JSON Schema

Add to `internal/schema/schema.go` so `krit --schema` includes
the new fields and editors get autocompletion.

## Links

- Parent: [`roadmap/67-rule-quality.md`](../../67-rule-quality.md)
- Depends on: [`configurable-test-paths.md`](configurable-test-paths.md)
