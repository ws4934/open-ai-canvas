#!/usr/bin/env bash
set -euo pipefail
image=${1:?image required}
docker run --rm --entrypoint /opt/canvas/yingce-backend "$image" --check
volume=$(docker volume create --label open-ai-canvas.test=single-app-smoke)
container=''
cleanup() {
  if [[ -n "$container" ]]; then docker logs "$container"; docker rm -f "$container" >/dev/null; fi
  docker volume rm "$volume" >/dev/null
}
trap cleanup EXIT
start() {
  container=$(docker run -d --init --mount "type=volume,src=$volume,dst=/data" -p 127.0.0.1::8080 "$image")
  port=$(docker port "$container" 8080/tcp | head -n 1)
}
wait_ready() {
  for _ in {1..60}; do
    if curl -fsS "http://$port/api/health/ready" >/dev/null; then return 0; fi
    sleep 2
  done
  return 1
}
start
wait_ready
curl -fsS "http://$port/" | grep -qi '<!doctype html>'
curl -fsS "http://$port/api/system/version" | jq -e --arg version "${RELEASE_VERSION:?}" '.data.build.version == $version'
# A sentinel mtime proves recreation has not recopied the image seed over the program.
docker exec "$container" touch -t 200001010000 /data/system/bin/yingce-backend
mtime=$(docker exec "$container" stat -c %Y /data/system/bin/yingce-backend)
docker restart "$container" >/dev/null
port=$(docker port "$container" 8080/tcp | head -n 1)
wait_ready
[[ "$(docker exec "$container" stat -c %Y /data/system/bin/yingce-backend)" == "$mtime" ]]
docker stop "$container" >/dev/null
docker rm "$container" >/dev/null
container=''
start
wait_ready
[[ "$(docker exec "$container" stat -c %Y /data/system/bin/yingce-backend)" == "$mtime" ]]
curl -fsS "http://$port/api/system/version" | jq -e --arg version "$RELEASE_VERSION" '.data.build.version == $version'
