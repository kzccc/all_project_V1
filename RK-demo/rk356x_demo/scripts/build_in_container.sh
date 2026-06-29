#!/bin/sh
set -eu

CONTAINER_NAME="${CONTAINER_NAME:-rk356x-build}"
IMAGE="${IMAGE:-ubuntu:22.04}"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required on the host machine." >&2
  exit 1
fi

if ! docker container inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
  docker run -d \
    --name "$CONTAINER_NAME" \
    -v "$PROJECT_DIR:/work" \
    -w /work \
    "$IMAGE" \
    sleep infinity
fi

if [ "$(docker inspect -f '{{.State.Running}}' "$CONTAINER_NAME")" != "true" ]; then
  docker start "$CONTAINER_NAME" >/dev/null
fi

docker exec "$CONTAINER_NAME" bash -lc '
  set -eu
  if ! command -v aarch64-linux-gnu-g++ >/dev/null 2>&1; then
    apt-get update
    DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      make g++-aarch64-linux-gnu binutils-aarch64-linux-gnu ca-certificates
  fi
  make clean all CXX=aarch64-linux-gnu-g++
  cp build/rk356x_demo build/rk356x_demo.unstripped
  aarch64-linux-gnu-strip build/rk356x_demo
  ls -lh build/rk356x_demo
'
