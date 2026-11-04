#!/usr/bin/env bash
set -Eeuo pipefail
INSTALL_DIR="${INSTALL_DIR:-/opt/open-ai-canvas}"
REPOSITORY="${REPOSITORY:-ws4934/open-ai-canvas}"
http_port_explicit="${CANVAS_HTTP_PORT+x}"
http_bind_explicit="${CANVAS_HTTP_BIND+x}"
CANVAS_HTTP_PORT="${CANVAS_HTTP_PORT:-3000}"
CANVAS_HTTP_BIND="${CANVAS_HTTP_BIND:-127.0.0.1}"
overwrite=false
confirmed=false
source_tree=''
fail() { printf '安装失败：%s\n' "$*" >&2; exit 1; }
while (($#)); do
  case "$1" in
    --overwrite) overwrite=true;;
    --yes) confirmed=true;;
    --source-tree) shift; [[ $# -gt 0 ]] || fail "缺少源码目录"; source_tree="$1";;
    --help) printf '用法：bash install-server-image.sh [--overwrite [--yes]]\n覆盖安装会重建本部署的数据库、资源、Redis、配置和管理员。\n'; exit 0;;
    *) fail "未知参数：$1";;
  esac
  shift
done
[[ "$confirmed" == false || "$overwrite" == true ]] || fail "--yes 只能与 --overwrite 一起使用"
[[ "$(uname -s)" == Linux ]] || fail "仅支持 Linux 服务器"
[[ "$REPOSITORY" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || fail "无效仓库名"
[[ "$CANVAS_HTTP_PORT" =~ ^[0-9]+$ ]] && ((CANVAS_HTTP_PORT >= 1 && CANVAS_HTTP_PORT <= 65535)) || fail "无效端口"
[[ "$CANVAS_HTTP_BIND" == 127.0.0.1 || "$CANVAS_HTTP_BIND" == 0.0.0.0 ]] || fail "绑定地址应为 127.0.0.1 或 0.0.0.0"
for command in curl openssl python3 sha256sum; do command -v "$command" >/dev/null || fail "请先安装 $command"; done
command -v docker >/dev/null && docker compose version >/dev/null 2>&1 || fail "请先安装 Docker Engine 和 Docker Compose v2"
INSTALL_DIR=$(python3 - "$INSTALL_DIR" <<'PY'
import os, pathlib, pwd, sys
path = pathlib.Path(sys.argv[1])
if not path.is_absolute() or str(path) != os.path.abspath(path):
    sys.exit("安装目录必须是无 .. 的绝对路径")
protected = {"/", "/opt", "/srv", "/home", "/root", "/usr", "/var", "/etc", "/tmp", "/run", os.path.expanduser("~")}
protected.update(user.pw_dir for user in pwd.getpwall())
if str(path) in protected or any(parent.is_symlink() for parent in (path, *path.parents)):
    sys.exit("拒绝根目录、家目录、系统目录或符号链接安装路径")
if path.exists() and not path.is_dir():
    sys.exit("安装路径不是目录")
print(path)
PY
) || fail "安装目录不安全"
existing=false
if [[ -d "$INSTALL_DIR" && -n "$(find "$INSTALL_DIR" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
  existing=true
  [[ "$overwrite" == true ]] || fail "安装目录不为空，拒绝覆盖；网页更新不能转换旧 fork 数据库时，请明确使用 --overwrite 重装"
  [[ -f "$INSTALL_DIR/.env" && ! -L "$INSTALL_DIR/.env" && -f "$INSTALL_DIR/docker-compose.deploy.yml" && ! -L "$INSTALL_DIR/docker-compose.deploy.yml" ]] || fail "不能识别已有安装：缺少普通 .env / docker-compose.deploy.yml 文件"
  for file in .canvas-overwrite-recovery.json .canvas-overwrite-recovery.sh; do
    [[ ! -L "$INSTALL_DIR/$file" && ( ! -e "$INSTALL_DIR/$file" || -f "$INSTALL_DIR/$file" ) ]] || fail "恢复信息必须是普通文件"
  done
  if [[ -e /etc/systemd/system/open-ai-canvas-updater.service ]] || grep -Eq '^(CANVAS_UPDATER_|YINGCE_AGENT_)' "$INSTALL_DIR/.env"; then
    fail "检测到独立 Host Updater / Agent 配置，请先人工停用并核对旧部署；脚本不会接管它"
  fi
fi
umask 077
temporary=$(mktemp -d)
stage=''
backup=''
switching=false
cleanup() {
  local status=$?
  rm -rf "$temporary"
  [[ -z "$stage" || ! -d "$stage" ]] || rm -rf "$stage"
  if ((status != 0)) && [[ "$switching" == true ]]; then
    printf '覆盖安装未完成；安装位置：%s。\n' "$INSTALL_DIR" >&2
    if [[ -n "$backup" && -d "$backup" ]]; then
      printf '旧安装已隔离于：%s；不要把旧数据接回新程序。恢复信息见其中 .canvas-overwrite-recovery.json。\n' "$backup" >&2
      printf '人工恢复：停止新项目 → 隔离失败目录 → 将旧目录移回 → 启动旧项目。\n以安装时相同权限执行：bash %q\n' "$backup/.canvas-overwrite-recovery.sh" >&2
    else
      printf '旧目录仍在原位置；核对其中 .canvas-overwrite-recovery.json 和容器状态后人工恢复。\n' >&2
      printf '人工恢复原 Compose 项目（以安装时相同权限执行）：bash %q\n' "$INSTALL_DIR/.canvas-overwrite-recovery.sh" >&2
    fi
  fi
}
trap cleanup EXIT
# .env is authoritative; caller overrides must not redirect an existing deployment's mounts.
compose_at() {
  local directory="$1"; shift
  local -a clean=(env)
  local variable
  while IFS= read -r variable; do
    case "$variable" in CANVAS_*|POSTGRES_*|COMPOSE_*|YINGCE_*|PG*|DATABASE_URL|REDIS_URL) clean+=(-u "$variable");; esac
  done < <(compgen -e)
  "${clean[@]}" docker compose --project-directory "$directory" --env-file "$directory/.env" -f "$directory/docker-compose.deploy.yml" "$@"
}
cat >"$temporary/validate.py" <<'PY'
import json, os, pathlib, sys
mode, directory, filename = sys.argv[1:4]
root = pathlib.Path(directory)
expected = {"backend": ("data", "/data"), "postgres": ("postgres-data", "/var/lib/postgresql/data"), "redis": ("redis-data", "/data")}
def reject(message):
    sys.exit("拒绝覆盖：" + message)
def check_mounts(service, mounts, live=False):
    folder, target = expected[service]
    source = str(root / folder)
    if len(mounts) != 1:
        reject(service + " 存在额外或缺失的数据挂载")
    mount = mounts[0]
    keys = ("Type", "Source", "Destination") if live else ("type", "source", "target")
    if tuple(mount.get(key) for key in keys) != ("bind", source, target):
        reject(service + " 不是安装目录内的受管 bind 数据目录")
    if os.path.realpath(source) != source:
        reject(service + " 数据目录包含符号链接")
with open(filename) as handle:
    data = json.load(handle)
if mode == "config":
    services = data.get("services", {})
    if set(services) != set(expected):
        reject("仅支持 backend、postgres、redis 三容器统一应用布局")
    project = data.get("name", "")
    if not project or not all(c.isalnum() or c in "_-" for c in project):
        reject("无效 Compose 项目标识")
    for service, value in services.items():
        check_mounts(service, value.get("volumes", []))
        if value.get("container_name") or value.get("network_mode") or value.get("privileged"):
            reject("不支持自定义容器名称、网络模式或特权容器")
    backend = services["backend"].get("environment", {})
    if backend.get("CANVAS_DATABASE_DRIVER") != "postgres" or backend.get("CANVAS_MANAGED_BINARY_PATH") != "/data/system/bin/yingce-backend" or backend.get("YINGCE_AGENT_URL"):
        reject("不是受管统一程序部署")
    ports = services["backend"].get("ports", [])
    if len(ports) != 1 or ports[0].get("target") != 8080 or ports[0].get("protocol", "tcp") != "tcp":
        reject("不支持自定义业务端口映射")
    port = str(ports[0].get("published", ""))
    if not port.isdigit() or not 1 <= int(port) <= 65535 or ports[0].get("host_ip", "0.0.0.0") not in ("127.0.0.1", "0.0.0.0"):
        reject("不支持的业务监听地址或端口")
    for network in data.get("networks", {}).values():
        if network.get("external"):
            reject("不支持外部网络")
    print(project)
else:
    project = sys.argv[4]
    seen = set()
    for container in data:
        labels = container.get("Config", {}).get("Labels", {})
        service = labels.get("com.docker.compose.service")
        if labels.get("com.docker.compose.project") != project or labels.get("com.docker.compose.project.working_dir") != directory or service not in expected or service in seen:
            reject("Compose 项目含有其他目录、重复或未知服务的容器")
        files = set(labels.get("com.docker.compose.project.config_files", "").split(","))
        allowed = {str(root / "docker-compose.deploy.yml"), str(root / "docker-compose.build.yml")}
        if str(root / "docker-compose.deploy.yml") not in files or not files <= allowed:
            reject("容器使用了未知 Compose 配置")
        check_mounts(service, container.get("Mounts", []), True)
        seen.add(service)
        print(container["Id"])
PY
old_project=''
old_ids=()
if [[ "$existing" == true ]]; then
  compose_at "$INSTALL_DIR" config --format json >"$temporary/old-config.json"
  old_project=$(python3 "$temporary/validate.py" config "$INSTALL_DIR" "$temporary/old-config.json")
  python3 - "$temporary/old-config.json" >"$temporary/old-listener" <<'PY'
import json, sys
with open(sys.argv[1]) as handle:
    port = json.load(handle)["services"]["backend"]["ports"][0]
print(port.get("host_ip", "0.0.0.0"))
print(port["published"])
PY
  old_bind=$(sed -n '1p' "$temporary/old-listener")
  old_port=$(sed -n '2p' "$temporary/old-listener")
  [[ -n "$http_bind_explicit" ]] || CANVAS_HTTP_BIND="$old_bind"
  [[ -n "$http_port_explicit" ]] || CANVAS_HTTP_PORT="$old_port"
  docker ps -aq --filter "label=com.docker.compose.project=$old_project" >"$temporary/old-ids"
  docker ps -aq --filter "label=com.docker.compose.project.working_dir=$INSTALL_DIR" >"$temporary/directory-ids"
  cmp -s "$temporary/old-ids" "$temporary/directory-ids" || fail "安装目录存在其他 Compose 项目的容器，请先人工核对"
  while IFS= read -r id; do [[ -z "$id" ]] || old_ids+=("$id"); done <"$temporary/old-ids"
  if (("${#old_ids[@]}")); then
    docker inspect "${old_ids[@]}" >"$temporary/old-containers.json"
    python3 "$temporary/validate.py" containers "$INSTALL_DIR" "$temporary/old-containers.json" "$old_project" >/dev/null
  fi
  printf '覆盖安装：%s\n将重建本部署的数据库、Redis、资源、配置和管理员状态。\n受管目录：%s/{data,postgres-data,redis-data}\n旧 Compose 项目：%s；旧目录会隔离，不会接入新应用。\n' "$INSTALL_DIR" "$INSTALL_DIR" "$old_project"
  printf '业务监听：%s:%s → %s:%s（外部反向代理不变更）。\n' "$old_bind" "$old_port" "$CANVAS_HTTP_BIND" "$CANVAS_HTTP_PORT"
  if [[ "$confirmed" == false ]]; then
    printf '请输入完整安装目录确认（非交互可使用 --overwrite --yes）：'
    IFS= read -r answer || fail "未确认覆盖安装"
    [[ "$answer" == "$INSTALL_DIR" ]] || fail "确认目录不匹配，已取消"
  fi
fi
mkdir -p "$(dirname "$INSTALL_DIR")"
stage=$(mktemp -d "${INSTALL_DIR}.install.XXXXXXXX")
nonce=$(openssl rand -hex 8)
if [[ -n "$source_tree" ]]; then
  [[ -f "$source_tree/Dockerfile" && -f "$source_tree/docker-compose.deploy.yml" && -f "$source_tree/docker-compose.build.yml" ]] || fail "源码缺少统一部署文件"
  cp -a "$source_tree/." "$stage/"
  [[ ! -L "$stage/.env" && ! -L "$stage/docker-compose.deploy.yml" ]] || fail "源码安装配置不能是符号链接"
  for folder in data postgres-data redis-data; do
    [[ ! -e "$stage/$folder" && ! -L "$stage/$folder" ]] || fail "源码目录包含运行数据，拒绝作为全新安装"
  done
  image="open-ai-canvas:source-$nonce"
  version=source
else
  version="${CANVAS_IMAGE_TAG:-}"
  if [[ -z "$version" ]]; then
    version=$(curl -fsSL "https://api.github.com/repos/$REPOSITORY/releases/latest" | python3 -c 'import json,sys; print(json.load(sys.stdin)["tag_name"])')
  fi
  version="v${version#v}"
  [[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(\.[0-9]+)?(-[0-9A-Za-z.-]+)?$ ]] || fail "无效 Release 版本"
  image="ghcr.io/$(printf '%s' "$REPOSITORY" | tr '[:upper:]' '[:lower:]'):${version#v}"
  case "$(uname -m)" in x86_64|amd64) arch=amd64;; aarch64|arm64) arch=arm64;; *) fail "不支持的架构";; esac
  base="https://github.com/$REPOSITORY/releases/download/$version"
  curl -fsSL "$base/SHA256SUMS" -o "$temporary/SHA256SUMS"
  curl -fsSL "$base/docker-compose.deploy.yml" -o "$stage/docker-compose.deploy.yml"
  grep -Eq "^[a-f0-9]{64}  open-ai-canvas_${version}_linux_${arch}\.tar\.gz$" "$temporary/SHA256SUMS" || fail "Release 缺少当前架构的程序包"
  curl -fsIL "$base/open-ai-canvas_${version}_linux_${arch}.tar.gz" >/dev/null
  (cd "$stage" && grep -E '  docker-compose.deploy.yml$' "$temporary/SHA256SUMS" | sha256sum -c -) || fail "Compose 校验失败"
  if [[ -n "${GHCR_USERNAME:-}" || -n "${GHCR_TOKEN:-}" ]]; then
    [[ -n "${GHCR_USERNAME:-}" && -n "${GHCR_TOKEN:-}" ]] || fail "GHCR 用户与 Token 必须同时提供"
    printf '%s' "$GHCR_TOKEN" | docker login ghcr.io --username "$GHCR_USERNAME" --password-stdin
  fi
