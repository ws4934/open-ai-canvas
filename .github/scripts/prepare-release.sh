#!/usr/bin/env bash
set -euo pipefail
: "${GITHUB_SHA:?}" "${GITHUB_EVENT_NAME:?}" "${GITHUB_REF:?}" "${GITHUB_OUTPUT:?}"
[[ "$GITHUB_SHA" =~ ^[0-9a-f]{40}$ ]] || { echo "Invalid commit SHA" >&2; exit 1; }
version=$(tr -d '\r\n' < VERSION)
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?(-[0-9A-Za-z.-]+)?$ ]] || { echo "Invalid VERSION" >&2; exit 1; }
release=false
if [[ "$GITHUB_EVENT_NAME" == workflow_dispatch ]]; then
  [[ "${REQUESTED_VERSION:?}" == "$version" ]] || { echo "Requested version differs from VERSION" >&2; exit 1; }
  release=true
elif [[ "$GITHUB_REF" == refs/tags/* ]]; then
  [[ "${GITHUB_REF#refs/tags/}" == "$version" ]] || { echo "Tag differs from VERSION" >&2; exit 1; }
  release=true
fi
if [[ "$release" == true ]]; then
  grep -Fqx "## $version" CHANGELOG.md || { echo "Missing release changelog" >&2; exit 1; }
  # A dispatch pins the selected commit. Never force or move an existing tag.
  if git show-ref --verify --quiet "refs/tags/$version"; then
    [[ "$(git rev-parse "$version^{commit}")" == "$GITHUB_SHA" ]] || { echo "Existing tag points at another commit" >&2; exit 1; }
  elif [[ "$GITHUB_EVENT_NAME" == workflow_dispatch ]]; then
    gh api --method POST "repos/${GITHUB_REPOSITORY:?}/git/refs" -f "ref=refs/tags/$version" -f "sha=$GITHUB_SHA" >/dev/null
  else
    echo "Release tag is missing" >&2; exit 1
  fi
fi
owner=$(printf '%s' "${REPOSITORY_OWNER:?}" | tr '[:upper:]' '[:lower:]')
printf 'version=%s\nrelease=%s\nbuild_time=%s\nowner=%s\n' "$version" "$release" "$(git show -s --format=%cI "$GITHUB_SHA")" "$owner" >> "$GITHUB_OUTPUT"
