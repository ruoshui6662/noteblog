#!/usr/bin/env bash
set -euo pipefail
image="${1:-markdown-docs:local}"
name="markdown-docs-smoke-${RANDOM}-$$"
volume="${name}-data"
cleanup() {
  docker logs "$name" 2>/dev/null || true
  docker rm -f "$name" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
}
trap cleanup EXIT
docker volume create "$volume" >/dev/null
start() {
  docker run -d --name "$name" --read-only --cap-drop ALL \
    --security-opt no-new-privileges:true -p 127.0.0.1::8080 \
    -v "$volume:/data" "$image" >/dev/null
  for attempt in $(seq 1 30); do
    if docker exec "$name" /usr/local/bin/markdown-docs healthcheck; then
      return
    fi
    sleep 1
  done
  echo 'Container did not become ready' >&2
  return 1
}
start
address="$(docker port "$name" 8080/tcp)"
curl --fail --silent "http://${address}/" | grep -q '<div id="app">'
curl --fail --silent "http://${address}/healthz" | grep -q 'ok'
curl --fail --silent "http://${address}/api/v1/site" | grep -q 'container'
test "$(curl -s -o /dev/null -w '%{http_code}' "http://${address}/api/v1/missing")" = 404
test "$(curl -s -o /dev/null -w '%{http_code}' "http://${address}/assets/missing.js")" = 404
test "$(docker exec "$name" id -u)" = 10001
docker exec "$name" sh -c 'test -d /data/content && test -d /data/media && test -d /data/backups; echo retained > /data/content/smoke.txt'
docker rm -f "$name" >/dev/null
start
test "$(docker exec "$name" cat /data/content/smoke.txt)" = retained
echo 'Container smoke and persistence checks passed.'