fi
password=$(openssl rand -hex 32)
cat >"$stage/.env" <<EOF
COMPOSE_PROJECT_NAME=canvas_$nonce
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
CANVAS_AGENT_MAX_PROCESSES=${CANVAS_AGENT_MAX_PROCESSES:-30}
CANVAS_AGENT_NODE_MEMORY_MB=${CANVAS_AGENT_NODE_MEMORY_MB:-512}
CANVAS_WORKER_CONCURRENCY=${CANVAS_WORKER_CONCURRENCY:-3}
CANVAS_CHANNEL_CONCURRENCY=${CANVAS_CHANNEL_CONCURRENCY:-3}
CANVAS_REGISTRATION_ENABLED=false
CANVAS_CORS_ORIGINS=${CANVAS_CORS_ORIGINS:-}
CANVAS_PUBLIC_BASE_URL=${CANVAS_PUBLIC_BASE_URL:-}
CANVAS_TRUSTED_PROXIES=${CANVAS_TRUSTED_PROXIES:-}
EOF
compose_at "$stage" config --format json >"$temporary/new-config.json"
python3 "$temporary/validate.py" config "$stage" "$temporary/new-config.json" >/dev/null
if [[ -n "$source_tree" ]]; then
  printf 'services:\n  backend:\n    image: %s\n' "$image" >"$temporary/build-image.yml"
  compose_at "$stage" -f "$stage/docker-compose.build.yml" -f "$temporary/build-image.yml" build backend
  compose_at "$stage" pull postgres redis
