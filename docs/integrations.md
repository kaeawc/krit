# Integrations

Krit's shipped integration today is the **raw CLI**, which emits
SARIF / Checkstyle / JSON / plain output for any CI runner or
pre-commit hook.

Editor (LSP) and AI-agent (MCP) integrations also ship as separate
binaries (`krit-lsp`, `krit-mcp`) installed alongside `krit`. For
configuring `krit-mcp` with Claude Code, Claude Desktop, Codex, or
any MCP-compatible client see [MCP setup](mcp.md).

## Bazel

The `krit` Bazel module (`bazel/` in this repository) downloads the
prebuilt release for the host platform, so krit's version is an ordinary
`bazel_dep` that Dependabot or Renovate can bump:

```starlark
# MODULE.bazel
bazel_dep(name = "krit", version = "x.y.z")
```

```bash
bazel run @krit//:krit -- --base-path "$PWD" "$PWD"
```

`@krit//:krit-lsp` and `@krit//:krit-mcp` are available too. The archive
is verified against the release's `checksums.txt`. To pin the checksums
yourself, or to run a different release than the module's version:

```starlark
krit = use_extension("@krit//:extensions.bzl", "krit")
krit.toolchain(
    version = "0.3.0",
    sha256 = {"krit_0.3.0_linux_amd64.tar.gz": "<sha256>"},
)
```

