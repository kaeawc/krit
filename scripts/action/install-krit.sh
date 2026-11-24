#!/usr/bin/env bash
# Resolve, download, verify and install a krit release for the root
# action.yml (`uses: kaeawc/krit@vX.Y.Z`).
#
# Usage: install-krit.sh resolve   # writes version/platform outputs
#        install-krit.sh install   # downloads and verifies into KRIT_DIR
#
# resolve env:
#   KRIT_VERSION   The `version` input: X.Y.Z, vX.Y.Z, "latest", or empty
#                  for the release the action itself is pinned to.
#   ACTION_REF     github.action_ref: the tag, branch or SHA the workflow
#                  pinned the action to.
#   ACTION_REPO    github.action_repository (owner/name).
#   KRIT_LIBC      The `libc` input: auto, glibc or musl.
#   GH_TOKEN       Token for the GitHub API (resolving SHA, major-tag and
#                  latest refs).
#   RUNNER_OS, RUNNER_ARCH  Set by the runner.
# install env:
#   KRIT_TAG, KRIT_ASSET, KRIT_DIR  From resolve.
#   KRIT_REPO      owner/name the release is downloaded from.
#   KRIT_VERIFY_ATTESTATION  "true" to also run `gh attestation verify`.
#
# Verification fails closed: a missing checksums.txt, a checksums.txt that
# does not list the archive, or a mismatching digest stops the install.

set -euo pipefail

api() {
  curl -fsSL --retry 3 \
    -H "Accept: application/vnd.github+json" \
    ${GH_TOKEN:+-H "Authorization: Bearer ${GH_TOKEN}"} \
    "https://api.github.com/repos/$1"
}

# stable_tag_for_ref prints the vX.Y.Z tag whose commit is the one ref
# names. Pinning the action by SHA (Dependabot's default) or by a floating
# major tag (v0) still installs the matching release.
stable_tag_for_ref() {
  local repo="$1" ref="$2" sha page tags tag
  sha=$(api "${repo}/commits/${ref}" | jq -r '.sha // empty')
  if [ -z "$sha" ]; then
    return 1
  fi
  for page in 1 2 3 4 5; do
    tags=$(api "${repo}/tags?per_page=100&page=${page}")
    tag=$(jq -r --arg sha "$sha" \
      '[.[] | select(.commit.sha == $sha) | .name | select(test("^v[0-9]+\\.[0-9]+\\.[0-9]+$"))] | first // empty' <<<"$tags")
    if [ -n "$tag" ]; then
      printf '%s\n' "$tag"
      return 0
    fi
    if [ "$(jq 'length' <<<"$tags")" -lt 100 ]; then
      break
    fi
  done
  return 1
}

resolve_tag() {
  local version="${KRIT_VERSION:-}" ref="${ACTION_REF:-}" repo="${ACTION_REPO:-kaeawc/krit}"
  case "$version" in
    latest)
      api "${repo}/releases/latest" | jq -r '.tag_name'
      return
      ;;
    "")
      ;;
    v*) printf '%s\n' "$version"; return ;;
    *) printf 'v%s\n' "$version"; return ;;
  esac
  if [[ "$ref" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    printf '%s\n' "$ref"
    return
  fi
  if [ -n "$ref" ] && stable_tag_for_ref "$repo" "$ref"; then
    return
  fi
  echo "::error::Cannot tell which krit release ${repo}@${ref:-<unknown>} is. Pin the action to a release tag (uses: kaeawc/krit@vX.Y.Z) or set the version input." >&2
  return 1
}

detect_platform() {
  local os arch libc="${KRIT_LIBC:-auto}"
  case "${RUNNER_OS:-$(uname -s)}" in
    Linux) os=linux ;;
    macOS|Darwin) os=darwin ;;
    Windows|MINGW*|MSYS*) os=windows ;;
    *) echo "::error::Unsupported runner OS: ${RUNNER_OS:-$(uname -s)}" >&2; return 1 ;;
  esac
  case "${RUNNER_ARCH:-$(uname -m)}" in
    X64|x86_64|amd64) arch=amd64 ;;
    ARM64|aarch64|arm64) arch=arm64 ;;
    *) echo "::error::Unsupported runner architecture: ${RUNNER_ARCH:-$(uname -m)}" >&2; return 1 ;;
  esac
  if [ "$os" = linux ] && [ "$libc" = auto ]; then
    libc=glibc
    if ldd --version 2>&1 | grep -qi musl || [ -f /etc/alpine-release ]; then
      libc=musl
    fi
  fi
  if [ "$os" = linux ] && [ "$libc" = musl ]; then
    if [ "$arch" != amd64 ]; then
      echo "::error::krit publishes musl builds for linux/amd64 only." >&2
      return 1
    fi
    printf 'linux_musl_amd64\n'
    return
  fi
  if [ "$os" = windows ] && [ "$arch" != amd64 ]; then
    echo "::error::krit publishes Windows builds for amd64 only." >&2
    return 1
  fi
  printf '%s_%s\n' "$os" "$arch"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

