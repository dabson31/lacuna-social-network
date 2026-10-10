#!/usr/bin/env bash
#
# builds both images:
#   ./build.sh           build and start
#   ./build.sh --fresh   same, but discard the database volume first
#   ./build.sh --down    stop everything, keeping the volume

set -euo pipefail
cd "$(dirname "$0")"

FRONTEND_URL="http://localhost:3000"
BACKEND_URL="http://localhost:8080/api/health"

if docker compose version >/dev/null 2>&1; then
  dc() { docker compose "$@"; }
elif command -v docker-compose >/dev/null 2>&1; then
  dc() { docker-compose "$@"; }
else
  echo "docker compose is not installed, or the docker daemon is not reachable" >&2
  exit 1
fi

case "${1:-}" in
  --down)
    dc down
    echo "stopped. the database volume is still there; ./build.sh --fresh discards it"
    exit 0
    ;;
  --fresh)
    #? -v drops the named volume, so the next boot starts on an empty database and every
    #? migration runs from scratch. this is the state an auditor sees on a clean clone
    echo "removing the existing stack and its data volume"
    dc down -v
    ;;
esac

echo "building both images"
dc build

echo "starting"
#! --wait blocks until every service with a healthcheck reports healthy, which is the whole
#! point of this script: the backend applies migrations before it listens, so "started" and
#! "answering" are different moments on a cold volume
dc up -d --wait

echo
echo "containers"
docker ps -a --filter "name=social-network-" \
  --format "  {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}"

echo
echo "size on disk"
docker images --filter "reference=social-network-*" \
  --format "  {{.Repository}}\t{{.Size}}"

echo
printf "  backend  %s -> " "$BACKEND_URL"
curl -fsS "$BACKEND_URL" || echo "unreachable"
printf "  frontend %s -> " "$FRONTEND_URL/login"
curl -fsS -o /dev/null -w "HTTP %{http_code}\n" "$FRONTEND_URL/login" || echo "unreachable"

echo
echo "open $FRONTEND_URL in a browser. ./build.sh --down stops it."
