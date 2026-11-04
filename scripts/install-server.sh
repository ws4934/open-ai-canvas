#!/usr/bin/env bash
set -Eeuo pipefail
# Source and Release installations share the same ownership checks and replacement flow.
REPOSITORY_URL="${REPOSITORY_URL:-https://github.com/ws4934/open-ai-canvas.git}"
REPOSITORY_REF="${REPOSITORY_REF:-main}"
for argument in "$@"; do
  case "$argument" in
    --overwrite|--yes) ;;
    --help) printf '用法：bash install-server.sh [--overwrite [--yes]]\n'; exit 0;;
    *) printf '安装失败：未知参数 %s\n' "$argument" >&2; exit 1;;
  esac
done
for command in git docker openssl python3; do command -v "$command" >/dev/null || { printf '请先安装 %s\n' "$command" >&2; exit 1; }; done
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT
git clone --depth 1 --branch "$REPOSITORY_REF" -- "$REPOSITORY_URL" "$temporary/source"
grep -Fq -- '--source-tree)' "$temporary/source/scripts/install-server-image.sh" || {
  printf '安装失败：目标源码不支持统一安装器，请选择包含覆盖安装入口的新版本。\n' >&2
  exit 1
}
bash "$temporary/source/scripts/install-server-image.sh" --source-tree "$temporary/source" "$@"
