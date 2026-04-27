# Configuration

Krit reads `krit.yml` or `.krit.yml` from the project root. The format is compatible with detekt — copy your `detekt.yml` and rename it.

## Resolution order

1. `--config FILE` flag
2. `krit.yml` in the working directory
3. `.krit.yml` in the working directory
4. Built-in defaults

## Example

```yaml
complexity:
  LongMethod:
    active: true
    threshold: 120
  NestedBlockDepth:
    threshold: 6

naming:
  FunctionNaming:
    ignoreAnnotated: ['Composable']

style:
  MagicNumber:
    ignoreAnnotated: ['Preview']
    ignoreNumbers: ['-1', '0', '1', '2']
  ReturnCount:
    max: 4
```

## Enabling and disabling rules

```yaml
style:
  WildcardImport:
    active: true
  ForbiddenComment:
    active: false
```

Run everything at once (including opt-in rules):

```bash
krit --all-rules .
```

## Ignore patterns

```yaml
naming:
  FunctionNaming:
    ignoreAnnotated: ['Composable', 'Test']
    excludes: ['**/test/**']
```

## Android policy rules

Android-specific thresholds also live in the same file:

```yaml
android-lint:
  OldTargetApi:
    threshold: 29
  MinSdkTooLow:
    threshold: 16
  NewerVersionAvailable:
    recommendedVersions:
      - "androidx.appcompat:appcompat=1.5.0"
```

Unset values fall back to built-in defaults.

## Baselines

Freeze current findings, catch regressions:

```bash
krit --create-baseline baseline.xml .
krit --baseline baseline.xml .
```

## Experimental performance knobs

Cold type-oracle cache misses use one-shot `krit-types` analysis with `--experimental-parallel-files 4` by default. Override the worker count with `KRIT_TYPES_PARALLEL_FILES=N`; set it to `0` or `1` to disable in-JVM file parallelism. Set `KRIT_DAEMON_CACHE=on` to use the persistent daemon miss-analysis path instead.

Kotlin compiler diagnostics are disabled in the default type-oracle path because `collectDiagnostics()` is expensive on large projects and most type-aware rules use FlatNode or expression facts instead. Pass `--oracle-diagnostics` when you need diagnostic-backed oracle findings such as compiler-proven unreachable code or impossible casts.

`KRIT_DAEMON_POOL=N` is an opt-in benchmark knob for warm type-oracle cache misses. The default is `1`. Values greater than `1` keep additional persistent Kotlin Analysis API JVM daemons for the same source tree and shard miss analysis only for larger miss sets.

Each pool member is a full JVM with its own Analysis API session, so idle memory use scales roughly with the pool size. Use this only while measuring warm edit runs, not as a default project setting.

## Migrating from detekt

Krit reads detekt's format natively:

```bash
krit --config detekt.yml .
```

Overlapping rules keep the same names and compatible keys, but behavior isn't guaranteed identical in every edge case — sanity-check a few files after migration.
