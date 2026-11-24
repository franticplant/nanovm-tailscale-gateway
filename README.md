# nanovm-tailscale-gateway

A minimal, immutable **Tailscale TCP subnet-routing appliance** built as a single statically compiled Go binary (`tailscale.com/tsnet`) running on the unmodified upstream **[Nanos](https://github.com/nanovms/nanos)** unikernel.

Designed for Proxmox VE / QEMU/KVM environments where you want a dedicated, shell-less, read-only Tailscale gateway into isolated management and guest VNets — with Tailscale identity persisted on a separate writable volume so the system image can be replaced atomically on every update.

## What it is (and is not)

- **Zero Linux userspace:** no shell, no SSH daemon, no package manager, no separate `tailscaled` daemon.
- **Unmodified upstream dependencies:** upstream Nanos kernel, upstream OPS, upstream Go, and upstream `tailscale.com/tsnet` (no forks or patches).
- **Immutable system disk + separate state volume:** `readonly_rootfs` system image with zero credentials baked in; Tailscale state lives on a dedicated `/data` VirtIO volume.
- **TCP-only subnet routing (by design):** uses `tsnet.Server.RegisterFallbackTCPHandler`, strictly scoped to explicitly advertised CIDRs (refuses `0.0.0.0/0`, `::/0`, loopback, and unadvertised destinations). Covers SSH, HTTPS APIs, WebSockets (e.g., Proxmox consoles), SFTP/SCP, and internal TCP services.
- **Not an exit node or UDP router:** `tsnet` does not expose a UDP fallback forwarding hook, and this appliance intentionally refuses default routes (`0.0.0.0/0` / `::/0`).
- **Source IP semantics:** userspace TCP proxying means destination services see the gateway's VNet IP, while end-to-end SSH/TLS encryption is preserved.

## Quick start (containerized, offline-capable)

Requires `podman`, `qemu-system-x86_64`, and `qemu-img`. All Go and OPS tooling runs inside plain temporary `podman run --rm` containers.

```bash
# 1. Fetch Go modules, OPS, and the pinned Nanos kernel into ~/.cache/
make fetch

# 2. Run Go vet + unit tests offline (--network=none)
make vet test

# 3. Build static amd64 binary + Nanos smoke image offline, then boot under QEMU
make smoke
```

See [docs/compatibility.md](docs/compatibility.md) for Nanos/tsnet runtime findings and [docs/architecture.md](docs/architecture.md) for the storage, multi-NIC, and update model.

## CI/CD & Release Discipline

- **PR CI (`ci.yml`):** `go vet`, `go test -race`, `govulncheck`, `gitleaks`, static amd64 build, pinned Nanos image build, and a real QEMU boot smoke test (`SMOKE_TEST_OK` + multi-NIC check).
- **Dependabot (`dependabot.yml` + `dependabot-automerge.yml`):** automated daily `gomod` and weekly `github-actions` updates; patch/minor updates auto-merge only when all CI checks pass.
- **Nanos updates (`check-nanos-updates.yml`):** daily check for new upstream Nanos releases, opening a PR gated by CI + QEMU smoke test.
- **Daily & tagged builds (`daily-build.yml`, `release.yml`):** publishes `.img`, `.qcow2`, `SHA256SUMS`, `build-info.json`, and GitHub build provenance attestations.
