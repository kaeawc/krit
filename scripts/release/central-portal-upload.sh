#!/usr/bin/env bash
set -euo pipefail

: "${SONATYPE_USERNAME:?SONATYPE_USERNAME is required}"
: "${SONATYPE_PASSWORD:?SONATYPE_PASSWORD is required}"

CENTRAL_NAMESPACE="${CENTRAL_NAMESPACE:-dev.jasonpearson.krit}"
CENTRAL_PUBLISHING_TYPE="${CENTRAL_PUBLISHING_TYPE:-automatic}"

token="$(printf '%s' "${SONATYPE_USERNAME}:${SONATYPE_PASSWORD}" | base64 | tr -d '\n')"
url="https://ossrh-staging-api.central.sonatype.com/manual/upload/defaultRepository/${CENTRAL_NAMESPACE}?publishing_type=${CENTRAL_PUBLISHING_TYPE}"

if ! curl --fail-with-body -sS -X POST \
  -H "Authorization: Bearer ${token}" \
  "${url}"; then
  printf '%s\n' 'central-portal-upload: finalize POST failed' >&2
  exit 1
fi