else
  compose_at "$stage" pull || fail "镜像拉取失败；请检查网络、包可见性和认证"
fi
docker run --rm --entrypoint /opt/canvas/yingce-backend "$image" --check >"$temporary/check.json"
if [[ "$version" != source ]]; then
  python3 - "$temporary/check.json" "$version" <<'PY'
import json, sys
with open(sys.argv[1]) as handle:
    info = json.load(handle)
if str(info.get("version", "")).lstrip("v") != sys.argv[2].lstrip("v"):
    sys.exit("镜像程序版本与 Release 不一致")
PY
fi
if [[ "$existing" == true ]]; then
  # Stop only inspected IDs, not a project-wide down -v; leave unrelated volumes and networks alone.
  docker ps -aq --filter "label=com.docker.compose.project=$old_project" >"$temporary/current-ids"
  cmp -s "$temporary/old-ids" "$temporary/current-ids" || fail "旧项目容器已变化，请重新检查后执行"
  docker ps -aq --filter "label=com.docker.compose.project.working_dir=$INSTALL_DIR" >"$temporary/directory-ids"
  cmp -s "$temporary/old-ids" "$temporary/directory-ids" || fail "安装目录的 Compose 归属已变化，请重新检查后执行"
  if (("${#old_ids[@]}")); then
    docker inspect "${old_ids[@]}" >"$temporary/old-containers.json"
    python3 "$temporary/validate.py" containers "$INSTALL_DIR" "$temporary/old-containers.json" "$old_project" >/dev/null
  fi
  backup="${INSTALL_DIR}.replaced-$(date -u +%Y%m%dT%H%M%SZ)-$nonce"
  [[ ! -e "$backup" ]] || fail "隔离目录已存在"
  failed_directory="${INSTALL_DIR}.failed-$nonce"
  python3 - "$INSTALL_DIR" "$old_project" "$temporary/old-ids" "$backup" "$failed_directory" "canvas_$nonce" >"$INSTALL_DIR/.canvas-overwrite-recovery.json" <<'PY'
