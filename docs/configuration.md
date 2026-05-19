# Configuration

Krit reads `krit.yml` or `.krit.yml` from the project root. It accepts the same rule-set/rule/key layout used by common Kotlin analyzer YAML configs, so migration usually starts by copying an existing config and renaming it.

## Resolution order

1. `--config FILE` flag (explicit override).
2. `krit.yml` or `.krit.yml`, found by walking *upward* from the analyzed
   directory. The closest-to-source file wins; the walk stops at the
   worktree root (`.git` / `.hg` / `.jj` marker, file or directory), at
   `$HOME`, or at the filesystem root.
3. Built-in defaults.

The walk-up step means a single config at the repo root covers every
subproject without per-module wiring, while a subproject can still
override by dropping its own `krit.yml` closer to the source.

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

Krit XML baselines use the `SmellBaseline` document shape with `ManuallySuppressedIssues`, `CurrentIssues`, and `RuleName:path:signature` IDs. Filename-only IDs and module-relative IDs are both recognized so existing baseline files can be reused during migration.

## Cache location

Generated repo-local state lives under `.krit/` by default. The incremental analysis cache uses `.krit/cache/`; parse, resource, type, file-walk, and library-profile indexes use sibling directories under the same root. `--cache-dir DIR` only overrides the incremental analysis cache.

## Analysis depth

Krit exposes a single `analysis.depth` dial that selects how much
compiler-backed analysis runs. It is the supported way to trade cold-run
time against precision; individual `--no-*` flags still work and beat
the preset.

```yaml
analysis:
  depth: balanced   # fast | balanced | thorough
```

| Preset    | What it does                                                                                          |
|-----------|-------------------------------------------------------------------------------------------------------|
| `fast`    | Skips the JVM type oracle. Source-level inference still runs. Best for low-latency local checks.      |
| `balanced`| (Default) Source inference + JVM type oracle.                                                         |
| `thorough`| Balanced plus a targeted-resolution pre-pass: opt-in rules batch expression-position queries to KAA so the oracle has precise type facts before dispatch. Improves precision on lambda-param nullsafety and properties with externally-typed initializers. |

Precedence, highest first: explicit individual flag (e.g.
`--no-type-oracle`) → `--depth=<preset>` → `analysis.depth` → `balanced`.

Compare `fast` and `balanced` on a representative target before changing CI
defaults. `fast` is appropriate for projects that do not enable rules requiring
oracle facts.

## Experimental performance knobs

Cold type-oracle cache misses use one-shot `krit-types` analysis with `--experimental-parallel-files 4` by default. Override the worker count with `KRIT_TYPES_PARALLEL_FILES=N`; set it to `0` or `1` to disable in-JVM file parallelism. Set `KRIT_DAEMON_CACHE=on` to use the persistent daemon miss-analysis path instead.

Kotlin compiler diagnostics are disabled in the default type-oracle path because `collectDiagnostics()` is expensive on large projects and most type-aware rules use FlatNode or expression facts instead. Pass `--oracle-diagnostics` when you need diagnostic-backed oracle findings such as compiler-proven unreachable code or impossible casts.

`KRIT_DAEMON_POOL=N` is an opt-in benchmark knob for warm type-oracle cache misses. The default is `1`. Values greater than `1` keep additional persistent Kotlin Analysis API JVM daemons for the same source tree and shard miss analysis only for larger miss sets.

Each pool member is a full JVM with its own Analysis API session, so idle memory use scales roughly with the pool size. Use this only while measuring warm edit runs, not as a default project setting.

## Migrating from detekt

Krit accepts detekt-style rule names, config keys, suppression prefixes, and baseline XML files:

```bash
krit --config detekt.yml .
```

Overlapping rules keep the same names and compatible keys, but behavior isn't guaranteed identical in every edge case — sanity-check a few files after migration.
