#!/usr/bin/env bash
set -euo pipefail
image="${1:-markdown-docs:local}"
name="markdown-docs-smoke-${RANDOM}-$$"
volume="${name}-data"
annotate_failure() {
  local message="$1"
  if [ -n "${GITHUB_ACTIONS:-}" ]; then
    message="${message//'%'/'%25'}"
    message="${message//$'\n'/'%0A'}"
    message="${message//$'\r'/'%0D'}"
    echo "::error file=scripts/smoke-docker.sh,line=1::$message"
  fi
}
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
  details="$(docker inspect "$name" --format 'state={{.State.Status}} exit={{.State.ExitCode}} error={{.State.Error}} health={{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' 2>&1 || true)"
  logs="$(docker logs --tail=80 "$name" 2>&1 || true)"
  annotate_failure "Container did not become ready. ${details}. Logs: ${logs}"
  echo "Container did not become ready. ${details}" >&2
  return 1
}
start
address="$(docker port "$name" 8080/tcp)"
curl --fail --silent "http://${address}/" | grep -q '<div id="app">'
curl --fail --silent "http://${address}/healthz" | grep -q 'ok'
curl --fail --silent "http://${address}/api/v1/site" | grep -q 'container'
test "$(curl -s -o /dev/null -w '%{http_code}' "http://${address}/api/v1/missing")" = 404
test "$(curl -s -o /dev/null -w '%{http_code}' "http://${address}/assets/missing.js")" = 404
# The image starts as root only for bind-mount preparation, then execs the
# actual HTTP process as 10001. Check the running process rather than the
# default user used by a new docker exec session.
docker top "$name" -eo uid | awk 'NR > 1 && $1 == "10001" { found = 1 } END { exit(found ? 0 : 1) }'
docker exec "$name" sh -c 'test -d /data/content && test -d /data/media && test -d /data/backups; echo retained > /data/content/smoke.txt; echo attachment > /data/media/smoke.txt'
test "$(curl --fail --silent "http://${address}/media/smoke.txt")" = attachment
test "$(curl -s -o /dev/null -w '%{http_code}' "http://${address}/media/../content/smoke.txt")" = 404
docker rm -f "$name" >/dev/null
start
test "$(docker exec "$name" cat /data/content/smoke.txt)" = retained
echo 'Container smoke and persistence checks passed.'
