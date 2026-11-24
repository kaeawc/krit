#!/usr/bin/env bash
# End-to-end check of a released krit Bazel module, the way a Bazel Central
# Registry entry consumes it: a throwaway local registry lists the module at
# the release's version, with source.json pointing at the Bazel source
# archive (krit-bazel-TAG.tar.gz, see bazel-source-archive.sh), and the e2e
# test module from that archive runs `bazel test //...` against it. The
# module then downloads that release's krit binaries and checksums.txt.
#
# Usage: scripts/release/bazel-e2e.sh TAG [ARCHIVE]
#
#   TAG      Published release whose binaries the module downloads
#            (e.g. v0.4.0 or v0.5.0-nightly.20261124).
#   ARCHIVE  Bazel source archive: a local path or URL. Defaults to the
#            release asset https://github.com/$REPO/releases/download/TAG/
#            krit-bazel-TAG.tar.gz. To test unreleased bazel/ changes
#            against a published release, build one from a local tag with
#            bazel-source-archive.sh and pass its path.
#
# Optional env:
#   REPO   owner/name of the release (default kaeawc/krit)
#   BAZEL  Bazel launcher (default bazel; bazelisk honours the archive's
#          bazel/e2e/.bazelversion and USE_BAZEL_VERSION)
#   WORK   scratch directory to keep (default: a temporary one, removed)

set -euo pipefail

TAG="${1:?usage: bazel-e2e.sh TAG [ARCHIVE]}"
REPO="${REPO:-kaeawc/krit}"
BAZEL="${BAZEL:-bazel}"
VERSION="${TAG#v}"
ARCHIVE="${2:-https://github.com/${REPO}/releases/download/${TAG}/krit-bazel-${TAG}.tar.gz}"
STRIP_PREFIX="krit-bazel-${TAG}/bazel"

if [ -z "${WORK:-}" ]; then
  WORK="$(mktemp -d)"
  trap 'chmod -R u+w "$WORK" 2> /dev/null || true; rm -rf "$WORK"' EXIT
fi
mkdir -p "$WORK"
WORK="$(cd "$WORK" && pwd)"

case "$ARCHIVE" in
  http://* | https://*)
    curl -fsSL --retry 3 -o "$WORK/source.tar.gz" "$ARCHIVE"
    ;;
  *)
    cp "$ARCHIVE" "$WORK/source.tar.gz"
    ;;
esac

rm -rf "$WORK/src"
mkdir -p "$WORK/src"
tar -xzf "$WORK/source.tar.gz" -C "$WORK/src"
module_root="$WORK/src/$STRIP_PREFIX"
if [ ! -f "$module_root/MODULE.bazel" ]; then
  echo "ERROR: $ARCHIVE has no $STRIP_PREFIX/MODULE.bazel" >&2
  exit 1
fi

# The registry's MODULE.bazel carries the release version; publish-to-bcr
# stamps it the same way over the archive's unreleased 0.0.0.
entry="$WORK/registry/modules/krit/$VERSION"
rm -rf "$WORK/registry"
mkdir -p "$entry"
printf '{"versions": ["%s"]}\n' "$VERSION" > "$WORK/registry/modules/krit/metadata.json"
sed -E "s/^([[:space:]]*version = )\"0\\.0\\.0\",/\\1\"${VERSION}\",/" "$module_root/MODULE.bazel" > "$entry/MODULE.bazel"
if ! grep -q "version = \"${VERSION}\"," "$entry/MODULE.bazel"; then
  echo "ERROR: could not stamp version ${VERSION} into MODULE.bazel" >&2
  exit 1
fi
integrity="sha384-$(openssl dgst -sha384 -binary "$WORK/source.tar.gz" | openssl base64 -A)"
cat > "$entry/source.json" << EOF
{
  "integrity": "${integrity}",
  "strip_prefix": "${STRIP_PREFIX}",
  "url": "file://${WORK}/source.tar.gz"
}
EOF

# The e2e module depends on krit through local_path_override; replace that
# with the registry version so the test resolves the module like a user.
e2e="$WORK/e2e"
rm -rf "$e2e"
cp -R "$module_root/e2e" "$e2e"
awk -v version="$VERSION" '
  /^local_path_override\(/ { skip = 1 }
  skip { if ($0 ~ /^\)/) skip = 0; next }
  $0 == "bazel_dep(name = \"krit\")" { print "bazel_dep(name = \"krit\", version = \"" version "\")"; found = 1; next }
  { print }
  END { if (!found) exit 1 }
' "$module_root/e2e/MODULE.bazel" > "$e2e/MODULE.bazel" || {
  echo "ERROR: bazel/e2e/MODULE.bazel has no bazel_dep(name = \"krit\") line" >&2
  exit 1
}

cd "$e2e"
env -u KRIT_BAZEL_RELEASE "$BAZEL" test \
  --lockfile_mode=off \
  --registry="file://$WORK/registry" \
  --registry=https://bcr.bazel.build \
  --test_output=errors \
  //...
