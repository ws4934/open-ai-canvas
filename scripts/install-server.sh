#!/usr/bin/env bash
set -Eeuo pipefail
# Explicit source installations use the same single-application Dockerfile.
INSTALL_DIR="${INSTALL_DIR:-/opt/open-ai-canvas}"
REPOSITORY_URL="${REPOSITORY_URL:-https://github.com/ws4934/open-ai-canvas.git}"
REPOSITORY_REF="${REPOSITORY_REF:-main}"
fail() { printf '安装失败：%s\n' "$*" >&2; exit 1; }
for command in git docker openssl; do command -v "$command" >/dev/null || fail "请先安装 $command"; done
docker compose version >/dev/null
[[ ! -e "$INSTALL_DIR" || -z "$(find "$INSTALL_DIR" -mindepth 1 -maxdepth 1 -print -quit)" ]] || fail "安装目录不为空，拒绝覆盖"
git clone --depth 1 --branch "$REPOSITORY_REF" "$REPOSITORY_URL" "$INSTALL_DIR"
cd "$INSTALL_DIR"
umask 077
password=$(openssl rand -hex 32)
cat >.env <<EOF
POSTGRES_DB=open_ai_canvas
POSTGRES_USER=open_ai_canvas
POSTGRES_PASSWORD=$password
CANVAS_IMAGE_TAG=source
CANVAS_IMAGE=open-ai-canvas:source
CANVAS_HTTP_BIND=${CANVAS_HTTP_BIND:-127.0.0.1}
CANVAS_HTTP_PORT=${CANVAS_HTTP_PORT:-3000}
CANVAS_DATA_PATH=./data
CANVAS_POSTGRES_DATA_PATH=./postgres-data
CANVAS_REDIS_DATA_PATH=./redis-data
CANVAS_REGISTRATION_ENABLED=false
CANVAS_CORS_ORIGINS=${CANVAS_CORS_ORIGINS:-}
CANVAS_PUBLIC_BASE_URL=${CANVAS_PUBLIC_BASE_URL:-}
CANVAS_TRUSTED_PROXIES=${CANVAS_TRUSTED_PROXIES:-}
EOF
docker compose --env-file .env -f docker-compose.deploy.yml -f docker-compose.build.yml build backend
docker compose --env-file .env -f docker-compose.deploy.yml -f docker-compose.build.yml up -d --wait --wait-timeout 600
printf '\n源码部署完成。只提供应用端口，不安装或修改 Nginx。\n'
