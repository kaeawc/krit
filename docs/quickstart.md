# Quickstart

```bash
krit --init     # write a starter krit.yml
krit .          # analyze the current directory
krit --fix .    # apply safe fixes
```

Exit code is `0` when clean, `1` when findings exist, `2` on config errors.

## Output formats

```bash
krit .                                 # JSON (default)
krit --format sarif . > results.sarif  # GitHub Code Scanning
krit --format checkstyle .             # Jenkins, etc.
krit --format plain .                  # Human-readable
```

## Auto-fix

```bash
krit --fix .                        # cosmetic + idiomatic fixes
krit --fix --fix-level=semantic .   # also apply semantic fixes
krit --dry-run .                    # preview without writing
```

Safety levels: **cosmetic** (whitespace, redundant keywords), **idiomatic** (equivalent Kotlin conventions), **semantic** (changes that can affect edge-case behavior).

## Only check what changed

```bash
krit --diff origin/main .         # findings in files touched since main
krit --diff HEAD~1 .              # findings in files touched in the last commit
krit --diff origin/main --fix .   # auto-fix findings in those files
```

`--diff <ref>` is a file-level filter. It runs `git diff --name-only <ref>` to find changed files, then reports every finding inside those files — including pre-existing ones, not just new violations introduced by the change.

## Baselines

Freeze existing findings, catch new ones:

```bash
krit --create-baseline baseline.xml .
krit --baseline baseline.xml .
```

## Suppress a rule

```kotlin
@Suppress("MagicNumber")
fun calculateOffset() = 42
```

Also supports `@Suppress("all")`, `@SuppressWarnings`, and `detekt:RuleName` prefixes.

## Useful flags

```bash
krit --all-rules .    # enable every rule (many are opt-in)
krit --list-rules     # list all available rules
krit --doctor         # check environment and config
krit --perf .         # show per-rule timing
```

Next: [Configuration](configuration.md) · [Rules](rules.md) · [Integrations](integrations.md)
