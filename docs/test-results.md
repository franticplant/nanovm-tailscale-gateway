# Local KVM Validation & Benchmark Results

Measured on a Fedora 43 workstation (Intel i7-7820HQ, QEMU/KVM) against a ~95-node Tailscale tailnet using:
- **Nanos kernel:** `0.1.56`
- **OPS:** `0.1.47`
- **Tailscale (`tsnet`):** `v1.106.0`
- **Go:** `1.27.4`

## 1. Idle resource usage & memory floor (1 vCPU)

| VM RAM | Boots & reaches `Running`? | Go `HeapAlloc` / `HeapInuse` | Go `Sys` | Goroutines | Host QEMU `VmRSS` | Idle CPU (5s) | Direct ping |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 256 MiB | Yes | `6.3 / 7.9 MiB` | `17.6 MiB` | 50 | `90.9 MiB` | `0.00%` | Yes |
| 160 MiB | Yes | `4.5 / 6.1 MiB` | `17.3 MiB` | 50 | `89.9 MiB` | `0.00%` | `0ms` |
| 128 MiB | Yes | `4.4 / 6.1 MiB` | `21.7 MiB` | 52 | `86.8 MiB` | `0.20%` | `1ms` |
| 96 MiB | Yes | `7.4 / 8.3 MiB` | `17.6 MiB` | 50 | `82.5 MiB` | `0.20%` | `1ms` |
| 64 MiB | Yes | `7.3 / 8.2 MiB` | `17.6 MiB` | 52 | `84.8 MiB` | `0.40%` | `1ms` |

## 2. TCP throughput over VirtIO TAP + Linux bridges (`iperf3`, 5s runs)

Isolated topology: host Tailscale client → `br-nanos-up` (`tap-nanos-up`) → Nanos `en1` (`10.231.99.2`) → `tsnet` + gVisor `netstack` → `forwarder.TCP` → Nanos `en2` (`10.231.10.1`) → `br-nanos-a` (`tap-nanos-a`, no host IP) → `nanos-target-a` (`10.231.10.10:5201`).

| Config | TX (1 stream) | RX (1 stream) | TX (4 streams) | Go `HeapAlloc` / `Sys` | Host QEMU `VmRSS` | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| 256 MiB / 1 vCPU | `488 Mbit/s` | `501 Mbit/s` | `444 Mbit/s` | `6.2 / 30.0 MiB` | `105.5 MiB` | CPU-bound (`~22%` vCPU per 100 Mbit/s) |
| 256 MiB / 2 vCPU | `633 Mbit/s` | `707 Mbit/s` | `596 Mbit/s` | `9.8 / 30.0 MiB` | `121.8 MiB` | CPU-bound (lwIP lock limits scaling) |
| 128 MiB / 1 vCPU | `298 Mbit/s` | `367 Mbit/s` | `322 Mbit/s` | `7.2 / 25.9 MiB` | `114.1 MiB` | Clean |
| 64 MiB / 1 vCPU | `291 Mbit/s` | `361 Mbit/s` | `319 Mbit/s` | `5.7 / 29.9 MiB` | `103.9 MiB` | Clean (minimum recommended RAM) |
| 48 MiB / 1 vCPU | 1st connect timed out | `332 Mbit/s` | `328 Mbit/s` | `7.1 / 17.6 MiB` | `93.8 MiB` | Stability edge |
| 32 MiB / 1 vCPU | OOM under load | — | — | — | — | Boots & pings at idle; crashes under bulk TCP |

## 3. Sizing recommendation

- **Minimum reliable:** `64 MiB` RAM / `1 vCPU` (~300–360 Mbit/s TCP, `0.4%` idle CPU).
- **Recommended:** `128–256 MiB` RAM / `1 vCPU` (~500 Mbit/s TCP at 256 MiB, generous GC/connection headroom).
