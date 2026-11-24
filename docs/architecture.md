# Architecture

## Runtime model

```text
Tailnet client --(WireGuard)--> tsnet + gVisor netstack --> forwarder.TCP --> Nanos OS TCP socket --> VNet target
```

- **Uplink (`en1`):** DHCP (or static) interface with the default route for Tailscale control plane, DERP, and direct WireGuard UDP.
- **VNet interfaces (`en2`, `en3`, …):** static IPv4/netmask configured via Nanos `ManifestPassthrough` (`en2.ipaddr`, `en2.netmask`, etc.) with no default gateway.
- **Forwarding policy:** `internal/forwarder.TCP` implements `tsnet.FallbackTCPHandler`:
  - Fail-closed: rejects any destination not inside the explicitly configured `routes` prefixes, plus loopback (`127.0.0.0/8`, `::1`), unspecified, and multicast addresses.
  - Bounded concurrency (`max_concurrent`, default 256) and explicit TCP dial timeout (`dial_timeout`, default 10s).
  - Clean TCP half-close propagation (`CloseWrite`).

## Storage & update lifecycle

- **Disk 0 (System image):** immutable Nanos image built with `readonly_rootfs: "t"` and attached `readonly=on` in QEMU/Proxmox. Contains zero credentials.
- **Disk 1 (Persistent state volume):** mounted at `/data`; stores `/data/tailscale/tailscaled.state` and `TS_LOGS_DIR`. Survives abrupt VM termination and full system-disk replacement.

## Fallback path (Gokrazy)

`cmd/gateway` and `internal/*` contain zero Nanos-specific imports or syscalls. The exact same static amd64 binary runs on ordinary Linux and can be deployed onto Gokrazy if Nanos ever proves unsuitable for a target environment.
