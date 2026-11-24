# Nanos + `tsnet` Compatibility Notes

Verified against:
- **Nanos kernel:** `0.1.56`
- **OPS:** `0.1.47`
- **Tailscale:** `tailscale.com@v1.96.5`
- **Go:** `1.26.8` (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`)

## Confirmed working on unmodified Nanos `0.1.56`

1. **Network monitor (`netmon`):** `AF_NETLINK` `RTMGRP` fails on Nanos, and Tailscale's Linux `netmon` automatically falls back to polling (`monitor_linux: AF_NETLINK RTMGRP failed, falling back to polling`).
2. **Interface discovery (`net.Interfaces()`):** enumerates `lo0`, `en1`, `en2`, `en3` with IPv4 and IPv6 link-local addresses.
3. **Multi-NIC mixed DHCP + static IPs:** `en1` via DHCP + `en2`/`en3` via `ManifestPassthrough` static `ipaddr`/`netmask` routes outbound VNet connections out the matching interface while preserving `en1` as the default route.
4. **Tailscale overlay:** HTTPS control-plane registration, DERP relay, direct UDP peer connections (`1ms` local), and `LocalClient().EditPrefs` subnet route advertisement all work.
5. **Immutable rootfs + separate persistent volume:** `readonly_rootfs: "t"` + QEMU `readonly=on` with a separate `/data` volume preserves Tailscale identity across abrupt VM kill and credential-free system image replacement.

## Required workarounds / caveats

1. **Set `TS_LOGS_DIR` and `HOME` in Nanos `Env`:**
   Without `TS_LOGS_DIR=/data/tailscale` (and `HOME=/data`), `tailscale.com/logpolicy.LogsDir` panics at startup with `panic: no safe place found to store log state`.
2. **`tsnet.Server` rejects unmatched flows by default:**
   Even though `tsnet` sets `ns.ProcessSubnets = true`, its `getTCPHandlerForFlow` returns `(nil, true)` unless a listener or `RegisterFallbackTCPHandler` handles the flow. Registering `internal/forwarder.TCP.FallbackHandler` enables TCP subnet forwarding with zero Tailscale patches.
3. **TCP-only via `tsnet`:**
   `tsnet@v1.96.5` has no `RegisterFallbackUDPHandler`. UDP subnet forwarding is intentionally out of scope while staying on the stable public `tsnet` API.
4. **Harmless `/proc/net/route` warning:**
   Tailscale logs `interfaces: failed to read /proc/net/route: open /proc/net/route: no such file or directory` once at startup; it does not affect control-plane, DERP, direct UDP, or multi-NIC forwarding.