cmd_resolve() {
  local tag platform ext asset
  tag=$(resolve_tag)
  platform=$(detect_platform)
  ext=tar.gz
  if [[ "$platform" == windows_* ]]; then
    ext=zip
  fi
  asset="krit_${tag#v}_${platform}.${ext}"
  {
    echo "tag=${tag}"
    echo "version=${tag#v}"
    echo "platform=${platform}"
    echo "asset=${asset}"
  } >> "${GITHUB_OUTPUT:-/dev/stdout}"
}

cmd_install() {
  : "${KRIT_TAG:?}" "${KRIT_ASSET:?}" "${KRIT_DIR:?}"
  local repo="${KRIT_REPO:-kaeawc/krit}" base tmp expected actual bin
  bin=krit
  if [[ "$KRIT_ASSET" == *.zip ]]; then
    bin=krit.exe
  fi
  if [ -x "${KRIT_DIR}/${bin}" ]; then
    echo "krit ${KRIT_TAG} already installed in ${KRIT_DIR}"
    return 0
  fi
  base="https://github.com/${repo}/releases/download/${KRIT_TAG}"
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' RETURN
  if ! curl -fsSL --retry 3 -o "${tmp}/checksums.txt" "${base}/checksums.txt"; then
    echo "::error::Could not download checksums.txt for krit ${KRIT_TAG}; refusing to install an unverified binary." >&2
    return 1
  fi
  expected=$(awk -v name="$KRIT_ASSET" '$2 == name || $2 == "*" name {print $1}' "${tmp}/checksums.txt")
  if [ -z "$expected" ]; then
    echo "::error::checksums.txt for krit ${KRIT_TAG} does not list ${KRIT_ASSET}; refusing to install an unverified binary." >&2
    return 1
  fi
  curl -fsSL --retry 3 -o "${tmp}/${KRIT_ASSET}" "${base}/${KRIT_ASSET}"
  actual=$(sha256_of "${tmp}/${KRIT_ASSET}")
  if [ "$actual" != "$expected" ]; then
    echo "::error::SHA-256 mismatch for ${KRIT_ASSET}: expected ${expected}, got ${actual}." >&2
    return 1
  fi
  echo "Verified ${KRIT_ASSET} (sha256 ${actual})"
  if [ "${KRIT_VERIFY_ATTESTATION:-false}" = "true" ]; then
    gh attestation verify "${tmp}/${KRIT_ASSET}" --repo "$repo"
  fi
  mkdir -p "$KRIT_DIR"
  if [[ "$KRIT_ASSET" == *.zip ]]; then
    if command -v unzip >/dev/null 2>&1; then
      unzip -oq "${tmp}/${KRIT_ASSET}" -d "$KRIT_DIR"
    else
      7z x -y -o"$KRIT_DIR" "${tmp}/${KRIT_ASSET}" >/dev/null
    fi
  else
    tar -xzf "${tmp}/${KRIT_ASSET}" -C "$KRIT_DIR"
  fi
  if [ ! -f "${KRIT_DIR}/${bin}" ]; then
    echo "::error::${KRIT_ASSET} did not contain ${bin}." >&2
    return 1
  fi
}

case "${1:-}" in
  resolve) cmd_resolve ;;
  install) cmd_install ;;
  *) echo "usage: $0 resolve|install" >&2; exit 2 ;;
esac
