# Quickstart

```bash
krit --init     # write a starter krit.yml
krit .          # analyze the current directory
krit --fix .    # apply safe fixes
```

Exit code is `0` when clean, `1` when findings exist, `2` when setup or config
preflight fails. FIR is on by default. Install Java 21+, provide the matching
`krit-fir.jar` (or set `KRIT_FIR_JAR` for air-gapped use), and export a Gradle
model with `kritExportModel` or declare `oracle.classpath` in `krit.yml`.
Krit prints one actionable stderr error and stops before analysis if any of
these are missing. `krit --no-fir .` runs Go-only. See
[configuration](configuration.md#fir-scan-requirements) for Maven/Bazel recipes
and `fir.goAuthoritativeRules`.

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
krit --diff origin/main .         # findings on changed lines since main
krit --diff HEAD~1 .              # findings on lines changed in the last commit
krit --diff origin/main --fix .   # auto-fix findings on changed lines
```

`--diff <ref>` is a line-level filter. It first narrows findings to changed files, then parses zero-context git hunks and reports only findings whose source line falls inside a changed hunk.

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
