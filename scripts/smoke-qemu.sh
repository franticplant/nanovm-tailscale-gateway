#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE_PATH="${1:-${ROOT_DIR}/build/gateway-smoke.img}"
TIMEOUT_SECS="${SMOKE_TIMEOUT_SECS:-30}"
LOG_FILE="${ROOT_DIR}/build/smoke-serial.log"

if [ ! -f "${IMAGE_PATH}" ]; then
  echo "error: image not found: ${IMAGE_PATH}" >&2
  exit 1
fi

KVM_FLAGS=()
if [ -r /dev/kvm ] && [ -w /dev/kvm ]; then
  KVM_FLAGS=(-enable-kvm -cpu host)
  echo "==> Booting Nanos smoke image under QEMU/KVM"
else
  KVM_FLAGS=(-cpu max)
  echo "==> Booting Nanos smoke image under QEMU TCG (no /dev/kvm)"
fi

rm -f "${LOG_FILE}"
set +e
timeout "${TIMEOUT_SECS}s" qemu-system-x86_64 \
  -machine q35 "${KVM_FLAGS[@]}" \
  -m 512M -smp 1 \
  -display none -monitor none -serial "file:${LOG_FILE}" -no-reboot \
  -drive "file=${IMAGE_PATH},format=raw,if=none,id=hd0" \
  -device virtio-blk-pci,drive=hd0 \
  -netdev user,id=uplink \
  -device virtio-net-pci,netdev=uplink \
  -netdev user,id=vnet_a,net=10.231.10.0/24,restrict=on \
  -device virtio-net-pci,netdev=vnet_a \
  >/dev/null 2>&1
RC=$?
set -e

CLEAN_LOG="$(tr -d '\000\r' < "${LOG_FILE}")"
echo "${CLEAN_LOG}"

if ! printf '%s\n' "${CLEAN_LOG}" | grep -q "SMOKE_TEST_OK"; then
  echo "FAIL: SMOKE_TEST_OK not found in Nanos serial output (qemu rc=${RC})" >&2
  exit 1
fi
if ! printf '%s\n' "${CLEAN_LOG}" | grep -q "en2.*10\.231\.10\.1"; then
  echo "FAIL: expected static interface en2 (10.231.10.1) not found in serial output" >&2
  exit 1
fi
if printf '%s\n' "${CLEAN_LOG}" | grep -qiE "panic:|fatal error:"; then
  echo "FAIL: panic detected in Nanos serial output" >&2
  exit 1
fi

echo "==> PASS: Nanos QEMU boot smoke test succeeded (multi-NIC + SMOKE_TEST_OK)"
