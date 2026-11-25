# Deploying on Proxmox VE (GUI-Only, No Console / WebSockets)

This guide deploys the generic `gateway.qcow2` release image alongside a small, site-specific `gw-state.qcow2` data volume using only the Proxmox VE web UI.

## Why two disks?

- **Disk 1 (`gateway.qcow2`, ~10 MiB):** 100% generic, immutable Nanos unikernel + Go `tsnet` gateway. Contains **no** site-specific routes, hostnames, or secrets. Mounts rootfs read-only and reads `/data/config.json` at boot.
- **Disk 2 (`gw-state.qcow2`, ~64 MiB):** Nanos **TFS** volume labeled `gw-state` (mounted at `/data`). Holds your `config.json`, optional one-shot `authkey` (auto-deleted after first login), and persistent `/data/tailscale/` node identity.

> **Important:** A blank disk created in the Proxmox GUI will **not** mount as `/data`, because Nanos expects its own TFS filesystem labeled `gw-state`. Always generate `gw-state.qcow2` locally with `scripts/make-state-volume.sh` and upload it.

---

## Step 1 — Generate `gw-state.qcow2` on your workstation

1. In the Tailscale Admin Console → **Settings → Keys**, generate a **single-use, tagged** auth key (not reusable) and save it locally with `chmod 600` (e.g. `/tmp/ts-once.key`).
2. Run:
   ```bash
   ./scripts/fetch.sh   # first time only, downloads ops + Nanos
   ./scripts/make-state-volume.sh \
     --hostname pve-gw \
     --routes 10.0.200.100/32 \
     --authkey-file /tmp/ts-once.key \
     --out build/gw-state
   rm -f /tmp/ts-once.key
   ```
   This produces `build/gw-state.qcow2` (`chmod 0600`).

3. Download the latest generic `gateway-*.qcow2` from GitHub Releases (or build locally with `make build && ./scripts/build-image.sh config/ops.example.json gateway`).

> **Note on IP addressing:** By default `gateway.qcow2` uses **DHCP** on its first NIC (`vmbr0`). If your management bridge does not have DHCP, add `"IPAddr"`, `"Gateway"`, and `"NetMask"` to a local copy of `config/ops.example.json` and rebuild `gateway.qcow2` locally.

---

## Step 2 — Upload & Create the VM in Proxmox VE GUI

1. **Upload both QCOW2 files** to a storage that allows disk images / import (Proxmox VE 8.2+ / 9.x **Content → Import**, or ISO/disk upload).
2. **Create VM:**
   - **OS:** *Do not use any media*
   - **System:** Machine `q35` (or `i440fx`), BIOS **`SeaBIOS`** *(do NOT use OVMF/UEFI)*
   - **Disks:** delete the default disk for now (or attach imported disks in the next step)
   - **CPU:** `1` or `2` cores, type `host` (or `x86-64-v2-AES`)
   - **Memory:** `128`–`256` MiB (minimum `64` MiB; Ballooning disabled)
   - **Network:** Bridge `vmbr0`, Model **`VirtIO (paravirtualized)`**
3. **Attach both disks as VirtIO Block:**
   - **`virtio0`:** `gateway.qcow2` (system disk)
   - **`virtio1`:** `gw-state.qcow2` (state/config disk)
4. **Options → Boot Order:** enable **`virtio0`** first.
5. **Start** the VM.

---

## Step 3 — Verify Without Console Access

1. **DHCP / NIC check:** Confirm the VM's MAC acquired an IP on your router's DHCP/ARP table.
2. **Enrollment check (2–10s after boot):** Open the **Tailscale Admin Console → Machines** — `pve-gw` should appear online and request approval for `10.0.200.100/32`.
3. **Approve the subnet route** in Tailscale Admin Console.
4. **Test access:** From any Tailscale device, open `https://10.0.200.100:8006` (or `curl -k https://10.0.200.100:8006/api2/json/version`).
5. **If it doesn't appear:** Check the Proxmox VM **Summary** graphs:
   - Healthy idle Nanos sits near `0–1%` CPU with small periodic network packets.
   - Flat `0%` CPU immediately after start or repeated restarts usually means wrong BIOS (must be SeaBIOS) or missing/unlabeled `gw-state` volume.
   - No DHCP lease on router means `vmbr0` needs DHCP or a static-IP build of `gateway.qcow2`.
