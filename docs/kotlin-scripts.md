# Kotlin scripts (.kts)

Krit scans `.kts` files together with `.kt` files. Three kinds of script exist:

| Kind | Files | Examples |
| --- | --- | --- |
| Gradle build script | `*.gradle.kts` | `build.gradle.kts`, `settings.gradle.kts`, `init.gradle.kts`, precompiled script plugins such as `conventions.gradle.kts` |
| Standalone script | `*.main.kts` | `report.main.kts` with `@file:DependsOn` |
| Other script | any other `*.kts` | scripts with a custom script definition |

Standalone and other scripts are treated the same.

## What runs on a script

A script has no package, no Android framework classes, no Compose or DI graph, no public API
and no test sources. Rules built on those do not run on scripts. Everything else follows
detekt 1.23.8, which analyzes `.kts` like `.kt`.

Each rule has a script scope. `krit --list-rules --format json` prints it as `scripts`, and
`krit --list-rules -v` adds `scripts: none` to rules that skip scripts.

| Rule set | Scripts |
| --- | --- |
| style, naming, complexity, exceptions, potential-bugs, empty-blocks, coroutines, performance, security, comments, licensing, release-engineering, supply-chain | run |
| android-lint, a11y, compose, database, di-hygiene, i18n, observability, privacy, resource-cost | skipped |
| architecture, libraries, dead-code, testing-quality, precompile | skipped (module and public-API rules) |

Gradle rules (dependency, version catalog, plugin and Android Gradle checks) read build scripts
in their own phase. The script scope does not apply to them.

A few rules differ inside the running rule sets. They match what detekt does on a script:

| Rule | On scripts | Why |
| --- | --- | --- |
| `MagicNumber` | skipped | detekt excludes `**/*.kts` by default |
| `MissingPackageDeclaration` | skipped | detekt excludes `**/*.kts` by default |
| `MatchingDeclarationName` | skipped | a script has no declaration that names the file |
| `MayBeConstant`, `TopLevelPropertyNaming`, `NestedClassesVisibility` | skipped | script top-level code is not file-level declarations |
| `ThrowingExceptionInMain`, `ExitOutsideMain` | skipped | a script has no `main` entry point |
| `PrintlnInProduction`, `PrintStackTraceInProduction` | skipped | console output is the interface of a script |
| `UnsafeCallOnNullableType`, `ImplicitDefaultLocale` | skipped | script blocks use `!!` and locale-default calls freely |
| `TooGenericExceptionThrown`, `UnsafeCast`, `MaxLineLength` | skipped in Gradle build scripts only | build scripts throw for build errors, cast extra properties and chain long DSL calls |
| `TooManyFunctions` | class, object and interface thresholds only | detekt does not count script functions toward the file threshold |
| `VariableNaming` | also checks top-level properties | detekt reads them as local variables |
| `UndocumentedPublicProperty` | skips top-level properties | the same reason |

`@file:Suppress`, `@Suppress`, `// krit:ignore` and per-rule `excludes` globs work in scripts
like in `.kt` files. A shebang line (`#!/usr/bin/env kotlin`) stays the first line:
`AbsentOrWrongFileLicense` reads the header after it and its fix inserts there.

## Configuration

```yaml
scripts:
  # auto: each rule's script scope decides (default)
  # all:  every rule runs on scripts, as on .kt files
  # none: no source rule runs on scripts
  policy: auto
  # Run these on scripts whatever the policy says.
  include:
    - DeadCode
  # Never run these on scripts. Wins over include.
  exclude:
    - TooGenericExceptionCaught
    - SwallowedException
```

Use the rule's `excludes` and `includes` globs to pick scripts by path. They match `.kts`
paths like any other:

```yaml
exceptions:
  TooGenericExceptionCaught:
    excludes:
      - '**/*.main.kts'
      - '**/tooling/**'

style:
  ForbiddenMethodCall:
    includes:
      - '**/*.gradle.kts'
```

To keep a rule set off all scripts, exclude `**/*.kts` at the rule-set level:

```yaml
naming:
  excludes: ['**/*.kts']
```

## Type information

The JVM helpers (type oracle and FIR checks) compile `.kt` sources only, and a script is never
sent to the compiler. A request that names a script gets "script not compiled" for it and
Go keeps its findings. The type oracle returns no facts or diagnostics for scripts.

Compiling scripts in the existing krit-fir session is not feasible with bounded work:

- **No script definition.** The krit-fir jar bundles the K2 compiler but not
  `kotlin-scripting-*` or `kotlin-main-kts`. Without a `ScriptCompilationConfiguration` and a
  registered `FirScriptConfiguratorExtension`, a script has no implicit `args`, no default
  imports and no implicit receiver, so every such reference is unresolved.
- **`@file:DependsOn` needs the network.** `.main.kts` dependencies resolve through Ivy or Maven
  while compiling. Offline they stay unresolved, and the calls a script is about (CLI libraries,
  YAML parsers) then have no types. Verdicts that need those types would be wrong or unknown.
- **Gradle scripts need generated code.** `build.gradle.kts` resolves against the `Project`
  receiver, the Gradle API jar and type-safe accessors that Gradle generates per build. The
  exported module model does not carry them.
- **One failed script gates the batch.** A script error has no source location, so a failing
  script compile would block verdicts for every file in the same compilation. This is why scripts
  are partitioned out of the request today.

So scripts keep the Go findings only. Those are the syntactic and source-inferred results, the
same a type-less detekt run reports. No finding on a script comes from oracle diagnostics, so a
script never gets an unresolved-type finding. The partition is tested in
`internal/firchecks/pass_test.go`.

## Parsing

Scripts use the Kotlin grammar. Top-level statements and calls are valid in a script, so the
repairs for misparsed declarations leave them alone: an annotated expression statement stays
with its own statement and does not move to the declaration that follows.
