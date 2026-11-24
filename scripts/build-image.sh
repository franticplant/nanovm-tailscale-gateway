#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=../config/versions.env
source "${ROOT_DIR}/config/versions.env"

CACHE_OPS="${CACHE_OPS:-${HOME}/.cache/ops}"
OPS_CONFIG="${1:-config/ops.smoke.json}"
IMAGE_NAME="${2:-gateway-smoke}"
OUT_DIR="${ROOT_DIR}/build"

mkdir -p "${OUT_DIR}"

if [ ! -x "${ROOT_DIR}/build/gateway" ]; then
  echo "error: build/gateway not found; run scripts/build-binary.sh first" >&2
  exit 1
fi

echo "==> Building Nanos ${NANOS_VERSION} image offline (--network=none) with ${OPS_CONFIG}"
podman run --rm --network=none \
  -v "${ROOT_DIR}:/src:z" \
  -v "${CACHE_OPS}:/ops:z" \
  -e HOME=/ops \
  -e OPS_DIR=/ops/.ops \
  -w /src \
  "${OPS_CONTAINER_IMAGE}" \
  /ops/bin/ops build build/gateway --nanos-version "${NANOS_VERSION}" -c "${OPS_CONFIG}" -i "${IMAGE_NAME}"

SRC_IMG="${CACHE_OPS}/.ops/images/${IMAGE_NAME}"
cp "${SRC_IMG}" "${OUT_DIR}/${IMAGE_NAME}.img"

if command -v qemu-img >/dev/null 2>&1; then
  qemu-img convert -f raw -O qcow2 -c "${OUT_DIR}/${IMAGE_NAME}.img" "${OUT_DIR}/${IMAGE_NAME}.qcow2"
fi

GIT_SHA="$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || echo 'unknown')"
IMG_SHA256="$(sha256sum "${OUT_DIR}/${IMAGE_NAME}.img" | awk '{print $1}')"
BIN_SHA256="$(sha256sum "${OUT_DIR}/gateway" | awk '{print $1}')"

cat > "${OUT_DIR}/${IMAGE_NAME}.build-info.json" <<EOF
{
  "image_name": "${IMAGE_NAME}",
  "git_sha": "${GIT_SHA}",
  "go_image": "${GO_IMAGE}",
  "nanos_version": "${NANOS_VERSION}",
  "ops_version": "${OPS_VERSION}",
  "ops_config": "${OPS_CONFIG}",
  "binary_sha256": "${BIN_SHA256}",
  "image_sha256": "${IMG_SHA256}",
  "built_at_utc": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF

(cd "${OUT_DIR}" && sha256sum "${IMAGE_NAME}.img" > "${IMAGE_NAME}.SHA256SUMS")
if [ -f "${OUT_DIR}/${IMAGE_NAME}.qcow2" ]; then
  (cd "${OUT_DIR}" && sha256sum "${IMAGE_NAME}.qcow2" >> "${IMAGE_NAME}.SHA256SUMS")
fi

echo "==> Built ${OUT_DIR}/${IMAGE_NAME}.img (sha256: ${IMG_SHA256})"
