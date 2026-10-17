#!/usr/bin/env bash
set -euo pipefail
: "${IMAGE:?}" "${GITHUB_SHA:?}" "${DIGEST_DIR:?}" "${EXPECTED_DIGESTS:?}"
[[ "$GITHUB_SHA" =~ ^[0-9a-f]{40}$ ]] || { echo "Invalid commit SHA" >&2; exit 1; }
shopt -s nullglob
files=("$DIGEST_DIR"/*)
[[ ${#files[@]} -eq "$EXPECTED_DIGESTS" ]] || { echo "Incomplete image digest set" >&2; exit 1; }
sources=()
for file in "${files[@]}"; do
  digest=${file##*/}
  [[ "$digest" =~ ^[0-9a-f]{64}$ ]] || { echo "Invalid image digest" >&2; exit 1; }
  sources+=("$IMAGE@sha256:$digest")
done
tags=(--tag "$IMAGE:sha-$GITHUB_SHA")
if [[ "${IS_RELEASE:-false}" == true ]]; then
  [[ "${RELEASE_VERSION:?}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?(-[0-9A-Za-z.-]+)?$ ]] || exit 1
  tags+=(--tag "$IMAGE:${RELEASE_VERSION#v}")
fi
docker buildx imagetools create "${tags[@]}" "${sources[@]}"
