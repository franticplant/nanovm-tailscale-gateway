#!/usr/bin/env bash
set -euo pipefail

IMAGE_PATH="${1:?usage: run-qemu.sh <system.img> <state.raw>}"
STATE_DISK="${2:?usage: run-qemu.sh <system.img> <state.raw>}"
MEMORY="${QEMU_MEMORY:-512M}"

KVM_FLAGS=()
if [ -r /dev/kvm ] && [ -w /dev/kvm ]; then
  KVM_FLAGS=(-enable-kvm -cpu host)
else
  KVM_FLAGS=(-cpu max)
fi

exec qemu-system-x86_64 \
  -machine q35 "${KVM_FLAGS[@]}" \
  -m "${MEMORY}" -smp 1 \
  -display none -serial stdio -no-reboot \
  -drive "file=${IMAGE_PATH},format=raw,readonly=on,if=none,id=hd0" \
  -device virtio-blk-pci,drive=hd0 \
  -drive "file=${STATE_DISK},format=raw,if=none,id=hd1" \
  -device virtio-blk-pci,drive=hd1 \
  -netdev user,id=uplink \
  -device virtio-net-pci,netdev=uplink
