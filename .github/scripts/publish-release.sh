#!/usr/bin/env bash
set -euo pipefail
: "${IS_RELEASE:?}" "${RELEASE_VERSION:?}" "${GITHUB_REPOSITORY:?}"
if [[ "$IS_RELEASE" != true ]]; then
  exec bash .github/scripts/promote-image.sh
fi
[[ "$RELEASE_VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?(-[0-9A-Za-z.-]+)?$ ]] || exit 1
error_file=$(mktemp)
trap 'rm -f "$error_file"' EXIT
existing=''
if existing=$(gh api "repos/$GITHUB_REPOSITORY/releases/tags/$RELEASE_VERSION" 2>"$error_file"); then
  [[ "$(jq -r .draft <<<"$existing")" == true ]] || { echo "Published releases are immutable; choose a new version" >&2; exit 1; }
  [[ "$(jq -r .target_commitish <<<"$existing")" == "$GITHUB_SHA" ]] || { echo "Draft belongs to a different commit" >&2; exit 1; }
elif ! grep -q 'HTTP 404' "$error_file"; then
  cat "$error_file" >&2; exit 1
fi
assets=()
for arch in amd64 arm64; do
  file="dist/open-ai-canvas_${RELEASE_VERSION}_linux_${arch}.tar.gz"
  test -s "$file"
  [[ "$(tar -tzf "$file")" == yingce-backend ]] || { echo "Invalid release archive" >&2; exit 1; }
  assets+=("$file")
done
cp docker-compose.deploy.yml dist/docker-compose.deploy.yml
(cd dist && sha256sum "open-ai-canvas_${RELEASE_VERSION}_linux_amd64.tar.gz" "open-ai-canvas_${RELEASE_VERSION}_linux_arm64.tar.gz" docker-compose.deploy.yml > SHA256SUMS)
assets+=(dist/SHA256SUMS dist/docker-compose.deploy.yml)
awk -v heading="## $RELEASE_VERSION" '$0 == heading { found=1; next } found && /^## / { exit } found { print }' CHANGELOG.md > dist/RELEASE_NOTES.md
test -s dist/RELEASE_NOTES.md
if [[ -z "$existing" ]]; then
  gh release create "$RELEASE_VERSION" --repo "$GITHUB_REPOSITORY" --verify-tag --target "$GITHUB_SHA" --draft --title "$RELEASE_VERSION" --notes-file dist/RELEASE_NOTES.md
fi
# Clobber is limited to a draft at this exact SHA; a public release is never overwritten.
gh release upload "$RELEASE_VERSION" "${assets[@]}" --repo "$GITHUB_REPOSITORY" --clobber
uploaded=$(gh release view "$RELEASE_VERSION" --repo "$GITHUB_REPOSITORY" --json assets --jq '.assets[].name')
for asset in "${assets[@]}"; do
  grep -Fxq "${asset##*/}" <<<"$uploaded" || { echo "Release asset was not uploaded" >&2; exit 1; }
done
bash .github/scripts/promote-image.sh
flags=(--draft=false)
if [[ "$RELEASE_VERSION" == *-* ]]; then flags+=(--prerelease --latest=false); else flags+=(--prerelease=false --latest); fi
gh release edit "$RELEASE_VERSION" --repo "$GITHUB_REPOSITORY" "${flags[@]}"
