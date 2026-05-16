# Release

Krit ships in three places on every `v*` tag:

- **GitHub Releases** — `krit`, `krit-lsp`, `krit-mcp` binaries plus SBOMs,
  signed checksums, Homebrew cask, and Scoop manifest. Owned by
  [`release.yml`](../.github/workflows/release.yml).
- **Maven Central** — `dev.jasonpearson.krit:krit-rule-api` so external
  rule authors can compile against the SPI without vendoring the
  analyzer. Owned by
  [`publish-krit-rule-api.yml`](../.github/workflows/publish-krit-rule-api.yml).
- **Gradle Plugin Portal** — `dev.jasonpearson.krit` Gradle plugin
  (separate; not yet automated, see
  [`krit-gradle-plugin/`](../krit-gradle-plugin/)).

This page focuses on the Maven Central piece. The binary release flow
is documented inline in `release.yml`.

## Versioning

Both workflows derive their version from the pushed tag: `vX.Y.Z` →
`X.Y.Z`. `publish-krit-rule-api.yml` also accepts a manual-dispatch
input so a maintainer can republish a specific version (e.g. after a
Central staging rejection) without cutting a new tag.

The rule API jar does not embed the Krit binary version directly —
they share a release cadence but are decoupled artifacts. A consumer
of `krit-rule-api:0.2.0` is not required to run the `0.2.0` binary,
only one within the SPI compatibility window.

## Required repository secrets

Configure these in **Settings → Secrets and variables → Actions** on
the `kaeawc/krit` repo. All four are required for a successful
`publish-krit-rule-api.yml` run; the workflow fails fast if any are
missing during signing or upload.

| Secret | Purpose |
| --- | --- |
| `MAVEN_CENTRAL_SIGNING_KEY` | ASCII-armored PGP private key (subkey is fine) used by Gradle's `signing` plugin to sign the publication. Generate with `gpg --armor --export-secret-keys <KEYID>`. Must be uploaded to a public keyserver (`keys.openpgp.org`) so Central can verify. |
| `MAVEN_CENTRAL_SIGNING_PASSWORD` | Passphrase for `MAVEN_CENTRAL_SIGNING_KEY`. Empty passphrase is allowed but discouraged. |
| `SONATYPE_USERNAME` | Sonatype Central Portal user token name (**not** the portal account email). Generate from the Central Portal "Account → Generate User Token" page. |
| `SONATYPE_PASSWORD` | Paired Central Portal user token password. |

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

To validate the workflow without publishing publicly, push a release
candidate tag (`vX.Y.Z-rc1`). The publish step will run against the
real Central staging repository but the resulting release sits in
"staging" until manually promoted from the Central Portal UI, giving
you a chance to inspect the artifacts and abort.

## Troubleshooting

- **Signing succeeds locally but fails in CI.** Confirm
  `MAVEN_CENTRAL_SIGNING_KEY` was added without trimming whitespace —
  the value must include the full `-----BEGIN PGP PRIVATE KEY BLOCK-----`
  header and trailing newline.
- **Central rejects with "PGP signature not found".** The signing key
  has not been uploaded to `keys.openpgp.org`. Central polls that
  keyserver to verify the detached signatures.
- **Workflow exits with "No version available".** A push event landed
  on a non-tag ref. The workflow's `on.push.tags` filter normally
  prevents this; if it occurs, manually dispatch with the desired
  version.
