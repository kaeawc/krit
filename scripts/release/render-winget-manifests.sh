#!/usr/bin/env bash
# Render the winget manifests (version, installer, default locale) for the
# current release tag, laid out as they appear in microsoft/winget-pkgs:
#   ${OUT_DIR}/manifests/k/kaeawc/krit/<version>/kaeawc.krit*.yaml
#
# The release workflow uploads the rendered directory as an artifact and,
# when a WINGET_TOKEN secret is configured, submits it to winget-pkgs with
# wingetcreate.
#
# Required env:
#   TAG       Release tag (e.g. v0.2.0)
#   REPO      Source repo, owner/name (e.g. kaeawc/krit)
# Optional env:
#   OUT_DIR   Output root (default: dist/winget)
#
# Reads the Windows archive's SHA256 from dist/checksums.txt produced by
# the funnel job.

set -euo pipefail

: "${TAG:?TAG is required}"
: "${REPO:?REPO is required}"
OUT_DIR="${OUT_DIR:-dist/winget}"

VERSION="${TAG#v}"
ID="kaeawc.krit"
MANIFEST_VERSION="1.6.0"
ARCHIVE="krit_${VERSION}_windows_amd64.zip"

sha=$(grep "  ${ARCHIVE}\$" dist/checksums.txt | awk '{print $1}' || true)
if [ -z "$sha" ]; then
  echo "ERROR: no checksum for $ARCHIVE in dist/checksums.txt" >&2
  exit 1
fi
SHA256=$(printf '%s' "$sha" | tr '[:lower:]' '[:upper:]')

dir="${OUT_DIR}/manifests/k/kaeawc/krit/${VERSION}"
mkdir -p "$dir"

cat > "${dir}/${ID}.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.version.${MANIFEST_VERSION}.schema.json
PackageIdentifier: ${ID}
PackageVersion: ${VERSION}
DefaultLocale: en-US
ManifestType: version
ManifestVersion: ${MANIFEST_VERSION}
EOF

cat > "${dir}/${ID}.installer.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.installer.${MANIFEST_VERSION}.schema.json
PackageIdentifier: ${ID}
PackageVersion: ${VERSION}
InstallerType: zip
NestedInstallerType: portable
NestedInstallerFiles:
  - RelativeFilePath: krit.exe
    PortableCommandAlias: krit
  - RelativeFilePath: krit-lsp.exe
    PortableCommandAlias: krit-lsp
  - RelativeFilePath: krit-mcp.exe
    PortableCommandAlias: krit-mcp
Installers:
  - Architecture: x64
    InstallerUrl: https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}
    InstallerSha256: ${SHA256}
ManifestType: installer
ManifestVersion: ${MANIFEST_VERSION}
EOF

cat > "${dir}/${ID}.locale.en-US.yaml" <<EOF
# yaml-language-server: \$schema=https://aka.ms/winget-manifest.defaultLocale.${MANIFEST_VERSION}.schema.json
PackageIdentifier: ${ID}
PackageVersion: ${VERSION}
PackageLocale: en-US
Publisher: kaeawc
PublisherUrl: https://github.com/kaeawc
PackageName: Krit
PackageUrl: https://github.com/${REPO}
License: MIT
LicenseUrl: https://github.com/${REPO}/blob/${TAG}/LICENSE
ShortDescription: Fast Kotlin static analysis powered by tree-sitter
Tags:
  - android
  - java
  - kotlin
  - linter
  - static-analysis
ReleaseNotesUrl: https://github.com/${REPO}/releases/tag/${TAG}
ManifestType: defaultLocale
ManifestVersion: ${MANIFEST_VERSION}
EOF

echo "Rendered winget manifests in ${dir}:"
ls -1 "$dir"