Linux (amd64, arm64), macOS (amd64, arm64) and Windows (amd64) are
supported. krit itself needs no Bazel build to analyze a Bazel workspace;
see [Bazel workspaces](hermetic-builds.md#bazel-workspaces).

### krit_test

`krit_test` runs krit as a Bazel test, so `bazel test //...` lints the
sources and Bazel caches the result until a source or the config changes.
The test fails when krit reports a finding:

```starlark
load("@krit//:defs.bzl", "krit_test")
load("@rules_kotlin//kotlin:jvm.bzl", "kt_jvm_library")

kt_jvm_library(
    name = "lib",
    srcs = glob(["src/main/kotlin/**/*.kt"]),
)

krit_test(
    name = "lib_krit",
    targets = [":lib"],
    config = "//:krit.yml",
)
```

| Attribute | |
| --- | --- |
| `targets` | Targets whose `srcs` are analyzed: `kt_jvm_library`, `kt_android_library`, `java_library`, or any rule with a `srcs` attribute. Only each target's own `.kt`, `.kts` and `.java` files are analyzed, not its dependencies'. |
| `srcs` | Kotlin and Java files to analyze directly. |
| `config` | `krit.yml` passed as `--config`. |
| `baseline` | Baseline file passed as `--baseline`. |
| `format` | Report format printed to the test log (default `plain`). |
| `args`, `data` | Extra krit flags, placed before the files, and files they refer to (`$(rootpath ...)`). |

The test runs Go-only and in-process: no daemon, no incremental cache, no
FIR checker, no type oracle and no Java facts helper. It needs no JDK or
network and reports the same findings on every machine. Rules that rely on
the JVM fall back to their source-level checks, as with `--no-fir`. Rules that
look across modules (dead code, unused resources) only see the test's
sources; run those as a whole-repository `bazel run @krit//:krit` step (see
[Sharding rules by scope](hermetic-builds.md#sharding-rules-by-scope)).
Reported paths are relative to the workspace root.

krit skips files under test-data and test-resource directories
(`testdata/`, `testData/`, `test-data/`, `src/test/resources/`, ...). A
`krit_test` whose sources are all skipped fails instead of passing without
checking anything. `krit_test` is a shell script, so it runs on Linux and
macOS, not Windows.

## GitHub Actions

The repository root is a GitHub Action. It installs a krit release, puts
`krit`, `krit-lsp` and `krit-mcp` on `PATH`, and can run a scan:

```yaml
permissions:
  contents: read
  security-events: write   # only for upload-sarif

steps:
  - uses: actions/checkout@v4
  - uses: kaeawc/krit@v0.4.0
    with:
      run: 'true'
      args: --no-fir
      upload-sarif: 'true'
```

- **Version**: the action installs the release it is pinned to, so
  `kaeawc/krit@v0.4.0` runs krit 0.4.0, and Dependabot's `github-actions`
  ecosystem bumps the action and the CLI together. A SHA pin, or the
  floating major tag `@v0` (moved to every stable release), installs the
  release tagged on that commit. `version:` overrides it; `latest` works but
  is not reproducible.
- **Verification**: the archive is checked against the release's
  `checksums.txt` and the install fails if it is missing, does not list the
  archive, or does not match. `verify-attestation: 'true'` also checks the
  GitHub build-provenance attestation with `gh attestation verify`.
- **Platforms**: Linux amd64/arm64 (musl amd64 detected, or forced with
  `libc: musl`), macOS amd64/arm64 and Windows amd64. The binary is cached
  with `actions/cache` unless `cache: 'false'`.
- **Run** (`run: 'true'`): `paths`, `args`, `config` and `baseline` pass
  through to krit, which writes `sarif-file` (default `krit.sarif`) with
  locations relative to the workspace. The `findings-count` output is the
  number of SARIF results (counted with `jq`), and `fail-on-findings`
  (default `true`) fails the step when it is non-zero. `upload-sarif: 'true'`
  uploads the report to code scanning under `category` (default `krit`).

Outputs: `version`, `path`, `findings-count`, `sarif-file`, `exit-code`.

`kaeawc/krit/.github/actions/krit-action` is the older composite action
(annotations through a problem matcher, a `fir` input that sets up Java). It
now shares the root action's fail-closed download and counts findings with
`jq`; new workflows should use the root action.

## Pre-commit

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/kaeawc/krit
    rev: <pinned tag>
    hooks:
      - id: krit          # analyze source files
      - id: krit-fix      # auto-fix source files (optional)
```

`krit` runs analysis and fails the commit on findings. `krit-fix` runs
`krit --fix` over the staged files — use one or the other, not both in
the same hook stage.

FIR runs by default and needs Java 21+ plus a declared classpath source.
Run `./gradlew kritExportModel` before the hook, or set `oracle.classpath`
in `krit.yml`. To use Go analysis only, add `args: [--no-fir]` to the hook.

## Output formats for CI

The older `krit-action`'s `fir` input defaults to `true` and sets up Temurin
Java 21. Before the action runs, export a Gradle model with
`./gradlew kritExportModel` or set `oracle.classpath` in `krit.yml`.
Set `fir: 'false'` to run with `--no-fir`.

| Format     | Use case                    | Flag |
|------------|-----------------------------|------|
| SARIF      | GitHub Code Scanning        | `--format sarif` |
| Checkstyle | Jenkins, other CI tools     | `--format checkstyle` |
| JSON       | Custom processing (default) | `--format json` |
| Plain      | Human-readable logs         | `--format plain` |

### Several reports from one run

`--report FORMAT=PATH`, repeated, writes one file per report from a single
analysis run, so enabling SARIF and JSON does not scan twice:

```bash
krit --report sarif=build/krit.sarif --report json=build/krit.json src/
```

Every report renders the same final findings (after baseline, `--diff`,
`--warnings-as-errors` and `--min-confidence`). Like `-o`, each file is staged
next to its target and only replaces it once the run produced a complete
report; a run that exits with code 2 leaves the previous files in place. With
only `FORMAT=PATH` reports, stdout gets no report (the summary still goes to
stderr); add `-f`/`--format` or `-o` to also write the usual output. A bare
`--report FORMAT` keeps its old meaning, an alias for `-f`. Such scans run
in-process rather than through the daemon.

### Finding paths

Every format reports each finding's file path the same way, whatever form the
scan path took (`krit .`, `krit src`, `krit /abs/project`) and whether or not
a daemon serves the scan:

- A file under the base path is reported relative to it, with forward slashes.
  The base path is `--base-path` when set, otherwise the first scan path (its
  directory when it is a file).
- A file outside the base path is reported absolute.
- `--absolute-paths` reports every file as absolute instead.

Baselines and `--fix` are unaffected: they match and edit files on disk.

### JSON finding fields

Each entry in `findings` has these fields:

| Field | Meaning |
|-------|---------|
| `file`, `line`, `column` | 1-based position of the flagged node: the declaration name for function, class and property rules, the call or operator for expression rules, the attribute (or element start when the attribute is absent) for XML rules. |
| `startByte`, `endByte` | Byte span in the file, starting at `line`/`column`. Present on every finding whose position maps into a readable file; `endByte` is always greater than `startByte`. Rules that know the flagged node report its full range (a cast, a call, a function declaration). For the others, Kotlin and Java findings end with the innermost syntax node that starts at the position (usually the identifier or keyword), and XML findings end with the attribute (`name="value"`) or the element name. Use `line` and `column` for navigation. |
| `ruleSet`, `rule`, `severity`, `message` | Rule identity and report text. |
| `fixable`, `fixLevel` | `fixable` is true when the finding carries an autofix; `fixLevel` (`cosmetic`, `idiomatic`, `semantic`) describes its safety and is omitted otherwise. |
| `confidence` | Number in (0, 1], always written with a decimal point (`1.0`, `0.75`). Compiler-confirmed findings score higher than name-based heuristics. |
| `effort` | `trivial` (one line or one attribute), `local` (one file), `refactor` (several files) or `architectural`. |

## Suggested fixes (CLI and IDE)

Some rules emit ordered, user-selectable **suggested fixes** instead
of a single autofix (see [Suggested fixes](suggested-fixes.md)). The
two surfaces below are how integrations apply one.

### CLI: `krit apply-suggestion`

```bash
krit --format=json . > findings.json

# List all findings that carry suggestions and their suggestion ids:
krit apply-suggestion --list findings.json

# Preview what a specific suggestion would change:
krit apply-suggestion \
  --finding 'PreferVal:src/main/kotlin/Example.kt:5:3' \
  --suggestion use-val \
  --dry-run \
  findings.json

# Apply it:
krit apply-suggestion \
  --finding 'PreferVal:src/main/kotlin/Example.kt:5:3' \
  --suggestion use-val \
  findings.json
```

The verb reuses the autofix application path
(`fixer.ApplyAllFixesColumns`), so cross-file edits, byte/line spans,
overlap deduplication, and ktfmt-shaped output match `krit --fix`.
Informational suggestions (no `edits`, only `applicationToken`) are
rejected — they are for the integration to interpret, not for the CLI
to write.

### IDE / LSP

Integrations should render each `suggestedFixes[]` entry as its own
native quick-fix / code-action, **in the order the rule emitted them**
— do not sort by id, title, or safety level:

- Register one quick-fix per machine-applicable suggestion (those with
  non-empty `edits`).
- When suggestions are present, suppress the generic "apply autofix"
  entry so the user is never offered both choices at once.
- Surface informational suggestions (no `edits`) as intentions that
  route through the integration's help/`applicationToken` handling.

For LSP wrappers, expose each suggestion as a distinct `CodeAction`
with a stable `title` (the rule-provided `Title`) and a `data` blob
that carries `{findingId, suggestionId}` so the resolve step can run
`krit apply-suggestion` with deterministic ids.
