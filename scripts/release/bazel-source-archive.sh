#!/usr/bin/env bash
# Build the source archive of the krit Bazel module (bazel/) for a release
# tag. The Bazel Central Registry entry points at this release asset
# (.bcr/bazel/source.template.json), because BCR only treats
# releases/download URLs as stable; GitHub's generated tag archives are not
# guaranteed to keep their checksum.
#
# Usage: scripts/release/bazel-source-archive.sh TAG OUTDIR
#
# Writes OUTDIR/krit-bazel-TAG.tar.gz holding krit-bazel-TAG/bazel/...,
# hence strip_prefix "krit-bazel-{TAG}/bazel". Archiving the tag (not the
# bazel/ tree object) stamps files with the commit time, and gzip -n omits
# the timestamp and name, so a rebuild of the same tag is byte-identical.

set -euo pipefail

TAG="${1:?usage: bazel-source-archive.sh TAG OUTDIR}"
OUTDIR="${2:?usage: bazel-source-archive.sh TAG OUTDIR}"

if ! git rev-parse -q --verify "refs/tags/${TAG}^{commit}" > /dev/null; then
  echo "ERROR: tag ${TAG} not found" >&2
  exit 1
fi
if ! git cat-file -e "${TAG}:bazel/MODULE.bazel" 2> /dev/null; then
  echo "ERROR: ${TAG} has no bazel/MODULE.bazel" >&2
  exit 1
fi

mkdir -p "$OUTDIR"
out="${OUTDIR}/krit-bazel-${TAG}.tar.gz"
git archive --format=tar --prefix="krit-bazel-${TAG}/" "$TAG" bazel | gzip -n -9 > "$out"
echo "$out"