import datetime, json, sys
with open(sys.argv[3]) as handle:
    containers = handle.read().splitlines()
json.dump({"originalDirectory": sys.argv[1], "composeProject": sys.argv[2], "containers": containers, "backupDirectory": sys.argv[4], "failedDirectory": sys.argv[5], "replacementProject": sys.argv[6], "replacedAt": datetime.datetime.now(datetime.timezone.utc).isoformat(), "recoveryScript": ".canvas-overwrite-recovery.sh", "note": "手工运行恢复脚本：仅停止并移除新项目容器，隔离失败目录，将旧目录移回原位，按旧配置启动。若旧目录尚未移动，只恢复原项目。不会自动执行，也不删除数据目录或卷。"}, sys.stdout, ensure_ascii=False, indent=2)
PY
  {
    printf '#!/usr/bin/env bash\nset -Eeuo pipefail\n'
    printf 'original=%q\nbackup=%q\nfailed=%q\nold_project=%q\nnew_project=%q\n' "$INSTALL_DIR" "$backup" "$failed_directory" "$old_project" "canvas_$nonce"
    declare -f compose_at
    cat <<'RECOVERY'
fail() { printf '恢复停止：%s\n' "$*" >&2; exit 1; }
python3 - "$original" "$backup" "$failed" "$new_project" <<'PY'
import json, os, pathlib, sys
original, backup, failed, project = sys.argv[1:]
for path in (original, backup, failed):
    if os.path.realpath(path) != path:
        sys.exit("恢复目录包含符号链接，拒绝操作")
record = pathlib.Path(backup if os.path.isdir(backup) else original) / ".canvas-overwrite-recovery.json"
if record.is_symlink():
    sys.exit("恢复记录不能是符号链接")
with record.open() as handle:
    metadata = json.load(handle)
if metadata.get("originalDirectory") != original or metadata.get("replacementProject") != project:
    sys.exit("恢复记录与本次安装不匹配")
PY
if [[ -d "$backup" ]]; then
  [[ ! -e "$failed" ]] || fail "失败目录已存在：$failed"
  if [[ -e "$original" ]]; then
    [[ -d "$original" && -f "$original/.env" && ! -L "$original/.env" && -f "$original/docker-compose.deploy.yml" && ! -L "$original/docker-compose.deploy.yml" ]] || fail "新安装目录结构已变化"
    grep -Fxq "COMPOSE_PROJECT_NAME=$new_project" "$original/.env" || fail "新安装目录已属于其他项目"
    compose_at "$original" --project-name "$new_project" stop
    compose_at "$original" --project-name "$new_project" rm --force
    mv "$original" "$failed"
  fi
  mv "$backup" "$original"
fi
compose_at "$original" --project-name "$old_project" up -d --wait --wait-timeout 600
printf '旧安装已按原配置启动：%s\n' "$original"
[[ ! -d "$failed" ]] || printf '失败的新安装保留在：%s\n' "$failed"
RECOVERY
  } >"$INSTALL_DIR/.canvas-overwrite-recovery.sh"
  switching=true
  if (("${#old_ids[@]}")); then
    docker stop --time 600 "${old_ids[@]}"
    docker rm "${old_ids[@]}"
  fi
  mv "$INSTALL_DIR" "$backup"
elif [[ -d "$INSTALL_DIR" ]]; then
  rmdir "$INSTALL_DIR" || fail "安装目录在准备期间发生变化"
fi
mv "$stage" "$INSTALL_DIR"
stage=''
chmod 0750 "$INSTALL_DIR"
compose_at "$INSTALL_DIR" up -d --wait --wait-timeout 600
switching=false
printf '\n部署完成：%s:%s；外部反向代理直接指向此端口。\n' "$CANVAS_HTTP_BIND" "$CANVAS_HTTP_PORT"
printf '安装目录：%s。后续在管理后台检查更新；脚本不会配置 Nginx 或证书。\n' "$INSTALL_DIR"
[[ -z "$backup" ]] || printf '旧安装隔离目录：%s（未接入新应用；确认新部署可用后可人工清理）。\n' "$backup"
