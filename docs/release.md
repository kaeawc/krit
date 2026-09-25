# Release

Krit ships from `v*` tags:

- **GitHub Releases** — every `v*` tag. `krit_<version>_<os>_<arch>.tar.gz`
  archives (`.zip` on Windows, `krit_<version>_linux_musl_amd64.tar.gz`
  for musl) bundling `krit`, `krit-lsp`, and `krit-mcp`, plus SBOMs, the
  `krit-types` jar, and a signed `checksums.txt`. Tags with a prerelease
  suffix (`-nightly.*`, `-rc1`, ...) are marked as GitHub prereleases.
  Owned by [`release.yml`](../.github/workflows/release.yml). The Gradle
  plugin, VS Code extension, and `install.sh` all download these archives
  and verify them against `checksums.txt`.
- **Package managers** — stable `vX.Y.Z` tags only. `release.yml` pushes
  the Homebrew cask (`scripts/release/update-brew-tap.sh`) and Scoop
  manifest (`scripts/release/update-scoop-bucket.sh`), and renders winget
  manifests (`scripts/release/render-winget-manifests.sh`) that its
  `winget` job submits to microsoft/winget-pkgs.
- **Maven Central** — stable `vX.Y.Z` tags only.
  `dev.jasonpearson.krit:krit-rule-api` so external rule authors can
  compile against the SPI without vendoring the analyzer, plus the
  self-contained `:krit-types` and `:krit-fir` oracle jars (see
  [Oracle jars on Maven Central](#oracle-jars-on-maven-central) below).
  Owned by
  [`publish-maven-central.yml`](../.github/workflows/publish-maven-central.yml).
- **Gradle Plugin Portal** — `dev.jasonpearson.krit` Gradle plugin
  (separate; not yet automated, see
  [`krit-gradle-plugin/`](../krit-gradle-plugin/)). The plugin's version
  is the krit release it downloads by default; build it with
  `-PkritVersion=X.Y.Z` to match the tag.

[`nightly.yml`](../.github/workflows/nightly.yml) pushes a
`vX.Y.Z-nightly.<date>` tag daily when `main` has new commits, so
nightlies get a GitHub prerelease and nothing else.

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

| Secret | Purpose |
| --- | --- |
| `MAVEN_CENTRAL_SIGNING_KEY` | ASCII-armored PGP private key (subkey is fine) used by Gradle's `signing` plugin to sign the publication. Generate with `gpg --armor --export-secret-keys <KEYID>`. Must be uploaded to a public keyserver (`keys.openpgp.org`) so Central can verify. |
| `MAVEN_CENTRAL_SIGNING_PASSWORD` | Passphrase for `MAVEN_CENTRAL_SIGNING_KEY`. Empty passphrase is allowed but discouraged. |
| `SONATYPE_USERNAME` | Sonatype Central Portal user token name (**not** the portal account email). Generate from the Central Portal "Account → Generate User Token" page. |
| `SONATYPE_PASSWORD` | Paired Central Portal user token password. |

`release.yml` uses these for the package-manager steps:

| Secret | Purpose |
| --- | --- |
| `HOMEBREW_TAP_TOKEN` | PAT with Contents: write on `kaeawc/homebrew-tap`. |
| `SCOOP_BUCKET_TOKEN` | PAT with Contents: write on `kaeawc/scoop-krit`. |
| `WINGET_TOKEN` | Optional. Classic PAT with `public_repo` scope; `wingetcreate` opens the winget-pkgs PR from this account's fork. Without it the `winget` job is a no-op and the manifests are left in the run's `winget-manifests` artifact. |

Reuse note: the binary release already uses a separate GPG identity
(`GPG_PRIVATE_KEY` / `GPG_PASSPHRASE` / `GPG_FINGERPRINT`) for signing
release artifacts via goreleaser. The Maven Central secrets are
intentionally distinct so that the Maven signing subkey can be rotated
independently of the binary release key.

## Coordinates

```
group:    dev.jasonpearson.krit
artifact: krit-rule-api
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
and stages all three artifacts for real, then calls
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

## Oracle jars on Maven Central

The Kotlin Analysis API and FIR oracle helpers `krit` shells out to are
also published to Maven Central as self-contained fat jars:

- `dev.jasonpearson.krit:krit-types`
- `dev.jasonpearson.krit:krit-fir`

Each publication uses the shadow jar as its main artifact and ships a
dependency-free POM, so consumers don't need access to JetBrains'
`intellij-dependencies` repository, which hosts the Kotlin compiler and
Analysis API modules these jars bundle and that Maven Central itself
cannot resolve. The jars carry `META-INF/LICENSE-krit.txt` (Krit's own
license) and `META-INF/THIRD_PARTY_NOTICES.txt` (notices for the bundled
Apache-2.0 compiler/Analysis API) so the bundled code stays attributed.

`publish-maven-central.yml` publishes `krit-types` and `krit-fir` to the
`centralPortal` repository right after `krit-rule-api`, on the same
stable-tag or dispatch trigger, then runs its finalize step
(`scripts/release/central-portal-upload.sh`) once for all three
artifacts together — see [Dry-running the workflow](#dry-running-the-workflow)
for what that finalize step does. Publishing either shadow jar to
`centralPortal` with a non-SNAPSHOT version fails fast if `SIGNING_KEY`
or `SIGNING_PASSWORD` is missing, the same guard
`krit-rule-api` uses.
