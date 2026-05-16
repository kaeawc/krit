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
Central staging rejection) without cutting a new tag. The resolved
version is passed to Gradle as `-PVERSION_NAME=<version>`, overriding
the SNAPSHOT default in [`tools/krit-rule-api/gradle.properties`](../tools/krit-rule-api/gradle.properties).

The rule API jar does not embed the Krit binary version directly —
they share a release cadence but are decoupled artifacts. A consumer
of `krit-rule-api:0.2.0` is not required to run the `0.2.0` binary,
only one within the SPI compatibility window.

## Publishing stack

`tools/krit-rule-api/` uses the
[Vanniktech Gradle Maven Publish plugin](https://vanniktech.github.io/gradle-maven-publish-plugin/)
(`com.vanniktech.maven.publish`). The plugin owns:

- Sources jar + (empty placeholder) Javadoc jar.
- Full POM (license, SCM, developers, issue tracker) sourced from
  `POM_*` properties in `gradle.properties`.
- PGP signing via the `signing-plugin` in-memory key flow.
- Upload to the Sonatype Central Portal and automatic promotion to
  the public repository once Central validation passes
  (`mavenCentralAutomaticPublishing=true`).

The pattern mirrors `~/kaeawc/auto-mobile/android` so the two
projects share secret names and operator workflow.

## Required repository secrets

Configure these in **Settings → Secrets and variables → Actions** on
the `kaeawc/krit` repo. All five are required for a successful
`publish-krit-rule-api.yml` run; the workflow fails if any are
missing during signing or upload.

| Secret | Purpose |
| --- | --- |
| `MAVEN_USERNAME` | Sonatype Central Portal user token name (**not** the portal account email). Generate from the Central Portal "Account → Generate User Token" page. Read by Gradle as `ORG_GRADLE_PROJECT_mavenCentralUsername`. |
| `MAVEN_PASSWORD` | Paired Central Portal user token password. Read as `ORG_GRADLE_PROJECT_mavenCentralPassword`. |
| `SIGNING_IN_MEMORY_KEY` | ASCII-armored PGP private key (subkey is fine). Generate with `gpg --armor --export-secret-keys <KEYID>`. Must be uploaded to a public keyserver (`keys.openpgp.org`) so Central can verify. Read as `ORG_GRADLE_PROJECT_signingInMemoryKey`. |
| `SIGNING_IN_MEMORY_KEY_ID` | Short key ID for the subkey above (last 8 chars of the fingerprint). Read as `ORG_GRADLE_PROJECT_signingInMemoryKeyId`. |
| `SIGNING_IN_MEMORY_KEY_PASSWORD` | Passphrase for the in-memory key. Read as `ORG_GRADLE_PROJECT_signingInMemoryKeyPassword`. |

Reuse note: the binary release uses a separate GPG identity
(`GPG_PRIVATE_KEY` / `GPG_PASSPHRASE` / `GPG_FINGERPRINT`) for
signing release artifacts via goreleaser. The Maven Central secrets
are intentionally distinct so that the Maven signing subkey can be
rotated independently of the binary release key.

## Coordinates

```
group:    dev.jasonpearson.krit
artifact: krit-rule-api
version:  <tag-without-leading-v>
```

Sources jar (`-sources`) and Javadoc jar (`-javadoc`) are produced
automatically by the plugin. The Javadoc jar is currently empty —
the Rule SPI is Kotlin-only and Dokka HTML output is published
separately on the docs site; the empty jar exists to satisfy
Central's "must contain javadoc" requirement.

## Local verification

Before tagging, validate the publishing config end to end against
the local Maven cache:

```bash
cd tools/krit-rule-api
./gradlew publishToMavenLocal -PVERSION_NAME=0.2.0-LOCAL
```

This produces `~/.m2/repository/dev/jasonpearson/krit/krit-rule-api/0.2.0-LOCAL/`
containing the jar, sources jar, javadoc jar, Gradle module metadata,
and POM. Inspect the POM to confirm coordinates, license, SCM, and
developer blocks are populated.

The vanniktech plugin also exposes a dry-run path that builds the
full Central-bound bundle into a local directory without uploading:

```bash
./gradlew publishAllPublicationsToMavenCentralRepository \
  -PVERSION_NAME=0.2.0-LOCAL \
  --no-configuration-cache
```

Without credentials in the environment the upload step fails fast
with a clear auth error — the prior tasks (sign, sourcesJar, jar,
POM generation) still run and any failure there surfaces locally.

## Dry-running the workflow

To validate the workflow without publishing a stable release, push
a release-candidate tag (`vX.Y.Z-rc1`). The plugin still uploads to
Central, but `mavenCentralAutomaticPublishing=true` only promotes
when Central's validator accepts the bundle — manually log into the
Central Portal to drop the staging repo if the rc shouldn't promote.

## Troubleshooting

- **Signing succeeds locally but fails in CI.** Confirm
  `SIGNING_IN_MEMORY_KEY` was added without trimming whitespace —
  the value must include the full `-----BEGIN PGP PRIVATE KEY BLOCK-----`
  header and trailing newline. The `SIGNING_IN_MEMORY_KEY_ID` must
  match the subkey used to ASCII-armor the export.
- **Central rejects with "PGP signature not found".** The signing
  key has not been uploaded to `keys.openpgp.org`. Central polls
  that keyserver to verify the detached signatures.
- **Workflow exits with "No version available".** A push event
  landed on a non-tag ref. The workflow's `on.push.tags` filter
  normally prevents this; if it occurs, manually dispatch with
  the desired version.
