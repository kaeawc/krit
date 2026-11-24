# Release

Krit ships from `v*` tags:

- **GitHub Releases** — every `v*` tag. `krit_<version>_<os>_<arch>.tar.gz`
  archives (`.zip` on Windows, `krit_<version>_linux_musl_amd64.tar.gz`
  for musl) bundling `krit`, `krit-lsp`, and `krit-mcp`, plus SBOMs, the
  `krit-fir` jar, and a signed `checksums.txt`. Tags with a prerelease
  suffix (`-nightly.*`, `-rc1`, ...) are marked as GitHub prereleases.
  Owned by [`release.yml`](https://github.com/kaeawc/krit/blob/main/.github/workflows/release.yml). The Gradle
  plugin, VS Code extension, and `install.sh` all download these archives
  and verify them against `checksums.txt`. Released CLIs download
  `krit-fir-<tag>.jar` the same way, so the release job fails when that jar
  is missing, is not a jar, or has no matching `checksums.txt` line. Every
  release also carries `krit-bazel-<tag>.tar.gz`, the source archive of the
  Bazel module (see [Bazel Central Registry](#bazel-central-registry)),
  checked the same way. The
  release is created as a draft, each asset upload is retried up to three
  times, and the draft is published only after the uploaded asset list is
  checked (`internal/devtools/releaseassets`).
- **Package managers** — stable `vX.Y.Z` tags only. `release.yml` pushes
  the Homebrew cask (`scripts/release/update-brew-tap.sh`) and Scoop
  manifest (`scripts/release/update-scoop-bucket.sh`), and renders winget
  manifests (`scripts/release/render-winget-manifests.sh`) that its
  `winget` job submits to microsoft/winget-pkgs.
- **Maven Central** — stable `vX.Y.Z` tags only.
  `dev.jasonpearson.krit:krit-rule-api` so external rule authors can
  compile against the SPI without vendoring the analyzer, plus the
  self-contained `:krit-fir` oracle jar (see
  [Oracle jar on Maven Central](#oracle-jar-on-maven-central) below),
  plus the `dev.jasonpearson.krit` and `dev.jasonpearson.krit.settings`
  Gradle plugins (see [Gradle plugins](#gradle-plugins) below).
  Owned by
  [`publish-maven-central.yml`](https://github.com/kaeawc/krit/blob/main/.github/workflows/publish-maven-central.yml).
- **Gradle Plugin Portal** — the same two plugins,
  published by `publish-maven-central.yml` once `GRADLE_PUBLISH_KEY` and
  `GRADLE_PUBLISH_SECRET` are set (skipped with a warning until then). The
  project plugin's version is the krit release it downloads by default;
  the workflow builds both with the tag's version.
- **Bazel Central Registry** — stable `vX.Y.Z` tags only. The `krit`
  module in `bazel/` (see [Bazel Central Registry](#bazel-central-registry)
  below). Owned by
  [`publish-bcr.yml`](https://github.com/kaeawc/krit/blob/main/.github/workflows/publish-bcr.yml).

- **GitHub Action** — the root `action.yml` is versioned by the release
  tags themselves (`uses: kaeawc/krit@vX.Y.Z`). On stable tags `release.yml`
  moves the floating major tag (`v0`) to the release. Listing it on the
  GitHub Marketplace is a one-time manual step from the repository's
  release page ("Publish this Action to the GitHub Marketplace").

[`nightly.yml`](https://github.com/kaeawc/krit/blob/main/.github/workflows/nightly.yml) pushes a
`vX.Y.Z-nightly.<date>` tag daily when `main` has new commits, so
nightlies get a GitHub prerelease and nothing else.
The tag is pushed with `GITHUB_TOKEN`, which cannot trigger `release.yml`'s
`push: tags` event, so `nightly.yml` dispatches `release.yml` on the new tag.
The version is the next minor after the latest stable tag (after `v0.3.1`,
nightlies are `v0.4.0-nightly.<date>`).

## Cadence

- **Nightly**: daily at 06:00 UTC when `main` has new commits. Pin a nightly
  tag to try fixes before they reach a stable release.
- **Stable**: a minor release (`vX.Y.0`) about once a month, cut from a
  green nightly; patch releases (`vX.Y.Z`) when a regression needs a fix
  before the next minor.

## Cutting a stable release

1. Pick a recent nightly whose `Release` run passed, and check that `main`'s
   CI is green at the commit you will tag.
2. In one PR: rename `## [Unreleased]` in `CHANGELOG.md` to
   `## [X.Y.Z] - <date>` (keep an empty `## [Unreleased]` above it), and
   bump the version pins that name a release: the `krit-gradle-plugin`
   default in `krit-gradle-plugin/build.gradle.kts` and the version
   examples in the plugin READMEs. Merge it.
3. Check the publishing credentials (see
   [Required repository secrets](#required-repository-secrets)): the
   GitHub App is installed on `kaeawc/homebrew-tap` and `kaeawc/scoop-krit`,
   or the PAT fallbacks are current.
4. Tag the merge commit and push the tag:
   `git tag -a vX.Y.Z -m "vX.Y.Z" && git push origin vX.Y.Z`.
5. Watch `Release` and `Publish to Maven Central`. The GitHub release is
   published (with provenance attestation) before the package-manager
   steps; if the Homebrew or Scoop push fails, the job fails at
   "Check package-manager publishing" with the reason, and re-running the
   `release` job after fixing access reuses the published release.
6. Confirm `brew install kaeawc/tap/krit` and `scoop install krit` pick up
   the new version, and submit `winget-manifests` by hand if no
   `WINGET_TOKEN` is configured.

This page focuses on the Maven Central piece. The binary release flow
is documented inline in `release.yml`.

## Versioning

Both workflows derive their version from the pushed tag: `vX.Y.Z` →
`X.Y.Z`. `publish-maven-central.yml` skips tags with a prerelease
suffix, and also accepts a manual-dispatch input so a maintainer can
republish a specific version (e.g. after a Central staging rejection)
without cutting a new tag. A dispatched version is published as given,
prerelease or not.

The rule API jar does not embed the Krit binary version directly —
they share a release cadence but are decoupled artifacts. A consumer
of `krit-rule-api:0.2.0` is not required to run the `0.2.0` binary,
only one within the SPI compatibility window.

## Required repository secrets

Configure these in **Settings → Secrets and variables → Actions** on
the `kaeawc/krit` repo. All four are required for a successful
`publish-maven-central.yml` run; the workflow fails fast if any are
missing during signing or upload.

If your Central credentials are already in a Gradle properties file under
the keys the vanniktech `maven-publish` plugin uses (`mavenCentralUsername`,
`mavenCentralPassword`, `signingInMemoryKey`, `signingInMemoryKeyPassword`),
copy them over without printing them:

```bash
scripts/release/set-maven-central-secrets.sh   # defaults to ~/.gradle/gradle.properties
```

The Central Portal account behind the token must own the
`dev.jasonpearson.krit` namespace (or its parent `dev.jasonpearson`).
A token that only owns another namespace is rejected at upload with
"File is not related to an authorized namespace". Register it under
**Publish → Namespaces** in the Central Portal and add the TXT record it
shows to the `jasonpearson.dev` DNS zone.

| Secret | Purpose |
| --- | --- |
| `MAVEN_CENTRAL_SIGNING_KEY` | ASCII-armored PGP private key (subkey is fine) used by Gradle's `signing` plugin to sign the publication. Generate with `gpg --armor --export-secret-keys <KEYID>`. Must be uploaded to a public keyserver (`keys.openpgp.org`) so Central can verify. |
| `MAVEN_CENTRAL_SIGNING_PASSWORD` | Passphrase for `MAVEN_CENTRAL_SIGNING_KEY`. Empty passphrase is allowed but discouraged. |
| `SONATYPE_USERNAME` | Sonatype Central Portal user token name (**not** the portal account email). Generate from the Central Portal "Account → Generate User Token" page. |
| `SONATYPE_PASSWORD` | Paired Central Portal user token password. |
| `GRADLE_PUBLISH_KEY` / `GRADLE_PUBLISH_SECRET` | Optional. Gradle Plugin Portal API key pair (plugins.gradle.org → your account → API Keys). Without them the Plugin Portal step is skipped; the plugin is still on Maven Central. `scripts/release/set-maven-central-secrets.sh` copies them from `gradle.publish.key` / `gradle.publish.secret` when the properties file has both. |
| `BCR_PUBLISH_TOKEN` | Optional, for `publish-bcr.yml`. Classic PAT with `repo` and `workflow` scopes that can push to `kaeawc/bazel-central-registry` (a fork of `bazelbuild/bazel-central-registry`). Without it a tag push skips the BCR publish with a warning. |

When any of the four is missing, a tag push skips the publish with a
warning naming them (the tag's other workflows are unaffected); a manual
dispatch fails instead.

`release.yml` uses these for the package-manager steps:

| Secret | Purpose |
| --- | --- |
| `GH_APP_ID` / `GH_APP_PRIVATE_KEY` | The repository's GitHub App. `release.yml` mints a short-lived installation token for `kaeawc/homebrew-tap` and `kaeawc/scoop-krit` with Contents: write, so the App must be installed on both repositories with that permission. Preferred: nothing expires. |
| `HOMEBREW_TAP_TOKEN` | Fallback when the App cannot mint a token: PAT with Contents: write on `kaeawc/homebrew-tap`. |
| `SCOOP_BUCKET_TOKEN` | Fallback when the App cannot mint a token: PAT with Contents: write on `kaeawc/scoop-krit`. |
| `WINGET_TOKEN` | Optional. Classic PAT with `public_repo` scope; `wingetcreate` opens the winget-pkgs PR from this account's fork. Without it the `winget` job is a no-op and the manifests are left in the run's `winget-manifests` artifact. |

Reuse note: the binary release already uses a separate GPG identity
(`GPG_PRIVATE_KEY` / `GPG_PASSPHRASE` / `GPG_FINGERPRINT`) for signing
release artifacts via goreleaser. The Maven Central secrets are
intentionally distinct so that the Maven signing subkey can be rotated
independently of the binary release key.

## Coordinates

```
group:    dev.jasonpearson.krit
artifact: krit-rule-api | krit-fir | krit-gradle-plugin | krit-settings-plugin
group:    dev.jasonpearson.krit
artifact: dev.jasonpearson.krit.gradle.plugin           (krit-gradle-plugin marker)
group:    dev.jasonpearson.krit.settings
artifact: dev.jasonpearson.krit.settings.gradle.plugin  (krit-settings-plugin marker)
version:  <tag-without-leading-v>
```

Sources jar (`-sources`) and Javadoc jar (`-javadoc`) are produced
automatically. The Javadoc jar is currently empty — the Rule SPI is
Kotlin-only and Dokka HTML output is published separately on the docs
site; the empty jar exists to satisfy Central's "must contain
javadoc" requirement.

## Local verification

Before tagging, validate the publishing config end to end against the
local Maven cache:

```bash
cd tools/krit-rule-api
./gradlew publishToMavenLocal -PkritVersion=0.2.0-LOCAL
```

This produces `~/.m2/repository/dev/jasonpearson/krit/krit-rule-api/0.2.0-LOCAL/`
containing the jar, sources jar, javadoc jar, gradle module metadata,
and POM. Inspect the POM to confirm coordinates, license, SCM, and
developer blocks are populated.

To exercise the full Central-bound bundle (release-mode artifacts,
checksums, no signatures since the key is missing locally), build into
the staging dir:

```bash
./gradlew publishAllPublicationsToStagingDirRepository -PkritVersion=0.2.0-LOCAL
```

Outputs land in `tools/krit-rule-api/build/staging-deploy/`.

## Dry-running the workflow

To stage a deployment for inspection without publishing it, dispatch
`publish-maven-central.yml` manually and leave `publishing_type` at its
default, `user_managed` (or set it explicitly). The workflow publishes
and stages both artifacts for real, then calls
`scripts/release/central-portal-upload.sh`, which POSTs to Central's
`/manual/upload` transfer endpoint with `publishing_type=user_managed`.
That moves the OSSRH staging deployment into the Central Portal, where
it waits for a maintainer to inspect it and manually publish or drop
it — nothing becomes public automatically. Setting `publishing_type` to
`automatic` on a dispatch publishes the deployment for real, matching
what a tag push does. Release-candidate tags (`vX.Y.Z-rc1`) don't run
the job at all — only manual dispatches and dash-free stable tags pass
the job's `if` condition.

## Troubleshooting

- **Signing succeeds locally but fails in CI.** Confirm
  `MAVEN_CENTRAL_SIGNING_KEY` was added without trimming whitespace —
  the value must include the full `-----BEGIN PGP PRIVATE KEY BLOCK-----`
  header and trailing newline.
- **Central rejects with "PGP signature not found".** The signing key
  has not been uploaded to `keys.openpgp.org`. Central polls that
  keyserver to verify the detached signatures.
- **Workflow exits with "No version available".** A push event landed
  on a non-tag ref, or a manual dispatch from a branch omitted the
  version input. Dispatch again with the desired version.

## Oracle jar on Maven Central

The FIR oracle helper `krit` shells out to is also published to Maven
Central as a self-contained fat jar, `dev.jasonpearson.krit:krit-fir`.

The publication uses the shadow jar as its main artifact and ships a
dependency-free POM, so consumers don't need access to JetBrains'
`intellij-dependencies` repository, which hosts the Kotlin compiler
modules the jar bundles and that Maven Central itself
cannot resolve. The jar carries `META-INF/LICENSE-krit.txt` (Krit's own
license) and `META-INF/THIRD_PARTY_NOTICES.txt` (notices for the bundled
Apache-2.0 Kotlin compiler) so the bundled code stays attributed.

`publish-maven-central.yml` publishes `krit-fir` to the
`centralPortal` repository right after `krit-rule-api`, on the same
stable-tag or dispatch trigger, then runs its finalize step
(`scripts/release/central-portal-upload.sh`) once for both
artifacts together — see [Dry-running the workflow](#dry-running-the-workflow)
for what that finalize step does. Publishing the shadow jar to
`centralPortal` with a non-SNAPSHOT version fails fast if `SIGNING_KEY`
or `SIGNING_PASSWORD` is missing, the same guard
`krit-rule-api` uses.

## Gradle plugins

`krit-gradle-plugin` publishes the plugin jar
(`dev.jasonpearson.krit:krit-gradle-plugin`) and its plugin marker
(`dev.jasonpearson.krit:dev.jasonpearson.krit.gradle.plugin`) to Maven
Central in the same deployment as `krit-rule-api` and `krit-fir`.
`krit-settings-plugin` does the same for the settings plugin
(`dev.jasonpearson.krit:krit-settings-plugin` and the marker
`dev.jasonpearson.krit.settings:dev.jasonpearson.krit.settings.gradle.plugin`).
The settings plugin depends on `krit-gradle-plugin` at its own version, so
both always ship together. Until the plugins are on the Gradle Plugin Portal,
consumers resolve them from Maven Central:

```kotlin
// settings.gradle.kts
pluginManagement {
    repositories {
        mavenCentral()
        gradlePluginPortal()
    }
}
```

`com.gradle.plugin-publish` supplies the sources and javadoc jars and the
`publishPlugins` task. The Plugin Portal step runs only for real
publishes (tag pushes and `automatic` dispatches), since the portal has
no staging step to drop. A plugin's first Plugin Portal release waits
for manual approval by the Gradle team.

Before either plugin is uploaded to Central, the workflow stages both and
runs `scripts/release/plugin-smoke.sh`, which resolves them from the staged
output in a throwaway Gradle project (settings plugin applied from
`settings.gradle.kts`, project plugin through its own marker). A broken POM,
module file, or marker fails the release before either plugin is uploaded
(krit-rule-api and krit-fir are already in the open staging repository by
then, but nothing is finalized, so nothing is published).

The settings plugin's marker is in the `dev.jasonpearson.krit.settings`
group, a sub-group of the verified namespace. Make the first release that
includes it a `user_managed` dispatch and check that the deployment lists
`dev.jasonpearson.krit.settings.gradle.plugin` before publishing it.

Check the bundles and the smoke test locally (unsigned without a key):

```bash
(cd krit-gradle-plugin && ./gradlew publishAllPublicationsToStagingDirRepository -PkritVersion=0.4.0-LOCAL)
(cd krit-settings-plugin && ./gradlew publishAllPublicationsToStagingDirRepository -PkritVersion=0.4.0-LOCAL)
scripts/release/plugin-smoke.sh 0.4.0-LOCAL \
  krit-gradle-plugin/build/staging-deploy \
  krit-settings-plugin/build/staging-deploy
```

## Bazel Central Registry

`bazel/` is a Bzlmod module that downloads the prebuilt release, so the
registry entry only needs the module's source archive. Nothing from Maven
Central is involved. `publish-bcr.yml` runs bazel-contrib's
`publish-to-bcr` reusable workflow on stable tags. It reads the templates
in `.bcr/bazel/` (`.bcr/config.yml` sets `moduleRoots: ["bazel"]`), sets
the module version from the tag, and opens a pull request from
`kaeawc/bazel-central-registry` against `bazelbuild/bazel-central-registry`.
BCR CI then runs `.bcr/bazel/presubmit.yml`, which builds the
`bazel/e2e` test module against the new version.

One-time setup:

1. Fork `bazelbuild/bazel-central-registry` to `kaeawc/bazel-central-registry`.
2. Create a classic PAT with `repo` and `workflow` scopes and save it as
   the `BCR_PUBLISH_TOKEN` repository secret.

The source archive is the release asset `krit-bazel-<tag>.tar.gz`, which
`release.yml` builds with `scripts/release/bazel-source-archive.sh`
(`git archive` of `bazel/` at the tag, so a rebuild is byte-identical) and
lists in `checksums.txt`. `.bcr/bazel/source.template.json` points at
`releases/download/<tag>/krit-bazel-<tag>.tar.gz` with `strip_prefix`
`krit-bazel-<tag>/bazel`; BCR treats release-download URLs as stable, so
its validation passes without a `skip_check` comment. A test in
`internal/devtools/releaseassets` keeps the template and the asset name in
step. publish-to-bcr stamps the tag's version into the entry's
`MODULE.bazel`; the copy in `bazel/` stays at `0.0.0`.

On nightly tags the `bazel-e2e` job in `release.yml` checks the published
archive the way a registry entry consumes it: `scripts/release/bazel-e2e.sh`
writes a local registry whose `source.json` points at the archive, and runs
the `bazel/e2e` test module against it, which downloads that nightly's
binaries. Run it locally against any published release (needs Bazelisk):

```bash
scripts/release/bazel-e2e.sh v0.5.0-nightly.20261124
# Test unreleased bazel/ changes against v0.4.0's binaries: in a scratch
# clone (never push it), point a local v0.4.0 tag at your commit first.
git tag -f v0.4.0 HEAD
scripts/release/bazel-source-archive.sh v0.4.0 /tmp/out
scripts/release/bazel-e2e.sh v0.4.0 /tmp/out/krit-bazel-v0.4.0.tar.gz
```

Attestations are off (`attest: false`): BCR only accepts source archive
attestations made by bazel-contrib's `release_ruleset` workflow, which
krit's release does not use. To republish an existing tag, dispatch
`publish-bcr.yml` with `tag_name`.

### First submission

`krit` is not in the registry yet, so its first registry pull request is
reviewed by a BCR maintainer. A person with admin access to `kaeawc/krit`
runs these steps:

1. Do the one-time setup above (the fork and `BCR_PUBLISH_TOKEN`).
2. Pick a stable release that carries `krit-bazel-<tag>.tar.gz`. Releases
   cut before the source archive was added (v0.4.0 and older) do not.
   Either wait for the next stable tag, whose Release run triggers
   `publish-bcr.yml`, or dispatch `publish-bcr.yml` with that `tag_name`.
3. On the registry pull request (opened from
   `kaeawc/bazel-central-registry`), check that the entry
   `modules/krit/<version>/` has `MODULE.bazel` with the release version,
   `source.json` pointing at the release's `krit-bazel-<tag>.tar.gz` and
   `presubmit.yml`, and that `metadata.json` lists `@kaeawc` as maintainer.
   No `skip_check` comment is needed. BCR presubmit runs the `bazel/e2e`
   module on the platforms and Bazel versions in `.bcr/bazel/presubmit.yml`.
4. Answer the maintainer review. Later versions are approved by the
   module's maintainers listed in `metadata.json`.

If the workflow cannot run, open the pull request by hand: add
`modules/krit/metadata.json` from `.bcr/bazel/metadata.template.json` with
`"versions": ["<version>"]`, and `modules/krit/<version>/` holding
`bazel/MODULE.bazel` with `version = "<version>"`,
`.bcr/bazel/presubmit.yml`, and `source.json` from
`.bcr/bazel/source.template.json`. Fill `{OWNER}`, `{REPO}` and `{TAG}`,
and set `integrity` to
`sha384-$(curl -fsSL <url> | openssl dgst -sha384 -binary | openssl base64 -A)`.
Run `bazel run //tools:bcr_validation -- --check=krit@<version>` in the
registry checkout before pushing.

To test changes to `bazel/` before a release, run the e2e module against a
published release's binaries:

```bash
cd bazel/e2e
KRIT_BAZEL_RELEASE=0.4.0 bazel test //...
USE_BAZEL_VERSION=9.x KRIT_BAZEL_RELEASE=0.4.0 bazel test //...
```
