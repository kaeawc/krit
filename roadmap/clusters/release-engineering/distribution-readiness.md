# DistributionReadiness

**Cluster:** [release-engineering](README.md) · **Status:** open ·
**Severity:** n/a (infra) · **Default:** n/a

## What it does

Five open decisions about krit's own distribution infrastructure,
surfaced during a `docs/` audit on 2026-04-17. Each was cited as a
working install path or integration in the shipped docs, but none
actually function end-to-end today. Resolving each is a build-and-publish
vs. remove-from-docs call.

## Current cost

Documented install and integration paths that fail for real users:

| Surface | Claim in docs | Reality |
|---|---|---|
| Homebrew tap | `brew install kaeawc/tap/krit` (`docs/index.md:14`, `docs/install.md:5-7`) | `kaeawc/homebrew-tap` repo does not exist on GitHub. Local `homebrew/krit.rb` has `sha256 "PLACEHOLDER"` × 4. |
| Scoop bucket | `scoop bucket add krit https://github.com/kaeawc/scoop-krit` (`docs/install.md:30-33`) | `kaeawc/scoop-krit` repo does not exist. Local `scoop/krit.json` has one PLACEHOLDER. |
| winget | Implied by `winget/krit.yaml` presence in repo | Local manifest has one PLACEHOLDER; no public submission. |
| GitHub releases | `KRIT_VERSION=0.2.0` example (`docs/install.md:18`), `rev: v0.1.0` pre-commit (`docs/integrations.md:131`), every `tar.gz` URL in Homebrew/Scoop/winget | No releases exist on `kaeawc/krit`. Install scripts (`scripts/install.sh`, `scripts/install.ps1`) download from release URLs that 404. Homebrew formula pins `version "0.1.0"` while docs mention `0.2.0`. |
| Gradle plugin | `id("io.github.kaeawc.krit") version "0.1.0"` with tasks `kritAnalyze`/`kritFix`/`kritBaseline` (`docs/integrations.md:97,108-113`) | Actual plugin ID is `dev.krit` (`krit-gradle-plugin/build.gradle.kts:17`). Actual tasks are `kritCheck`/`kritFormat`/`kritBaseline`. Extension DSL uses `reports { sarif { ... } }` blocks, not the `format`/`outputFile` keys shown in docs. |
| GitHub Action | Full workflow example using `./.github/actions/krit-action/` with `args:` and `diff:` inputs (previously in `docs/integrations.md`) | `.github/actions/krit-action/action.yml` exists in-repo but the action is not published to the Marketplace, not tagged, and not validated end-to-end. Removed from user-facing docs 2026-04-17. |

## Proposed design

Each surface is a standalone yes/no decision. The GitHub-release decision
gates four of the five, so it should be taken first.

### 1. Cut a first GitHub release

Gates: Homebrew tap publish, Scoop bucket publish, winget submission,
install-script download URLs, pre-commit `rev:` pinning.

- **Yes**: tag `v0.1.0`, run `scripts/release-check.sh`, upload binaries
  via `.goreleaser.yml`, then fill in sha256 placeholders in
  `homebrew/krit.rb` / `scoop/krit.json` / `winget/krit.yaml`.
- **No**: remove every version-specific reference and every
  release-URL-dependent install path from `docs/`.

### 2. Homebrew tap

- **Publish**: create `kaeawc/homebrew-tap` repo, copy `homebrew/krit.rb`
  there, wire `goreleaser` to auto-update the formula on tag. Depends on
  decision 1.
- **Remove**: delete the `brew install` lines from `docs/index.md` and
  `docs/install.md`. Keep the `homebrew/` dir as a stub with a note that
  it's not yet published.

### 3. Scoop bucket

- **Publish**: create `kaeawc/scoop-krit` repo, copy `scoop/krit.json`
  there, wire `goreleaser` to auto-update the manifest on tag. Depends
  on decision 1.
- **Remove**: delete the Scoop section from `docs/install.md`.

### 4. winget

- **Publish**: submit `winget/krit.yaml` to
  `microsoft/winget-pkgs` with real sha256s. Depends on decision 1.
  Higher friction than tap/scoop — PR reviewed by Microsoft.
- **Remove**: docs don't currently advertise winget, but the `winget/`
  dir implies it. Either note "not yet published" or remove the dir.

### 5. GitHub Action

- **Publish**: tag a release (decision 1), push the action to the GitHub
  Marketplace, validate the workflow end-to-end against a sample Kotlin
  repo. Restore the full workflow example to `docs/integrations.md` —
  the previous draft is preserved below.
- **Remove**: delete `.github/actions/krit-action/` and drop any
  in-repo references until someone owns it.

Preserved draft (for restoration if we ship):

```yaml
name: Krit
on: [push, pull_request]

jobs:
  analyze:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: kaeawc/krit-action@v0.1.0     # or ./.github/actions/krit-action/ for local
        with:
          args: '--format sarif -o results.sarif .'
      - uses: github/codeql-action/upload-sarif@v3
        if: always()
        with:
          sarif_file: results.sarif
```

PR-only check:

```yaml
- uses: kaeawc/krit-action@v0.1.0
  with:
    diff: origin/main
```

### 6. Gradle plugin: rename to match docs, or update docs to match code

- **Rename code**: change the plugin ID from `dev.krit` to
  `io.github.kaeawc.krit` in `krit-gradle-plugin/build.gradle.kts:17`
  and the Kotlin package path from `dev.krit.gradle` to
  `io.github.kaeawc.krit.gradle`. Add `kritAnalyze`/`kritFix` as aliases
  (or rename `kritCheck` → `kritAnalyze`, `kritFormat` → `kritFix`).
  Update extension DSL to expose `format`/`outputFile` keys.
- **Update docs**: change `docs/integrations.md:95-113` to document
  `dev.krit` plugin ID, `kritCheck`/`kritFormat`/`kritBaseline` task
  names, and the `reports { sarif { ... } }` DSL shape.

The **update docs** path is cheaper and non-breaking. The **rename code**
path is only worth it if the `io.github.*` ID has already been reserved
on the Gradle Plugin Portal or is strongly preferred for branding.

## Relevant files

- `docs/index.md`, `docs/install.md`, `docs/integrations.md` — claim
  sites to fix or gate once decisions land
- `homebrew/krit.rb`, `scoop/krit.json`, `winget/krit.yaml` — local
  manifests with placeholder hashes
- `scripts/install.sh`, `scripts/install.ps1` — depend on release URLs
- `.goreleaser.yml` — would wire up tap/bucket auto-update on tag
- `.pre-commit-hooks.yaml` — advertised via `rev: v0.1.0`
- `krit-gradle-plugin/build.gradle.kts`,
  `krit-gradle-plugin/src/main/kotlin/dev/krit/gradle/KritPlugin.kt` —
  plugin ID + package to match or deliberately diverge from

## Links

- Parent: [`roadmap/58-release-engineering-rules.md`](../../58-release-engineering-rules.md)
  (note: the parent's scope is rule authoring, not distribution — this
  document is cluster-adjacent rather than a sub-rule of the parent doc)
