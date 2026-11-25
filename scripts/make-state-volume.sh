#!/usr/bin/env bash
# Build a Nanos TFS state volume (raw + qcow2) labeled `gw-state` containing
# /config.json and an optional one-shot /authkey file.
#
# Usage:
#   ./scripts/make-state-volume.sh \
#     --hostname pve-gw \
#     --routes 10.200.0.100/32 \
#     --authkey-file /path/to/single-use-authkey.txt \
#     [--size 64m] [--out build/gw-state]
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=../config/versions.env
source "${ROOT_DIR}/config/versions.env"

CACHE_OPS="${CACHE_OPS:-${HOME}/.cache/ops}"
HOSTNAME_VAL="nanos-tsnet-gateway"
ROUTES_CSV=""
AUTHKEY_FILE=""
VOL_SIZE="64m"
OUT_PREFIX="${ROOT_DIR}/build/gw-state"
VOL_LABEL="gw-state"

while [ $# -gt 0 ]; do
  case "$1" in
    --hostname)     HOSTNAME_VAL="$2"; shift 2 ;;
    --routes)       ROUTES_CSV="$2"; shift 2 ;;
    --authkey-file) AUTHKEY_FILE="$2"; shift 2 ;;
    --size)         VOL_SIZE="$2"; shift 2 ;;
    --out)          OUT_PREFIX="$2"; shift 2 ;;
    -h|--help)
      sed -n '2,10p' "$0"
      exit 0
      ;;
    *) echo "error: unknown option $1" >&2; exit 2 ;;
  esac
done

if [ -z "${ROUTES_CSV}" ]; then
  echo "error: --routes is required (e.g. --routes 10.0.200.100/32)" >&2
  exit 2
fi

if [ ! -x "${CACHE_OPS}/bin/ops" ]; then
  echo "error: ops not found in ${CACHE_OPS}/bin/ops; run ./scripts/fetch.sh first" >&2
  exit 1
fi

STAGE_DIR="$(mktemp -d)"
trap 'rm -rf "${STAGE_DIR}"' EXIT

mkdir -p "${STAGE_DIR}/data/tailscale" "$(dirname "${OUT_PREFIX}")"

# Build routes JSON array cleanly via python3 (stdlib only).
python3 - "${HOSTNAME_VAL}" "${ROUTES_CSV}" "${AUTHKEY_FILE}" "${STAGE_DIR}/data/config.json" <<'PY'
import json, sys
hostname, routes_csv, authkey_file, out_path = sys.argv[1:5]
routes = [r.strip() for r in routes_csv.split(",") if r.strip()]
cfg = {
    "hostname": hostname,
    "state_dir": "/data/tailscale",
    "routes": routes,
}
if authkey_file:
    cfg["authkey_file"] = "/data/authkey"
with open(out_path, "w") as f:
    json.dump(cfg, f, indent=2)
    f.write("\n")
PY

if [ -n "${AUTHKEY_FILE}" ]; then
  if [ ! -f "${AUTHKEY_FILE}" ]; then
    echo "error: authkey file not found: ${AUTHKEY_FILE}" >&2
    exit 1
  fi
  install -m 0600 "${AUTHKEY_FILE}" "${STAGE_DIR}/data/authkey"
fi

# Create a fresh TFS volume labeled `gw-state` inside an offline container.
# Uses a temporary OPS_DIR linking Nanos binaries (excluding existing volumes) so we never clobber ~/.cache/ops.
mkdir -p "${STAGE_DIR}/ops-work/.ops/volumes"

echo "==> Creating Nanos TFS state volume '${VOL_LABEL}' (${VOL_SIZE}) offline (--network=none)"
podman run --rm --network=none \
  -v "${CACHE_OPS}:/ops:ro,z" \
  -v "${STAGE_DIR}:/stage:z" \
  -e HOME=/stage/ops-work \
  -e OPS_DIR=/stage/ops-work/.ops \
  "${OPS_CONTAINER_IMAGE}" \
  sh -c '
    set -eu
    for item in /ops/.ops/*; do
      base="$(basename "$item")"
      [ "$base" = "volumes" ] && continue
      ln -s "$item" "/stage/ops-work/.ops/$base"
    done
    /ops/bin/ops volume create "'"${VOL_LABEL}"'" \
      --nanos-version "'"${NANOS_VERSION}"'" \
      -d /stage/data \
      -s "'"${VOL_SIZE}"'"
  '

RAW_VOL="$(find "${STAGE_DIR}/ops-work/.ops/volumes" -maxdepth 1 -type f -name "${VOL_LABEL}:*.raw" | head -n1)"
if [ -z "${RAW_VOL}" ]; then
  echo "error: expected ${VOL_LABEL}:*.raw volume was not created" >&2
  exit 1
fi

install -m 0600 "${RAW_VOL}" "${OUT_PREFIX}.raw"
if command -v qemu-img >/dev/null 2>&1; then
  qemu-img convert -f raw -O qcow2 -c "${OUT_PREFIX}.raw" "${OUT_PREFIX}.qcow2"
  chmod 0600 "${OUT_PREFIX}.qcow2"
  echo "==> Built ${OUT_PREFIX}.qcow2 (and ${OUT_PREFIX}.raw) — mode 0600 (contains authkey if supplied)"
else
  echo "==> Built ${OUT_PREFIX}.raw — mode 0600 (install qemu-img to also produce .qcow2)"
fi
