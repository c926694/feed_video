#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

export DOCKER_BUILDKIT=1
export BUILDX_NO_DEFAULT_ATTESTATIONS=1

echo "[1/5] Validate docker compose config..."
docker compose config >/dev/null

echo "[2/5] Pull latest base images..."
docker compose pull || true

echo "[3/5] Build images..."
docker compose build --progress=plain

echo "[4/5] Start containers..."
docker compose up -d --remove-orphans

echo "[5/5] Current container status:"
docker compose ps

if docker compose ps --format json | grep -q '"State":"exited"\|"State":"dead"'; then
  echo
  echo "Some containers are not running. Recent logs:"
  docker compose logs --tail=100
  exit 1
fi