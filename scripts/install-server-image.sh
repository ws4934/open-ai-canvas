#!/usr/bin/env bash
set -Eeuo pipefail
INSTALL_DIR="${INSTALL_DIR:-/opt/open-ai-canvas}"
REPOSITORY="${REPOSITORY:-ws4934/open-ai-canvas}"
CANVAS_HTTP_PORT="${CANVAS_HTTP_PORT:-3000}"
CANVAS_HTTP_BIND="${CANVAS_HTTP_BIND:-127.0.0.1}"
fail() { printf '安装失败：%s\n' "$*" >&2; exit 1; }
[[ "$(uname -s)" == Linux ]] || fail "仅支持 Linux 服务器"
[[ "$REPOSITORY" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || fail "无效仓库名"
[[ "$CANVAS_HTTP_PORT" =~ ^[0-9]+$ ]] && ((CANVAS_HTTP_PORT >= 1 && CANVAS_HTTP_PORT <= 65535)) || fail "无效端口"
[[ "$CANVAS_HTTP_BIND" == 127.0.0.1 || "$CANVAS_HTTP_BIND" == 0.0.0.0 ]] || fail "绑定地址应为 127.0.0.1 或 0.0.0.0"
if [[ -e "$INSTALL_DIR/.env" || -e "$INSTALL_DIR/docker-compose.deploy.yml" ]]; then
  fail "安装目录已有配置，拒绝覆盖；后续版本在网页更新，安装未完成时请先检查现有 Compose"
fi
for command in curl openssl python3 sha256sum; do command -v "$command" >/dev/null || fail "请先安装 $command"; done
if ! command -v docker >/dev/null || ! docker compose version >/dev/null 2>&1; then
  fail "请先安装 Docker Engine 和 Docker Compose v2"
fi
version="${CANVAS_IMAGE_TAG:-}"
if [[ -z "$version" ]]; then
  version=$(curl -fsSL "https://api.github.com/repos/$REPOSITORY/releases/latest" | python3 -c 'import json,sys; print(json.load(sys.stdin)["tag_name"])')
fi
version="v${version#v}"
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?(-[0-9A-Za-z.-]+)?$ ]] || fail "无效 Release 版本"
image="ghcr.io/${REPOSITORY,,}:${version#v}"
case "$(uname -m)" in x86_64|amd64) arch=amd64;; aarch64|arm64) arch=arm64;; *) fail "不支持的架构";; esac
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT
base="https://github.com/$REPOSITORY/releases/download/$version"
curl -fsSL "$base/SHA256SUMS" -o "$temporary/SHA256SUMS"
curl -fsSL "$base/docker-compose.deploy.yml" -o "$temporary/docker-compose.deploy.yml"
grep -Eq "^[a-f0-9]{64}  open-ai-canvas_${version}_linux_${arch}\.tar\.gz$" "$temporary/SHA256SUMS" || fail "Release 缺少当前架构的程序包"
curl -fsIL "$base/open-ai-canvas_${version}_linux_${arch}.tar.gz" >/dev/null
(cd "$temporary" && grep -E '  docker-compose.deploy.yml$' SHA256SUMS | sha256sum -c -) || fail "Compose 校验失败"
if [[ -n "${GHCR_USERNAME:-}" || -n "${GHCR_TOKEN:-}" ]]; then
  [[ -n "${GHCR_USERNAME:-}" && -n "${GHCR_TOKEN:-}" ]] || fail "GHCR 用户与 Token 必须同时提供"
  printf '%s' "$GHCR_TOKEN" | docker login ghcr.io --username "$GHCR_USERNAME" --password-stdin
fi
docker pull "$image" || fail "镜像拉取失败；请检查网络、包可见性和认证"
docker run --rm --entrypoint /opt/canvas/yingce-backend "$image" --check >/dev/null
mkdir -p "$INSTALL_DIR"
chmod 0750 "$INSTALL_DIR"
umask 077
password=$(openssl rand -hex 32)
cat >"$INSTALL_DIR/.env" <<EOF
POSTGRES_DB=open_ai_canvas
POSTGRES_USER=open_ai_canvas
POSTGRES_PASSWORD=$password
CANVAS_IMAGE_TAG=${version#v}
CANVAS_IMAGE=$image
CANVAS_HTTP_BIND=$CANVAS_HTTP_BIND
CANVAS_HTTP_PORT=$CANVAS_HTTP_PORT
CANVAS_DATA_PATH=./data
CANVAS_POSTGRES_DATA_PATH=./postgres-data
CANVAS_REDIS_DATA_PATH=./redis-data
CANVAS_REGISTRATION_ENABLED=false
CANVAS_CORS_ORIGINS=${CANVAS_CORS_ORIGINS:-}
CANVAS_PUBLIC_BASE_URL=${CANVAS_PUBLIC_BASE_URL:-}
CANVAS_TRUSTED_PROXIES=${CANVAS_TRUSTED_PROXIES:-}
EOF
cp "$temporary/docker-compose.deploy.yml" "$INSTALL_DIR/docker-compose.deploy.yml"
cd "$INSTALL_DIR"
docker compose --env-file .env -f docker-compose.deploy.yml up -d --wait --wait-timeout 600
printf '\n部署完成：%s:%s；外部反向代理直接指向此端口。\n' "$CANVAS_HTTP_BIND" "$CANVAS_HTTP_PORT"
printf '安装目录：%s。后续在管理后台检查更新；脚本不会配置 Nginx 或证书。\n' "$INSTALL_DIR"
