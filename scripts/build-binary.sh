#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=../config/versions.env
source "${ROOT_DIR}/config/versions.env"

CACHE_GO_MOD="${CACHE_GO_MOD:-${HOME}/.cache/go-mod}"
CACHE_GO_BUILD="${CACHE_GO_BUILD:-${HOME}/.cache/go-build}"
OUT_BIN="${1:-build/gateway}"

mkdir -p "${ROOT_DIR}/$(dirname "${OUT_BIN}")"

echo "==> Building static amd64 binary offline (--network=none) -> ${OUT_BIN}"
podman run --rm --network=none \
  -v "${ROOT_DIR}:/src:z" \
  -v "${CACHE_GO_MOD}:/go/pkg/mod:z" \
  -v "${CACHE_GO_BUILD}:/root/.cache/go-build:z" \
  -w /src \
  "${GO_IMAGE}" \
  sh -c "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o '${OUT_BIN}' ./cmd/gateway"

sha256sum "${ROOT_DIR}/${OUT_BIN}"
