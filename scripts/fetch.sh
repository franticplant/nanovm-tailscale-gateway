#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=../config/versions.env
source "${ROOT_DIR}/config/versions.env"

CACHE_GO_MOD="${CACHE_GO_MOD:-${HOME}/.cache/go-mod}"
CACHE_GO_BUILD="${CACHE_GO_BUILD:-${HOME}/.cache/go-build}"
CACHE_OPS="${CACHE_OPS:-${HOME}/.cache/ops}"

mkdir -p "${CACHE_GO_MOD}" "${CACHE_GO_BUILD}" "${CACHE_OPS}/bin"

echo "==> Fetching Go modules into ${CACHE_GO_MOD}"
podman run --rm \
  -v "${ROOT_DIR}:/src:ro,z" \
  -v "${CACHE_GO_MOD}:/go/pkg/mod:z" \
  -v "${CACHE_GO_BUILD}:/root/.cache/go-build:z" \
  -w /src \
  "${GO_IMAGE}" \
  go mod download

echo "==> Fetching OPS and Nanos ${NANOS_VERSION} into ${CACHE_OPS}"
if [ ! -x "${CACHE_OPS}/bin/ops" ]; then
  podman run --rm \
    -v "${CACHE_OPS}:/ops:z" \
    "${GO_IMAGE}" \
    sh -c 'wget -qO /ops/bin/ops https://storage.googleapis.com/cli/linux/ops && chmod +x /ops/bin/ops'
fi

podman run --rm \
  -v "${CACHE_OPS}:/ops:z" \
  -e HOME=/ops \
  -e OPS_DIR=/ops/.ops \
  "${OPS_CONTAINER_IMAGE}" \
  sh -c "/ops/bin/ops version && /ops/bin/ops update --version ${NANOS_VERSION} || /ops/bin/ops update"
